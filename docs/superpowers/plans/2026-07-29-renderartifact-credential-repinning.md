# RenderArtifact Credential Re-Pinning Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ensure a shared `RenderArtifact` always has access to the right credentials to delete its OCI tag in the finalizer step, even after the Target/Registry/Secret that originally created it are gone — without SolAr ever storing or copying Secret material itself.

**Architecture:** Neither `RenderArtifact` nor `RenderBinding` will reference a Secret at all. Instead, both carry a `RegistryRef *ObjectReference` pointing at the *Registry* that owns the credentials — exactly how `Target` already resolves its own push credentials (`Target` never touches a Secret directly, only `Registry.Spec.SolarSecretRef`). `RenderBinding` snapshots the Registry its owning Target currently resolves at binding-creation time. `RenderArtifactReconciler` re-pins `RenderArtifact.Spec.RegistryRef` from a deterministically-chosen still-existing `RenderBinding` on every reconcile where bindings remain (triggered by the existing watch on every `RenderBinding` create/update/delete event), so the pinned reference is always in sync with a live consumer right up until the last one disappears. At actual cleanup time, credentials are resolved live: `RegistryRef` → `Registry` → `Registry.Spec.SolarSecretRef` → `Secret`. Because `RegistryRef` is namespace-capable and Registries can live in a different namespace than the `RenderBinding`/`RenderArtifact` referencing them, cross-namespace resolution is gated by the same `ReferenceGrant` mechanism `Target` already uses for the identical relationship.

**Tech Stack:** Go, controller-runtime, a custom aggregated Kubernetes API server (`go.opendefense.cloud/kit/apiserver`) with hand-maintained internal/versioned type mirrors, `k8s.io/code-generator` for deepcopy/conversion/openapi/clientset codegen, Ginkgo v2 + Gomega tests against a real envtest-backed API server.

## Global Constraints

- Every API type exists in two mirrored packages that must stay field-for-field identical: `api/solar/v1alpha1/*_types.go` (versioned, external) and `api/solar/*_types.go` (internal). Some list-level conversions use `unsafe.Pointer` casts that require identical struct layout between them.
- New cross-namespace-capable reference fields must use the existing `ObjectReference{Name, Namespace}` type (`api/solar/v1alpha1/reference_types.go` / `api/solar/reference_types.go`), matching the convention already used for `Target.Spec.RenderRegistryRef`.
- Cross-namespace access to a `Registry` must be gated by a `ReferenceGrant`, exactly like `TargetReconciler` already does for `Target.Spec.RenderRegistryRef` (`pkg/controller/target_controller.go`, `registryGranted`/`grantPermitsRegistryAccess`). This plan promotes `registryGranted` from a `*TargetReconciler` method to a package-level function so `RenderArtifactReconciler` can reuse it unchanged. It deliberately reuses the existing grant check, which authorizes `(Group=solar.opendefense.cloud, Kind=Target)` as the `From` subject — it does **not** introduce a new `RenderArtifact`/`RenderBinding` grant kind. This means existing `ReferenceGrant` objects continue to work unmodified: the check is really "does this namespace have permission to reach that Registry namespace at all," and `RenderBinding`/`RenderArtifact` only ever reference a Registry a Target in the same namespace already had permission to reach when the binding was created.
- After changing any type in `api/solar/`, regenerate with `./hack/update-codegen.sh` (deepcopy, conversion, openapi, clientset/listers/informers/applyconfigurations), `make manifests` (RBAC), and `make docs-crd-ref` (regenerates `docs/user-guide/api-reference.md`). Never hand-edit any `zz_generated.*.go` file.
- Prefer `r.APIReader` (an uncached direct client, already present on `RenderArtifactReconciler`) over the cached `r.Client` for the new `Registry` and `ReferenceGrant` lookups in `RenderArtifactReconciler`. This avoids requiring a cluster-wide cache/watch (and the broader RBAC that implies) for resources this controller only occasionally reads during cleanup — the same reasoning already used for its existing `r.APIReader.List` binding-count double-check.
- No changes are needed to `web/src/` — nothing there references `RenderArtifact`, `RenderBinding`, or their credential fields.
- Controller tests live in `pkg/controller/*_test.go`, use Ginkgo v2 + Gomega, and run against a real aggregated API server started once for the whole package (`pkg/controller/suite_test.go`). The whole suite is labeled `Label("integration")`, so it only runs via `make test` (not `make test-unit`). Use the package constants `eventuallyTimeout` (8s) and `consistentlyDuration` (2s) with `Eventually`/`Consistently`. Run a scoped slice of the suite during development with `make test testargs="--label-filter=renderartifact"` or `testargs="--label-filter=target"`. `TestRegistryGranted`/`TestGrantPermitsRegistryAccess` in `pkg/controller/target_controller_grants_test.go` are plain `go test` unit tests (not Ginkgo), runnable directly with `go test ./pkg/controller/... -run TestRegistryGranted`.
- Do not add validation, defaulting, or immutability rules beyond what's specified below — `RegistryRef` is an optional reference, matching how `RenderArtifact.Spec.PushSecretRef` was unvalidated beyond the `ObjectReference.Name` marker before this change.

---

## File Structure

- Modify `api/solar/v1alpha1/renderbinding_types.go` / `api/solar/renderbinding_types.go` — add `RegistryRef *ObjectReference` to `RenderBindingSpec`.
- Modify `api/solar/v1alpha1/renderartifact_types.go` / `api/solar/renderartifact_types.go` — replace `PushSecretRef *ObjectReference`, `RegistryFlavor string`, `PlainHTTP bool` with a single `RegistryRef *ObjectReference` on `RenderArtifactSpec`.
- Regenerate (scripted, not hand-edited): `api/solar/v1alpha1/zz_generated.deepcopy.go`, `api/solar/zz_generated.deepcopy.go`, `api/solar/v1alpha1/zz_generated.conversion.go`, `client-go/**`, RBAC chart files, `docs/user-guide/api-reference.md`.
- Modify `pkg/controller/target_controller.go` — `ensureRenderArtifact`/`ensureRenderBinding` take the Target's already-resolved `RenderRegistryRef` and store it verbatim (no Secret/flavor plumbing needed at all, since `RenderArtifact`/`RenderBinding` live in the same namespace as their owning Target, so the same relative reference resolves correctly for either); `registryGranted` becomes a package-level function.
- Modify `pkg/controller/renderartifact_controller.go` — `resolveAuth` follows `RegistryRef` → `Registry` (with grant check) → `Secret`, returning the resolved `PlainHTTP` alongside the authenticator; `cleanupOCIArtifact` updated accordingly; new RBAC markers for `registries` (`get`) and `referencegrants` (`get;list`); new `repinCredentials`/`registryRefEqual` wired into `Reconcile`.
- Modify `pkg/controller/target_controller_grants_test.go` — update the 4 `registryGranted` call sites for the new package-level signature.
- Test: `pkg/controller/renderartifact_controller_test.go` — `RegistryRef` round-trip, rewritten `resolveAuth`/`GC with PlainHTTP` contexts, new "credential re-pinning" context with the bug's regression scenario.
- Test: `pkg/controller/target_controller_test.go` — replace the `RegistryFlavor` propagation test with one asserting `RegistryRef` propagation onto both `RenderArtifact` and `RenderBinding`.

---

### Task 1: Replace Secret/flavor snapshot fields with a live-resolved `RegistryRef`

