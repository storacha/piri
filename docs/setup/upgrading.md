# Upgrading Piri

This guide covers how to upgrade an existing Piri installation to a newer version.

## Prerequisites

Before upgrading, ensure you have:
- ✅ A running Piri installation
- ✅ Access to both UCAN and PDP server processes
- ✅ Noted your current server configurations

## Upgrade Process

### Step 1: Stop Running Servers

First, gracefully stop both Piri servers:

1. **Stop the UCAN server**
2. **Stop the PDP server**

### Step 2: Install New Version

Follow the [installation instructions](./installation.md) to download and install the latest version of Piri.

### Step 3: Verify Installation

Confirm the new version is installed:

```bash
piri version
```

### Step 4: Restart Servers

Restart your servers with the same configuration as before:

1. **Start the PDP server** (see [PDP server guide](../guides/pdp-server.md) for details)
   ```bash
   piri serve pdp --lotus-url=wss://YOUR_LOTUS_ENDPOINT/rpc/v1 --eth-address=YOUR_ETH_ADDRESS
   ```

2. **Start the UCAN server** (see [UCAN server guide](../guides/ucan-server.md) for details)
   ```bash
   piri serve ucan --key-file=service.pem --pdp-server-url=https://up.piri.example.com
   ```

### Step 5: Validate Services

After restarting, verify both servers are running correctly:
- Check server logs for any errors
- Follow the [validation guide](./validation.md) to test functionality

## Important Notes

- Always stop servers before upgrading to prevent data corruption
- Check the [release notes](https://github.com/storacha/piri/releases/latest) for any breaking changes

## Troubleshooting

If you encounter issues after upgrading:
1. Check server logs for error messages
2. Verify your configuration files are intact
3. Ensure your Lotus node is still synced and accessible
4. Contact the Storacha team if problems persist