package testing

import (
	"bytes"
	"context"
	"encoding/hex"
	"math/big"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-ucanto/core/ipld/hash/sha256"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/database/gormdb"
	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service"
	"github.com/storacha/piri/pkg/pdp/service/contract"
	"github.com/storacha/piri/pkg/pdp/service/contract/mocks"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/tasks"
	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/storacha/piri/pkg/store/keystore"
	"github.com/storacha/piri/pkg/store/stashstore"
	"github.com/storacha/piri/pkg/wallet"
)

// NB: this address is never sent to during testing so it's value is insignificant.
// picked a valid address anyways, but can be whatever we want.
var RecordKeepAddress = common.HexToAddress("0x6170dE2b09b404776197485F3dc6c968Ef948505")

// this address also doesn't matter
var ListenerAddress = common.HexToAddress("0x6170dE2b09b404776197485F3dc6c968Ef948506")

const SQLiteDBConfig = "pdp.db"

type Harness struct {
	T          testing.TB
	Chain      *FakeChainClient
	EthClient  *MockEthClient
	Contract   *mocks.MockPDP
	Verifier   *mocks.MockPDPVerifier
	Schedule   *mocks.MockPDPProvingSchedule
	DB         *gorm.DB
	ClientAddr common.Address
	clock      scheduler.Clock

	messageNonce uint64
}

func (h *Harness) SendCreateProofSet(proofSetID uint64) {
	defer func() {
		h.messageNonce++
	}()
	h.EthClient.MockSenderETHClient.EXPECT().EstimateGas(gomock.Any(), gomock.Any()).Return(uint64(1), nil)
	h.EthClient.MockSenderETHClient.EXPECT().HeaderByNumber(gomock.Any(), nil).Return(&ethtypes.Header{
		BaseFee: big.NewInt(1),
	}, nil)
	h.EthClient.MockSenderETHClient.EXPECT().SuggestGasTipCap(gomock.Any()).Return(big.NewInt(1), nil)
	h.EthClient.MockSenderETHClient.EXPECT().NetworkID(gomock.Any()).Return(big.NewInt(1), nil)
	h.EthClient.MockSenderETHClient.EXPECT().PendingNonceAt(gomock.Any(), gomock.Any()).Return(h.messageNonce, nil)
	h.EthClient.MockSenderETHClient.EXPECT().NetworkID(gomock.Any()).Return(big.NewInt(1), nil)
	h.EthClient.MockSenderETHClient.EXPECT().SendTransaction(gomock.Any(), gomock.Any()).Return(nil)
}

// Require_MessageSendsEth_SendSuccess asserts a message in MessageSendsEth was sent successfully
// given a reason and signedTx hash.
func (h *Harness) Require_MessageSendsEth_SendSuccess(reason string, signedTx common.Hash) {
	var record []models.MessageSendsEth
	result := h.DB.Where(&models.MessageSendsEth{SignedHash: Ptr(signedTx.Hex())}).Find(&record)

	require.NoError(h.T, result.Error)
	require.Len(h.T, record, 1)

	message := record[0]
	require.NotNil(h.T, message.SendSuccess)
	require.True(h.T, *message.SendSuccess)
	require.NotNil(h.T, message.SendError)
	require.Empty(h.T, *message.SendError)
	require.Equal(h.T, reason, message.SendReason)
	require.Equal(h.T, h.ClientAddr.Hex(), message.FromAddress)
	require.Equal(h.T, contract.Addresses().PDPVerifier.Hex(), message.ToAddress)
}

func (h *Harness) WaitFor_MessageWaitsEth_TxSuccess(signedTx common.Hash) {
	require.Eventually(h.T, func() bool {
		var record models.MessageWaitsEth
		result := h.DB.Where(&models.MessageWaitsEth{SignedTxHash: signedTx.Hex()}).First(&record)
		require.NoError(h.T, result.Error)

		return record.TxSuccess != nil && *record.TxSuccess
	},
		time.Minute,
		50*time.Millisecond)
}

func (h *Harness) WaitFor_PDPProofsetCreate_OK(signedTx common.Hash) {
	require.Eventually(h.T, func() bool {
		var record models.PDPProofsetCreate
		result := h.DB.Where(&models.PDPProofsetCreate{CreateMessageHash: signedTx.Hex()}).First(&record)
		require.NoError(h.T, result.Error)

		return record.Ok != nil && *record.Ok
	},
		time.Minute,
		50*time.Millisecond,
	)
}

func (h *Harness) WaitFor_PDPProofsetCreate_ProofsetCreated(signedTx common.Hash) {
	require.Eventually(h.T, func() bool {
		var record models.PDPProofsetCreate
		result := h.DB.Where(&models.PDPProofsetCreate{CreateMessageHash: signedTx.Hex()}).First(&record)
		require.NoError(h.T, result.Error)

		return record.Ok != nil && *record.Ok && record.ProofsetCreated
	},
		time.Minute,
		50*time.Millisecond,
	)

}

