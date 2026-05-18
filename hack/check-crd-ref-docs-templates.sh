#!/usr/bin/env bash
# Verify that the upstream crd-ref-docs markdown templates have not changed since
# hack/crd-ref-docs-templates/ was last synced. Run as part of lint-no-golangci.
# If this script fails, review the upstream diff, update the templates, reapply
# the *List patch in type.tpl, regenerate api-reference.md, and update
# hack/crd-ref-docs-templates/.upstream-checksums.

set -euo pipefail

TOOLS_LOCK="tools.lock"
CHECKSUMS="hack/crd-ref-docs-templates/.upstream-checksums"

version=$(awk '/^crd-ref-docs/{print $2}' "$TOOLS_LOCK" | cut -d'@' -f2)
upstream=$(go mod download -json "github.com/elastic/crd-ref-docs@${version}" \
  | grep '"Dir"' | cut -d'"' -f4)/templates/markdown

failed=0
while IFS='  ' read -r expected file; do
  actual=$(sha256sum "${upstream}/${file}" | awk '{print $1}')
  if [ "$actual" != "$expected" ]; then
    echo "ERROR: upstream crd-ref-docs template '${file}' has changed in ${version}."
    echo "       Review the diff below and update hack/crd-ref-docs-templates/ accordingly."
    diff "${upstream}/${file}" "hack/crd-ref-docs-templates/${file}" || true
    failed=1
  fi
done < "$CHECKSUMS"

if [ "$failed" -eq 1 ]; then
  echo ""
  echo "After updating, regenerate .upstream-checksums with:"
  echo "  upstream=\$(go mod download -json github.com/elastic/crd-ref-docs@\${version} | grep '\"Dir\"' | cut -d'\"' -f4)/templates/markdown"
  echo "  sha256sum \"\$upstream\"/{gv_details,gv_list,type,type_members}.tpl | sed \"s|\$upstream/||\""
  exit 1
fi
