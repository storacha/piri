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

---

## Next Steps

After generating your keys:
- [Configure TLS Termination](./tls-termination.md)
- Then proceed to set up:
  - [PDP Server](../guides/pdp-server.md)
  - [UCAN Server](../guides/ucan-server.md)