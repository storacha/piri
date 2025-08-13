package tasks

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"go.uber.org/multierr"
	"golang.org/x/xerrors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/storacha/piri/pkg/pdp/promise"
	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/wallet"
)

var SendLockedWait = 100 * time.Millisecond

var _ scheduler.TaskInterface = &SendTaskETH{}

type SenderETHClient interface {
	NetworkID(ctx context.Context) (*big.Int, error)
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
	PendingNonceAt(ctx context.Context, account common.Address) (uint64, error)
	EstimateGas(ctx context.Context, msg ethereum.CallMsg) (uint64, error)
	SendTransaction(ctx context.Context, transaction *types.Transaction) error
	SuggestGasTipCap(ctx context.Context) (*big.Int, error)
}

type SenderETH struct {
	client SenderETHClient

	sendTask *SendTaskETH

	db *gorm.DB
}

// NewSenderETH creates a new SenderETH.
func NewSenderETH(client SenderETHClient, wallet wallet.Wallet, db *gorm.DB) (*SenderETH, *SendTaskETH) {
	st := &SendTaskETH{
		client: client,
		wallet: wallet,
		db:     db,
	}

	return &SenderETH{
		client:   client,
		db:       db,
		sendTask: st,
	}, st
}

func (s *SenderETH) Send(ctx context.Context, fromAddress common.Address, tx *types.Transaction, reason string) (common.Hash, error) {
	log.Infow("Send message", "from", fromAddress.String(), "tx", tx.Hash().Hex(), "reason", reason)
	// Ensure the transaction has zero nonce; it will be assigned during send task
	if tx.Nonce() != 0 {
		return common.Hash{}, xerrors.Errorf("Send expects transaction nonce to be 0, was %d", tx.Nonce())
	}

	if tx.Gas() == 0 {
		// Estimate gas limit
		msg := ethereum.CallMsg{
			From:  fromAddress,
			To:    tx.To(),
			Value: tx.Value(),
			Data:  tx.Data(),
		}

		gasLimit, err := s.client.EstimateGas(ctx, msg)
		if err != nil {
			return common.Hash{}, fmt.Errorf("failed to estimate gas: %w", err)
		}
		if gasLimit == 0 {
			return common.Hash{}, fmt.Errorf("estimated gas limit is zero")
		}

		// Fetch current base fee
		header, err := s.client.HeaderByNumber(ctx, nil)
		if err != nil {
			return common.Hash{}, fmt.Errorf("failed to get latest block header: %w", err)
		}

		baseFee := header.BaseFee
		if baseFee == nil {
			return common.Hash{}, fmt.Errorf("base fee not available; network might not support EIP-1559")
		}

		// Set GasTipCap (maxPriorityFeePerGas)
		gasTipCap, err := s.client.SuggestGasTipCap(ctx)
		if err != nil {
			return common.Hash{}, xerrors.Errorf("estimating gas premium: %w", err)
		}

		// Calculate GasFeeCap (maxFeePerGas)
		gasFeeCap := new(big.Int).Add(baseFee, gasTipCap)

		chainID, err := s.client.NetworkID(ctx)
		if err != nil {
			return common.Hash{}, xerrors.Errorf("getting network ID: %w", err)
		}

		// Create a new transaction with estimated gas limit and fee caps
		tx = types.NewTx(&types.DynamicFeeTx{
			ChainID:   chainID,
			Nonce:     0, // nonce will be set later
			GasFeeCap: gasFeeCap,
			GasTipCap: gasTipCap,
			Gas:       gasLimit,
			To:        tx.To(),
			Value:     tx.Value(),
			Data:      tx.Data(),
		})
	}

	// Serialize the unsigned transaction
	unsignedTxData, err := tx.MarshalBinary()
	if err != nil {
		return common.Hash{}, xerrors.Errorf("marshaling unsigned transaction: %w", err)
	}

	unsignedHash := tx.Hash().Hex()

	// Push the task
	taskAdder := s.sendTask.sendTF.Val(ctx)

	var sendTaskID *scheduler.TaskID
	taskAdder(func(id scheduler.TaskID, txdb *gorm.DB) (shouldCommit bool, seriousError error) {
		err := txdb.Create(&models.MessageSendsEth{
			FromAddress:  fromAddress.Hex(),
			ToAddress:    tx.To().Hex(),
			SendReason:   reason,
			UnsignedTx:   unsignedTxData,
			UnsignedHash: unsignedHash,
			SendTaskID:   int(id),
		}).Error
		if err != nil {
			return false, xerrors.Errorf("inserting transaction into db: %w", err)
		}

		sendTaskID = &id

		return true, nil
	})

	if sendTaskID == nil {
		return common.Hash{}, xerrors.Errorf("failed to add task")
	}

	// Wait for execution
	var (
		pollInterval    = 50 * time.Millisecond
		pollIntervalMul = 2
		maxPollInterval = 5 * time.Second
		pollLoops       = 0

		signedHash common.Hash
		sendErr    error
	)

	for {
		var row models.MessageSendsEth
		err := s.db.Where("send_task_id = ?", sendTaskID).First(&row).Error
		if err != nil {
			return common.Hash{}, xerrors.Errorf("getting send status for task: %w", err)
		}

		if row.SendSuccess == nil {
			time.Sleep(pollInterval)
			pollLoops++
			pollInterval *= time.Duration(pollIntervalMul)
			if pollInterval > maxPollInterval {
				pollInterval = maxPollInterval
			}
			continue
		}

		if row.SignedHash == nil || row.SendError == nil {
			return common.Hash{}, xerrors.Errorf("unexpected null values in send status")
		}

		if !*row.SendSuccess {
			sendErr = xerrors.Errorf("send error: %s", *row.SendError)
		} else {
			signedHash = common.HexToHash(*row.SignedHash)
		}

		break
	}

	log.Infow("sent transaction", "hash", signedHash, "task_id", sendTaskID, "send_error", sendErr, "poll_loops", pollLoops)

	return signedHash, sendErr
}

