package client

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/go-ucanto/principal"

	"github.com/storacha/piri/lib"
	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/pdp/httpapi"
	"github.com/storacha/piri/pkg/pdp/types"
)

var log = logging.Logger("pdp/client")

var _ types.API = (*Client)(nil)

const (
	pdpRoutePath  = "/pdp"
	proofSetsPath = "/proof-sets"
	piecePath     = "/piece"
	pingPath      = "/ping"
	rootsPath     = "/roots"
)

type EndpointType string

const (
	GenericEndpoint EndpointType = "generic"
	PiriEndpoint    EndpointType = "piri"
)

type Client struct {
	authHeader string
	endpoint   *url.URL
	client     *http.Client
	serverType EndpointType
}

type Option func(c *Client) error

func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) error {
		c.client = client
		return nil
	}
}

func WithBearerFromSigner(id principal.Signer) Option {
	return func(c *Client) error {
		authHeader, err := createAuthBearerTokenFromID(id)
		if err != nil {
			return fmt.Errorf("creating auth header from ID: %w", err)
		}
		c.authHeader = authHeader
		return nil
	}
}

func WithEndpointType(t EndpointType) Option {
	return func(c *Client) error {
		c.serverType = t
		return nil
	}
}

// New creates a new PDP API client and automatically detects the server type
func New(endpoint *url.URL, opts ...Option) (*Client, error) {
	if endpoint == nil {
		return nil, fmt.Errorf("endpoint is required")
	}
	c := &Client{
		endpoint: endpoint,
		client:   http.DefaultClient,
	}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, fmt.Errorf("applying options: %w", err)
		}
	}

	// if a server type wasn't provided, attempt to detect it by pinging the endpoint
	if c.serverType == "" {
		// Detect server type
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := c.detectServerType(ctx); err != nil {
			log.Errorw("pdp client failed to ping endpoint", "endpoint", c.endpoint, "err", err)
			c.serverType = GenericEndpoint
		}
	}

	return c, nil
}

func NewFromConfig(cfg config.Client) (*Client, error) {
	endpoint, err := url.Parse(cfg.API.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("parsing node URL: %w", err)
	}
	id, err := lib.SignerFromEd25519PEMFile(cfg.Identity.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("loading identity key file: %w", err)
	}
	return New(endpoint, WithBearerFromSigner(id))
}

func createAuthBearerTokenFromID(id principal.Signer) (string, error) {
	claims := jwt.MapClaims{
		"service_name": "storacha",
	}

	// Create the token
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)

	// Sign the token
	tokenString, err := token.SignedString(ed25519.PrivateKey(id.Raw()))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %v", err)
	}

	return "Bearer " + tokenString, nil
}

func (c *Client) CreateProofSet(ctx context.Context, recordKeeper common.Address) (common.Hash, error) {
	route := c.endpoint.JoinPath(pdpRoutePath, proofSetsPath).String()
	request := httpapi.CreateProofSetRequest{
		RecordKeeper: recordKeeper.String(),
	}
	// send request
	res, err := c.postJson(ctx, route, request)
	if err != nil {
		return common.Hash{}, err
	}
	// all successful responses are 201
	if res.StatusCode != http.StatusCreated {
		return common.Hash{}, errFromResponse(res)
	}

	var payload httpapi.CreateProofSetResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return common.Hash{}, fmt.Errorf("failed to decode create proof-sets response: %w", err)
	}

	return common.HexToHash(payload.TxHash), nil
}

func (c *Client) GetProofSetStatus(ctx context.Context, txHash common.Hash) (*types.ProofSetStatus, error) {
	route := c.endpoint.JoinPath(pdpRoutePath, proofSetsPath, "created", txHash.String()).String()
	var resp httpapi.ProofSetStatusResponse
	err := c.getJsonResponse(ctx, route, &resp)
	if err != nil {
		return nil, err
	}

	id := uint64(0)
	if resp.ProofSetId != nil {
		id = *resp.ProofSetId
	}
	return &types.ProofSetStatus{
		TxHash:   common.HexToHash(resp.CreateMessageHash),
		TxStatus: resp.TxStatus,
		Created:  resp.ProofsetCreated,
		ID:       id,
	}, nil
}

