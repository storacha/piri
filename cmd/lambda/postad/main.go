package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipni/go-libipni/dagsync/ipnisync/head"
	"github.com/ipni/go-libipni/ingest/schema"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/multiformats/go-multihash"

	"github.com/ipfs/go-cid"

	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-libstoracha/metadata"
	"github.com/storacha/piri/cmd/lambda"
	"github.com/storacha/piri/pkg/aws"
)

func main() {
	lambda.StartHTTPHandler(makeHandler)
}

func makeHandler(cfg aws.Config) (http.Handler, error) {
	sk, err := crypto.UnmarshalEd25519PrivateKey(cfg.Signer.Raw())
	if err != nil {
		return nil, err
	}
	ipniStore := aws.NewS3Store(cfg.Config, cfg.IPNIStoreBucket, cfg.IPNIStorePrefix, cfg.S3Options...)
	chunkLinksTable := aws.NewDynamoProviderContextTable(cfg.Config, cfg.ChunkLinksTableName, cfg.DynamoOptions...)
	metadataTable := aws.NewDynamoProviderContextTable(cfg.Config, cfg.MetadataTableName, cfg.DynamoOptions...)
	publisherStore := store.NewPublisherStore(ipniStore, chunkLinksTable, metadataTable, store.WithMetadataContext(metadata.MetadataContext))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ad, err := decodeAdvert(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("decoding advert: %s", err.Error()), http.StatusBadRequest)
			return
		}

		if err := validateAdvertSig(sk, ad); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		adlink, err := publishAdvert(r.Context(), sk, publisherStore, ad)
		if err != nil {
			http.Error(w, fmt.Sprintf("publishing advert: %s", err.Error()), http.StatusInternalServerError)
			return
		}

		out, err := json.Marshal(adlink)
		if err != nil {
			http.Error(w, fmt.Sprintf("marshaling JSON: %s", err.Error()), http.StatusInternalServerError)
			return
		}
		w.Write(out)
	}), nil
}

// ensures the advert came from this node originally
func validateAdvertSig(sk crypto.PrivKey, ad schema.Advertisement) error {
	sigBytes := ad.Signature
	err := ad.Sign(sk)
	if err != nil {
		return fmt.Errorf("signing advert: %w", err)
	}
	if !bytes.Equal(sigBytes, ad.Signature) {
		return errors.New("advert was not created by this node")
	}
	return nil
}

// assumed in DAG-JSON encoding
func decodeAdvert(r io.Reader) (schema.Advertisement, error) {
	advBytes, err := io.ReadAll(r)
	if err != nil {
		return schema.Advertisement{}, err
	}

	adLink, err := cid.V1Builder{
		Codec:  cid.DagJSON,
		MhType: multihash.SHA2_256,
	}.Sum(advBytes)
	if err != nil {
		return schema.Advertisement{}, err
	}

	return schema.BytesToAdvertisement(adLink, advBytes)
}

func publishAdvert(ctx context.Context, sk crypto.PrivKey, store store.PublisherStore, ad schema.Advertisement) (ipld.Link, error) {
	prevHead, err := store.Head(ctx)
	if err != nil {
		return nil, err
	}

	ad.PreviousID = prevHead.Head

	// Sign the advertisement.
	if err = ad.Sign(sk); err != nil {
		return nil, fmt.Errorf("signing advert: %w", err)
	}

	if err := ad.Validate(); err != nil {
		return nil, fmt.Errorf("validating advert: %w", err)
	}

	link, err := store.PutAdvert(ctx, ad)
	if err != nil {
		return nil, fmt.Errorf("putting advert: %w", err)
	}

	head, err := head.NewSignedHead(link.(cidlink.Link).Cid, "/indexer/ingest/mainnet", sk)
	if err != nil {
		return nil, fmt.Errorf("signing head: %w", err)
	}
	if _, err := store.ReplaceHead(ctx, prevHead, head); err != nil {
		return nil, fmt.Errorf("replacing head: %w", err)
	}

	return link, nil
}
