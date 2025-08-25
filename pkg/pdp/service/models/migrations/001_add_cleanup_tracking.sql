-- Migration: Add cleanup tracking to parked_pieces table
-- This migration adds the cleanup_task_id field to track cleanup operations

-- Add cleanup_task_id column to parked_pieces table
ALTER TABLE parked_pieces ADD COLUMN cleanup_task_id BIGINT;

-- Add foreign key constraint for cleanup_task_id
ALTER TABLE parked_pieces ADD CONSTRAINT fk_parked_pieces_cleanup_task 
    FOREIGN KEY (cleanup_task_id) REFERENCES task(id) ON DELETE SET NULL;

-- Add index for efficient cleanup task queries
CREATE INDEX idx_parked_pieces_cleanup_task ON parked_pieces(cleanup_task_id);

-- Add index for finding completed pieces that need cleanup
CREATE INDEX idx_parked_pieces_complete_cleanup ON parked_pieces(complete, cleanup_task_id) 
    WHERE complete = TRUE AND cleanup_task_id IS NULL;

-- Add index for finding pieces with zero reference count
CREATE INDEX idx_pdp_piecerefs_zero_refcount ON pdp_piecerefs(proofset_refcount) 
    WHERE proofset_refcount = 0;
