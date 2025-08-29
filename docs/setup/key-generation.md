# Generating and Managing DIDs & Cryptographic Keys

The `service.pem` file contains your storage provider's cryptographic identity, which corresponds to its DID. This single file is shared by all Piri services _you_ operate to maintain a consistent identity.

## Prerequisites

Before generating keys, ensure you have:
- ✅ [Installed Piri](./installation.md)

## Key Requirements

Piri requires an **Ed25519** private key. Ed25519 is a modern elliptic curve signature scheme.

## Generating a PEM File & DID

**Step 1: Generate a new Ed25519 key**

```bash
piri identity generate > service.pem
```

**Step 2: Verify and derive your DID**

```bash
piri identity parse service.pem
```

Example output: `did:key:z6MkhaXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX`

## Security Considerations

- **Protect this file**: It contains your private key
- **Set appropriate file permissions**: `chmod 600 service.pem`
- **Backup securely**: Loss of this file means loss of your provider identity

## Preparing Your Wallet File

Export your Filecoin wallet address to a hex file:

```bash
# Export wallet from Lotus to hex format
lotus wallet export YOUR_DELEGATED_ADDRESS > wallet.hex
```

This wallet file will be used during the initialization process to import your wallet into Piri.

---

## Next Steps

After generating your keys:
- [Configure TLS Termination](./tls-termination.md)