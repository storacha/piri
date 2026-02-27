package smartcontracts

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"golang.org/x/xerrors"

	"github.com/storacha/filecoin-services/go/bindings"
)

var log = logging.Logger("smartcontracts")

type Verifier interface {
	GetDataSetLeafCount(ctx context.Context, setId *big.Int) (*big.Int, error)
	GetNextChallengeEpoch(ctx context.Context, setId *big.Int) (*big.Int, error)
	GetDataSetListener(ctx context.Context, setId *big.Int) (common.Address, error)
	GetDataSetStorageProvider(ctx context.Context, setId *big.Int) (common.Address, common.Address, error)
	GetChallengeRange(ctx context.Context, setId *big.Int) (*big.Int, error)
	GetScheduledRemovals(ctx context.Context, setId *big.Int) ([]*big.Int, error)
	FindPieceIds(ctx context.Context, setId *big.Int, leafIndexs []*big.Int) ([]bindings.IPDPTypesPieceIdAndOffset, error)
	CalculateProofFee(ctx context.Context, setId *big.Int) (*big.Int, error)
	MaxPieceSizeLog2(ctx context.Context) (*big.Int, error)
	GetActivePieces(ctx context.Context, setID *big.Int, offset *big.Int, limit *big.Int) (*ActivePieces, error)
	GetActivePieceCount(ctx context.Context, setId *big.Int) (*big.Int, error)

	// not part of contract code, added for convience in testing and usage
	Address() common.Address
	GetDataSetIdFromReceipt(receipt *types.Receipt) (uint64, error)
	GetPieceIdsFromReceipt(receipt *types.Receipt) ([]uint64, error)
	GetABI() (*abi.ABI, error)
}

type verifierContract struct {
	address  common.Address
	verifier *bindings.PDPVerifier
	client   bind.ContractBackend
	abi      *abi.ABI
}

func NewVerifierContract(address common.Address, backend bind.ContractBackend) (Verifier, error) {
	verifier, err := bindings.NewPDPVerifier(address, backend)
	if err != nil {
		return nil, fmt.Errorf("creating verifier contract: %v", err)
	}

	pdpABI, err := bindings.PDPVerifierMetaData.GetAbi()
	if err != nil {
		return nil, fmt.Errorf("getting verifier ABI: %v", err)
	}
	return &verifierContract{
		address:  address,
		verifier: verifier,
		client:   backend,
		abi:      pdpABI,
	}, nil
}

type ActivePieces struct {
	Pieces   []cid.Cid
	PieceIds []*big.Int
	HasMore  bool
}

func (v *verifierContract) GetActivePieces(
	ctx context.Context,
	setID *big.Int,
	offset *big.Int,
	limit *big.Int,
) (*ActivePieces, error) {
	res, err := v.verifier.GetActivePieces(&bind.CallOpts{Context: ctx}, setID, offset, limit)
	if err != nil {
		return nil, err
	}

	pieces := make([]cid.Cid, len(res.Pieces))
	for i, piece := range res.Pieces {
		parsedCid, err := cid.Cast(piece.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse piece CID at index %d: %w", i, err)
		}
		pieces[i] = parsedCid
	}

	out := &ActivePieces{
		Pieces:   pieces,
		PieceIds: res.PieceIds,
		HasMore:  res.HasMore,
	}

	log.Debugw("cached GetActivePieces result", "setID", setID, "offset", offset, "limit", limit)

	return out, nil
}

func (v *verifierContract) GetActivePieceCount(ctx context.Context, setId *big.Int) (*big.Int, error) {
	return v.verifier.GetActivePieceCount(&bind.CallOpts{Context: ctx}, setId)
}

func (v *verifierContract) MaxPieceSizeLog2(ctx context.Context) (*big.Int, error) {
	return v.verifier.MAXPIECESIZELOG2(&bind.CallOpts{Context: ctx})
}

func (v *verifierContract) GetDataSetLeafCount(ctx context.Context, setId *big.Int) (*big.Int, error) {
	return v.verifier.GetDataSetLeafCount(&bind.CallOpts{Context: ctx}, setId)
}

func (v *verifierContract) GetNextChallengeEpoch(ctx context.Context, setId *big.Int) (*big.Int, error) {
	return v.verifier.GetNextChallengeEpoch(&bind.CallOpts{Context: ctx}, setId)
}

func (v *verifierContract) GetDataSetListener(ctx context.Context, setId *big.Int) (common.Address, error) {
	return v.verifier.GetDataSetListener(&bind.CallOpts{Context: ctx}, setId)
}

