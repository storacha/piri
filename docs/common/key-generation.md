# Generating and Managing Cryptographic Keys

The `service.pem` file contains your storage provider's cryptographic identity. 
This single file is shared by all Piri services to maintain a consistent identity.

## What is a PEM File?

A PEM ([Privacy-Enhanced Mail](https://en.wikipedia.org/wiki/Privacy-Enhanced_Mail)) file contains cryptographic keys in a base64-encoded format. 
For Piri, this file includes:
- **Private Key**: Used for signing operations and proving identity (must be Ed25519)
- **Public Key**: Shared with other services for verification

## Key Requirements

Piri requires an **Ed25519** private key. Ed25519 is a modern elliptic curve signature scheme that provides:
- High security with small key sizes (32 bytes)
- Fast signature generation and verification
- Deterministic signatures

## Generating a PEM File

1. Generate a new Ed25519 key and save as a PEM file using Piri:
   ```bash
   piri identity generate > service.pem
   ```

2. Verify the file and derive your DID:
   ```bash
   piri identity parse service.pem
   ```
   Example output: `did:key:z6MkhaXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX`

## Important Usage Notes

### Single File for Multiple Services
- Use the **SAME** `service.pem` file for both your PDP and UCAN servers
- This ensures consistent identity across your infrastructure
- Never generate separate keys for different services

### Curio Integration
When using Curio as your PDP backend:
1. Extract the **Public Key** block from your `service.pem`
2. Add it to Curio's PDP configuration section as a service named `storacha`
3. This allows Piri UCAN server to authenticate with Curio

To extract just the public key:
```bash
openssl pkey -in service.pem -pubout
```

### Security Considerations
- **Protect this file**: It contains your private key
- Set appropriate file permissions: `chmod 600 service.pem`
- **Backup securely**: Loss of this file means loss of your provider identity

## File Location

While you can place `service.pem` anywhere, we recommend:
- Development: Project root directory
- Production: `/etc/piri/service.pem` or similar secure location

## Next Steps

After generating your PEM file:
- For PDP server setup: Pass via `--key-file=service.pem` parameter
- For UCAN server setup: Pass via `--key-file=service.pem` parameter
- For delegation: Your DID (derived above) will be registered with Storacha