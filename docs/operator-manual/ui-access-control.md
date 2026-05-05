# UI Access Control

The SolAr UI uses Kubernetes RBAC to control which features are available to logged-in users. There are two distinct access levels:

- **Admin** — can use the "Preview as" impersonation feature to browse the UI as another persona
- **Regular user** — sees only the resources their own OIDC identity is permitted to access

## Granting Admin Access

Admin access is determined by membership in a `ClusterRoleBinding` labeled `solar.opendefense.cloud/admin=true`. The BFF checks this label using its own service-account credentials, so the check is independent of any active impersonation state.

Use the `ui.admin.subjects` Helm value to declare which users or groups are admins:

```yaml
ui:
  admin:
    subjects:
      - kind: User
        name: admin@example.com
        apiGroup: rbac.authorization.k8s.io
      - kind: Group
        name: platform-admins
        apiGroup: rbac.authorization.k8s.io
```

This creates a `ClusterRoleBinding` named `solar-ui:admin` with the required label. You can also manage the binding manually — any `ClusterRoleBinding` with the label `solar.opendefense.cloud/admin=true` that lists the user as a subject will grant admin access.

## Configuring Impersonation Personas

Admins can preview the UI as another persona via the "Preview as" dropdown. Personas are defined with `ui.impersonation.targets`:

```yaml
ui:
  impersonation:
    targets:
      - username: maintainer@example.com
        groups:
          - maintainer
      - username: coordinator@example.com
        groups:
          - coordinator
```

For each entry the chart creates a `ClusterRole` labeled `solar.opendefense.cloud/impersonatable=true` and a `ClusterRoleBinding` that grants the BFF service account the right to impersonate that user. The BFF lists these roles at runtime, so adding or removing personas only requires a `helm upgrade` — no restart needed.

!!! note
    Personas do not need to be real OIDC users. They are K8s impersonation targets only — give them appropriate `ClusterRole` bindings to define what they can see.

## Required BFF Service Account Permissions

The chart automatically creates a `ClusterRole` named `solar-ui:read-rbac` and binds it to the BFF service account. It grants `list` on `clusterroles` and `clusterrolebindings` — the minimum required for admin checks and persona discovery. No additional setup is needed.

## Summary

| Helm value | Effect |
|---|---|
| `ui.admin.subjects` | Users/groups that can access impersonation management |
| `ui.impersonation.targets` | Personas available in the "Preview as" dropdown |
| `ui.impersonation.serviceAccountName` | BFF service account that receives impersonation rights (default: `solar-ui`) |
