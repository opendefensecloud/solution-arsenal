# User Stories

## Application Catalog

As an **app catalog maintainer**, I want to add applications to the catalog so that they are discoverable for deployment.

As an **app catalog maintainer**, I want to manage access policies for applications in the catalog to ensure only authorized **K8s cluster provider** and **K8s cluster user** can deploy them.

As an **app catalog maintainer**, I want the catalog to sync automatically with an OCI registry to reduce manual effort.

As a **K8s cluster provider** and **K8s cluster user**, I want to browse a catalog with applications I can deploy to a K8s cluster.

As an **app provider**, I want to package my application for the catalog so that it is deployable to a K8s cluster.

## K8s Cluster Management

As a **K8s cluster provider**, I want to register K8s clusters as deployment targets so that applications from the catalog can be deployed to them.

As a **K8s cluster provider**, I want K8s clusters to automatically report their hardware and software specifications (CPU, RAM, GPU, K8s version) to eliminate manual inventory tracking.

As a **K8s cluster provider** and **K8s cluster user**, I want to define capacity constraints (CPU, memory, GPU, storage) for my K8s clusters so that deployments are validated against the capabilities.

## Deployment Management

As a **K8s cluster provider**, I want to deploy applications across an entire fleet of K8s clusters at scale.

As a **K8s cluster user**, I want to deploy applications to my K8s clusters.

As a **K8s cluster provider**, I want to see the deployment status of all applications for all K8s clusters I have registered.

As a **K8s cluster user**, I want to see the deployment status of both my own applications and those deployed by the **K8s cluster provider**.

As a **K8s cluster provider** and **K8s cluster user**, I want Solar to automatically select the correct application version based on K8s cluster properties to prevent version mismatches.

As a **K8s cluster provider** and **K8s cluster user**, I want deployments to follow a gitless GitOps pattern so that I do not need to maintain a separate Git repository for deployment manifests.

As a **K8s cluster provider** and **K8s cluster user**, I want applications to be checked for required attestations (security scans, STIG conformance) before deployment so that compliance requirements are enforced automatically.

## Catalog Chaining & Multi-Environment

As an **app catalog maintainer**, I want to chain application catalogs across environments (via Sync and ARC) so that applications can be transported from one Solar instance to another through security boundaries.

## Administration

As a **Solar operator**, I want insights into Solar so that I can ensure the health of the system.

As a **Solar operator**, I want to enforce a permission model (e.g. app catalog maintainer, K8s cluster provider, K8s cluster user).