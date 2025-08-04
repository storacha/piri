package curio

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

	"github.com/golang-jwt/jwt/v4"
	"github.com/storacha/go-ucanto/principal"
)

type PDPClient interface {
	Ping(ctx context.Context) error
	CreateProofSet(ctx context.Context, request CreateProofSet) (StatusRef, error)
	ProofSetCreationStatus(ctx context.Context, ref StatusRef) (ProofSetStatus, error)
	GetProofSet(ctx context.Context, id uint64) (ProofSet, error)
	DeleteProofSet(ctx context.Context, id uint64) error
	AddRootsToProofSet(ctx context.Context, id uint64, addRoots []AddRootRequest) error
	AddPiece(ctx context.Context, addPiece AddPiece) (*UploadRef, error)
	UploadPiece(ctx context.Context, ref UploadRef, data io.Reader) error
	FindPiece(ctx context.Context, piece PieceHash) (FoundPiece, error)
	GetPiece(ctx context.Context, pieceCid string) (io.ReadCloser, error)
	GetPieceURL(pieceCid string) url.URL
}

const pdpRoutePath = "/pdp"
const proofSetsPath = "/proof-sets"
const piecePath = "/piece"
const pingPath = "/ping"
const rootsPath = "/roots"

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
	return fmt.Sprintf("http request failed, status: %d %s, message: %s", e.StatusCode, http.StatusText(e.StatusCode), e.Body)
}

type (
	StatusRef struct {
		URL string
	}

	CreateProofSet struct {
		RecordKeeper string `json:"recordKeeper"`
	}

	ProofSetStatus struct {
		CreateMessageHash string  `json:"createMessageHash"`
		ProofsetCreated   bool    `json:"proofsetCreated"`
		Service           string  `json:"service"`
		TxStatus          string  `json:"txStatus"`
		OK                *bool   `json:"ok"`
		ProofSetId        *uint64 `json:"proofSetId,omitempty"`
	}

	// RootEntry represents a root in the proof set for JSON serialization
	RootEntry struct {
		RootID        uint64 `json:"rootId"`
		RootCID       string `json:"rootCid"`
		SubrootCID    string `json:"subrootCid"`
		SubrootOffset int64  `json:"subrootOffset"`
	}

	ProofSet struct {
		ID                 uint64      `json:"id"`
		NextChallengeEpoch *int64      `json:"nextChallengeEpoch"`
		Roots              []RootEntry `json:"roots"`
	}

	SubrootEntry struct {
		SubrootCID string `json:"subrootCid"`
	}

	AddRootRequest struct {
		RootCID  string         `json:"rootCid"`
		Subroots []SubrootEntry `json:"subroots"`
	}

	// AddRootsPayload defines the structure for the entire add roots request payload
	AddRootsPayload struct {
		Roots     []AddRootRequest `json:"roots"`
		ExtraData *string          `json:"extraData,omitempty"`
	}

	UploadRef struct {
		URL string
	}

	PieceHash struct {
		// Name of the hash function used
		// sha2-256-trunc254-padded - CommP
		// sha2-256 - Blob sha256
		Name string `json:"name"`

		// hex encoded hash
		Hash string `json:"hash"`

		// Size of the piece in bytes
		Size int64 `json:"size"`
	}

	AddPiece struct {
		Check  PieceHash `json:"check"`
		Notify string    `json:"notify,omitempty"`
	}

	FoundPiece struct {
		PieceCID string `json:"piece_cid"`
	}

	Client struct {
		authHeader string
		endpoint   *url.URL
		client     *http.Client
	}
)

var _ PDPClient = (*Client)(nil)

func NEw(client *http.Client, endpoint *url.URL, authHeader string) *Client {
	return &Client{
		authHeader: authHeader,
		endpoint:   endpoint,
		client:     client,
	}
}

func (c *Client) Ping(ctx context.Context) error {
	url := c.endpoint.JoinPath(pdpRoutePath, pingPath).String()
	return c.verifySuccess(c.sendRequest(ctx, http.MethodGet, url, nil))
}

