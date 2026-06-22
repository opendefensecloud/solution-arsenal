#!/usr/bin/env bash
# Sideload envtest binaries from canonical upstream sources (dl.k8s.io and
# etcd-io GitHub releases) for K8s versions that controller-tools hasn't
# packaged into its envtest-releases index yet. See issue #556 for
# the rationale (envtest's release cadence lags K8s releases sporadically;
# upstream is closed-as-not-planned).
#
# dl.k8s.io has all versions but only Linux builds. controller-tools has 
# Darwin builds, but not for all versions of Kubernetes and etcd.
# To run a specific version under Darwin might not be possible, 
# 
# So it should:
#   - check K8S_VERSION, use envtest-setup binaries if available
#   - if not, fall back to dl.k8s.io(which has no Darwin builds)
#
# Idempotent — exits 0 immediately if setup-envtest already has the version
# cached, so this is safe as a `test` prerequisite that runs every invocation.
#
# Required env / args:
#   $1                 K8s version (e.g. 1.36.0)
#   SETUP_ENVTEST      path to setup-envtest binary
#   BIN_DIR            cache directory (matches the Makefile's $(LOCALBIN))
#
# Optional env:
#   YQ                 path to yq (defaults to `yq` on PATH)

set -euo pipefail

K8S_VERSION="${1:?Usage: $0 <k8s-version>}"
# Accept either "1.32.1" or "v1.32.1" — strip an optional leading "v" so
# URL construction (which adds its own "v") doesn't end up with "vv1.32.1".
K8S_VERSION="${K8S_VERSION#v}"
: "${SETUP_ENVTEST:?SETUP_ENVTEST must be set}"
: "${BIN_DIR:?BIN_DIR must be set}"
YQ="${YQ:-yq}"

# Idempotency — bail out if setup-envtest already finds this version in cache.
if "$SETUP_ENVTEST" use "$K8S_VERSION" --bin-dir "$BIN_DIR" -i -p path >/dev/null 2>&1; then
  exit 0
fi

# Detect host platform up front — both the sideload paths and the messages
# below need it.
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$(uname -m)" in
  x86_64)        arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) echo "envtest-sideload: unsupported arch $(uname -m)" >&2; exit 1 ;;
esac

# We can only assemble a sideload tarball from upstream sources on linux and
# darwin. For anything else (e.g. windows) controller-tools cross-compiles its
# own envtest archives that we can't replicate from a shell script, so defer to
# vanilla `setup-envtest use` (the controller-tools index) and only fail when
# the requested version isn't there.
if [ "$os" != linux ] && [ "$os" != darwin ]; then
  if "$SETUP_ENVTEST" use "$K8S_VERSION" --bin-dir "$BIN_DIR" -p path >/dev/null 2>&1; then
    exit 0
  fi
  cat >&2 <<EOF
envtest-sideload: ${os} is supported only via controller-tools' pre-packaged
  archives, but '${K8S_VERSION}' is not in their index and this script can only
  sideload upstream binaries on linux and darwin.
  List available versions:    ${SETUP_ENVTEST} list
  Then pin ENVTEST_K8S_VERSION to one of those.
EOF
  exit 1
fi

# On darwin, dl.k8s.io publishes no kube-apiserver (Kubernetes builds server
# binaries only for linux/windows), so sideloading there means building
# kube-apiserver from source — a slow, heavy first run. Prefer controller-tools'
# pre-packaged darwin archive whenever the requested version is in their index;
# only fall through to the source build when it isn't.
if [ "$os" = darwin ]; then
  if "$SETUP_ENVTEST" use "$K8S_VERSION" --bin-dir "$BIN_DIR" -p path >/dev/null 2>&1; then
    exit 0
  fi
fi

# Wrap curl with retries + timeouts so transient network blips don't fail
# the whole sideload. --retry-all-errors covers HTTP 5xx (curl 7.71+, 2020).
# --max-time bounds a single request; bytes are ~150 MB max (kube-apiserver),
# 300 s is plenty even on slow runners.
fetch() {
  curl --fail --silent --show-error --location \
    --retry 3 --retry-delay 2 --retry-all-errors \
    --connect-timeout 10 --max-time 300 "$@"
}

# Portable SHA-256 verification: reads `<hash>  <file>` lines on stdin and
# checks them. GNU coreutils ships `sha256sum`; stock macOS ships only
# `shasum`. Both accept the same check format on stdin.
sha256_check() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum -c -
  else
    shasum -a 256 -c -
  fi
}

# Look up the etcd version K8s ships with from its build/dependencies.yaml.
# Self-updating per K8s release.
deps_url="https://raw.githubusercontent.com/kubernetes/kubernetes/v${K8S_VERSION}/build/dependencies.yaml"
etcd_version=$(fetch "$deps_url" \
  | "$YQ" '.dependencies[] | select(.name == "etcd") | .version')
if [[ -z "$etcd_version" ]]; then
  echo "envtest-sideload: could not resolve etcd version from $deps_url" >&2
  exit 1
fi

echo "envtest-sideload: K8s ${K8S_VERSION} ships with etcd ${etcd_version} (${os}/${arch})"

stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT
mkdir -p "$stage/k8s-bin"

