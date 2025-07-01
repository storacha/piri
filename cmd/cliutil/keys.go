package cliutil

import (
	crypto_ed25519 "crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"os"

	"github.com/storacha/go-ucanto/principal"
	ed25519 "github.com/storacha/go-ucanto/principal/ed25519/signer"
)

func ReadPrivateKeyFromPEM(path string) (principal.Signer, error) {
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
