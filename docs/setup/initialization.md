# Initialize Your Piri Node

This guide shows you how to initialize your Piri node configuration.

## Prerequisites

Before you start, make sure you have:

1. ✅ [Set up your system](./prerequisites.md)
2. ✅ [Set up and synced a Lotus Node](./prerequisites.md#filecoin-prerequisites)
3. ✅ [Downloaded Piri](./download.md)
4. ✅ [Created your key pair](./key-generation.md)
5. ✅ [Created a wallet with funds](./prerequisites.md#funded-delegated-wallet)
6. ✅ [Set up TLS (secure connections)](./tls-termination.md)

## What is Initialization?

The `piri init` command performs all the setup needed to join the Storacha network:
- Imports your wallet into Piri
- Creates a proof set on-chain (if you don't have one)
- Registers you with the Storacha network
- Obtains delegation proofs from the network
- Generates a complete configuration file (`config.toml`)

This configuration file will be used in the next step when you choose your installation method.

## Run Initialization

Run the `piri init` command with all required settings. The configuration file is written to stdout and progress messages go to stderr, so you can save the output to a file:

```bash
./piri init \
  --data-dir=/path/to/data \
  --temp-dir=/path/to/temp \
  --key-file=/path/to/service.pem \
  --wallet-file=/path/to/wallet.hex \
  --lotus-endpoint=wss://YOUR_LOTUS_ENDPOINT/rpc/v1 \
  --operator-email=your-email@example.com \
  --public-url=https://piri.example.com > config.toml
```

### Command Parameters

- `--data-dir`: Directory for permanent Piri data
- `--temp-dir`: Directory for temporary data
- `--key-file`: Path to your [identity PEM file](./key-generation.md#generating-a-pem-file--did)
- `--wallet-file`: Path to your [wallet hex file](./key-generation.md#preparing-your-wallet-file)
- `--lotus-endpoint`: WebSocket address of your Lotus node
- `--operator-email`: Your email so the Storacha team can contact you
- `--public-url`: The public HTTPS address for your Piri node (use the domain from the [TLS setup](./tls-termination.md), like `https://piri.example.com`)

### Expected Output

💡Note: Step 3 `Setting up proof set` can take up to 5 minutes to complete.

```bash
🚀 Initializing your Piri node on the Storacha Network...

[1/5] Validating configuration...
✅ Configuration validated

[2/5] Creating Piri node...
✅ Node created with DID: did:key:z6MkhaXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX

[3/5] Setting up proof set...
📝 Creating new proof set...
⏳ Waiting for proof set creation to be confirmed on-chain...
   Transaction status: pending
   Transaction status: confirmed
✅ Proof set created with ID: 123

[4/5] Registering with delegator service...
✅ Successfully registered with delegator service
📥 Requesting proof from delegator service...
✅ Received delegator proof

[5/5] Generating configuration file...

🎉 Initialization complete! Your configuration:
```

The command creates a complete TOML configuration file. If you didn't save it to a file earlier, you can copy and save it now.

## Understanding the Configuration

The configuration file created by `piri init` contains all the settings needed to run your node. To learn more about each setting, see the [Configuration Reference](./configuration.md).

## Troubleshooting

If you have problems, the Storacha team can help you in the [Storacha Discord server](https://discord.gg/pqa6Dn6RnP).

### Initialization Issues

If initialization fails:
1. Check your Lotus node is synced and reachable
2. Make sure your wallet has enough funds
3. Verify your public URL is reachable from the internet
4. Check DNS is configured correctly for your domain
5. Ensure the binary has execute permissions: `chmod +x ./piri`

---

## Next Steps

After initializing your Piri node:
- [Choose Your Installation Method](./choosing-installation.md)
