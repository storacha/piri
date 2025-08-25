package service

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/store/stashstore"
)

type StartupCleanupService struct {
	db *gorm.DB
	ss stashstore.Stash
}

func NewStartupCleanupService(db *gorm.DB, ss stashstore.Stash) *StartupCleanupService {
	return &StartupCleanupService{
		db: db,
		ss: ss,
	}
}

func (s *StartupCleanupService) CleanupOrphanedStashes(ctx context.Context) error {
	log.Info("starting orphaned stash cleanup")

	// Get all stash files from the filesystem
	stashFiles, err := s.getStashFiles()
	if err != nil {
		return fmt.Errorf("getting stash files: %w", err)
	}

	// Get all stash URLs from the database
	dbStashURLs, err := s.getStashURLsFromDB(ctx)
	if err != nil {
		return fmt.Errorf("getting stash URLs from database: %w", err)
	}

	// Find orphaned stash files
	orphanedFiles := s.findOrphanedFiles(stashFiles, dbStashURLs)

	// Remove orphaned files
	removedCount := 0
	for _, orphanedFile := range orphanedFiles {
		if err := s.removeOrphanedStash(ctx, orphanedFile); err != nil {
			log.Errorw("failed to remove orphaned stash", "file", orphanedFile, "error", err)
			continue
		}
		removedCount++
	}

	log.Infow("startup cleanup completed", "removed_count", removedCount, "total_orphaned", len(orphanedFiles))
	return nil
}

func (s *StartupCleanupService) getStashFiles() ([]string, error) {
	// This is a simplified implementation
	// You'll need to implement this based on your stash store implementation
	// For LocalStashStore, you would scan the stash directory
	var files []string

	// Get the stash directory path from the stash store
	// This is a placeholder - you'll need to implement this based on your stash store
	stashDir := "/tmp/stash" // Placeholder path

	entries, err := os.ReadDir(stashDir)
	if err != nil {
		return nil, fmt.Errorf("reading stash directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".tmp") {
			files = append(files, entry.Name())
		}
	}

	return files, nil
}

func (s *StartupCleanupService) getStashURLsFromDB(ctx context.Context) (map[string]bool, error) {
	var refs []models.ParkedPieceRef
	err := s.db.WithContext(ctx).
		Select("data_url").
		Where("data_url LIKE 'file://%'").
		Find(&refs).Error
	if err != nil {
		return nil, fmt.Errorf("querying parked piece refs: %w", err)
	}

	stashURLs := make(map[string]bool)
	for _, ref := range refs {
		stashURLs[ref.DataURL] = true
	}

	return stashURLs, nil
}

func (s *StartupCleanupService) findOrphanedFiles(stashFiles []string, dbStashURLs map[string]bool) []string {
	var orphaned []string

	for _, file := range stashFiles {
		// Extract stash ID from filename
		stashID := strings.TrimSuffix(file, ".tmp")

		// Check if this stash ID exists in the database
		found := false
		for dbURL := range dbStashURLs {
			if strings.Contains(dbURL, stashID) {
				found = true
				break
			}
		}

		if !found {
			orphaned = append(orphaned, file)
		}
	}

	return orphaned
}

func (s *StartupCleanupService) removeOrphanedStash(ctx context.Context, filename string) error {
	// Extract stash ID from filename
	stashIDStr := strings.TrimSuffix(filename, ".tmp")

	// Parse stash ID to UUID
	stashID, err := uuid.Parse(stashIDStr)
	if err != nil {
		return fmt.Errorf("parsing stash ID: %w", err)
	}

	// Remove the stash file
	if err := s.ss.StashRemove(ctx, stashID); err != nil {
		return fmt.Errorf("removing orphaned stash: %w", err)
	}

	log.Infow("removed orphaned stash", "stash_id", stashID, "filename", filename)
	return nil
}
