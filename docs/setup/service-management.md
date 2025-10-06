# Managing Piri Service Installation

This guide covers managing an existing Piri service installation.

## Prerequisites

This guide assumes you have already:
- ✅ [Installed Piri as a service](./service-installation.md)

If you haven't installed Piri as a service yet, see the [Service Installation Guide](./service-installation.md).

## Overview

Piri service installations provide:

- **Automatic operation** - Starts on boot, restarts on failure
- **Auto-updates** - Checks for new patch releases every 30 minutes
- **Managed installation** - All files in `/opt/piri/` with versioned directories
- **systemd integration** - Standard Linux service management

## Directory Structure

```

/opt/piri/
├── bin/
│  ├── current -> v0.0.14/    # Symlink to active version
│  └── v0.0.14/piri        # Versioned binary
├── etc/
│  └── piri-config.toml      # Your configuration
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

**Services:**

- `piri.service` - Main Piri node

- `piri-updater.timer` - Triggers update checks every 30 minutes

- `piri-updater.service` - Performs update when triggered

## Auto-Updates

### How It Works

Every 30 minutes (with 5-minute random delay):

1. Checks if safe to update (not proving, not in challenge window)
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

# Enable/disable
sudo systemctl enable --now piri-updater.timer

sudo systemctl disable --now piri-updater.timer

# Manually trigger update
sudo systemctl start piri-updater.service

```

**Common skip reasons:**

- Node is actively proving

- Node in unproven challenge window

- Already running latest version

- Major/minor version change detected

## Managing the Service

```bash

# Service control
sudo systemctl status piri
sudo systemctl [start|stop|restart] piri
sudo systemctl [enable|disable] piri

# Logs
journalctl -u piri -f               # Follow logs
journalctl -u piri -n 50            # Last 50 lines
journalctl -u piri -p err           # Errors only
journalctl -u piri --since "1h ago" # Logs in past hour

```

## Versioning and Rollback

Each version is installed in its own directory. The `current` symlink points to the active version:

```

/opt/piri/bin/current -> v0.0.15

```

**Manual rollback:**

```bash

# List versions
ls -la /opt/piri/bin/

# Rollback to previous version
sudo ln -sfn /opt/piri/bin/v0.0.14 /opt/piri/bin/current
sudo systemctl restart piri

```

Old versions remain on disk for manual rollback. Clean up old versions to free space:

```bash

cd /opt/piri/bin
sudo rm -rf v0.0.12 v0.0.13

```

## Troubleshooting

**Auto-updates not running:**

```bash

systemctl is-enabled piri-updater.timer

journalctl -u piri-updater.service --since "1 day ago"

sudo systemctl enable --now piri-updater.timer

```



**Service won't start:**

```bash

systemctl status piri

journalctl -u piri -p err -n 50

/opt/piri/bin/current/piri version # Test binary

```

**Update failed:**

Check logs - automatic rollback should have occurred:

```bash

journalctl -u piri-updater.service -n 50

systemctl status piri # Should be running on previous version

```

**Update config:**

```bash

vim /opt/piri/etc/piri-config.toml

sudo systemctl restart piri

```



## Uninstalling

```bash

sudo piri uninstall       # Stops services, removes system files, preserves /opt/piri

sudo rm -rf /opt/piri      # Completely remove (run after uninstall)

```

**See also:**

- [Service Installation](./service-installation.md) - Initial installation guide
- [Validation](./validation.md) - Testing your setup
- [Configuration](./configuration.md) - Configuration reference