func (h *Harness) UploadPiece(ctx context.Context, svc *service.PDPService, piece []byte, notify string) *types.AllocatedPiece {
	pieceDigest, err := sha256.Hasher.Sum(piece)
	require.NoError(h.T, err)

	decoded, err := multihash.Decode(pieceDigest.Bytes())
	require.NoError(h.T, err)

	var nurl *url.URL
	if notify != "" {
		nurl, err = url.Parse(notify)
		require.NoError(h.T, err)
	}
	prepareRequest := types.PieceAllocation{
		Piece: types.Piece{
			Name: decoded.Name,
			Hash: hex.EncodeToString(decoded.Digest),
			Size: int64(len(piece)),
		},
		Notify: nurl,
	}
	resp, err := svc.AllocatePiece(ctx, prepareRequest)
	require.NoError(h.T, err)

	// ensure the piece was added to the prepareUpload table
	var prepareUpload []models.PDPPieceUpload
	expectedID := resp.UploadID.String()
	res := h.DB.Where(&models.PDPPieceUpload{ID: expectedID}).Find(&prepareUpload)
	require.NoError(h.T, res.Error)
	require.Len(h.T, prepareUpload, 1)
	require.Equal(h.T, prepareUpload[0].CheckHashCodec, prepareRequest.Piece.Name)
	require.Equal(h.T, prepareUpload[0].CheckSize, prepareRequest.Piece.Size)

	// upload piece we prepared above.
	expectedUUID, err := uuid.Parse(expectedID)
	require.NoError(h.T, err)
	err = svc.UploadPiece(ctx, types.PieceUpload{
		ID:   expectedUUID,
		Data: bytes.NewReader(piece),
	})
	require.NoError(h.T, err)

	// after a piece is uploaded, the prepareUpload table will be populated with its details.
	// Find the piece CID recorded by the service
	pieceCid, found, err := svc.FindPiece(ctx, types.Piece{
		Name: prepareRequest.Piece.Name,
		Hash: prepareRequest.Piece.Hash,
		Size: prepareRequest.Piece.Size,
	})
	require.NoError(h.T, err)
	require.True(h.T, found)

	var parkedPiece []models.ParkedPiece
	res = h.DB.Where(&models.ParkedPiece{PieceCID: pieceCid.String()}).
		Find(&parkedPiece)
	require.NoError(h.T, res.Error)
	require.Len(h.T, parkedPiece, 1)
	require.EqualValues(h.T, len(piece), parkedPiece[0].PieceRawSize)
	require.True(h.T, parkedPiece[0].LongTerm)
	require.True(h.T, parkedPiece[0].Complete)

	var parkedPieceRef []models.ParkedPieceRef
	res = h.DB.Where(&models.ParkedPieceRef{PieceID: parkedPiece[0].ID}).
		Find(&parkedPieceRef)
	require.NoError(h.T, res.Error)
	require.Len(h.T, parkedPieceRef, 1)
	require.True(h.T, parkedPieceRef[0].LongTerm)

	// ensure the prepareUpload is removed from the table
	var rmUpload []models.PDPPieceUpload
	res = h.DB.Where(&models.PDPPieceUpload{ID: expectedID}).Find(&rmUpload)
	require.NoError(h.T, res.Error)
	require.Len(h.T, rmUpload, 0)

	var parkedPieceReady []models.ParkedPiece
	res = h.DB.Where(&models.ParkedPiece{PieceCID: pieceCid.String()}).
		Find(&parkedPieceReady)
	require.NoError(h.T, res.Error)
	require.Len(h.T, parkedPieceReady, 1)
	require.EqualValues(h.T, len(piece), parkedPiece[0].PieceRawSize)
	require.True(h.T, parkedPieceReady[0].LongTerm)
	require.True(h.T, parkedPieceReady[0].Complete)

	// TODO make an actual assertion on the CID equal to the expected
	require.NotEqual(h.T, cid.Undef, pieceCid)
	h.T.Logf("found piece CID: %s", pieceCid)

	return resp
}

