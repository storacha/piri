package principalresolver_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/storacha/go-ucanto/did"
	"github.com/stretchr/testify/require"

	"github.com/storacha/piri/pkg/presets"
	"github.com/storacha/piri/pkg/principalresolver"
)

func TestNewHTTPResolver(t *testing.T) {
	t.Run("creates resolver with default timeout", func(t *testing.T) {
		mapping := make([]did.DID, 0)
		resolver, err := principalresolver.NewHTTPResolver(mapping)
		require.NoError(t, err)
		require.NotNil(t, resolver)
	})

	t.Run("creates resolver with custom timeout", func(t *testing.T) {
		mapping := make([]did.DID, 0)
		resolver, err := principalresolver.NewHTTPResolver(mapping, principalresolver.WithTimeout(5*time.Second), principalresolver.InsecureResolution())
		require.NoError(t, err)
		require.NotNil(t, resolver)
	})

	t.Run("fails with zero timeout", func(t *testing.T) {
		mapping := make([]did.DID, 0)
		resolver, err := principalresolver.NewHTTPResolver(mapping, principalresolver.WithTimeout(0))
		require.Error(t, err)
		require.Contains(t, err.Error(), "timeout cannot be zero")
		require.Nil(t, resolver)
	})

	t.Run("fails with duplicate DIDs", func(t *testing.T) {
		didWeb, _ := did.Parse("did:web:example.com")
		mapping := []did.DID{didWeb, didWeb}
		resolver, err := principalresolver.NewHTTPResolver(mapping)
		require.Error(t, err)
		require.Contains(t, err.Error(), "duplicate did's provided")
		require.Nil(t, resolver)
	})

	t.Run("fails with invalid DID format", func(t *testing.T) {
		didWeb, _ := did.Parse("did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK")
		mapping := []did.DID{didWeb}
		resolver, err := principalresolver.NewHTTPResolver(mapping)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid DID web format")
		require.Nil(t, resolver)
	})
}

