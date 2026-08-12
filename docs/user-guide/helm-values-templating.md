# Helm Values Templating

SolAr supports an optional **values template** that ships alongside a Helm
chart inside an OCM component. The template is rendered once per `Target`,
against the component's own resources and that target's registry pull
secrets, and handed to the release as a ConfigMap.

This page covers both perspectives:

- **OCM package authors** — how to add a values template to a component
  descriptor so the chart works no matter which registry it lives in.
- **SolAr operators and catalog consumers** — what SolAr does with the
  rendered template and how it interacts with `Release.spec.values`.

The templating logic is provided by the
[`helmvalues`](https://github.com/opendefensecloud/ocm-kit/tree/main/helmvalues)
package of the [`ocm-kit`](https://github.com/opendefensecloud/ocm-kit)
library.

## Why this exists

Two problems, one mechanism.

**Registry portability.** When an OCM package is transferred between OCI
registries (for example, mirrored from a build registry into a
customer-controlled registry), the Helm chart inside the component still
carries its original, registry-specific image references in `values.yaml`.
The Helm release would then pull images from the wrong place — or fail
outright in an air-gapped environment.

**Pull secrets.** Even pointing at the right registry is not enough if
that registry is private. The Pods the chart creates need
`imagePullSecrets`, and the name of that Secret is a property of the
_target cluster_, not of the package.

The values template solves both by being rendered against the OCM
component **as SolAr sees it**, for **the target it is being deployed
to**. Image references point at the registry the component currently
lives in, and pull secret names come from the target's
`RegistryBinding`s.

## How it fits together

1. The package author adds a YAML resource to the component descriptor
   carrying the label `opendefense.cloud/helm/values-for:
<chart-resource-name>`. The resource content is the values template.
2. During discovery, SolAr records the component and its resources.
3. When a `Release` is scheduled onto a `Target`, the target controller
   resolves the target's `RegistryBinding`s into a registry-host to
   pull-secret-name mapping, and records the component's OCM reference on
   the resulting `RenderTask`.
4. The renderer fetches the component from its source registry, renders
   the values template with those pull secrets in scope, and emits a
   sibling `ConfigMap` plus a `valuesFrom` entry on the generated
   `HelmRelease`. Any inline `Release.spec.values` is layered on top by
   Flux.

The template is optional. If a component has no labeled values template,
rendering proceeds normally and SolAr emits the `HelmRelease` without the
extra ConfigMap.

## Authoring a values template

### The label

The values template is identified by a label on an OCM resource:

```yaml
labels:
  - name: opendefense.cloud/helm/values-for
    value: <helm-chart-resource-name>
```

The label value must match the `name:` of the Helm chart resource in the
same component descriptor.

### Template syntax

Templates use Go's `text/template` syntax with the following extensions:

- All [sprig](https://masterminds.github.io/sprig/) functions except
  `env` and `expandenv` (these are disabled for safety).
- `toJSON` — marshal any value to a JSON string.
- `parseRef` — parse an OCI image reference into its components.
- `pullSecretFor` — resolve an image reference to the name of the pull
  secret on the target cluster. See
  [Pull secrets](#pull-secrets-imagepullsecrets) below.

### Available data

The template receives a `RenderingInput` value as the dot context.

#### `.OCIResources`

A map of OCI-backed resources in the component, keyed by the resource's
`name:` from the component descriptor. Only resources whose access
method is `ociArtifact` or `relativeOciReference` are included. Each
value is an `ImageReference` with these fields:

- `.Host` — the registry host (may include port).
- `.Repository` — the repository path.
- `.Tag` — the image tag.
- `.Digest` — the image digest, when available.

`ociArtifact` resources keep their original absolute reference.
`relativeOciReference` resources are resolved against the registry SolAr
discovered the component in.

#### `.Component`

The OCM `ComponentSpec` describing the component itself — name, version,
provider, resource list, sources, and references. Useful when the
template needs to expose metadata other than image references.

#### `.PullSecrets`

The raw registry-host to secret-name mapping. Prefer the `pullSecretFor`
function over reaching into this map directly, since the function handles
reference parsing and fallback.

### Worked example

The component descriptor below packages a Helm chart together with three
OCI images and a values template that rewires the chart's image
references to point at whichever registry the component lives in. This
matches the
[ARC fixture](https://github.com/opendefensecloud/ocm-kit/tree/main/test/fixtures/arc)
used by ocm-kit's tests.

```yaml
# component-constructor.yaml
components:
  - name: opendefense.cloud/arc
    provider:
      name: opendefense.cloud
    resources:
      - name: helm-chart
        type: helmChart
        version: v0.2.0
        relation: external
        access:
          type: ociArtifact
          imageReference: ghcr.io/opendefensecloud/charts/arc:0.1.4

      - name: arc-apiserver-image
        type: ociImage
        version: v0.2.0
        relation: external
        access:
          type: ociArtifact
          imageReference: ghcr.io/opendefensecloud/arc-apiserver:v0.2.0

      - name: arc-controller-manager-image
        type: ociImage
        version: v0.2.0
        relation: external
        access:
          type: ociArtifact
          imageReference: ghcr.io/opendefensecloud/arc-controller-manager:v0.2.0

      - name: etcd-image
        type: ociImage
        version: v3.6.6
        relation: external
        access:
          type: ociArtifact
          imageReference: quay.io/coreos/etcd:v3.6.6

      - name: helm-values-template
        type: yaml
        labels:
          - name: opendefense.cloud/helm/values-for
            value: helm-chart
        relation: local
        input:
          type: file
          path: values.yaml.tpl
```

The template:

```yaml
# values.yaml.tpl
apiserver:
  image:
    {{- $apiserver := index .OCIResources "arc-apiserver-image" }}
    repository: {{ $apiserver.Host }}/{{ $apiserver.Repository }}
    tag: {{ $apiserver.Tag }}

controller:
  image:
    {{- $controller := index .OCIResources "arc-controller-manager-image" }}
    repository: {{ $controller.Host }}/{{ $controller.Repository }}
    tag: {{ $controller.Tag }}

etcd:
  image:
    {{- $etcdImage := index .OCIResources "etcd-image" }}
    repository: {{ $etcdImage.Host }}/{{ $etcdImage.Repository }}
    tag: {{ $etcdImage.Tag }}
```

After SolAr renders this for a target, with the component discovered from
`registry.example.com/mirror`, the values handed to the chart look like:

```yaml
apiserver:
  image:
    repository: registry.example.com/mirror/opendefensecloud/arc-apiserver
    tag: v0.2.0

controller:
  image:
    repository: registry.example.com/mirror/opendefensecloud/arc-controller-manager
    tag: v0.2.0

etcd:
  image:
    repository: registry.example.com/mirror/coreos/etcd
    tag: v3.6.6
```

## Pull secrets (`imagePullSecrets`)

`pullSecretFor` takes an image reference and returns the name of the
Secret on the target cluster that holds credentials for that registry, or
an empty string if the target has no binding for it.

```yaml
{{- $apiserver := index .OCIResources "arc-apiserver-image" }}

apiserver:
  image:
    repository: {{ $apiserver.Host }}/{{ $apiserver.Repository }}
    tag: {{ $apiserver.Tag }}

{{- with pullSecretFor (printf "%s/%s" $apiserver.Host $apiserver.Repository) }}
imagePullSecrets:
  - name: {{ . }}
{{- end }}
```

Where the name comes from: for each `RegistryBinding` on the target,
SolAr reads the bound `Registry` and maps its `spec.hostname` to its
`spec.targetPullSecretName`. SolAr never reads the Secret itself — the
cluster maintainer must provision a Secret with that name on each target.

Lookup walks the reference from most specific to least specific
(`host/org/repo` → `host/org` → `host`), so passing either a full image
reference or a bare host resolves to the same entry. SolAr populates the
mapping at host granularity, because `Registry.spec.hostname` is
host-scoped.

!!! warning "Always guard the output"
A host with no matching `RegistryBinding` yields an empty string. Use
`{{- with pullSecretFor ... }}` as above so the `imagePullSecrets`
block is omitted entirely rather than emitting `name: ""`, which
Kubernetes rejects.

If the controller runs with strict registry binding enabled, a resource
whose host has no `RegistryBinding` fails the release before a
`RenderTask` is ever created, rather than rendering an empty name.

## Previewing locally

The `ocm-kit` CLI renders a values template against a real OCM component
without going through SolAr — useful while iterating on a template before
publishing the component.

```bash
# Render the template embedded in a published component
ocm-kit "oci://localhost:5000/my-components//opendefense.cloud/arc:0.1.0" \
  --chart-resource helm-chart

# Render a local template file against a published component, leaving
# the component untouched
ocm-kit "oci://localhost:5000/my-components//opendefense.cloud/arc:0.1.0" \
  --local-helm-values-template ./values.yaml.tpl

# Supply pull secret mappings so pullSecretFor resolves
ocm-kit "oci://localhost:5000/my-components//opendefense.cloud/arc:0.1.0" \
  --chart-resource helm-chart \
  --pull-secrets-file ./pull-secrets.json
```

The pull secrets file mirrors what SolAr derives from `RegistryBinding`s:

```json
{
  "pullSecrets": [
    { "registry": "registry.example.com", "secretName": "regcred" },
    { "registry": "quay.io", "secretName": "quay-pull" }
  ]
}
```

Registry credentials come from `~/.ocmconfig` (or `$OCM_CONFIG`) and fall
back to the docker config at `$DOCKER_CONFIG` or `~/.docker/config.json`.
Registries matching neither are accessed anonymously.

## What SolAr does with the rendered template

When a `Release` referencing a `ComponentVersion` is rendered for a
target, SolAr generates:

- A `ConfigMap` named `<release>-values` containing the rendered
  template under `values.yaml`.
- A `HelmRelease` with `valuesFrom` pointing at that ConfigMap and any
  inline `Release.spec.values` rendered into `HelmRelease.spec.values`.

Flux applies `valuesFrom` first and inline `values` last, so values
supplied through `Release.spec.values` override the rendered template.
This makes the template a safe default — operators can still override
individual fields per release without coordinating with the package
author.

## Caveats

- **Rendering happens at render time.** The `.OCIResources` map is
  built from the component as the renderer fetches it, and
  `.PullSecrets` from the target's `RegistryBinding`s. Changing a
  `RegistryBinding` produces a new chart on the next reconcile.
  Absolute `ociArtifact` references retain their original host;
  `relativeOciReference`s resolve to the discovery registry.
- **The renderer needs read access to the source registry.** It resolves
  the component from wherever SolAr discovered it, using credentials from
  that `Registry`'s `solarSecretRef`. Both shapes are supported: a Secret
  carrying `username`/`password` keys, or a
  `kubernetes.io/dockerconfigjson`. A registry with no matching `Registry`
  object is read anonymously.
- **A failed render fails one `RenderTask`** Parse errors
  and template errors surface as a failed render job for the affected
  target. Validate templates with the `ocm-kit` CLI before
  publishing.
- **Rendered output is validated as YAML.** A template that produces
  invalid YAML fails the render with a clear error, rather than surfacing
  later when Flux tries to consume the ConfigMap.

## See also

- [Discovery](discovery.md) — how SolAr scans registries for OCM
  components.
- [API reference](api-reference.md) — schema for
  `ComponentVersion`, `Release`, and related resources.
- [`ocm-kit`](https://github.com/opendefensecloud/ocm-kit)
  — the upstream library and CLI.
