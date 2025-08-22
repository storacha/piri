package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/go-libstoracha/capabilities/blob"
	"github.com/storacha/go-libstoracha/capabilities/blob/replica"
	"github.com/storacha/go-libstoracha/capabilities/claim"
	"github.com/storacha/go-libstoracha/capabilities/pdp"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/go-ucanto/ucan"
	"go.uber.org/fx"

	"github.com/storacha/piri/delegator/internal/store"
)

var log = logging.Logger("service/delegator")

var (
	// we use these to return valid http codes from the handlers using this service
	ErrDIDNotAllowed        = errors.New("did not is not allowed to register")
	ErrDIDAlreadyRegistered = errors.New("did already registered")
	ErrDIDNotRegistered     = errors.New("did not registered")
	ErrBadEndpoint          = errors.New("did not found at endpoint")
	ErrInvalidProof         = errors.New("invalid proof")
)

type DelegatorService struct {
	store store.Store

	signer principal.Signer

	indexingServiceWebDID did.DID
	indexingServiceProof  delegation.Delegation

	uploadServiceDID did.DID
}

type DelegatorParams struct {
	fx.In

	// the store registered providers are persisted to
	Store store.Store

	// the identity of the delegator service
	Signer principal.Signer

	// the web did of the indexing service (TODO is this still required after ./well-known change?
	IndexingServiceWebDID did.DID `name:"indexing_service_web_did"`
	// proof from the indexer, delegated to this delegator, allowing it to create delegations on behalf of indexing service
	IndexingServiceProof delegation.Delegation `name:"indexing_service_proof"`

	// the did of the upload service, used for validating operator proofs are correct
	UploadServiceDID did.DID `name:"upload_service_did"`
}

func NewDelegatorService(p DelegatorParams) *DelegatorService {
	return &DelegatorService{
		store:                 p.Store,
		signer:                p.Signer,
		indexingServiceWebDID: p.IndexingServiceWebDID,
		indexingServiceProof:  p.IndexingServiceProof,
		uploadServiceDID:      p.UploadServiceDID,
	}
}

type RegisterParams struct {
	DID           did.DID
	OwnerAddress  common.Address
	ProofSetID    uint64
	OperatorEmail string
	PublicURL     url.URL
	Proof         string
}

func (s *DelegatorService) Register(ctx context.Context, req RegisterParams) error {
	// ensure they are allowed to register
	allowed, err := s.store.IsAllowedDID(ctx, req.DID)
	if err != nil {
		return fmt.Errorf("failed to check if DID is allowed: %w", err)
	}
	if !allowed {
		return ErrDIDNotAllowed
	}

	// ensure they haven't already registered
	registered, err := s.store.IsRegisteredDID(ctx, req.DID)
	if err != nil {
		return fmt.Errorf("failed to check if DID is registered: %w", err)
	}
	if registered {
		return ErrDIDAlreadyRegistered
	}

	// ensure the did they claim to own is served from the endpoint they calim to own
	if valid, err := assertEndpointServesDID(ctx, req.PublicURL, req.DID); err != nil {
		log.Errorw("failed to assert endpoint", "DID", req.DID, "error", err)
		return err
	} else if !valid {
		return ErrBadEndpoint
	}

	// ensure the proof they provided, allowing the upload service to write to their node is valid
	if err := s.assertProofValid(req.Proof, req.DID); err != nil {
		log.Errorw("failed to validate proof", "error", err)
		return ErrInvalidProof
	}

	// if we reach here, they have a valid unregistered did in the allow list, with a domain service the did, and valid proof
	// so we can create the provider record now.
	now := time.Now()
	if err := s.store.RegisterProvider(ctx, store.StorageProviderInfo{
		Provider:      req.DID.String(),
		Endpoint:      req.PublicURL.String(),
		Address:       req.OwnerAddress.String(),
		ProofSet:      req.ProofSetID,
		OperatorEmail: req.OperatorEmail,
		Proof:         req.Proof,
		InsertedAt:    now,
		UpdatedAt:     now,
	}); err != nil {
		return fmt.Errorf("failed to register provider: %w", err)
	}
	// success!
	return nil
}

func (s *DelegatorService) IsRegisteredDID(ctx context.Context, operator did.DID) (bool, error) {
	// ensure they haven't already registered
	registered, err := s.store.IsRegisteredDID(ctx, operator)
	if err != nil {
		return false, fmt.Errorf("failed to check if DID is registered: %w", err)
	}
	return registered, nil
}