func TestHTTPResolver_ResolveDIDKey(t *testing.T) {
	testCases := []struct {
		name           string
		setupServer    func() *httptest.Server
		setupMapping   func(serverURL string) []did.DID
		inputDID       string
		expectedDIDKey string
		expectError    bool
		errorContains  string
	}{
		{
			name: "successful resolution",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != principalresolver.WellKnownDIDPath {
						w.WriteHeader(http.StatusNotFound)
						return
					}
					doc := principalresolver.Document{
						Context: []string{"https://w3id.org/did/v1"},
						ID:      "did:web:example.com",
						VerificationMethod: []principalresolver.VerificationMethod{
							{
								ID:                 "did:web:example.com#key1",
								Type:               "Ed25519VerificationKey2018",
								Controller:         "did:web:example.com",
								PublicKeyMultibase: "z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
							},
						},
					}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(doc)
				}))
			},
			setupMapping: func(serverURL string) []did.DID {
				// Extract domain from server URL to create matching did:web
				u, _ := url.Parse(serverURL)
				didWeb, _ := did.Parse("did:web:" + u.Host)
				return []did.DID{didWeb}
			},
			inputDID:       "", // Will be set based on server URL
			expectedDIDKey: "did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
			expectError:    false,
		},
		{
			name:        "DID not in mapping",
			setupServer: func() *httptest.Server { return nil },
			setupMapping: func(serverURL string) []did.DID {
				return []did.DID{}
			},
			inputDID:      "did:web:notfound.com",
			expectError:   true,
			errorContains: "not found in mapping",
		},
		{
			name: "HTTP error response",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				}))
			},
			setupMapping: func(serverURL string) []did.DID {
				u, _ := url.Parse(serverURL)
				didWeb, _ := did.Parse("did:web:" + u.Host)
				return []did.DID{didWeb}
			},
			inputDID:      "", // Will be set based on server URL
			expectError:   true,
			errorContains: "received status 404",
		},
		{
			name: "invalid JSON response",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != principalresolver.WellKnownDIDPath {
						w.WriteHeader(http.StatusNotFound)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte("invalid json"))
				}))
			},
			setupMapping: func(serverURL string) []did.DID {
				u, _ := url.Parse(serverURL)
				didWeb, _ := did.Parse("did:web:" + u.Host)
				return []did.DID{didWeb}
			},
			inputDID:      "", // Will be set based on server URL
			expectError:   true,
			errorContains: "failed to parse JSON",
		},
		{
			name: "no verification methods",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != principalresolver.WellKnownDIDPath {
						w.WriteHeader(http.StatusNotFound)
						return
					}
					doc := principalresolver.Document{
						Context:            []string{"https://w3id.org/did/v1"},
						ID:                 "did:web:example.com",
						VerificationMethod: []principalresolver.VerificationMethod{},
					}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(doc)
				}))
			},
			setupMapping: func(serverURL string) []did.DID {
				u, _ := url.Parse(serverURL)
				didWeb, _ := did.Parse("did:web:" + u.Host)
				return []did.DID{didWeb}
			},
			inputDID:      "", // Will be set based on server URL
			expectError:   true,
			errorContains: "no verificationMethod found",
		},
		{
			name: "empty public key",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != principalresolver.WellKnownDIDPath {
						w.WriteHeader(http.StatusNotFound)
						return
					}
					doc := principalresolver.Document{
						Context: []string{"https://w3id.org/did/v1"},
						ID:      "did:web:example.com",
						VerificationMethod: []principalresolver.VerificationMethod{
							{
								ID:                 "did:web:example.com#key1",
								Type:               "Ed25519VerificationKey2018",
								Controller:         "did:web:example.com",
								PublicKeyMultibase: "",
							},
						},
					}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(doc)
				}))
			},
			setupMapping: func(serverURL string) []did.DID {
				u, _ := url.Parse(serverURL)
				didWeb, _ := did.Parse("did:web:" + u.Host)
				return []did.DID{didWeb}
			},
			inputDID:      "", // Will be set based on server URL
			expectError:   true,
			errorContains: "no public key found",
		},
		{
			name: "invalid public key format",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != principalresolver.WellKnownDIDPath {
						w.WriteHeader(http.StatusNotFound)
						return
					}
					doc := principalresolver.Document{
						Context: []string{"https://w3id.org/did/v1"},
						ID:      "did:web:example.com",
						VerificationMethod: []principalresolver.VerificationMethod{
							{
								ID:                 "did:web:example.com#key1",
								Type:               "Ed25519VerificationKey2018",
								Controller:         "did:web:example.com",
								PublicKeyMultibase: "invalid-key",
							},
						},
					}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(doc)
				}))
			},
			setupMapping: func(serverURL string) []did.DID {
				u, _ := url.Parse(serverURL)
				didWeb, _ := did.Parse("did:web:" + u.Host)
				return []did.DID{didWeb}
			},
			inputDID:      "", // Will be set based on server URL
			expectError:   true,
			errorContains: "failed to parse public multibase key",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var server *httptest.Server
			if tc.setupServer != nil {
				server = tc.setupServer()
				if server != nil {
					defer server.Close()
				}
			}

			var serverURL string
			if server != nil {
				serverURL = server.URL
			}
			mapping := tc.setupMapping(serverURL)

			resolver, err := principalresolver.NewHTTPResolver(mapping, principalresolver.InsecureResolution())
			require.NoError(t, err)

			// For tests where inputDID is empty, derive it from server URL
			var inputDID did.DID
			if tc.inputDID == "" && server != nil {
				u, _ := url.Parse(serverURL)
				inputDID, err = did.Parse("did:web:" + u.Host)
				require.NoError(t, err)
			} else {
				inputDID, err = did.Parse(tc.inputDID)
				require.NoError(t, err)
			}

			result, unresolvedErr := resolver.ResolveDIDKey(t.Context(), inputDID)

			if tc.expectError {
				require.NotNil(t, unresolvedErr)
				require.Contains(t, unresolvedErr.Error(), "Unable to resolve")
				require.Equal(t, did.Undef, result)
			} else {
				require.Nil(t, unresolvedErr)
				expectedDID, err := did.Parse(tc.expectedDIDKey)
				require.NoError(t, err)
				require.Equal(t, expectedDID, result)
			}
		})
	}
}

