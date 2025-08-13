package flags

import (
	"os"
	"path/filepath"

	"github.com/samber/lo"
	"github.com/spf13/pflag"

	"github.com/storacha/piri/pkg/presets"
)

func SetupCoreFlags(fs *pflag.FlagSet) error {
	fs.String(
		"data-dir",
		filepath.Join(lo.Must(os.UserHomeDir()), ".storacha"),
		"Storage service data directory",
	)
	fs.String(
		"temp-dir",
		filepath.Join(os.TempDir(), "storage"),
		"Storage service temp directory",
	)
	fs.String(
		"key-file",
		"",
		"Path to a PEM file containing ed25519 private key",
	)

	bindings := []FlagBinding{
		{"data-dir", "repo.data_dir", "PIRI_DATA_DIR"},
		{"temp-dir", "repo.temp_dir", "PIRI_TEMP_DIR"},
		{"key-file", "identity.key_file", "PIRI_KEY_FILE"},
	}

	return AddAndBindFlags(fs, bindings)
}

func SetupPDPFlags(fs *pflag.FlagSet) error {
	fs.String(
		"lotus-url",
		"",
		"A websocket url for lotus node",
	)
	fs.String(
		"owner-address",
		"",
		"The ethereum address to submit PDP Proofs with (must be in piri wallet - see `piri wallet` command for help)",
	)
	fs.String(
		"contract-address",
		"0x6170dE2b09b404776197485F3dc6c968Ef948505", // NB(forrest): default to calibration contract addrese
		"The ethereum address of the PDP Contract",
	)

	bindings := []FlagBinding{
		{"lotus-url", "pdp.lotus_endpoint", "PIRI_LOTUS_URL"},
		{"owner-address", "pdp.owner_address", ""},
		{"contract-address", "pdp.contract_address", ""},
	}

	return AddAndBindFlags(fs, bindings)
}

func SetupUCANFlags(fs *pflag.FlagSet) error {
	fs.Uint64(
		"proof-set",
		0,
		"Proofset to use with PDP",
	)
	fs.String(
		"indexing-service-proof",
		"",
		"A delegation that allows the node to cache claims with the indexing service",
	)
	fs.String(
		"indexing-service-did",
		presets.IndexingServiceDID.String(),
		"DID of the indexing service",
	)
	fs.String(
		"indexing-service-url",
		presets.IndexingServiceURL.String(),
		"URL of the indexing service",
	)
	fs.String(
		"upload-service-did",
		presets.UploadServiceDID.String(),
		"DID of the upload service",
	)
	fs.String(
		"upload-service-url",
		presets.UploadServiceURL.String(),
		"URL of the upload service",
	)
	fs.StringSlice(
		"ipni-announce-urls",
		func() []string {
			out := make([]string, 0)
			for _, u := range presets.IPNIAnnounceURLs {
				out = append(out, u.String())
			}
			return out
		}(),
		"A list of IPNI announce URLs",
	)
	fs.StringToString(
		"service-principal-mapping",
		presets.PrincipalMapping,
		"Mapping of service DIDs to principal DIDs",
	)
	bindings := []FlagBinding{
		{"proof-set", "ucan.proof_set", "PIRI_PROOF_SET"},
		{"indexing-service-proof", "ucan.services.indexer.proof", "PIRI_INDEXING_SERVICE_PROOF"},
		{"indexing-service-did", "ucan.services.indexer.did", "PIRI_INDEXING_SERVICE_DID"},
		{"indexing-service-url", "ucan.services.indexer.url", "PIRI_INDEXING_SERVICE_URL"},
		{"upload-service-did", "ucan.services.upload.did", "PIRI_UPLOAD_SERVICE_DID"},
		{"upload-service-url", "ucan.services.upload.url", "PIRI_UPLOAD_SERVICE_URL"},
		{"ipni-announce-urls", "ucan.services.publisher.ipni_announce.urls", "PIRI_IPNI_ANNOUNCE_URLS"},
		{"service-principal-mapping", "ucan.services.principal_mapping", "PIRI_SERVICE_PRINCIPAL_MAPPING"},
	}
	return AddAndBindFlags(fs, bindings)
}