**Files:**
- Modify: `api/solar/v1alpha1/renderbinding_types.go:11-24`, `api/solar/renderbinding_types.go:11-24`
- Modify: `api/solar/v1alpha1/renderartifact_types.go:11-35`, `api/solar/renderartifact_types.go:11-35`
- Modify: `pkg/controller/target_controller.go:476-495` (release call site), `pkg/controller/target_controller.go:606-615` (bootstrap call site), `pkg/controller/target_controller.go:887-967` (`ensureRenderArtifact`/`ensureRenderBinding`), `pkg/controller/target_controller.go:184-207` (call site of `registryGranted`), `pkg/controller/target_controller.go:1123-1138` (`registryGranted` definition)
- Modify: `pkg/controller/renderartifact_controller.go:61-65` (RBAC), `pkg/controller/renderartifact_controller.go:166-311` (`cleanupOCIArtifact`, `resolveAuth`)
- Modify: `pkg/controller/target_controller_grants_test.go:66-136` (`TestRegistryGranted` call sites)
- Regenerate: `api/solar/v1alpha1/zz_generated.deepcopy.go`, `api/solar/zz_generated.deepcopy.go`, `api/solar/v1alpha1/zz_generated.conversion.go`, `client-go/**`, RBAC chart files, `docs/user-guide/api-reference.md`
- Test: `pkg/controller/renderartifact_controller_test.go`, `pkg/controller/target_controller_test.go:1025-1062`

**Interfaces:**
- Produces: `solarv1alpha1.RenderBindingSpec.RegistryRef *solarv1alpha1.ObjectReference`, `solarv1alpha1.RenderArtifactSpec.RegistryRef *solarv1alpha1.ObjectReference` — consumed by Task 2's `repinCredentials`.
- Produces: package-level `registryGranted(ctx context.Context, reader client.Reader, registryNamespace, fromNamespace string) (bool, error)` in `pkg/controller/target_controller.go` — consumed by `RenderArtifactReconciler.resolveAuth`.
- Produces: `(*TargetReconciler).ensureRenderArtifact(ctx, name string, rt *solarv1alpha1.RenderTask, registryRef solarv1alpha1.ObjectReference) error` and `(*TargetReconciler).ensureRenderBinding(ctx, target *solarv1alpha1.Target, artifactName, bindingName string, registryRef solarv1alpha1.ObjectReference) error` (new signatures, replacing the old `flavor, pushSecretNamespace string` params).
- Produces: `(*RenderArtifactReconciler).resolveAuth(ctx, artifact *solarv1alpha1.RenderArtifact, registryHost string) (authn.Authenticator, bool, error)` (new signature — third return value is the resolved `PlainHTTP` setting) — consumed by `cleanupOCIArtifact`.

- [ ] **Step 1: Write the failing test**

Add to `pkg/controller/renderartifact_controller_test.go`, as a new `Context` right after the `Context("GC with PlainHTTP", ...)` block (still inside the outer `Describe("RenderArtifactController", ...)`):

```go
	Context("RegistryRef persistence", Label("renderartifact"), func() {
		It("should persist a cross-namespace RegistryRef on RenderArtifact and RenderBinding", func() {
			art := newArtifact("art-registryref-roundtrip")
			art.Spec.RegistryRef = &solarv1alpha1.ObjectReference{Name: "reg-a", Namespace: "ns-a"}
			Expect(k8sClient.Create(ctx, art)).To(Succeed())

			gotArt := &solarv1alpha1.RenderArtifact{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(art), gotArt)).To(Succeed())
			Expect(gotArt.Spec.RegistryRef).To(Equal(&solarv1alpha1.ObjectReference{Name: "reg-a", Namespace: "ns-a"}))

			binding := newBinding("binding-registryref-roundtrip", "art-registryref-roundtrip")
			binding.Spec.RegistryRef = &solarv1alpha1.ObjectReference{Name: "reg-b", Namespace: "ns-b"}
			Expect(k8sClient.Create(ctx, binding)).To(Succeed())

			gotBinding := &solarv1alpha1.RenderBinding{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(binding), gotBinding)).To(Succeed())
			Expect(gotBinding.Spec.RegistryRef).To(Equal(&solarv1alpha1.ObjectReference{Name: "reg-b", Namespace: "ns-b"}))
		})
	})
```

Also remove the `RegistryFlavor: "zot"` line from the `newArtifact` helper (near the top of the same `Describe` block) — it references a field this task removes:

```go
	newArtifact := func(name string) *solarv1alpha1.RenderArtifact {
		return &solarv1alpha1.RenderArtifact{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns.Name,
			},
			Spec: solarv1alpha1.RenderArtifactSpec{
				BaseURL:       "registry.example.com",
				Repository:    "ns/myapp",
				Tag:           "v1.0.0",
				RenderTaskRef: "rt-" + name,
			},
		}
	}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `make test testargs="--label-filter=renderartifact --focus='RegistryRef persistence'"`
Expected: compile error — `art.Spec.RegistryRef` undefined (and `RegistryFlavor` still referenced by other existing tests at this point, which is fine; those get fixed in later steps of this same task).

- [ ] **Step 3: Add `RegistryRef` to `RenderBindingSpec` on both type mirrors**

Edit `api/solar/v1alpha1/renderbinding_types.go`, adding this field at the end of `RenderBindingSpec` (after `OwnerNamespace`):

```go
	// RegistryRef references the Registry this binding's owner currently resolves for
	// pushing the shared RenderArtifact. The RenderArtifact controller re-pins
	// RenderArtifact.Spec.RegistryRef from a surviving RenderBinding's value whenever a
	// binding is removed, so the artifact always resolves credentials through a Registry
	// belonging to a consumer that still exists. RenderArtifact/RenderBinding never store
	// Secret-identifying information directly — only a reference to the Registry that owns
	// the credentials, resolved fresh at use time, mirroring how Target resolves its own
	// push credentials.
	// +optional
	RegistryRef *ObjectReference `json:"registryRef,omitempty"`
```

Apply the identical field (same name, type, json tag, doc comment) to `api/solar/renderbinding_types.go`.

- [ ] **Step 4: Replace `PushSecretRef`/`RegistryFlavor`/`PlainHTTP` with `RegistryRef` on `RenderArtifactSpec`**

Edit `api/solar/v1alpha1/renderartifact_types.go`, replacing:

```go
	// PushSecretRef references a Secret containing registry credentials used to push this
	// artifact. Used for tag deletion during GC. When Namespace is empty, the Secret is
	// resolved in the RenderArtifact's own namespace; a non-empty Namespace identifies the
	// namespace the referenced Secret lives in.
	// +optional
	PushSecretRef *ObjectReference `json:"pushSecretRef,omitempty"`
	// RegistryFlavor identifies the registry implementation (e.g. "zot", "harbor").
	// +optional
	RegistryFlavor string `json:"registryFlavor,omitempty"`
	// PlainHTTP uses HTTP instead of HTTPS for OCI registry connections.
	// +optional
	PlainHTTP bool `json:"plainHTTP,omitempty"`
```

with:

```go
	// RegistryRef references the Registry that owns the credentials used to push (and
	// later delete) this artifact's OCI tag. When Namespace is empty, the Registry is
	// resolved in the RenderArtifact's own namespace; a non-empty Namespace identifies a
	// different namespace and requires a ReferenceGrant there permitting access, mirroring
	// how Target resolves its RenderRegistryRef. RenderArtifact never stores Secret- or
	// PlainHTTP-identifying information directly: both are read live from the referenced
	// Registry whenever credentials are needed, so a Registry's credentials or transport
	// settings can change without ever going stale on the artifact.
	// +optional
	RegistryRef *ObjectReference `json:"registryRef,omitempty"`
