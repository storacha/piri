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

	"github.com/storacha/piri/pkg/principalresolver"
)

func TestNewHTTPResolver(t *testing.T) {
	t.Run("creates resolver with default timeout", func(t *testing.T) {
		mapping := make(map[string]string)
		resolver, err := principalresolver.NewHTTPResolver(mapping)
		require.NoError(t, err)
		require.NotNil(t, resolver)
	})

	t.Run("creates resolver with custom timeout", func(t *testing.T) {
		mapping := make(map[string]string)
		resolver, err := principalresolver.NewHTTPResolver(mapping, principalresolver.WithTimeout(5*time.Second))
		require.NoError(t, err)
		require.NotNil(t, resolver)
	})

	t.Run("fails with zero timeout", func(t *testing.T) {
		mapping := make(map[string]string)
		resolver, err := principalresolver.NewHTTPResolver(mapping, principalresolver.WithTimeout(0))
		require.Error(t, err)
		require.Contains(t, err.Error(), "timeout cannot be zero")
		require.Nil(t, resolver)
	})
}

func TestHTTPResolver_ResolveDIDKey(t *testing.T) {
	testCases := []struct {
		name           string
		setupServer    func() *httptest.Server
		setupMapping   func(serverURL string) map[string]string
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
			setupMapping: func(serverURL string) map[string]string {
				didWeb, _ := did.Parse("did:web:example.com")
				u, _ := url.Parse(serverURL)
				return map[string]string{didWeb.String(): u.String()}
			},
			inputDID:       "did:web:example.com",
			expectedDIDKey: "did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
			expectError:    false,
		},
		{
			name:        "DID not in mapping",
			setupServer: func() *httptest.Server { return nil },
			setupMapping: func(serverURL string) map[string]string {
				return make(map[string]string)
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
			setupMapping: func(serverURL string) map[string]string {
				return map[string]string{"did:web:example.com": serverURL}
			},
			inputDID:      "did:web:example.com",
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
			setupMapping: func(serverURL string) map[string]string {
				return map[string]string{"did:web:example.com": serverURL}
			},
			inputDID:      "did:web:example.com",
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
			setupMapping: func(serverURL string) map[string]string {
				return map[string]string{"did:web:example.com": serverURL}
			},
			inputDID:      "did:web:example.com",
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
			setupMapping: func(serverURL string) map[string]string {
				return map[string]string{"did:web:example.com": serverURL}
			},
			inputDID:      "did:web:example.com",
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
			setupMapping: func(serverURL string) map[string]string {
				return map[string]string{"did:web:example.com": serverURL}
			},
			inputDID:      "did:web:example.com",
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

			resolver, err := principalresolver.NewHTTPResolver(mapping)
			require.NoError(t, err)

			inputDID, err := did.Parse(tc.inputDID)
			require.NoError(t, err)

			result, unresolvedErr := resolver.ResolveDIDKey(inputDID)

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

	didWeb, err := did.Parse("did:web:slow.com")
	require.NoError(t, err)

	serverURL, err := url.Parse(slowServer.URL)
	require.NoError(t, err)

	mapping := map[string]string{didWeb.String(): serverURL.String()}

	resolver, err := principalresolver.NewHTTPResolver(mapping, principalresolver.WithTimeout(50*time.Millisecond))
	require.NoError(t, err)

	result, unresolvedErr := resolver.ResolveDIDKey(didWeb)
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

	didWeb, err := did.Parse("did:web:example.com")
	require.NoError(t, err)

	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	mapping := map[string]string{didWeb.String(): serverURL.String()}

	resolver, err := principalresolver.NewHTTPResolver(mapping)
	require.NoError(t, err)

	result, unresolvedErr := resolver.ResolveDIDKey(didWeb)
	require.Nil(t, unresolvedErr)
	require.NotEqual(t, did.Undef, result)

	select {
	case <-requestReceived:
	case <-time.After(time.Second):
		t.Fatal("request was not received by server")
	}
}
