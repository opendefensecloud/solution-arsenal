---
status: proposed
date: 2026-04-11
---

# UI Architecture: Go Backend-for-Frontend with React SPA

## Context and Problem Statement

SolAr currently provides a fully Kubernetes-native API — all interaction happens via `kubectl`, GitOps tooling, or direct API calls. While this is sufficient for platform operators and developers, several user stories require a graphical interface for catalog browsing, target management, and deployment visibility. Non-CLI users (deployment coordinators, solution consumers, administrators) need a WYSIWYG experience that surfaces the relationships between Components, Releases, Targets, Profiles, and their bindings without requiring Kubernetes expertise.

The central challenge is how a UI should interact with the Kubernetes API while preserving the existing RBAC model — users in the UI must have the same permissions as they would via `kubectl`. This is complicated by the variety of authentication setups across Kubernetes clusters (OIDC, client certificates, service account tokens, cloud provider IAM) and the fact that SolAr is designed to work across different environments.

## Decision Drivers

- Users must have the same permissions in the UI as in `kubectl` — no privilege escalation
- Support multiple authentication mechanisms to work across different cluster setups
- Minimize operational complexity — few additional moving parts to deploy
- Leverage existing Go codebase and generated client-go clients
- Enable real-time visibility into resource state changes
- Keep the frontend decoupled from Kubernetes API specifics
- Support single-binary deployment for simplicity

## Considered Options

### Option 1: SPA interacting directly with the Kubernetes API

The React SPA talks directly to the Kubernetes API server. Authentication tokens are obtained client-side (e.g. via OIDC implicit/PKCE flow) and sent as Bearer tokens.

**Advantages:**
- No backend to build or maintain
- Direct use of Kubernetes RBAC — no proxy layer
- Lowest latency for API calls

**Disadvantages:**
- Kubernetes API servers do not serve CORS headers by default — requires a reverse proxy or API server configuration changes
- Tokens must be stored client-side (localStorage, sessionStorage, or cookies), increasing the attack surface
- No aggregation layer — the frontend must make multiple calls and stitch data together (e.g. Target + ReleaseBindings + RenderTasks for a single target view)
- Every Kubernetes API change (field renames, version bumps) directly impacts the frontend
- Supporting multiple auth mechanisms client-side is complex and fragile
- WebSocket-based watches require direct connectivity to the API server
- Cannot support kubeconfig upload without a backend to process it

### Option 2: GraphQL gateway (e.g. kubernetes-graphql-gateway)

A GraphQL middleware translates frontend queries into Kubernetes API calls. The frontend uses GraphQL for flexible, aggregated queries.

**Advantages:**
- Flexible query model — frontend can request exactly the data it needs in a single call
- Can aggregate related resources (Target + bindings + status) in one query
- Growing ecosystem of Kubernetes-to-GraphQL bridges

**Disadvantages:**
- Existing K8s-to-GraphQL bridges are immature and not production-hardened
- Schema maintenance overhead — must be kept in sync with SolAr API types
- Still requires a separate authentication layer in front of the gateway
- Adds operational complexity (another service to deploy, monitor, upgrade)
- GraphQL subscription support for K8s watches is not well established
- Overkill for an MVP — the query flexibility is not needed when the resource model is well-defined

### Option 3: Dedicated Go backend (Backend-for-Frontend)

A purpose-built Go HTTP server serves a REST API tailored to the UI's needs. It handles authentication, session management, and translates frontend requests into Kubernetes API calls using the generated client-go. The React SPA is embedded in the binary via `go:embed` and served as static files.

**Advantages:**
- Full control over authentication flows — can support OIDC, kubeconfig upload, and impersonation in one place
- Credentials never reach the browser — tokens and kubeconfigs stay server-side
- Aggregation layer — can combine multiple K8s resources into frontend-friendly responses
- Uses existing generated client-go directly — no schema translation
- Single binary deployment (`go:embed` for static SPA assets)
- Server-Sent Events for real-time updates backed by Kubernetes watches
- Can add validation, rate limiting, and audit logging without K8s API server changes
- Natural fit for the team's Go expertise

**Disadvantages:**
- More code to write and maintain than option 1
- An additional component to deploy (though single-binary mitigates this)
- API surface must be designed and versioned

## Decision Outcome

