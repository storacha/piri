----------------------------------------------------------------
-- queues
----------------------------------------------------------------
CREATE TABLE queues (
                        queue TEXT PRIMARY KEY,             -- logical queue name ("manager", "signer", etc.)
                        dedupe_enabled INTEGER NOT NULL
                            DEFAULT 0 -- 0 = no dedup; 1 = permanent dedupe enforced via job_done
) STRICT; -- strict typing; rejects wrong types

----------------------------------------------------------------
-- job_ns
-- Maps (queue, task name) → compact integer namespace id
-- Reduces size of foreign keys and PRIMARY KEYs in large tables.
----------------------------------------------------------------
CREATE TABLE job_ns (
                        id     INTEGER PRIMARY KEY,         -- autoincrement namespace id
                        queue  TEXT    NOT NULL,            -- queue this namespace belongs to
                        name   TEXT    NOT NULL,            -- task name within that queue

                        UNIQUE(queue, name)                 -- ensures one id per (queue,name)
) STRICT;



----------------------------------------------------------------
-- jobs
-- Live job queue; holds only unprocessed / inflight jobs.
-- Each row is a single task payload awaiting execution.
----------------------------------------------------------------
CREATE TABLE jobs (
                      id        INTEGER PRIMARY KEY,          -- rowid table for fast inserts/claims
                      ns_id     INTEGER NOT NULL,             -- foreign key into job_ns.id
                      key       BLOB    NOT NULL,             -- 16- or 32-byte dedupe hash of payload
                      body      BLOB    NOT NULL,             -- raw CBOR or JSON payload
                      created_s INTEGER NOT NULL              -- epoch seconds when inserted
                                                 DEFAULT (strftime('%s')),
                      avail_s   INTEGER NOT NULL,             -- epoch seconds when visible again
                      attempts  INTEGER NOT NULL DEFAULT 0,   -- claim count (for backoff/DLQ)

                      FOREIGN KEY(ns_id) REFERENCES job_ns(id)
) STRICT;

-- Enforces uniqueness of (namespace,payload) among live jobs.
CREATE UNIQUE INDEX jobs_unique_live ON jobs(ns_id, key);

-- Optimizes claim queries (find oldest available task per queue).
CREATE INDEX jobs_claim_idx ON jobs(ns_id, avail_s, id);

----------------------------------------------------------------
-- job_done
-- Permanent dedupe log: every (namespace,payload) that has been processed.
-- Ensures true “never repeat” semantics across all time.
-- WITHOUT ROWID because PRIMARY KEY(ns_id,key) is compact and fixed-width.
----------------------------------------------------------------
CREATE TABLE job_done (
                          ns_id  INTEGER NOT NULL,            -- namespace id (queue,name pair)
                          key    BLOB    NOT NULL,            -- same hash as jobs.key
                          status INTEGER NOT NULL,            -- 1=success, 2=dead-letter, etc.
                          done_s INTEGER NOT NULL             -- epoch seconds when finalized
                              DEFAULT (strftime('%s')),

                          PRIMARY KEY (ns_id, key)            -- composite PK; perfect for WITHOUT ROWID
) WITHOUT ROWID;


----------------------------------------------------------------
-- job_dead
-- Optional dead-letter queue retaining permanently failed jobs.
-- Does not participate in dedupe enforcement unless desired.
----------------------------------------------------------------
CREATE TABLE job_dead (
                          id        INTEGER PRIMARY KEY,      -- preserves original job id for tracing
                          ns_id     INTEGER NOT NULL,         -- namespace (queue,name)
                          key       BLOB    NOT NULL,         -- dedupe hash
                          body      BLOB    NOT NULL,         -- original payload
                          attempts  INTEGER NOT NULL,         -- attempt count at failure
                          reason    TEXT    NOT NULL,         -- failure class or label
                          error     TEXT    NOT NULL,         -- human-readable error message
                          moved_s   INTEGER NOT NULL          -- epoch seconds moved into DLQ
                              DEFAULT (strftime('%s')),

                          FOREIGN KEY(ns_id) REFERENCES job_ns(id)
) STRICT;
