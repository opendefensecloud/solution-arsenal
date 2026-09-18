# Build the manager binary
FROM --platform=$BUILDPLATFORM golang:1.27.0@sha256:4013ae0f9e7994f8535c58c811f8f863fbed38b72e0d51e6592156f758d66146 AS builder

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# Cache modules and build artifacts so source-only changes don't re-download.
# Modules → default $GOMODCACHE (/go/pkg/mod); build cache → /root/.cache/go-build.
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    go mod download

# Copy the go source
COPY api/ api/
COPY client-go/ client-go/
COPY cmd/ cmd/
COPY pkg/ pkg/

ARG TARGETOS
ARG TARGETARCH
ARG GO_BUILD_FLAGS

RUN mkdir bin

FROM builder AS apiserver-builder
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GO111MODULE=on go build -ldflags="-s -w" ${GO_BUILD_FLAGS} -o bin/solar-apiserver ./cmd/solar-apiserver

FROM builder AS manager-builder
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GO111MODULE=on go build -ldflags="-s -w" ${GO_BUILD_FLAGS} -o bin/solar-controller-manager ./cmd/solar-controller-manager

FROM builder AS discovery-builder
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GO111MODULE=on go build -ldflags="-s -w" ${GO_BUILD_FLAGS} -o bin/solar-discovery ./cmd/solar-discovery

FROM builder AS renderer-builder
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GO111MODULE=on go build -ldflags="-s -w" ${GO_BUILD_FLAGS} -o bin/solar-renderer ./cmd/solar-renderer

FROM --platform=$BUILDPLATFORM node:24-alpine@sha256:333f6b3eca25980d5682c26207665b93c9417786b21760b2764d5821d9704c8a AS ui-frontend-builder
ENV CI=true
WORKDIR /workspace/web
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY web/ .
RUN pnpm build

FROM builder AS ui-builder
COPY --from=ui-frontend-builder /workspace/web/dist pkg/ui/static/
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GO111MODULE=on go build -ldflags="-s -w" ${GO_BUILD_FLAGS} -o bin/solar-ui ./cmd/solar-ui

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3 AS apiserver
WORKDIR /
COPY --from=apiserver-builder /workspace/bin/solar-apiserver .
USER 65532:65532
ENTRYPOINT ["/solar-apiserver"]

FROM gcr.io/distroless/static:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3 AS manager
WORKDIR /
COPY --from=manager-builder /workspace/bin/solar-controller-manager .
USER 65532:65532
ENTRYPOINT ["/solar-controller-manager"]

FROM gcr.io/distroless/static:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3 AS renderer
WORKDIR /
COPY --from=renderer-builder /workspace/bin/solar-renderer .
USER 65532:65532
ENTRYPOINT ["/solar-renderer"]

FROM gcr.io/distroless/static:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3 AS discovery
WORKDIR /
COPY --from=discovery-builder /workspace/bin/solar-discovery .
USER 65532:65532
ENTRYPOINT ["/solar-discovery"]

FROM gcr.io/distroless/static:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3 AS ui
WORKDIR /
COPY --from=ui-builder /workspace/bin/solar-ui .
USER 65532:65532
ENTRYPOINT ["/solar-ui"]
