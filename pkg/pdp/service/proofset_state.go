package service

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/storacha/piri/pkg/pdp/service/contract"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/types"
)

func (p *PDPService) GetProofSetState(ctx context.Context, id uint64) (res types.ProofSetState, retErr error) {
	log.Info("getting proof set state")
	defer func() {
		if retErr != nil {
			log.Errorw("failed to get proof set state", "error", retErr)
		} else {
			log.Info("got proof set state")
		}
	}()

	// get the current epoch of the chain
	head, err := p.chainClient.ChainHead(ctx)
	if err != nil {
		return types.ProofSetState{}, fmt.Errorf("failed to get chain head: %w", err)
	}
	currentEpoch := int64(head.Height())

	// get the proof set details
	var ps models.PDPProofSet
	if err := p.db.
		WithContext(ctx).
		Where("service = ?", p.name).
		Where("id = ?", id).
		First(&ps).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return types.ProofSetState{}, types.NewErrorf(types.KindNotFound, "no proof set found")
		}
		return types.ProofSetState{}, fmt.Errorf("failed to retrieve proof set: %w", err)
	}

	// check if we are actively proving
	var provingTasks int64
	if err := p.db.WithContext(ctx).
		Model(&models.PDPProveTask{}).
		Where("proofset_id = ?", id).
		Count(&provingTasks).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			provingTasks = 0
		} else {
			return types.ProofSetState{}, fmt.Errorf("failed to retrieve proof set tasks: %w", err)
		}
	}

	// don't get contract state if ps isn't initialized since it will fail
	if !ps.InitReady {
		return types.ProofSetState{
			ID: id,
		}, nil
	}

	cs, err := p.getContractState(big.NewInt(int64(id)))
	if err != nil {
		return types.ProofSetState{}, fmt.Errorf("failed to get contract state: %w", err)
	}

	result := types.ProofSetState{
		ID:                     id,
		Initialized:            ps.InitReady,
		NextChallengeEpoch:     int64OrZero(ps.ProveAtEpoch),
		PreviousChallengeEpoch: int64OrZero(ps.PrevChallengeRequestEpoch),
		ProvingPeriod:          int64OrZero(ps.ProvingPeriod),
		ChallengeWindow:        int64OrZero(ps.ChallengeWindow),
		CurrentEpoch:           currentEpoch,
		IsProving:              provingTasks > 0,
		ContractState:          cs,
	}

	if result.NextChallengeEpoch > 0 && result.ChallengeWindow >= 0 {
		inWindow := currentEpoch >= result.NextChallengeEpoch && currentEpoch < (result.NextChallengeEpoch+result.ChallengeWindow)
		result.HasProven = inWindow && ps.ChallengeRequestMsgHash == nil
	}

	if result.NextChallengeEpoch > 0 {
		result.ChallengedIssued = currentEpoch >= result.NextChallengeEpoch
	}
	if result.NextChallengeEpoch > 0 && result.ChallengeWindow > 0 {
		result.InChallengeWindow = currentEpoch < (result.NextChallengeEpoch + result.ChallengeWindow)
		result.IsInFaultState = currentEpoch > (result.NextChallengeEpoch + result.ChallengeWindow)
	}

	return result, nil
}

func (p *PDPService) getContractState(id *big.Int) (types.ProofSetContractState, error) {

	// Get the listener address for this proof set from the PDPVerifier contract
	pdpVerifier, err := p.contractClient.NewPDPVerifier(contract.Addresses().PDPVerifier, p.contractBackend)
	if err != nil {
		return types.ProofSetContractState{}, fmt.Errorf("failed to instantiate PDPVerifier contract: %w", err)
	}

	ownerAddr1, ownerAddre2, err := pdpVerifier.GetProofSetOwner(nil, id)
	if err != nil {
		return types.ProofSetContractState{}, fmt.Errorf("failed to retrieve owner address: %w", err)
	}

	nextChallengeEpoch, err := pdpVerifier.GetNextChallengeEpoch(nil, id)
	if err != nil {
		return types.ProofSetContractState{}, fmt.Errorf("failed to retrieve next challenge epoch: %w", err)
	}

	challengeRange, err := pdpVerifier.GetChallengeRange(nil, id)
	if err != nil {
		return types.ProofSetContractState{}, fmt.Errorf("failed to retrieve challenge range: %w", err)
	}

	scheduledRemovals, err := pdpVerifier.GetScheduledRemovals(nil, id)
	if err != nil {
		return types.ProofSetContractState{}, fmt.Errorf("failed to retrieve scheduled removals: %w", err)
	}

	// If gas used is 0 fee is maximized
	proofFee, err := pdpVerifier.CalculateProofFee(nil, id, big.NewInt(0))
	if err != nil {
		return types.ProofSetContractState{}, fmt.Errorf("failed to calculate proof fee: %w", err)
	}
	// Add 2x buffer for certainty (as is done in the prove task)
	proofFeeBuffer := new(big.Int).Mul(proofFee, big.NewInt(3))

	listenerAddr, err := pdpVerifier.GetProofSetListener(nil, id)
	if err != nil {
		return types.ProofSetContractState{}, fmt.Errorf("failed to get listener address for proof set %d: %w", id, err)
	}

	// Determine the next challenge window start by consulting the listener
	provingSchedule, err := p.contractClient.NewIPDPProvingSchedule(listenerAddr, p.contractBackend)
	if err != nil {
		return types.ProofSetContractState{}, fmt.Errorf("failed to create proving schedule binding, check that listener has proving schedule methods: %w", err)
	}
	nextChallengeWindowStart, err := provingSchedule.NextChallengeWindowStart(nil, id)
	if err != nil {
		return types.ProofSetContractState{}, fmt.Errorf("failed to get next challenge window start: %w", err)
	}
	maxProvingPeriod, err := provingSchedule.GetMaxProvingPeriod(nil)
	if err != nil {
		return types.ProofSetContractState{}, fmt.Errorf("failed to get max proving period: %w", err)
	}
	chalWindow, err := provingSchedule.ChallengeWindow(nil)
	if err != nil {
		return types.ProofSetContractState{}, fmt.Errorf("failed to get challenge window: %w", err)
	}

	removeIdx := make([]uint64, len(scheduledRemovals))
	for i, idx := range scheduledRemovals {
		removeIdx[i] = idx.Uint64()
	}

	return types.ProofSetContractState{
		Owners:                   []common.Address{ownerAddr1, ownerAddre2},
		NextChallengeWindowStart: nextChallengeWindowStart.Uint64(),
		NextChallengeEpoch:       nextChallengeEpoch.Uint64(),
		MaxProvingPeriod:         maxProvingPeriod,
		ChallengeWindow:          chalWindow.Uint64(),
		ChallengeRange:           challengeRange.Uint64(),
		ScheduledRemovals:        removeIdx,
		ProofFee:                 proofFee.Uint64(),
		ProofFeeBuffered:         proofFeeBuffer.Uint64(),
	}, nil

}

func int64OrZero(in *int64) int64 {
	if in == nil {
		return 0
	}
	return *in
}
