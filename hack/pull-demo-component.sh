#!/usr/bin/env bash
#
# Pull the ocm-demo component fixture into a local CTF.
#
# The component is released from github.com/opendefensecloud/ocm (ocm-demo/).
# It is transferred by value, so every resource becomes a local blob carrying
# the referenceName derived from its source reference. Test transfers are
# offline from then on, and the fixture matches the shape of a real air-gapped
# CTF. A later transfer into a registry, with the same uploader configuration,
# republishes chart and image as tagged artifacts at "<target>/<referenceName>".
# The values template has no referenceName and stays a local blob.
#
# The signature is verified against the identity of the ocm repository's
# release workflow before the CTF replaces the previous one.
set -euo pipefail

OCM="${OCM:-ocm}"
OCM_DEMO_REPO="${OCM_DEMO_REPO:-ghcr.io/opendefensecloud}"
OCM_DEMO_COMPONENT="${OCM_DEMO_COMPONENT:-opendefense.cloud/ocm-demo}"
OCM_DEMO_VERSION="${OCM_DEMO_VERSION:?OCM_DEMO_VERSION must be set}"
OCM_DEMO_DIR="${OCM_DEMO_DIR:?OCM_DEMO_DIR must point at the CTF to create}"
OCM_UPLOADERS="${OCM_UPLOADERS:-$(pwd)/test/fixtures/ocm-uploaders.yaml}"
OCM_DEMO_VERIFIER="${OCM_DEMO_VERIFIER:-$(pwd)/test/fixtures/ocm-demo-verifier.ocmconfig}"

ref="${OCM_DEMO_COMPONENT}:${OCM_DEMO_VERSION}"
stamp="${OCM_DEMO_DIR}/pulled"

if [ -f "$stamp" ] && [ "$(cat "$stamp")" == "${OCM_DEMO_REPO}//${ref}" ]; then
	echo "ocm-demo ${OCM_DEMO_VERSION} already pulled, skipping..."
	exit 0
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

echo "Pulling ${OCM_DEMO_REPO}//${ref} by value..."
"$OCM" --config "$OCM_UPLOADERS" transfer cv "${OCM_DEMO_REPO}//${ref}" "ctf::${work}/ctf"

echo "Verifying the signature..."
"$OCM" --config "$OCM_DEMO_VERIFIER" verify cv "ctf::${work}/ctf//${ref}"

rm -rf "$OCM_DEMO_DIR"
mkdir -p "$(dirname "$OCM_DEMO_DIR")"
mv "${work}/ctf" "$OCM_DEMO_DIR"
echo "${OCM_DEMO_REPO}//${ref}" >"$stamp"