func TestHTTPResolver_ResolveDIDKey_Timeout(t *testing.T) {
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer slowServer.Close()

	u, err := url.Parse(slowServer.URL)
	require.NoError(t, err)

	didWeb, err := did.Parse("did:web:" + u.Host)
	require.NoError(t, err)

	mapping := []did.DID{didWeb}

	resolver, err := principalresolver.NewHTTPResolver(mapping, principalresolver.WithTimeout(50*time.Millisecond), principalresolver.InsecureResolution())
	require.NoError(t, err)

	result, unresolvedErr := resolver.ResolveDIDKey(t.Context(), didWeb)
	require.NotNil(t, unresolvedErr)
	require.Contains(t, unresolvedErr.Error(), "Unable to resolve")
	require.Equal(t, did.Undef, result)
}

func TestHTTPResolver_ResolveDIDKey_Context(t *testing.T) {
	requestReceived := make(chan bool, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case requestReceived <- true:
		default:
		}

		if r.URL.Path != principalresolver.WellKnownDIDPath {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		doc := principalresolver.Document{
			Context: []string{"https://w3id.org/did/v1"},
			ID:      "did:web:example.com",
			VerificationMethod: []principalresolver.VerificationMethod{
				{
					ID:                 "did:web:example.com#key1",
					Type:               "Ed25519VerificationKey2018",
					Controller:         "did:web:example.com",
					PublicKeyMultibase: "z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(doc)
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	require.NoError(t, err)

	didWeb, err := did.Parse("did:web:" + u.Host)
	require.NoError(t, err)

	mapping := []did.DID{didWeb}

	resolver, err := principalresolver.NewHTTPResolver(mapping, principalresolver.InsecureResolution())
	require.NoError(t, err)

	result, unresolvedErr := resolver.ResolveDIDKey(t.Context(), didWeb)
	require.Nil(t, unresolvedErr)
	require.NotEqual(t, did.Undef, result)

	select {
	case <-requestReceived:
	case <-time.After(time.Second):
		t.Fatal("request was not received by server")
	}
}

func TestFlexibleContext_UnmarshalJSON(t *testing.T) {
	testCases := []struct {
		name          string
		input         string
		expectedValue principalresolver.FlexibleContext
		expectError   bool
		errorContains string
	}{
		{
			name:          "single string context",
			input:         `"https://w3id.org/did/v1"`,
			expectedValue: principalresolver.FlexibleContext{"https://w3id.org/did/v1"},
			expectError:   false,
		},
		{
			name:          "array of strings context",
			input:         `["https://w3id.org/did/v1", "https://w3id.org/security/v1"]`,
			expectedValue: principalresolver.FlexibleContext{"https://w3id.org/did/v1", "https://w3id.org/security/v1"},
			expectError:   false,
		},
		{
			name:          "empty array context",
			input:         `[]`,
			expectedValue: principalresolver.FlexibleContext{},
			expectError:   false,
		},
		{
			name:          "invalid type - number",
			input:         `123`,
			expectError:   true,
			errorContains: "@context must be string or array of strings",
		},
		{
			name:          "invalid type - object",
			input:         `{"foo": "bar"}`,
			expectError:   true,
			errorContains: "@context must be string or array of strings",
		},
		{
			name:          "invalid type - boolean",
			input:         `true`,
			expectError:   true,
			errorContains: "@context must be string or array of strings",
		},
		{
			name:          "array with non-string elements",
			input:         `["https://w3id.org/did/v1", 123]`,
			expectError:   true,
			errorContains: "@context must be string or array of strings",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var fc principalresolver.FlexibleContext
			err := json.Unmarshal([]byte(tc.input), &fc)

			if tc.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errorContains)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedValue, fc)
			}
		})
	}
}

