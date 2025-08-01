package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/storacha/piri/pkg/pdp/apiv2"
)

var _ apiv2.PDP = (*PDPClient)(nil)

const pdpRoutePath = "/pdp"
const proofSetsPath = "/proof-sets"
const piecePath = "/piece"
const pingPath = "/ping"
const rootsPath = "/roots"

// PDPClient implements PDP interface using HTTP calls
type PDPClient struct {
	endpoint   *url.URL
	authHeader string
	client     *http.Client
}

func New(client *http.Client, endpoint *url.URL, authHeader string) *PDPClient {
	return &PDPClient{
		endpoint:   endpoint,
		authHeader: authHeader,
		client:     client,
	}
}

func (c *PDPClient) Ping(ctx context.Context) error {
	u := c.endpoint.JoinPath(pdpRoutePath, pingPath).String()
	return c.verifySuccess(c.sendRequest(ctx, http.MethodGet, u, nil))
}

func (c *PDPClient) CreateProofSet(ctx context.Context, request apiv2.CreateProofSet) (apiv2.StatusRef, error) {
	u := c.endpoint.JoinPath(pdpRoutePath, proofSetsPath).String()
	// send request
	res, err := c.postJson(ctx, u, request)
	if err != nil {
		return apiv2.StatusRef{}, err
	}
	// all successful responses are 201
	if res.StatusCode != http.StatusCreated {
		return apiv2.StatusRef{}, errFromResponse(res)
	}

	return apiv2.StatusRef{URL: res.Header.Get("Location")}, nil
}

func (c *PDPClient) ProofSetCreationStatus(ctx context.Context, ref apiv2.StatusRef) (apiv2.ProofSetStatus, error) {
	// we could do this in a number of ways, including having StatusRef actually
	// just be the TXHash, extracted from the location header. But ultimately
	// it makes the most sense as an opaque reference from the standpoint of anyone
	// using the client
	// generate request
	u := c.endpoint.JoinPath(ref.URL).String()
	var proofSetStatus apiv2.ProofSetStatus
	err := c.getJsonResponse(ctx, u, &proofSetStatus)
	return proofSetStatus, err
}

func (c *PDPClient) GetProofSet(ctx context.Context, id uint64) (apiv2.ProofSet, error) {
	u := c.endpoint.JoinPath(pdpRoutePath, proofSetsPath, "/", strconv.FormatUint(id, 10)).String()
	var proofSet apiv2.ProofSet
	err := c.getJsonResponse(ctx, u, &proofSet)
	return proofSet, err
}

func (c *PDPClient) DeleteProofSet(ctx context.Context, id uint64) error {
	u := c.endpoint.JoinPath(pdpRoutePath, proofSetsPath, strconv.FormatUint(id, 10)).String()
	return c.verifySuccess(c.sendRequest(ctx, http.MethodDelete, u, nil))
}

func (c *PDPClient) AddRootsToProofSet(ctx context.Context, id uint64, roots []apiv2.AddRootRequest) error {
	u := c.endpoint.JoinPath(pdpRoutePath, proofSetsPath, "/", strconv.FormatUint(id, 10), rootsPath).String()
	payload := apiv2.AddRootsPayload{Roots: roots}
	return c.verifySuccess(c.postJson(ctx, u, payload))
}

func (c *PDPClient) AddPiece(ctx context.Context, addPiece apiv2.AddPiece) (*apiv2.UploadRef, error) {
	u := c.endpoint.JoinPath(pdpRoutePath, piecePath).String()
	res, err := c.postJson(ctx, u, addPiece)
	if err != nil {
		return nil, err
	}
	if res.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if res.StatusCode == http.StatusCreated {
		return &apiv2.UploadRef{
			URL: c.endpoint.JoinPath(res.Header.Get("Location")).String(),
		}, nil
	}
	return nil, errFromResponse(res)
}

func (c *PDPClient) UploadPiece(ctx context.Context, ref apiv2.UploadRef, data io.Reader) error {
	return c.verifySuccess(c.sendRequest(ctx, http.MethodPut, ref.URL, data))
}

func (c *PDPClient) FindPiece(ctx context.Context, piece apiv2.PieceHash) (apiv2.FoundPiece, error) {
	u := c.endpoint.JoinPath(pdpRoutePath, piecePath)
	query := u.Query()
	query.Add("size", strconv.FormatInt(piece.Size, 10))
	query.Add("name", piece.Name)
	query.Add("hash", piece.Hash)
	u.RawQuery = query.Encode()
	var foundPiece apiv2.FoundPiece
	err := c.getJsonResponse(ctx, u.String(), &foundPiece)
	return foundPiece, err
}

func (c *PDPClient) GetPiece(ctx context.Context, pieceCid string) (apiv2.PieceReader, error) {
	// piece gets are not at the pdp path but rather the raw /piece path
	u := c.endpoint.JoinPath(piecePath, "/", pieceCid).String()
	res, err := c.sendRequest(ctx, http.MethodGet, u, nil)
	if err != nil {
		return apiv2.PieceReader{}, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return apiv2.PieceReader{}, errFromResponse(res)
	}
	return apiv2.PieceReader{
		Data: res.Body,
		Size: res.ContentLength,
	}, nil
}

func (c *PDPClient) GetPieceURL(pieceCid string) url.URL {
	return *c.endpoint.JoinPath(piecePath, "/", pieceCid)
}

func (c *PDPClient) sendRequest(ctx context.Context, method string, url string, body io.Reader) (*http.Response, error) {
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

func (c *PDPClient) postJson(ctx context.Context, url string, params interface{}) (*http.Response, error) {
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

func (c *PDPClient) getJsonResponse(ctx context.Context, url string, target interface{}) error {
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

func (c *PDPClient) verifySuccess(res *http.Response, err error) error {
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
	return fmt.Sprintf("http request failed, status: %d %s, message: %s", e.StatusCode, http.StatusText(e.StatusCode), e.Body)
}