# --- K8s binaries (kube-apiserver, kubectl) --------------------------------
# Direct per-binary downloads avoid pulling the ~600 MB server tarball when we
# only need two binaries. kubectl is published for every platform (incl.
# darwin); kube-apiserver only for linux/windows, so darwin builds it from
# source below.
#
# NOTE: If a future K8s release ships a new server binary that envtest
# expects (e.g. some hypothetical "super-controller-thing"), envtest will
# fail to start the control plane and the test run will surface the missing
# binary. Add the new name to the download/build steps below — the server
# tarball at dl.k8s.io/v${VER}/kubernetes-server-${os}-${arch}.tar.gz is the
# canonical index of what's available per release. Same applies if envtest
# itself ever adds a required binary beyond kube-apiserver/etcd/kubectl.
k8s_base="https://dl.k8s.io/release/v${K8S_VERSION}/bin/${os}/${arch}"
download_k8s_bin() {
  local bin="$1"
  echo "envtest-sideload: downloading ${k8s_base}/${bin}"
  fetch -o "$stage/k8s-bin/$bin"        "$k8s_base/$bin"
  fetch -o "$stage/k8s-bin/$bin.sha256" "$k8s_base/$bin.sha256"
  # The .sha256 sibling is just the hex digest; pair with the filename ourselves.
  ( cd "$stage/k8s-bin" && printf '%s  %s\n' "$(cat "$bin.sha256")" "$bin" | sha256_check )
  chmod +x "$stage/k8s-bin/$bin"
}

download_k8s_bin kubectl

if [ "$os" = linux ]; then
  download_k8s_bin kube-apiserver
else
  # darwin: no upstream kube-apiserver binary exists, so build it from the
  # tagged Kubernetes source — exactly what controller-tools does to produce
  # its darwin envtest archives. First run is slow (~hundreds of MB of source
  # + a multi-minute compile); the result is cached by setup-envtest afterward.
  command -v go >/dev/null 2>&1 || {
    echo "envtest-sideload: building kube-apiserver for ${os} requires Go on PATH" >&2
    exit 1
  }
  echo "envtest-sideload: no upstream ${os} kube-apiserver binary exists; building v${K8S_VERSION} from source (this can take several minutes; the result is cached)"
  src="$stage/kubernetes"
  git clone --quiet --depth 1 --branch "v${K8S_VERSION}" \
    https://github.com/kubernetes/kubernetes "$src"
  # Stamp version metadata the way upstream's hack/lib/version.sh does, so the
  # apiserver reports the correct version over /version (some tests read it).
  # `go build -X` silently ignores symbols a given release doesn't define, so
  # listing both the component-base and client-go mirrors is safe across versions.
  ver_major="${K8S_VERSION%%.*}"
  ver_rest="${K8S_VERSION#*.}"
  ver_minor="${ver_rest%%.*}"
  build_date=$(date -u +'%Y-%m-%dT%H:%M:%SZ')
  ldflags=""
  for pkg in k8s.io/component-base/version k8s.io/client-go/pkg/version; do
    ldflags+=" -X ${pkg}.gitVersion=v${K8S_VERSION}"
    ldflags+=" -X ${pkg}.gitMajor=${ver_major}"
    ldflags+=" -X ${pkg}.gitMinor=${ver_minor}"
    ldflags+=" -X ${pkg}.gitCommit=v${K8S_VERSION}"
    ldflags+=" -X ${pkg}.gitTreeState=clean"
    ldflags+=" -X ${pkg}.buildDate=${build_date}"
  done
  ( cd "$src" && CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
      go build -ldflags "$ldflags" -o "$stage/k8s-bin/kube-apiserver" ./cmd/kube-apiserver )
  chmod +x "$stage/k8s-bin/kube-apiserver"
fi

# --- etcd archive (etcd binary) --------------------------------------------
# etcd publishes a .tar.gz per linux platform and a .zip per darwin platform.
etcd_dir="etcd-v${etcd_version}-${os}-${arch}"
etcd_base="https://github.com/etcd-io/etcd/releases/download/v${etcd_version}"
if [ "$os" = linux ]; then
  etcd_archive="${etcd_dir}.tar.gz"
  extract_etcd() { tar -C "$stage" -xzf "$stage/$etcd_archive" "$etcd_dir/etcd"; }
else
  etcd_archive="${etcd_dir}.zip"
  extract_etcd() { unzip -q -o -d "$stage" "$stage/$etcd_archive" "$etcd_dir/etcd"; }
fi
echo "envtest-sideload: downloading ${etcd_base}/${etcd_archive}"
fetch -o "$stage/$etcd_archive" "$etcd_base/$etcd_archive"
fetch -o "$stage/SHA256SUMS"    "$etcd_base/SHA256SUMS"
# etcd's SHA256SUMS lists `<hash>  <filename>` for every platform archive;
# select our exact filename (awk field match — no regex metachars) and verify.
( cd "$stage" && awk -v f="$etcd_archive" '$2 == f' SHA256SUMS | sha256_check )
extract_etcd

# --- Assemble the sideload tarball with the three binaries at top level ---
sideload_tar="$stage/sideload.tar.gz"
tar -czf "$sideload_tar" \
  -C "$stage/k8s-bin"     kube-apiserver kubectl \
  -C "$stage/$etcd_dir"   etcd

# Feed it to setup-envtest. --os/--arch matter — sideload writes the cache
# entry keyed on platform.
echo "envtest-sideload: sideloading ${K8S_VERSION} into ${BIN_DIR}"
"$SETUP_ENVTEST" sideload "$K8S_VERSION" \
  --os "$os" --arch "$arch" --bin-dir "$BIN_DIR" \
  < "$sideload_tar"

# Sanity-check that setup-envtest can now find it.
"$SETUP_ENVTEST" use "$K8S_VERSION" --bin-dir "$BIN_DIR" -i -p path >/dev/null
echo "envtest-sideload: done"
