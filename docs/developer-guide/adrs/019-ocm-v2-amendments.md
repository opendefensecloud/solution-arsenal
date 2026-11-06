---
status: accepted
date: 2026-11-03
---

# OCM v2 Amendments to Component Dependencies and Catalog Chaining

## Context

The OCM v2 migration changes the descriptor types, resource access handling,
and transfer configuration used by SolAr. Some implementation details in
[ADR 011 — Component Dependencies](011-component-dependencies.md) and
[ADR 013 — Solar Catalog Chaining via ARC](013-catalog-chaining.md) describe
the earlier implementation.

This amendment records the current implementation while preserving those
ADRs as historical records. It does not reopen the rejected dependency
proposal in ADR 011 or change the catalog-chaining decisions in ADR 013.
The proposed pull-side changes in [ADR 017](017-catalog-chaining-catalog-index.md)
remain a separate decision.

### ADR 013: Resource References After Transfer

The "OCI URL Re-Mapping" section needs the following qualifications:

- OCM transfers must publish images and charts as pullable OCI artifacts.
  With the v2 CLI, the `oci.uploader` transfer configuration performs that
  publication. A component-local blob containing OCI content without a
  `globalAccess` is rejected by SolAr's resource resolver. Non-OCI local
  blobs, such as values templates, are omitted from the catalog's resource
  map and remain available to the renderer within the component.
- SolAr resolves OCI image access (including the `ociArtifact` alias) and
  local blobs with an OCI `globalAccess`. The old `relativeOciReference`
  access type is not supported by the current resolver, so configuring
  relative paths is not an alternative supported by this implementation.
- Discovery and values rendering share SolAr's resource-reference resolver.
  If a resource's repository path is within the component repository's
  namespace, SolAr uses the discovery registry's host. This allows artifacts
  transferred through a port-forward to be read through the registry address
  reachable by the cluster. References outside that namespace retain their
  original host. At the registry root, without a namespace, SolAr retains
  the recorded host.
- Helm values templates are rendered by the release renderer using
  `ocm-kit`, with the resolved resource references and the target's pull
  secret mapping. Discovery records the resources and chart metadata;
  it does not render the values template.

ComponentVersions continue to be derived in the destination
environment. These corrections do not change ADR 013's separation between
package transfer and destination discovery.

## Consequences

The historical ADRs retain their original status and text, with links to
this amendment. Package authors should use the current
[Helm values templating guide](../../user-guide/helm-values-templating.md)
for OCM v2 uploader configuration and rendering examples.

## Implementation References

- [Discovery event types](../../../pkg/discovery/events.go)
- [Shared resource-reference resolver](../../../pkg/ocmv2/ociref.go)
- [Discovery API writer](../../../pkg/discovery/apiwriter/apiwriter.go)
- [Release values renderer](../../../pkg/renderer/values_template.go)
- [OCM v2 uploader configuration](../../../test/fixtures/ocm-uploaders.yaml)
