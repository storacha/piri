package tasks

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	logging "github.com/ipfs/go-log/v2"
	"golang.org/x/xerrors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/filecoin-project/go-state-types/abi"
	chaintypes "github.com/filecoin-project/lotus/chain/types"

	"github.com/storacha/piri/pkg/pdp/chainsched"
	"github.com/storacha/piri/pkg/pdp/ethereum"
	"github.com/storacha/piri/pkg/pdp/promise"
	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
)

var log = logging.Logger("pdp/tasks")

// TODO determine if this is a requirement.
// based on curio it appears this is needed for task summary details via the RPC.
// var _ = scheduler.Reg(&InitProvingPeriodTask{})
var _ scheduler.TaskInterface = &InitProvingPeriodTask{}

type InitProvingPeriodTask struct {
	db             *gorm.DB
	ethClient      bind.ContractBackend
	contractClient smartcontracts.PDP
	sender         ethereum.Sender

	chain ChainAPI

	addFunc promise.Promise[scheduler.AddTaskFunc]
}

type ChainAPI interface {
	ChainHead(context.Context) (*chaintypes.TipSet, error)
	StateGetRandomnessDigestFromBeacon(ctx context.Context, randEpoch abi.ChainEpoch, tsk chaintypes.TipSetKey) (abi.Randomness, error) //perm:read
}

func NewInitProvingPeriodTask(
	db *gorm.DB,
	ethClient bind.ContractBackend,
	contractClient smartcontracts.PDP,
	chain ChainAPI,
	chainSched *chainsched.Scheduler,
	sender ethereum.Sender,
) (*InitProvingPeriodTask, error) {
	log.Infow("Initializing proving period task", "component", "InitProvingPeriodTask")

	ipp := &InitProvingPeriodTask{
		db:             db,
		ethClient:      ethClient,
		contractClient: contractClient,
		sender:         sender,
		chain:          chain,
	}

	if err := chainSched.AddHandler(func(ctx context.Context, revert, apply *chaintypes.TipSet) error {
		if apply == nil {
			return nil
		}

		log.Debugw("Chain update triggered proving period initialization check",
			"tipset_height", apply.Height(),
			"component", "InitProvingPeriodTask")

		log.Debugw("Querying for proof sets needing initialization",
			"query_conditions", "challenge_request_task_id IS NULL AND init_ready = true AND prove_at_epoch IS NULL")

		// each time a new head is applied to the chain, query the db for proof sets needing initialization
		// via nextProvingPeriod initial call
		var proofSetIDs []int64
		if err := db.WithContext(ctx).
			Model(&models.PDPProofSet{}).
			Where("challenge_request_task_id IS NULL").
			Where("init_ready = ?", true).
			Where("prove_at_epoch IS NULL").
			Pluck("id", &proofSetIDs).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				log.Errorw("Failed to query proof sets needing initialization", "error", err)
				return fmt.Errorf("failed to select proof sets needing nextProvingPeriod: %w", err)
			}
		}

		if len(proofSetIDs) == 0 {
			log.Debugw("No proof sets need initialization")
			return nil
		}

		log.Infow("Found proof sets needing initialization", "count", len(proofSetIDs))

		for i, psID := range proofSetIDs {
			log.Infow("Scheduling initialization task for proof set",
				"proof_set_id", psID,
				"index", i+1,
				"total", len(proofSetIDs))

			ipp.addFunc.Val(ctx)(func(taskID scheduler.TaskID, tx *gorm.DB) (shouldCommit bool, seriousError error) {
				log.Debugw("Assigning task ID to proof set",
					"proof_set_id", psID,
					"task_id", taskID)

				result := tx.Model(&models.PDPProofSet{}).
					Where("id = ? AND challenge_request_task_id IS NULL", psID).
					Update("challenge_request_task_id", taskID)
				if result.Error != nil {
					log.Errorw("Failed to update proof set with task ID",
						"proof_set_id", psID,
						"task_id", taskID,
						"error", result.Error)
					return false, fmt.Errorf("failed to update pdp_proof_sets: %w", result.Error)
				}
				if result.RowsAffected == 0 {
					// With only one worker executing tasks, if no rows are updated it likely means that
					// this record was already processed.
					log.Debugw("Proof set already processed by another task",
						"proof_set_id", psID,
						"task_id", taskID)
					return false, nil
				}

				log.Debugw("Successfully assigned task ID to proof set",
					"proof_set_id", psID,
					"task_id", taskID)
				return true, nil
			})
		}
		return nil
	}); err != nil {
		log.Errorw("Failed to register proving period task handler", "error", err)
		return nil, fmt.Errorf("failed to register pdp InitProvingPersiodTask: %w", err)
	}

	log.Infow("Successfully registered proving period initialization task", "component", "InitProvingPeriodTask")
	return ipp, nil
}

