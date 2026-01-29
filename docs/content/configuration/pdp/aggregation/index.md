# Aggregation

The aggregation pipeline processes accepted blobs through four stages before submitting proofs to the blockchain:

1. **CommP Calculation** - Computes the Piece Commitment/PieceCID.
2. **Aggregation** - Groups pieces into 128-256MB Aggregates.
3. **Batch Management** - Batches aggregates for efficient chain submission.
4. **Chain Submission** - Submits roots to the PDP smart contract(s).

## Subsections

### [commp](commp.md)

CommP calculation job queue configuration.

### [aggregator](aggregator.md)

Piece aggregation job queue configuration.

### [manager](manager.md)

Batch management and chain submission configuration.
