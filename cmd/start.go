package cmd

import (
	crypto_ed25519 "crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	leveldb "github.com/ipfs/go-ds-leveldb"
	"github.com/ipni/go-libipni/maurl"
	"github.com/multiformats/go-multiaddr"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/principal"
	ed25519 "github.com/storacha/go-ucanto/principal/ed25519/signer"
	ucanserver "github.com/storacha/go-ucanto/server"
	"github.com/urfave/cli/v2"

	"github.com/storacha/piri/cmd/enum"
	"github.com/storacha/piri/pkg/database"
	"github.com/storacha/piri/pkg/database/sqlitedb"
	"github.com/storacha/piri/pkg/presets"
	"github.com/storacha/piri/pkg/principalresolver"
	"github.com/storacha/piri/pkg/server"
	"github.com/storacha/piri/pkg/service/storage"
	"github.com/storacha/piri/pkg/store/blobstore"
)

var StartCmd = &cli.Command{
	Name:  "start",
	Usage: "Start the piri node daemon.",
	Flags: []cli.Flag{
		KeyFileFlag,
		CurioURLFlag,
		&cli.IntFlag{
			Name:    "port",
			Aliases: []string{"p"},
			Value:   3000,
			Usage:   "Port to bind the server to.",
			EnvVars: []string{"PIRI_PORT"},
		},
		&cli.StringFlag{
			Name:    "data-dir",
			Aliases: []string{"d"},
			Usage:   "Root directory to store data in.",
			EnvVars: []string{"PIRI_DATA_DIR"},
		},
		&cli.StringFlag{
			Name:    "tmp-dir",
			Aliases: []string{"t"},
			Usage:   "Temporary directory data is uploaded to before being moved to data-dir.",
			EnvVars: []string{"PIRI_TMP_DIR"},
		},
		&cli.StringFlag{
			Name:    "public-url",
			Aliases: []string{"u"},
			Usage:   "URL the node is publically accessible at.",
			EnvVars: []string{"PIRI_PUBLIC_URL"},
		},
		ProofSetFlag,
		&cli.StringFlag{
			Name:    "indexing-service-proof",
			Usage:   "A delegation that allows the node to cache claims with the indexing service.",
			EnvVars: []string{"PIRI_INDEXING_SERVICE_PROOF"},
		},
	},
	Action: func(cCtx *cli.Context) error {
		id, err := PrincipalSignerFromFile(cCtx.String("key-file"))
		if err != nil {
			return err
		}

		dataDir := cCtx.String("data-dir")
		if dataDir == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("getting user home directory: %w", err)
			}

			dir, err := mkdirp(homeDir, ".storacha")
			if err != nil {
				return err
			}
			log.Errorf("Data directory is not configured, using default: %s", dir)
			dataDir = dir
		}

		tmpDir := cCtx.String("tmp-dir")
		if tmpDir == "" {
			dir, err := mkdirp(path.Join(os.TempDir(), "storage"))
			if err != nil {
				return err
			}
			log.Warnf("Tmp directory is not configured, using default: %s", dir)
			tmpDir = dir
		}

		blobStore, err := blobstore.NewFsBlobstore(path.Join(dataDir, "blobs"), path.Join(tmpDir, "blobs"))
		if err != nil {
			return fmt.Errorf("creating blob storage: %w", err)
		}

		allocsDir, err := mkdirp(dataDir, "allocation")
		if err != nil {
			return err
		}
		allocDs, err := leveldb.NewDatastore(allocsDir, nil)
		if err != nil {
			return err
		}
		claimsDir, err := mkdirp(dataDir, "claim")
		if err != nil {
			return err
		}
		claimDs, err := leveldb.NewDatastore(claimsDir, nil)
		if err != nil {
			return err
		}
		publisherDir, err := mkdirp(dataDir, "publisher")
		if err != nil {
			return err
		}
		publisherDs, err := leveldb.NewDatastore(publisherDir, nil)
		if err != nil {
			return err
		}
		receiptDir, err := mkdirp(dataDir, "receipt")
		if err != nil {
			return err
		}
		receiptDs, err := leveldb.NewDatastore(receiptDir, nil)
		if err != nil {
			return err
		}

		var pdpConfig *storage.PDPConfig
		var blobAddr multiaddr.Multiaddr
		curioURLStr := cCtx.String("curio-url")
		if curioURLStr != "" {
			curioURL, err := url.Parse(curioURLStr)
			if err != nil {
				return fmt.Errorf("parsing curio URL: %w", err)
			}
			if !cCtx.IsSet("pdp-proofset") {
				return errors.New("pdp-proofset must be set if curio is used")
			}
			proofSet := cCtx.Int64("pdp-proofset")
			/*
				.storacha/aggregator/
				├── datastore
				│ ├── 000001.log
				│ ├── CURRENT
				│ ├── LOCK
				│ ├── LOG
				│ └── MANIFEST-000000
				└── jobqueue
				    ├── jobqueue.db
				    ├── jobqueue.db-shm
				    └── jobqueue.db-wal
			*/
			aggRootDir, err := mkdirp(dataDir, "aggregator")
			if err != nil {
				return err
			}
			aggDsDir, err := mkdirp(aggRootDir, "datastore")
			if err != nil {
				return err
			}
			aggDs, err := leveldb.NewDatastore(aggDsDir, nil)
			if err != nil {
				return err
			}
			aggJobQueueDir, err := mkdirp(aggRootDir, "jobqueue")
			if err != nil {
				return err
			}
			pdpConfig = &storage.PDPConfig{
				PDPDatastore:  aggDs,
				CurioEndpoint: curioURL,
				ProofSet:      uint64(proofSet),
				DatabasePath:  filepath.Join(aggJobQueueDir, "jobqueue.db"),
			}
			curioAddr, err := maurl.FromURL(curioURL)
			if err != nil {
				return err
			}
			pieceAddr, err := multiaddr.NewMultiaddr("/http-path/" + url.PathEscape("piece/{blobCID}"))
			if err != nil {
				return err
			}
			blobAddr = multiaddr.Join(curioAddr, pieceAddr)
		}

		port := cCtx.Int("port")
		pubURLstr := cCtx.String("public-url")
		if pubURLstr == "" {
			pubURLstr = fmt.Sprintf("http://localhost:%d", port)
			log.Errorf("Public URL is not configured, using: %s", pubURLstr)
		}
		pubURL, err := url.Parse(pubURLstr)
		if err != nil {
			return fmt.Errorf("parsing public URL: %w", err)
		}

		var ipniAnnounceURLs []url.URL
		if os.Getenv("PIRI_IPNI_ANNOUNCE_URLS") != "" {
			var urls []string
			err := json.Unmarshal([]byte(os.Getenv("PIRI_IPNI_ANNOUNCE_URLS")), &urls)
			if err != nil {
				return fmt.Errorf("parsing IPNI announce URLs JSON: %w", err)
			}
			for _, s := range urls {
				url, err := url.Parse(s)
				if err != nil {
					return fmt.Errorf("parsing IPNI announce URL: %s: %w", s, err)
				}
				ipniAnnounceURLs = append(ipniAnnounceURLs, *url)
			}
		} else {
			ipniAnnounceURLs = presets.IPNIAnnounceURLs
		}

		indexingServiceDID := presets.IndexingServiceDID
		if os.Getenv("PIRI_INDEXING_SERVICE_DID") != "" {
			d, err := did.Parse(os.Getenv("PIRI_INDEXING_SERVICE_DID"))
			if err != nil {
				return fmt.Errorf("parsing indexing service DID: %w", err)
			}
			indexingServiceDID = d
		}

		indexingServiceURL := *presets.IndexingServiceURL
		if os.Getenv("PIRI_INDEXING_SERVICE_URL") != "" {
			u, err := url.Parse(os.Getenv("PIRI_INDEXING_SERVICE_URL"))
			if err != nil {
				return fmt.Errorf("parsing indexing service URL: %w", err)
			}
			indexingServiceURL = *u
		}

		uploadServiceDID := presets.UploadServiceDID
		if os.Getenv("PIRI_UPLOAD_SERVICE_DID") != "" {
			d, err := did.Parse(os.Getenv("PIRI_UPLOAD_SERVICE_DID"))
			if err != nil {
				return fmt.Errorf("parsing indexing service DID: %w", err)
			}
			uploadServiceDID = d
		}

		uploadServiceURL := *presets.UploadServiceURL
		if os.Getenv("PIRI_UPLOAD_SERVICE_URL") != "" {
			u, err := url.Parse(os.Getenv("PIRI_UPLOAD_SERVICE_URL"))
			if err != nil {
				return fmt.Errorf("parsing indexing service URL: %w", err)
			}
			uploadServiceURL = *u
		}

		var indexingServiceProofs delegation.Proofs
		if cCtx.String("indexing-service-proof") != "" {
			dlg, err := delegation.Parse(cCtx.String("indexing-service-proof"))
			if err != nil {
				return fmt.Errorf("parsing indexing service proof: %w", err)
			}
			indexingServiceProofs = append(indexingServiceProofs, delegation.FromDelegation(dlg))
		}

		replicatorDir, err := mkdirp(dataDir, "replicator")
		if err != nil {
			return err
		}

		db, err := sqlitedb.New(filepath.Join(replicatorDir, "replicator.db"),
			database.WithJournalMode("WAL"),
			database.WithTimeout(5*time.Second),
			database.WithSyncMode(database.SyncModeNORMAL),
		)
		if err != nil {
			return fmt.Errorf("creating jobqueue database: %w", err)
		}

		opts := []storage.Option{
			storage.WithIdentity(id),
			storage.WithBlobstore(blobStore),
			storage.WithAllocationDatastore(allocDs),
			storage.WithClaimDatastore(claimDs),
			storage.WithPublisherDatastore(publisherDs),
			storage.WithPublicURL(*pubURL),
			storage.WithPublisherDirectAnnounce(ipniAnnounceURLs...),
			storage.WithUploadServiceConfig(uploadServiceDID, uploadServiceURL),
			storage.WithPublisherIndexingServiceConfig(indexingServiceDID, indexingServiceURL),
			storage.WithPublisherIndexingServiceProof(indexingServiceProofs...),
			storage.WithReceiptDatastore(receiptDs),
			storage.WithReplicatorDB(db),
		}
		if pdpConfig != nil {
			opts = append(opts, storage.WithPDPConfig(*pdpConfig))
		}
		if blobAddr != nil {
			opts = append(opts, storage.WithPublisherBlobAddress(blobAddr))
		}
		svc, err := storage.New(opts...)
		if err != nil {
			return fmt.Errorf("creating service instance: %w", err)
		}
		err = svc.Startup(cCtx.Context)
		if err != nil {
			return fmt.Errorf("starting service: %w", err)
		}

		defer svc.Close(cCtx.Context)

		principalMapping := presets.PrincipalMapping
		if os.Getenv("PIRI_PRINCIPAL_MAPPING") != "" {
			var pm map[string]string
			err := json.Unmarshal([]byte(os.Getenv("PIRI_PRINCIPAL_MAPPING")), &pm)
			if err != nil {
				return fmt.Errorf("parsing principal mapping: %w", err)
			}
			principalMapping = pm
		}
		presolv, err := principalresolver.New(principalMapping)
		if err != nil {
			return fmt.Errorf("creating principal resolver: %w", err)
		}

		go func() {
			time.Sleep(time.Millisecond * 50)
			if err == nil {
				PrintHero(id.DID())
			}
		}()

		err = server.ListenAndServe(
			fmt.Sprintf(":%d", cCtx.Int("port")),
			svc,
			ucanserver.WithPrincipalResolver(presolv.ResolveDIDKey),
		)
		return err
	},
}

func PrincipalSignerFromFile(path string) (principal.Signer, error) {
	// open the file
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	// ensure its either json or pem
	baseName := strings.Trim(filepath.Ext(path), ".")
	typ := enum.ParseKeyFormat(strings.ToUpper(baseName))
	if !typ.IsValid() {
		return nil, fmt.Errorf("unsupported file type: %s, expected .json or .pem", baseName)
	}

	// read accordingly
	switch typ {
	case enum.KeyFormats.PEM:
		return readPrivateKeyFromPEM(f)
	case enum.KeyFormats.JSON:
		return readPrivateKeyFromJSON(f)
	}

	return nil, fmt.Errorf("unsupported file type: %s, expected .json or .pem", baseName)
}

func readPrivateKeyFromJSON(f io.Reader) (principal.Signer, error) {
	jsonData, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("reading private key file: %w", err)
	}

	var key struct {
		DID string `json:"did"`
		Key string `json:"key"`
	}
	if err := json.Unmarshal(jsonData, &key); err != nil {
		return nil, fmt.Errorf("unmarshaling private key file to json: %w", err)
	}

	return ed25519.Parse(key.Key)
}

func readPrivateKeyFromPEM(f io.Reader) (principal.Signer, error) {
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
