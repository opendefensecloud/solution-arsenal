{
  description = "SOLAR development flake";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";

    go-overlay = {
      url = "github:purpleclay/go-overlay";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.flake-utils.follows = "flake-utils";
    };

    dev-kit = {
      url = "github:opendefensecloud/dev-kit";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.go-overlay.follows = "go-overlay";
      inputs.flake-utils.follows = "flake-utils";
    };
  };

  outputs = { nixpkgs, flake-utils, dev-kit, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        kindOverlay = final: prev: {
          kind = prev.kind.overrideAttrs (old: {
            version = "0.32.0";
            src = final.fetchFromGitHub {
              owner = "kubernetes-sigs";
              repo = "kind";
              rev = "v0.32.0";
              hash = "sha256-ii0VhS1Nib+r2ZFIIkRvkcGY1fLxev6WnhbqvaZW7j8=";
            };
            vendorHash = "sha256-tRpylYpEGF6XqtBl7ESYlXKEEAt+Jws4x4VlUVW8SNI=";
          });
        };
        pkgs = import nixpkgs {
          inherit system;
          overlays = [ kindOverlay ];
        };
      in
      {
        devShells.default = (dev-kit.lib.mkShell {
          inherit system;
          goVersion = "1.26.4";
          packages = with pkgs; [
            fluxcd
            nodejs_22
            pnpm
            chromium
          ];

          env.PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD = "1";
          env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH = "${pkgs.chromium}/bin/chromium";

          preCommitHooks = {
            commitlint.enable = true;
          };
        }).overrideAttrs (old: {
          nativeBuildInputs = builtins.filter
            (p: p.name or "" != "kind-0.31.0")
            (old.nativeBuildInputs or []) ++ [ pkgs.kind ];
        });
      }
    );
}
