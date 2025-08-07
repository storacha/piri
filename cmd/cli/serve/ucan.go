package serve

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	leveldb "github.com/ipfs/go-ds-leveldb"
	"github.com/ipni/go-libipni/maurl"
	"github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/did"
	ucanserver "github.com/storacha/go-ucanto/server"

	"github.com/storacha/piri/cmd/cliutil"
	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/principalresolver"
	"github.com/storacha/piri/pkg/server"
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
		"host",
		config.DefaultUCANServer.Host,
		"Host to listen on")
	cobra.CheckErr(viper.BindPFlag("host", UCANCmd.Flags().Lookup("host")))

	UCANCmd.Flags().Uint(
		"port",
		config.DefaultUCANServer.Port,
		"Port to listen on",
	)
	cobra.CheckErr(viper.BindPFlag("port", UCANCmd.Flags().Lookup("port")))

	UCANCmd.Flags().String(
		"public-url",
		config.DefaultUCANServer.PublicURL,
		"URL the node is publicly accessible at and exposed to other storacha services",
	)
	cobra.CheckErr(viper.BindPFlag("public_url", UCANCmd.Flags().Lookup("public-url")))

	UCANCmd.Flags().String(
		"pdp-server-url",
		config.DefaultUCANServer.PDPServerURL,
		"URL used to connect to pdp server",
	)
	cobra.CheckErr(viper.BindPFlag("pdp_server_url", UCANCmd.Flags().Lookup("pdp-server-url")))

	UCANCmd.Flags().Uint64(
		"proof-set",
		config.DefaultUCANServer.ProofSet,
		"Proofset to use with PDP",
	)
	cobra.CheckErr(viper.BindPFlag("proof_set", UCANCmd.Flags().Lookup("proof-set")))

	UCANCmd.Flags().String(
		"indexing-service-proof",
		config.DefaultUCANServer.IndexingServiceProof,
		"A delegation that allows the node to cache claims with the indexing service",
	)
	cobra.CheckErr(viper.BindPFlag("indexing_service_proof", UCANCmd.Flags().Lookup("indexing-service-proof")))

	UCANCmd.Flags().String(
		"indexing-service-did",
		config.DefaultUCANServer.IndexingServiceDID,
		"DID of the indexing service",
	)
	cobra.CheckErr(UCANCmd.Flags().MarkHidden("indexing-service-did"))
	cobra.CheckErr(viper.BindPFlag("indexing_service_did", UCANCmd.Flags().Lookup("indexing-service-did")))

	UCANCmd.Flags().String(
		"indexing-service-url",
		config.DefaultUCANServer.IndexingServiceURL,
		"URL of the indexing service",
	)
	cobra.CheckErr(UCANCmd.Flags().MarkHidden("indexing-service-url"))
	cobra.CheckErr(viper.BindPFlag("indexing_service_url", UCANCmd.Flags().Lookup("indexing-service-url")))

	UCANCmd.Flags().String(
		"upload-service-did",
		config.DefaultUCANServer.UploadServiceDID,
		"DID of the upload service",
	)
	cobra.CheckErr(UCANCmd.Flags().MarkHidden("upload-service-did"))
	cobra.CheckErr(viper.BindPFlag("upload_service_did", UCANCmd.Flags().Lookup("upload-service-did")))

	UCANCmd.Flags().String(
		"upload-service-url",
		config.DefaultUCANServer.UploadServiceURL,
		"URL of the upload service",
	)
	cobra.CheckErr(UCANCmd.Flags().MarkHidden("upload-service-url"))
	cobra.CheckErr(viper.BindPFlag("upload_service_url", UCANCmd.Flags().Lookup("upload-service-url")))

	UCANCmd.Flags().StringSlice(
		"ipni-announce-urls",
		config.DefaultUCANServer.IPNIAnnounceURLs,
		"A list of IPNI announce URLs")
	cobra.CheckErr(UCANCmd.Flags().MarkHidden("ipni-announce-urls"))
	cobra.CheckErr(viper.BindPFlag("ipni_announce_urls", UCANCmd.Flags().Lookup("ipni-announce-urls")))

	UCANCmd.Flags().StringToString(
		"service-principal-mapping",
		config.DefaultUCANServer.ServicePrincipalMapping,
		"Mapping of service DIDs to principal DIDs",
	)
	cobra.CheckErr(UCANCmd.Flags().MarkHidden("service-principal-mapping"))
	cobra.CheckErr(viper.BindPFlag("service_principal_mapping", UCANCmd.Flags().Lookup("service-principal-mapping")))

}