```

Apply the identical replacement to `api/solar/renderartifact_types.go`.

- [ ] **Step 5: Regenerate deepcopy, conversion, openapi and clientset**

Run:
```bash
./hack/update-codegen.sh
```
This updates `api/solar/v1alpha1/zz_generated.deepcopy.go`, `api/solar/zz_generated.deepcopy.go`, `api/solar/v1alpha1/zz_generated.conversion.go`, and everything under `client-go/`. Do not hand-edit any of these; `go build ./...` will still fail after this step because production code in `pkg/controller/` still references the removed fields — that's expected and fixed in the next steps.

- [ ] **Step 6: Promote `registryGranted` to a package-level function**

In `pkg/controller/target_controller.go:1123-1138`, replace:

```go
// registryGranted checks whether a ReferenceGrant in registryNamespace permits
// fromNamespace to reference the named registry.
func (r *TargetReconciler) registryGranted(ctx context.Context, registryNamespace, fromNamespace string) (bool, error) {
	grantList := &solarv1alpha1.ReferenceGrantList{}
	if err := r.List(ctx, grantList, client.InNamespace(registryNamespace)); err != nil {
		return false, err
	}
	for i := range grantList.Items {
		grant := &grantList.Items[i]
		if grantPermitsRegistryAccess(grant, fromNamespace) {
			return true, nil
		}
	}

	return false, nil
}
```

with:

```go
// registryGranted checks whether a ReferenceGrant in registryNamespace permits
// fromNamespace to reference the named registry. Package-level (not a *TargetReconciler
// method) so RenderArtifactReconciler can reuse it with its own client.Reader (typically
// an uncached APIReader) rather than duplicating the grant-lookup logic.
func registryGranted(ctx context.Context, reader client.Reader, registryNamespace, fromNamespace string) (bool, error) {
	grantList := &solarv1alpha1.ReferenceGrantList{}
	if err := reader.List(ctx, grantList, client.InNamespace(registryNamespace)); err != nil {
		return false, err
	}
	for i := range grantList.Items {
		grant := &grantList.Items[i]
		if grantPermitsRegistryAccess(grant, fromNamespace) {
			return true, nil
		}
	}

	return false, nil
}
```

Update its one call site at `pkg/controller/target_controller.go:193`, changing:
```go
		granted, err := r.registryGranted(ctx, registryNamespace, target.Namespace)
```
to:
```go
		granted, err := registryGranted(ctx, r.Client, registryNamespace, target.Namespace)
```

- [ ] **Step 7: Update `TestRegistryGranted` for the new signature**

In `pkg/controller/target_controller_grants_test.go`, in each of the four `t.Run(...)` subtests inside `TestRegistryGranted` (`pkg/controller/target_controller_grants_test.go:66-136`), remove the `r := &TargetReconciler{Client: c, Scheme: sch}` line and change the call from `r.registryGranted(context.Background(), "registry-ns", "target-ns")` to `registryGranted(context.Background(), c, "registry-ns", "target-ns")`. For example, the first subtest becomes:

```go
	t.Run("returns true when a matching grant exists in the registry namespace", func(t *testing.T) {
		t.Parallel()
		grant := newRegistryGrant("target-ns", "Registry")
		c := fake.NewClientBuilder().WithScheme(sch).WithObjects(grant).Build()

		granted, err := registryGranted(context.Background(), c, "registry-ns", "target-ns")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !granted {
			t.Error("expected registry access to be granted")
		}
	})
```

Apply the same mechanical change (drop `r :=`, call `registryGranted(context.Background(), c, ...)` directly) to the other three subtests ("returns false when no grant exists", "returns false when the grant permits a different from-namespace", "propagates the error when the List call fails").

- [ ] **Step 8: Update `ensureRenderArtifact` and `ensureRenderBinding`**

Replace `pkg/controller/target_controller.go:887-967` in full with:

```go
// ensureRenderArtifact creates a RenderArtifact for the given RenderTask's OCI coordinates
// if one does not already exist. Idempotent: if it already exists (possibly created by
// another Target reconciling the same shared artifact), this is a no-op — RegistryRef for
// an existing artifact is kept in sync separately, by RenderArtifactReconciler re-pinning
// from RenderBinding snapshots (see ensureRenderBinding below).
func (r *TargetReconciler) ensureRenderArtifact(ctx context.Context, name string, rt *solarv1alpha1.RenderTask, registryRef solarv1alpha1.ObjectReference) error {
	artifact := &solarv1alpha1.RenderArtifact{}
	if err := r.Get(ctx, client.ObjectKey{Name: name, Namespace: rt.Namespace}, artifact); err == nil {
		if !artifact.DeletionTimestamp.IsZero() {
			// The artifact is terminating (OCI cleanup in progress). Creating a binding
			// against it would race with the finalizer. Requeue and wait for full deletion.
			return fmt.Errorf("RenderArtifact %s/%s is terminating; requeuing", rt.Namespace, name)
		}

		return nil
	} else if !apierrors.IsNotFound(err) {
		return err
	}

	artifact = &solarv1alpha1.RenderArtifact{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rt.Namespace,
		},
		Spec: solarv1alpha1.RenderArtifactSpec{
			BaseURL:       rt.Spec.BaseURL,
			Repository:    rt.Spec.Repository,
			Tag:           rt.Spec.Tag,
			RenderTaskRef: rt.Name,
			RegistryRef:   &registryRef,
		},
	}

	if err := r.Create(ctx, artifact); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

// ensureRenderBinding creates a RenderBinding linking this Target to the named
// RenderArtifact if one does not already exist. Idempotent.
//
// The binding snapshots the Registry this Target currently resolves (registryRef) so
// RenderArtifactReconciler can re-pin the shared RenderArtifact's RegistryRef from a
// surviving binding whenever another binding referencing the same artifact is removed.
// Like ensureRenderArtifact, this snapshot is write-once: if the Target's
// RenderRegistryRef changes while this binding still exists, the binding is not
// refreshed. That's a separate, narrower staleness problem than the one this plan fixes
// (a deleted consumer's Registry reference outliving its binding).
func (r *TargetReconciler) ensureRenderBinding(ctx context.Context, target *solarv1alpha1.Target, artifactName, bindingName string, registryRef solarv1alpha1.ObjectReference) error {
	binding := &solarv1alpha1.RenderBinding{}
	if err := r.Get(ctx, client.ObjectKey{Name: bindingName, Namespace: target.Namespace}, binding); err == nil {
		return nil
	} else if !apierrors.IsNotFound(err) {
		return err
	}

	binding = &solarv1alpha1.RenderBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bindingName,
			Namespace: target.Namespace,
		},
		Spec: solarv1alpha1.RenderBindingSpec{
			RenderArtifactRef: corev1.LocalObjectReference{Name: artifactName},
			OwnerKind:         "Target",
			OwnerName:         target.Name,
			OwnerNamespace:    target.Namespace,
			RegistryRef:       &registryRef,
		},
	}

	if err := r.Create(ctx, binding); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}
