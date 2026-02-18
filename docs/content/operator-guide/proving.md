# How Proving Works

Proving is how you demonstrate you still hold the data you committed to store. Without proofs, there's no way to verify storage claims—and no basis for payment.

## What is PDP?

**Proof of Data Possession (PDP)** is a cryptographic protocol that allows a client or smart contract to verify that a storage provider still holds a data set, without downloading it again. For a detailed overview, see the [PDP documentation](https://docs.filecoin.cloud/core-concepts/pdp-overview/).

The core idea:

1. A verifier issues a **challenge**: "Show me proof you have leaf X from your data"
2. You generate a **proof** using the actual data
3. The verifier checks the proof against the known commitment

If you don't have the data, you can't generate a valid proof. This happens on-chain through smart contracts on Filecoin.

## Key Concepts

### Dataset

Your **dataset** is the on-chain record of what you've committed to store. It contains:

- **Roots**: Cryptographic commitments (CommP hashes) representing your aggregated data
- **Challenge range**: 5 pieces per challenge
- **Proving schedule**: When you need to submit proofs

When you run `piri init`, a dataset is created for you on-chain. As you accept and aggregate data, roots are added to your dataset.

### Proving Period

The **proving period** is the maximum time allowed between proof submissions. Your node must prove possession within each period to avoid faults.

### Challenge Window

When a proving period ends, a **challenge window** opens:

- The smart contract generates a random challenge using blockchain randomness
- Your node has a limited number of epochs to respond with a valid proof
- Missing the window triggers a fault

### Epochs

Time on Filecoin is measured in **epochs** (approximately 30 seconds each). Proving schedules, challenge windows, and deadlines are all expressed in epochs.

### Protocol Parameters

| Parameter | Value | Approximate Time |
|-----------|-------|------------------|
| Epoch duration | ~30 seconds | — |
| Challenge finality | 150 epochs | ~75 minutes |
| Max proving period | 2880 epochs | ~1 day |
| Challenge window | 20 epochs | ~10 minutes |

In practice: your node must submit a proof approximately once per day. When a challenge is issued, you have about 10 minutes to respond with a valid proof.

## The Proving Lifecycle

Here's what happens during normal operation:

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Proving        │────▶│  Challenge      │────▶│  Next Proving   │
│  Period         │     │  Window Opens   │     │  Period         │
└─────────────────┘     └─────────────────┘     └─────────────────┘
                              │
                              ▼
                        ┌─────────────────┐
                        │  Generate &     │
                        │  Submit Proof   │
                        └─────────────────┘
```

1. **Challenge issued**: At the scheduled epoch, a challenge is generated from blockchain randomness
2. **Proof generation**: Your node reads the challenged data pieces and computes a proof
3. **Proof submission**: The proof is submitted to the smart contract
4. **Verification**: The contract verifies the proof against your committed roots
5. **Next period**: The cycle advances and a new proving period begins

## Automatic Operation

Your Piri node handles proving automatically through two background tasks:

**NextProvingPeriod task** monitors the chain and advances your proving schedule when the current window closes.

**Prove task** generates and submits proofs when challenges are active.

You don't need to manually trigger proofs—the system watches chain epochs and acts at the right time.

## Faults

A **fault** occurs when you miss a challenge window. This can happen if:

- Your Lotus node falls out of sync
- Your Piri node is down during the window
- Network issues prevent transaction submission
- Insufficient gas funds in your wallet

Faults have consequences:

- Reduced or withheld payments for the affected period
- Potential reputation impact

Avoiding faults is your primary operational concern.

## Keeping Proofs Healthy

To maintain reliable proving:

**Keep Lotus synced**: Your Lotus node must stay current with the chain. A node that falls behind can't see current epochs or submit timely transactions.

**Monitor your node**: Watch for fault indicators. See [Monitoring](monitoring.md).

**Maintain wallet balance**: Proof submission requires gas. Ensure your wallet has sufficient FIL.

**Stable infrastructure**: Uptime matters. Challenge windows don't wait for your node to come back online.

## Gas Costs

Proof submission consumes gas. Empirical measurements:

- **ProvePossession** for a data set of 100 roots costs approximately 140M gas units
- Data set size (total bytes) does not significantly impact gas usage
- Increasing roots increases cost by ~10M gas per 10x more roots

The largest gas consumer during data ingestion is adding roots to the chain—this is why the aggregation manager's `batch_size` and `poll_interval` are configurable. Adding multiple roots in a single message is more efficient than one root per message. See [aggregation manager configuration](../configuration/pdp/aggregation/manager.md) for tuning options.
