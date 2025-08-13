package config

import (
	"fmt"
	"net/url"

	"github.com/storacha/piri/pkg/config/app"
)

type UCANServerConfig struct {
	Identity     IdentityConfig    `mapstructure:"identity"`
	Repo         RepoConfig        `mapstructure:"repo"`
	Server       ServerConfig      `mapstructure:"server"`
	UCANService  UCANServiceConfig `mapstructure:"ucan"`
	PDPServerURL string            `mapstructure:"pdp_server_url"`
}

func (u UCANServerConfig) Validate() error {
	return validateConfig(u)
}

func (u UCANServerConfig) ToAppConfig() (app.AppConfig, error) {
	var (
		err error
		out app.AppConfig
	)

	out.Identity, err = u.Identity.ToAppConfig()
	if err != nil {
		return app.AppConfig{}, fmt.Errorf("converting identity to app config: %s", err)
	}

	out.Server, err = u.Server.ToAppConfig()
	if err != nil {
		return app.AppConfig{}, fmt.Errorf("converting server config to app config: %s", err)
	}

	out.Storage, err = u.Repo.ToAppConfig()
	if err != nil {
		return app.AppConfig{}, fmt.Errorf("converting repo to app config: %s", err)
	}

	out.UCANService, err = u.UCANService.ToAppConfig(out.Server.PublicURL)
	if err != nil {
		return app.AppConfig{}, fmt.Errorf("converting services to app config: %s", err)
	}

	return out, nil
}

type UCANServiceConfig struct {
	Services   ServicesConfig `mapstructure:"services"`
	ProofSetID uint64         `mapstructure:"proof_set" flag:"proof-set"`
}

func (s UCANServiceConfig) Validate() error {
	return validateConfig(s)
}

func (s UCANServiceConfig) ToAppConfig(publicURL url.URL) (app.UCANServiceConfig, error) {
	svcCfg, err := s.Services.ToAppConfig(publicURL)
	if err != nil {
		return app.UCANServiceConfig{}, err
	}
	return app.UCANServiceConfig{
		Services:   svcCfg,
		ProofSetID: s.ProofSetID,
	}, nil
}
