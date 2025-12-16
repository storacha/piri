package serve

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	leveldb "github.com/ipfs/go-ds-leveldb"
	"github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	uclient "github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/did"
	edverifier "github.com/storacha/go-ucanto/principal/ed25519/verifier"
	ucanserver "github.com/storacha/go-ucanto/server"
	ucanretrieval "github.com/storacha/go-ucanto/server/retrieval"
	ucanhttp "github.com/storacha/go-ucanto/transport/http"
	"github.com/storacha/go-ucanto/validator"

	"github.com/storacha/piri/cmd/cliutil"
	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/principalresolver"
	"github.com/storacha/piri/pkg/server"
	"github.com/storacha/piri/pkg/service/retrieval"
	"github.com/storacha/piri/pkg/service/storage"
	"github.com/storacha/piri/pkg/store/blobstore"
)

var (
	UCANCmd = &cobra.Command{
		Use:   "ucan",
		Short: "Start the UCAN server.",
		Args:  cobra.NoArgs,
		RunE:  startServer,
	}
)

func init() {
	UCANCmd.Flags().String(
		"indexing-service-proof",
		"",
		"A delegation that allows the node to cache claims with the indexing service",
	)
	cobra.CheckErr(viper.BindPFlag("ucan.services.indexer.proof", UCANCmd.Flags().Lookup("indexing-service-proof")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.services.indexer.proof", "PIRI_INDEXING_SERVICE_PROOF"))

	UCANCmd.Flags().String(
		"indexing-service-did",
		"",
		"DID of the indexing service",
	)
	cobra.CheckErr(UCANCmd.Flags().MarkHidden("indexing-service-did"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.indexer.did", UCANCmd.Flags().Lookup("indexing-service-did")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.services.indexer.did", "PIRI_INDEXING_SERVICE_DID"))

	UCANCmd.Flags().String(
		"indexing-service-url",
		"",
		"URL of the indexing service",
	)
	cobra.CheckErr(UCANCmd.Flags().MarkHidden("indexing-service-url"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.indexer.url", UCANCmd.Flags().Lookup("indexing-service-url")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.services.indexer.url", "PIRI_INDEXING_SERVICE_URL"))

	UCANCmd.Flags().String(
		"upload-service-did",
		"",
		"DID of the upload service",
	)
	cobra.CheckErr(UCANCmd.Flags().MarkHidden("upload-service-did"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.upload.did", UCANCmd.Flags().Lookup("upload-service-did")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.services.upload.did", "PIRI_UPLOAD_SERVICE_DID"))

	UCANCmd.Flags().String(
		"upload-service-url",
		"",
		"URL of the upload service",
	)
	cobra.CheckErr(UCANCmd.Flags().MarkHidden("upload-service-url"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.upload.url", UCANCmd.Flags().Lookup("upload-service-url")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.services.upload.did", "PIRI_UPLOAD_SERVICE_URL"))

	UCANCmd.Flags().StringSlice(
		"ipni-announce-urls",
		[]string{},
		"A list of IPNI announce URLs")
	cobra.CheckErr(UCANCmd.Flags().MarkHidden("ipni-announce-urls"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.publisher.ipni_announce_urls", UCANCmd.Flags().Lookup("ipni-announce-urls")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.services.publisher.ipni_announce_urls", "PIRI_IPNI_ANNOUNCE_URLS"))

	UCANCmd.Flags().StringToString(
		"service-principal-mapping",
		map[string]string{},
		"Mapping of service DIDs to principal DIDs",
	)
	cobra.CheckErr(UCANCmd.Flags().MarkHidden("service-principal-mapping"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.principal_mapping", UCANCmd.Flags().Lookup("service-principal-mapping")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.services.principal_mapping", "PIRI_SERVICE_PRINCIPAL_MAPPING"))

}

func startServer(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	cfg, err := config.Load[config.UCANServerConfig]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	id, err := cliutil.ReadPrivateKeyFromPEM(cfg.Identity.KeyFile)
	if err != nil {
		return fmt.Errorf("loading principal signer: %w", err)
	}

	if err := os.MkdirAll(cfg.Repo.DataDir, 0755); err != nil {
		return fmt.Errorf("creating directory: %s: %w", cfg.Repo.DataDir, err)
	}
	if err := os.MkdirAll(cfg.Repo.TempDir, 0755); err != nil {
		return fmt.Errorf("creating directory: %s: %w", cfg.Repo.TempDir, err)
	}
	blobStore, err := blobstore.NewFsBlobstore(
		filepath.Join(cfg.Repo.DataDir, "blobs"),
		filepath.Join(cfg.Repo.TempDir, "blobs"),
	)
	if err != nil {
		return fmt.Errorf("creating blob storage: %w", err)
	}

	allocsDir, err := cliutil.Mkdirp(cfg.Repo.DataDir, "allocation")
	if err != nil {
		return err
	}
	allocDs, err := leveldb.NewDatastore(allocsDir, nil)
	if err != nil {
		return err
	}
	acceptDir, err := cliutil.Mkdirp(cfg.Repo.DataDir, "acceptance")
	if err != nil {
		return err
	}
	acceptDs, err := leveldb.NewDatastore(acceptDir, nil)
	if err != nil {
		return err
	}
	claimsDir, err := cliutil.Mkdirp(cfg.Repo.DataDir, "claim")
	if err != nil {
		return err
	}
	claimDs, err := leveldb.NewDatastore(claimsDir, nil)
	if err != nil {
		return err
	}
	publisherDir, err := cliutil.Mkdirp(cfg.Repo.DataDir, "publisher")
	if err != nil {
		return err
	}
	publisherDs, err := leveldb.NewDatastore(publisherDir, nil)
	if err != nil {
		return err
	}
	receiptDir, err := cliutil.Mkdirp(cfg.Repo.DataDir, "receipt")
	if err != nil {
		return err
	}
	receiptDs, err := leveldb.NewDatastore(receiptDir, nil)
	if err != nil {
		return err
	}

	var blobAddr multiaddr.Multiaddr

	var ipniAnnounceURLs []url.URL
	for _, s := range cfg.UCANService.Services.Publisher.AnnounceURLs {
		url, err := url.Parse(s)
		if err != nil {
			return fmt.Errorf("parsing IPNI announce URL: %s: %w", s, err)
		}
		ipniAnnounceURLs = append(ipniAnnounceURLs, *url)
	}

	uploadServiceDID, err := did.Parse(cfg.UCANService.Services.Upload.DID)
	if err != nil {
		return fmt.Errorf("parsing upload service DID: %w", err)
	}

	uploadServiceURL, err := url.Parse(cfg.UCANService.Services.Upload.URL)
	if err != nil {
		return fmt.Errorf("parsing upload service URL: %w", err)
	}

	uploadServiceConn, err := uclient.NewConnection(uploadServiceDID, ucanhttp.NewChannel(uploadServiceURL))
	if err != nil {
		return fmt.Errorf("creating upload service connection: %w", err)
	}

	indexingServiceDID, err := did.Parse(cfg.UCANService.Services.Indexer.DID)
	if err != nil {
		return fmt.Errorf("parsing indexing service DID: %w", err)
	}

	indexingServiceURL, err := url.Parse(cfg.UCANService.Services.Indexer.URL)
	if err != nil {
		return fmt.Errorf("parsing indexing service URL: %w", err)
	}

	var storageOpts []storage.Option
	var indexingServiceProof delegation.Proof
	if cfg.UCANService.Services.Indexer.Proof != "" {
		dlg, err := delegation.Parse(cfg.UCANService.Services.Indexer.Proof)
		if err != nil {
			return fmt.Errorf("parsing indexing service proof: %w", err)
		}
		indexingServiceProof = delegation.FromDelegation(dlg)
		storageOpts = append(storageOpts, storage.WithPublisherIndexingServiceProof(indexingServiceProof))
	}

	var pubURL *url.URL
	if cfg.Server.PublicURL == "" {
		pubURL, err = url.Parse(fmt.Sprintf("http://%s:%d", cfg.Server.Host, cfg.Server.Port))
		if err != nil {
			return fmt.Errorf("DEVELOPER ERROR parsing public URL: %w", err)
		}
		log.Warnf("no public URL configured, using %s", pubURL)
	} else {
		pubURL, err = url.Parse(cfg.Server.PublicURL)
		if err != nil {
			return fmt.Errorf("parsing server public url: %w", err)
		}
	}

	presolv, err := principalresolver.NewHTTPResolver([]did.DID{indexingServiceDID, uploadServiceDID})
	if err != nil {
		return fmt.Errorf("creating http principal resolver: %w", err)
	}
	cachedpresolv, err := principalresolver.NewCachedResolver(presolv, 24*time.Hour)
	if err != nil {
		return fmt.Errorf("creating cached principal resolver: %w", err)
	}

	claimValidationCtx := validator.NewClaimContext(
		id.Verifier(),
		validator.IsSelfIssued,
		func(context.Context, validator.Authorization[any]) validator.Revoked {
			return nil
		},
		validator.ProofUnavailable,
		edverifier.Parse,
		cachedpresolv.ResolveDIDKey,
		validator.NotExpiredNotTooEarly,
	)

	storageOpts = append(storageOpts,
		storage.WithIdentity(id),
		storage.WithBlobstore(blobStore),
		storage.WithAllocationDatastore(allocDs),
		storage.WithAcceptanceDatastore(acceptDs),
		storage.WithClaimDatastore(claimDs),
		storage.WithPublisherDatastore(publisherDs),
		storage.WithPublicURL(*pubURL),
		storage.WithPublisherDirectAnnounce(ipniAnnounceURLs...),
		storage.WithPublisherIndexingServiceConfig(indexingServiceDID, *indexingServiceURL),
		storage.WithReceiptDatastore(receiptDs),
		storage.WithClaimValidationContext(claimValidationCtx),
	)

	if blobAddr != nil {
		storageOpts = append(storageOpts, storage.WithPublisherBlobAddress(blobAddr))
	}
	storageSvc, err := storage.New(uploadServiceConn, storageOpts...)
	if err != nil {
		return fmt.Errorf("creating storage service instance: %w", err)
	}
	err = storageSvc.Startup(ctx)
	if err != nil {
		return fmt.Errorf("starting storage service: %w", err)
	}

	blobGetter := blobstore.BlobGetter(blobStore)
	retrievalSvc := retrieval.New(id, blobGetter, storageSvc.Blobs().Allocations())

	go func() {
		serverConfig := cliutil.UCANServerConfig{
			Host:                 cfg.Server.Host,
			Port:                 cfg.Server.Port,
			DataDir:              cfg.Repo.DataDir,
			PublicURL:            pubURL,
			BlobAddr:             blobAddr,
			IndexingServiceDID:   indexingServiceDID,
			IndexingServiceURL:   indexingServiceURL,
			IndexingServiceProof: indexingServiceProof,
			UploadServiceDID:     uploadServiceDID,
			UploadServiceURL:     uploadServiceURL,
			IPNIAnnounceURLs:     ipniAnnounceURLs,
		}
		cliutil.PrintUCANServerConfig(cmd, serverConfig)
		cliutil.PrintHero(cmd.OutOrStdout(), id.DID())
	}()

	defer storageSvc.Close(ctx)

	errHandler := func(err ucanserver.HandlerExecutionError[any]) {
		l := log.With("error", err.Error())
		if s := err.Stack(); s != "" {
			l.With("stack", s)
		}
		l.Error("ucan handler execution error")
	}

	err = server.ListenAndServe(
		fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		storageSvc,
		retrievalSvc,
		server.WithUCANServerOptions(
			ucanserver.WithPrincipalResolver(cachedpresolv.ResolveDIDKey),
			ucanserver.WithErrorHandler(errHandler),
		),
		server.WithUCANRetrievalServerOptions(
			ucanretrieval.WithPrincipalResolver(cachedpresolv.ResolveDIDKey),
			ucanretrieval.WithErrorHandler(errHandler),
		),
	)
	return err

}
