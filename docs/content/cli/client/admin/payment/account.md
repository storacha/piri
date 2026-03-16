# account

Get payment account information as JSON.

## Usage

```
piri client admin payment account
```

## Example

```bash
piri client admin payment account
```

```json
{
  "account_address": "0x1234...abcd",
  "settled_balance": "1500000000000000000",
  "rails": [
    {
      "rail_id": "1",
      "dataset_id": "bага...",
      "payer_address": "0xabcd...1234",
      "payment_rate_per_epoch": "100000000000",
      "settled_up_to_epoch": 4200000,
      "net_earnings": "500000000000000000"
    }
  ]
}
```
