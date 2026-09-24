#!/bin/sh
set -e

# jcore installer script
# Installs the latest release of jcore to ~/.local/bin/jcore and configures shell completions.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/JCO-Digital/jcore-cli/main/install.sh | sh

OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
  Linux)
    OS_TYPE="linux"
    ;;
  Darwin)
    OS_TYPE="darwin"
    ;;
  *)
    echo "Error: jcore prebuilt binaries currently support Linux (including WSL) and macOS (Darwin). Detected OS: $OS"
    echo "For other operating systems, please build from source: https://github.com/JCO-Digital/jcore-cli#option-b-build-from-source"
    exit 1
    ;;
esac

case "$ARCH" in
  x86_64|amd64)
    ARCH_TYPE="amd64"
    ;;
  arm64|aarch64)
    ARCH_TYPE="arm64"
    ;;
  *)
    echo "Error: Unsupported architecture: $ARCH"
    echo "For other architectures, please build from source: https://github.com/JCO-Digital/jcore-cli#option-b-build-from-source"
    exit 1
    ;;
esac

if [ -z "$JCORE_RELEASE_URL" ]; then
  if [ "$OS_TYPE" = "linux" ] && [ "$ARCH_TYPE" != "amd64" ]; then
    echo "Error: jcore prebuilt binaries for Linux currently support x86_64 architecture. Detected architecture: $ARCH"
    echo "For other architectures, please build from source: https://github.com/JCO-Digital/jcore-cli#option-b-build-from-source"
    exit 1
  fi

  if [ "$OS_TYPE" = "darwin" ] && [ "$ARCH_TYPE" != "arm64" ]; then
    echo "Error: jcore prebuilt binaries for macOS currently support Apple Silicon (arm64). Detected architecture: $ARCH"
    echo "For other architectures, please build from source: https://github.com/JCO-Digital/jcore-cli#option-b-build-from-source"
    exit 1
  fi
fi

# Check for download tool (curl or wget)
if command -v curl >/dev/null 2>&1; then
  DOWNLOADER="curl"
elif command -v wget >/dev/null 2>&1; then
  DOWNLOADER="wget"
else
  echo "Error: Neither curl nor wget was found. Please install curl or wget to continue."
  exit 1
fi

download_file() {
  url="$1"
  dest="$2"
  case "$url" in
    file://*|/*)
      cp "${url#file://}" "$dest"
      ;;
    *)
      if [ "$DOWNLOADER" = "curl" ]; then
        curl -fsSL "$url" -o "$dest"
      else
        wget -qO "$dest" "$url"
      fi
      ;;
  esac
}

ASSET_NAME="jcore_${OS_TYPE}_${ARCH_TYPE}"
INSTALL_DIR="${JCORE_INSTALL_DIR:-${HOME}/.local/bin}"
mkdir -p "$INSTALL_DIR"
TARGET="$INSTALL_DIR/jcore"
TMP_TARGET="$INSTALL_DIR/.jcore.tmp.$$"

# Download latest jcore release
RELEASE_URL="${JCORE_RELEASE_URL:-https://github.com/JCO-Digital/jcore-cli/releases/latest/download/${ASSET_NAME}}"
echo "Downloading latest jcore release from GitHub..."
if ! download_file "$RELEASE_URL" "$TMP_TARGET"; then
  echo "Error: Failed to download jcore from $RELEASE_URL"
  rm -f "$TMP_TARGET"
  exit 1
fi

chmod 755 "$TMP_TARGET"
mv -f "$TMP_TARGET" "$TARGET"
echo "Installed jcore to $TARGET"

# Check for old installations elsewhere in PATH
echo "Checking for old installations of jcore elsewhere..."

RESOLVED_TARGET="$TARGET"
if command -v readlink >/dev/null 2>&1; then
  RESOLVED_TARGET="$(readlink -f "$TARGET" 2>/dev/null || echo "$TARGET")"
fi

# Check what 'which jcore' finds
WHICH_DIR=""
if command -v which >/dev/null 2>&1; then
  WHICH_PATH="$(which jcore 2>/dev/null || true)"
  [ -n "$WHICH_PATH" ] && WHICH_DIR="$(dirname "$WHICH_PATH")"
fi
if [ -z "$WHICH_DIR" ] && command -v jcore >/dev/null 2>&1; then
  WHICH_PATH="$(command -v jcore 2>/dev/null || true)"
  [ -n "$WHICH_PATH" ] && WHICH_DIR="$(dirname "$WHICH_PATH")"
fi

SEARCH_DIRS="${WHICH_DIR:+${WHICH_DIR}:}$PATH:/usr/local/bin:/usr/bin:/bin:$HOME/bin"
OLD_IFS="$IFS"
IFS=':'
FOUND_CONFLICT=0
SEEN_PATHS=""

