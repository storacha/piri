package service

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/service/models"
)

type ProofSet struct {
	ID                 int64
	Roots              []RootEntry
	NextChallengeEpoch int64
}

type RootEntry struct {
	RootID        uint64 `json:"rootId"`
	RootCID       string `json:"rootCid"`
	SubrootCID    string `json:"subrootCid"`
	SubrootOffset int64  `json:"subrootOffset"`
}

func (p *PDPService) ProofSet(ctx context.Context, id int64) (*ProofSet, error) {
	// Retrieve the proof set record.
	var proofSet models.PDPProofSet
	if err := p.db.WithContext(ctx).First(&proofSet, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("proof set not found")
		}
		return nil, fmt.Errorf("failed to retrieve proof set: %w", err)
	}

	if proofSet.Service != p.name {
		return nil, fmt.Errorf("proof set does not belong to your service")
	}

	// Retrieve the roots associated with the proof set.
	var roots []models.PDPProofsetRoot
	if err := p.db.WithContext(ctx).
		Where("proofset_id = ?", id).
		Order("root_id, subroot_offset").
		Find(&roots).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve proof set roots: %w", err)
	}

	// Step 5: Build the response.
	response := &ProofSet{
		ID: proofSet.ID,
		// TODO this will panic if ProveAtEpoch is nill, which it is when the proofset is first created
		NextChallengeEpoch: *proofSet.ProveAtEpoch,
	}
	for _, r := range roots {
		response.Roots = append(response.Roots, RootEntry{
			RootID:        uint64(r.RootID),
			RootCID:       r.Root,
			SubrootCID:    r.Subroot,
			SubrootOffset: r.SubrootOffset,
		})
	}

	return response, nil
}
