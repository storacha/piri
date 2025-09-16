# Running Piri as a System Service

## Why a System Service?

When you're running infrastructure that other systems depend on, you need it to be reliable. That means your storage node should start automatically when the machine boots, restart if it crashes, and run independently of user sessions. This is exactly what system services provide.

The `piri install` command transforms your initialized node into a proper system service. After installation, piri runs like any other critical system component: always on, properly monitored, and managed by systemd.

## How Installation Works

The install command takes the configuration you created with `piri init` and sets up everything needed for production operation. Here's what happens when you run `sudo piri install --config piri-config.toml`:

### 1. User Detection

First, we figure out who should own the service. When you run the command with `sudo`, we detect your actual username (not root) using the `SUDO_USER` environment variable. This becomes the service user. Your piri service will run with the same permissions you have as a regular user, which follows the principle of least privilege.

### 2. Directory Structure

We create a self-contained installation under `/opt/piri/`:

```
/opt/piri/
├── bin/
│   ├── v0.0.13/
│   │   └── piri          # Version 0.0.13 binary
│   ├── v0.0.14/
│   │   └── piri          # Version 0.0.14 binary (after update)
│   └── current -> v0.0.14/  # Symlink to active version
├── etc/
│   └── piri-config.toml  # Your configuration (shared across versions)
└── systemd/
    ├── piri.service
    ├── piri-updater.service
    └── piri-updater.timer
```

The key innovation here is **versioned binaries**. Each piri version gets its own directory under `/opt/piri/bin/`, and a `current` symlink points to the active version. Old versions are preserved, not overwritten. This design enables:

- **Atomic updates**: The symlink switch is instantaneous
- **Easy rollback**: Previous versions remain available
- **Clean version management**: Like package managers (apt, yum), we never delete old binaries

Everything piri needs lives in one place. The entire directory tree is owned by your user account, not root. This design has an important consequence: the service can update its own binary without needing root privileges.

### 3. Systemd Integration

Rather than copying service files directly to `/etc/systemd/system/`, we create symlinks:

```
/etc/systemd/system/piri.service → /opt/piri/systemd/piri.service
/etc/systemd/system/piri-updater.service → /opt/piri/systemd/piri-updater.service
/etc/systemd/system/piri-updater.timer → /opt/piri/systemd/piri-updater.timer
```

The service files reference the binary through the `current` symlink: `/opt/piri/bin/current/piri`. This means when we update and switch the symlink, systemd automatically uses the new version after restart.

Additionally, we create a convenience symlink for CLI access:

```
/usr/local/bin/piri → /opt/piri/bin/current/piri
```

This lets you run `piri` commands from anywhere without adding to PATH. When you uninstall piri, we remove the symlinks but preserve the `/opt/piri/` directory - your binaries and data remain intact.

## The Auto-Update System

If you include the `--enable-auto-update` flag during installation, piri gains the ability to update itself. This isn't just downloading the latest version and hoping for the best. The updater is aware of what your node is doing and makes intelligent decisions about when it's safe to update.

### How Updates Work

Every 30 minutes (with some randomization to prevent thundering herd problems), the update timer triggers. The updater then:

1. Checks if a new version is available
2. Connects to your running piri service to check its current state
3. Decides whether it's safe to update

### When Updates Don't Happen

The updater will refuse to update if your node is:

- **Actively proving**: If piri is in the middle of generating a proof, the updater waits. Interrupting proof generation could cause you to miss a challenge window.
- **In an unproven challenge window**: If you've received a challenge but haven't proven it yet, updating would be risky. The updater waits until you've submitted your proof.
- **Unable to determine state**: If the updater can't connect to your piri service to check its state, it errs on the side of caution and skips the update.

This intelligence prevents updates from interfering with your node's primary responsibilities. Your reputation and stake are protected.

### The Update Process

When conditions are right for an update:

1. The updater downloads the new version
2. Verifies checksums to ensure integrity
3. Creates a new version directory (e.g., `/opt/piri/bin/v0.0.14/`)
4. Installs the new binary there
5. Atomically switches the `current` symlink to the new version
6. Restarts the piri service to run the new version

Because the updater runs as your user (not root) and the `/opt/piri/` directory is owned by your user, this all works without elevated privileges. The service briefly goes offline during restart, but systemd brings it back up immediately with the new version.

The versioned structure means:
- **No file overwrites**: Each version lives in its own directory
- **Atomic switches**: The symlink update is instantaneous
- **Rollback capability**: Old versions remain available if needed

### Manual Updates vs Auto-Updates

There's an important distinction between manual and automatic updates:

