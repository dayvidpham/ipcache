{
  description = "Nix Develop Scaffolder";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.05";
    nixpkgs-unstable.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, nixpkgs-unstable, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let pkgs = import nixpkgs { inherit system; };
      in
      {
        devShell = pkgs.mkShell {
          packages = [
            pkgs.go
            pkgs.openssl
            pkgs.oci-cli
          ];

          inputsFrom = [
            pkgs.go
          ];


          #nativeBuildInputs = [
          #  pkgs.stdenv
          #];

          #buildInputs = [
          #  pkgs.go
          #  pkgs.gotools
          #  pkgs.golangci-lint
          #  pkgs.gopls
          #  pkgs.go-outline
          #  pkgs.gopkgs
          #  pkgs.godef
          #  pkgs.gocode-gomod
          #  pkgs.gocode
          #];
        };

        #package = pkgs.buildGoModule {
        #  src = ./.;
        #};
      });
}