func (c *Client) CreateProofSet(ctx context.Context, request CreateProofSet) (StatusRef, error) {
	url := c.endpoint.JoinPath(pdpRoutePath, proofSetsPath).String()
	// send request
	res, err := c.postJson(ctx, url, request)
	if err != nil {
		return StatusRef{}, err
	}
	// all successful responses are 201
	if res.StatusCode != http.StatusCreated {
		return StatusRef{}, errFromResponse(res)
	}

	return StatusRef{URL: res.Header.Get("Location")}, nil
}

func (c *Client) ProofSetCreationStatus(ctx context.Context, ref StatusRef) (ProofSetStatus, error) {
	// we could do this in a number of ways, including having StatusRef actually
	// just be the TXHash, extracted from the location header. But ultimately
	// it makes the most sense as an opaque reference from the standpoint of anyone
	// using the client
	// generate request
	url := c.endpoint.JoinPath(ref.URL).String()
	var proofSetStatus ProofSetStatus
	err := c.getJsonResponse(ctx, url, &proofSetStatus)
	return proofSetStatus, err
}

func (c *Client) GetProofSet(ctx context.Context, id uint64) (ProofSet, error) {
	url := c.endpoint.JoinPath(pdpRoutePath, proofSetsPath, "/", strconv.FormatUint(id, 10)).String()
	var proofSet ProofSet
	err := c.getJsonResponse(ctx, url, &proofSet)
	return proofSet, err
}

func (c *Client) DeleteProofSet(ctx context.Context, id uint64) error {
	url := c.endpoint.JoinPath(pdpRoutePath, proofSetsPath, strconv.FormatUint(id, 10)).String()
	return c.verifySuccess(c.sendRequest(ctx, http.MethodDelete, url, nil))
}

func (c *Client) AddRootsToProofSet(ctx context.Context, id uint64, roots []AddRootRequest) error {
	url := c.endpoint.JoinPath(pdpRoutePath, proofSetsPath, "/", strconv.FormatUint(id, 10), rootsPath).String()
	payload := AddRootsPayload{Roots: roots}
	return c.verifySuccess(c.postJson(ctx, url, payload))
}

func (c *Client) AddPiece(ctx context.Context, addPiece AddPiece) (*UploadRef, error) {
	url := c.endpoint.JoinPath(pdpRoutePath, piecePath).String()
	res, err := c.postJson(ctx, url, addPiece)
	if err != nil {
		return nil, err
	}
	if res.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if res.StatusCode == http.StatusCreated {
		return &UploadRef{
			URL: c.endpoint.JoinPath(res.Header.Get("Location")).String(),
		}, nil
	}
	return nil, errFromResponse(res)
}

func (c *Client) UploadPiece(ctx context.Context, ref UploadRef, data io.Reader) error {
	return c.verifySuccess(c.sendRequest(ctx, http.MethodPut, ref.URL, data))
}

func (c *Client) FindPiece(ctx context.Context, piece PieceHash) (FoundPiece, error) {
	url := c.endpoint.JoinPath(pdpRoutePath, piecePath)
	query := url.Query()
	query.Add("size", strconv.FormatInt(piece.Size, 10))
	query.Add("name", piece.Name)
	query.Add("hash", piece.Hash)
	url.RawQuery = query.Encode()
	var foundPiece FoundPiece
	err := c.getJsonResponse(ctx, url.String(), &foundPiece)
	return foundPiece, err
}

func (c *Client) GetPiece(ctx context.Context, pieceCid string) (io.ReadCloser, error) {
	// piece gets are not at the pdp path but rather the raw /piece path
	url := c.endpoint.JoinPath(piecePath, "/", pieceCid).String()
	res, err := c.sendRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, errFromResponse(res)
	}
	return res.Body, nil
}

func (c *Client) GetPieceURL(pieceCid string) url.URL {
	return *c.endpoint.JoinPath(piecePath, "/", pieceCid)
}

func (c *Client) sendRequest(ctx context.Context, method string, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("generating http request: %w", err)
	}
	// add authorization header
	req.Header.Add("Authorization", c.authHeader)
	req.Header.Add("Content-Type", "application/json")
	// send request
	res, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request to curio: %w", err)
	}
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

func CreateCurioJWTAuthHeader(serviceName string, id principal.Signer) (string, error) {
	// Create JWT claims
	claims := jwt.MapClaims{
		"service_name": serviceName,
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
