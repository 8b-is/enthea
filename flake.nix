{
  description = "enthea — the deepsiper-enthea engine entry (pure-stdlib Go)";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "aarch64-darwin" "x86_64-darwin" ];
      forAll = nixpkgs.lib.genAttrs systems;
    in
    {
      packages = forAll (system:
        let pkgs = nixpkgs.legacyPackages.${system};
        in {
          default = pkgs.buildGoModule {
            pname = "enthea";
            version = "0.1.0";
            src = self;
            # zero external modules — pure stdlib, so no vendor hash is needed
            vendorHash = null;
            ldflags = [ "-s" "-w" ];
          };
        });

      apps = forAll (system: {
        default = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/enthea";
        };
      });

      # `nix develop` drops you into a shell with Go + a static build path.
      devShells = forAll (system:
        let pkgs = nixpkgs.legacyPackages.${system};
        in {
          default = pkgs.mkShell {
            packages = [ pkgs.go ];
          };
        });
    };
}
