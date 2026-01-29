# parse

Parse a DID from a PEM file containing an Ed25519 key.

## Usage

```
piri identity parse <pem-file>
```

## Arguments

| Argument | Description |
|----------|-------------|
| `<pem-file>` | Path to PEM file containing Ed25519 private key |

## Example

```bash
piri identity parse my-key.pem
```

```
# did:key:z6MkhaXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
```
