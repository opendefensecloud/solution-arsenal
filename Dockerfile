# Build the manager binary
FROM --platform=$BUILDPLATFORM golang:1.27.4@sha256:205c7636d74e0ffbe3d007162df7d74f859f5a8c8322dfb9f907977686c126cf AS builder

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

FROM builder AS agent-builder
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GO111MODULE=on go build -ldflags="-s -w" ${GO_BUILD_FLAGS} -o bin/solar-agent ./cmd/solar-agent

FROM --platform=$BUILDPLATFORM node:26-alpine@sha256:a90bda2938f2b57112d94ee2e3722c5346efca76e4a034042b5223197567a16c AS ui-frontend-builder
ENV CI=true
WORKDIR /workspace/web
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN npm install -g "$(node -p 'require("./package.json").packageManager')" && pnpm install --frozen-lockfile
COPY web/ .
RUN pnpm build

FROM builder AS ui-builder
COPY --from=ui-frontend-builder /workspace/web/dist pkg/ui/static/
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GO111MODULE=on go build -ldflags="-s -w" ${GO_BUILD_FLAGS} -o bin/solar-ui ./cmd/solar-ui

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot@sha256:8ce6dd87e9823c5b48dcfb2c8604714ccb351ce59607327bcc4957448da279b5 AS apiserver
WORKDIR /
COPY --from=apiserver-builder /workspace/bin/solar-apiserver .
USER 65532:65532
ENTRYPOINT ["/solar-apiserver"]

FROM gcr.io/distroless/static:nonroot@sha256:8ce6dd87e9823c5b48dcfb2c8604714ccb351ce59607327bcc4957448da279b5 AS manager
WORKDIR /
COPY --from=manager-builder /workspace/bin/solar-controller-manager .
USER 65532:65532
ENTRYPOINT ["/solar-controller-manager"]

FROM gcr.io/distroless/static:nonroot@sha256:8ce6dd87e9823c5b48dcfb2c8604714ccb351ce59607327bcc4957448da279b5 AS renderer
WORKDIR /
COPY --from=renderer-builder /workspace/bin/solar-renderer .
USER 65532:65532
ENTRYPOINT ["/solar-renderer"]

FROM gcr.io/distroless/static:nonroot@sha256:8ce6dd87e9823c5b48dcfb2c8604714ccb351ce59607327bcc4957448da279b5 AS discovery
WORKDIR /
COPY --from=discovery-builder /workspace/bin/solar-discovery .
USER 65532:65532
ENTRYPOINT ["/solar-discovery"]

FROM gcr.io/distroless/static:nonroot@sha256:8ce6dd87e9823c5b48dcfb2c8604714ccb351ce59607327bcc4957448da279b5 AS agent
WORKDIR /
COPY --from=agent-builder /workspace/bin/solar-agent .
USER 65532:65532
ENTRYPOINT ["/solar-agent"]

FROM gcr.io/distroless/static:nonroot@sha256:8ce6dd87e9823c5b48dcfb2c8604714ccb351ce59607327bcc4957448da279b5 AS ui
WORKDIR /
COPY --from=ui-builder /workspace/bin/solar-ui .
USER 65532:65532
ENTRYPOINT ["/solar-ui"]
