# Overview

## Non-production installation

If you just want to try out SolAr in a non-production environment (including on desktop via minikube/kind/k3d etc) follow the [quick-start guide](../../getting-started.md).

## Production installation

### Prerequisites

SolAr requires [cert-manager](https://cert-manager.io/docs/installation/) as a dependency.

SolAr also requires [Flux](https://fluxcd.io/flux/installation/)'s `source-controller` and `helm-controller` to be installed: SolAr's renderer produces `OCIRepository`/`HelmRelease` resources, and Flux is what actually reconciles them into a deployed release. Without Flux running, releases render but never roll out.

If your OCI registries (component/chart sources) use a private or self-signed CA, also install [trust-manager](https://cert-manager.io/docs/trust/trust-manager/) and set `caBundle.enabled=true` on the SolAr chart so the controller trusts that CA.

### Installation Methods

To install SolAr, navigate to the [releases page](https://github.com/opendefensecloud/solution-arsenal/releases) and find the release you wish to use (the latest full release is preferred).

#### Helm

See [Helm installation](./helm.md) for more information.

### Web UI (OIDC)

The UI is off by default because it needs an OIDC issuer. The backend is a **public client**: it holds no client secret and authenticates the authorization-code exchange with PKCE (S256) instead. Set `ui.oidc.existingSecret` only if your IdP issues a confidential client.

Installing against the Open Defense Cloud Zitadel:

```yaml
ui:
  enabled: true
  oidc:
    issuer: https://zitadel.opendefense.cloud
    clientID: '387085129840888657'
    # The externally reachable /api/auth/callback of this UI. SolAr ships no
    # Ingress, so access is via `kubectl port-forward` and the browser sees
    # loopback. Registered as a native app in Zitadel, which permits loopback
    # redirect URIs without putting the application into development mode.
    redirectURL: http://localhost:8090/api/auth/callback
    existingSecret: '' # public client — PKCE, no secret
```

Then port-forward and open `http://localhost:8090`:

```bash
kubectl port-forward -n solar-system svc/solar-ui 8090:8090
```

Kubernetes RBAC binds to the `email` claim, so the API server must be configured to trust the same issuer with the client ID as audience. See [Roles](../../developer-guide/roles.md) for the persona bindings.
