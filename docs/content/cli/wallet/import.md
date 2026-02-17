# import

Import an address to the wallet.

## Usage

```
piri wallet import <wallet-file>
```

## Arguments

| Argument | Description |
|----------|-------------|
| `<wallet-file>` | Path to a file containing a delegated Filecoin address private key in hex format |

## Example

```bash
piri wallet import wallet.hex
```

```
imported wallet f410fxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx successfully!
```

To create a wallet file using Lotus:

```bash
lotus wallet new delegated
lotus wallet export <FILECOIN_ADDRESS> > wallet.hex
```

The wallet must be imported before running `piri init` or `piri serve`.
