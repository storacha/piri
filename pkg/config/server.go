package config

type ServerConfig struct {
	Port      uint   `mapstructure:"port" validate:"required,min=1,max=65535" flag:"port"`
	Host      string `mapstructure:"host" validate:"required" flag:"host"`
	PublicURL string `mapstructure:"public_url" validate:"omitempty,url" flag:"public-url"`
}

func (s ServerConfig) Validate() error {
	return validateConfig(s)
}
