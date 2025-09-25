package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"

	chaintypes "github.com/filecoin-project/lotus/chain/types"

	"github.com/storacha/piri/pkg/pdp/chainsched"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
)

type ProofSetCreate struct {
	CreateMessageHash string `db:"create_message_hash"`
	Service           string `db:"service"`
}

func NewWatcherCreate(
	db *gorm.DB,
	ethClient bind.ContractBackend,
	contractClient smartcontracts.PDP,
	pcs *chainsched.Scheduler,
) error {
	log.Infow("Initializing proof set creation watcher")
	if err := pcs.AddHandler(func(ctx context.Context, revert, apply *chaintypes.TipSet) error {
		log.Debugw("Chain update triggered proof set creation check", "tipset_height", apply.Height())
		err := processPendingProofSetCreates(ctx, db, ethClient, contractClient)
		if err != nil {
			log.Warnw("Failed to process pending proof set creates", "error", err, "tipset_height", apply.Height())
		}
		return nil
	}); err != nil {
		log.Errorw("Failed to register proof set watcher handler", "error", err)
		return err
	}
	log.Infow("Successfully registered proof set creation watcher")
	return nil
}

func processPendingProofSetCreates(
	ctx context.Context,
	db *gorm.DB,
	ethClient bind.ContractBackend,
	contractClient smartcontracts.PDP,
) error {
	log.Debugw("Querying for pending proof set creations", "query_conditions", "ok=true AND proofset_created=false")
	// Query for pdp_proofset_creates entries where ok = TRUE and proofset_created = FALSE
	var proofSetCreates []models.PDPProofsetCreate
	err := db.WithContext(ctx).
		Where("ok = ? AND proofset_created = ?", true, false).
		Find(&proofSetCreates).Error
	if err != nil {
		log.Errorw("Database query for pending proof set creates failed", "error", err)
		return fmt.Errorf("failed to select proof set creates: %w", err)
	}

	if len(proofSetCreates) == 0 {
		log.Debugw("No pending proof set creations found")
		return nil
	}

	log.Infow("Found pending proof set creations to process", "count", len(proofSetCreates))
	// Process each proof set create
	for i, psc := range proofSetCreates {
		start := time.Now()
		log.Infow("Processing proof set creation",
			"index", i+1,
			"total", len(proofSetCreates),
			"tx_hash", psc.CreateMessageHash,
			"service", psc.Service)

		err := processProofSetCreate(ctx, db, psc, ethClient, contractClient)
		if err != nil {
			log.Errorw("Failed to process proof set create",
				"tx_hash", psc.CreateMessageHash,
				"service", psc.Service,
				"error", err)
			continue
		}
		log.Infow("Successfully processed proof set creation",
			"tx_hash", psc.CreateMessageHash,
			"service", psc.Service,
			"duration", time.Since(start))
	}

	return nil
}

