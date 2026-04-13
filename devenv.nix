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
  languages.go.version = "1.26.2";

  files."bin/pre-commit-golangci-lint" = {
    text = ''
        #!/usr/bin/env bash
        set -e
        make golangci-lint
        for dir in $(echo "$@" | xargs -n1 dirname | sort -u); do
          ./bin/golangci-lint run ./"$dir"
        done
      '';
    executable = true;
  };

  git-hooks.hooks = {
    gofmt.enable = true;
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