```

Note `registryRef` is taken by value: `RenderArtifact`/`RenderBinding` always live in the same namespace as their owning `Target` (both are created with `Namespace: rt.Namespace` / `Namespace: target.Namespace`, which is the Target's own namespace), so `Target.Spec.RenderRegistryRef` — an empty `Namespace` meaning "same namespace as me" — resolves identically whether "me" is the Target, the RenderArtifact, or the RenderBinding. No separate namespace plumbing (like the old `pushSecretNamespace` parameter) is needed at all.

- [ ] **Step 9: Update both call sites**

In `pkg/controller/target_controller.go:476-495` (per-release loop), change:
```go
			if err := r.ensureRenderBinding(ctx, target, aName, bName); err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to ensure RenderBinding for release")
			}
			if err := r.ensureRenderArtifact(ctx, aName, rt, registry.Spec.Flavor, registryNamespace); err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to ensure RenderArtifact for release")
			}
```
to:
```go
			if err := r.ensureRenderBinding(ctx, target, aName, bName, target.Spec.RenderRegistryRef); err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to ensure RenderBinding for release")
			}
			if err := r.ensureRenderArtifact(ctx, aName, rt, target.Spec.RenderRegistryRef); err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to ensure RenderArtifact for release")
			}
```

In `pkg/controller/target_controller.go:606-615` (bootstrap path), change:
```go
		if err := r.ensureRenderBinding(ctx, target, bootstrapArtifactName, bootstrapBindingName); err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to ensure RenderBinding for bootstrap")
		}
		if err := r.ensureRenderArtifact(ctx, bootstrapArtifactName, bootstrapRT, registry.Spec.Flavor, registryNamespace); err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to ensure RenderArtifact for bootstrap")
		}
```
to:
```go
		if err := r.ensureRenderBinding(ctx, target, bootstrapArtifactName, bootstrapBindingName, target.Spec.RenderRegistryRef); err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to ensure RenderBinding for bootstrap")
		}
		if err := r.ensureRenderArtifact(ctx, bootstrapArtifactName, bootstrapRT, target.Spec.RenderRegistryRef); err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to ensure RenderArtifact for bootstrap")
		}
```

- [ ] **Step 10: Add RBAC and rewrite `resolveAuth`/`cleanupOCIArtifact`**

In `pkg/controller/renderartifact_controller.go:61-65`, replace:
```go
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=renderartifacts,verbs=get;list;watch;update;patch;delete
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=renderartifacts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=renderartifacts/finalizers,verbs=update
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=renderbindings,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get
```
with:
```go
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=renderartifacts,verbs=get;list;watch;update;patch;delete
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=renderartifacts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=renderartifacts/finalizers,verbs=update
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=renderbindings,verbs=get;list;watch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=registries,verbs=get
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=referencegrants,verbs=get;list
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get
```

Replace `pkg/controller/renderartifact_controller.go:166-311` (`cleanupOCIArtifact` through `resolveAuth`, i.e. everything up to but not including `ociAuthFromSecret`) with:

```go
// cleanupOCIArtifact attempts to delete the OCI tag from the registry.
// On failure it sets a status condition and fires a Warning event so the user
// can see why the RenderArtifact is stuck, then returns the error to keep the
// finalizer in place.
func (r *RenderArtifactReconciler) cleanupOCIArtifact(ctx context.Context, artifact *solarv1alpha1.RenderArtifact) error {
	log := ctrl.LoggerFrom(ctx)

	registryHost := strings.TrimPrefix(strings.TrimSuffix(artifact.Spec.BaseURL, "/"), "oci://")
	rawRef := registryHost + "/" + strings.TrimPrefix(artifact.Spec.Repository, "/") + ":" + artifact.Spec.Tag
	log.V(1).Info("Attempting OCI tag cleanup", "ref", rawRef)

	deleteFn := r.DeleteTag
	if deleteFn == nil {
		deleteFn = ociregistry.DeleteTag
	}

	auth, plainHTTP, err := r.resolveAuth(ctx, artifact, registryHost)
	if err != nil {
		log.Error(err, "Failed to resolve OCI auth; RenderArtifact will remain until secret is accessible",
			"artifact", artifact.Name)
		r.Recorder.Eventf(artifact, nil, corev1.EventTypeWarning,
			"OCICleanupFailed", "Delete",
			"Failed to resolve OCI auth for %s: %s", rawRef, err.Error())

		latest := artifact.DeepCopy()
		apimeta.SetStatusCondition(&latest.Status.Conditions, metav1.Condition{
			Type:               ConditionTypeOCICleanup,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: artifact.Generation,
			Reason:             "AuthFailed",
			Message:            err.Error(),
		})
		if sErr := r.Status().Patch(ctx, latest, client.MergeFrom(artifact)); sErr != nil {
			log.Error(sErr, "failed to update status condition after OCI auth failure")
		}

		return err
	}

	deleteCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := deleteFn(deleteCtx, rawRef, auth, plainHTTP); err != nil {
		// If the tag is already gone, proceed normally.
		var transportErr *transport.Error
		if errors.As(err, &transportErr) && transportErr.StatusCode == http.StatusNotFound {
			log.V(1).Info("OCI tag already absent — skipping delete", "ref", rawRef)
			return nil
		}

		log.Error(err, "Failed to delete OCI tag; RenderArtifact will remain until deletion succeeds",
			"ref", rawRef, "artifact", artifact.Name)
		r.Recorder.Eventf(artifact, nil, corev1.EventTypeWarning,
			"OCICleanupFailed", "Delete",
			"Failed to delete OCI tag %s: %s", rawRef, err.Error())

		latest := artifact.DeepCopy()
		apimeta.SetStatusCondition(&latest.Status.Conditions, metav1.Condition{
			Type:               ConditionTypeOCICleanup,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: artifact.Generation,
			Reason:             "DeleteFailed",
			Message:            err.Error(),
		})
		// Status patch, if it fails, the event + log are visible in kubectl
		if sErr := r.Status().Patch(ctx, latest, client.MergeFrom(artifact)); sErr != nil {
			log.Error(sErr, "failed to update status condition after OCI cleanup failure")
		}

		return err
	}

	log.V(1).Info("OCI tag deleted successfully", "ref", rawRef)
	r.Recorder.Eventf(artifact, nil, corev1.EventTypeNormal,
		"OCICleanupSucceeded", "Delete",
		"Successfully deleted OCI tag %s", rawRef)

	return nil
}

