{
  description = "Go CLI application that performs health checks and stores IPs of authenticated machines over TLS";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.05";
    nixpkgs-unstable.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, nixpkgs-unstable, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs-unstable { inherit system; };
        #pkgs-unstable = import nixpkgs-unstable { inherit system; };
      in
      {
        devShell = pkgs.mkShell {
          packages = [
            pkgs.go
            pkgs.openssl
            pkgs.oci-cli
            pkgs.gcc13
            pkgs.sqlite
            pkgs.sqlite-utils
          ];

          inputsFrom = [
            pkgs.go
          ];

          shellHook = ''
            # CGo dep
            CGO_ENABLED=1
          '';


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