func TestHTTPResolver_ResolveDIDKey_ContextFormats(t *testing.T) {
	testCases := []struct {
		name           string
		setupServer    func() *httptest.Server
		expectedDIDKey string
	}{
		{
			name: "DID document with string context",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != principalresolver.WellKnownDIDPath {
						w.WriteHeader(http.StatusNotFound)
						return
					}
					// Using raw JSON to ensure we send a string context, not array
					docJSON := `{
						"@context": "https://w3id.org/did/v1",
						"id": "did:web:example.com",
						"verificationMethod": [{
							"id": "did:web:example.com#key1",
							"type": "Ed25519VerificationKey2018",
							"controller": "did:web:example.com",
							"publicKeyMultibase": "z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK"
						}]
					}`
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(docJSON))
				}))
			},
			expectedDIDKey: "did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
		},
		{
			name: "DID document with array context",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != principalresolver.WellKnownDIDPath {
						w.WriteHeader(http.StatusNotFound)
						return
					}
					doc := principalresolver.Document{
						Context: principalresolver.FlexibleContext{"https://w3id.org/did/v1", "https://w3id.org/security/v1"},
						ID:      "did:web:example.com",
						VerificationMethod: []principalresolver.VerificationMethod{
							{
								ID:                 "did:web:example.com#key1",
								Type:               "Ed25519VerificationKey2018",
								Controller:         "did:web:example.com",
								PublicKeyMultibase: "z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
							},
						},
					}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(doc)
				}))
			},
			expectedDIDKey: "did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			server := tc.setupServer()
			defer server.Close()

			u, err := url.Parse(server.URL)
			require.NoError(t, err)

			didWeb, err := did.Parse("did:web:" + u.Host)
			require.NoError(t, err)

			resolver, err := principalresolver.NewHTTPResolver([]did.DID{didWeb}, principalresolver.InsecureResolution())
			require.NoError(t, err)

			result, unresolvedErr := resolver.ResolveDIDKey(t.Context(), didWeb)
			require.Nil(t, unresolvedErr)

			expectedDID, err := did.Parse(tc.expectedDIDKey)
			require.NoError(t, err)
			require.Equal(t, expectedDID, result)
		})
	}
}

func TestExtractDomainFromDID(t *testing.T) {
	testCases := []struct {
		name           string
		did            string
		expectedDomain string
		expectError    bool
		errorContains  string
	}{
		{
			name:           "valid did:web",
			did:            "did:web:example.com",
			expectedDomain: "example.com",
			expectError:    false,
		},
		{
			name:           "valid did:web with subdomain",
			did:            "did:web:api.example.com",
			expectedDomain: "api.example.com",
			expectError:    false,
		},
		{
			name:          "invalid prefix",
			did:           "did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
			expectError:   true,
			errorContains: "invalid DID web format: must start with 'did:web:'",
		},
		{
			name:          "empty domain",
			did:           "did:web:",
			expectError:   true,
			errorContains: "invalid DID web format: no domain specified",
		},
		{
			name:          "domain too long",
			did:           "did:web:" + string(make([]byte, 254)),
			expectError:   true,
			errorContains: "domain too long",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			did, err := did.Parse(tc.did)
			require.NoError(t, err)

			domain, err := principalresolver.ExtractDomainFromDID(did)

			if tc.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errorContains)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedDomain, domain)
			}
		})
	}
}

func TestRealTest(t *testing.T) {
	iKey, _ := did.Parse("did:key:z6Mkr4QkdinnXQmJ9JdnzwhcEjR8nMnuVPEwREyh9jp2Pb7k")
	uKey, _ := did.Parse("did:key:z6MkpR58oZpK7L3cdZZciKT25ynGro7RZm6boFouWQ7AzF7v")

	presolv, err := principalresolver.NewHTTPResolver([]did.DID{presets.IndexingServiceDID, presets.UploadServiceDID})
	require.NoError(t, err)

	resp, err := presolv.ResolveDIDKey(t.Context(), presets.IndexingServiceDID)
	require.NoError(t, err)

	require.Equal(t, iKey, resp.DID())

	resp, err = presolv.ResolveDIDKey(t.Context(), presets.UploadServiceDID)
	require.NoError(t, err)

	require.Equal(t, uKey, resp.DID())
}
