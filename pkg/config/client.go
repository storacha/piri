package config

type Client struct {
	Identity IdentityConfig `mapstructure:"identity"`
	API      API            `mapstructure:"api"`
	UCAN     UCANConfig     `mapstructure:"ucan" toml:"ucan"`
}

type UCANConfig struct {
	ProofSetID uint64 `mapstructure:"proof_set" toml:"proof_set"`
}

func (c Client) Validate() error {
	return validateConfig(c)
}

type API struct {
	// The URL of the node to establish an API connection with
	Endpoint string `mapstructure:"endpoint" validate:"required" flag:"node-url"`
}
