#!/bin/bash
# Quick script to verify the mock server is working correctly

set -e

echo "Verifying mock GitHub API server..."
echo "===================================="
echo ""

# Test the release endpoint
echo "1. Testing /repos/storacha/piri/releases/latest endpoint:"
RESPONSE=$(curl -s http://localhost:8080/repos/storacha/piri/releases/latest)
echo "$RESPONSE" | jq '.'

VERSION=$(echo "$RESPONSE" | jq -r '.tag_name')
echo ""
echo "   ✓ Advertised version: $VERSION"
echo ""

# Test downloading the binary
echo "2. Testing binary download:"
curl -s http://localhost:8080/download/piri_linux_amd64.tar.gz -o /tmp/test_piri.tar.gz
SIZE=$(stat -f%z /tmp/test_piri.tar.gz 2>/dev/null || stat -c%s /tmp/test_piri.tar.gz 2>/dev/null)
echo "   ✓ Downloaded piri_linux_amd64.tar.gz ($SIZE bytes)"

# Extract and check the binary
cd /tmp
tar -xzf test_piri.tar.gz
if [ -f piri ]; then
    echo "   ✓ Successfully extracted piri binary"
    rm -f piri test_piri.tar.gz
else
    echo "   ✗ Failed to extract piri binary"
    exit 1
fi
echo ""

# Test checksums
echo "3. Testing checksums.txt:"
CHECKSUMS=$(curl -s http://localhost:8080/download/checksums.txt)
echo "$CHECKSUMS"
echo ""

# Verify checksum format
if echo "$CHECKSUMS" | grep -q "piri_linux_amd64.tar.gz"; then
    echo "   ✓ Checksums file contains expected entries"
else
    echo "   ✗ Checksums file missing expected entries"
    exit 1
fi
echo ""

echo "✅ All mock server endpoints working correctly!"