{
  pkgs,
  lib,
  config,
  inputs,
  ...
}:
let
  pkgs-unstable = import inputs.nixpkgs-unstable { system = pkgs.stdenv.system; };
in
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
  ];

  # https://devenv.sh/languages/
  languages.go.enable = true;
  languages.go.package = pkgs-unstable.go;

  git-hooks.hooks = {
    gofmt.enable = true;
    golangci-lint.enable = false;
  };
  # See full reference at https://devenv.sh/reference/options/

  difftastic.enable = true;
  delta.enable = true;
}
