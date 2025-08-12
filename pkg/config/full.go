package config

import (
	"fmt"

	"github.com/storacha/piri/pkg/config/app"
)

type FullServerConfig struct {
	Identity    IdentityConfig    `mapstructure:"identity"`
	Repo        RepoConfig        `mapstructure:"repo"`
	Server      ServerConfig      `mapstructure:"server"`
	PDPService  PDPServiceConfig  `mapstructure:"pdp"`
	UCANService UCANServiceConfig `mapstructure:"ucan"`
}

func (f FullServerConfig) Validate() error {
	return validateConfig(f)
}

func (f FullServerConfig) ToAppConfig() (app.AppConfig, error) {
	var (
		err error
		out app.AppConfig
	)

	out.Identity, err = f.Identity.ToAppConfig()
	if err != nil {
		return app.AppConfig{}, fmt.Errorf("converting identity to app config: %s", err)
	}

	out.Server, err = f.Server.ToAppConfig()
	if err != nil {
		return app.AppConfig{}, fmt.Errorf("converting server config to app config: %s", err)
	}

	out.Storage, err = f.Repo.ToAppConfig()
	if err != nil {
		return app.AppConfig{}, fmt.Errorf("converting repo to app config: %s", err)
	}

	out.UCANService, err = f.UCANService.ToAppConfig(out.Server.PublicURL)
	if err != nil {
		return app.AppConfig{}, fmt.Errorf("converting services to app config: %s", err)
	}

	out.PDPService, err = f.PDPService.ToAppConfig()
	if err != nil {
		return app.AppConfig{}, fmt.Errorf("converting local pdp to app config: %s", err)
	}
	return out, nil
}
