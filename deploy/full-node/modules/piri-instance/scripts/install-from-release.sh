#!/bin/bash
set -euo pipefail

VERSION="$1"

echo "Installing Piri from release version: $VERSION"

# Create temp directory
TMP_DIR=$(mktemp -d)
cd "$TMP_DIR"

# Download release
DOWNLOAD_URL="https://github.com/storacha/piri/releases/download/${VERSION}/piri_${VERSION#v}_linux_amd64.tar.gz"
echo "Downloading from: $DOWNLOAD_URL"
curl -L -o piri.tar.gz "$DOWNLOAD_URL"

# Extract binary
tar -xzf piri.tar.gz

# Install binary
sudo mv piri /usr/local/bin/piri
sudo chmod +x /usr/local/bin/piri

# Clean up
cd /
rm -rf "$TMP_DIR"

export HOME=/home/ubuntu
echo "Piri version $VERSION installed successfully"
/usr/local/bin/piri version
