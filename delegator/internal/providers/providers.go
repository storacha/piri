package providers

import (
	crypto_ed25519 "crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"os"

	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/principal"
	ed25519 "github.com/storacha/go-ucanto/principal/ed25519/signer"
	"go.uber.org/fx"

	"github.com/storacha/piri/delegator/internal/config"
)

type SignerParams struct {
	fx.In
	Config *config.Config
}

type SignerResult struct {
	fx.Out
	Signer principal.Signer
}

func ProvideSigner(params SignerParams) (SignerResult, error) {
	signer, err := signerFromEd25519PEMFile(params.Config.Delegator.KeyFile)
	if err != nil {
		return SignerResult{}, err
	}

	return SignerResult{Signer: signer}, nil
}

func signerFromEd25519PEMFile(path string) (principal.Signer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	pemData, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("reading private key: %w", err)
	}

	var privateKey *crypto_ed25519.PrivateKey
	rest := pemData

	// Loop until no more blocks
	for {
		block, remaining := pem.Decode(rest)
		if block == nil {
			// No more PEM blocks
			break
		}
		rest = remaining

		// Look for "PRIVATE KEY"
		if block.Type == "PRIVATE KEY" {
			parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("failed to parse PKCS#8 private key: %w", err)
			}

			// We expect a ed25519 private key, cast it
			key, ok := parsedKey.(crypto_ed25519.PrivateKey)
			if !ok {
				return nil, fmt.Errorf("the parsed key is not an ED25519 private key")
			}
			privateKey = &key
			break
		}
	}

	if privateKey == nil {
		return nil, fmt.Errorf("could not find a PRIVATE KEY block in the PEM file")
	}
	return ed25519.FromRaw(*privateKey)
}

type IndexingServiceWebDIDParams struct {
	fx.In
	Config *config.Config
}

type IndexingServiceWebDIDResult struct {
	fx.Out
	IndexingServiceWebDID did.DID `name:"indexing_service_web_did"`
}

func ProvideIndexingServiceWebDID(params IndexingServiceWebDIDParams) (IndexingServiceWebDIDResult, error) {
	parsedDID, err := did.Parse(params.Config.Delegator.IndexingServiceWebDID)
	if err != nil {
		return IndexingServiceWebDIDResult{}, fmt.Errorf("failed to parse indexing service DID: %w", err)
	}

	return IndexingServiceWebDIDResult{IndexingServiceWebDID: parsedDID}, nil
}

type UploadServiceDIDParams struct {
	fx.In
	Config *config.Config
}

type UploadServiceDIDResult struct {
	fx.Out
	UploadServiceDID did.DID `name:"upload_service_did"`
}

func ProvideUploadServiceDID(params UploadServiceDIDParams) (UploadServiceDIDResult, error) {
	parsedDID, err := did.Parse(params.Config.Delegator.UploadServiceDID)
	if err != nil {
		return UploadServiceDIDResult{}, fmt.Errorf("failed to parse upload service DID: %w", err)
	}

	return UploadServiceDIDResult{UploadServiceDID: parsedDID}, nil
}

type IndexingServiceProofParams struct {
	fx.In
	Config *config.Config
}

type IndexingServiceProofResult struct {
	fx.Out
	IndexingServiceProof delegation.Delegation `name:"indexing_service_proof"`
}

func ProvideIndexingServiceProof(params IndexingServiceProofParams) (IndexingServiceProofResult, error) {
	proof, err := delegation.Parse(params.Config.Delegator.IndexingServiceProof)
	if err != nil {
		return IndexingServiceProofResult{}, fmt.Errorf("failed to parse proof: %w", err)
	}

	return IndexingServiceProofResult{IndexingServiceProof: proof}, nil
}
