# Piri Update Mechanism Testing

This directory contains tools for testing the piri update mechanism without hitting the real GitHub API.

## Mock GitHub API Server

The `mock_github_server.go` implements a fake GitHub Releases API that serves a configurable piri binary while claiming it's a different version. This creates a perpetual update condition perfect for testing.

### How It Works

1. **Version Mismatch**: The server advertises a different version (e.g., v99.99.99) than what the binary reports internally (e.g., v0.0.13)
2. **Same Binary**: Serves the same binary you're testing, but the updater thinks it's getting a new version
3. **Continuous Testing**: The update mechanism will always see an available update, allowing repeated testing

### Building

```bash
cd test/
go build -o mock_github_server mock_github_server.go
```

### Running the Mock Server

```bash
# Serve the current piri binary as "v99.99.99"
./mock_github_server --binary-path ../piri --advertised-version v99.99.99

# Custom port
./mock_github_server --binary-path ../piri --advertised-version v99.99.99 --port 9090
```

### Testing Updates

The piri update commands check the `PIRI_GITHUB_API_URL` environment variable:

```bash
# Set the test server URL
export PIRI_GITHUB_API_URL=http://localhost:8080

# Test manual update (will be blocked for managed installations)
piri update

# Test auto-update (for managed installations)
sudo piri update-internal
```

## Test Script

The `test_update.sh` script automates the setup:

```bash
./test_update.sh
```

This script:
1. Builds the piri binary
2. Builds and starts the mock server
3. Shows how to test different update scenarios

## Testing Scenarios

### 1. Standalone Binary Update
Test updating a standalone piri binary (not installed as a service):

```bash
export PIRI_GITHUB_API_URL=http://localhost:8080
./piri update
```

### 2. Managed Installation Update
Test the auto-update mechanism for managed installations:

```bash
# First install piri normally
sudo piri install --config piri-config.toml

# Then test auto-update with mock server
export PIRI_GITHUB_API_URL=http://localhost:8080
sudo piri update-internal
```

### 3. Continuous Update Testing
To test the update timer continuously:

1. Install piri with auto-updates enabled
2. Edit `/opt/piri/systemd/piri-updater.service` to add:
   ```
   [Service]
   Environment="PIRI_GITHUB_API_URL=http://localhost:8080"
   ```
3. Reload systemd: `sudo systemctl daemon-reload`
4. Start the timer: `sudo systemctl start piri-updater.timer`

This will trigger updates every 30 minutes, creating new version directories each time.

## What Gets Tested

- ✅ Version comparison logic
- ✅ Binary download and verification
- ✅ Checksum validation
- ✅ Tar.gz extraction
- ✅ Versioned directory creation (`/opt/piri/bin/v*/`)
- ✅ Symlink updates (`/opt/piri/bin/current`)
- ✅ Service restart logic
- ✅ Permission handling
- ✅ Update safety checks (not during proof generation)

## Monitoring Updates

Watch the version directories accumulate:
```bash
watch -n 1 'ls -la /opt/piri/bin/'
```

Check update logs:
```bash
journalctl -u piri-updater -f
```

## Cleanup

After testing, you may want to clean up old version directories:
```bash
# Remove all but current version
cd /opt/piri/bin/
sudo rm -rf v99.99.99/
```