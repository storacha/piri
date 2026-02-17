# Database

Piri uses databases to manage operational state, job queues, and task scheduling. This page explains the database architecture and helps you choose the right backend for your deployment.

## Logical Databases

Piri maintains four logical databases, each serving a distinct purpose:

| Database | Purpose |
|----------|---------|
| **Scheduler** | Task engine state and PDP proof scheduling |
| **Replicator** | Data replication job tracking |
| **Aggregator** | CommP hash aggregation job queue |
| **Egress Tracker** | Data egress operation tracking |

## SQLite Mode (Default)

SQLite is the default database backend, requiring no additional configuration.

### File Locations

When using SQLite, Piri creates separate database files under your configured `data_dir`:

```
{data_dir}/
├── replicator/
│   └── replicator.db
├── pdp/
│   └── state/
│       └── state.db
├── aggregator/
│   └── jobqueue/
│       └── jobqueue.db
└── egress_tracker/
    └── jobqueue/
        └── jobqueue.db
```

### Characteristics

- **Write-Ahead Logging (WAL)**: Enables concurrent read access while writing
- **Single Writer**: SQLite only supports one writer at a time (1 connection per database)
- **Zero Configuration**: Works out of the box with just `data_dir` set

## PostgreSQL Mode

PostgreSQL mode uses a single database instance with separate schemas for each logical database.

### Schema Layout

All four logical databases share one PostgreSQL database, isolated by schema:

- `replicator` schema
- `scheduler` schema
- `aggregator` schema
- `egress_tracker` schema

### Characteristics

- **Connection Pooling**: Configurable max connections and idle pool size
- **Concurrent Writers**: Supports multiple simultaneous writers
- **External Management**: Database runs separately from Piri

### Configuration

See [Database Configuration](../configuration/repo/database.md) for PostgreSQL setup.

## Choosing a Backend

| Consideration | SQLite | PostgreSQL |
|---------------|--------|------------|
| Setup complexity | None | Requires external database |
| Concurrent writers | Single | Multiple |
| Scaling | Single node | Multiple Piri instances |
| Operational overhead | Low | Higher |
| Backup strategy | File copy | Database dumps |

**Recommended:**

- **SQLite**: Development, testing, single-node deployments
- **PostgreSQL**: Production environments, high availability requirements, multiple Piri instances sharing state

## Backend Switching

> **Warning**: You cannot switch database backends after initial setup. There is no migration path between SQLite and PostgreSQL.

Choose your database backend before storing any data. If you need to switch backends later, you must start with a fresh Piri installation.
