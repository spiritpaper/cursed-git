#!/bin/sh
# Cursed GIT installer for Linux and macOS:
#
#   curl -fsSL https://raw.githubusercontent.com/spiritpaper/cursed-git/main/install.sh | sh
#
# Downloads the latest release for your system, checks it against the release's checksums,
# and puts `cg` in ~/.local/bin (or $CG_INSTALL_DIR). Pick a version with CG_VERSION=v0.1.0.
set -eu

repo="spiritpaper/cursed-git"
dir="${CG_INSTALL_DIR:-$HOME/.local/bin}"

say() { printf '%s\n' "$*"; }
die() { printf 'cg install: %s\n' "$*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || die "this needs $1"; }

need curl
need tar
need uname

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) die "unsupported system $(uname -s) (on Windows, use install.ps1)" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) die "unsupported CPU $(uname -m)" ;;
esac

version="${CG_VERSION:-}"
if [ -z "$version" ]; then
  # the latest release's tag, from where /releases/latest redirects to
  version=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$repo/releases/latest" | sed 's#.*/tag/##')
  case "$version" in v*) ;; *) die "couldn't find the latest release" ;; esac
fi

name="cg-$version-$os-$arch"
base="https://github.com/$repo/releases/download/$version"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

say "Downloading Cursed GIT $version for $os/$arch..."
curl -fsSL -o "$tmp/$name.tar.gz" "$base/$name.tar.gz" || die "download failed: $base/$name.tar.gz"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" || die "couldn't download the checksums"

want=$(grep " $name.tar.gz\$" "$tmp/checksums.txt" | cut -d' ' -f1)
if command -v sha256sum >/dev/null 2>&1; then
  got=$(sha256sum "$tmp/$name.tar.gz" | cut -d' ' -f1)
else
  got=$(shasum -a 256 "$tmp/$name.tar.gz" | cut -d' ' -f1)
fi
[ -n "$want" ] && [ "$got" = "$want" ] || die "checksum mismatch: the download isn't what was released. Nothing was installed."

tar -xzf "$tmp/$name.tar.gz" -C "$tmp"
mkdir -p "$dir"
mv "$tmp/$name/cg" "$dir/cg"
chmod 755 "$dir/cg"
say "Installed: $dir/cg ($("$dir/cg" version | cut -d' ' -f3))"

case ":$PATH:" in
  *":$dir:"*) ;;
  *) say ""
     say "$dir isn't on your PATH yet. Add this to your shell's rc file (~/.bashrc, ~/.zshrc):"
     say "  export PATH=\"$dir:\$PATH\"" ;;
esac
if ! command -v git >/dev/null 2>&1; then
  say ""
  say "cg hands most commands to git, and git isn't installed: https://git-scm.com/downloads"
fi
say ""
say "Start a game:  cg init        (Miranda)    or    cg init --preset gentle    (Juniper)"