func (s *DelegatorService) RequestProof(ctx context.Context, operator did.DID) (delegation.Delegation, error) {
	// ensure they are allowed to register
	allowed, err := s.store.IsAllowedDID(ctx, operator)
	if err != nil {
		return nil, fmt.Errorf("failed to check if DID is allowed: %w", err)
	}
	if !allowed {
		return nil, ErrDIDNotAllowed
	}

	// ensure they haven't already registered
	registered, err := s.store.IsRegisteredDID(ctx, operator)
	if err != nil {
		return nil, fmt.Errorf("failed to check if DID is registered: %w", err)
	}
	// must be registered to request a proof
	if !registered {
		return nil, ErrDIDNotRegistered
	}

	// the node is in allow list, and already registered, they may haz proof
	proof, err := s.generateDelegation(operator)
	if err != nil {
		return nil, fmt.Errorf("failed to generate delegation: %w", err)
	}

	return proof, nil
}

// generateDelegation generates a delegation for the provider (stubbed)
func (s *DelegatorService) generateDelegation(id did.DID) (delegation.Delegation, error) {
	// the delegator creates a delegation for the storage node to invoke claim/cache w/ proof from indexer.
	indxToStrgDelegation, err := delegation.Delegate(
		s.signer,
		id,
		[]ucan.Capability[ucan.NoCaveats]{
			ucan.NewCapability(
				claim.CacheAbility,
				s.indexingServiceWebDID.String(),
				ucan.NoCaveats{},
			),
		},
		delegation.WithNoExpiration(),
		delegation.WithProof(delegation.FromDelegation(s.indexingServiceProof)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate delegation from indexing service to storage node: %w", err)
	}

	return indxToStrgDelegation, nil
}

func (s *DelegatorService) assertProofValid(proofString string, provider did.DID) error {
	if strings.TrimSpace(proofString) == "" {
		return fmt.Errorf("proof cannot be empty")
	}

	proof, err := delegation.Parse(proofString)
	if err != nil {
		return err
	}
	expiration := proof.Expiration()

	now := time.Now().Unix()
	if expiration != nil {
		if *expiration != 0 && *expiration <= int(now) {
			return fmt.Errorf("delegation expired. expiration: %d, now: %d", expiration, now)
		}
	}
	if proof.Issuer().DID().String() != provider.String() {
		return fmt.Errorf("delegation issuer (%s) does not match provider DID (%s)", proof.Issuer().DID().String(), provider)
	}
	if proof.Audience().DID().String() != s.uploadServiceDID.DID().String() {
		return fmt.Errorf("delegation audience (%s) does not match upload service DID (%s)", proof.Audience().DID().String(), s.uploadServiceDID.DID())
	}
	var expectedCapabilities = map[string]struct{}{
		blob.AcceptAbility:      {},
		blob.AllocateAbility:    {},
		replica.AllocateAbility: {},
		pdp.InfoAbility:         {},
	}
	if len(proof.Capabilities()) != len(expectedCapabilities) {
		return fmt.Errorf("expected exact %v capabilities, got %v", expectedCapabilities, proof.Capabilities())
	}
	for _, c := range proof.Capabilities() {
		_, ok := expectedCapabilities[c.Can()]
		if !ok {
			return fmt.Errorf("unexpected capability: %s", c.Can())
		}
		if c.With() != provider.String() {
			return fmt.Errorf("capability %s has unexpected resource %s, expected: %s", c.Can(), c.With(), provider)
		}
	}

	return nil
}

func assertEndpointServesDID(ctx context.Context, endpoint url.URL, expectedDID did.DID) (bool, error) {
	// Create HTTP client with reasonable timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return false, fmt.Errorf("creating request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("making request to %s: %w", endpoint.String(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status code %d from %s", resp.StatusCode, endpoint.String())
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("reading response body: %w", err)
	}

	// Parse the response to extract DID
	responseText := strings.TrimSpace(string(body))

	// Try to parse as JSON first (in case of structured response)
	var didResponse struct {
		DID string `json:"did"`
	}

	if err := json.Unmarshal(body, &didResponse); err == nil && didResponse.DID != "" {
		// Successfully parsed as JSON
		if didResponse.DID != expectedDID.String() {
			return false, nil
		}
		return true, nil
	}

	// TODO a dedicated DID endpoint on the storage node would be helpful here.
	// Parse plain text response to extract DID
	// Expected format: "🔥 storage v0.0.3-d6f3761-dirty\n- https://github.com/storacha/storage\n- did:key:z6MksvRCPWoXvMj8sUzuHiQ4pFkSawkKRz2eh1TALNEG6s3e"
	lines := strings.Split(responseText, "\n")
	var foundDID string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Look for lines that start with "- did:" or just "did:"
		if strings.HasPrefix(line, "- did:") {
			foundDID = strings.TrimPrefix(line, "- ")
			break
		} else if strings.HasPrefix(line, "did:") {
			foundDID = line
			break
		}
	}

	if foundDID == "" {
		return false, nil
	}

	if foundDID != expectedDID.String() {
		return false, nil
	}

	return true, nil
}
