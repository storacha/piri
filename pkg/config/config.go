package config

import (
	"github.com/spf13/viper"

	"github.com/storacha/piri/pkg/presets"
)

type UCANServer struct {
	Identity `mapstructure:"identity,squash" validate:"required"`
	Repo     `mapstructure:"repo,squash" validate:"required"`

	Port                    uint              `mapstructure:"port" validate:"required,min=1,max=65535" flag:"port"`
	Host                    string            `mapstructure:"host" validate:"required" flag:"host"`
	PDPServerURL            string            `mapstructure:"pdp_server_url" validate:"omitempty,url" flag:"pdp-server-url"`
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
	Host: "localhost",
	Port: 3000,
	IPNIAnnounceURLs: func() []string {
		out := make([]string, len(presets.IPNIAnnounceURLs))
		for i, p := range presets.IPNIAnnounceURLs {
			out[i] = p.String()
		}
		return out
	}(),
	IndexingServiceDID:      presets.IndexingServiceDID.String(),
	IndexingServiceURL:      presets.IndexingServiceURL.String(),
	UploadServiceDID:        presets.UploadServiceDID.String(),
	UploadServiceURL:        presets.UploadServiceURL.String(),
	ServicePrincipalMapping: presets.PrincipalMapping,
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
	if err := viper.WriteConfigAs("test.toml"); err != nil {
		panic(err)
	}
	v := viper.GetViper()
	_ = v
	if err := out.Validate(); err != nil {
		return out, err
	}

	return out, nil
}
