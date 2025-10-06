# Choose Your Installation Method

Now that you've initialized your Piri node configuration, you need to choose how to install and run Piri.

## Prerequisites

Before choosing your installation method, ensure you have:
- ✅ [Initialized your Piri node](./initialization.md) and have a `config.toml` file

## Two Installation Methods

### Option A: Service Installation (Recommended)

**For production nodes**, we **strongly recommend service installation** because it provides:

- ✅ **Automatic patch updates** - Checks for new patch releases every 30 minutes
- ✅ **Automatic restart on failure** - Systemd monitors and restarts your node
- ✅ **Starts on system boot** - Your node runs automatically when the server starts
- ✅ **Centralized logging** - View logs with `journalctl`
- ✅ **Easy rollback** - Previous versions are kept for quick rollback
- ✅ **Managed installation** - All files organized in `/opt/piri/`

**Best for:** Production storage providers, long-running nodes, automated operations

👉 **[Install as Service](./service-installation.md)** (Recommended)

---

### Option B: Manual Installation

**For development or testing**, manual installation gives you:

- Full control over when to update
- No systemd dependency
- Flexibility to run in non-standard environments
- Suitable for temporary or experimental setups

**Best for:** Development, testing, learning, non-systemd environments

👉 **[Manual Installation](./manual-installation.md)**

---

## Comparison

| Feature | Service Installation | Manual Installation |
|---------|---------------------|---------------------|
| **Auto-updates** | ✅ Every 30 minutes (patch only) | ❌ Manual updates required |
| **Auto-restart** | ✅ On failure | ❌ Manual restart |
| **Boot on startup** | ✅ Yes | ❌ Must configure manually |
| **Logging** | ✅ journalctl | ❌ Manual setup |
| **Rollback** | ✅ Easy | ❌ Manual backup/restore |
| **Best for** | Production | Development/Testing |

## Still Deciding?

**Choose Service Installation if:**
- You're running a production storage node
- You want automatic updates and reliability
- You're using a systemd-based Linux distribution

**Choose Manual Installation if:**
- You're testing or developing
- You need full control over the update process
- You're not using systemd
- This is a temporary setup

---

## Next Steps

Choose your installation method and continue:
- **[Service Installation](./service-installation.md)** (Recommended)
- **[Manual Installation](./manual-installation.md)**
