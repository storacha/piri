# Garbage Collection System

The PDP server implements a comprehensive garbage collection system to automatically clean up temporary stash files and unused blob data, preventing storage bloat and ensuring efficient resource utilization.

## Overview

The garbage collection system consists of three main components:

1. **Stash Cleanup**: Removes temporary stash files after data is successfully stored in blobstore
2. **Blob Cleanup**: Removes blob data when its reference count reaches zero
3. **Startup Cleanup**: Removes orphaned stash files on server startup

## Architecture

### Cleanup Task System

The cleanup system uses the existing task scheduler to handle cleanup operations asynchronously:

- **CleanupTask**: Main task implementation that handles both stash and blob cleanup
- **StartupCleanupService**: Handles orphaned stash cleanup on server startup
- **CleanupMetricsService**: Tracks cleanup operation statistics
- **CleanupService**: Orchestrates all cleanup operations

### Database Schema

The system uses the existing database schema with the following key fields:

- `parked_pieces.cleanup_task_id`: Tracks cleanup task assignment
- `pdp_piecerefs.proofset_refcount`: Reference counting for blob cleanup
- `task` table: Stores cleanup task information

### Reference Counting

The system uses reference counting to determine when blob data can be safely deleted:

- When a piece is added to a proof set, the reference count is incremented
- When a root is removed from a proof set, the reference count is decremented
- When the reference count reaches zero, the blob is scheduled for deletion

## Cleanup Lifecycle

### 1. Piece Upload Process

```
Piece Upload → Create Stash → ParkedPiece (complete=false)
```

### 2. Stash to Blobstore Transfer

```
ParkPieceTask → Copy to Blobstore → ParkedPiece (complete=true)
```

### 3. Stash Cleanup

```
CleanupTask → Remove Stash → Update cleanup_task_id
```

### 4. Blob Cleanup on Root Removal

```
RemoveRoot → Decrement refcount → If 0, schedule blob cleanup
CleanupTask → Remove Blob from Blobstore
```

## Configuration

The cleanup system is configurable through the `CleanupConfig` structure:

```go
type CleanupConfig struct {
    StashCleanupEnabled     bool          // Enable/disable stash cleanup
    StashCleanupInterval    time.Duration // How often to check for stash cleanup
    StashRetentionPeriod    time.Duration // How long to keep stash files
    
    BlobCleanupEnabled      bool          // Enable/disable blob cleanup
    BlobCleanupInterval     time.Duration // How often to check for blob cleanup
    
    StartupCleanupEnabled   bool          // Enable/disable startup cleanup
    
    MaxRetries             int           // Maximum retry attempts
    RetryBackoffMultiplier float64       // Exponential backoff multiplier
    BatchSize              int           // Number of items to process per batch
    
    MetricsEnabled         bool          // Enable/disable metrics collection
    MetricsInterval        time.Duration // How often to collect metrics
}
```

## Usage

### Starting the Cleanup Service

```go
config := DefaultCleanupConfig()
cleanupService := NewCleanupService(db, blobstore, stashstore, config)

// Start the service
if err := cleanupService.Start(ctx); err != nil {
    log.Fatal("Failed to start cleanup service:", err)
}

// Register with task scheduler (if using scheduler)
cleanupService.RegisterWithScheduler(scheduler.AddTask)
```

### Manual Cleanup

```go
// Trigger manual cleanup
if err := cleanupService.ManualCleanup(ctx); err != nil {
    log.Error("Manual cleanup failed:", err)
}
```

### Monitoring

```go
// Get cleanup status
status, err := cleanupService.GetCleanupStatus(ctx)
if err != nil {
    log.Error("Failed to get cleanup status:", err)
}

// Get metrics
metrics := cleanupService.GetMetrics()
log.Infof("Stash cleanup success: %d, failures: %d", 
    metrics.StashCleanupSuccess, metrics.StashCleanupFailure)
log.Infof("Space reclaimed: %d bytes", metrics.SpaceReclaimedBytes)
```

## Database Migrations

The system includes database migrations to add cleanup tracking:

```sql
-- Add cleanup_task_id column to parked_pieces table
ALTER TABLE parked_pieces ADD COLUMN cleanup_task_id BIGINT;

-- Add foreign key constraint
ALTER TABLE parked_pieces ADD CONSTRAINT fk_parked_pieces_cleanup_task 
    FOREIGN KEY (cleanup_task_id) REFERENCES task(id) ON DELETE SET NULL;

-- Add indexes for efficient queries
CREATE INDEX idx_parked_pieces_cleanup_task ON parked_pieces(cleanup_task_id);
CREATE INDEX idx_parked_pieces_complete_cleanup ON parked_pieces(complete, cleanup_task_id) 
    WHERE complete = TRUE AND cleanup_task_id IS NULL;
CREATE INDEX idx_pdp_piecerefs_zero_refcount ON pdp_piecerefs(proofset_refcount) 
    WHERE proofset_refcount = 0;
```

## Error Handling

The cleanup system implements robust error handling:

- **Idempotent Operations**: Cleanup operations can be safely retried
- **Exponential Backoff**: Failed operations are retried with increasing delays
- **Graceful Degradation**: Individual cleanup failures don't stop the entire system
- **Comprehensive Logging**: All operations are logged for debugging

## Metrics

The system tracks various metrics:

- Stash cleanup success/failure counts
- Blob cleanup success/failure counts
- Startup cleanup success/failure counts
- Total space reclaimed
- Last cleanup operation time

## Testing

The system includes comprehensive unit tests:

```bash
# Run cleanup task tests
go test ./pkg/pdp/tasks -run TestCleanupTask

# Run cleanup service tests
go test ./pkg/pdp/service -run TestCleanup
```

## Integration

The cleanup system integrates with existing components:

- **Task Scheduler**: Uses the existing task scheduling infrastructure
- **Blobstore**: Extends blobstore interface for deletion operations
- **Stash Store**: Uses existing stash store for file management
- **Database**: Uses existing models and triggers for reference counting

## Future Enhancements

The system is designed to be extensible for future needs:

- **Batching**: Process multiple cleanup operations in batches for efficiency
- **Cloud Storage**: Support for cloud blobstore deletion operations
- **Advanced Metrics**: Integration with monitoring systems
- **Configuration Management**: Integration with configuration management systems
- **Cleanup Policies**: Configurable retention policies based on piece types
