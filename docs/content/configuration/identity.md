# identity

Node identity configuration.

| Key                  | Default | Env                      | Dynamic |
|----------------------|---------|--------------------------|---------|
| `identity.key_file`  | -       | `PIRI_IDENTITY_KEY_FILE` | No      |

## Fields

### `key_file`

Path to ED25519 PEM private key file. Generate with `piri identity generate`.

## TOML

```toml
[identity]
key_file = "/etc/piri/service.pem"
```
