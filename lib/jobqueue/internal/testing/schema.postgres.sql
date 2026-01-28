-- Combined PostgreSQL schema for testing
-- This includes both the classic queue and dedup queue schemas.

----------------------------------------------------------------
-- Classic Queue Schema (from lib/jobqueue/queue/schema.postgres.sql)
----------------------------------------------------------------

CREATE TABLE IF NOT EXISTS jobqueue (
  id TEXT PRIMARY KEY DEFAULT ('m_' || replace(gen_random_uuid()::text, '-', '')),
  created TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  queue TEXT NOT NULL,
  body BYTEA NOT NULL,
  timeout TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  received INTEGER NOT NULL DEFAULT 0
);

-- Trigger function for auto-updating the updated timestamp
CREATE OR REPLACE FUNCTION jobqueue_update_timestamp()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Drop and recreate trigger (PostgreSQL doesn't have CREATE TRIGGER IF NOT EXISTS in older versions)
DROP TRIGGER IF EXISTS jobqueue_updated_timestamp ON jobqueue;
CREATE TRIGGER jobqueue_updated_timestamp
  BEFORE UPDATE ON jobqueue
  FOR EACH ROW
  EXECUTE FUNCTION jobqueue_update_timestamp();

CREATE INDEX IF NOT EXISTS jobqueue_queue_created_idx ON jobqueue (queue, created);

-- Dead letter queue for permanently failed jobs (classic queue)
CREATE TABLE IF NOT EXISTS jobqueue_dead (
    id TEXT PRIMARY KEY,
    created TIMESTAMPTZ NOT NULL,
    updated TIMESTAMPTZ NOT NULL,
    queue TEXT NOT NULL,
    body BYTEA NOT NULL,
    timeout TIMESTAMPTZ NOT NULL,
    received INTEGER NOT NULL,
    job_name TEXT NOT NULL,
    failure_reason TEXT NOT NULL,
    error_message TEXT NOT NULL,
    moved_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS jobqueue_dead_queue_moved_at_idx ON jobqueue_dead (queue, moved_at);

----------------------------------------------------------------
-- Dedup Queue Schema (from lib/jobqueue/dedup/schema.postgres.sql)
----------------------------------------------------------------

CREATE TABLE IF NOT EXISTS queues (
    queue TEXT PRIMARY KEY,             -- logical queue name ("manager", "signer", etc.)
    dedupe_enabled INTEGER NOT NULL
        DEFAULT 0 -- 0 = no dedup; 1 = permanent dedupe enforced via job_done
);

----------------------------------------------------------------
-- job_ns
-- Maps (queue, task name) -> compact integer namespace id
-- Reduces size of foreign keys and PRIMARY KEYs in large tables.
----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS job_ns (
    id     SERIAL PRIMARY KEY,          -- autoincrement namespace id
    queue  TEXT   NOT NULL,             -- queue this namespace belongs to
    name   TEXT   NOT NULL,             -- task name within that queue

    UNIQUE(queue, name)                 -- ensures one id per (queue,name)
);

----------------------------------------------------------------
-- jobs
-- Live job queue; holds only unprocessed / inflight jobs.
-- Each row is a single task payload awaiting execution.
----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS jobs (
    id        SERIAL PRIMARY KEY,         -- auto-increment for fast inserts/claims
    ns_id     INTEGER NOT NULL,           -- foreign key into job_ns.id
    key       BYTEA   NOT NULL,           -- 16- or 32-byte dedupe hash of payload
    body      BYTEA   NOT NULL,           -- raw CBOR or JSON payload
    created_s BIGINT  NOT NULL            -- epoch seconds when inserted
        DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
    avail_s   BIGINT  NOT NULL,           -- epoch seconds when visible again
    attempts  INTEGER NOT NULL DEFAULT 0, -- claim count (for backoff/DLQ)

    FOREIGN KEY(ns_id) REFERENCES job_ns(id)
);

-- Enforces uniqueness of (namespace,payload) among live jobs.
CREATE UNIQUE INDEX IF NOT EXISTS jobs_unique_live ON jobs(ns_id, key);

-- Optimizes claim queries (find oldest available task per queue).
CREATE INDEX IF NOT EXISTS jobs_claim_idx ON jobs(ns_id, avail_s, id);

----------------------------------------------------------------
-- job_done
-- Permanent dedupe log: every (namespace,payload) that has been processed.
-- Ensures true "never repeat" semantics across all time.
-- Note: PostgreSQL doesn't have WITHOUT ROWID, but composite PK is still efficient.
----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS job_done (
    ns_id  INTEGER NOT NULL,            -- namespace id (queue,name pair)
    key    BYTEA   NOT NULL,            -- same hash as jobs.key
    status INTEGER NOT NULL,            -- 1=success, 2=dead-letter, etc.
    done_s BIGINT  NOT NULL             -- epoch seconds when finalized
        DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,

    PRIMARY KEY (ns_id, key)            -- composite PK
);

----------------------------------------------------------------
-- job_dead
-- Optional dead-letter queue retaining permanently failed jobs (dedup version).
-- Does not participate in dedupe enforcement unless desired.
----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS job_dead (
    id        SERIAL PRIMARY KEY,       -- auto-increment for tracing
    ns_id     INTEGER NOT NULL,         -- namespace (queue,name)
    key       BYTEA   NOT NULL,         -- dedupe hash
    body      BYTEA   NOT NULL,         -- original payload
    attempts  INTEGER NOT NULL,         -- attempt count at failure
    reason    TEXT    NOT NULL,         -- failure class or label
    error     TEXT    NOT NULL,         -- human-readable error message
    moved_s   BIGINT  NOT NULL          -- epoch seconds moved into DLQ
        DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,

    FOREIGN KEY(ns_id) REFERENCES job_ns(id)
);
