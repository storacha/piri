package config

type Client struct {
	Identity    IdentityConfig    `mapstructure:"identity"`
	API         API               `mapstructure:"api"`
	UCANService UCANServiceConfig `mapstructure:"ucan" toml:"ucan"`
}

func (c Client) Validate() error {
	return validateConfig(c)
}

type API struct {
	// The URL of the node to establish an API connection with
	Endpoint string `mapstructure:"endpoint" validate:"required" flag:"node-url"`
}
