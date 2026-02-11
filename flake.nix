{
  description = "Easy GitHub repository management CLI";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in {
        packages.default = pkgs.buildGoModule {
          pname = "ezgit";
          version = pkgs.lib.strings.removeSuffix "\n" (builtins.readFile ./internal/version/VERSION);
          src = pkgs.lib.cleanSource ./.;
          vendorHash = "sha256-sq+Q0x1+MAoH/0X0cK3cil/JEYLKk4C/R/nvUGE+F0Y=";
          nativeCheckInputs = with pkgs; [ git ];
        };

        apps.default = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/ezgit";
          meta = {
            description = "Easy GitHub repository management CLI";
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [ go ];
        };
      });
}
