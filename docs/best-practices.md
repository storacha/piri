# Best Practices for running a Piri node

1. [Keep your Lotus node up to date](#keep-your-lotus-node-up-to-date)
1. [Keep your Piri node up to date](#keep-your-piri-node-up-to-date)
1. [Do not use the same Filecoin wallet on staging and production](#do-not-use-the-same-filecoin-wallet-on-staging-and-production)
1. [Install Piri as a systemd service](#install-piri-as-a-systemd-service)

## Keep your Lotus node up to date

Your Piri node depends on a Lotus node that is fully synced and on the current release.
If Lotus falls behind, Piri cannot submit proofs on time and you risk missing proving windows. 
Always update Lotus to the [latest release](https://github.com/filecoin-project/lotus/releases/latest) as soon as it is published, and monitor the Filecoin Slack [`#fil-lotus-announcements`](https://filecoinproject.slack.com/archives/C027TQMUVJN) channel for release notices and upgrade guidance.

## Keep your Piri node up to date

Running the latest Piri release keeps you protocol-compatible with the Storacha network, pulls in bug fixes and security patches, and ensures your node understands the current proof and API expectations. 
Watch the [latest releases](https://github.com/storacha/piri/releases/latest) and upgrade promptly when a new version is published; release notes will call out any operator actions or migrations needed.

Join the Storacha Discord for announcements and operator guidance: [invite](https://discord.gg/pqa6Dn6RnP).

Announcements related to Piri and the Forge network are made in:
- [`#piri-announcements`](https://discord.com/channels/1247475892435816553/1445143779945484328)
- [`#storacha-forge-production-network`](https://discord.com/channels/1247475892435816553/1443282702374801549).

## Do not use the same Filecoin wallet on staging and production

Use separate wallets for staging/test networks and production.
Sharing the same key leaks metadata across chains, risks accidentally spending production funds during testing, and complicates audits of reward flows.
Keep production keys in your normal secure key management flow, and configure Piri with the appropriate `wallet.hex` for each environment.
If you rotate keys, update both the file and any systemd unit or environment configuration to point at the new wallet.

## Install Piri as a systemd service

The following unit file is an **example only**.
Replace the placeholder values (network, endpoints, paths, email, public URL, etc.) with the settings that match your host layout and preferences.
Typical workflow:
- Save the adapted unit as `/etc/systemd/system/piri.service`.
- Create the `piri` user and group (or use an existing service account), and ensure `/etc/piri`, `/data/piri`, and `/tmp/piri` exist with correct ownership.
- Reload units after edits with `sudo systemctl daemon-reload`.
- Enable at boot with `sudo systemctl enable piri`, start with `sudo systemctl start piri`, stop with `sudo systemctl stop piri`, restart with `sudo systemctl restart piri`, and check status with `sudo systemctl status piri`.
- View logs with `sudo journalctl -u piri -f` (live) or add `--since`/`--until` for time windows.

Use the template below as a starting point (replace the sample values with your own):
```unit file (systemd)
[Unit]
# Human-readable description of the service, shown in tools like `systemctl status`.
Description=Piri Server 
# Tells systemd to only start this unit after the network stack is fully up.
After=network-online.target
# Declares a soft dependency on the network being online (systemd will try to bring it up).
Wants=network-online.target

[Service]
# `simple` means the process started by ExecStart is the main service process
# and systemd considers it started as soon as it’s spawned.
Type=simple
# Unix user account that the service runs under (change to whatever you use).
User=piri
# Primary group for the service process (usually matches the user).
Group=piri
# Working directory for the service process; relative paths are resolved from here.
WorkingDirectory=/etc/piri
# Maximum time in seconds systemd waits for all ExecStartPre + ExecStart to complete startup
# before marking the service as failed. 900 = 5 min
TimeoutStartSec=900
# Automatically restart the service when it exits with a non-zero status.
Restart=on-failure
# How long systemd waits before attempting a restart after a failure.
RestartSec=10

# Pre-start command: (re)generate piri config before launching the server.
# All values shown below are examples, populate the values based on your setup.
ExecStartPre=/usr/local/bin/piri init \
  --network=[forge-prod|warm-staging] \
  --data-dir=/data/piri \
  --temp-dir=/tmp/piri \
  --key-file=/etc/piri/service.pem \
  --wallet-file=/etc/piri/wallet.hex \
  --lotus-endpoint="https://lotus.example.com/rpc/v1" \
  --operator-email="operator@example.com" \
  --public-url="https://piri.example.com" \
  > /etc/piri/config.toml

# Main process to run for this service: starts the piri server using the generated config.
ExecStart=/usr/local/bin/piri serve --config=/etc/piri/config.toml

[Install]
# Targets this service should be pulled into when enabled. `multi-user.target`
# is the standard “normal system” runlevel on most distros, so this makes the
# service start automatically at boot in multi-user mode.
WantedBy=multi-user.target
```