Chosen option: **Option 3 — Dedicated Go Backend-for-Frontend with React SPA.**

The authentication requirements alone make a backend mandatory. Supporting OIDC, kubeconfig upload, and impersonation across different cluster setups cannot be handled purely client-side. Since the project already has generated Go clients and deep Go expertise, a Go BFF is the natural choice over introducing a Node.js layer or an immature GraphQL bridge.

### Architecture

```
┌─────────────────┐       ┌───────────────────────┐       ┌──────────────────────┐
│   React SPA     │──────→│   solar-ui (Go)        │──────→│  K8s API Server      │
│   (embedded)    │ REST  │   BFF + Auth + SSE     │       │  + SolAr Extension   │
└─────────────────┘       └───────────────────────┘       └──────────────────────┘
                           ├─ Session store (cookie)│
                           ├─ Auth providers        │
                           └─ K8s client-go         │
```

### Authentication Strategy

The backend supports multiple authentication providers, selectable per deployment or per session. All providers implement a common interface that returns a per-request `rest.Config`:

| Provider | Flow | Use Case |
|----------|------|----------|
| **OIDC (Authorization Code + PKCE)** | Backend performs OAuth2 code exchange with the cluster's IdP, stores tokens in httpOnly secure cookie, refreshes transparently. Uses obtained ID/access token against K8s API. | Production clusters with OIDC-capable API server |
| **Impersonation** | Backend authenticates with its own ServiceAccount, sets `Impersonate-User` and `Impersonate-Group` headers based on the session's authenticated identity (from OIDC or other). | Clusters where the backend ServiceAccount is granted impersonation rights |
| **Kubeconfig upload** | User uploads/pastes kubeconfig in the UI. Backend extracts credentials (token, client cert, exec-based), stores them in the encrypted session, uses them for K8s API calls. | Development, local clusters, quick onboarding, air-gapped environments |

The OIDC and impersonation providers can be combined: OIDC establishes the user's identity, and impersonation is used if the cluster's API server does not accept the OIDC token directly (e.g. the IdP is not configured as an API server OIDC issuer, but the backend is trusted to impersonate).

### Backend Design

**Binary:** `cmd/solar-ui/main.go`

**Key packages:**

```
pkg/ui/
  server.go             — HTTP server, middleware, router
  auth/
    provider.go         — AuthProvider interface
    oidc.go             — OIDC authorization code flow
    impersonation.go    — ServiceAccount + impersonation headers
    kubeconfig.go       — Kubeconfig upload and extraction
  session/
    store.go            — Encrypted cookie-based session management
  api/
    targets.go          — Target list/detail (aggregated with bindings and status)
    releases.go         — Release list/detail
    components.go       — Component and ComponentVersion catalog
    profiles.go         — Profile list/detail with target match preview
    registries.go       — Registry list/detail
    rendertasks.go      — RenderTask status
    events.go           — SSE endpoint backed by K8s watches
```

**API surface (REST):**

```
# Authentication
POST   /api/auth/login            — initiate OIDC flow (redirects to IdP)
GET    /api/auth/callback          — OIDC callback
POST   /api/auth/kubeconfig        — upload kubeconfig
DELETE /api/auth/session            — logout
GET    /api/auth/me                 — current user info

# Resources (read)
GET    /api/targets                 — list targets with aggregated status
GET    /api/targets/:name           — target detail (+ bindings + render status)
GET    /api/releases                — list releases
GET    /api/releases/:name          — release detail (+ component version info)
GET    /api/components              — list components
GET    /api/components/:name        — component detail (+ versions)
GET    /api/profiles                — list profiles
GET    /api/profiles/:name          — profile detail (+ matched targets preview)
GET    /api/registries              — list registries
GET    /api/rendertasks             — list render tasks

# Resources (write) — MVP may start read-only
POST   /api/targets                 — create target
PUT    /api/targets/:name           — update target
DELETE /api/targets/:name           — delete target
# (same pattern for releases, profiles, bindings)

# Real-time
GET    /api/events                  — SSE stream (resource watch events)
```

The backend aggregates related resources server-side. For example, `GET /api/targets/:name` returns the target, its ReleaseBindings, RegistryBindings, referenced Registry, and current RenderTask status in a single response — avoiding N+1 queries from the frontend.

