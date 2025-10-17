package service

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/storacha/piri/pkg/pdp/types"
)

// AddRootsRequest represents a single AddRoots request to be processed
type AddRootsRequest struct {
	ID       uint64
	Request  []types.RootAdd
	Response chan AddRootsResponse
}

// AddRootsResponse represents the response from processing an AddRoots request
type AddRootsResponse struct {
	TxHash common.Hash
	Error  error
}

// BatchedRootsRequest represents multiple AddRoots requests batched together
type BatchedRootsRequest struct {
	Requests   []AddRootsRequest
	FirstAdded *big.Int
}

// DatasetCoordinator manages AddRoots operations for a specific dataset
type DatasetCoordinator struct {
	datasetID        uint64
	queue            chan AddRootsRequest
	localNextPieceID uint64
	lastSyncTime     time.Time
	lastSyncBlock    uint64
	batchSize        int
	batchTimeout     time.Duration
	mu               sync.RWMutex
	service          *PDPService
	ctx              context.Context
	cancel           context.CancelFunc

	// Performance monitoring
	successCount    uint64
	failureCount    uint64
	lastSuccessTime time.Time
	avgBatchSize    float64
}

// CoordinatorConfig holds configuration for dataset coordinators
type CoordinatorConfig struct {
	MaxBatchSize     int           // Maximum roots per transaction
	InitialBatchSize int           // Starting batch size
	BatchTimeout     time.Duration // Maximum time to wait for batch to fill
	QueueSize        int           // Size of the request queue
	ResyncInterval   time.Duration // How often to verify sync with chain
	MaxLocalDrift    uint64        // Maximum allowed drift from chain state
}

// DefaultCoordinatorConfig returns default configuration values
func DefaultCoordinatorConfig() CoordinatorConfig {
	return CoordinatorConfig{
		MaxBatchSize:     20,
		InitialBatchSize: 5,
		BatchTimeout:     100 * time.Millisecond,
		QueueSize:        1000,
		ResyncInterval:   1 * time.Minute,
		MaxLocalDrift:    50, // Allow up to 50 pieces drift before forcing resync
	}
}

// CoordinatorRegistry manages all dataset coordinators
type CoordinatorRegistry struct {
	coordinators map[uint64]*DatasetCoordinator
	config       CoordinatorConfig
	mu           sync.RWMutex
	service      *PDPService
}

// NewCoordinatorRegistry creates a new coordinator registry
func NewCoordinatorRegistry(service *PDPService, config CoordinatorConfig) *CoordinatorRegistry {
	return &CoordinatorRegistry{
		coordinators: make(map[uint64]*DatasetCoordinator),
		config:       config,
		service:      service,
	}
}

// GetOrCreateCoordinator returns an existing coordinator or creates a new one
func (r *CoordinatorRegistry) GetOrCreateCoordinator(ctx context.Context, datasetID uint64) (*DatasetCoordinator, error) {
	r.mu.RLock()
	coordinator, exists := r.coordinators[datasetID]
	r.mu.RUnlock()

	if exists {
		return coordinator, nil
	}

	// Need to create a new coordinator
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if coordinator, exists := r.coordinators[datasetID]; exists {
		return coordinator, nil
	}

	// Create new coordinator
	coordinator, err := r.createCoordinator(ctx, datasetID)
	if err != nil {
		return nil, fmt.Errorf("failed to create coordinator for dataset %d: %w", datasetID, err)
	}

	r.coordinators[datasetID] = coordinator
	return coordinator, nil
}

// createCoordinator creates and initializes a new dataset coordinator
func (r *CoordinatorRegistry) createCoordinator(ctx context.Context, datasetID uint64) (*DatasetCoordinator, error) {
	// Get current nextPieceId from chain
	proofSetID := new(big.Int).SetUint64(datasetID)
	nextPieceId, err := r.service.verifierContract.GetNextPieceId(ctx, proofSetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get next piece ID from chain: %w", err)
	}

	// Create a new context for this coordinator
	coordCtx, cancel := context.WithCancel(ctx)

	coordinator := &DatasetCoordinator{
		datasetID:        datasetID,
		queue:            make(chan AddRootsRequest, r.config.QueueSize),
		localNextPieceID: nextPieceId.Uint64(),
		lastSyncTime:     time.Now(),
		batchSize:        r.config.InitialBatchSize,
		batchTimeout:     r.config.BatchTimeout,
		service:          r.service,
		ctx:              coordCtx,
		cancel:           cancel,
		avgBatchSize:     float64(r.config.InitialBatchSize),
	}

	// Start the queue processor
	go coordinator.processQueue()

	// Start the periodic sync checker
	go coordinator.periodicSync(r.config.ResyncInterval)

	log.Infow("Created new coordinator for dataset",
		"datasetID", datasetID,
		"initialNextPieceId", nextPieceId.Uint64(),
		"batchSize", r.config.InitialBatchSize)

	return coordinator, nil
}

