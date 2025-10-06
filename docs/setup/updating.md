# Updating Piri

> **⚠️ This guide is for manual installations only**
>
> If you used [Service Installation](./service-installation.md), updates are **automatic** and handled by the auto-updater.
> See the [Service Management Guide](./service-management.md) for managing auto-updates.

This guide covers manual updates for standalone Piri installations.

## Prerequisites

Before updating:
- ✅ [Manual Piri installation](./manual-installation.md) (not installed as a service)
- ✅ Binary file you can write to, or `sudo` access
- ✅ Node should not be actively proving

## Check for Updates

```bash
# Check if update is available
piri update --check
```

**Expected Output:**
```
Current version: v0.0.14
Latest version: v0.0.15
Update available: v0.0.14 -> v0.0.15
```

## Check if Safe to Update

```bash
# Check if node is safe to update
piri status upgrade-check
```

This checks if your node is:
- Not actively proving
- Not in an unproven challenge window

## Update Piri

### Standard Update

```bash
# Update to latest version
piri update
```

**Expected Output:**
```
Current version: v0.0.14
Latest version: v0.0.15
Downloading update from https://github.com/storacha/piri/releases/...
Downloading update ████████████████████ 100%
Verifying archive checksum...
Archive checksum verified successfully
Extracted binary from archive: piri (12345678 bytes)
Applying update...
Successfully updated to version v0.0.15
Restart required for update to take effect
```

### Update Options

```bash
# Force update (skip safety checks - use carefully)
piri update --force

# Update to specific version
piri update --version v1.2.3
```

### If Update Requires sudo

```bash
# Update with elevated privileges
sudo piri update
```

## After Updating

Stop your current Piri process and restart it:

```bash
# Stop current process (Ctrl+C if running in foreground)

# Restart with your config
piri serve full --config=config.toml
```

## Verify Update

```bash
# Check version
piri version
```

## Troubleshooting

### Update Blocked - Service Installation

If you see:
```
Error: cannot manually update managed installation
```

You have a service installation. Use auto-updates instead:
```bash
sudo systemctl enable --now piri-updater.timer
```

See the [Service Management Guide](./service-management.md) for more details.

### Update Blocked - Node Busy

If you see:
```
Error: Node is currently proving
Update blocked for safety. Use --force to override
```

Wait until proving completes, or use `--force` if absolutely necessary (not recommended).

### Permission Denied

If you see permission errors:
```bash
# Run update with sudo
sudo piri update

# After update, restart as normal user
piri serve full --config=config.toml
```

## Important Notes

- A backup of your previous binary is saved as `piri.old`
- Your configuration and data are not affected by updates
- Always check the [release notes](https://github.com/storacha/piri/releases/latest) for breaking changes
- If you want automatic updates, consider switching to [Service Installation](./service-installation.md)

---

## Next Steps

- Return to [Getting Started](../getting-started.md)
- Consider switching to [Service Installation](./service-installation.md) for automatic updates