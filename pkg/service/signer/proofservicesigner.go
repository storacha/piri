package signer

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/storacha/filecoin-services/go/eip712"
	"github.com/storacha/go-libstoracha/capabilities/pdp/sign"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/receipt"
	"github.com/storacha/go-ucanto/ucan"
	signerclient "github.com/storacha/piri-signing-service/pkg/client"
	signertypes "github.com/storacha/piri-signing-service/pkg/types"
	"github.com/storacha/piri/pkg/service/proofs"
)

type Client struct {
	signerclient.Client
	proofService proofs.ProofService
}

// NewProofServiceSigner creates a new signing service that uses the proof service
// to obtain delegations from the signing service via `access/grant` before
// invoking signing requests.
func NewProofServiceSigner(conn client.Connection, proofService proofs.ProofService) signertypes.SigningService {
	sc := signerclient.Client{Connection: conn}
	return &Client{
		Client:       sc,
		proofService: proofService,
	}
}

func (c *Client) SignCreateDataSet(
	ctx context.Context,
	issuer ucan.Signer,
	dataSet *big.Int,
	payee common.Address,
	metadata []eip712.MetadataEntry,
	options ...delegation.Option,
) (*eip712.AuthSignature, error) {
	dlg, err := c.proofService.RequestAccess(
		ctx,
		c.Client.Connection.ID(),
		sign.DataSetCreateAbility,
		nil,
		proofs.WithConnection(c.Client.Connection),
	)
	if err != nil {
		return nil, fmt.Errorf("requesting access: %w", err)
	}
	var opts []delegation.Option
	opts = append(append(opts, options...), delegation.WithProof(delegation.FromDelegation(dlg)))
	return c.Client.SignCreateDataSet(ctx, issuer, dataSet, payee, metadata, opts...)
}

// SignAddPieces signs an AddPieces operation via UCAN invocation
func (c *Client) SignAddPieces(
	ctx context.Context,
	issuer ucan.Signer,
	dataSet *big.Int,
	firstAdded *big.Int,
	pieceData [][]byte,
	metadata [][]eip712.MetadataEntry,
	prfs [][]receipt.AnyReceipt,
	options ...delegation.Option,
) (*eip712.AuthSignature, error) {
	dlg, err := c.proofService.RequestAccess(
		ctx,
		c.Client.Connection.ID(),
		sign.PiecesAddAbility,
		nil,
		proofs.WithConnection(c.Client.Connection),
	)
	if err != nil {
		return nil, fmt.Errorf("requesting access: %w", err)
	}
	var opts []delegation.Option
	opts = append(append(opts, options...), delegation.WithProof(delegation.FromDelegation(dlg)))
	return c.Client.SignAddPieces(ctx, issuer, dataSet, firstAdded, pieceData, metadata, prfs, opts...)
}

// SignSchedulePieceRemovals signs a SchedulePieceRemovals operation via UCAN invocation
func (c *Client) SignSchedulePieceRemovals(
	ctx context.Context,
	issuer ucan.Signer,
	dataSet *big.Int,
	pieceIds []*big.Int,
	options ...delegation.Option,
) (*eip712.AuthSignature, error) {
	dlg, err := c.proofService.RequestAccess(
		ctx,
		c.Client.Connection.ID(),
		sign.PiecesRemoveScheduleAbility,
		nil,
		proofs.WithConnection(c.Client.Connection),
	)
	if err != nil {
		return nil, fmt.Errorf("requesting access: %w", err)
	}
	var opts []delegation.Option
	opts = append(append(opts, options...), delegation.WithProof(delegation.FromDelegation(dlg)))
	return c.Client.SignSchedulePieceRemovals(ctx, issuer, dataSet, pieceIds, opts...)
}

// SignDeleteDataSet signs a DeleteDataSet operation via UCAN invocation
func (c *Client) SignDeleteDataSet(
	ctx context.Context,
	issuer ucan.Signer,
	dataSet *big.Int,
	options ...delegation.Option,
) (*eip712.AuthSignature, error) {
	dlg, err := c.proofService.RequestAccess(
		ctx,
		c.Client.Connection.ID(),
		sign.DataSetDeleteAbility,
		nil,
		proofs.WithConnection(c.Client.Connection),
	)
	if err != nil {
		return nil, fmt.Errorf("requesting access: %w", err)
	}
	var opts []delegation.Option
	opts = append(append(opts, options...), delegation.WithProof(delegation.FromDelegation(dlg)))
	return c.Client.SignDeleteDataSet(ctx, issuer, dataSet, opts...)
}
