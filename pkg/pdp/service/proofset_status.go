package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/types"
)

func (p *PDPService) GetProofSetStatus(ctx context.Context, txHash common.Hash) (*types.ProofSetStatus, error) {
	var proofSetCreate models.PDPProofsetCreate
	if err := p.db.WithContext(ctx).
		Preload("MessageWait").
		Where("create_message_hash = ?", txHash.Hex()).
		First(&proofSetCreate).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, types.NewErrorf(types.KindNotFound, "proof set with transaction hash %s not found", txHash.String())
		}
		return nil, fmt.Errorf("failed to retrieve proof set creation: %w", err)
	}

	if proofSetCreate.Service != p.name {
		return nil, fmt.Errorf("proof set creation not for given service")
	}

	var id uint64
	if proofSetCreate.ProofsetCreated {
		// The proof set has been created, get the proofSetId from pdp_proof_sets
		var proofSet models.PDPProofSet
		if err := p.db.WithContext(ctx).
			Where("create_message_hash = ?", txHash.Hex()).
			Find(&proofSet).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("proof set not found despite proofset_created = true")
			}
			return nil, fmt.Errorf("failed to retrieve proof set: %w", err)
		}
		id = uint64(proofSet.ID)

	}

	return &types.ProofSetStatus{
		TxHash:   common.HexToHash(proofSetCreate.CreateMessageHash),
		TxStatus: proofSetCreate.MessageWait.TxStatus,
		Created:  proofSetCreate.ProofsetCreated,
		ID:       id,
	}, nil
}