// Submit adds a new AddRoots request to the coordinator's queue
func (c *DatasetCoordinator) Submit(request AddRootsRequest) error {
	select {
	case c.queue <- request:
		return nil
	case <-c.ctx.Done():
		return errors.New("coordinator is shutting down")
	default:
		return errors.New("coordinator queue is full")
	}
}

// processQueue is the main worker loop for processing AddRoots requests
func (c *DatasetCoordinator) processQueue() {
	batch := make([]AddRootsRequest, 0, c.batchSize)
	timer := time.NewTimer(c.batchTimeout)
	defer timer.Stop()

	for {
		select {
		case req := <-c.queue:
			batch = append(batch, req)

			// Check if batch is full
			if len(batch) >= c.batchSize {
				c.processBatch(batch)
				batch = make([]AddRootsRequest, 0, c.batchSize)
				timer.Reset(c.batchTimeout)
			}

		case <-timer.C:
			// Process partial batch after timeout
			if len(batch) > 0 {
				c.processBatch(batch)
				batch = make([]AddRootsRequest, 0, c.batchSize)
			}
			timer.Reset(c.batchTimeout)

		case <-c.ctx.Done():
			// Process remaining batch before shutdown
			if len(batch) > 0 {
				c.processBatch(batch)
			}
			return
		}
	}
}

// processBatch processes a batch of AddRoots requests
func (c *DatasetCoordinator) processBatch(batch []AddRootsRequest) {
	if len(batch) == 0 {
		return
	}

	// Get and update local counter
	c.mu.Lock()
	firstAdded := c.localNextPieceID
	totalPieces := c.calculateTotalPieces(batch)
	c.localNextPieceID += totalPieces
	c.mu.Unlock()

	log.Infow("Processing batch of AddRoots requests",
		"datasetID", c.datasetID,
		"batchSize", len(batch),
		"totalPieces", totalPieces,
		"firstAdded", firstAdded,
		"nextPieceId", c.localNextPieceID)

	// Submit the batch
	txHash, err := c.submitBatchedTransaction(batch, firstAdded)

	// Update performance metrics
	c.updateMetrics(len(batch), err == nil)

	// Send responses to all requests in batch
	for _, req := range batch {
		select {
		case req.Response <- AddRootsResponse{TxHash: txHash, Error: err}:
		default:
			log.Warnw("Failed to send response to AddRoots request", "datasetID", c.datasetID)
		}
	}

	// Handle errors
	if err != nil {
		c.handleBatchError(err, batch, firstAdded)
	}
}

// calculateTotalPieces calculates the total number of pieces in a batch
func (c *DatasetCoordinator) calculateTotalPieces(batch []AddRootsRequest) uint64 {
	var total uint64
	for _, req := range batch {
		total += uint64(len(req.Request))
	}
	return total
}

// submitBatchedTransaction submits a batched transaction for multiple AddRoots requests
func (c *DatasetCoordinator) submitBatchedTransaction(batch []AddRootsRequest, firstAdded uint64) (common.Hash, error) {
	// Combine all roots from the batch
	var allRoots []types.RootAdd
	for _, req := range batch {
		allRoots = append(allRoots, req.Request...)
	}

	log.Infow("Submitting batched transaction",
		"datasetID", c.datasetID,
		"batchCount", len(batch),
		"totalRoots", len(allRoots),
		"firstAdded", firstAdded)

	// Use the internal AddRoots method with the specified firstAdded value
	ctx, cancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer cancel()

	txHash, err := c.service.AddRootsInternal(ctx, c.datasetID, allRoots, firstAdded)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to submit batch: %w", err)
	}

	return txHash, nil
}