func (c *Client) GetProofSet(ctx context.Context, proofSetID uint64) (*types.ProofSet, error) {
	route := c.endpoint.JoinPath(pdpRoutePath, proofSetsPath, "/", strconv.FormatUint(proofSetID, 10)).String()
	var proofSet httpapi.GetProofSetResponse
	err := c.getJsonResponse(ctx, route, &proofSet)
	if err != nil {
		return nil, fmt.Errorf("failed to get proof-set: %w", err)
	}
	nextChallenge := int64(0)
	if proofSet.NextChallengeEpoch != nil {
		nextChallenge = *proofSet.NextChallengeEpoch
	}
	roots := make([]types.RootEntry, 0, len(proofSet.Roots))
	for _, root := range proofSet.Roots {
		rcid, err := cid.Decode(root.RootCID)
		if err != nil {
			return nil, fmt.Errorf("failed to decode root CID: %w", err)
		}
		scid, err := cid.Decode(root.SubrootCID)
		if err != nil {
			return nil, fmt.Errorf("failed to decode subroot CID: %w", err)
		}
		roots = append(roots, types.RootEntry{
			RootCID:       rcid,
			RootID:        root.RootID,
			SubrootCID:    scid,
			SubrootOffset: root.SubrootOffset,
		})
	}
	out := &types.ProofSet{
		ID:                 proofSet.ID,
		NextChallengeEpoch: nextChallenge,
		Roots:              roots,
	}

	if !c.isPiriServer() {
		return out, nil
	}

	// response fields only supported by piri api.
	if proofSet.PreviousChallengeEpoch != nil {
		out.PreviousChallengeEpoch = *proofSet.PreviousChallengeEpoch
	}
	if proofSet.ProvingPeriod != nil {
		out.ProvingPeriod = *proofSet.ProvingPeriod
	}
	if proofSet.ChallengeWindow != nil {
		out.ChallengeWindow = *proofSet.ChallengeWindow
	}

	return out, nil
}

func (c *Client) GetProofSetState(ctx context.Context, proofSetID uint64) (*types.ProofSetState, error) {
	if !c.isPiriServer() {
		return nil, fmt.Errorf("pdp server does not support GetProofSetState")
	}
	route := c.endpoint.JoinPath(pdpRoutePath, proofSetsPath, "/", strconv.FormatUint(proofSetID, 10), "state").String()
	var state httpapi.GetProofSetStateResponse
	err := c.getJsonResponse(ctx, route, &state)
	if err != nil {
		return nil, fmt.Errorf("failed to get proof-set: %w", err)
	}

	return &types.ProofSetState{
		ID:                     state.ID,
		Initialized:            state.Initialized,
		NextChallengeEpoch:     state.NextChallengeEpoch,
		PreviousChallengeEpoch: state.PreviousChallengeEpoch,
		ProvingPeriod:          state.ProvingPeriod,
		ChallengeWindow:        state.ChallengeWindow,
		CurrentEpoch:           state.CurrentEpoch,
		ChallengedIssued:       state.ChallengedIssued,
		InChallengeWindow:      state.InChallengeWindow,
		IsInFaultState:         state.IsInFaultState,
		HasProven:              state.HasProven,
		IsProving:              state.IsProving,
		ContractState: types.ProofSetContractState{
			Owners:                   state.ContractState.Owners,
			NextChallengeWindowStart: state.ContractState.NextChallengeWindowStart,
			NextChallengeEpoch:       state.ContractState.NextChallengeEpoch,
			MaxProvingPeriod:         state.ContractState.MaxProvingPeriod,
			ChallengeWindow:          state.ContractState.ChallengeWindow,
			ChallengeRange:           state.ContractState.ChallengeRange,
			ScheduledRemovals:        state.ContractState.ScheduledRemovals,
			ProofFee:                 state.ContractState.ProofFee,
			ProofFeeBuffered:         state.ContractState.ProofFeeBuffered,
		},
	}, nil
}