func startServer(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	cfg, err := config.Load[config.UCANServer]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if cfg.PDPServerURL != "" && cfg.ProofSet == 0 {
		return fmt.Errorf("must set --proof-set when using --pdp-server-url")
	}
	if cfg.ProofSet != 0 && cfg.PDPServerURL == "" {
		return fmt.Errorf("must set --pdp-server-url when using --proofset")
	}

	id, err := cliutil.ReadPrivateKeyFromPEM(cfg.KeyFile)
	if err != nil {
		return fmt.Errorf("loading principal signer: %w", err)
	}

	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return fmt.Errorf("creating directory: %s: %w", cfg.DataDir, err)
	}
	if err := os.MkdirAll(cfg.TempDir, 0755); err != nil {
		return fmt.Errorf("creating directory: %s: %w", cfg.TempDir, err)
	}
	blobStore, err := blobstore.NewFsBlobstore(
		filepath.Join(cfg.DataDir, "blobs"),
		filepath.Join(cfg.TempDir, "blobs"),
	)
	if err != nil {
		return fmt.Errorf("creating blob storage: %w", err)
	}

	allocsDir, err := cliutil.Mkdirp(cfg.DataDir, "allocation")
	if err != nil {
		return err
	}
	allocDs, err := leveldb.NewDatastore(allocsDir, nil)
	if err != nil {
		return err
	}
	claimsDir, err := cliutil.Mkdirp(cfg.DataDir, "claim")
	if err != nil {
		return err
	}
	claimDs, err := leveldb.NewDatastore(claimsDir, nil)
	if err != nil {
		return err
	}
	publisherDir, err := cliutil.Mkdirp(cfg.DataDir, "publisher")
	if err != nil {
		return err
	}
	publisherDs, err := leveldb.NewDatastore(publisherDir, nil)
	if err != nil {
		return err
	}
	receiptDir, err := cliutil.Mkdirp(cfg.DataDir, "receipt")
	if err != nil {
		return err
	}
	receiptDs, err := leveldb.NewDatastore(receiptDir, nil)
	if err != nil {
		return err
	}

	var pdpConfig *storage.PDPConfig
	var blobAddr multiaddr.Multiaddr
	if pdpServerURL := cfg.PDPServerURL; pdpServerURL != "" {
		pdpServerURL, err := url.Parse(pdpServerURL)
		if err != nil {
			return fmt.Errorf("parsing curio URL: %w", err)
		}
		aggRootDir, err := cliutil.Mkdirp(cfg.DataDir, "aggregator")
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
		pdpConfig = &storage.PDPConfig{
			PDPDatastore: aggDs,
			PDPServerURL: pdpServerURL,
			ProofSet:     cfg.ProofSet,
			DatabasePath: filepath.Join(aggJobQueueDir, "jobqueue.db"),
		}
		curioAddr, err := maurl.FromURL(pdpServerURL)
		if err != nil {
			return fmt.Errorf("parsing pdp server url: %w", err)
		}
		pieceAddr, err := multiaddr.NewMultiaddr("/http-path/" + url.PathEscape("piece/{blobCID}"))
		if err != nil {
			return err
		}
		blobAddr = multiaddr.Join(curioAddr, pieceAddr)
	}

	var ipniAnnounceURLs []url.URL
	for _, s := range cfg.IPNIAnnounceURLs {
		url, err := url.Parse(s)
		if err != nil {
			return fmt.Errorf("parsing IPNI announce URL: %s: %w", s, err)
		}
		ipniAnnounceURLs = append(ipniAnnounceURLs, *url)
	}

	uploadServiceDID, err := did.Parse(cfg.UploadServiceDID)
	if err != nil {
		return fmt.Errorf("parsing upload service DID: %w", err)
	}

	uploadServiceURL, err := url.Parse(cfg.UploadServiceURL)
	if err != nil {
		return fmt.Errorf("parsing upload service URL: %w", err)
	}

	indexingServiceDID, err := did.Parse(cfg.IndexingServiceDID)
	if err != nil {
		return fmt.Errorf("parsing indexing service DID: %w", err)
	}

	indexingServiceURL, err := url.Parse(cfg.IndexingServiceURL)
	if err != nil {
		return fmt.Errorf("parsing indexing service URL: %w", err)
	}

	var indexingServiceProof delegation.Proof
	if cfg.IndexingServiceProof != "" {
		dlg, err := delegation.Parse(cfg.IndexingServiceProof)
		if err != nil {
			return fmt.Errorf("parsing indexing service proof: %w", err)
		}
		indexingServiceProof = delegation.FromDelegation(dlg)
	}

	var pubURL *url.URL
	if cfg.PublicURL == "" {
		pubURL, err = url.Parse(fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port))
		if err != nil {
			return fmt.Errorf("DEVELOPER ERROR parsing public URL: %w", err)
		}
		log.Warnf("no public URL configured, using %s", pubURL)
	} else {
		pubURL, err = url.Parse(strings.TrimSuffix(cfg.PublicURL, "/"))
		if err != nil {
			return fmt.Errorf("parsing server public url: %w", err)
		}
	}

	opts := []storage.Option{
		storage.WithIdentity(id),
		storage.WithBlobstore(blobStore),
		storage.WithAllocationDatastore(allocDs),
		storage.WithClaimDatastore(claimDs),
		storage.WithPublisherDatastore(publisherDs),
		storage.WithPublicURL(*pubURL),
		storage.WithPublisherDirectAnnounce(ipniAnnounceURLs...),
		storage.WithUploadServiceConfig(uploadServiceDID, *uploadServiceURL),
		storage.WithPublisherIndexingServiceConfig(indexingServiceDID, *indexingServiceURL),
		storage.WithPublisherIndexingServiceProof(indexingServiceProof),
		storage.WithReceiptDatastore(receiptDs),
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
	err = svc.Startup(ctx)
	if err != nil {
		return fmt.Errorf("starting service: %w", err)
	}

	defer svc.Close(ctx)

	presolv, err := principalresolver.NewHTTPResolver([]did.DID{indexingServiceDID, uploadServiceDID})
	if err != nil {
		return fmt.Errorf("creating http principal resolver: %w", err)
	}
	cachedpresolv, err := principalresolver.NewCachedResolver(presolv, 24*time.Hour)
	if err != nil {
		return fmt.Errorf("creating cached principal resolver: %w", err)
	}

	go func() {
		time.Sleep(time.Millisecond * 50)
		if err == nil {
			cliutil.PrintHero(id.DID())
		}
	}()

	telemetry.RecordServerInfo(ctx, "ucan",
		telemetry.StringAttr("did", id.DID().String()),
		telemetry.StringAttr("indexing_did", cfg.IndexingServiceDID),
		telemetry.StringAttr("indexing_url", cfg.IndexingServiceURL),
		telemetry.StringAttr("upload_did", cfg.UploadServiceDID),
		telemetry.StringAttr("upload_url", cfg.UploadServiceURL),
		telemetry.Int64Attr("proof_set", int64(cfg.ProofSet)),
	)

	err = server.ListenAndServe(
		fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		svc,
		ucanserver.WithPrincipalResolver(cachedpresolv.ResolveDIDKey),
	)
	return err

}
