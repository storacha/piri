package config

type UCANServerConfig struct {
	Identity     IdentityConfig    `mapstructure:"identity"`
	Repo         RepoConfig        `mapstructure:"repo"`
	Server       ServerConfig      `mapstructure:"server"`
	UCANService  UCANServiceConfig `mapstructure:"ucan_service"`
	PDPServerURL string            `mapstructure:"pdp_server_url"`
}

func (u UCANServerConfig) Validate() error {
	return validateConfig(u)
}

type UCANServiceConfig struct {
	Services   ServicesConfig `mapstructure:"services"`
	ProofSetID uint64         `mapstructure:"proof_set" flag:"proof-set"`
}
