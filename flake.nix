{
  description = "solar - development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";

    git-hooks = {
      url = "github:cachix/git-hooks.nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    go-overlay = {
      url = "github:purpleclay/go-overlay";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.git-hooks.follows = "git-hooks";
    };

    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = { nixpkgs, flake-utils, ... }@inputs:
    flake-utils.lib.eachDefaultSystem (system: let
      pkgs = import nixpkgs {
        inherit system;
        overlays = [
          inputs.go-overlay.overlays.default
          inputs.gomod2nix.overlays.default
        ];
      };

      goVersion = "1.26.2";
      go = pkgs.go-bin.versions.${goVersion};

      pre-commit-golangci-lint = pkgs.writeScriptBin "pre-commit-golangci-lint" ''
        #!/usr/bin/env bash
        set -e
        make golangci-lint
        for dir in $(echo "$@" | xargs -n1 dirname | sort -u); do
          ./bin/golangci-lint run ./"$dir"
        done
      '';

      pre-commit-check = inputs.git-hooks.lib.${system}.run {
        src = ./.;
        hooks = {
          gofmt.enable = true;

          golangci-lint = {
            enable = true;
            entry = "${pre-commit-golangci-lint}/bin/pre-commit-golangci-lint";
          };

          osv-scanner = {
            enable = true;
            entry = "make scan";
            files = "\\.(mod|sum)$|requirements\\.txt$";
            pass_filenames = false;
          };
        };
      };
    in {
      devShells.default = pkgs.mkShell {
        inherit (pre-commit-check) shellHook;
        packages = with pkgs; [
          fluxcd
          gnumake
          jq
          kind
          kubectl
          kubernetes-helm
          shellcheck
          yq-go
          go
          gotools
        ];
      };

      checks.pre-commit-check = pre-commit-check;
    }
  );
}
