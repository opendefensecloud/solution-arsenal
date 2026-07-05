{ pkgs, ... }:

{
  # https://devenv.sh/packages/
  packages = [
    pkgs.fluxcd
    pkgs.gnumake
    pkgs.jq
    pkgs.kind
    pkgs.kubectl
    pkgs.kubernetes-helm
    pkgs.nodejs_22
    pkgs.pnpm
    pkgs.chromium
    pkgs.shellcheck
    pkgs.yq-go
  ];

  env.PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD = "1";
  env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH = "${pkgs.chromium}/bin/chromium";

  # https://devenv.sh/languages/
  languages.go.enable = true;
  languages.go.version = "1.26.4";

  files."bin/pre-commit-golangci-lint" = {
    text = ''
        #!/usr/bin/env bash
        set -e

        BINDIR="$(cd "$(dirname "$0")" && pwd)"
        GOLANGCI_LINT="$BINDIR/golangci-lint"

        if [ ! -x "$GOLANGCI_LINT" ]; then
          if command -v go &>/dev/null && [ -f "$BINDIR/../tools.lock" ]; then
            TOOLSPEC=$(grep '^golangci-lint ' "$BINDIR/../tools.lock" | head -1)
            if [ -n "$TOOLSPEC" ]; then
              PKG="${TOOLSPEC#golangci-lint }"
              echo "pre-commit: installing golangci-lint ($PKG)..." >&2
              GOBIN="$BINDIR" go install "$PKG"
            fi
          fi
        fi

        if [ ! -x "$GOLANGCI_LINT" ]; then
          echo "error: golangci-lint not found at $GOLANGCI_LINT" >&2
          echo "Run 'make lint' or 'make fmt' to install it, or install it manually." >&2
          exit 1
        fi

        for dir in $(echo "$@" | xargs -n1 dirname | sort -u); do
          "$GOLANGCI_LINT" run ./"$dir"
        done
      '';
    executable = true;
  };

  git-hooks.hooks = {
    golangci-lint = {
      enable = true;
      entry = "./bin/pre-commit-golangci-lint";
    };
    osv-scanner = {
      enable = true;
      name = "osv-scanner";
      entry = "make scan";
      files = "\\.(mod|sum)$|requirements.txt$";
      pass_filenames = false;
    };
  };
  # See full reference at https://devenv.sh/reference/options/
}