func processProofSetCreate(
	ctx context.Context,
	db *gorm.DB,
	psc models.PDPProofsetCreate,
	ethClient bind.ContractBackend,
	contactClient smartcontracts.PDP,
) error {
	txHash := psc.CreateMessageHash
	service := psc.Service

	lg := log.With("tx_hash", txHash, "owner", service, "verifier_address", smartcontracts.Addresses().PDPVerifier.String())

	// Retrieve the tx_receipt from message_waits_eth
	lg.Debug("Retrieving transaction receipt")
	var msgWait models.MessageWaitsEth
	err := db.WithContext(ctx).
		Select("tx_receipt").
		First(&msgWait, "signed_tx_hash = ?", txHash).Error
	if err != nil {
		lg.Errorw("Failed to retrieve transaction receipt", "error", err)
		return fmt.Errorf("failed to get tx_receipt for tx %s: %w", txHash, err)
	}

	txReceiptJSON := msgWait.TxReceipt
	lg.Debugw("Successfully retrieved transaction receipt", "tx_status", msgWait.TxStatus, "tx_success", msgWait.TxSuccess)

	// Unmarshal the tx_receipt JSON into types.Receipt
	var txReceipt types.Receipt
	err = json.Unmarshal(txReceiptJSON, &txReceipt)
	if err != nil {
		lg.Error("Failed to unmarshal transaction receipt JSON", "error", err)
		return fmt.Errorf("failed to unmarshal tx_receipt for tx %s: %w", txHash, err)
	}

	// Parse the logs to extract the proofSetId
	lg.Debug("Extracting proof set ID from transaction receipt")
	proofSetId, err := contactClient.GetDataSetIdFromReceipt(&txReceipt)
	if err != nil {
		lg.Errorw("Failed to extract proof set ID from receipt",
			"tx_hash", txHash,
			"error", err)
		return fmt.Errorf("failed to extract proofSetId from receipt for tx %s: %w", txHash, err)
	}
	lg = lg.With("proof_set_id", proofSetId)
	lg.Debug("Extracted proof set ID")

	// Get the listener address for this proof set from the PDPVerifier contract
	lg.Debug("Getting PDP verifier contract")
	pdpVerifier, err := contactClient.NewPDPVerifier(smartcontracts.Addresses().PDPVerifier, ethClient)
	if err != nil {
		lg.Errorw("Failed to instantiate PDPVerifier contract", "error", err)
		return fmt.Errorf("failed to instantiate PDPVerifier contract: %w", err)
	}

	lg.Debug("Querying proof set listener address")
	listenerAddr, err := pdpVerifier.GetDataSetListener(nil, big.NewInt(int64(proofSetId)))
	if err != nil {
		lg.Errorw("Failed to get listener address for proof set", "error", err)
		return fmt.Errorf("failed to get listener address for proof set %d: %w", proofSetId, err)
	}
	lg = lg.With("listener_addr", listenerAddr.String())
	lg.Debug("Retrieved proof set listener")

	// Get the proving period from the listener
	// Assumption: listener is a PDP Service with proving window informational methods
	lg.Debug("Fetching proving period and challenge window")
	provingPeriod, challengeWindow, err := getProvingPeriodChallengeWindow(ctx, ethClient, listenerAddr, contactClient)
	if err != nil {
		lg.Errorw("Failed to get proving period parameters", "error", err)
		return fmt.Errorf("failed to get max proving period: %w", err)
	}
	lg = lg.With("proving_period", provingPeriod, "challenge_window", challengeWindow)
	lg.Debug("Retrieved proving parameters")

	// Insert a new entry into pdp_proof_sets
	lg.Debug("Inserting proof set data into database")
	err = insertProofSet(ctx, db, txHash, proofSetId, service, provingPeriod, challengeWindow)
	if err != nil {
		lg.Errorw("Failed to insert proof set into database", "error", err)
		return fmt.Errorf("failed to insert proof set %d for tx %s: %w", proofSetId, txHash, err)
	}
	lg.Debug("Successfully inserted proof set record")

	// Update pdp_proofset_creates to set proofset_created = TRUE
	lg.Debugw("Updating proof set creation status")
	err = db.WithContext(ctx).
		Model(&models.PDPProofsetCreate{}).
		Where("create_message_hash = ?", txHash).
		Update("proofset_created", true).Error
	if err != nil {
		lg.Errorw("Failed to update proof set creation status", "error", err)
		return fmt.Errorf("failed to update proofset_creates for tx %s: %w", txHash, err)
	}
	lg.Debug("Successfully updated proof set creation status")

	lg.Infow("Successfully created proof set")
	return nil
}

func insertProofSet(
	ctx context.Context,
	db *gorm.DB,
	createMsg string,
	proofSetId uint64,
	service string,
	provingPeriod uint64,
	challengeWindow uint64,
) error {
	log.Debugw("Preparing proof set database record",
		"proof_set_id", proofSetId,
		"tx_hash", createMsg,
		"service", service,
		"proving_period", provingPeriod,
		"challenge_window", challengeWindow)

	// Implement the insertion into pdp_proof_sets table
	proofset := models.PDPProofSet{
		ID:                int64(proofSetId),
		CreateMessageHash: createMsg,
		Service:           service,
		ProvingPeriod:     models.Ptr(int64(provingPeriod)),
		ChallengeWindow:   models.Ptr(int64(challengeWindow)),
	}

	log.Debugw("Inserting proof set into database", "proof_set_id", proofSetId)
	err := db.WithContext(ctx).Create(&proofset).Error
	if err != nil {
		log.Errorw("Database insert operation failed",
			"proof_set_id", proofSetId,
			"tx_hash", createMsg,
			"error", err)
		return fmt.Errorf("failed to insert proof set %d for tx %s: %w", proofSetId, createMsg, err)
	}

	log.Debugw("Successfully created database record for proof set",
		"proof_set_id", proofSetId,
		"tx_hash", createMsg)
	return nil
}

func getProvingPeriodChallengeWindow(ctx context.Context, ethClient bind.ContractBackend, listenerAddr common.Address, contractClient smartcontracts.PDP) (uint64, uint64, error) {
	log.Debugw("Creating proving schedule contract binding",
		"listener_address", listenerAddr.Hex())

	// ProvingPeriod
	schedule, err := smartcontracts.GetProvingScheduleFromListener(listenerAddr, ethClient)
	if err != nil {
		log.Errorw("Failed to create proving schedule contract binding",
			"listener_address", listenerAddr.Hex(),
			"error", err)
		return 0, 0, fmt.Errorf("failed to create proving schedule binding, check that listener has proving schedule methods: %w", err)
	}

	config, err := schedule.GetPDPConfig(&bind.CallOpts{Context: ctx})
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get pdp config config: %w", err)
	}

	return config.MaxProvingPeriod, config.ChallengeWindow.Uint64(), nil

}
