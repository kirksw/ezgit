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
      in
      {
        packages = {
          default = pkgs.buildGoModule {
            pname = "ezgit";
            version = "0.1.0";
            src = pkgs.lib.cleanSource ./.;
            vendorHash = "sha256-4T3ZIrEahZdZDe94Yl4AyvJnQyBHxxvKMgI0iIdrf0g=";

            ldflags = [
              "-s"
              "-w"
              "-X main.version=0.1.0"
            ];

            nativeBuildInputs = with pkgs; [ installShellFiles ];

            postPatch = ''
              # Remove any vendor directory if it exists
              rm -rf vendor
            '';

            postInstall = ''
              # Install shell completions
              $out/bin/ezgit completion bash > ezgit.bash || true
              $out/bin/ezgit completion fish > ezgit.fish || true
              $out/bin/ezgit completion zsh > ezgit.zsh || true
              installShellCompletion ezgit.{bash,fish,zsh}

              # Install launchd service plist
              mkdir -p $out/Library/LaunchAgents
              substituteAll ${./nix/com.github.kirksw.ezgit.plist} $out/Library/LaunchAgents/com.github.kirksw.ezgit.plist

              # Install service manager script
              substituteAll ${./nix/ezgit-service.sh} $out/bin/ezgit-service
              chmod +x $out/bin/ezgit-service
            '';

            meta = with pkgs.lib; {
              description = "A powerful CLI tool for managing GitHub repositories";
              homepage = "https://github.com/kirksw/ezgit";
              license = licenses.mit;
              mainProgram = "ezgit";
            };
          };
        };

        apps = {
          default = {
            type = "app";
            program = "${self.packages.${system}.default}/bin/ezgit";
          };
        };

        devShells = {
          default = pkgs.mkShell {
            buildInputs = with pkgs; [
              go
              gopls
              gotools
            ];
          };
        };
      }
    );
}