func (h *Harness) CreateProofSet(ctx context.Context, svc *service.PDPService, id, challengeWindow, provingPeriod uint64) *models.PDPProofSet {
	h.SendCreateProofSet(id)

	proofSetCreatTx, err := svc.CreateProofSet(ctx, RecordKeepAddress)
	require.NoError(h.T, err)

	h.Require_MessageSendsEth_SendSuccess("pdp-mkproofset", proofSetCreatTx)

	// trigger processing of watch_eth task
	currentHeight := h.Chain.AdvanceByHeight(tasks.MinConfidence)

	h.EthClient.MockMessageWatcherEthClient.EXPECT().
		TransactionReceipt(gomock.Any(), common.HexToHash(proofSetCreatTx.Hex())).
		Return(NewSuccessfulReceipt(int64(currentHeight-tasks.MinConfidence)), nil)

	h.EthClient.MockMessageWatcherEthClient.EXPECT().
		TransactionByHash(gomock.Any(), common.HexToHash(proofSetCreatTx.Hex())).
		Return(NewCreateProofSetTransaction(h.T), false, nil)

	h.WaitFor_MessageWaitsEth_TxSuccess(proofSetCreatTx)

	// trigger processing of watch_create task
	h.Chain.AdvanceChain()

	h.Contract.EXPECT().GetProofSetIdFromReceipt(gomock.Any()).Return(id, nil)
	h.Verifier.EXPECT().GetProofSetListener(gomock.Any(), big.NewInt(1)).Return(ListenerAddress, nil).AnyTimes()
	h.Schedule.EXPECT().GetMaxProvingPeriod(gomock.Any()).Return(provingPeriod, nil)
	h.Schedule.EXPECT().ChallengeWindow(gomock.Any()).Return(big.NewInt(int64(challengeWindow)), nil)

	h.WaitFor_PDPProofsetCreate_OK(proofSetCreatTx)
	h.WaitFor_PDPProofsetCreate_ProofsetCreated(proofSetCreatTx)

	var proofSet []models.PDPProofSet
	res := h.DB.Where(&models.PDPProofSet{CreateMessageHash: proofSetCreatTx.Hex()}).
		Find(&proofSet)

	require.NoError(h.T, res.Error)
	require.Len(h.T, proofSet, 1)
	require.Nil(h.T, proofSet[0].PrevChallengeRequestEpoch)
	require.Nil(h.T, proofSet[0].ChallengeRequestTaskID)
	require.Nil(h.T, proofSet[0].ChallengeRequestMsgHash)
	require.Nil(h.T, proofSet[0].ProveAtEpoch)
	require.False(h.T, proofSet[0].InitReady)
	require.EqualValues(h.T, challengeWindow, *proofSet[0].ChallengeWindow)
	require.EqualValues(h.T, provingPeriod, *proofSet[0].ProvingPeriod)

	return &proofSet[0]
}

func SetupTestDeps(t testing.TB, ctx context.Context, ctrl *gomock.Controller) (*Harness, *service.PDPService) {
	// a fake chain with methods to advance the state of the chain
	fakeChain := NewFakeChainClient(t)
	// a mocked ethereum client for interactions with the PDP contract
	mockEth := NewMockEthClient(ctrl)
	// various mocks for the PDP constract
	mockContract, mockVerifier, mockScheduler := NewMockContractClient(ctrl)

	// directory all state for test will be initialized in
	//testDir := "/home/frrist/workspace/"
	testDir := t.TempDir()

	// path to SQLite db used for test case
	dbPath := filepath.Join(testDir, SQLiteDBConfig)
	t.Logf("Database path: %s", dbPath)
	// sqlite database handle, may be queries in tests to assert state of DB
	db, err := gormdb.New(dbPath)
	require.NoError(t, err)

	// memory backed blob and stash store for PDP pieces.
	ss, err := stashstore.NewStashStore(testDir)
	require.NoError(t, err)
	bs := blobstore.NewTODOMapBlobstore()

	// memory backed keystore for persisting wallets
	ks := keystore.NewMemKeyStore()
	wlt, err := wallet.NewWallet(ks)
	require.NoError(t, err)

	// create a client address for interacting with PDP contract
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	clientAddress, err := wlt.Import(ctx, &keystore.KeyInfo{PrivateKey: crypto.FromECDSA(privKey)})
	require.NoError(t, err)
	t.Logf("Client address: %s", clientAddress.Hex())

	// Create a mock clock for consistent testing
	baseTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	mockClock := scheduler.NewMockClock(baseTime)

	// Advance the mock clock in the background so scheduler pollers and periodic tasks make progress
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				mockClock.Advance(100 * time.Millisecond)
			}
		}
	}()

	// The PDP service, backed by mocks and a fake chain
	svc, err := service.SetupPDPServiceWithClock(
		db,
		clientAddress,
		wlt,
		bs,
		ss,
		fakeChain,
		mockEth,
		mockContract,
		mockClock,
	)
	require.NoError(t, err)

	return &Harness{
		T:          t,
		Chain:      fakeChain,
		EthClient:  mockEth,
		Contract:   mockContract,
		Verifier:   mockVerifier,
		Schedule:   mockScheduler,
		DB:         db,
		ClientAddr: clientAddress,
		clock:      mockClock,
	}, svc

}

func Ptr[T any](v T) *T {
	return &v
}

func NewCreateProofSetTransaction(t testing.TB) *ethtypes.Transaction {
	abiData, err := contract.PDPVerifierMetaData()
	require.NoError(t, err)
	data, err := abiData.Pack("createProofSet", RecordKeepAddress, []byte{})
	require.NoError(t, err)
	return ethtypes.NewTransaction(
		0,
		contract.Addresses().PDPVerifier,
		contract.SybilFee(),
		0,
		nil,
		data,
	)
}

func NewSuccessfulReceipt(height int64) *ethtypes.Receipt {
	return &ethtypes.Receipt{
		BlockNumber: big.NewInt(height),
		Status:      ethtypes.ReceiptStatusSuccessful,
		Logs:        make([]*ethtypes.Log, 0),
	}
}

// AdvanceClock advances the mock clock by the given duration to trigger scheduled tasks
func (h *Harness) AdvanceClock(d time.Duration) {
	if h.clock != nil {
		if mockClock, ok := h.clock.(*scheduler.MockClock); ok {
			mockClock.Advance(d)
		}
	}
}
