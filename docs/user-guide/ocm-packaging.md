# OCM Packaging Contract

SolAr lists and deploys applications that are packaged as
[OCM](https://ocm.software) components. An OCM component must satisfy the
following rules to be listed in a SolAr catalog and deployed from there.

## Summary

| # | Rule | Effect of a violation |
|---|------|-----------------------|
| 1 | The component must be published to an OCI registry. | The component is not listed. |
| 2 | The component must contain exactly one resource of type `helmChart`. | The component is not listed. |
| 3 | The Helm chart must resolve to a published OCI artifact, directly or through `globalAccess`. | The component is not listed. |
| 4 | A values template is optional and is identified by the label `opendefense.cloud/helm/values-for`. | Without the label, no values are templated. |

Currently, Helm is the only supported deployment mechanism. Plain manifests,
Kustomize or other packaging formats are not deployed yet.

## Requirements

### 1. Published to an OCI registry

SolAr finds components by scanning OCI registries. A component that only
exists in a CTF archive or another OCM repository type is not found.
Use the OCM v2 CLI to build and transfer it. For the constructor below,
first publish the chart to a reachable OCI registry:

```bash
ocm add component-version --repository ctf::./ctf --constructor component-constructor.yaml
ocm --config uploaders.yaml transfer cv \
  "ctf::./ctf//opendefense.cloud/ocm-demo:0.1.0" \
  "https://registry.example.com/my-components"
```

Save the following transfer configuration as `uploaders.yaml`. Keep the
OCI uploader first so images and charts are published as separate artifacts;
the local blob uploader handles the remaining resources.

```yaml
type: generic.config.ocm.software/v1
configurations:
  - type: oci.uploader.transfer.config.ocm.software/v1alpha1
  - type: localblob.uploader.transfer.config.ocm.software/v1alpha1
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

- `OCIImage/v1` (also accepted as `ociArtifact`) — an absolute OCI
  reference, for example `registry.example.com/charts/demo:0.1.0`.
- `localBlob` with an OCI `globalAccess` pointing at a published artifact.

The old `relativeOciReference` access type is not supported. A local blob
containing OCI content without `globalAccess` is rejected because the
cluster cannot pull it as a published chart.

The OCI uploader derives a resource's `referenceName` from its OCI source
reference. A chart added only with `input: helm` has no `referenceName`
and is not published as a separate artifact by this configuration. Push the
chart first and reference it as shown in the example below.

To inspect the published descriptor and its resource access entries:

```bash
ocm get cv "https://registry.example.com/my-components//opendefense.cloud/ocm-demo:0.1.0" -o yaml
```

### 4. Optional values template

A component can ship a Helm values template that sets the chart's values,
for example image references, from the component's resources. The template
is a resource labeled
`opendefense.cloud/helm/values-for: <helm-chart-resource-name>`, see
[Helm values templating](helm-values-templating.md).

Image references set this way follow the image resources when the component
is transferred to another registry. This only takes effect if the images are
copied along using the uploader configuration above; otherwise the
image resources keep pointing at their original registry.

## Other resources

Besides the Helm chart and the values template, a component typically
contains the images the chart deploys, as resources of type `ociImage`.
OCI image resources and local blobs with an OCI `globalAccess` are available
in the values template's `.OCIResources` map. Local blobs containing no OCI
content, such as the values template itself, are skipped. OCI content held
as a local blob without `globalAccess` causes discovery or rendering to fail.

When a resource's repository path is within the component repository's
namespace, SolAr uses the discovery registry's host. References outside
that namespace retain their original host.

## Component references

Component references (`componentReferences`) are currently ignored.
Referenced components are not listed or deployed together with the
referencing component.

## Helm tests

The chart's test hooks run automatically after every install and upgrade.
A failing test counts as a failed deployment.

## The solar-agent component

The agent is packaged as the component `opendefense.cloud/solar-agent` with the
resources `solar-agent-chart` (`helmChart`), `solar-agent-image` (`ociImage`)
and `helm-values-template` (`yaml`). The released component is built and
published from the `ocm-components` repository, like every other catalog
component.

For dev clusters and E2E tests, `make ocm-agent-component` builds a local copy
into `.ocm/solar-agent-ctf` from `test/fixtures/solar-agent/`. Override
`AGENT_VERSION`, `AGENT_CHART_REF` and `AGENT_IMAGE_REF` to point it at a
locally pushed chart and image. The local component is not published anywhere.

For how to set the agent up with an operator, see [Setting up the
solar-agent](../operator-manual/solar-agent.md).

## Example

A minimal constructor using the chart published above:

```yaml
# component-constructor.yaml
components:
  - name: opendefense.cloud/ocm-demo
    version: 0.1.0
    provider:
      name: opendefense.cloud
    resources:
      # Rule 2: exactly one helmChart.
      # Rule 3: reference a published chart so the uploader can copy it.
      - name: demo-chart
        type: helmChart
        version: 0.1.0
        relation: external
        access:
          type: OCIImage/v1
          imageReference: registry.example.com/charts/demo:0.1.0

      # Image deployed by the chart, exposed to the values template.
      - name: nginx-image
        type: ociImage
        version: "1.28.3"
        relation: external
        access:
          type: OCIImage/v1
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
          mediaType: text/plain
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
