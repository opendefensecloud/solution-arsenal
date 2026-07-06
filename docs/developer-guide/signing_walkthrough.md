# OCI Artifact Signing Walkthrough

`solar-renderer` signs pushed Helm charts with cosign using a static key pair — no OIDC, Fulcio, or Rekor required.
The signature is stored as a separate OCI artifact in the same repository (`sha256-<digest>.sig`).

FluxCD can verify the signature via `OCIRepository.spec.verify`.

## Prerequisites

- Docker (for the zot registry container)
- `solar-renderer` binary
- `cosign` CLI (optional — only needed for key generation and manual verification)

## 1. Generate a Cosign Key Pair

```bash
cosign generate-key-pair
```

Creates `cosign.key` (encrypted private key) and `cosign.pub` (public key).

> For headless environments: `COSIGN_PASSWORD=<pw> cosign generate-key-pair --output-key-prefix cosign`

## 2. Start the Registry

Run zot as a Docker container:

```bash
docker run -d --rm --name zot \
  -p 5000:5000 \
  ghcr.io/project-zot/zot-linux-amd64:latest
```

Verify it's running:

```bash
curl -s http://localhost:5000/v2/ | jq .
```

## 3. Render, Push, and Sign

Create `config.yaml`:

```yaml
type: release
release:
  chart:
    name: my-app
    description: My Application
    version: 0.1.0
    appVersion: "1.0"
  input:
    component:
      name: my-component
    resources:
      main:
        repository: oci://registry.example.com/my-image
        tag: v1.0.0
    entrypoint:
      resourceName: main
      type: Helm
  values: {}
```

Run:

```bash
solar-renderer \
  --url oci://localhost:5000/my-app:0.1.0 \
  --plain-http=true \
  --signing-key=cosign.key \
  config.yaml
```

With a password (or set `SIGNING_PASSWORD` env var):

```bash
solar-renderer \
  --url oci://localhost:5000/my-app:0.1.0 \
  --plain-http=true \
  --signing-key=cosign.key \
  --signing-password=<your-password> \
  config.yaml
```

Expected output:

```
Rendered release to /tmp/solar-release-XXXXXXXX
Pushed result to oci://localhost:5000/my-app:0.1.0
```

## 4. Verify the Signature

```bash
cosign verify \
  --key cosign.pub \
  --insecure-ignore-tlog \
  localhost:5000/my-app:0.1.0
```

## 5. FluxCD Verification

Create a Secret with the public key:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: cosign-pub
  namespace: default
data:
  cosign.pub: $(base64 -w0 < cosign.pub)
```

Create an `OCIRepository` with verification enabled:

```yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: OCIRepository
metadata:
  name: my-app
  namespace: default
spec:
  interval: 5m
  url: oci://localhost:5000/my-app
  insecure: true
  ref:
    tag: 0.1.0
  verify:
    provider: cosign
    secretRef:
      name: cosign-pub
  layerSelector:
    mediaType: "application/vnd.cncf.helm.chart.content.v1.tar+gzip"
    operation: copy
---
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: my-app
  namespace: default
spec:
  interval: 10m
  chartRef:
    kind: OCIRepository
    name: my-app
```

## Troubleshooting

- **`failed to load signing key`**: The private key file is missing, invalid, or the password is wrong.
- **`failed to resolve image digest`**: The chart hasn't been pushed yet, the ref is wrong, or the registry is unreachable.
- **`failed to write signature to registry`**: The registry rejected the signature artifact — check auth and registry permissions.
- **FluxCD reports `verification failed`**: The public key in the Secret doesn't match the private key used during signing. Regenerate the pair.
