#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Piri Update Mechanism Test Script${NC}"
echo "======================================"
echo ""

# Check if we're in the right directory
if [ ! -f "../go.mod" ]; then
    echo -e "${RED}Error: Run this script from the test/ directory${NC}"
    exit 1
fi

# Build the current piri binary
echo -e "${YELLOW}Building piri binary...${NC}"
(cd .. && go build -o test/piri ./cmd/cli)

# Build the mock server
echo -e "${YELLOW}Building mock GitHub server...${NC}"
go build -o mock_github_server mock_github_server.go

# Start the mock server in background
echo -e "${YELLOW}Starting mock GitHub server...${NC}"
./mock_github_server --binary-path ./piri --advertised-version v99.99.99 &
SERVER_PID=$!

# Give server time to start
sleep 2

# Function to cleanup on exit
cleanup() {
    echo -e "${YELLOW}Cleaning up...${NC}"
    kill $SERVER_PID 2>/dev/null || true
    rm -f piri mock_github_server
}
trap cleanup EXIT

echo -e "${GREEN}Mock server running on PID $SERVER_PID${NC}"
echo ""

# Test the API endpoint
echo -e "${YELLOW}Testing API endpoint...${NC}"
curl -s http://localhost:8080/repos/storacha/piri/releases/latest | jq '.tag_name, .assets[].name'
echo ""

# Show how to test with piri
echo -e "${GREEN}Server is ready for testing!${NC}"
echo ""
echo "To test the update mechanism:"
echo ""
echo "1. For manual update testing (will be blocked for managed installs):"
echo "   export PIRI_GITHUB_API_URL=http://localhost:8080"
echo "   ./piri update"
echo ""
echo "2. For auto-update testing (managed installations):"
echo "   export PIRI_GITHUB_API_URL=http://localhost:8080"
echo "   sudo ./piri update-internal"
echo ""
echo "3. To test with an installed service:"
echo "   - Install piri normally: sudo ./piri install --config <config>"
echo "   - Modify /opt/piri/systemd/piri-updater.service to include:"
echo "     Environment=\"PIRI_GITHUB_API_URL=http://localhost:8080\""
echo "   - Reload and trigger: sudo systemctl daemon-reload && sudo systemctl start piri-updater"
echo ""
echo "Press Ctrl+C to stop the mock server"

# Wait for user to stop
wait $SERVER_PID