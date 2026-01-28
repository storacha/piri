-- PostgreSQL version of the jobqueue schema

-- Enable pgcrypto extension for gen_random_bytes() function
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS jobqueue (
  id TEXT PRIMARY KEY DEFAULT ('m_' || lower(encode(gen_random_bytes(16), 'hex'))),
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

-- Dead letter queue for permanently failed jobs
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
