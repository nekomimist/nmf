{
  description = "Reproducible development environment for nmf";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { nixpkgs, ... }:
    let
      supportedSystems = [ "x86_64-linux" "aarch64-linux" ];
      forEachSystem = function:
        nixpkgs.lib.genAttrs supportedSystems (system: function (import nixpkgs {
          inherit system;
        }));
    in {
      devShells = forEachSystem (pkgs: {
        default = pkgs.mkShell {
          # nixpkgs omits Go's top-level license files from the installed GOROOT.
          NMF_GO_LICENSE_DIR = pkgs.runCommand "nmf-go-license-documents" { } ''
            mkdir -p "$out"
            tar -xf ${pkgs.go_1_26.src} --strip-components=1 -C "$out" go/LICENSE go/PATENTS
          '';

          packages = with pkgs; [
            go_1_26
            zig
            fyne
            gnumake
            pkg-config
            python3
            llvmPackages.llvm
          ];

          buildInputs = with pkgs; [
            libGL
            libxkbcommon
            wayland
            libx11
            libxcursor
            libxi
            libxinerama
            libxrandr
            libxxf86vm
          ];

          shellHook = ''
            export GOTOOLCHAIN=local
            export GOFLAGS="-mod=readonly''${GOFLAGS:+ $GOFLAGS}"
            export NMF_NIX_DEV_SHELL=1
          '';
        };
      });
    };
}
