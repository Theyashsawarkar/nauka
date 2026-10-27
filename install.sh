#!/usr/bin/env bash
# Install nauka for the current user, no root needed:
#
#   curl -fsSL https://raw.githubusercontent.com/Theyashsawarkar/nauka/main/install.sh | bash
#
# Downloads the release binary for this machine into ~/.local/share/nauka
# (or builds it with Go when there is none) and links ~/.local/bin/nauka.
# Run it again to update.
#
#   install.sh [--bin-dir DIR] [--dir DIR] [--ref TAG] [--build] [--uninstall]
set -euo pipefail

REPO=Theyashsawarkar/nauka
DIR=${XDG_DATA_HOME:-$HOME/.local/share}/nauka
BIN_DIR=$HOME/.local/bin
REF=latest
BUILD=0
UNINSTALL=0

while [ $# -gt 0 ]; do
  case $1 in
    --bin-dir) BIN_DIR=$2; shift 2 ;;
    --dir) DIR=$2; shift 2 ;;
    --ref) REF=$2; shift 2 ;;
    --build) BUILD=1; shift ;;
    --uninstall) UNINSTALL=1; shift ;;
    -h | --help) sed -n '2,10p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "install.sh: unknown option $1" >&2; exit 2 ;;
  esac
done

say() { printf '\033[1;38;2;203;166;247m==>\033[0m %s\n' "$*"; }

if [ "$UNINSTALL" = 1 ]; then
  rm -f "$BIN_DIR/nauka"
  rm -rf "$DIR"
  say "Removed nauka ($BIN_DIR/nauka, $DIR)."
  exit 0
fi

mkdir -p "$DIR" "$BIN_DIR"
case $(uname -m) in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) arch="" ;;
esac

got=0
if [ "$BUILD" = 0 ] && [ -n "$arch" ] && command -v curl >/dev/null; then
  if [ "$REF" = latest ]; then
    url="https://github.com/$REPO/releases/latest/download/nauka-linux-$arch"
  else
    url="https://github.com/$REPO/releases/download/$REF/nauka-linux-$arch"
  fi
  say "Downloading $url"
  if curl -fsSL -o "$DIR/nauka.new" "$url"; then
    chmod +x "$DIR/nauka.new" && mv "$DIR/nauka.new" "$DIR/nauka" && got=1
  else
    rm -f "$DIR/nauka.new"
    say "No release binary; building from source instead."
  fi
fi

if [ "$got" = 0 ]; then
  command -v go >/dev/null || { echo "install.sh: no release binary for this machine, and Go is needed to build one" >&2; exit 1; }
  command -v git >/dev/null || { echo "install.sh: git is needed to build nauka" >&2; exit 1; }
  src="$DIR/src"
  if [ -d "$src/.git" ]; then
    git -C "$src" fetch --quiet --tags origin
  else
    git clone --quiet "https://github.com/$REPO.git" "$src"
  fi
  if [ "$REF" = latest ]; then git -C "$src" checkout --quiet main && git -C "$src" pull --quiet --ff-only; else git -C "$src" checkout --quiet "$REF"; fi
  say "Building nauka"
  (cd "$src" && go build -trimpath -o "$DIR/nauka" ./cmd/nauka)
fi

ln -sf "$DIR/nauka" "$BIN_DIR/nauka"
say "Linked $BIN_DIR/nauka -> $DIR/nauka"

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) say "$BIN_DIR isn't on your PATH yet. Add this to your shell's rc file:"
     echo "    export PATH=\"$BIN_DIR:\$PATH\"" ;;
esac

say "$("$DIR/nauka" --version) installed. Run nauka, or add the tmux plugin (prefix D opens it):"
echo "    set -g @plugin 'Theyashsawarkar/nauka'"
