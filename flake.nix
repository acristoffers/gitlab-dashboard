{
  description = "A focused GitLab merge request dashboard";

  inputs =
    {
      nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

      flake-utils.url = "github:numtide/flake-utils";

      gitignore.url = "github:hercules-ci/gitignore.nix";
      gitignore.inputs.nixpkgs.follows = "nixpkgs";
    };

  outputs = inputs:
    let
      inherit (inputs) nixpkgs gitignore flake-utils;
      inherit (gitignore.lib) gitignoreSource;
    in
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      rec {
        formatter = pkgs.nixpkgs-fmt;
        packages.default = packages.gitlab-dashboard;
        packages.gitlab-dashboard = pkgs.buildGoModule {
          pname = "gitlab-dashboard";
          version = "0.1.0";
          src = gitignoreSource ./.;
          vendorHash = null; # no third-party Go modules
          ldflags = [ "-s" "-w" ];
          meta.mainProgram = "gitlab-dashboard";
          meta.license = pkgs.lib.licenses.gpl3Plus;
        };
        apps = rec {
          gitlab-dashboard = { type = "app"; program = "${packages.gitlab-dashboard}/bin/gitlab-dashboard"; };
          default = gitlab-dashboard;
        };
        devShell = pkgs.mkShell {
          packages = with pkgs;[ packages.gitlab-dashboard go gopls glab git man busybox ];
        };
      }
    );
}