type SendTaskETH struct {
	sendTF promise.Promise[scheduler.AddTaskFunc]

	client SenderETHClient
	wallet wallet.Wallet

	db *gorm.DB
}

func (s *SendTaskETH) Do(taskID scheduler.TaskID) (done bool, err error) {
	ctx := context.TODO()

	// Get transaction from the database
	var dbTx models.MessageSendsEth
	err = s.db.Where("send_task_id = ?", taskID).First(&dbTx).Error
	if err != nil {
		return false, xerrors.Errorf("getting transaction from db: %w", err)
	}

	// Deserialize the unsigned transaction
	tx := new(types.Transaction)
	err = tx.UnmarshalBinary(dbTx.UnsignedTx)
	if err != nil {
		return false, xerrors.Errorf("unmarshaling unsigned transaction: %w", err)
	}

	fromAddress := common.HexToAddress(dbTx.FromAddress)

	// Acquire lock on from_address
	for {

		// Try to acquire lock
		res := s.db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "from_address"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"task_id":    taskID,
				"claimed_at": time.Now(),
			}),
			Where: clause.Where{
				Exprs: []clause.Expression{
					clause.Eq{Column: "message_send_eth_locks.task_id", Value: taskID},
				},
			},
		}).Create(&models.MessageSendEthLock{
			FromAddress: dbTx.FromAddress,
			TaskID:      int64(taskID),
			ClaimedAt:   time.Now(),
		})
		if res.Error != nil {
			return false, fmt.Errorf("aquiring send lock: %w", res.Error)
		}

		if res.RowsAffected == 1 {
			// Acquired the lock
			break
		}

		// Wait and retry
		log.Infow("waiting for send lock", "task_id", taskID, "from", dbTx.FromAddress)
		time.Sleep(SendLockedWait)
	}

	// Defer release of the lock
	defer func() {
		err2 := s.db.Where("from_address = ? AND task_id = ?", dbTx.FromAddress, taskID).
			Delete(&models.MessageSendEthLock{}).Error
		if err2 != nil {
			log.Errorw("releasing send lock", "task_id", taskID, "from", dbTx.FromAddress, "error", err2)

			// Ensure the task is retried
			done = false
			err = multierr.Append(err, xerrors.Errorf("releasing send lock: %w", err2))
		}
	}()

	var signedTx *types.Transaction

	if dbTx.Nonce == nil {
		// Get the latest nonce
		pendingNonce, err := s.client.PendingNonceAt(ctx, fromAddress)
		if err != nil {
			return false, xerrors.Errorf("getting pending nonce: %w", err)
		}

		// Get max nonce from successful transactions in DB
		var dbNonce *int64
		err = s.db.Model(&models.MessageSendsEth{}).
			Where("from_address = ? AND send_success = ?", dbTx.FromAddress, true).
			Select("MAX(nonce)").Scan(&dbNonce).Error
		if err != nil {
			return false, xerrors.Errorf("getting max nonce from db: %w", err)
		}

		assignedNonce := pendingNonce
		if dbNonce != nil && uint64(*dbNonce)+1 > pendingNonce {
			assignedNonce = uint64(*dbNonce) + 1
		}

		// Update the transaction with the assigned nonce
		tx = types.NewTransaction(assignedNonce, *tx.To(), tx.Value(), tx.Gas(), tx.GasPrice(), tx.Data())

		// Sign the transaction
		signedTx, err = s.signTransaction(ctx, fromAddress, tx)
		if err != nil {
			return false, xerrors.Errorf("signing transaction: %w", err)
		}

		// Serialize the signed transaction
		signedTxData, err := signedTx.MarshalBinary()
		if err != nil {
			return false, xerrors.Errorf("serializing signed transaction: %w", err)
		}

		// Update the database with nonce and signed transaction
		res := s.db.Model(&models.MessageSendsEth{}).
			Where("send_task_id = ?", taskID).
			Updates(map[string]interface{}{
				"nonce":       assignedNonce,
				"signed_tx":   signedTxData,
				"signed_hash": signedTx.Hash().Hex(),
			})
		if res.Error != nil {
			return false, xerrors.Errorf("updating db record: %w", err)
		}
		if res.RowsAffected != 1 {
			return false, xerrors.Errorf("expected to update 1 row, updated %d", res.RowsAffected)
		}
	} else {
		// Transaction was previously signed but possibly failed to send
		// Deserialize the signed transaction
		signedTx = new(types.Transaction)
		err = signedTx.UnmarshalBinary(dbTx.SignedTx)
		if err != nil {
			return false, xerrors.Errorf("unmarshaling signed transaction: %w", err)
		}
	}

	// Send the transaction
	err = s.client.SendTransaction(ctx, signedTx)

	// Persist send result
	var sendSuccess = err == nil
	var sendError string
	if err != nil {
		sendError = err.Error()
	}

	err = s.db.Model(&models.MessageSendsEth{}).
		Where("send_task_id = ?", taskID).
		Updates(map[string]interface{}{
			"send_success": sendSuccess,
			"send_error":   sendError,
			"send_time":    time.Now(),
		}).Error
	if err != nil {
		return false, xerrors.Errorf("updating db record: %w", err)
	}

	return true, nil
}

func (s *SendTaskETH) signTransaction(ctx context.Context, fromAddress common.Address, tx *types.Transaction) (*types.Transaction, error) {
	// Get the chain ID
	chainID, err := s.client.NetworkID(ctx)
	if err != nil {
		return nil, xerrors.Errorf("getting network ID: %w", err)
	}

	// Sign the transaction with our wallet
	signer := types.LatestSignerForChainID(chainID)
	signedTx, err := s.wallet.SignTransaction(ctx, fromAddress, signer, tx)
	if err != nil {
		return nil, xerrors.Errorf("signing transaction: %w", err)
	}

	return signedTx, nil
}

func (s *SendTaskETH) CanAccept(ids []scheduler.TaskID, engine *scheduler.TaskEngine) (*scheduler.TaskID, error) {
	if len(ids) == 0 {
		// Should not happen
		return nil, nil
	}

	return &ids[0], nil
}

func (s *SendTaskETH) TypeDetails() scheduler.TaskTypeDetails {
	return scheduler.TaskTypeDetails{
		Name:        "SendTransaction",
		MaxFailures: 1000,
	}
}

func (s *SendTaskETH) Adder(taskFunc scheduler.AddTaskFunc) {
	s.sendTF.Set(taskFunc)
}
