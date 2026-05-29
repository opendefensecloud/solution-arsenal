# Helm Values Templating

SolAr supports an optional **values template** that ships alongside a Helm
chart inside an OCM component. The template is rendered at discovery time
against the component's own resources and made available to releases as a
ConfigMap.

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

When an OCM package is transferred between OCI registries (for
example, mirrored from a build registry into a customer-controlled
registry), the Helm chart inside the component still carries its
original, registry-specific image references in `values.yaml`. The Helm
release would then pull images from the wrong place — or fail outright in
an air-gapped environment.

The values template solves this by being rendered against the OCM
component **as SolAr sees it**. Image references in the rendered output
point at the registry the component currently lives in, so the same OCM
component can be transferred between registries without rebuilding the
chart.

## How it fits together

1. The package author adds a YAML resource to the component descriptor
   carrying the label `opendefense.cloud/helm/values-for:
   <chart-resource-name>`. The resource content is the values template.
2. During discovery, SolAr's Helm handler looks up the template resource
   for each `helmChart` it discovers, builds a `RenderingInput` from the
   component's other resources, and renders the template.
3. The rendered string is stored on the resulting `ComponentVersion` at
   `spec.resources.<chart>.helm.valuesTemplate`.
4. When a `Release` for that `ComponentVersion` is rendered, SolAr emits
   a sibling `ConfigMap` containing the rendered values and adds a
   `valuesFrom` entry to the generated `HelmRelease`. Any inline
   `Release.spec.values` is layered on top of that ConfigMap by Flux.

The template is optional. If a component has no labeled values template,
discovery proceeds normally and SolAr emits the `HelmRelease` without
the extra ConfigMap.

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

Rendering runs with `missingkey=error`, so referencing an undefined map
key fails the render rather than producing an empty string. Plan for
this when designing templates: spell every key exactly.

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

After SolAr discovers this component from, say,
`registry.example.com/mirror`, the rendered template stored on the
`ComponentVersion` looks like this and will point to the moved images,
so that the helm values can be picked up as additional helm values by `Releases` to pull images from
the correct OCI registry:

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

## Previewing locally

The `ocm-kit` CLI renders a values template against a real OCM component
without going through SolAr — useful while iterating on a template
before publishing the component.

```bash
# Render the template embedded in a published component
ocm-kit "oci://localhost:5000/my-components//opendefense.cloud/arc:0.1.0" \
  --chart-resource helm-chart

# Render a local template file against a published component, leaving
# the component untouched
ocm-kit "oci://localhost:5000/my-components//opendefense.cloud/arc:0.1.0" \
  --local-helm-values-template ./values.yaml.tpl
```

Registry credentials come from `~/.ocmconfig`. See the
[ocm-kit README](https://github.com/opendefensecloud/ocm-kit#registry-credentials)
for the full credential setup.

## What SolAr does with the rendered template

The rendered template is stored on the `ComponentVersion`:

```yaml
apiVersion: solar.opendefense.cloud/v1alpha1
kind: ComponentVersion
spec:
  resources:
    helm-chart:
      helm:
        valuesTemplate: |
          apiserver:
            image:
              repository: ghcr.io/opendefensecloud/arc-apiserver
              tag: v0.2.0
          # ...
```

See the
[API reference for `HelmResourceMetadata`](api-reference.md#helmresourcemetadata)
for the surrounding schema.

When a `Release` referencing this `ComponentVersion` is rendered, SolAr
generates:

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

- **Rendering happens at discovery time.** The `.OCIResources` map is
  built from the component as SolAr discovered it. Absolute `ociArtifact`
  references retain their original host; `relativeOciReference`s resolve
  to the discovery registry. 
- **A failed render fails discovery for that component.** Parse errors,
  references to undefined keys, and other template errors propagate up
  and prevent the `ComponentVersion` from being written. Validate
  templates with the `ocm-kit` CLI before publishing.
- **YAML validity is the author's responsibility.** SolAr renders
  templates without YAML validation. A template that produces invalid
  YAML will only fail later, when the `ConfigMap` is consumed by the
  `HelmRelease`.

## See also

- [Discovery](discovery.md) — how SolAr scans registries for OCM
  components.
- [API reference](api-reference.md) — schema for
  `ComponentVersion`, `Release`, and related resources.
- [`ocm-kit`](https://github.com/opendefensecloud/ocm-kit) — the
  upstream library and CLI.
