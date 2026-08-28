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
          packages = with pkgs; [
            go_1_26
            zig
            fyne
            gnumake
            pkg-config
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
