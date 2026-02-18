# Getting Paid

Operators earn from two revenue streams: storage payments settled on-chain, and egress payments reconciled monthly.

## Storage Payments

Storage payments compensate you for storing data and generating proofs.

**Rate**: 0.90 USDFC per TiB per month, prorated daily.

### How It Works

Payments flow through smart contracts on Filecoin:

1. **Payment rails** are established between Storacha (payer) and you (payee)
2. **Funds accrue** continuously based on your storage rate
3. **You settle** when you want to claim accumulated payments
4. **You withdraw** after the lockup period to move funds to your wallet

Payments are denominated in **USDFC**, a stablecoin on the Filecoin network.

### Payment Rails

A **rail** is a payment channel between a payer and a payee. Each rail has:

- A payment rate (USDFC per epoch)
- Settlement tracking (last settled epoch)

Your account may have multiple rails, each accumulating independently.

### Settlement

Settlement transfers funds from your escrow balance to your payment balance:

```
Unsettled epochs × Payment rate = Claimable amount
Claimable amount × 0.5% = Settlement fee
Claimable amount - Settlement fee = Net payment
```

Settlement is an on-chain transaction you initiate. You pay gas fees to submit the transaction.

**Settlement strategy**: Settling daily wastes gas relative to earnings. Consider the economics:

| Daily Storage | Daily Earnings | Gas Cost | Net if Settled Daily |
|---------------|----------------|----------|----------------------|
| 1 TiB | ~$0.03 | ~$0.002 | ~$0.028 |
| 10 TiB | ~$0.30 | ~$0.002 | ~$0.298 |
| 100 TiB | ~$3.00 | ~$0.002 | ~$2.998 |

Let balances accumulate and settle periodically—weekly, monthly, or whenever the accumulated amount justifies the gas cost. Funds sit safely in escrow until you claim them.

### The 0.5% Settlement Fee

When settling, the network charges a 0.5% fee on the settled amount. This fee accumulates in the payment contract and is periodically sold via Dutch auction for FIL, which is then burned.

### Withdrawing Funds

After settlement, withdraw funds to your wallet:

1. View your payment balance via the admin API
2. Initiate a withdrawal transaction
3. Funds transfer to your wallet address

Both settlement and withdrawal are on-chain transactions that cost gas—roughly 1 milliFIL each at current rates.

### Missed Proofs

If you miss a proof, you lose that day's compensation—nothing more. The penalty is linear: miss 1 day out of 30, and you lose 1/30th of your potential monthly earnings. There is no slashing or additional punishment.

This applies whether the miss was intentional or accidental (network issues, Lotus desync, operator maintenance). The network pays for proven storage and doesn't pay for unproven storage.

## Egress Payments

Egress payments compensate you for serving data retrievals.

**Rate**: $2.80 per TiB.

### How It Works

1. **Retrieval happens**: Users fetch data from your node
2. **Events tracked**: Your node logs retrieval events via the egress tracker
3. **Batches submitted**: Retrieval data is batched and sent to Storacha
4. **Monthly reconciliation**: Storacha reconciles egress data
5. **Payment issued**: Storacha issues payment at the end of every month

Unlike storage payments, egress is paid in fiat (traditional currency), not on-chain. There's no smart contract settlement—just money transferred through conventional payment rails.

### Mutual Accountability

Your node reports what it served (bytes delivered). Clients report what they downloaded. Storacha reconciles both reports. This ensures neither side grades their own homework.

### Egress Tracking

Your node automatically:

- Records each retrieval event
- Batches events into journal files
- Submits batches to Storacha's egress tracker service
- Tracks which batches have been consolidated

You don't need to manage this directly. The egress tracker service handles it in the background.

## Replication

Data uploaded to the Forge network is replicated across multiple Piri nodes for redundancy. You may receive data as an original upload or as a replica from another node.

**Compensation is identical in both cases.** You earn 0.90 USDFC/TiB/month regardless of whether you ingested the original or a replica.

Replication transfers between Piri nodes do not count as billable egress. Egress compensation applies only to client retrieval requests.

## Summary

| Aspect | Storage Payments | Egress Payments |
|--------|------------------|-----------------|
| Rate | 0.90 USDFC/TiB/month | $2.80/TiB |
| Currency | USDFC (on-chain) | Fiat (off-chain) |
| Accrual | Per successful daily proof | Per bytes served |
| Collection | Settle + withdraw | Monthly payment |
| Fees | 0.5% settlement fee + gas | None |

## Tips

- **Don't settle too frequently**: Gas costs eat into small settlements
- **Monitor proof compliance**: Missed proofs mean missed payments
- **Maintain wallet balance**: You need FIL for settlement and withdrawal gas
- **Track both streams**: Storage payments require action; egress payments arrive automatically
