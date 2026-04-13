# Build the manager binary
FROM --platform=$BUILDPLATFORM golang:1.26.2 AS builder

WORKDIR /workspace
RUN go env -w GOMODCACHE=/root/.cache/go-build

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# Cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN --mount=type=cache,target=/root/.cache/go-build go mod download

# Copy the go source
COPY api/ api/
COPY client-go/ client-go/
COPY cmd/ cmd/
COPY pkg/ pkg/
COPY web/ web/

ARG TARGETOS
ARG TARGETARCH
ARG GO_BUILD_FLAGS

RUN mkdir bin

FROM builder AS apiserver-builder
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GO111MODULE=on go build -ldflags="-s -w" ${GO_BUILD_FLAGS} -o bin/solar-apiserver ./cmd/solar-apiserver

FROM builder AS manager-builder
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GO111MODULE=on go build -ldflags="-s -w" ${GO_BUILD_FLAGS} -o bin/solar-controller-manager ./cmd/solar-controller-manager

FROM builder AS discovery-builder
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GO111MODULE=on go build -ldflags="-s -w" ${GO_BUILD_FLAGS} -o bin/solar-discovery ./cmd/solar-discovery

FROM builder AS renderer-builder
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GO111MODULE=on go build -ldflags="-s -w" ${GO_BUILD_FLAGS} -o bin/solar-renderer ./cmd/solar-renderer

FROM --platform=$BUILDPLATFORM node:22-alpine AS ui-frontend-builder
WORKDIR /workspace/web
COPY web/package.json web/pnpm-lock.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY web/ .
RUN pnpm build

FROM builder AS ui-builder
COPY --from=ui-frontend-builder /workspace/web/dist pkg/ui/static/
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GO111MODULE=on go build -ldflags="-s -w" ${GO_BUILD_FLAGS} -o bin/solar-ui ./cmd/solar-ui

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot AS apiserver
WORKDIR /
COPY --from=apiserver-builder /workspace/bin/solar-apiserver .
USER 65532:65532
ENTRYPOINT ["/solar-apiserver"]

FROM gcr.io/distroless/static:nonroot AS manager
WORKDIR /
COPY --from=manager-builder /workspace/bin/solar-controller-manager .
USER 65532:65532
ENTRYPOINT ["/solar-controller-manager"]

FROM gcr.io/distroless/static:nonroot AS renderer
WORKDIR /
COPY --from=renderer-builder /workspace/bin/solar-renderer .
USER 65532:65532
ENTRYPOINT ["/solar-renderer"]

FROM gcr.io/distroless/static:nonroot AS discovery
WORKDIR /
COPY --from=discovery-builder /workspace/bin/solar-discovery .
USER 65532:65532
ENTRYPOINT ["/solar-discovery"]

FROM gcr.io/distroless/static:nonroot AS ui
WORKDIR /
COPY --from=ui-builder /workspace/bin/solar-ui .
USER 65532:65532
ENTRYPOINT ["/solar-ui"]