for dir in $SEARCH_DIRS; do
  [ -z "$dir" ] && continue
  candidate="$dir/jcore"

  if [ -f "$candidate" ] || [ -L "$candidate" ]; then
    RESOLVED_CANDIDATE="$candidate"
    if command -v readlink >/dev/null 2>&1; then
      RESOLVED_CANDIDATE="$(readlink -f "$candidate" 2>/dev/null || echo "$candidate")"
    fi

    # Skip the freshly installed target
    if [ "$RESOLVED_CANDIDATE" = "$RESOLVED_TARGET" ] || [ "$candidate" = "$TARGET" ]; then
      continue
    fi

    # Deduplicate candidate paths
    case ":$SEEN_PATHS:" in
      *":$candidate:"*) continue ;;
    esac
    SEEN_PATHS="${SEEN_PATHS}:${candidate}"

    if [ -w "$candidate" ]; then
      echo "  Removing older installation at $candidate (user-writable)..."
      rm -f "$candidate"
      echo "  Removed $candidate"
    else
      echo "  Notice: Existing installation found at $candidate that is not user-writable."
      echo "    Because $dir may appear before $INSTALL_DIR in PATH, your shell may run the old version."
      echo "    To remove it, run:"
      echo "      sudo rm -f \"$candidate\""
      FOUND_CONFLICT=1
    fi
  fi
done
IFS="$OLD_IFS"

# Install shell completions
echo "Installing shell completions..."

XDG_DATA="${XDG_DATA_HOME:-$HOME/.local/share}"
XDG_CONFIG="${XDG_CONFIG_HOME:-$HOME/.config}"

# Bash completion
BASH_COMP_DIR="$XDG_DATA/bash-completion/completions"
mkdir -p "$BASH_COMP_DIR"
if "$TARGET" completion bash > "$BASH_COMP_DIR/jcore" 2>/dev/null; then
  echo "  Bash: installed to $BASH_COMP_DIR/jcore"
fi

# Zsh completion
ZSH_COMP_DIR="$XDG_DATA/zsh/site-functions"
mkdir -p "$ZSH_COMP_DIR"
if "$TARGET" completion zsh > "$ZSH_COMP_DIR/_jcore" 2>/dev/null; then
  echo "  Zsh: installed to $ZSH_COMP_DIR/_jcore"
fi
if [ -d "$HOME/.zfunc" ]; then
  "$TARGET" completion zsh > "$HOME/.zfunc/_jcore" 2>/dev/null || true
  echo "  Zsh: installed to $HOME/.zfunc/_jcore"
fi

# Fish completion
FISH_CONFIG_DIR="$XDG_CONFIG/fish/completions"
mkdir -p "$FISH_CONFIG_DIR"
if "$TARGET" completion fish > "$FISH_CONFIG_DIR/jcore.fish" 2>/dev/null; then
  echo "  Fish: installed to $FISH_CONFIG_DIR/jcore.fish"
fi

FISH_VENDOR_DIR="$XDG_DATA/fish/vendor_completions.d"
mkdir -p "$FISH_VENDOR_DIR"
if "$TARGET" completion fish > "$FISH_VENDOR_DIR/jcore.fish" 2>/dev/null; then
  echo "  Fish: installed to $FISH_VENDOR_DIR/jcore.fish"
fi

# Check if install directory is in PATH
case ":$PATH:" in
  *":$INSTALL_DIR:"*)
    ;;
  *)
    echo ""
    echo "Note: $INSTALL_DIR is not currently in your PATH."
    echo "To run 'jcore' directly, add it to your PATH by adding this line to your shell configuration:"
    if [ "$INSTALL_DIR" = "$HOME/.local/bin" ]; then
      echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
      echo ""
      echo "For Bash (common on WSL and Ubuntu):"
      echo "  echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.bashrc && source ~/.bashrc"
      echo "For Zsh:"
      echo "  echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.zshrc && source ~/.zshrc"
      echo "For Fish:"
      echo "  fish_add_path ~/.local/bin"
    else
      echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
      echo ""
      echo "For Bash (common on WSL and Ubuntu):"
      echo "  echo 'export PATH=\"$INSTALL_DIR:\$PATH\"' >> ~/.bashrc && source ~/.bashrc"
      echo "For Zsh:"
      echo "  echo 'export PATH=\"$INSTALL_DIR:\$PATH\"' >> ~/.zshrc && source ~/.zshrc"
      echo "For Fish:"
      echo "  fish_add_path $INSTALL_DIR"
    fi
    ;;
esac

echo ""
echo "Installation complete! Run 'jcore version' to get started."
