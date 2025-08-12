package config

import (
	"github.com/storacha/piri/lib"
	"github.com/storacha/piri/pkg/config/app"
)

type Identity struct {
	KeyFile string `mapstructure:"key_file" validate:"required" flag:"key-file"`
}

func (i Identity) Validate() error {
	return validateConfig(i)
}

func (i Identity) ToAppConfig() (app.IdentityConfig, error) {
	id, err := lib.SignerFromEd25519PEMFile(i.KeyFile)
	if err != nil {
		return app.IdentityConfig{}, err
	}
	return app.IdentityConfig{
		Signer: id,
	}, nil
}
