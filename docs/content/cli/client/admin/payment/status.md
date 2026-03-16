# status

Display payment account status with an interactive TUI or JSON output.

## Usage

```
piri client admin payment status [--format <table|json>]
```

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--format` | string | `table` | Output format: `table` for interactive TUI, `json` for JSON output |

## Interactive TUI

When using the default `table` format, an interactive terminal UI is displayed showing:

**Overview**: Current epoch, settled balance, and earnings summary (gross, net, forfeited amounts).

**Rails table**: A scrollable table of payment rails with columns for rail ID, dataset ID, payer address, payment rate per epoch, settled-up-to epoch, and net earnings.

### Keyboard controls

| Key | Action |
|-----|--------|
| `↑` / `↓` or `j` / `k` | Scroll through rails table |
| `r` | Refresh account status |
| `S` | Settle the selected rail |
| `W` | Withdraw funds |
| `q` or `Ctrl+C` | Quit |

### Settlement flow

Press `S` on a rail to begin settlement. A breakdown is shown with gross earnings, penalties, net amount, network fees, and gas estimates. Press `Enter` to confirm or `Esc` to cancel. The transaction is submitted and polled for on-chain confirmation.

### Withdrawal flow

Press `W` to start a withdrawal. Choose the owner address or enter a custom recipient address. Review the withdrawal estimate (recipient, amount, gas costs), then press `Enter` to confirm or `Esc` to cancel. The transaction is submitted and polled for on-chain confirmation.

## JSON mode

Use `--format json` for scripted access to payment status data:

```bash
piri client admin payment status --format json
```

## See also

- [Getting Paid](../../../../operator-guide/payments.md) for an overview of how payments work in Piri.
