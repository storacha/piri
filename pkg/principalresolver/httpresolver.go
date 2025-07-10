package principalresolver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/validator"
)

// Document is a did document that describes a did subject.
// See https://www.w3.org/TR/did-core/#dfn-did-documents.
// Copied from: https://github.com/storacha/indexing-service/blob/fe8f2211a15d851f2672bfeb64dcfc65c52e6011/pkg/server/server.go#L238
type Document struct {
	Context            []string             `json:"@context"` // https://w3id.org/did/v1
	ID                 string               `json:"id"`
	Controller         []string             `json:"controller,omitempty"`
	VerificationMethod []VerificationMethod `json:"verificationMethod,omitempty"`
	Authentication     []string             `json:"authentication,omitempty"`
	AssertionMethod    []string             `json:"assertionMethod,omitempty"`
}

// VerificationMethod describes how to authenticate or authorize interactions
// with a did subject.
// See https://www.w3.org/TR/did-core/#dfn-verification-method.
type VerificationMethod struct {
	ID                 string `json:"id,omitempty"`
	Type               string `json:"type,omitempty"`
	Controller         string `json:"controller,omitempty"`
	PublicKeyMultibase string `json:"publicKeyMultibase,omitempty"`
}

type HTTPResolver struct {
	// mapping of did:web to url of service, where we fetch .well-known/did.json to obtain their did:key key
	mapping map[did.DID]url.URL
	timeout time.Duration
}

type Option func(*HTTPResolver) error

func WithTimeout(timeout time.Duration) Option {
	return func(r *HTTPResolver) error {
		if timeout == 0 {
			return fmt.Errorf("timeout cannot be zero")
		}
		r.timeout = timeout
		return nil
	}
}

func NewHTTPResolver(smap map[string]string, opts ...Option) (*HTTPResolver, error) {
	// Convert string map to DID/URL map
	didMap := make(map[did.DID]url.URL)
	for k, v := range smap {
		didKey, err := did.Parse(k)
		if err != nil {
			return nil, fmt.Errorf("invalid DID %s: %w", k, err)
		}
		endpointURL, err := url.Parse(v)
		if err != nil {
			return nil, fmt.Errorf("invalid URL %s: %w", v, err)
		}
		didMap[didKey] = *endpointURL
	}

	// default timeout of 10 seconds, options can override
	resolver := &HTTPResolver{mapping: didMap, timeout: 10 * time.Second}
	for _, opt := range opts {
		if err := opt(resolver); err != nil {
			return nil, err
		}
	}
	return resolver, nil
}

// TODO(forrest): the interface the implements in go-ucanto should probably accept a context
// since means of resolution here are open ended, and may go to network or disk.
func (r *HTTPResolver) ResolveDIDKey(input did.DID) (did.DID, validator.UnresolvedDID) {
	endpoint, ok := r.mapping[input]
	if !ok {
		return did.Undef, validator.NewDIDKeyResolutionError(input, fmt.Errorf("not found in mapping"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	didDoc, err := fetchDIDDocument(ctx, endpoint)
	if err != nil {
		return did.Undef, validator.NewDIDKeyResolutionError(input, fmt.Errorf("failed to resolve DID document: %w", err))
	}
	if len(didDoc.VerificationMethod) == 0 {
		return did.Undef, validator.NewDIDKeyResolutionError(input, fmt.Errorf("no verificationMethod found in DID document"))
	}

	pubKeyStr := didDoc.VerificationMethod[0].PublicKeyMultibase
	if pubKeyStr == "" {
		return did.Undef, validator.NewDIDKeyResolutionError(input, fmt.Errorf("no public key found in DID document"))
	}

	didKey, err := did.Parse(fmt.Sprintf("did:key:%s", pubKeyStr))
	if err != nil {
		return did.Undef, validator.NewDIDKeyResolutionError(input, fmt.Errorf("failed to parse public multibase key: %w", err))
	}

	return didKey, nil
}

const WellKnownDIDPath = "/.well-known/did.json"

func fetchDIDDocument(ctx context.Context, endpoint url.URL) (*Document, error) {
	// Clone the URL to avoid modifying the original
	u := endpoint
	u.Path = path.Join(u.Path, WellKnownDIDPath)

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var didDoc Document
	if err := json.Unmarshal(body, &didDoc); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &didDoc, nil
}
