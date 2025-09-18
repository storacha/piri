package serve

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	leveldb "github.com/ipfs/go-ds-leveldb"
	"github.com/ipni/go-libipni/maurl"
	"github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/did"
	ucanserver "github.com/storacha/go-ucanto/server"
	ucanretrieval "github.com/storacha/go-ucanto/server/retrieval"

	"github.com/storacha/piri/cmd/cliutil"
	"github.com/storacha/piri/lib"
	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/presets"
	"github.com/storacha/piri/pkg/principalresolver"
	"github.com/storacha/piri/pkg/server"
	"github.com/storacha/piri/pkg/service/retrieval"
	"github.com/storacha/piri/pkg/service/storage"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/storacha/piri/pkg/telemetry"
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
		"pdp-server-url",
		"",
		"URL used to connect to pdp server",
	)
	cobra.CheckErr(viper.BindPFlag("pdp_server_url", UCANCmd.Flags().Lookup("pdp-server-url")))

	UCANCmd.Flags().Uint64(
		"proof-set",
		0,
		"Proofset to use with PDP",
	)
	cobra.CheckErr(viper.BindPFlag("ucan.proof_set", UCANCmd.Flags().Lookup("proof-set")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.proof_set", "PIRI_PROOF_SET"))

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
		presets.IndexingServiceDID.String(),
		"DID of the indexing service",
	)
	cobra.CheckErr(UCANCmd.Flags().MarkHidden("indexing-service-did"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.indexer.did", UCANCmd.Flags().Lookup("indexing-service-did")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.services.indexer.did", "PIRI_INDEXING_SERVICE_DID"))

	UCANCmd.Flags().String(
		"indexing-service-url",
		presets.IndexingServiceURL.String(),
		"URL of the indexing service",
	)
	cobra.CheckErr(UCANCmd.Flags().MarkHidden("indexing-service-url"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.indexer.url", UCANCmd.Flags().Lookup("indexing-service-url")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.services.indexer.url", "PIRI_INDEXING_SERVICE_URL"))

	UCANCmd.Flags().String(
		"upload-service-did",
		presets.UploadServiceDID.String(),
		"DID of the upload service",
	)
	cobra.CheckErr(UCANCmd.Flags().MarkHidden("upload-service-did"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.upload.did", UCANCmd.Flags().Lookup("upload-service-did")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.services.upload.did", "PIRI_UPLOAD_SERVICE_DID"))

	UCANCmd.Flags().String(
		"upload-service-url",
		presets.UploadServiceURL.String(),
		"URL of the upload service",
	)
	cobra.CheckErr(UCANCmd.Flags().MarkHidden("upload-service-url"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.upload.url", UCANCmd.Flags().Lookup("upload-service-url")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.services.upload.did", "PIRI_UPLOAD_SERVICE_URL"))

	UCANCmd.Flags().StringSlice(
		"ipni-announce-urls",
		func() []string {
			out := make([]string, 0)
			for _, u := range presets.IPNIAnnounceURLs {
				out = append(out, u.String())
			}
			return out
		}(),
		"A list of IPNI announce URLs")
	cobra.CheckErr(UCANCmd.Flags().MarkHidden("ipni-announce-urls"))
	cobra.CheckErr(viper.BindPFlag("ucan.services.publisher.ipni_announce_urls", UCANCmd.Flags().Lookup("ipni-announce-urls")))
	// backwards compatibility
	cobra.CheckErr(viper.BindEnv("ucan.services.publisher.ipni_announce_urls", "PIRI_IPNI_ANNOUNCE_URLS"))

	UCANCmd.Flags().StringToString(
		"service-principal-mapping",
		presets.PrincipalMapping,
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

	if cfg.PDPServerURL != "" && cfg.UCANService.ProofSetID == 0 {
		return fmt.Errorf("must set --proof-set when using --pdp-server-url")
	}
	if cfg.UCANService.ProofSetID != 0 && cfg.PDPServerURL == "" {
		return fmt.Errorf("must set --pdp-server-url when using --proofset")
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

	var pdpConfig *pdp.Config
	var blobAddr multiaddr.Multiaddr
	if pdpServerURL := cfg.PDPServerURL; pdpServerURL != "" {
		pdpServerURL, err := url.Parse(pdpServerURL)
		if err != nil {
			return fmt.Errorf("parsing pdp server URL: %w", err)
		}
		aggRootDir, err := cliutil.Mkdirp(cfg.Repo.DataDir, "aggregator")
		if err != nil {
			return err
		}
		aggDsDir, err := cliutil.Mkdirp(aggRootDir, "datastore")
		if err != nil {
			return err
		}
		aggDs, err := leveldb.NewDatastore(aggDsDir, nil)
		if err != nil {
			return err
		}
		aggJobQueueDir, err := cliutil.Mkdirp(aggRootDir, "jobqueue")
		if err != nil {
			return err
		}
		pdpConfig = &pdp.Config{
			PDPDatastore: aggDs,
			PDPServerURL: pdpServerURL,
			ProofSet:     cfg.UCANService.ProofSetID,
			DatabasePath: filepath.Join(aggJobQueueDir, "jobqueue.db"),
		}
		pdpServerAddr, err := maurl.FromURL(pdpServerURL)
		if err != nil {
			return fmt.Errorf("parsing pdp server url: %w", err)
		}
		blobAddr, err = lib.JoinHTTPPath(pdpServerAddr, "piece/{blobCID}")
		if err != nil {
			return fmt.Errorf("joining blob path to PDP multiaddr: %w", err)
		}
	}

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

	storageOpts = append(storageOpts,
		storage.WithIdentity(id),
		storage.WithBlobstore(blobStore),
		storage.WithAllocationDatastore(allocDs),
		storage.WithClaimDatastore(claimDs),
		storage.WithPublisherDatastore(publisherDs),
		storage.WithPublicURL(*pubURL),
		storage.WithPublisherDirectAnnounce(ipniAnnounceURLs...),
		storage.WithUploadServiceConfig(uploadServiceDID, *uploadServiceURL),
		storage.WithPublisherIndexingServiceConfig(indexingServiceDID, *indexingServiceURL),
		storage.WithReceiptDatastore(receiptDs),
	)

	if pdpConfig != nil {
		storageOpts = append(storageOpts, storage.WithPDPConfig(*pdpConfig))
	}
	if blobAddr != nil {
		storageOpts = append(storageOpts, storage.WithPublisherBlobAddress(blobAddr))
	}
	storageSvc, err := storage.New(storageOpts...)
	if err != nil {
		return fmt.Errorf("creating storage service instance: %w", err)
	}
	err = storageSvc.Startup(ctx)
	if err != nil {
		return fmt.Errorf("starting storage service: %w", err)
	}

	if pdpConfig != nil {
		// TODO: blobstore that proxies to pdpConfig.PDPServerURL
	}
	retrievalSvc := retrieval.New(id, blobStore, storageSvc.Blobs().Allocations())

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
			PDPEnabled:           storageSvc.PDP() != nil,
		}
		if storageSvc.PDP() != nil {
			serverConfig.PDPServerURL = pdpConfig.PDPServerURL
			serverConfig.ProofSetID = pdpConfig.ProofSet
		}
		cliutil.PrintUCANServerConfig(cmd, serverConfig)
		cliutil.PrintHero(cmd.OutOrStdout(), id.DID())
	}()

	defer storageSvc.Close(ctx)

	presolv, err := principalresolver.NewHTTPResolver([]did.DID{indexingServiceDID, uploadServiceDID})
	if err != nil {
		return fmt.Errorf("creating http principal resolver: %w", err)
	}
	cachedpresolv, err := principalresolver.NewCachedResolver(presolv, 24*time.Hour)
	if err != nil {
		return fmt.Errorf("creating cached principal resolver: %w", err)
	}

	telemetry.RecordServerInfo(ctx, "ucan",
		telemetry.StringAttr("did", id.DID().String()),
		telemetry.StringAttr("indexing_did", indexingServiceDID.String()),
		telemetry.StringAttr("indexing_url", indexingServiceURL.String()),
		telemetry.StringAttr("upload_did", uploadServiceDID.String()),
		telemetry.StringAttr("upload_url", uploadServiceURL.String()),
		telemetry.Int64Attr("proof_set", int64(cfg.UCANService.ProofSetID)),
	)

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
