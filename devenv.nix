{ pkgs, ... }:

{
  # https://devenv.sh/packages/
  packages = [
    pkgs.gnumake
    pkgs.jq
    pkgs.shellcheck
    pkgs.yq-go
    pkgs.kind
    pkgs.kubectl
    pkgs.kubernetes-helm
    pkgs.osv-scanner
  ];

  # https://devenv.sh/languages/
  languages.go.enable = true;
  languages.go.version = "1.25.7";

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
      entry = "osv-scanner scan --config ./.osv-scanner.toml -r .";
      files = "\\.(mod|sum)$";
      pass_filenames = false;
    };
  };
  # See full reference at https://devenv.sh/reference/options/
}
