# Setup Piri Node

This guide shows you how to set up a Piri node.

## Prerequisites

Before you start, make sure you have:

1. ✅ [Set up your system](../setup/prerequisites.md)
2. ✅ [Set up and synced a Lotus Node](../setup/prerequisites.md#filecoin-prerequisites)
3. ✅ [Set up TLS (secure connections)](../setup/tls-termination.md)
4. ✅ [Installed Piri](../setup/installation.md)
5. ✅ [Created your key pair](../setup/key-generation.md)
6. ✅ [Created a wallet with funds](../setup/prerequisites.md#funded-delegated-wallet)

## Initialize Your Piri Node

The `piri init` command does all the setup needed to join the Storacha network:
- Imports your wallet
- Creates a proof set (if you don't have one)
- Signs you up with the Storacha network
- Gets the permissions you need
- Creates a configuration file

### Step 1: Prepare Your Wallet File

Export your Filecoin wallet address to a hex file:

```bash
# Export wallet from Lotus to hex format
lotus wallet export YOUR_DELEGATED_ADDRESS > wallet.hex
```

### Step 2: Run Initialization

Run the `piri init` command with all needed settings. The configuration file goes to stdout and progress messages go to stderr, so you can save the output to a file:

```bash
piri init \
  --data-dir=/path/to/data \
  --temp-dir=/path/to/temp \
  --key-file=/path/to/service.pem \
  --wallet-file=/path/to/wallet.hex \
  --lotus-endpoint=wss://YOUR_LOTUS_ENDPOINT/rpc/v1 \
  --operator-email=your-email@example.com \
  --public-url=https://piri.example.com > config.toml
```

**Settings:**
- `--data-dir`: Folder for permanent Piri data
- `--temp-dir`: Folder for temporary data
- `--key-file`: Path to your [identity PEM file](../setup/key-generation.md)
- `--wallet-file`: Path to the wallet hex file from Step 1
- `--lotus-endpoint`: WebSocket address of your Lotus node
- `--operator-email`: Your email so the Storacha team can contact you
- `--public-url`: The public HTTPS address for your Piri node (use the domain from the [TLS setup](../setup/tls-termination.md), like `https://piri.example.com`)

**Expected Output:**
```bash
🚀 Initializing your Piri node in the Storacha network...

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

## Run Your Piri Node

After setup is complete, run your Piri node using the configuration file:

```bash
piri serve full --config=config.toml
```

**Expected Output:**
```bash
▗▄▄▖ ▄  ▄▄▄ ▄   ▗
▐▌ ▐▌▄ █    ▄   █▌
▐▛▀▘ █ █    █  ▗█▘
▐▌   █      █  ▀▘

🔥 v0.0.12
🆔 did:key:z6MkhaXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
Piri Running on: localhost:3000
Piri Public Endpoint: https://piri.example.com
```

## Understanding the Configuration

The configuration file made by `piri init` has all the settings to run your node. To learn more about each setting, see the [Configuration Reference](../setup/configuration.md).

## Monitoring and Logs

Piri writes detailed logs about what it's doing. Check the logs to make sure:
- Connection to Lotus node works
- Proof set operations work
- UCAN authentication works
- Data uploads are being handled
- Proofs are being created and sent

## Troubleshooting

If you have problems, the Storacha team can help you in the [Storacha Discord server](https://discord.gg/pqa6Dn6RnP).

### Initialization Issues

If setup fails:
1. Check your Lotus node is synced and can be reached
2. Make sure your wallet has enough funds
3. Check your public URL can be reached from the internet
4. Check DNS is set up correctly for your domain

### Runtime Issues

If the server won't start:
1. Check that port 3000 is free to use
2. Check all configuration values are correct
3. Make sure the data and temp folders exist and you can write to them
4. Check the wallet was imported correctly during setup

---

## Next Steps

After setting up your Piri node:
- [Test Your Setup](../setup/validation.md)
- Watch the logs to see proofs being made
- Check your transactions on the blockchain