func (c *Client) ListProofSet(ctx context.Context) ([]types.ProofSet, error) {
	if !c.isPiriServer() {
		return nil, fmt.Errorf("method requires piri server implementation: unsupported method")
	}
	route := c.endpoint.JoinPath(pdpRoutePath, proofSetsPath).String()
	var proofSets httpapi.ListProofSetsResponse
	err := c.getJsonResponse(ctx, route, &proofSets)
	if err != nil {
		return nil, fmt.Errorf("failed to get proof-set: %w", err)
	}
	out := make([]types.ProofSet, 0, len(proofSets))
	for _, p := range proofSets {
		nextChallenge := int64(0)
		if p.NextChallengeEpoch != nil {
			nextChallenge = *p.NextChallengeEpoch
		}
		roots := make([]types.RootEntry, 0, len(p.Roots))
		for _, root := range p.Roots {
			rcid, err := cid.Decode(root.RootCID)
			if err != nil {
				return nil, fmt.Errorf("failed to decode root CID: %w", err)
			}
			scid, err := cid.Decode(root.SubrootCID)
			if err != nil {
				return nil, fmt.Errorf("failed to decode subroot CID: %w", err)
			}
			roots = append(roots, types.RootEntry{
				RootCID:       rcid,
				RootID:        root.RootID,
				SubrootCID:    scid,
				SubrootOffset: root.SubrootOffset,
			})
		}
		entry := types.ProofSet{
			ID:                 p.ID,
			Initialized:        p.Initialized,
			NextChallengeEpoch: nextChallenge,
			Roots:              roots,
		}

		// response fields only supported by piri api.
		if p.PreviousChallengeEpoch != nil {
			entry.PreviousChallengeEpoch = *p.PreviousChallengeEpoch
		}
		if p.ProvingPeriod != nil {
			entry.ProvingPeriod = *p.ProvingPeriod
		}
		if p.ChallengeWindow != nil {
			entry.ChallengeWindow = *p.ChallengeWindow
		}
		out = append(out, entry)
	}
	return out, nil
}

func (c *Client) AddRoots(ctx context.Context, proofSetID uint64, roots []types.RootAdd) (common.Hash, error) {
	route := c.endpoint.JoinPath(pdpRoutePath, proofSetsPath, "/", strconv.FormatUint(proofSetID, 10), rootsPath).String()

	addRoots := make([]httpapi.Root, 0, len(roots))
	for _, root := range roots {
		subRoots := make([]httpapi.SubrootEntry, 0, len(root.SubRoots))
		for _, sub := range root.SubRoots {
			subRoots = append(subRoots, httpapi.SubrootEntry{
				SubrootCID: sub.String(),
			})
		}
		addRoots = append(addRoots, httpapi.Root{
			RootCID:  root.Root.String(),
			Subroots: subRoots,
		})
	}
	payload := httpapi.AddRootsRequest{Roots: addRoots}
	if !c.isPiriServer() {
		return common.Hash{}, c.verifySuccess(c.postJson(ctx, route, payload))
	}
	res, err := c.postJson(ctx, route, payload)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to add roots: %w", err)
	}
	if res.StatusCode != http.StatusCreated {
		return common.Hash{}, errFromResponse(res)
	}
	var resp httpapi.AddRootsResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return common.Hash{}, fmt.Errorf("failed to decode add roots: %w", err)
	}
	return common.HexToHash(resp.TxHash), nil
}

func (c *Client) RemoveRoot(ctx context.Context, proofSetID uint64, rootID uint64) (common.Hash, error) {
	route := c.endpoint.JoinPath(pdpRoutePath, proofSetsPath, strconv.FormatUint(proofSetID, 10), "roots", strconv.FormatUint(rootID, 10)).String()
	res, err := c.sendRequest(ctx, http.MethodDelete, route, nil)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to remove root: %w", err)
	}
	if !c.isPiriServer() {
		return common.Hash{}, nil
	}
	var payload httpapi.RemoveRootResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return common.Hash{}, fmt.Errorf("failed to decode remove roots: %w", err)
	}
	return common.HexToHash(payload.TxHash), nil
}