func (v *verifierContract) GetDataSetStorageProvider(ctx context.Context, setId *big.Int) (common.Address, common.Address, error) {
	return v.verifier.GetDataSetStorageProvider(&bind.CallOpts{Context: ctx}, setId)
}

func (v *verifierContract) GetChallengeRange(ctx context.Context, setId *big.Int) (*big.Int, error) {
	out, err := v.verifier.GetChallengeRange(&bind.CallOpts{Context: ctx}, setId)
	if err != nil {
		return nil, err
	}
	log.Infow("verifier challenge range", "setId", setId, "range", out.Uint64())
	return out, nil
}

func (v *verifierContract) GetScheduledRemovals(ctx context.Context, setId *big.Int) ([]*big.Int, error) {
	return v.verifier.GetScheduledRemovals(&bind.CallOpts{Context: ctx}, setId)
}

func (v *verifierContract) FindPieceIds(ctx context.Context, setId *big.Int, leafIndexs []*big.Int) ([]bindings.IPDPTypesPieceIdAndOffset, error) {
	out, err := v.verifier.FindPieceIds(&bind.CallOpts{Context: ctx}, setId, leafIndexs)
	if err != nil {
		return nil, err
	}

	// Format out for human-readable logging
	pieceInfo := make([]map[string]interface{}, len(out))
	for i, piece := range out {
		pieceInfo[i] = map[string]interface{}{
			"pieceId": piece.PieceId.String(),
			"offset":  piece.Offset.String(),
		}
	}
	log.Infow("FindPieceIds", "setId", setId, "leafIndexs", leafIndexs, "pieces", pieceInfo)
	return out, nil
}

func (v *verifierContract) CalculateProofFee(ctx context.Context, setId *big.Int) (*big.Int, error) {
	return v.verifier.CalculateProofFee(&bind.CallOpts{Context: ctx}, setId)
}

func (v *verifierContract) Address() common.Address {
	return v.address
}

// GetDataSetIdFromReceipt parses DataSetCreated event from transaction receipt
func (v *verifierContract) GetDataSetIdFromReceipt(receipt *types.Receipt) (uint64, error) {
	event, exists := v.abi.Events["DataSetCreated"]
	if !exists {
		return 0, xerrors.Errorf("DataSetCreated event not found in ABI")
	}

	for _, vLog := range receipt.Logs {
		if len(vLog.Topics) > 0 && vLog.Topics[0] == event.ID {
			if len(vLog.Topics) < 2 {
				return 0, xerrors.Errorf("log does not contain setId topic")
			}
			setIdBigInt := new(big.Int).SetBytes(vLog.Topics[1].Bytes())
			return setIdBigInt.Uint64(), nil
		}
	}

	return 0, xerrors.Errorf("DataSetCreated event not found in receipt")
}

func (v *verifierContract) GetABI() (*abi.ABI, error) {
	return v.abi, nil
}

// GetPieceIdsFromReceipt parses PiecesAdded event from transaction receipt
func (v *verifierContract) GetPieceIdsFromReceipt(receipt *types.Receipt) ([]uint64, error) {
	event, exists := v.abi.Events["PiecesAdded"]
	if !exists {
		return nil, fmt.Errorf("PiecesAdded event not found in ABI")
	}

	eventFound := false
	for _, vLog := range receipt.Logs {
		if len(vLog.Topics) > 0 && vLog.Topics[0] == event.ID {
			// The setId is an indexed parameter in Topics[1]
			// The pieceIds array is in the data field

			unpacked, err := event.Inputs.NonIndexed().Unpack(vLog.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to unpack log data: %w", err)
			}

			if len(unpacked) < 1 {
				return nil, fmt.Errorf("no unpacked data found in log")
			}

			// The first non-indexed parameter is pieceIds
			bigIntPieceIds, ok := unpacked[0].([]*big.Int)
			if !ok {
				return nil, fmt.Errorf("failed to convert unpacked data to []*big.Int array")
			}

			pieceIds := make([]uint64, len(bigIntPieceIds))
			for i := range bigIntPieceIds {
				pieceIds[i] = bigIntPieceIds[i].Uint64()
			}

			eventFound = true
			return pieceIds, nil
		}
	}

	if !eventFound {
		return nil, fmt.Errorf("PiecesAdded event not found in receipt")
	}

	return nil, fmt.Errorf("unexpected error in GetPieceIdsFromReceipt")
}
