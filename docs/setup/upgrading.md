# Upgrading Piri

This guide covers how to upgrade an existing Piri installation to a newer version.

## Prerequisites

Before upgrading, ensure you have:
- ✅ A running Piri installation
- ✅ Your configuration file (piri-config.toml)
- ✅ Access to the Piri server process

## Upgrade Process

### Step 1: Stop Running Server

First, gracefully stop the Piri server:

```bash
# Stop the Piri full server (use Ctrl+C or your process manager)
```

### Step 2: Install New Version

Follow the [installation instructions](./installation.md) to download and install the latest version of Piri.

### Step 3: Verify Installation

Confirm the new version is installed:

```bash
piri version
```

### Step 4: Restart Server

Restart your Piri server using your existing configuration file:

```bash
piri serve full --config=piri-config.toml
```

**Note:** Do not run `piri init` when upgrading - your existing configuration and proof set should be preserved.

### Step 5: Validate Service

After restarting, verify the server is running correctly:
- Check server logs for any errors
- Follow the [validation guide](./validation.md) to test functionality

## Important Notes

- Always stop the server before upgrading to prevent data corruption
- Check the [release notes](https://github.com/storacha/piri/releases/latest) for any breaking changes

## Troubleshooting

If you encounter issues after upgrading:
1. Check server logs for error messages
2. Verify your configuration file is intact
3. Ensure your Lotus node is still synced and accessible
4. Contact the Storacha team in the [Storacha Discord server](https://discord.gg/pqa6Dn6RnP) if problems persist