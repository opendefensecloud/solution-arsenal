# OCM Packaging Contract

SolAr lists and deploys applications that are packaged as
[OCM](https://ocm.software) components. An OCM component must satisfy the
following rules to be listed in a SolAr catalog and deployed from there.

## Summary

| # | Rule | Effect of a violation |
|---|------|-----------------------|
| 1 | The component must be published to an OCI registry. | The component is not listed. |
| 2 | The component must contain exactly one resource of type `helmChart`. | The component is not listed. |
| 3 | The Helm chart's access type must be `ociArtifact` or `relativeOciReference`, not `localBlob`. | The component is not listed. |
| 4 | A values template is optional and is identified by the label `opendefense.cloud/helm/values-for`. | Without the label, no values are templated. |

Currently, Helm is the only supported deployment mechanism. Plain manifests,
Kustomize or other packaging formats are not deployed yet.

## Requirements

### 1. Published to an OCI registry

SolAr finds components by scanning OCI registries. A component that only
exists in a CTF archive or another OCM repository type is not found.
Publish it with `ocm transfer`, for example:

```bash
ocm add componentversion --version 0.1.0 --create --file ./ctf component-constructor.yaml
ocm transfer ctf --copy-local-resources ./ctf registry.example.com/my-components
```

### 2. Exactly one Helm chart

A component with zero or more than one resource of type `helmChart` is not
listed. The resource type must be exactly `helmChart`; the resource name is
free.

To ship several charts, either wrap them in one umbrella chart or package
them as separate components.

The catalog shows the `name`, `description`, `version` and `appVersion`
fields of the chart's `Chart.yaml`.

### 3. Access type of the Helm chart

The Helm chart must be addressable as an OCI artifact, because the cluster
it is deployed to pulls it directly from the registry. Its access type in the
published component descriptor must be one of:

- `ociArtifact` — an absolute OCI reference, for example
  `ghcr.io/example/charts/demo:0.1.0`.
- `relativeOciReference` — a reference relative to the registry the
  component lives in.

A Helm chart with any other access type, in particular `localBlob`, is not
accepted.

To check the access type after the transfer:

```console
$ ocm get resources registry.example.com/my-components//opendefense.cloud/ocm-demo:0.1.0 -o wide
NAME                 VERSION IDENTITY TYPE      RELATION ACCESSTYPE  ACCESSSPEC
demo-chart           0.1.0            helmChart local    ociArtifact {"imageReference":"registry.example.com/..."}
helm-values-template 0.1.0            yaml      local    localBlob   {"localReference":"sha256:...", ...}
nginx-image          1.28.3           ociImage  external ociArtifact {"imageReference":"ghcr.io/linuxserver/nginx:1.28.3"}
```

### 4. Optional values template

A component can ship a Helm values template that sets the chart's values,
for example image references, from the component's resources. The template
is a resource labeled
`opendefense.cloud/helm/values-for: <helm-chart-resource-name>`, see
[Helm values templating](helm-values-templating.md).

Image references set this way follow the image resources when the component
is transferred to another registry. This only takes effect if the images are
copied along, for example with `ocm transfer --copy-resources`; otherwise the
image resources keep pointing at their original registry.

## Other resources

Besides the Helm chart and the values template, a component typically
contains the images the chart deploys, as resources of type `ociImage`.
Only resources with access type `ociArtifact` or `relativeOciReference` are
available to the values template.

## Component references

Component references (`componentReferences`) are currently ignored.
Referenced components are not listed or deployed together with the
referencing component.

## Helm tests

The chart's test hooks run automatically after every install and upgrade.
A failing test counts as a failed deployment.

## Example

A minimal component that satisfies the contract is
[`ocm-demo`](https://github.com/opendefensecloud/ocm-components/tree/main/ocm-demo):

```yaml
# component-constructor.yaml
components:
  - name: opendefense.cloud/ocm-demo
    provider:
      name: opendefense.cloud
    resources:
      # Rule 2: exactly one helmChart.
      # Rule 3: stored as an OCI artifact by `ocm transfer`.
      - name: demo-chart
        type: helmChart
        relation: local
        input:
          type: helm
          path: ./charts/demo

      # Image deployed by the chart, exposed to the values template.
      - name: nginx-image
        type: ociImage
        version: "1.28.3"
        relation: external
        access:
          type: ociArtifact
          imageReference: ghcr.io/linuxserver/nginx:1.28.3

      # Rule 4: optional values template for demo-chart.
      - name: helm-values-template
        type: yaml
        labels:
          - name: opendefense.cloud/helm/values-for
            value: demo-chart
        relation: local
        input:
          type: file
          path: values.yaml.tpl
```

```yaml
# values.yaml.tpl
{{- $nginx := index .OCIResources "nginx-image" }}
image:
  repository: {{ $nginx.Host }}/{{ $nginx.Repository }}
  tag: {{ $nginx.Tag }}
```

A larger example with several images is the
[worked example](helm-values-templating.md#worked-example) in the Helm values
templating guide.