### Frontend Design

**Stack:**

- **React 19** + TypeScript
- **Vite** for build tooling
- **TanStack Query** for data fetching, caching, and SSE-driven cache invalidation
- **TanStack Router** for type-safe file-based routing
- **shadcn/ui** (Radix primitives + Tailwind CSS) for UI components — customizable, accessible, no heavy framework lock-in
- **SSE** via `EventSource` for live updates, feeding into TanStack Query cache

**Directory structure:**

```
ui/
  src/
    api/              — typed API client (hand-written or generated from OpenAPI)
    components/       — shared UI components
    pages/            — route-based page components
      dashboard/
      targets/
      releases/
      components/
      profiles/
    hooks/            — custom React hooks (useSSE, useAuth, etc.)
    lib/              — utilities
  vite.config.ts
  tsconfig.json
  package.json
```

**Key views (MVP):**

1. **Dashboard** — overview cards showing target count, release count, active render tasks, recent errors
2. **Targets** — list with status indicators; detail view showing bound releases, registries, render status
3. **Releases** — list/detail; shows referenced ComponentVersion, which targets use it
4. **Components** — catalog browser; component versions discovered by solar-discovery
5. **Profiles** — list/detail; shows target selector, matched targets, created ReleaseBindings

**Embedding:**

The built SPA is embedded into the Go binary via `//go:embed ui/dist`, making `solar-ui` a single binary that serves both the API and the frontend. No separate web server or CDN needed.

### Deployment

- **Helm chart**: new deployment in `charts/solar/` or a standalone `charts/solar-ui/` chart
- **ServiceAccount**: needs `get`, `list`, `watch` on all SolAr resources; needs `impersonate` verb on `users` and `groups` if using impersonation mode
- **Ingress/Route**: requires external access for browser clients; TLS termination at ingress
- **Configuration**: auth provider selection, OIDC issuer/client config, session encryption key

### Consequences

**Positive:**

- Single Go binary with embedded SPA — simple to build, deploy, and operate
- Authentication complexity is handled server-side; credentials never reach the browser
- Aggregated API responses reduce frontend complexity and improve performance
- SSE provides real-time updates without WebSocket complexity
- Reuses existing generated client-go — no schema translation or sync needed
- Same permission model as `kubectl` — no privilege escalation
- Can be deployed alongside the existing SolAr components with minimal changes

**Negative:**

- Additional component to develop and maintain (Go backend + React frontend)
- REST API surface must be designed, documented, and versioned
- Session management adds state to an otherwise stateless system
- OIDC configuration requires coordination with the cluster's IdP setup
- Frontend build tooling (Node.js, npm/pnpm) added to the project's dev dependencies

## Open Questions

- **Multi-cluster**: Should the UI support connecting to multiple clusters simultaneously, or is it one cluster per UI instance? Multi-cluster significantly complicates session management and resource aggregation. The simpler model (one instance per cluster, or one instance per SolAr control plane) is recommended for MVP.
- **Write operations**: Should the MVP include create/edit/delete, or start read-only? Read-only MVP reduces scope and risk; write operations can be added incrementally.
- **OpenAPI generation**: Should the BFF's REST API be defined OpenAPI-first (and generate server/client stubs), or code-first (and generate OpenAPI from code)? Code-first is faster for MVP; OpenAPI-first is better for long-term client generation.
- **Namespace scoping**: How does the UI handle multi-tenancy? Should users see all namespaces or only those they have access to? The backend can use SelfSubjectAccessReview to determine visibility.
- **Standalone or co-located chart**: Should `solar-ui` be a separate Helm chart or part of the main `charts/solar/` chart? Separate chart allows independent deployment lifecycle.

## Spike Recommendations

Before committing to full implementation, two focused spikes are recommended:

1. **Auth spike** (~2-3 days): Minimal Go HTTP server that performs OIDC authorization code flow against the cluster's IdP, stores the token in an encrypted cookie, and makes a `list namespaces` call using the user's identity. Validates the auth flow end-to-end. Stretch: also test impersonation mode.

2. **Frontend spike** (~2-3 days): React app with Vite + TanStack Query that renders a target list from a mock API, with live updates via SSE. Validates the frontend stack choice and real-time UX. Stretch: integrate with the auth spike's real backend.
