package tasks

import (
	"context"
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

type NextProvingPeriodTask struct {
	db        *gorm.DB
	ethClient bind.ContractBackend
	verifier  smartcontracts.Verifier
	service   smartcontracts.Service
	sender    ethereum.Sender

	fil ChainAPI

	addFunc promise.Promise[scheduler.AddTaskFunc]

	// cached config
	pdpConfig         smartcontracts.PDPConfig
	challengeFinality *big.Int
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
	// Fetch static on-chain config once up front
	ctx := context.Background()
	pdpConfig, err := service.PDPConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PDP config: %w", err)
	}
	challengeFinality, err := verifier.GetChallengeFinality(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch challengeFinality: %w", err)
	}

	n := &NextProvingPeriodTask{
		db:        db,
		ethClient: ethClient,
		sender:    sender,
		fil:       api,
		verifier:  verifier,
		service:   service,
		pdpConfig: pdpConfig,
		challengeFinality: challengeFinality,
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

func (n *NextProvingPeriodTask) Do(taskID scheduler.TaskID) (done bool, err error) {
	ctx := context.Background()
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
	log.Infow("NextPDPChallengeWindowStart: %s", nextProveAt.String())

	// Get the current tipset to reason about scheduling
	ts, err := n.fil.ChainHead(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get chain head: %w", err)
	}

	windowStart := nextProveAt.Uint64()
	challengeWindow := n.pdpConfig.ChallengeWindow.Uint64()
	minAllowed := uint64(ts.Height()) + n.challengeFinality.Uint64()

	// Advance whole proving periods until the window end is at/after minAllowed.
	for minAllowed > windowStart+challengeWindow {
		windowStart += n.pdpConfig.MaxProvingPeriod
	}
	// Pick the earliest epoch that satisfies both constraints: inside the window and >= minAllowed.
	challengeEpoch := minAllowed
	if challengeEpoch < windowStart {
		challengeEpoch = windowStart
	}

	// Prepare the transaction data
	abiData, err := n.verifier.GetABI()
	if err != nil {
		return false, fmt.Errorf("failed to get PDPVerifier ABI: %w", err)
	}

	data, err := abiData.Pack("nextProvingPeriod", big.NewInt(proofSetID), big.NewInt(int64(challengeEpoch)), []byte{})
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
		"next_prove_at", windowStart, "challenge_epoch", challengeEpoch, "current_height", ts.Height(),
		"challenge_window", challengeWindow, "proving_period", n.pdpConfig.MaxProvingPeriod, "challenge_finality", n.challengeFinality.Uint64())
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
				"prove_at_epoch":               challengeEpoch,
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