// resolveAuth resolves the authn.Authenticator and PlainHTTP setting to use for OCI
// operations on artifact, by following its RegistryRef to the live Registry and, from
// there, to the Secret the Registry currently designates for push credentials.
// RenderArtifact never stores secret-identifying information itself — only a reference
// to the Registry that owns the credentials — so rotating a Registry's SolarSecretRef in
// place is reflected automatically on the next resolution. Uses r.APIReader (an uncached
// direct client) for the Registry and ReferenceGrant lookups so this controller does not
// need a cluster-wide watch on either resource type.
// Returns authn.Anonymous, false, nil if the artifact has no RegistryRef configured.
func (r *RenderArtifactReconciler) resolveAuth(ctx context.Context, artifact *solarv1alpha1.RenderArtifact, registryHost string) (authn.Authenticator, bool, error) {
	log := ctrl.LoggerFrom(ctx)

	if artifact.Spec.RegistryRef == nil {
		return authn.Anonymous, false, nil
	}

	registryNamespace := artifact.Namespace
	if artifact.Spec.RegistryRef.Namespace != "" {
		registryNamespace = artifact.Spec.RegistryRef.Namespace
	}

	if registryNamespace != artifact.Namespace {
		granted, err := registryGranted(ctx, r.APIReader, registryNamespace, artifact.Namespace)
		if err != nil {
			return nil, false, fmt.Errorf("failed to check ReferenceGrant for Registry %s/%s: %w",
				registryNamespace, artifact.Spec.RegistryRef.Name, err)
		}
		if !granted {
			return nil, false, fmt.Errorf("no ReferenceGrant allows RenderArtifact %s/%s to access Registry %s/%s",
				artifact.Namespace, artifact.Name, registryNamespace, artifact.Spec.RegistryRef.Name)
		}
	}

	registry := &solarv1alpha1.Registry{}
	if err := r.APIReader.Get(ctx, client.ObjectKey{
		Name:      artifact.Spec.RegistryRef.Name,
		Namespace: registryNamespace,
	}, registry); err != nil {
		return nil, false, fmt.Errorf("failed to get Registry %s/%s: %w", registryNamespace, artifact.Spec.RegistryRef.Name, err)
	}

	if registry.Spec.SolarSecretRef == nil {
		return authn.Anonymous, registry.Spec.PlainHTTP, nil
	}

	secret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      registry.Spec.SolarSecretRef.Name,
		Namespace: registry.Namespace,
	}, secret); err != nil {
		log.Error(err, "Failed to get push secret for OCI auth",
			"secret", registry.Spec.SolarSecretRef.Name)

		return nil, false, fmt.Errorf("failed to get push secret %s/%s: %w", registry.Namespace, registry.Spec.SolarSecretRef.Name, err)
	}

	auth, err := ociAuthFromSecret(secret, registryHost)
	if err != nil {
		// A malformed dockerconfigjson is a configuration error; log it so the operator
		// is aware, but fall back to anonymous rather than blocking OCI cleanup.
		log.Error(err, "Malformed push secret; falling back to anonymous OCI auth",
			"secret", fmt.Sprintf("%s/%s", registry.Namespace, registry.Spec.SolarSecretRef.Name))
	}

	return auth, registry.Spec.PlainHTTP, nil
}
```

`ociAuthFromSecret` (immediately below, `pkg/controller/renderartifact_controller.go:277-311` in the original file) is unchanged.

- [ ] **Step 11: Rewrite the tests broken by the field removal**

In `pkg/controller/target_controller_test.go`, replace the `It("should propagate RegistryFlavor from the Registry to the RenderArtifact", ...)` test (`pkg/controller/target_controller_test.go:1025-1062`) with:

```go
		It("should set RegistryRef on the RenderArtifact and RenderBinding to the resolved Registry", func() {
			reg := newRegistry("test-registry-ref")
			Expect(k8sClient.Create(ctx, reg)).To(Succeed())

			cv := newComponentVersion("my-cv")
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())

			rel := newRelease("rel-registryref")
			Expect(k8sClient.Create(ctx, rel)).To(Succeed())

			target := newTarget("test-registryref-propagation")
			target.Spec.RenderRegistryRef.Name = "test-registry-ref"
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			Expect(k8sClient.Create(ctx, newReleaseBinding("rb-registryref", "test-registryref-propagation", "rel-registryref"))).To(Succeed())

			relRTName := releaseRenderTaskName(ns.Name, "rel-registryref", "test-registryref-propagation", 1)

			rt := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: relRTName, Namespace: ns.Name}, rt)
			}, eventuallyTimeout).Should(Succeed())

			actualBaseURL := rt.Spec.BaseURL
			actualRepo := rt.Spec.Repository
			actualTag := rt.Spec.Tag
			expectedArtName := renderArtifactName(ns.Name, actualBaseURL, actualRepo, actualTag)
			expectedBindingName := renderBindingName(expectedArtName, "test-registryref-propagation")

			markRenderTaskSucceeded(relRTName, "oci://"+actualBaseURL+"/"+actualRepo+":"+actualTag)

			expectedRef := &solarv1alpha1.ObjectReference{Name: "test-registry-ref"}

			Eventually(func(g Gomega) {
				art := &solarv1alpha1.RenderArtifact{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: expectedArtName, Namespace: ns.Name}, art)).To(Succeed())
				g.Expect(art.Spec.RegistryRef).To(Equal(expectedRef))
			}, eventuallyTimeout).Should(Succeed(), "RenderArtifact should carry the resolved RegistryRef")

			Eventually(func(g Gomega) {
				binding := &solarv1alpha1.RenderBinding{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: expectedBindingName, Namespace: ns.Name}, binding)).To(Succeed())
				g.Expect(binding.Spec.RegistryRef).To(Equal(expectedRef))
			}, eventuallyTimeout).Should(Succeed(), "RenderBinding should carry the resolved RegistryRef")
		})
