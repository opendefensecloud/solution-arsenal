# Upgrading

## Helm

See [Helm installation](./installation/helm.md#upgrading) for more information.

## From v0.4.0 or earlier

Release charts are now pinned by digest (see [Rendering Pipeline](../developer-guide/rendering-pipeline.md#stage-1-release-rendertasks)). This has two effects during the upgrade:

- **Every Release is rendered again.** The release chart tag now includes a hash of the component reference and the resource references (repository, tag, insecure flag), so the tag changes for every Release, pinned or not. Each release chart and each Target's bootstrap chart is rendered and pushed once more, and Flux upgrades the corresponding HelmReleases. For a ComponentVersion that did not change, the rendered content is the same apart from the chart tag, so the workloads themselves are not upgraded. Expect a burst of render Jobs and registry pushes right after the controller restarts.
- **Releases whose descriptor tag was re-pushed stop rendering.** If a component descriptor's tag was overwritten after discovery, the render now fails with "the tag was overwritten after discovery" instead of quietly using the new content. The Target keeps its previously deployed bootstrap until all of its release renders succeed, so this also holds back updates to the Target's other Releases; its `ReleasesRendered` condition stays `False` with "Waiting for release RenderTasks to complete". Restore the original tag, or publish the new content as a new component version and point the Release at it. Deleting the ComponentVersion does not help: while a Release references it, its deletion stays blocked.

Only newly discovered ComponentVersions get pinned resources. Existing ones keep their tag-based resource references, because the registry scan skips ComponentVersions that already exist. They get pinned only if a registry webhook reports the same descriptor again. To pin a deployed component by hand, publish it as a new component version and point its Releases at it.
