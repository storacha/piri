# generate

Generate a new PEM-encoded Ed25519 key pair for use with decentralized identities (DIDs).

## Usage

```
piri identity generate
```

## Aliases

- `gen`

## Example

```bash
piri identity generate > my-key.pem
```

```
# did:key:z6MkhaXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
```

The PEM-encoded key is written to stdout and can be redirected to a file. The DID is printed to stderr as a comment for convenience.