```

In `pkg/controller/renderartifact_controller_test.go`, replace the whole `Context("resolveAuth: cross-namespace push secret", ...)` block with:

```go
	Context("resolveAuth: RegistryRef resolution", Label("renderartifact"), func() {
		It("should resolve credentials from a same-namespace Registry", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "same-ns-creds", Namespace: ns.Name},
				Type:       corev1.SecretTypeBasicAuth,
				Data: map[string][]byte{
					corev1.BasicAuthUsernameKey: []byte("user"),
					corev1.BasicAuthPasswordKey: []byte("pass"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			registry := &solarv1alpha1.Registry{
				ObjectMeta: metav1.ObjectMeta{Name: "same-ns-registry", Namespace: ns.Name},
				Spec: solarv1alpha1.RegistrySpec{
					Hostname:       "registry.example.com",
					SolarSecretRef: &corev1.LocalObjectReference{Name: "same-ns-creds"},
					PlainHTTP:      true,
				},
			}
			Expect(k8sClient.Create(ctx, registry)).To(Succeed())

			reconciler := &RenderArtifactReconciler{Client: k8sClient, APIReader: k8sClient}
			art := &solarv1alpha1.RenderArtifact{
				ObjectMeta: metav1.ObjectMeta{Name: "art-same-ns-auth", Namespace: ns.Name},
				Spec: solarv1alpha1.RenderArtifactSpec{
					BaseURL:     "registry.example.com",
					Repository:  "ns/myapp",
					Tag:         "v1.0.0",
					RegistryRef: &solarv1alpha1.ObjectReference{Name: "same-ns-registry"},
				},
			}

			auth, plainHTTP, err := reconciler.resolveAuth(ctx, art, art.Spec.BaseURL)
			Expect(err).NotTo(HaveOccurred())
			Expect(auth).NotTo(Equal(authn.Anonymous))
			Expect(plainHTTP).To(BeTrue())
		})

		It("should resolve credentials from a cross-namespace Registry when a ReferenceGrant permits it", func() {
			crossNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "cross-ns-"}}
			Expect(k8sClient.Create(ctx, crossNs)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, crossNs) })

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "cross-ns-creds", Namespace: crossNs.Name},
				Type:       corev1.SecretTypeBasicAuth,
				Data: map[string][]byte{
					corev1.BasicAuthUsernameKey: []byte("user"),
					corev1.BasicAuthPasswordKey: []byte("pass"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			registry := &solarv1alpha1.Registry{
				ObjectMeta: metav1.ObjectMeta{Name: "cross-ns-registry", Namespace: crossNs.Name},
				Spec: solarv1alpha1.RegistrySpec{
					Hostname:       "registry.example.com",
					SolarSecretRef: &corev1.LocalObjectReference{Name: "cross-ns-creds"},
				},
			}
			Expect(k8sClient.Create(ctx, registry)).To(Succeed())

			grant := &solarv1alpha1.ReferenceGrant{
				ObjectMeta: metav1.ObjectMeta{Name: "grant", Namespace: crossNs.Name},
				Spec: solarv1alpha1.ReferenceGrantSpec{
					From: []solarv1alpha1.ReferenceGrantFromSubject{
						{Group: solarGroup, Kind: "Target", Namespace: ns.Name},
					},
					To: []solarv1alpha1.ReferenceGrantToTarget{
						{Group: solarGroup, Kind: "Registry"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, grant)).To(Succeed())

			reconciler := &RenderArtifactReconciler{Client: k8sClient, APIReader: k8sClient}
			art := &solarv1alpha1.RenderArtifact{
				ObjectMeta: metav1.ObjectMeta{Name: "art-cross-ns-auth", Namespace: ns.Name},
				Spec: solarv1alpha1.RenderArtifactSpec{
					BaseURL:    "registry.example.com",
					Repository: "ns/myapp",
					Tag:        "v1.0.0",
					RegistryRef: &solarv1alpha1.ObjectReference{
						Name:      "cross-ns-registry",
						Namespace: crossNs.Name,
					},
				},
			}

			auth, _, err := reconciler.resolveAuth(ctx, art, art.Spec.BaseURL)
			Expect(err).NotTo(HaveOccurred())
			Expect(auth).NotTo(Equal(authn.Anonymous))
		})

		It("should fail when no ReferenceGrant permits the cross-namespace Registry", func() {
			crossNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "cross-ns-ungranted-"}}
			Expect(k8sClient.Create(ctx, crossNs)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, crossNs) })

			registry := &solarv1alpha1.Registry{
				ObjectMeta: metav1.ObjectMeta{Name: "ungranted-registry", Namespace: crossNs.Name},
				Spec: solarv1alpha1.RegistrySpec{
					Hostname:       "registry.example.com",
					SolarSecretRef: &corev1.LocalObjectReference{Name: "does-not-matter"},
				},
			}
			Expect(k8sClient.Create(ctx, registry)).To(Succeed())
			// No ReferenceGrant created in crossNs — access must be denied.

			reconciler := &RenderArtifactReconciler{Client: k8sClient, APIReader: k8sClient}
			art := &solarv1alpha1.RenderArtifact{
				ObjectMeta: metav1.ObjectMeta{Name: "art-ungranted-auth", Namespace: ns.Name},
				Spec: solarv1alpha1.RenderArtifactSpec{
					BaseURL:    "registry.example.com",
					Repository: "ns/myapp",
					Tag:        "v1.0.0",
					RegistryRef: &solarv1alpha1.ObjectReference{
						Name:      "ungranted-registry",
						Namespace: crossNs.Name,
					},
				},
			}

			_, _, err := reconciler.resolveAuth(ctx, art, art.Spec.BaseURL)
			Expect(err).To(HaveOccurred(), "cross-namespace Registry access without a ReferenceGrant must fail")
		})
	})
```

Replace the whole `Context("GC with PlainHTTP", ...)` block with:

```go
	Context("GC with PlainHTTP", Label("renderartifact"), func() {
		It("should pass Insecure=true to DeleteTag when the Registry has PlainHTTP set", func() {
			registry := &solarv1alpha1.Registry{
				ObjectMeta: metav1.ObjectMeta{Name: "plainhttp-registry", Namespace: ns.Name},
				Spec: solarv1alpha1.RegistrySpec{
					Hostname:  "registry.example.com",
					PlainHTTP: true,
				},
			}
			Expect(k8sClient.Create(ctx, registry)).To(Succeed())

			art := &solarv1alpha1.RenderArtifact{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "art-plainhttp",
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.RenderArtifactSpec{
					BaseURL:       "registry.example.com",
					Repository:    "ns/myapp",
					Tag:           "v1.0.0",
					RenderTaskRef: "rt-plainhttp",
					RegistryRef:   &solarv1alpha1.ObjectReference{Name: "plainhttp-registry"},
				},
			}
			Expect(k8sClient.Create(ctx, art)).To(Succeed())

			// No bindings → GC should delete the artifact and call DeleteTag.
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(art), &solarv1alpha1.RenderArtifact{})
				return apierrors.IsNotFound(err)
			}, eventuallyTimeout).Should(BeTrue())

			records := fakeTagDeleter.callsWithOpts()
			Expect(records).NotTo(BeEmpty())
			for _, rec := range records {
				if rec.rawRef == "registry.example.com/ns/myapp:v1.0.0" {
					Expect(rec.insecure).To(BeTrue(), "PlainHTTP Registry should delete with Insecure=true")
					return
				}
			}
			Fail("expected DeleteTag call for registry.example.com/ns/myapp:v1.0.0")
		})

		It("should pass Insecure=false to DeleteTag when the artifact has no RegistryRef", func() {
			art := newArtifact("art-secure-default")
			Expect(k8sClient.Create(ctx, art)).To(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(art), &solarv1alpha1.RenderArtifact{})
				return apierrors.IsNotFound(err)
			}, eventuallyTimeout).Should(BeTrue())

			records := fakeTagDeleter.callsWithOpts()
			Expect(records).NotTo(BeEmpty())
			for _, rec := range records {
				if rec.rawRef == "registry.example.com/ns/myapp:v1.0.0" {
					Expect(rec.insecure).To(BeFalse(), "default artifact should delete with Insecure=false")
					return
				}
			}
			Fail("expected DeleteTag call for registry.example.com/ns/myapp:v1.0.0")
		})
	})
```

- [ ] **Step 12: Regenerate RBAC and docs**

Run:
```bash
make manifests
make docs-crd-ref
```

- [ ] **Step 13: Verify the build and tests pass**

Run:
```bash
go build ./...
go test ./pkg/controller/... -run TestRegistryGranted
make test testargs="--label-filter=renderartifact,target"
```
Expected: build succeeds; all three pass, including every pre-existing test in both label groups (this task is a like-for-like replacement of the credential-resolution path, not a behavior change for single-consumer scenarios).

- [ ] **Step 14: Commit**

```bash
git add api/solar/v1alpha1/renderbinding_types.go api/solar/renderbinding_types.go \
  api/solar/v1alpha1/renderartifact_types.go api/solar/renderartifact_types.go \
  api/solar/v1alpha1/zz_generated.deepcopy.go api/solar/zz_generated.deepcopy.go \
  api/solar/v1alpha1/zz_generated.conversion.go client-go/ docs/user-guide/api-reference.md \
  pkg/controller/target_controller.go pkg/controller/target_controller_test.go \
  pkg/controller/target_controller_grants_test.go \
  pkg/controller/renderartifact_controller.go pkg/controller/renderartifact_controller_test.go
