{
  description = "Easy GitHub repository management CLI";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
      ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
      versionFromFile = nixpkgs.lib.strings.removeSuffix "\n" (
        builtins.readFile ./internal/version/VERSION
      );
      releaseNotesLines = nixpkgs.lib.splitString "\n" (builtins.readFile ./RELEASE_NOTES.md);
      firstReleaseHeading = nixpkgs.lib.findFirst (
        line: nixpkgs.lib.hasPrefix "## " line
      ) "" releaseNotesLines;
      releaseHeadingMatch = builtins.match "^## ([0-9]+\\.[0-9]+\\.[0-9]+(-[0-9A-Za-z.-]+)?(\\+[0-9A-Za-z.-]+)?) - .*$" firstReleaseHeading;
      versionFromReleaseNotes =
        if releaseHeadingMatch != null then builtins.elemAt releaseHeadingMatch 0 else null;
      version =
        if self ? rev && versionFromReleaseNotes != null then versionFromReleaseNotes else versionFromFile;
    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        rec {
          ezgit = pkgs.buildGoModule {
            pname = "ezgit";
            inherit version;
            src = self;
            vendorHash = "sha256-sq+Q0x1+MAoH/0X0cK3cil/JEYLKk4C/R/nvUGE+F0Y=";
            ldflags = [
              "-X github.com/kirksw/ezgit/internal/version.Value=${version}"
            ];
            nativeCheckInputs = [ pkgs.git ];
            env.GOWORK = "off";
            meta = {
              description = "Easy GitHub repository management CLI";
              homepage = "https://github.com/kirksw/ezgit";
              license = pkgs.lib.licenses.mit;
              mainProgram = "ezgit";
              platforms = systems;
            };
          };
          default = ezgit;
        }
      );

      apps = forAllSystems (system: {
        ezgit = {
          type = "app";
          program = "${self.packages.${system}.ezgit}/bin/ezgit";
        };
        default = self.apps.${system}.ezgit;
      });

      devShells = forAllSystems (system: {
        default = nixpkgs.legacyPackages.${system}.mkShell {
          packages = with nixpkgs.legacyPackages.${system}; [
            go
            gopls
            nixfmt-rfc-style
          ];
        };
      });
    };
}
