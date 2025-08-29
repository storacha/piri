#!/bin/bash
set -euo pipefail

BRANCH="$1"

echo "Installing Piri from branch: $BRANCH"

# Install build dependencies
echo "Installing build dependencies..."
sudo apt-get update
sudo apt-get install -y git build-essential

# Install Go if not present
if ! command -v go &> /dev/null; then
    echo "Installing Go..."
    curl -L https://go.dev/dl/go1.21.5.linux-amd64.tar.gz | sudo tar -C /usr/local -xzf -
fi

# Set up Go environment
export PATH=/usr/local/go/bin:$PATH
export HOME=/home/ubuntu
export GOPATH=/home/ubuntu/go
export GOMODCACHE=/home/ubuntu/go/pkg/mod
export GOCACHE=/home/ubuntu/.cache/go-build
export GO111MODULE=on

# Create necessary directories
mkdir -p /home/ubuntu/go/pkg/mod
mkdir -p /home/ubuntu/.cache/go-build

# Create temp directory
TMP_DIR=$(mktemp -d)
cd "$TMP_DIR"

# Clone repository
echo "Cloning repository..."
git clone https://github.com/storacha/piri.git
cd piri

# Checkout branch
echo "Checking out branch: $BRANCH"
git checkout "$BRANCH"

# Build binary
echo "Building Piri..."
make

# Install binary
sudo mv piri /usr/local/bin/piri
sudo chmod +x /usr/local/bin/piri

# Clean up
cd /
rm -rf "$TMP_DIR"

echo "Piri from branch $BRANCH installed successfully"
/usr/local/bin/piri version