# solution-arsenal
Solution Arsenal (SolAr) is an application catalog based on Open Component Model packages (ocm.software) and fleet rollout managemnt for these solutions onto Kubernetes Clusters.
It features a catalog of solutions, which are application bundles provided as OCM packages from an OCI compliant registry. Additionally Kubernetes clusters can be registered with SolAr to turn them into deployment targets for the solutions from the catalog. The deployment itself then uses OCM Controllers with fluxCD as a deployer (<https://ocm.software/docs/concepts/ocm-controllers/>).

SolAr itself has several components:

* `solar-index` contains the catalog data and target cluster registrations and their desired state. The component is implemented as extension apiserver.
* `solar-discovery` continuously scans an OCI registry for relevant OCM packages, that are marked to be exposed to a catalog. It updates the `solar-index` accordingly.
* `solar-ui` is a management UI, which allows users to interact with `solar-index` to explore the catalog and manage deployments.
* `solar-renderer` is a component watching the `solar-index` for desired state updates to render and update the relevant OCI images containing the deployment manifests.
* `solar-agent` runs in registered clusters to update the cluster status in `solar-index`.


The backend is implemented as an API Extension Server to Kubernetes. The starting point is the kubernetes Sample API Server <https://github.com/kubernetes/sample-apiserver>

## "Use-Case Walkthrough"

### Deployment

An OCM package is imported into an environment via ARC and stored in an OCI Registry.

The OCI registry is scanned by `solar-discovery` and a corresponding `CatalogItem` or `ClusterCatalogItem` is created (via K8s API in `solar-index` extension apiserver).

A user is onboarded and gets underlying permissions to manage `CatalogItem`, `ClusterRegistration` and `Release` in a particular namespace (tenant separtion based on namespaces).

When the user interacts with the UI the same underlying permissions are used to manage the aforementioned K8s objects. The user creates a `ClusterRegistration` via the UI and retrieves a corresponding agent configuration. The agent configuration contains credentials that are required to access the `solar-index` APIs relevant to the cluster within the tenant boundaries and access credentials for the source OCI registry including the desired state OCI image URL (_do we need deeper integration with the source registry?_).

The user manually deploys the `solar-agent` to the target cluster with the retrieved agent configuration. The `solar-agent` does preflight checks and then creates `fluxcd` resources to reconcile the desired state from for this cluster using "Gitless GitOps" from the source OCI registry. The cluster status is updated and the user can now draft releases to deploy catalog items to the cluster.

The user creates a `Release` via the UI, the corresponding object causes the `solar-renderer` to update the desired state for this cluster in the OCI registry and a few moments later `fluxcd` instatiates the desired package. The `solar-agent` tracks the changes and reports the status changes back.

### Catalog Chaining

The user is onboarded and registers a cluster.

The `solar-agent` is configured to allow syncs and an ARC endpoint as destination is specified.
The `solar-agent` now also watches `Sync`-Resources in the catalog cluster. For each Sync resource an ARC `Order` is created/updated to trigger workflows pulling/scanning/pushing the packages to the destination.

The destination OCI is part of a second SolAr setup and the `solar-discovery` of the second SolAr setup picks up the packages and makes them available in the second environment.

