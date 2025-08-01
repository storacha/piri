package apiv2

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"

	"github.com/storacha/piri/pkg/pdp/service"
	"github.com/storacha/piri/pkg/pdp/service/types"
	"github.com/storacha/piri/pkg/store"
)

var _ PDP = (*API)(nil)

// API implements core PDP operations (transport-agnostic)
type API struct {
	service  *service.PDPService
	endpoint *url.URL
}

// endpoint is the URL the service backing this api is avaliable at
func New(endpoint *url.URL, s *service.PDPService) *API {
	return &API{service: s, endpoint: endpoint}
}

func (h *API) CreateProofSet(ctx context.Context, req CreateProofSet) (StatusRef, error) {
	if !common.IsHexAddress(req.RecordKeeper) {
		return StatusRef{}, NewError(http.StatusBadRequest, "record keeper address is not a valid address")
	}
	recordKeeperAddr := common.HexToAddress(req.RecordKeeper)
	resp, err := h.service.ProofSetCreate(ctx, recordKeeperAddr)
	if err != nil {
		return StatusRef{}, WrapError(err, http.StatusInternalServerError, "failed to create proof set")
	}
	return StatusRef{URL: path.Join("/pdp/proof-sets/created", resp.String())}, nil
}

func (h *API) ProofSetCreationStatus(ctx context.Context, ref StatusRef) (ProofSetStatus, error) {
	// Clean txHash (ensure it starts with '0x' and is lowercase)
	txHash := path.Base(ref.URL)
	if !strings.HasPrefix(txHash, "0x") {
		txHash = "0x" + txHash
	}
	txHash = strings.ToLower(txHash)

	// Validate txHash is a valid hash
	if len(txHash) != 66 { // '0x' + 64 hex chars
		return ProofSetStatus{}, NewError(http.StatusBadRequest, "invalid tx hash length: %s", txHash)
	}
	if _, err := hex.DecodeString(txHash[2:]); err != nil {
		return ProofSetStatus{}, WrapError(err, http.StatusBadRequest, "invalid tx hash: %s", txHash)
	}
	txh := common.HexToHash(txHash)
	status, err := h.service.ProofSetStatus(ctx, txh)
	if err != nil {
		return ProofSetStatus{}, WrapError(err, http.StatusInternalServerError, "failed to set proof set status")
	}
	psID := uint64(status.ProofSetId)
	return ProofSetStatus{
		CreateMessageHash: status.CreateMessageHash,
		ProofsetCreated:   status.ProofsetCreated,
		Service:           status.Service,
		TxStatus:          status.TxStatus,
		OK:                &status.OK,
		ProofSetId:        &psID,
	}, nil
}

func (h *API) GetProofSet(ctx context.Context, id uint64) (ProofSet, error) {
	ps, err := h.service.ProofSet(ctx, int64(id))
	if err != nil {
		if errors.Is(err, service.ErrProofSetNotFound) {
			return ProofSet{}, NewError(http.StatusNotFound, "proof set not found")
		}
		return ProofSet{}, WrapError(err, http.StatusInternalServerError, "failed to get proof set")
	}

	resp := ProofSet{
		ID:                 uint64(ps.ID),
		NextChallengeEpoch: &ps.NextChallengeEpoch,
	}
	for _, root := range ps.Roots {
		resp.Roots = append(resp.Roots, RootEntry{
			RootID:        root.RootID,
			RootCID:       root.RootCID,
			SubrootCID:    root.SubrootCID,
			SubrootOffset: root.SubrootOffset,
		})
	}
	return resp, nil
}

func (h *API) DeleteProofSetRoot(ctx context.Context, proofSetID uint64, rootID uint64) error {
	return h.service.RemoveRoot(ctx, proofSetID, rootID)
}

func (h *API) DeleteProofSet(ctx context.Context, id uint64) error {
	return NewError(http.StatusNotImplemented, "delete proofSet not implemented")
}

