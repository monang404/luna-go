#!/bin/sh
set -e

REPO="monang404/luna-go"
BIN_NAME="luna"
INSTALL_DIR="${LUNA_INSTALL_DIR:-$HOME/.local/bin}"

# Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  linux|darwin) ;;
  *) echo "OS $OS is not supported by this script yet."; exit 1 ;;
esac

# Detect Architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="x86_64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "Architecture $ARCH is not supported by this script yet."; exit 1 ;;
esac

echo "=> Fetching latest release of $BIN_NAME for $OS/$ARCH..."

# Get latest release tag
LATEST_TAG=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_TAG" ]; then
  echo "Error: Could not fetch latest release tag."
  exit 1
fi

echo "=> Latest release is $LATEST_TAG"

# Construct download URL
FILENAME="luna-go_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/$REPO/releases/download/$LATEST_TAG/$FILENAME"
TMP_DIR=$(mktemp -d)

echo "=> Downloading $DOWNLOAD_URL..."
curl -L -o "$TMP_DIR/$FILENAME" "$DOWNLOAD_URL"

echo "=> Extracting to $INSTALL_DIR..."
mkdir -p "$INSTALL_DIR"
tar -xzf "$TMP_DIR/$FILENAME" -C "$TMP_DIR"
mv "$TMP_DIR/$BIN_NAME" "$INSTALL_DIR/"

chmod +x "$INSTALL_DIR/$BIN_NAME"
rm -rf "$TMP_DIR"

echo "=> Done! $BIN_NAME has been installed to $INSTALL_DIR."
echo "=> Please ensure $INSTALL_DIR is in your PATH."
