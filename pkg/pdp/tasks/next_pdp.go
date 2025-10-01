package tasks

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
	chaintypes "github.com/filecoin-project/lotus/chain/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/storacha/piri/pkg/pdp/chainsched"
	"github.com/storacha/piri/pkg/pdp/ethereum"
	"github.com/storacha/piri/pkg/pdp/promise"
	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
)

var _ scheduler.TaskInterface = &NextProvingPeriodTask{}

//var _ = scheduler.Reg(&NextProvingPeriodTask{})

type NextProvingPeriodTask struct {
	db             *gorm.DB
	ethClient      bind.ContractBackend
	contractClient smartcontracts.PDP
	sender         ethereum.Sender

	fil ChainAPI

	addFunc promise.Promise[scheduler.AddTaskFunc]
}

func NewNextProvingPeriodTask(db *gorm.DB, ethClient bind.ContractBackend, contractClient smartcontracts.PDP, api ChainAPI, chainSched *chainsched.Scheduler, sender ethereum.Sender) (*NextProvingPeriodTask, error) {
	n := &NextProvingPeriodTask{
		db:             db,
		ethClient:      ethClient,
		contractClient: contractClient,
		sender:         sender,
		fil:            api,
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
					return false, fmt.Errorf("failed to update pdp_proof_sets: %w", err)
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
// Algorithm: Find the first challenge window that starts after (currentHeight + challengeFinality),
// then schedule 1 epoch after that window starts. This is deterministic and always contract-compliant.
//
// Example: currentHeight=1000, finality=2, window=30
// → minRequired=1002, nextWindow=1020, result=1021
func adjustNextProveAt(currentHeight int64, challengeFinality, challengeWindow *big.Int) *big.Int {
	// Calculate minimum required epoch (current height + challenge finality)
	minRequiredEpoch := currentHeight + challengeFinality.Int64()

	// Find the challenge window that contains or comes after minRequiredEpoch
	// Window boundaries are at multiples of challengeWindow: 0, 30, 60, 90, etc.
	windowNumber := minRequiredEpoch / challengeWindow.Int64()
	windowStart := windowNumber * challengeWindow.Int64()

	// If minRequiredEpoch falls exactly on a window boundary or we need the next window
	if windowStart <= minRequiredEpoch {
		windowStart += challengeWindow.Int64() // Move to next window
	}

	// Schedule 1 epoch after the window starts for safety
	return big.NewInt(windowStart + 1)
}

func (n *NextProvingPeriodTask) Do(taskID scheduler.TaskID) (done bool, err error) {
	ctx := context.Background()
	// Select the proof set where challenge_request_task_id equals taskID and prove_at_epoch is not NULL
	var pdp models.PDPProofSet
	err = n.db.WithContext(ctx).
		Model(&models.PDPProofSet{}).
		Where("challenge_request_task_id = ? AND prove_at_epoch IS NOT NULL", taskID).
		Select("id").
		First(&pdp).Error
	if errors.Is(err, sql.ErrNoRows) {
		// No matching proof set, task is done (something weird happened, and e.g another task was spawned in place of this one)
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to query pdp_proof_sets: %w", err)
	}
	proofSetID := pdp.ID

	// Get the listener address for this proof set from the PDPVerifier contract
	pdpVerifier, err := n.contractClient.NewPDPVerifier(smartcontracts.Addresses().PDPVerifier, n.ethClient)
	if err != nil {
		return false, fmt.Errorf("failed to instantiate PDPVerifier contract: %w", err)
	}

	listenerAddr, err := pdpVerifier.GetDataSetListener(nil, big.NewInt(proofSetID))
	if err != nil {
		return false, fmt.Errorf("failed to get listener address for data set %d: %w", proofSetID, err)
	}

	// Determine the next challenge window start by consulting the proving schedule provider
	provingSchedule, err := smartcontracts.GetProvingScheduleFromListener(listenerAddr, n.ethClient, n.fil)
	if err != nil {
		return false, fmt.Errorf("failed to create proving schedule provider: %w", err)
	}
	next_prove_at, err := provingSchedule.NextPDPChallengeWindowStart(ctx, big.NewInt(proofSetID))
	if err != nil {
		return false, fmt.Errorf("failed to get next challenge window start: %w", err)
	}

	// Instantiate the PDPVerifier contract
	pdpContracts := smartcontracts.Addresses()
	pdpVerifierAddress := pdpContracts.PDPVerifier

	// Prepare the transaction data
	abiData, err := smartcontracts.PDPVerifierMetaData()
	if err != nil {
		return false, fmt.Errorf("failed to get PDPVerifier ABI: %w", err)
	}

	data, err := abiData.Pack("nextProvingPeriod", big.NewInt(proofSetID), next_prove_at, []byte{})
	if err != nil {
		return false, fmt.Errorf("failed to pack data: %w", err)
	}

	// Prepare the transaction
	txEth := types.NewTransaction(
		0,                  // nonce (will be set by sender)
		pdpVerifierAddress, // to
		big.NewInt(0),      // value
		0,                  // gasLimit (to be estimated)
		nil,                // gasPrice (to be set by sender)
		data,               // data
	)

	fromAddress, _, err := pdpVerifier.GetDataSetStorageProvider(nil, big.NewInt(proofSetID))
	if err != nil {
		return false, fmt.Errorf("failed to get default sender address: %w", err)
	}

	// Get the current tipset
	ts, err := n.fil.ChainHead(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get chain head: %w", err)
	}

	// Send the transaction
	reason := "pdp-proving-period"
	log.Infow("Sending next proving period transaction", "task_id", taskID, "proof_set_id", proofSetID,
		"next_prove_at", next_prove_at, "current_height", ts.Height())
	txHash, err := n.sender.Send(ctx, fromAddress, txEth, reason)
	if err != nil {
		return false, fmt.Errorf("failed to send transaction: %w", err)
	}

	if err := n.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Update pdp_proof_sets within a transaction
		result := tx.Model(&models.PDPProofSet{}).
			Where("id = ?", proofSetID).
			Updates(map[string]interface{}{
				"challenge_request_msg_hash":   txHash.Hex(),
				"prev_challenge_request_epoch": ts.Height(),
				"prove_at_epoch":               next_prove_at.Uint64(),
			})
		if result.Error != nil {
			return fmt.Errorf("failed to update pdp_proof_sets: %w", err)
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
	log.Infow("Next challenge window scheduled", "epoch", next_prove_at)

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