func (ipp *InitProvingPeriodTask) TypeDetails() scheduler.TaskTypeDetails {
	return scheduler.TaskTypeDetails{
		Name: "PDPInitPP",
	}
}

func (ipp *InitProvingPeriodTask) Do(taskID scheduler.TaskID) (done bool, err error) {
	ctx := context.Background()

	log.Infow("Starting proving period initialization task",
		"task_id", taskID,
		"component", "InitProvingPeriodTask")

	// Select the proof set where challenge_request_task_id = taskID
	log.Debugw("Selecting proof set for task", "task_id", taskID)
	var proofSet models.PDPProofSet
	err = ipp.db.WithContext(ctx).
		Select("id").
		Where("challenge_request_task_id = ?", taskID).
		First(&proofSet).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// No matching proof set; task is done (e.g., another task was spawned in place of this one)
		log.Debugw("No matching proof set found, task is complete", "task_id", taskID)
		return true, nil
	} else if err != nil {
		log.Errorw("Failed to select proof set for task",
			"task_id", taskID,
			"error", err)
		return false, fmt.Errorf("failed to select PDPProofSet: %w", err)
	}

	proofSetID := proofSet.ID
	lg := log.With("task_id", taskID, "proof_set_id", proofSetID)
	lg.Debug("Found proof set for task")

	// Get the listener address for this proof set from the PDPVerifier contract
	lg.Debugw("Getting PDP verifier contract",
		"verifier_address", smartcontracts.Addresses().PDPVerifier.Hex())
	pdpVerifier, err := ipp.contractClient.NewPDPVerifier(smartcontracts.Addresses().PDPVerifier, ipp.ethClient)
	if err != nil {
		lg.Errorw("Failed to instantiate PDPVerifier contract", "error", err)
		return false, fmt.Errorf("failed to instantiate PDPVerifier contract: %w", err)
	}

	// Check if the data set has any leaves (pieces) before attempting to initialize proving period
	leafCount, err := pdpVerifier.GetDataSetLeafCount(nil, big.NewInt(proofSetID))
	if err != nil {
		return false, fmt.Errorf("failed to get leaf count for data set %d: %w", proofSetID, err)
	}
	if leafCount.Cmp(big.NewInt(0)) == 0 {
		// No leaves in the data set yet, skip initialization
		// Return done=false to retry later (the task will be retried by the scheduler)
		return false, nil
	}

	lg.Debug("Querying data set listener address")
	listenerAddr, err := pdpVerifier.GetDataSetListener(nil, big.NewInt(proofSetID))
	if err != nil {
		lg.Errorw("Failed to get listener address for data set", "error", err)
		return false, fmt.Errorf("failed to get listener address for data set %d: %w", proofSetID, err)
	}
	lg = lg.With("listener_address", listenerAddr.Hex())
	lg.Debug("Retrieved data set listener")

	// Determine the next challenge window start by consulting the proving schedule provider
	lg.Debug("Creating proving schedule provider")
	provingSchedule, err := smartcontracts.GetProvingScheduleFromListener(listenerAddr, ipp.ethClient, ipp.chain)
	if err != nil {
		lg.Errorw("Failed to create proving schedule provider", "error", err)
		return false, fmt.Errorf("failed to create proving schedule provider: %w", err)
	}

	config, err := provingSchedule.GetPDPConfig(ctx)
	if err != nil {
		return false, xerrors.Errorf("failed to GetPDPConfig: %w", err)
	}

	// Give a buffer of 1/2 challenge window epochs so that we are still within challenge window
	initProveAt := config.InitChallengeWindowStart.Add(config.InitChallengeWindowStart, config.ChallengeWindow.Div(config.ChallengeWindow, big.NewInt(2)))

	// Instantiate the PDPVerifier contract
	pdpContracts := smartcontracts.Addresses()
	pdpVeriferAddress := pdpContracts.PDPVerifier

	abiData, err := smartcontracts.PDPVerifierMetaData()
	if err != nil {
		lg.Errorw("Failed to get PDPVerifier ABI", "error", err)
		return false, fmt.Errorf("failed to get PDPVerifier ABI: %w", err)
	}

	data, err := abiData.Pack("nextProvingPeriod", big.NewInt(proofSetID), initProveAt, []byte{})
	if err != nil {
		lg.Errorw("Failed to pack transaction data", "error", err)
		return false, fmt.Errorf("failed to pack data: %w", err)
	}

	// Prepare the transaction
	txEth := types.NewTransaction(
		0,                 // nonce (will be set by sender)
		pdpVeriferAddress, // to
		big.NewInt(0),     // value
		0,                 // gasLimit (to be estimated)
		nil,               // gasPrice (to be set by sender)
		data,              // data
	)

	lg.Debug("Getting data set storage provider")
	fromAddress, _, err := pdpVerifier.GetDataSetStorageProvider(nil, big.NewInt(proofSetID))
	if err != nil {
		lg.Errorw("Failed to get data set storage provider address", "error", err)
		return false, fmt.Errorf("failed to get default sender address: %w", err)
	}
	lg = lg.With("storage_provider_address", fromAddress.Hex())
	lg.Debug("Retrieved data set storage provider")

	// Get the current tipset
	lg.Debug("Getting current chain head")
	ts, err := ipp.chain.ChainHead(ctx)
	if err != nil {
		lg.Errorw("Failed to get chain head", "error", err)
		return false, fmt.Errorf("failed to get chain head: %w", err)
	}
	lg = lg.With("tipset_height", ts.Height())
	lg.Debug("Retrieved chain head")

	// Send the transaction
	reason := "pdp-proving-init"
	lg.Infow("Sending nextProvingPeriod transaction",
		"to_address", pdpVeriferAddress.Hex(),
		"reason", reason)

	txHash, err := ipp.sender.Send(ctx, fromAddress, txEth, reason)
	if err != nil {
		lg.Errorw("Failed to send transaction", "error", err)
		return false, fmt.Errorf("failed to send transaction: %w", err)
	}
	lg = lg.With("tx_hash", txHash.Hex())
	lg.Infow("Successfully sent transaction")

	// Update the database in a transaction
	lg.Debug("Updating database with transaction details")

	if err := ipp.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		lg.Debug("Updating proof set record")
		result := tx.Model(&models.PDPProofSet{}).
			Where("id = ?", proofSetID).
			Updates(map[string]interface{}{
				"challenge_request_msg_hash":   txHash.Hex(),
				"prev_challenge_request_epoch": ts.Height(),
				"prove_at_epoch":               initProveAt.Uint64(),
			})
		if result.Error != nil {
			lg.Errorw("Failed to update proof set record", "error", result.Error)
			return fmt.Errorf("failed to update pdp_proof_sets: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			lg.Errorw("Proof set update affected 0 rows")
			return fmt.Errorf("pdp_proof_sets update affected 0 rows")
		}
		lg.Debug("Successfully updated proof set record")

		lg.Debug("Creating message wait record")
		msg := models.MessageWaitsEth{
			SignedTxHash: txHash.Hex(),
			TxStatus:     "pending",
		}
		// Use OnConflict DoNothing to avoid errors on duplicate keys.
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&msg).Error; err != nil {
			lg.Errorw("Failed to create message wait record", "error", err)
			return fmt.Errorf("failed to insert into message_waits_eth: %w", err)
		}
		lg.Debug("Successfully created message wait record")

		return nil
	}); err != nil {
		lg.Errorw("Database transaction failed", "error", err)
		return false, fmt.Errorf("failed to perform database transaction: %w", err)
	}

	// Task completed successfully
	lg.Infow("Successfully completed proving period initialization")
	return true, nil
}

func (ipp *InitProvingPeriodTask) Adder(taskFunc scheduler.AddTaskFunc) {
	ipp.addFunc.Set(taskFunc)
}
