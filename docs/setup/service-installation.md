# Service Installation

This guide shows you how to install Piri as a systemd service with automatic updates.

## Overview

When you install Piri as a service, you get:

- **Piri runs automatically** - Starts on boot, restarts on failure
- **Auto-updates enabled** - Checks for new patch releases every 30 minutes
- **Managed installation** - All files in `/opt/piri/` with versioned directories
- **systemd integration** - Standard Linux service management

This is the **recommended setup for production nodes**.

## Prerequisites

Before installing as a service, ensure you have:

- ✅ Linux system with systemd
- ✅ Root access (sudo)
- ✅ [Downloaded Piri binary](./download.md) (should be `./piri` in your current directory)
- ✅ [Configuration file from initialization](./initialization.md) (`config.toml`)

## Installation

Run the install command with your configuration file:

```bash
# Install with auto-updates (recommended)
sudo ./piri install --config config.toml

# Install without auto-updates
sudo ./piri install --config config.toml --enable-auto-update=false
```

The service will run as the user who invoked sudo, and `/opt/piri/` will be owned by that user. A sudoers entry is created to allow non-root service management for auto-updates.

### What Gets Installed

The installation creates the following structure:

```
/opt/piri/
├── bin/
│   ├── current -> v0.0.14/    # Symlink to active version
│   └── v0.0.14/piri           # Versioned binary
├── etc/
│   └── piri-config.toml       # Your configuration
└── systemd/
    └── current -> v0.0.14/    # Active service files
        ├── piri.service
        ├── piri-updater.service
        └── piri-updater.timer
```

**Key Locations:**
- Binary: `/opt/piri/bin/current/piri`
- Config: `/opt/piri/etc/piri-config.toml`
- CLI: `/usr/local/bin/piri` → current binary
- Service files: `/etc/systemd/system/piri*` → current version

**Services Created:**
- `piri.service` - Main Piri node
- `piri-updater.timer` - Triggers update checks every 30 minutes
- `piri-updater.service` - Performs update when triggered

## Verify Installation

Check that the service is running:

```bash
# Check service status
sudo systemctl status piri

# View logs
journalctl -u piri -f
```

You should see output similar to:

```
▗▄▄▖ ▄  ▄▄▄ ▄   ▗
▐▌ ▐▌▄ █    ▄   █▌
▐▛▀▘ █ █    █  ▗█▘
▐▌   █      █  ▀▘

🔥 v0.0.14
🆔 did:key:z6MkhaXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
Piri Running on: localhost:3000
Piri Public Endpoint: https://piri.example.com
```

## Migrating from Manual Installation

If you already have Piri installed manually at `/usr/local/bin/piri`:

```bash
# Service installation will automatically handle the existing binary
sudo ./piri install --config config.toml
```

The install command detects and handles existing installations, so no `--force` flag is needed.

## Auto-Updates

By default, auto-updates are enabled. The system:

1. Checks every 30 minutes if it's safe to update (not proving, not in challenge window)
2. Checks GitHub for new patch releases
3. Downloads and installs to new version directory
4. Updates symlinks and restarts service
5. On failure: automatically rolls back

**Note:** Only patch versions auto-update (v1.2.3 → v1.2.4). Major/minor updates require manual reinstallation.

### Managing Auto-Updates

```bash
# Check when next update runs
systemctl list-timers piri-updater.timer

# View update history
journalctl -u piri-updater.service -n 20

# Disable auto-updates (if needed)
sudo systemctl disable --now piri-updater.timer

# Enable auto-updates
sudo systemctl enable --now piri-updater.timer

# Manually trigger update
sudo systemctl start piri-updater.service
```

## Troubleshooting

### Service Won't Start

```bash
# Check status and errors
systemctl status piri
journalctl -u piri -p err -n 50

# Test binary
/opt/piri/bin/current/piri version
```

### Auto-Updates Not Running

```bash
# Check timer is enabled
systemctl is-enabled piri-updater.timer

# Check update logs
journalctl -u piri-updater.service --since "1 day ago"

# Enable timer if disabled
sudo systemctl enable --now piri-updater.timer
```

### Permission Issues

Ensure the user who invoked sudo owns `/opt/piri/`:

```bash
ls -la /opt/piri
```

---

## Next Steps

After installing as a service:
- [Validate Your Setup](./validation.md)
- [Learn about Service Management](./service-management.md) for ongoing operations
