package service

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/storacha/piri/pkg/pdp/types"
)

// AddRootsWithCoordinator processes AddRoots requests using the coordinator system
// This method provides high-throughput processing by batching requests per dataset
func (p *PDPService) AddRootsWithCoordinator(ctx context.Context, id uint64, request []types.RootAdd) (common.Hash, error) {
	// Get or create coordinator for this dataset
	coordinator, err := p.coordinatorRegistry.GetOrCreateCoordinator(ctx, id)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get coordinator for dataset %d: %w", id, err)
	}

	// Create a response channel for this request
	responseChan := make(chan AddRootsResponse, 1)

	// Submit request to coordinator queue
	addRootsReq := AddRootsRequest{
		ID:       id,
		Request:  request,
		Response: responseChan,
	}

	if err := coordinator.Submit(addRootsReq); err != nil {
		return common.Hash{}, fmt.Errorf("failed to submit request to coordinator: %w", err)
	}

	// Wait for response with timeout
	select {
	case response := <-responseChan:
		if response.Error != nil {
			return common.Hash{}, response.Error
		}
		return response.TxHash, nil

	case <-ctx.Done():
		return common.Hash{}, ctx.Err()

	case <-time.After(5 * time.Minute):
		return common.Hash{}, fmt.Errorf("timeout waiting for AddRoots response")
	}
}

// EnableCoordinatedAddRoots enables the coordinator system by default
// This can be called to switch the default AddRoots behavior to use coordinators
func (p *PDPService) EnableCoordinatedAddRoots() {
	log.Infow("Enabling coordinated AddRoots for high-throughput processing")
	// This could set a flag in PDPService to route AddRoots through the coordinator
	// For now, users must explicitly call AddRootsWithCoordinator
}

// DisableCoordinatedAddRoots disables the coordinator system
// This reverts to the original AddRoots behavior
func (p *PDPService) DisableCoordinatedAddRoots() {
	log.Infow("Disabling coordinated AddRoots, reverting to sequential processing")
	// Shutdown all coordinators
	if p.coordinatorRegistry != nil {
		p.coordinatorRegistry.ShutdownAll()
	}
}

// GetCoordinatorStats returns performance statistics for a dataset's coordinator
func (p *PDPService) GetCoordinatorStats(datasetID uint64) (map[string]interface{}, error) {
	coordinator, err := p.coordinatorRegistry.GetOrCreateCoordinator(context.Background(), datasetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get coordinator: %w", err)
	}

	// Read stats (protected by mutex)
	coordinator.mu.RLock()
	stats := map[string]interface{}{
		"dataset_id":        coordinator.datasetID,
		"local_next_piece":  coordinator.localNextPieceID,
		"batch_size":        coordinator.batchSize,
		"success_count":     coordinator.successCount,
		"failure_count":     coordinator.failureCount,
		"avg_batch_size":    coordinator.avgBatchSize,
		"last_sync_time":    coordinator.lastSyncTime,
		"last_success_time": coordinator.lastSuccessTime,
	}
	coordinator.mu.RUnlock()

	return stats, nil
}

// GetAllCoordinatorStats returns statistics for all active coordinators
func (p *PDPService) GetAllCoordinatorStats() []map[string]interface{} {
	var allStats []map[string]interface{}

	p.coordinatorRegistry.mu.RLock()
	for datasetID := range p.coordinatorRegistry.coordinators {
		if stats, err := p.GetCoordinatorStats(datasetID); err == nil {
			allStats = append(allStats, stats)
		}
	}
	p.coordinatorRegistry.mu.RUnlock()

	return allStats
}