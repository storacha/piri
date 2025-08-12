package config

type IdentityConfig struct {
	KeyFile string `mapstructure:"key_file" validate:"required" flag:"key-file"`
}

func (i IdentityConfig) Validate() error {
	return validateConfig(i)
}