func (h *API) AddRootsToProofSet(ctx context.Context, id uint64, roots []AddRootRequest) error {
	serviceRequests := make([]service.AddRootRequest, 0, len(roots))
	for _, r := range roots {
		subroots := make([]string, 0, len(r.Subroots))
		for _, s := range r.Subroots {
			subroots = append(subroots, s.SubrootCID)
		}
		serviceRequests = append(serviceRequests, service.AddRootRequest{
			RootCID:     r.RootCID,
			SubrootCIDs: subroots,
		})
	}

	// TODO return the tx hash of the proof set create message
	todoHash, err := h.service.ProofSetAddRoot(ctx, int64(id), serviceRequests)
	_ = todoHash
	return err
}

func (h *API) AddPiece(ctx context.Context, piece AddPiece) (*UploadRef, error) {
	// Validate input
	if piece.Check.Hash == "" {
		return nil, NewError(http.StatusBadRequest, "piece hash is required")
	}
	if piece.Check.Name == "" {
		return nil, NewError(http.StatusBadRequest, "piece name is required")
	}

	resp, err := h.service.PreparePiece(ctx, service.PiecePrepareRequest{
		Check: types.PieceHash{
			Name: piece.Check.Name,
			Hash: piece.Check.Hash,
			Size: piece.Check.Size,
		},
		Notify: piece.Notify,
	})
	if err != nil {
		return nil, WrapError(err, http.StatusInternalServerError, "failed to add piece")
	}
	// piece already exists
	// TODO do better, we should return a more complete response
	if !resp.Created {
		return nil, nil
	}
	return &UploadRef{URL: resp.Location}, nil
}

func (h *API) UploadPiece(ctx context.Context, ref UploadRef, data io.Reader) error {
	pieceUUID := path.Base(ref.URL)
	uploadID, err := uuid.Parse(pieceUUID)
	if err != nil {
		return WrapError(err, http.StatusBadRequest, "invalid upload uuid")
	}
	_, err = h.service.UploadPiece(ctx, uploadID, data)
	if err != nil {
		return WrapError(err, http.StatusInternalServerError, "failed to upload piece")
	}
	return nil
}

func (h *API) FindPiece(ctx context.Context, piece PieceHash) (FoundPiece, error) {
	// Validate input
	if piece.Hash == "" {
		return FoundPiece{}, NewError(http.StatusBadRequest, "piece hash is required")
	}
	if piece.Name == "" {
		return FoundPiece{}, NewError(http.StatusBadRequest, "piece name is required")
	}

	p, found, err := h.service.FindPiece(ctx, piece.Name, piece.Hash, piece.Size)
	if err != nil {
		return FoundPiece{}, WrapError(err, http.StatusInternalServerError, "failed to find piece")
	}
	if !found {
		return FoundPiece{}, NewError(http.StatusNotFound, "piece not found")
	}
	return FoundPiece{PieceCID: p.String()}, nil
}

func (h *API) GetPiece(ctx context.Context, pieceCid string) (PieceReader, error) {
	pCID, err := cid.Parse(pieceCid)
	if err != nil {
		return PieceReader{}, WrapError(err, http.StatusBadRequest, "invalid piece cid")
	}
	obj, err := h.service.Storage().Get(ctx, pCID.Hash())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return PieceReader{}, WrapError(err, http.StatusNotFound, "piece not found")
		}
		return PieceReader{}, WrapError(err, http.StatusInternalServerError, "failed to read piece")
	}
	// TODO we should return an io.ReadCloser, which the object store now supports.
	return PieceReader{
		Data: &wrapper{obj: obj.Body()},
		Size: obj.Size(),
	}, nil
}

const piecePath = "/piece"

func (h *API) GetPieceURL(pieceCid string) url.URL {
	return *h.endpoint.JoinPath(piecePath, "/", pieceCid)
}

func (h *API) Ping(_ context.Context) error {
	return nil
}

type wrapper struct {
	obj io.Reader
}

func (w *wrapper) Read(p []byte) (n int, err error) {
	return w.obj.Read(p)
}

func (w *wrapper) Close() error {
	return fmt.Errorf("close not implemented")
}

// Helper to check if an error from the service layer indicates "not found"
func isNotFoundError(err error) bool {
	// This depends on how your service layer reports not found errors
	// Examples:
	// - return err.Error() == "not found"
	// - return errors.Is(err, service.ErrNotFound)
	// - return strings.Contains(err.Error(), "not found")

	// For now, a simple check:
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "not found")
}
