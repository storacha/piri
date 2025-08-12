package config

type RepoConfig struct {
	DataDir string `mapstructure:"data_dir" validate:"required" flag:"data-dir"`
	TempDir string `mapstructure:"temp_dir" validate:"required" flag:"temp-dir"`
}

func (r RepoConfig) Validate() error {
	return validateConfig(r)
}
