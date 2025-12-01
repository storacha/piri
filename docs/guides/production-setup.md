# Production Network Setup

This guide covers how to setup a Piri node on the Forge Production Network. It covers two scenarios:

1. [Setting up a brand new Piri node on the Forge Production Network](#setup-a-new-piri-node).
2. [Migrating a Piri node currently operating on the Staging Network](#migrate-a-piri-node-from-staging).

## Setup a new Piri node

Follow the Piri [getting started guide](../production/getting-started.md) for Forge Production.

## Migrate a Piri node from Staging

If your Piri node is already running the latest version on the Staging network, follow these instructions to migrate it to the Forge Production network:

1. Satisfy Filecoin [Mainnet](https://docs.filecoin.io/networks/mainnet) prerequisites.
    * Setup a Lotus node on the Filecoin Mainnet.
        * Sync Lotus using the latest Mainnet [Snapshot](https://forest-archive.chainsafe.dev/latest/mainnet/).
        * Enable [ETH RPC](https://lotus.filecoin.io/lotus/configure/ethereum-rpc/#enableethrpc).
    * Create a Funded Delegated Wallet on the Filecoin Mainnet.
        * Run: `lotus wallet new delegated`
        * Add funds!
        * Verify funds: `lotus wallet balance YOUR_DELEGATED_ADDRESS`
    * Read our [Filecoin prerequisites guide](../setup/prerequisites.md) for full details.
2. Stop your Piri process.
3. Delete your data directory (preserving `service.pem`):
     ```bash
     rm -rf /path/to/piri/data  # or your custom data directory path
     ```
     > ⚠️ IMPORTANT: Keep your `service.pem` key file!
4. Prepare your wallet for import into Piri:
    ```bash
    # Export wallet from Lotus to hex format
    lotus wallet export YOUR_DELEGATED_ADDRESS > wallet.hex
    ```
5. Run `piri init` with the following parameters, updated as appropriate:
    ```bash
    piri init \
      --network=forge-prod \
      --data-dir=/path/to/data \
      --temp-dir=/path/to/temp \
      --key-file=/path/to/service.pem \
      --wallet-file=/path/to/wallet.hex \
      --lotus-endpoint=wss://YOUR_LOTUS_ENDPOINT/rpc/v1 \
      --operator-email=your-email@example.com \
      --public-url=https://piri.example.com > config.toml
    ```
    > ℹ️ All parameters other than `--network` MUST be updated.
6. Start your Piri node using the config you generated:
    ```bash
    piri serve --config=config.toml
    ```
    > ℹ️ If the `--config` option is not provided, Piri will automatically load config from your user config directory e.g. `~/.config/piri/config.toml`.

## Troubleshooting

If you have problems, the Storacha team can help you in the [Storacha Discord server](https://discord.gg/pqa6Dn6RnP).
