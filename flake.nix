{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.05";
    nixpkgs-unstable.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = inputs@{ self, nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        buildInputs = with pkgs; [
          # Packages required at runtime
        ];

        nativeBuildInputs = with pkgs; [
          # Packages only required at build time
          go
        ];

      in
      {
        devShells.default = pkgs.mkShell {
          inherit buildInputs;
          shellHook = ''
            echo "go `go --version`"
            #export PATH="$(pwd)/node_modules/.bin:$PATH"
          '';
        };

        #packages.default = pkgs.buildNpmPackage {
        #  pname = "dhpham-website";
        #  version = "0.0.1";
        #  src = ./.;
        #  npmDepsHash = "sha256-T/sR+XTIF+R0UAfFqsmoy//dvrv2KOV7HR1fiQ327k8=";

        #  inherit buildInputs;
        #  npmPackFlags = [ "--ignore-scripts" ];
        #};
      });
}
