# Store Package

This package provides storage abstractions for Piri. Stores are composed in layers, with different patterns for structured data vs binary blobs.

## Architecture

```
                    ┌─────────────────────────────────────────────────┐
                    │              Application Layer                  │
                    └─────────────────────────────────────────────────┘
                                          │
              ┌───────────────────────────┴───────────────────────────┐
              │                                                       │
              ▼                                                       ▼
┌─────────────────────────────┐                         ┌─────────────────────────┐
│   genericstore.Store[T]     │                         │    blobstore.Store      │
│   ─────────────────────     │                         │    ───────────────      │
│   • Codec[T] serialization  │                         │    • KeyEncoder only    │
│   • Full buffering (ReadAll)│                         │    • Streaming preserved│
│   • For small structured    │                         │    • Range requests     │
│     records (KB-sized)      │                         │    • For large binaries │
└─────────────────────────────┘                         │     (100s of MiB)       │
              │                                         └─────────────────────────┘
              │                                                       │
              └───────────────────────────┬───────────────────────────┘
                                          │
                                          ▼
                            ┌─────────────────────────────┐
                            │      objectstore.Store      │
                            │      ────────────────       │
                            │   • Get() → Object (stream) │
                            │   • Put(io.Reader)          │
                            │   • Delete()                │
                            │   • List()                  │
                            └─────────────────────────────┘
                                          │
              ┌───────────────────────────┼───────────────────────────┐
              │                           │                           │
              ▼                           ▼                           ▼
    ┌─────────────────┐         ┌─────────────────┐         ┌─────────────────┐
    │   minio.Store   │         │  flatfs.Store   │         │  memory.Store   │
    │   (S3/MinIO)    │         │  (filesystem)   │         │  dsadapter      │
    └─────────────────┘         └─────────────────┘         └─────────────────┘
```

All blobstore implementations use a single `blobstore.Store` type that wraps different objectstore backends:

```go
// Production backends
blobstore.NewS3Store(minioStore)      // S3/MinIO
blobstore.NewFlatfsStore(flatfsStore) // Filesystem

// Testing backends
blobstore.NewMemoryStore()            // In-memory map
blobstore.NewDatastoreStore(ds)       // IPFS datastore
```

## Store Patterns

### Structured Data Stores (via genericstore)

These stores use `genericstore.Store[T]` which provides typed access with automatic serialization. Objects are fully buffered in memory during read/write since they're small (KB-sized).

| Store | Type Parameter | Description |
|-------|----------------|-------------|
| `allocationstore` | `Allocation` | Pending blob allocations |
| `acceptancestore` | `Acceptance` | Accepted/stored blob records |
| `delegationstore` | `delegation.Delegation` | UCAN delegations |
| `receiptstore` | `receipt.AnyReceipt` | UCAN invocation receipts |
| `consolidationstore` | `Consolidation` | Consolidation tracking |

Each store follows a consistent pattern with backend-specific constructors:

```go
// S3/MinIO backends
allocationstore.NewS3Store(minioStore)
acceptancestore.NewS3Store(minioStore)
delegationstore.NewS3Store(minioStore)
receiptstore.NewS3Store(minioStore)

// Datastore backends (LevelDB, in-memory)
allocationstore.NewDatastoreStore(ds)
acceptancestore.NewDatastoreStore(ds)
delegationstore.NewDatastoreStore(ds)
receiptstore.NewDatastoreStore(ds)
```

Note: `receiptstore` also maintains a secondary index (RanLinkIndex) for looking up receipts by their "ran" CID.

### Binary Data Store (direct objectstore wrapper)

`blobstore.Store` wraps `objectstore.Store` directly to preserve streaming. It cannot use genericstore because:

1. Blobs are large (100s of MiB) - buffering would cause OOM
2. Range requests are required for partial reads
3. No serialization needed - data is stored as-is

The blobstore adds only a `KeyEncoder` for consistent key formatting across backends.

### Other Stores

| Store | Backend | Notes |
|-------|---------|-------|
| `claimstore` | delegationstore | Alias for delegation storage |

### Local-Only Stores (`local/`)

These stores are in the `local/` subdirectory because they are inherently filesystem-based and do not participate in S3/filesystem backend selection. They will never have cloud implementations.

| Store | Backend | Notes |
|-------|---------|-------|
| `local/keystore` | LevelDB datastore | Cryptographic key storage, always on disk for security |
| `local/retrievaljournal` | Filesystem | Egress tracking journal with periodic GC, filesystem-only by design |

## Key Encoding

Both patterns use key encoding for backend compatibility:

- **S3/MinIO**: Base32-encoded multihash (lowercase, no prefix)
- **Filesystem**: Base32-encoded with sharding (NextToLast(2))
- **Memory/Datastore**: Plain formatted digest

## Backend Selection

Storage backend is selected via configuration:

- `storage.s3` configured → S3/MinIO backends (`pkg/fx/store/s3`)
- `storage.data_dir` configured → Filesystem backends (`pkg/fx/store/filesystem`)
- Neither configured → In-memory backends (`pkg/fx/store/memory`)
