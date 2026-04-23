# User Stories

## Catalog & Discovery

As a **managed K8s provider**, I want to automatically discover OCM packages from OCI registries so that my catalog stays up-to-date without manual intervention.

As a **K8s user**, I want to browse a catalog of pre-packaged applications so that I can reduce the work necessary to deploy common components.

As a **solution maintainer**, I want to publish OCM packages to a registry and have them appear in the SolAr catalog so that consumers can find and deploy my software.

As a **solution maintainer**, I want to control which groups have permission to use my solutions so that only authorized teams can deploy them.

## Cluster Registration & Targets

As a **cluster maintainer**, I want to register Kubernetes clusters as deployment targets so that solutions from the catalog can be rolled out to them.

As a **cluster maintainer**, I want to define capacity constraints (CPU, memory, GPU, storage) and security domains on a target so that deployments are validated against the cluster's capabilities before rollout.

As a **cluster maintainer**, I want to retrieve an agent configuration upon target registration so that I can deploy the solar-agent to the target cluster with minimal manual setup.

As a **managed K8s provider**, I want to roll out different versions based on target properties so that incompatibilities are avoided.

## Agent & Cluster Operations

As a **solar-agent**, I want to perform preflight checks on the target cluster before reconciling desired state so that misconfigurations or missing prerequisites are caught before deployment.

As a **solar-agent**, I want to continuously report deployment status and cluster health back to the solar-apiserver so that operators have real-time visibility into the state of each target.

As a **solar-agent**, I want to track changes to deployed resources and report status transitions (e.g. progressing, ready, degraded) so that the UI and API reflect the actual rollout state.

## Releases & Deployment

As a **managed K8s provider**, I want to seamlessly roll out cluster addons to many clusters so that I have a declarative way to manage fleet state at scale.

As a **managed K8s provider**, I want to offer release channels (via Profiles) so that customers can choose how rapidly their cluster components are updated.

As a **deployment coordinator**, I want to create a Release referencing a ComponentVersion and assign it to targets so that I can control what gets deployed where.

As a **deployment coordinator**, I want to group multiple Releases into a Profile so that I can manage a consistent set of applications as a single unit.

As a **deployment coordinator**, I want capacity and capability pre-checks before deployment so that I am warned early when a solution does not fit a target cluster.

As a **K8s user**, I want deployments to follow a gitless GitOps pattern (OCI-based FluxCD) so that I do not need to maintain a separate Git repository for deployment manifests.

As a **K8s user** and **managed K8s provider**, I want to always rollout all applications with positive preflight checks to targets without negative side-effects from problematic releases, so that issues with one release do not affect changes by other releases for a particular target.

## Catalog Chaining & Multi-Environment

As a **managed K8s provider**, I want to chain SolAr catalogs across environments (via Sync and ARC) so that packages can be transported from one SolAr instance to another through security boundaries.

As a **solution maintainer**, I want packages to be checked for required attestations (security scans, STIG conformance) before deployment so that compliance requirements are enforced automatically.

## Administration

As a **super admin**, I want to define required attestation policies on a per-cluster or cluster-group level so that compliance requirements are enforced consistently across the fleet.

As a **super admin**, I want full visibility and control over all tenants, solutions, targets, and deployments so that I can intervene in exceptional situations across the entire system.

As a **super admin**, I want to manage user-to-group assignments and role bindings so that the permission model (solution maintainer, cluster maintainer, deployment coordinator) is enforced correctly.

## Platform & Developer Experience

As a **SolAr developer**, I want SolAr to be a composable solution supporting a variety of environments so that as many organizations can benefit from it as possible.

As a **SolAr developer**, I want to not be involved in how targets authenticate towards OCI registries so that authentication concerns are delegated to the cluster operator.

As a **K8s application developer**, I want to package my application as an OCM component and have SolAr handle the deployment lifecycle so that I can focus on building the application rather than deployment tooling.

As a **platform operator**, I want all SolAr resources to be manageable via kubectl and GitOps tools so that SolAr fits naturally into existing Kubernetes workflows.