func (c *Client) AllocatePiece(ctx context.Context, allocation types.PieceAllocation) (*types.AllocatedPiece, error) {
	route := c.endpoint.JoinPath(pdpRoutePath, piecePath).String()
	req := httpapi.AddPieceRequest{
		Check: httpapi.PieceHash{
			Name: allocation.Piece.Name,
			Hash: allocation.Piece.Hash,
			Size: allocation.Piece.Size,
		},
	}
	if allocation.Notify != nil {
		req.Notify = allocation.Notify.String()
	}
	res, err := c.postJson(ctx, route, req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		return nil, errFromResponse(res)
	}
	if !c.isPiriServer() {
		// piece already exists
		if res.StatusCode == http.StatusOK {
			var result map[string]interface{}
			if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
				return nil, fmt.Errorf("failed to decode response for piece allocation: %w", err)
			}
			pieceCIDStr, ok := result["pieceCID"]
			if !ok {
				return nil, fmt.Errorf("failed to find pieceCID in response for piece allocation")
			}
			pieceCID, err := cid.Parse(pieceCIDStr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse pieceCID: %w", err)
			}
			return &types.AllocatedPiece{
				Allocated: false,
				Piece:     pieceCID,
			}, nil
		}
		// piece was created
		if res.StatusCode == http.StatusCreated {
			uid, err := uuid.Parse(res.Header.Get("Location"))
			if err != nil {
				return nil, fmt.Errorf("failed to parse piece's upload UUID: %w", err)
			}
			return &types.AllocatedPiece{
				Allocated: true,
				UploadID:  uid,
			}, nil
		}
	}
	var payload httpapi.AddPieceResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to decode response for piece: %w", err)
	}
	if res.StatusCode == http.StatusCreated {
		uid, err := uuid.Parse(payload.UploadID)
		if err != nil {
			return nil, fmt.Errorf("failed to parse piece's upload UUID: %w", err)
		}
		return &types.AllocatedPiece{
			Allocated: payload.Allocated,
			Piece:     cid.Undef,
			UploadID:  uid,
		}, nil
	}
	// else, already exists
	pcid, err := cid.Decode(payload.PieceCID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse piece CID: %w", err)
	}
	return &types.AllocatedPiece{
		Allocated: payload.Allocated,
		Piece:     pcid,
		UploadID:  uuid.Nil,
	}, nil
}

func (c *Client) UploadPiece(ctx context.Context, upload types.PieceUpload) error {
	route := c.endpoint.JoinPath(pdpRoutePath, piecePath, "upload", upload.ID.String()).String()
	return c.verifySuccess(c.sendRequest(ctx, http.MethodPut, route, upload.Data))
}

func (c *Client) FindPiece(ctx context.Context, piece types.Piece) (cid.Cid, bool, error) {
	route := c.endpoint.JoinPath(pdpRoutePath, piecePath)
	query := route.Query()
	query.Add("size", strconv.FormatInt(piece.Size, 10))
	query.Add("name", piece.Name)
	query.Add("hash", piece.Hash)
	route.RawQuery = query.Encode()
	res, err := c.sendRequest(ctx, http.MethodGet, route.String(), nil)
	if err != nil {
		return cid.Undef, false, fmt.Errorf("failed to find piece: %w", err)
	}
	if res.StatusCode == http.StatusNotFound {
		return cid.Undef, false, nil
	}
	if res.StatusCode == http.StatusOK {
		var foundPiece httpapi.FoundPieceResponse
		if err := json.NewDecoder(res.Body).Decode(&foundPiece); err != nil {
			return cid.Undef, false, fmt.Errorf("failed to decode response for piece: %w", err)
		}
		pcid, err := cid.Decode(foundPiece.PieceCID)
		if err != nil {
			return cid.Undef, false, fmt.Errorf("failed to parse found piece CID: %w", err)
		}
		return pcid, true, nil

	}
	return cid.Undef, false, errFromResponse(res)
}

