package tasks

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
	chaintypes "github.com/filecoin-project/lotus/chain/types"
	"go.opentelemetry.io/otel"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/storacha/filecoin-services/go/evmerrors"

	"github.com/storacha/piri/lib/telemetry"
	"github.com/storacha/piri/pkg/pdp/chainsched"
	"github.com/storacha/piri/pkg/pdp/ethereum"
	"github.com/storacha/piri/pkg/pdp/promise"
	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
)

var _ scheduler.TaskInterface = &NextProvingPeriodTask{}

type NextProvingPeriodTask struct {
	db        *gorm.DB
	ethClient bind.ContractBackend
	verifier  smartcontracts.Verifier
	service   smartcontracts.Service
	sender    ethereum.Sender

	fil ChainAPI

	addFunc promise.Promise[scheduler.AddTaskFunc]

	taskFailure *telemetry.Counter
}

func NewNextProvingPeriodTask(
	db *gorm.DB,
	ethClient bind.ContractBackend,
	api ChainAPI,
	chainSched *chainsched.Scheduler,
	sender ethereum.Sender,
	verifier smartcontracts.Verifier,
	service smartcontracts.Service,
) (*NextProvingPeriodTask, error) {
	meter := otel.GetMeterProvider().Meter("github.com/storacha/piri/pkg/pdp/tasks")
	pdpNextFailureCounter, err := telemetry.NewCounter(
		meter,
		"pdp_next_failure",
		"records failure of next pdp task",
		"1",
	)
	if err != nil {
		return nil, err
	}
	n := &NextProvingPeriodTask{
		db:          db,
		ethClient:   ethClient,
		sender:      sender,
		fil:         api,
		verifier:    verifier,
		service:     service,
		taskFailure: pdpNextFailureCounter,
	}

	if err := chainSched.AddHandler(func(ctx context.Context, revert, apply *chaintypes.TipSet) error {
		if apply == nil {
			return nil
		}

		// Now query the db for proof sets needing nextProvingPeriod
		var toCallNext []struct {
			ProofSetID int64
		}
		err := db.WithContext(ctx).
			Model(&models.PDPProofSet{}).
			Select("id as proof_set_id").
			Where("challenge_request_task_id IS NULL").
			Where("(prove_at_epoch + challenge_window) <= ?", apply.Height()).
			Find(&toCallNext).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("failed to select proof sets needing nextProvingPeriod: %w", err)
		}

		for _, ps := range toCallNext {
			n.addFunc.Val(ctx)(func(id scheduler.TaskID, tx *gorm.DB) (shouldCommit bool, seriousError error) {
				// Update pdp_proof_sets to set challenge_request_task_id = id
				// Query 2: Update pdp_proof_sets to set challenge_request_task_id
				result := tx.Model(&models.PDPProofSet{}).
					Where("id = ? AND challenge_request_task_id IS NULL", ps.ProofSetID).
					Update("challenge_request_task_id", id)
				if result.Error != nil {
					return false, fmt.Errorf("failed to update pdp_proof_sets: %w", result.Error)
				}
				if result.RowsAffected == 0 {
					// Someone else might have already scheduled the task
					return false, nil
				}

				return true, nil
			})
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to register pdp NextProvingPersionTask: %w", err)
	}

	return n, nil
}

// adjustNextProveAt fixes the "Next challenge epoch must fall within the next challenge window" contract error
// by calculating a proper next_prove_at epoch that's guaranteed to be valid.
//
// The contract requires:
// 1. next_prove_at >= currentHeight + challengeFinality (enough time for tx processing)
// 2. next_prove_at must fall within a challenge window boundary (windows are at multiples of challengeWindow)
//
// Algorithm: advance whole proving periods until the window [start, start+challengeWindow] reaches
// minRequiredEpoch, then clamp to the earliest epoch that satisfies both rules (usually the window start,
// or minRequiredEpoch if that falls inside the window).
func adjustNextProveAt(nextProveAt int64, minRequiredEpoch int64, provingPeriod int64, challengeWindow int64) int64 {
	// Fall back to minimum required epoch if metadata is missing
	if provingPeriod <= 0 || challengeWindow <= 0 {
		epoch := nextProveAt
		if epoch < minRequiredEpoch {
			epoch = minRequiredEpoch
		}
		return epoch
	}

	// nextProveAt marks the beginning of the current challenge window.
	// When we miss a proving period the next valid window is always a multiple of the `provingPeriod` epochs laster.
	// Slide the window forward by while proving periods until it covers minRequiredEpoch,
	// which embeds the challenge finality rule.
	windowStart := nextProveAt
	windowEnd := windowStart + challengeWindow

	// Move forward by whole proving periods until the window reaches the minimum required epoch.
	for windowEnd < minRequiredEpoch {
		windowStart += provingPeriod
		windowEnd += provingPeriod
	}

	// At this point the contract will accept any epoch inside [windowStart, windowEnd].
	// Chose one that also satisfies the finality requirement by clamping minRequiredEpoch to that range.
	adjusted := windowStart
	if minRequiredEpoch > adjusted {
		adjusted = minRequiredEpoch
	}
	if adjusted > windowEnd {
		adjusted = windowEnd
	}

	return adjusted
}

func (n *NextProvingPeriodTask) Do(taskID scheduler.TaskID) (done bool, err error) {
	ctx := context.Background()
	defer func() {
		if err != nil {
			n.taskFailure.Inc(ctx)
		}
	}()
	// Select the proof set where challenge_request_task_id equals taskID and prove_at_epoch is not NULL
	var pdp models.PDPProofSet
	err = n.db.WithContext(ctx).
		Model(&models.PDPProofSet{}).
		Where("challenge_request_task_id = ? AND prove_at_epoch IS NOT NULL", taskID).
		Select("id", "challenge_window", "proving_period").
		First(&pdp).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// No matching proof set, task is done (something weird happened, and e.g another task was spawned in place of this one)
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to query pdp_proof_sets: %w", err)
	}
	proofSetID := pdp.ID

	nextProveAt, err := n.service.NextPDPChallengeWindowStart(ctx, big.NewInt(proofSetID))
	if err != nil {
		return false, fmt.Errorf("failed to get next challenge window start: %w", err)
	}

	if pdp.ChallengeWindow == nil {
		return false, fmt.Errorf("proof set %d missing challenge window metadata", proofSetID)
	}
	if pdp.ProvingPeriod == nil {
		return false, fmt.Errorf("proof set %d missing proving period", proofSetID)
	}

	challengeFinality, err := n.verifier.GetChallengeFinality(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get challenge finality: %w", err)
	}

	challengeWindow := *pdp.ChallengeWindow
	provingPeriod := *pdp.ProvingPeriod

	ts, err := n.fil.ChainHead(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get chain head: %w", err)
	}

	minEpoch := big.NewInt(int64(ts.Height()))
	minEpoch.Add(minEpoch, challengeFinality)

	windowStart := nextProveAt.Int64()
	windowEnd := windowStart + challengeWindow

	if minEpoch.Int64() > windowEnd {
		// If the chain height + finality already pushes us beyond the reported window end,
		// the service contract will still insist on the current window and will reject a future epoch.
		// Defer sending until the next window by updating prove_at_epoch and clearing the task marker
		// so the scheduler can retry once the chain height reaches that window.
		adjusted := adjustNextProveAt(windowStart, minEpoch.Int64(), provingPeriod, challengeWindow)
		log.Warnw("deferring next proving period until next window",
			"proof_set_id", proofSetID,
			"original_epoch", windowStart,
			"adjusted_epoch", adjusted,
			"current_height", ts.Height(),
			"challenge_window", challengeWindow,
			"proving_period", provingPeriod,
		)

		if err := n.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			result := tx.Model(&models.PDPProofSet{}).
				Where("id = ?", proofSetID).
				Updates(map[string]interface{}{
					"prove_at_epoch":            uint64(adjusted),
					"challenge_request_task_id": nil,
				})
			if result.Error != nil {
				return fmt.Errorf("failed to defer pdp_proof_sets update: %w", result.Error)
			}
			if result.RowsAffected == 0 {
				return fmt.Errorf("pdp_proof_sets defer update affected 0 rows")
			}
			return nil
		}); err != nil {
			return false, err
		}

		// Stop this task; a new one will be scheduled when the chain height reaches the next window.
		return true, nil
	}

	if nextProveAt.Cmp(minEpoch) < 0 {
		// The condition only runs when the listener contract hands us a challenge window start that is already behind
		// the current epoch + challengeFinality. That happens when a proving period was missed, usually from:
		// 1. piri was offline
		// 2. lotus was stuck
		// 3. the previous next proving period transaction never made it to the chain
		// Thus the listener is still reporting the "stale" window from the previous proving period.
		// When 1, 2, or 3 happen, and are fixed, we will hit this branch. Here we advance the window
		// to the first future period the verifier will accept so we don't loop on invalid epochs forever.
		adjusted := adjustNextProveAt(nextProveAt.Int64(), minEpoch.Int64(), provingPeriod, challengeWindow)
		log.Warnw("adjusting next prove epoch",
			"proof_set_id", proofSetID,
			"original_epoch", nextProveAt,
			"adjusted_epoch", adjusted,
			"current_height", ts.Height(),
			"challenge_window", challengeWindow,
			"proving_period", provingPeriod,
		)
		nextProveAt = big.NewInt(adjusted)
	}

	// Prepare the transaction data
	abiData, err := n.verifier.GetABI()
	if err != nil {
		return false, fmt.Errorf("failed to get PDPVerifier ABI: %w", err)
	}

	data, err := abiData.Pack("nextProvingPeriod", big.NewInt(proofSetID), nextProveAt, []byte{})
	if err != nil {
		return false, fmt.Errorf("failed to pack data: %w", err)
	}

	// Prepare the transaction
	txEth := types.NewTransaction(
		0,                    // nonce (will be set by sender)
		n.verifier.Address(), // to
		big.NewInt(0),        // value
		0,                    // gasLimit (to be estimated)
		nil,                  // gasPrice (to be set by sender)
		data,                 // data
	)

	fromAddress, _, err := n.verifier.GetDataSetStorageProvider(ctx, big.NewInt(proofSetID))
	if err != nil {
		return false, fmt.Errorf("failed to get default sender address: %w", err)
	}

	// Send the transaction
	reason := "pdp-proving-period"
	log.Infow("Sending next proving period transaction", "task_id", taskID, "proof_set_id", proofSetID,
		"next_prove_at", nextProveAt, "current_height", ts.Height())
	txHash, err := n.sender.Send(ctx, fromAddress, txEth, reason)
	if err != nil {
		var ice *evmerrors.InvalidChallengeEpoch
		if errors.As(err, &ice) {
			minAllowed := ice.MinAllowed.Int64()
			maxAllowed := ice.MaxAllowed.Int64()
			adjusted := minAllowed
			if minEpoch.Int64() > adjusted {
				adjusted = adjustNextProveAt(adjusted, minEpoch.Int64(), provingPeriod, challengeWindow)
			}
			if adjusted > maxAllowed {
				// move to the next window after the one reported in the revert
				adjusted = adjustNextProveAt(maxAllowed+provingPeriod, minEpoch.Int64(), provingPeriod, challengeWindow)
			}

			// The service contract is authoritative about the valid window; when it disagrees with what
			// NextPDPChallengeWindowStart handed us, the only reliable source of the current [min,max] is
			// this revert. We rewrite prove_at_epoch to that window (or the next one) and clear the task
			// so the scheduler can resubmit with a contract-accepted epoch instead of looping on reverts.
			log.Warnw("deferring after InvalidChallengeEpoch revert",
				"proof_set_id", proofSetID,
				"min_allowed", minAllowed,
				"max_allowed", maxAllowed,
				"min_required", minEpoch.Int64(),
				"adjusted_epoch", adjusted,
			)

			if err := n.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				result := tx.Model(&models.PDPProofSet{}).
					Where("id = ?", proofSetID).
					Updates(map[string]interface{}{
						"prove_at_epoch":            uint64(adjusted),
						"challenge_request_task_id": nil,
					})
				if result.Error != nil {
					return fmt.Errorf("failed to defer after InvalidChallengeEpoch: %w", result.Error)
				}
				if result.RowsAffected == 0 {
					return fmt.Errorf("pdp_proof_sets defer update affected 0 rows")
				}
				return nil
			}); err != nil {
				return false, err
			}

			// End this task; scheduler will reschedule with the corrected epoch.
			return true, nil
		}
		return false, fmt.Errorf("failed to send transaction: %w", err)
	}

	if err := n.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Update pdp_proof_sets within a transaction
		result := tx.Model(&models.PDPProofSet{}).
			Where("id = ?", proofSetID).
			Updates(map[string]interface{}{
				"challenge_request_msg_hash":   txHash.Hex(),
				"prev_challenge_request_epoch": ts.Height(),
				"prove_at_epoch":               nextProveAt.Uint64(),
			})
		if result.Error != nil {
			return fmt.Errorf("failed to update pdp_proof_sets: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("pdp_proof_sets update affected 0 rows")
		}

		// Insert into message_waits_eth with ON CONFLICT DO NOTHING
		msg := models.MessageWaitsEth{
			SignedTxHash: txHash.Hex(),
			TxStatus:     "pending",
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&msg).Error; err != nil {
			return fmt.Errorf("failed to insert into message_waits_eth: %w", err)
		}

		return nil
	}); err != nil {
		return false, fmt.Errorf("failed to perform database transaction: %w", err)
	}

	// Task completed successfully
	log.Infow("Next challenge window scheduled", "epoch", nextProveAt)

	return true, nil
}

func (n *NextProvingPeriodTask) CanAccept(ids []scheduler.TaskID, engine *scheduler.TaskEngine) (*scheduler.TaskID, error) {
	id := ids[0]
	return &id, nil
}

func (n *NextProvingPeriodTask) TypeDetails() scheduler.TaskTypeDetails {
	return scheduler.TaskTypeDetails{
		Name: "PDPProvingPeriod",
	}
}

func (n *NextProvingPeriodTask) Adder(taskFunc scheduler.AddTaskFunc) {
	n.addFunc.Set(taskFunc)
}
