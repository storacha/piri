# Operator Guide

This guide explains what the Forge network is, what it means to operate a Piri node, and what to expect as a storage provider on the network.

## What is Forge?

Forge is Storacha's decentralized storage network built on Filecoin. It connects users who need reliable, verifiable storage with operators who provide that capacity. Unlike traditional cloud storage, Forge requires cryptographic proofs that data remains intact and available—you can't just claim to store something, you have to prove it.

As an operator, you run a Piri node that:

- **Stores data** replicated from Storacha's upload service
- **Serves retrievals** when users request their content
- **Generates proofs** demonstrating you still hold the data you committed to store

In return, you earn payments for both storage and retrieval.

## Why Operate a Node?

Operators earn revenue from two sources:

**Storage payments** are earned continuously for data you store and prove. These are settled on-chain in USDFC tokens via smart contracts on Filecoin. The more data you store and the more reliably you prove it, the more you earn.

**Egress payments** are earned when users retrieve data from your node. Storacha tracks retrieval events and issues monthly payments based on bandwidth served.

## What Does Operating Involve?

Running a Piri node is largely automated once set up. Your node handles replication, proof generation, and retrieval serving without manual intervention. Your responsibilities are:

- **Keep infrastructure healthy**: Your node needs stable power, network, and storage. Your Lotus node must stay synced with the Filecoin chain.
- **Monitor for issues**: Watch for faults (missed proof windows), failed jobs, or disk pressure.
- **Settle payments**: Periodically claim your earned storage payments from the smart contracts.

## Getting Started

If you're ready to become an operator:

1. **[Joining the Network](joining.md)** — How the approval process works and what happens after
2. **[Lotus Node Setup](lotus.md)** — Requirements for your Filecoin Lotus node
3. **[How Storage Works](storage.md)** — Understanding replication and data flow
4. **[How Proving Works](proving.md)** — The proof lifecycle and challenge windows
5. **[Configuration & Tuning](tuning.md)** — Key settings you can adjust
6. **[Getting Paid](payments.md)** — Storage and egress payment details
7. **[Monitoring](monitoring.md)** — Day-to-day operations and what to watch

For technical details on networks, services, and smart contracts, see [Concepts > Networks](../concepts/networks.md).