func (c *Client) ReadPiece(ctx context.Context, piece cid.Cid) (*types.PieceReader, error) {
	// piece gets are not at the pdp path but rather the raw /piece path
	route := c.endpoint.JoinPath(piecePath, "/", piece.String()).String()
	res, err := c.sendRequest(ctx, http.MethodGet, route, nil)
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, errFromResponse(res)
	}
	return &types.PieceReader{
		Size: res.ContentLength,
		Data: res.Body,
	}, nil
}

// detectServerType pings the server to determine if it's a piri server or generic server
func (c *Client) detectServerType(ctx context.Context) error {
	route := c.endpoint.JoinPath(pdpRoutePath, pingPath).String()
	res, err := c.sendRequest(ctx, http.MethodGet, route, nil)
	if err != nil {
		return fmt.Errorf("failed to ping server: %w", err)
	}
	defer res.Body.Close()

	// Default to generic server
	c.serverType = "generic"

	// Check response status
	if res.StatusCode == http.StatusNoContent {
		// Server returned 204 No Content - it's a generic server
		return nil
	}

	if res.StatusCode != http.StatusOK {
		// Unexpected status, but not necessarily an error - treat as generic
		return nil
	}

	// Try to parse JSON response
	var pingResponse map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&pingResponse); err != nil {
		// Can't parse JSON - treat as generic server
		return nil
	}

	// Check if response has "type" field set to "piri"
	if typeValue, ok := pingResponse["type"]; ok {
		if typeStr, ok := typeValue.(string); ok && typeStr == string(PiriEndpoint) {
			c.serverType = PiriEndpoint
		}
	}

	return nil
}

// isPiriServer returns true if the client is connected to a piri server
func (c *Client) isPiriServer() bool {
	return c.serverType == PiriEndpoint
}

func (c *Client) sendRequest(ctx context.Context, method string, url string, body io.Reader) (*http.Response, error) {
	log.Debugf("requesting [%s]: %s", method, url)
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("generating http request: %w", err)
	}
	// add authorization header
	if c.authHeader != "" {
		req.Header.Add("Authorization", c.authHeader)
	}
	req.Header.Add("Content-Type", "application/json")
	// send request
	res, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request to pdp server: %w", err)
	}
	log.Debugf("sent request [%s]: %s response %s", method, url, res.Status)
	return res, nil
}

func (c *Client) postJson(ctx context.Context, url string, params interface{}) (*http.Response, error) {
	var body io.Reader
	if params != nil {
		asBytes, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("encoding request parameters: %w", err)
		}
		body = bytes.NewReader(asBytes)
	}

	return c.sendRequest(ctx, http.MethodPost, url, body)
}

func (c *Client) getJsonResponse(ctx context.Context, url string, target interface{}) error {
	res, err := c.sendRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return errFromResponse(res)
	}
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}
	err = json.Unmarshal(data, target)
	if err != nil {
		return fmt.Errorf("unmarshalling JSON response to target: %w", err)
	}
	return nil
}

func (c *Client) verifySuccess(res *http.Response, err error) error {
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return errFromResponse(res)
	}
	return nil
}

type ErrFailedResponse struct {
	StatusCode int
	Body       string
}

func errFromResponse(res *http.Response) ErrFailedResponse {
	err := ErrFailedResponse{StatusCode: res.StatusCode}

	message, merr := io.ReadAll(res.Body)
	if merr != nil {
		err.Body = merr.Error()
	} else {
		err.Body = string(message)
	}
	return err
}

func (e ErrFailedResponse) Error() string {
	return fmt.Sprintf("http request receieved unexpected status: %d %s, message: %s", e.StatusCode, http.StatusText(e.StatusCode), e.Body)
}
