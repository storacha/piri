package config

type PDPServerConfig struct {
	Identity   IdentityConfig   `mapstructure:"identity"`
	Repo       RepoConfig       `mapstructure:"repo"`
	Server     ServerConfig     `mapstructure:"server"`
	PDPService PDPServiceConfig `mapstructure:"pdp"`
}

func (c PDPServerConfig) Validate() error {
	return validateConfig(c)
}

type PDPServiceConfig struct {
	OwnerAddress    string `mapstructure:"owner_address" validate:"required" flag:"owner-address"`
	ContractAddress string `mapstructure:"contract_address" validate:"required" flag:"contract-address"`
	LotusEndpoint   string `mapstructure:"lotus_endpoint" validate:"required" flag:"lotus-endpoint"`
}

func (c PDPServiceConfig) Validate() error {
	return validateConfig(c)
}
