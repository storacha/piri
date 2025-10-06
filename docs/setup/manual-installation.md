# Manual Installation

This guide shows you how to manually install and run Piri without systemd service management.

## Overview

Manual installation gives you full control over the Piri binary and how it runs, but requires you to:
- Manually start/stop Piri
- Manually update to new versions
- Set up your own service management (if desired)

**Note:** For production nodes, we recommend [Service Installation](./service-installation.md) instead, which provides automatic updates and reliability.

## Prerequisites

Before manual installation, ensure you have:

- ✅ [Downloaded Piri binary](./download.md) (should be `./piri` in your current directory)
- ✅ [Configuration file from initialization](./initialization.md) (`config.toml`)

## Installation

### Step 1: Move Binary to System Path

Move the Piri binary to a location in your PATH:

```bash
# Move to /usr/local/bin (recommended)
sudo mv piri /usr/local/bin/piri

# Make sure it's executable
sudo chmod +x /usr/local/bin/piri
```

### Step 2: Verify Installation

```bash
# Verify piri is accessible
piri version
```

### Step 3: Store Your Configuration

You can store your configuration file in several locations:

**Option A: User config directory (recommended)**
```bash
# Piri automatically loads from here
mkdir -p ~/.config/piri
cp config.toml ~/.config/piri/config.toml
```

**Option B: Custom location**
```bash
# Store anywhere, but you'll need to specify --config flag
mkdir -p /etc/piri
sudo cp config.toml /etc/piri/config.toml
```

## Running Piri

### Start Piri Node

Start your Piri node using the configuration file:

```bash
# If config is in ~/.config/piri/config.toml
piri serve full

# If config is in custom location
piri serve full --config=/path/to/config.toml
```

**Expected Output:**
```bash
▗▄▄▖ ▄  ▄▄▄ ▄   ▗
▐▌ ▐▌▄ █    ▄   █▌
▐▛▀▘ █ █    █  ▗█▘
▐▌   █      █  ▀▘

🔥 v0.0.14
🆔 did:key:z6MkhaXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
Piri Running on: localhost:3000
Piri Public Endpoint: https://piri.example.com
```

### Run in Background

To keep Piri running in the background:

```bash
# Using nohup
nohup piri serve full > piri.log 2>&1 &

# Or using screen
screen -S piri
piri serve full
# Press Ctrl+A then D to detach

# Or using tmux
tmux new -s piri
piri serve full
# Press Ctrl+B then D to detach
```

## Optional: Create a systemd Service

If you're using systemd but don't want automatic updates, you can create a basic service file:

```bash
# Create service file
sudo nano /etc/systemd/system/piri.service
```

Add the following content (adjust paths as needed):

```ini
[Unit]
Description=Piri Storage Node
After=network.target

[Service]
Type=simple
User=YOUR_USERNAME
ExecStart=/usr/local/bin/piri serve full --config=/home/YOUR_USERNAME/.config/piri/config.toml
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Enable and start the service:

```bash
# Reload systemd
sudo systemctl daemon-reload

# Enable service to start on boot
sudo systemctl enable piri

# Start service
sudo systemctl start piri

# Check status
sudo systemctl status piri

# View logs
journalctl -u piri -f
```

## Updating Piri

For manual installations, you must update Piri yourself:

```bash
# Check for updates
piri update --check

# Check if safe to update
piri status upgrade-check

# Update to latest version
piri update
```

See the [Updating Guide](./updating.md) for detailed update instructions.

## Troubleshooting

### Piri Won't Start

```bash
# Check configuration is valid
piri serve full --config=config.toml --dry-run

# Check port 3000 is available
sudo lsof -i :3000

# Check data directories exist and are writable
ls -la /path/to/data
```

### Permission Denied

```bash
# Ensure binary is executable
chmod +x /usr/local/bin/piri

# Check data directory permissions
sudo chown -R $USER:$USER /path/to/data
```

### Process Management

```bash
# Find Piri process
ps aux | grep piri

# Stop Piri (if running in background)
pkill piri

# Or kill by PID
kill <PID>
```

## Understanding the Configuration

The configuration file created by `piri init` contains all the settings needed to run your node. To learn more about each setting, see the [Configuration Reference](./configuration.md).

---

## Next Steps

After manual installation:
- [Validate Your Setup](./validation.md)
- Learn about [Manual Updates](./updating.md)
