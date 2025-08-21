package config

type LocalConfig struct {
	Repo RepoConfig `mapstructure:"repo"`
}

func (l LocalConfig) Validate() error {
	return validateConfig(l)
}
