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
│   └── piri              # The piri executable
├── etc/
│   └── piri-config.toml  # Your configuration
└── systemd/
    ├── piri.service
    ├── piri-updater.service
    └── piri-updater.timer
```

Everything piri needs lives in one place. The entire directory tree is owned by your user account, not root. This design has an important consequence: the service can update its own binary without needing root privileges.

### 3. Systemd Integration

Rather than copying service files directly to `/etc/systemd/system/`, we create symlinks:

```
/etc/systemd/system/piri.service → /opt/piri/systemd/piri.service
/etc/systemd/system/piri-updater.service → /opt/piri/systemd/piri-updater.service
/etc/systemd/system/piri-updater.timer → /opt/piri/systemd/piri-updater.timer
```

This approach keeps everything organized. When you uninstall piri, we just remove `/opt/piri/` and the symlinks. Clean and simple.

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
3. Replaces the binary at `/opt/piri/bin/piri`
4. Restarts the piri service to run the new version

Because the updater runs as your user (not root) and the binary is in `/opt/piri/bin/` (owned by your user), this all works without elevated privileges. The service briefly goes offline during restart, but systemd brings it back up immediately with the new version.

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

## Design Philosophy

This installation approach reflects a few core principles:

- **Least privilege**: Services run as regular users, not root
- **Self-containment**: Everything in `/opt/piri/` for easy management
- **Smart automation**: Updates that understand your node's responsibilities
- **Standard tooling**: Works with systemd like any other system service

The goal is to make running a storage node as operationally simple as possible while maintaining the flexibility advanced users need. You shouldn't have to babysit your node or worry about updates breaking things at critical moments. Set it up once, and it just works.