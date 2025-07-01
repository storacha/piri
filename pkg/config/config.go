package config

import (
	"os"
	"path/filepath"

	"github.com/samber/lo"
	"github.com/spf13/viper"
)

type Config struct {
	Identity   Identity   `mapstructure:"identity" validate:"required"`
	Repo       Repo       `mapstructure:"repo" validate:"required"`
	UCANServer UCANServer `mapstructure:"ucan_server" validate:"required"`
	PDPServer  PDPServer  `mapstructure:"pdp_server" validate:"required"`
	PDPClient  PDPClient  `mapstructure:"pdp_client" validate:"required"`
	UCANClient UCANClient `mapstructure:"ucan_client" validate:"required"`
}

type Identity struct {
	KeyFile string `mapstructure:"key_file" validate:"required" flag:"key-file"`
}

func (i Identity) Validate() error {
	return validateConfig(i)
}

type Repo struct {
	DataDir string `mapstructure:"data_dir" validate:"required" flag:"data-dir"`
	TempDir string `mapstructure:"temp_dir" validate:"required" flag:"temp-dir"`
}

func (r Repo) Validate() error {
	return validateConfig(r)
}

var DefaultRepo = Repo{
	DataDir: filepath.Join(lo.Must(os.UserHomeDir()), ".storacha"),
	TempDir: filepath.Join(os.TempDir(), "storage"),
}

type UCANServer struct {
	Identity `mapstructure:"identity,squash" validate:"required"`
	Repo     `mapstructure:"repo,squash" validate:"required"`

	Port                    uint              `mapstructure:"port" validate:"required,min=1,max=65535" flag:"port"`
	Host                    string            `mapstructure:"host" validate:"required" flag:"host"`
	PDPServerURL            string            `mapstructure:"pdp_server_url" validate:"required,url" flag:"pdp-server-url"`
	PublicURL               string            `mapstructure:"public_url" validate:"omitempty,url" flag:"public-url"`
	IndexingServiceProof    string            `mapstructure:"indexing_service_proof" flag:"indexing-service-proof"`
	ProofSet                uint64            `mapstructure:"proof_set" flag:"proof-set"`
	IPNIAnnounceURLs        []string          `mapstructure:"ipni_announce_urls" validate:"required,min=1,dive,url" flag:"ipni-announce-urls"`
	IndexingServiceDID      string            `mapstructure:"indexing_service_did" validate:"required" flag:"indexing-service-did"`
	IndexingServiceURL      string            `mapstructure:"indexing_service_url" validate:"required,url" flag:"indexing-service-url"`
	UploadServiceDID        string            `mapstructure:"upload_service_did" validate:"required" flag:"upload-service-did"`
	UploadServiceURL        string            `mapstructure:"upload_service_url" validate:"required,url" flag:"upload-service-url"`
	ServicePrincipalMapping map[string]string `mapstructure:"service_principal_mapping" flag:"service-principal-mapping"`
}

func (u UCANServer) Validate() error {
	return validateConfig(u)
}

var DefaultUCANServer = UCANServer{
	Host:               "localhost",
	Port:               3000,
	PDPServerURL:       "http://localhost:3001",
	IPNIAnnounceURLs:   []string{"https://cid.contact/announce"},
	IndexingServiceDID: "did:web:indexer.storacha.network",
	IndexingServiceURL: "https://indexer.storacha.network",
	UploadServiceDID:   "did:web:up.storacha.network",
	UploadServiceURL:   "https://up.storacha.network",
	ServicePrincipalMapping: map[string]string{
		"did:web:staging.up.storacha.network": "did:key:z6MkhcbEpJpEvNVDd3n5RurquVdqs5dPU16JDU5VZTDtFgnn",
		"did:web:up.storacha.network":         "did:key:z6MkqdncRZ1wj8zxCTDUQ8CRT8NQWd63T7mZRvZUX8B7XDFi",
		"did:web:staging.web3.storage":        "did:key:z6MkhcbEpJpEvNVDd3n5RurquVdqs5dPU16JDU5VZTDtFgnn",
		"did:web:web3.storage":                "did:key:z6MkqdncRZ1wj8zxCTDUQ8CRT8NQWd63T7mZRvZUX8B7XDFi",
	},
}

type UCANClient struct {
	Identity `mapstructure:"identity,squash" validate:"required"`
	NodeURL  string `mapstructure:"node_url" validate:"required,url" flag:"node-url"`
	NodeDID  string `mapstructure:"node_did" validate:"required" flag:"node-did"`
	Proof    string `mapstructure:"proof" validate:"required" flag:"proof"`
}

type PDPServer struct {
	Repo `mapstructure:"repo,squash" validate:"required"`

	Endpoint   string `mapstructure:"endpoint" validate:"required,url" flag:"host"`
	LotusURL   string `mapstructure:"lotus_url" validate:"required,url" flag:"lotus-url"`
	EthAddress string `mapstructure:"eth_address" validate:"required" flag:"eth-address"`
}

func (p PDPServer) Validate() error {
	return validateConfig(p)
}

var DefaultPDPServer = PDPServer{
	Endpoint: "http://localhost:3001",
	LotusURL: "http://localhost:1234",
}

type PDPClient struct {
	Identity `mapstructure:"identity,squash" validate:"required"`
	NodeURL  string `mapstructure:"node_url" validate:"required,url" flag:"node-url"`
}

var DefaultPDPClient = PDPClient{
	NodeURL: "http://localhost:3001",
}

func Load[T Validatable]() (T, error) {
	var out T
	if err := viper.Unmarshal(&out); err != nil {
		return out, err
	}
	if err := out.Validate(); err != nil {
		return out, err
	}

	return out, nil
}
