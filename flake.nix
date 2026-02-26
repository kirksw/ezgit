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
        versionFromFile = pkgs.lib.strings.removeSuffix "\n" (builtins.readFile ./internal/version/VERSION);
        releaseNotesLines = pkgs.lib.splitString "\n" (builtins.readFile ./RELEASE_NOTES.md);
        firstReleaseHeading = pkgs.lib.findFirst (line: pkgs.lib.hasPrefix "## " line) "" releaseNotesLines;
        releaseHeadingMatch = builtins.match "^## ([0-9]+\\.[0-9]+\\.[0-9]+(-[0-9A-Za-z.-]+)?(\\+[0-9A-Za-z.-]+)?) - .*$" firstReleaseHeading;
        versionFromReleaseNotes =
          if releaseHeadingMatch != null
          then builtins.elemAt releaseHeadingMatch 0
          else null;
        buildVersion =
          if self ? rev && versionFromReleaseNotes != null
          then versionFromReleaseNotes
          else versionFromFile;
      in {
        packages.default = pkgs.buildGoModule {
          pname = "ezgit";
          version = buildVersion;
          src = pkgs.lib.cleanSource ./.;
          vendorHash = "sha256-sq+Q0x1+MAoH/0X0cK3cil/JEYLKk4C/R/nvUGE+F0Y=";
          ldflags = [
            "-X github.com/kirksw/ezgit/internal/version.Value=${buildVersion}"
          ];
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