// handleBatchError handles errors from batch submission
func (c *DatasetCoordinator) handleBatchError(err error, batch []AddRootsRequest, firstAdded uint64) {
	log.Errorw("Failed to submit batch",
		"datasetID", c.datasetID,
		"error", err,
		"batchSize", len(batch),
		"firstAdded", firstAdded)

	// Check if error is due to signature mismatch (nextPieceId drift)
	if isSignatureMismatchError(err) {
		log.Warnw("Detected signature mismatch, resyncing with chain",
			"datasetID", c.datasetID)
		c.resyncWithChain()

		// Requeue the batch for retry
		c.requeueBatch(batch)
	}

	// Adapt batch size based on failures
	c.adaptBatchSize()
}

// resyncWithChain synchronizes the local counter with the blockchain state
func (c *DatasetCoordinator) resyncWithChain() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
	defer cancel()

	proofSetID := new(big.Int).SetUint64(c.datasetID)
	nextPieceId, err := c.service.verifierContract.GetNextPieceId(ctx, proofSetID)
	if err != nil {
		return fmt.Errorf("failed to get next piece ID from chain: %w", err)
	}

	oldValue := c.localNextPieceID
	c.localNextPieceID = nextPieceId.Uint64()
	c.lastSyncTime = time.Now()

	log.Infow("Resynced local counter with chain",
		"datasetID", c.datasetID,
		"oldValue", oldValue,
		"newValue", c.localNextPieceID,
		"drift", int64(c.localNextPieceID)-int64(oldValue))

	return nil
}

// periodicSync periodically verifies synchronization with blockchain
func (c *DatasetCoordinator) periodicSync(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := c.resyncWithChain(); err != nil {
				log.Errorw("Failed periodic resync",
					"datasetID", c.datasetID,
					"error", err)
			}

		case <-c.ctx.Done():
			return
		}
	}
}

// requeueBatch requeues a failed batch for retry
func (c *DatasetCoordinator) requeueBatch(batch []AddRootsRequest) {
	for _, req := range batch {
		select {
		case c.queue <- req:
			// Successfully requeued
		default:
			// Queue is full, send error response
			select {
			case req.Response <- AddRootsResponse{Error: errors.New("failed to requeue after error")}:
			default:
			}
		}
	}
}

// updateMetrics updates performance metrics
func (c *DatasetCoordinator) updateMetrics(batchSize int, success bool) {
	if success {
		c.successCount++
		c.lastSuccessTime = time.Now()
	} else {
		c.failureCount++
	}

	// Update moving average of batch size
	alpha := 0.1 // Smoothing factor
	c.avgBatchSize = alpha*float64(batchSize) + (1-alpha)*c.avgBatchSize
}

// adaptBatchSize adjusts batch size based on performance
func (c *DatasetCoordinator) adaptBatchSize() {
	successRate := float64(c.successCount) / float64(c.successCount+c.failureCount)

	if successRate < 0.8 && c.batchSize > 1 {
		// Reduce batch size if success rate is low
		c.batchSize = max(1, c.batchSize/2)
		log.Infow("Reduced batch size due to low success rate",
			"datasetID", c.datasetID,
			"newBatchSize", c.batchSize,
			"successRate", successRate)
	} else if successRate > 0.95 && c.batchSize < 20 {
		// Increase batch size if success rate is high
		c.batchSize = min(20, c.batchSize+2)
		log.Infow("Increased batch size due to high success rate",
			"datasetID", c.datasetID,
			"newBatchSize", c.batchSize,
			"successRate", successRate)
	}
}

// isSignatureMismatchError checks if an error is due to signature/nextPieceId mismatch
func isSignatureMismatchError(err error) bool {
	if err == nil {
		return false
	}
	// Check for specific error patterns that indicate signature mismatch
	// This would need to be updated based on actual error messages from the contract
	errStr := err.Error()
	return contains(errStr, "signature") || contains(errStr, "firstAdded") || contains(errStr, "InvalidSignature")
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > 0 && len(substr) > 0 &&
				containsHelper(strings.ToLower(s), strings.ToLower(substr)))
}

func containsHelper(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Shutdown gracefully shuts down the coordinator
func (c *DatasetCoordinator) Shutdown() {
	log.Infow("Shutting down coordinator",
		"datasetID", c.datasetID,
		"successCount", c.successCount,
		"failureCount", c.failureCount,
		"avgBatchSize", c.avgBatchSize)

	c.cancel()
}

// ShutdownRegistry shuts down all coordinators
func (r *CoordinatorRegistry) ShutdownAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, coordinator := range r.coordinators {
		coordinator.Shutdown()
		delete(r.coordinators, id)
	}
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
