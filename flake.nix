{
  description = "Cursed GIT: like git, but she has feelings";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      # nix run github:spiritpaper/cursed-git -- version
      # nix profile install github:spiritpaper/cursed-git
      packages = forAllSystems (pkgs: rec {
        cg = pkgs.buildGoModule {
          pname = "cursed-git";
          version = "0.3.0";
          src = self;
          vendorHash = null; # no dependencies
          subPackages = [ "cmd/cg" ];
          ldflags = [
            "-s" "-w"
            "-X github.com/spiritpaper/cursed-git/internal/cli.Version=v0.3.0-nix+${self.shortRev or "dirty"}"
          ];
          nativeBuildInputs = [ pkgs.makeWrapper ];
          # cg hands most commands to git: make sure there is one
          postInstall = ''
            wrapProgram $out/bin/cg --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.git ]}
          '';
          # the test suite runs real git in temp folders; CI runs it on every push
          doCheck = false;
          meta = {
            description = "Like git, but she has feelings: a git-learning game you play on a real project";
            homepage = "https://github.com/spiritpaper/cursed-git";
            license = pkgs.lib.licenses.mit;
            mainProgram = "cg";
          };
        };
        default = cg;
      });
    };
}
