package principalresolver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/validator"
)

var log = logging.Logger("principal-resolver")

// FlexibleContext handles both string and []string formats for @context field
// as allowed by the DID Core specification
type FlexibleContext []string

func (fc *FlexibleContext) UnmarshalJSON(data []byte) error {
	// Try array first (most common format)
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*fc = FlexibleContext(arr)
		return nil
	}

	// Fall back to single string format
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		*fc = FlexibleContext([]string{str})
		return nil
	}

	return fmt.Errorf("@context must be string or array of strings")
}

// Document is a did document that describes a did subject.
// See https://www.w3.org/TR/did-core/#dfn-did-documents.
// Copied from: https://github.com/storacha/indexing-service/blob/fe8f2211a15d851f2672bfeb64dcfc65c52e6011/pkg/server/server.go#L238
type Document struct {
	Context            FlexibleContext      `json:"@context"` // https://w3id.org/did/v1
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
	webKeys map[did.DID]url.URL
	cfg     config
}

type config struct {
	timeout  time.Duration
	insecure bool
}

type Option func(*config) error

func WithTimeout(timeout time.Duration) Option {
	return func(c *config) error {
		if timeout == 0 {
			return fmt.Errorf("timeout cannot be zero")
		}
		c.timeout = timeout
		return nil
	}
}

func InsecureResolution() Option {
	return func(c *config) error {
		c.insecure = true
		return nil
	}
}

const didWebPrefix = "did:web:"

// ExtractDomainFromDID extracts the domain from a DID web string
func ExtractDomainFromDID(didWeb did.DID) (string, error) {
	// Check if it starts with the required prefix
	if !strings.HasPrefix(didWeb.String(), didWebPrefix) {
		return "", fmt.Errorf("invalid DID web format: must start with '%s'", didWebPrefix)
	}

	// Extract the domain part
	domain := strings.TrimPrefix(didWeb.String(), didWebPrefix)

	// Check if domain is empty
	if domain == "" {
		return "", fmt.Errorf("invalid DID web format: no domain specified")
	}

	// Validate the domain format
	if err := validateDomain(domain); err != nil {
		return "", fmt.Errorf("invalid domain '%s': %w", domain, err)
	}

	return domain, nil
}

// validateDomain checks if a string is a valid domain name
func validateDomain(domain string) error {
	// Basic length check
	if len(domain) > 253 {
		return fmt.Errorf("domain too long (max 253 characters)")
	}

	// TODO we could do further checking that the domain is valid, length seems fine for now.

	return nil
}

const WellKnownDIDPath = "/.well-known/did.json"

func NewHTTPResolver(webKeys []did.DID, opts ...Option) (*HTTPResolver, error) {
	cfg := &config{
		timeout:  10 * time.Second,
		insecure: false,
	}
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}

	// Convert string map to DID/URL map
	didMap := make(map[did.DID]url.URL)
	for _, w := range webKeys {
		if _, ok := didMap[w]; ok {
			return nil, fmt.Errorf("duplicate did's provided")
		}
		domain, err := ExtractDomainFromDID(w)
		if err != nil {
			return nil, err
		}

		schema := "https"
		if cfg.insecure {
			schema = "http"
		}

		endpoint := url.URL{
			Scheme: schema,
			Host:   domain,
			Path:   WellKnownDIDPath,
		}

		if _, err := url.Parse(endpoint.String()); err != nil {
			return nil, fmt.Errorf("invalid did domain: %w", err)
		}

		didMap[w] = endpoint
	}
	// default timeout of 10 seconds, options can override
	resolver := &HTTPResolver{webKeys: didMap, cfg: *cfg}
	return resolver, nil
}

// TODO(forrest): the interface this implements in go-ucanto should probably accept a context
// since means of resolution here are open ended, and may go to network or disk.
func (r *HTTPResolver) ResolveDIDKey(input did.DID) (did.DID, validator.UnresolvedDID) {
	endpoint, ok := r.webKeys[input]
	if !ok {
		log.Error("failed to find did in set for resolution")
		return did.Undef, validator.NewDIDKeyResolutionError(input, fmt.Errorf("not found in mapping"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.cfg.timeout)
	defer cancel()
	didDoc, err := fetchDIDDocument(ctx, endpoint)
	if err != nil {
		log.Errorf("failed to resolve DID document from endpoint %s: %s", endpoint.String(), err)
		return did.Undef, validator.NewDIDKeyResolutionError(input, fmt.Errorf("failed to resolve DID document: %w", err))
	}
	if len(didDoc.VerificationMethod) == 0 {
		log.Errorf("failed to resolve DID document from endpoint %s: no verification methods", endpoint.String())
		return did.Undef, validator.NewDIDKeyResolutionError(input, fmt.Errorf("no verificationMethod found in DID document"))
	}

	pubKeyStr := didDoc.VerificationMethod[0].PublicKeyMultibase
	if pubKeyStr == "" {
		log.Errorf("failed to resolve DID document from endpoint %s: no public key", endpoint.String())
		return did.Undef, validator.NewDIDKeyResolutionError(input, fmt.Errorf("no public key found in DID document"))
	}

	didKey, err := did.Parse(fmt.Sprintf("did:key:%s", pubKeyStr))
	if err != nil {
		log.Errorf("failed to parse DID document from endpoint %s: %s", endpoint.String(), err)
		return did.Undef, validator.NewDIDKeyResolutionError(input, fmt.Errorf("failed to parse public multibase key: %w", err))
	}

	return didKey, nil
}

func fetchDIDDocument(ctx context.Context, endpoint url.URL) (*Document, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint.String(), nil)
	if err != nil {
		log.Error("failed to build request for DID document")
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("failed to make request for DID document at endpoint %s", endpoint.String())
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Errorf("bad status code for DID document at endpoint %s: %d", endpoint.String(), resp.StatusCode)
		return nil, fmt.Errorf("received status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("failed to read response body for DID document at endpoint %s: %s", endpoint.String(), err)
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var didDoc Document
	if err := json.Unmarshal(body, &didDoc); err != nil {
		log.Errorf("failed to unmarshal DID document at endpoint %s: %s", endpoint.String(), err)
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &didDoc, nil
}