**Manual updates** (`piri update`):
- Work only for standalone binaries (not managed installations)
- If you try to manually update a managed installation, you'll get an error with instructions
- This prevents accidentally breaking the versioned structure

**Automatic updates** (`piri update-internal`):
- Called by the systemd timer
- Properly handle the versioned directory structure
- Create new version directories and update symlinks correctly

This separation ensures the integrity of managed installations while still allowing flexibility for standalone deployments.

## Security Considerations

Running as a non-root user provides good security isolation. If someone compromises your piri service, they get access to what your user account can access, not the entire system.

The service configuration includes systemd's `WorkingDirectory` directive, which sets the current directory to `/opt/piri/etc/`. This means piri can find its configuration file without needing absolute paths, and any relative file paths in your config work correctly.

## Operational Benefits

This design gives you several operational advantages:

1. **Reliability**: Systemd restarts piri if it crashes. Your node stays online.

2. **Observability**: Standard systemd tools work perfectly:
   - `systemctl status piri` shows service state
   - `journalctl -u piri` shows logs
   - `systemctl stop piri` cleanly shuts down the service

3. **Clean updates**: The binary can be updated without root access, and updates are smart enough to avoid disrupting critical operations.

4. **Easy uninstall**: Everything lives under `/opt/piri/`. Removal is straightforward.

## Version Management

The versioned binary structure provides several operational benefits:

### Listing Installed Versions

You can see all installed versions:

```bash
ls -la /opt/piri/bin/
```

Output might look like:
```
drwxr-xr-x  5 ubuntu ubuntu 4096 Jan 15 10:00 .
drwxr-xr-x  5 ubuntu ubuntu 4096 Jan 15 09:00 ..
lrwxrwxrwx  1 ubuntu ubuntu   10 Jan 15 10:00 current -> v0.0.14/
drwxr-xr-x  2 ubuntu ubuntu 4096 Jan 14 12:00 v0.0.12/
drwxr-xr-x  2 ubuntu ubuntu 4096 Jan 14 18:00 v0.0.13/
drwxr-xr-x  2 ubuntu ubuntu 4096 Jan 15 10:00 v0.0.14/
```

### Manual Rollback

If you need to rollback to a previous version:

```bash
# Stop the service
sudo systemctl stop piri

# Switch the symlink
sudo ln -sfn /opt/piri/bin/v0.0.13 /opt/piri/bin/current

# Restart the service
sudo systemctl start piri
```

### Cleaning Old Versions

While we never automatically remove old versions, you can manually clean them up:

```bash
# Remove specific old version
sudo rm -rf /opt/piri/bin/v0.0.12/

# Keep only the current and one previous version
cd /opt/piri/bin/
ls -d v*/ | head -n -2 | xargs -r sudo rm -rf
```

## Manual Installation

If you prefer to set things up manually or need a custom configuration, you can inspect what the install command would do by using the `--dry-run` flag:

```bash
sudo piri install --config piri-config.toml --dry-run
```

This shows you exactly what files would be created and where, without making any changes. You can use this output as a template for your own installation process.

## Troubleshooting

If your service fails to start after installation, check:

1. **File permissions**: Ensure your user can read all files referenced in the config (especially the key file)
2. **Port availability**: Make sure the port specified in your config isn't already in use
3. **Logs**: Run `journalctl -u piri -e` to see recent log entries

The install command performs basic validation, but it can't catch every possible configuration issue. The service logs usually point directly to the problem.

## Migration from Older Installations

If you're upgrading from a pre-versioned piri installation (where the binary was at `/usr/local/bin/piri`), the new installer handles this gracefully:

1. The installer will replace the old binary with a symlink to the managed installation
2. Your existing binary gets moved to the versioned structure
3. Auto-updates will start using the new versioned approach

No manual intervention needed - the installer detects and migrates automatically.

## Design Philosophy

This installation approach reflects a few core principles:

- **Least privilege**: Services run as regular users, not root
- **Self-containment**: Everything in `/opt/piri/` for easy management
- **Smart automation**: Updates that understand your node's responsibilities
- **Standard tooling**: Works with systemd like any other system service
- **Version preservation**: Like traditional package managers, we never delete old binaries
- **Atomic operations**: Updates via symlink switches are instantaneous and safe

The goal is to make running a storage node as operationally simple as possible while maintaining the flexibility advanced users need. You shouldn't have to babysit your node or worry about updates breaking things at critical moments. Set it up once, and it just works.

The versioned binary structure, borrowed from package management best practices, ensures that updates are always safe and reversible. Combined with the intelligent update timing that respects your node's proof obligations, this creates a robust, production-ready deployment system.