git commit -m "refactor(api): resolve RenderArtifact push credentials via RegistryRef instead of a Secret snapshot"
```

Also stage and include whatever RBAC chart file(s) `make manifests` updated (check `git status` for changes under the Helm chart's `files/` directory).

---

### Task 2: Re-pin `RenderArtifact.Spec.RegistryRef` from a surviving `RenderBinding`

**Files:**
- Modify: `pkg/controller/renderartifact_controller.go:129-157` (inside `Reconcile`), plus new functions
- Test: `pkg/controller/renderartifact_controller_test.go`

**Interfaces:**
- Consumes: `solarv1alpha1.RenderBindingSpec.RegistryRef` from Task 1, populated by Task 1's `ensureRenderBinding`.
- Produces: `(*RenderArtifactReconciler).repinCredentials(ctx context.Context, artifact *solarv1alpha1.RenderArtifact, bindings []solarv1alpha1.RenderBinding) error` and `registryRefEqual(a, b *solarv1alpha1.ObjectReference) bool` in `pkg/controller/renderartifact_controller.go`.

- [ ] **Step 1: Write the failing tests**

Add to `pkg/controller/renderartifact_controller_test.go`, as a new `Context` after the `Context("RegistryRef persistence", ...)` block added in Task 1 (still inside the outer `Describe("RenderArtifactController", ...)`):

```go
	Context("credential re-pinning", Label("renderartifact"), func() {
		newRegistryWithSecret := func(name, secretName string) (*solarv1alpha1.Registry, *corev1.Secret) {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: ns.Name},
				Type:       corev1.SecretTypeBasicAuth,
				Data: map[string][]byte{
					corev1.BasicAuthUsernameKey: []byte("user-" + secretName),
					corev1.BasicAuthPasswordKey: []byte("pass-" + secretName),
				},
			}
			registry := &solarv1alpha1.Registry{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns.Name},
				Spec: solarv1alpha1.RegistrySpec{
					Hostname:       "registry.example.com",
					SolarSecretRef: &corev1.LocalObjectReference{Name: secretName},
				},
			}
			return registry, secret
		}

		It("should re-pin RegistryRef to a surviving binding when the currently-pinned binding is removed", func() {
			registryA, secretA := newRegistryWithSecret("a-registry-repin", "a-secret-repin")
			Expect(k8sClient.Create(ctx, secretA)).To(Succeed())
			Expect(k8sClient.Create(ctx, registryA)).To(Succeed())

			registryB, secretB := newRegistryWithSecret("b-registry-repin", "b-secret-repin")
			Expect(k8sClient.Create(ctx, secretB)).To(Succeed())
			Expect(k8sClient.Create(ctx, registryB)).To(Succeed())

			art := newArtifact("art-repin")
			art.Spec.RegistryRef = &solarv1alpha1.ObjectReference{Name: "a-registry-repin"}
			Expect(k8sClient.Create(ctx, art)).To(Succeed())

			bindingA := newBinding("a-binding-repin", "art-repin")
			bindingA.Spec.RegistryRef = &solarv1alpha1.ObjectReference{Name: "a-registry-repin"}
			Expect(k8sClient.Create(ctx, bindingA)).To(Succeed())

			bindingB := newBinding("b-binding-repin", "art-repin")
			bindingB.Spec.RegistryRef = &solarv1alpha1.ObjectReference{Name: "b-registry-repin"}
			Expect(k8sClient.Create(ctx, bindingB)).To(Succeed())

			// Sanity: artifact starts pinned to registry A (alphabetically first binding).
			Eventually(func(g Gomega) {
				a := &solarv1alpha1.RenderArtifact{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(art), a)).To(Succeed())
				g.Expect(a.Spec.RegistryRef.Name).To(Equal("a-registry-repin"))
			}, eventuallyTimeout).Should(Succeed())

			// Simulate Target A (and its Registry/Secret) being torn down: remove its binding.
			Expect(k8sClient.Delete(ctx, bindingA)).To(Succeed())
			Expect(k8sClient.Delete(ctx, registryA)).To(Succeed())
			Expect(k8sClient.Delete(ctx, secretA)).To(Succeed())

			// The artifact must re-pin to the surviving binding's Registry.
			Eventually(func(g Gomega) {
				a := &solarv1alpha1.RenderArtifact{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(art), a)).To(Succeed())
				g.Expect(a.Spec.RegistryRef.Name).To(Equal("b-registry-repin"))
			}, eventuallyTimeout).Should(Succeed(), "RenderArtifact should re-pin to the surviving binding's Registry")
		})

		It("should GC successfully using the last surviving binding's Registry instead of getting stuck terminating", func() {
			DeferCleanup(fakeTagDeleter.reset)

			registryA, secretA := newRegistryWithSecret("a-registry-gc", "a-secret-gc")
			Expect(k8sClient.Create(ctx, secretA)).To(Succeed())
			Expect(k8sClient.Create(ctx, registryA)).To(Succeed())

			registryB, secretB := newRegistryWithSecret("b-registry-gc", "b-secret-gc")
			Expect(k8sClient.Create(ctx, secretB)).To(Succeed())
			Expect(k8sClient.Create(ctx, registryB)).To(Succeed())

			art := newArtifact("art-repin-gc")
			art.Spec.RegistryRef = &solarv1alpha1.ObjectReference{Name: "a-registry-gc"}
			Expect(k8sClient.Create(ctx, art)).To(Succeed())

			bindingA := newBinding("a-binding-gc", "art-repin-gc")
			bindingA.Spec.RegistryRef = &solarv1alpha1.ObjectReference{Name: "a-registry-gc"}
			Expect(k8sClient.Create(ctx, bindingA)).To(Succeed())

			bindingB := newBinding("b-binding-gc", "art-repin-gc")
			bindingB.Spec.RegistryRef = &solarv1alpha1.ObjectReference{Name: "b-registry-gc"}
			Expect(k8sClient.Create(ctx, bindingB)).To(Succeed())

			// Target A's Registry+Secret go away first, while Target B is still bound —
			// this is the exact scenario that used to leave the RenderArtifact stuck.
			Expect(k8sClient.Delete(ctx, bindingA)).To(Succeed())
			Expect(k8sClient.Delete(ctx, registryA)).To(Succeed())
			Expect(k8sClient.Delete(ctx, secretA)).To(Succeed())

			Eventually(func(g Gomega) {
				a := &solarv1alpha1.RenderArtifact{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(art), a)).To(Succeed())
				g.Expect(a.Spec.RegistryRef.Name).To(Equal("b-registry-gc"))
			}, eventuallyTimeout).Should(Succeed())

			// Now Target B's binding is removed too — this must GC cleanly, not get stuck.
			Expect(k8sClient.Delete(ctx, bindingB)).To(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(art), &solarv1alpha1.RenderArtifact{})
				return apierrors.IsNotFound(err)
			}, eventuallyTimeout).Should(BeTrue(), "RenderArtifact must be GC'd, not stuck terminating with a dangling RegistryRef")

			Expect(fakeTagDeleter.calls()).To(ContainElement("registry.example.com/ns/myapp:v1.0.0"))
		})
	})
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `make test testargs="--label-filter=renderartifact --focus='credential re-pinning'"`
Expected: FAIL — the first test times out waiting for `RegistryRef.Name` to become `"b-registry-repin"` (nothing re-pins it today, so it stays `"a-registry-repin"` forever). The second test times out waiting for the artifact to become `NotFound` (the last binding's removal triggers GC, but `resolveAuth` fails trying to reach the already-deleted `a-registry-gc`, so the finalizer never clears).

- [ ] **Step 3: Implement `repinCredentials` and `registryRefEqual`**

Add `"sort"` to the import block of `pkg/controller/renderartifact_controller.go` (alongside the existing `"slices"` import).

Add these two functions after `cleanupOCIArtifact`, before `resolveAuth`:

```go
// repinCredentials keeps artifact.Spec.RegistryRef pinned to the Registry snapshotted on
// a still-existing RenderBinding. Bindings are chosen deterministically by name so
// repeated reconciles converge instead of flapping between equally-valid choices.
// Because this runs on every RenderBinding create/update/delete event (see
// mapRenderBindingToArtifact), the artifact's pinned RegistryRef is always synced to a
// binding that exists — including immediately after the second-to-last binding is
// removed, which is exactly the moment that matters: it leaves the artifact holding a
// Registry reference that was valid for the binding that survives until the final
// removal, which is what the finalizer step needs to delete the OCI tag.
func (r *RenderArtifactReconciler) repinCredentials(ctx context.Context, artifact *solarv1alpha1.RenderArtifact, bindings []solarv1alpha1.RenderBinding) error {
	sort.Slice(bindings, func(i, j int) bool { return bindings[i].Name < bindings[j].Name })
	chosen := bindings[0]

	if registryRefEqual(artifact.Spec.RegistryRef, chosen.Spec.RegistryRef) {
		return nil
	}

	latest := artifact.DeepCopy()
	latest.Spec.RegistryRef = chosen.Spec.RegistryRef

	return r.Patch(ctx, latest, client.MergeFrom(artifact))
}

// registryRefEqual reports whether two (possibly nil) Registry references are equal.
func registryRefEqual(a, b *solarv1alpha1.ObjectReference) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}
```

- [ ] **Step 4: Wire `repinCredentials` into `Reconcile`**

In `pkg/controller/renderartifact_controller.go:129-157`, replace:

```go
	// List RenderBindings referencing this artifact.
	bindingList := &solarv1alpha1.RenderBindingList{}
	if err := r.List(ctx, bindingList,
		client.InNamespace(artifact.Namespace),
		client.MatchingFields{indexRenderBindingArtifactName: artifact.Name},
	); err != nil {
		return ctrl.Result{}, errLogAndWrap(log, err, "failed to list RenderBindings for RenderArtifact")
	}

	// If no bindings remain, trigger GC by deleting this object.
	// The finalizer above will intercept the deletion and handle OCI cleanup.
	if len(bindingList.Items) == 0 {
		// Confirm via direct API call — cache may lag on concurrent creates.
		confirmed := &solarv1alpha1.RenderBindingList{}
		if err := r.APIReader.List(ctx, confirmed, client.InNamespace(artifact.Namespace)); err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to confirm RenderBinding absence via API")
		}
		for i := range confirmed.Items {
			if confirmed.Items[i].Spec.RenderArtifactRef.Name == artifact.Name {
				// A binding exists in the API server that the cache missed.
				return ctrl.Result{}, nil
			}
		}
		log.V(1).Info("No RenderBindings remain for RenderArtifact — triggering GC",
			"artifact", artifact.Name)
		if err := r.Delete(ctx, artifact); client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to delete orphaned RenderArtifact")
		}
	}

	return ctrl.Result{}, nil
}
```

with:

```go
	// List RenderBindings referencing this artifact.
	bindingList := &solarv1alpha1.RenderBindingList{}
	if err := r.List(ctx, bindingList,
		client.InNamespace(artifact.Namespace),
		client.MatchingFields{indexRenderBindingArtifactName: artifact.Name},
	); err != nil {
		return ctrl.Result{}, errLogAndWrap(log, err, "failed to list RenderBindings for RenderArtifact")
	}

	if len(bindingList.Items) > 0 {
		// While at least one binding survives, keep the artifact's RegistryRef pinned to
		// a binding that still exists.
		if err := r.repinCredentials(ctx, artifact, bindingList.Items); err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to re-pin RenderArtifact credentials")
		}
	} else {
		// If no bindings remain, trigger GC by deleting this object.
		// The finalizer above will intercept the deletion and handle OCI cleanup.
		// Confirm via direct API call — cache may lag on concurrent creates.
		confirmed := &solarv1alpha1.RenderBindingList{}
		if err := r.APIReader.List(ctx, confirmed, client.InNamespace(artifact.Namespace)); err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to confirm RenderBinding absence via API")
		}
		for i := range confirmed.Items {
			if confirmed.Items[i].Spec.RenderArtifactRef.Name == artifact.Name {
				// A binding exists in the API server that the cache missed.
				return ctrl.Result{}, nil
			}
		}
		log.V(1).Info("No RenderBindings remain for RenderArtifact — triggering GC",
			"artifact", artifact.Name)
		if err := r.Delete(ctx, artifact); client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to delete orphaned RenderArtifact")
		}
	}

	return ctrl.Result{}, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `make test testargs="--label-filter=renderartifact --focus='credential re-pinning'"`
Expected: PASS.

- [ ] **Step 6: Run the full renderartifact and target suites to check for regressions**

Run: `make test testargs="--label-filter=renderartifact,target"`
Expected: PASS — in particular, the existing "GC: with RenderBindings" and "GC: no RenderBindings" tests must still pass, since `repinCredentials` only runs when `len(bindingList.Items) > 0` and is a no-op patch when the chosen binding's `RegistryRef` already matches the artifact's (true for every Task 1 test, since none of them set `RegistryRef` on their bindings — both sides are `nil` and compare equal).

- [ ] **Step 7: Commit**

```bash
git add pkg/controller/renderartifact_controller.go pkg/controller/renderartifact_controller_test.go
git commit -m "fix(controller): re-pin RenderArtifact RegistryRef from a surviving RenderBinding"
```

---

## Self-Review Notes

- **Spec coverage:** The AC ("A RenderArtifact has always access to the right credentials to delete the OCI tag ... within the finalizer step") is covered by Task 2's re-pin logic plus its direct regression test reproducing the exact reported scenario (Target 1 + Registry + Secret deleted while Target 2's binding survives, then Target 2's binding also removed). The user's explicit steer — "SolAr doesn't want to manage secrets herself" — is reflected in Task 1: neither `RenderArtifact` nor `RenderBinding` ever store a Secret name or namespace; both only ever reference a `Registry`, resolved live, exactly mirroring how `Target` already resolves its own push credentials. The user's follow-up correction — Registries may live in a different namespace than the `RenderBinding`/`RenderArtifact` referencing them — is handled by making `RegistryRef` an `ObjectReference` (not a same-namespace-only `LocalObjectReference`) and by adding the matching `ReferenceGrant` check in `resolveAuth`, reusing the exact grant relationship `Target` already relies on rather than inventing a new one.
- **Placeholder scan:** No TBDs; every step has literal code and exact file/line references gathered directly from the current source.
- **Type consistency:** `ensureRenderArtifact`/`ensureRenderBinding`'s new `registryRef solarv1alpha1.ObjectReference` parameter matches `target.Spec.RenderRegistryRef`'s type (a value, not a pointer — `Target.Spec.RenderRegistryRef` is a required field, never nil). `resolveAuth`'s new three-value return (`authn.Authenticator, bool, error`) is threaded through unchanged into `cleanupOCIArtifact`'s single call site. `repinCredentials`'s `bindings []solarv1alpha1.RenderBinding` matches `bindingList.Items`'s type. `registryRefEqual`'s signature matches `*solarv1alpha1.ObjectReference`, the type of both `RenderArtifactSpec.RegistryRef` and `RenderBindingSpec.RegistryRef`. `registryGranted`'s new `reader client.Reader` parameter is satisfied by both `r.Client` (used by `TargetReconciler`, a `client.Client`) and `r.APIReader` (used by `RenderArtifactReconciler`, already typed `client.Reader`).
