package config

import (
	"net/url"

	"github.com/storacha/piri/pkg/config/app"
)

type UCANServerConfig struct {
	Identity    IdentityConfig    `mapstructure:"identity"`
	Repo        RepoConfig        `mapstructure:"repo"`
	Server      ServerConfig      `mapstructure:"server"`
	UCANService UCANServiceConfig `mapstructure:"ucan"`
}

func (u UCANServerConfig) Validate() error {
	return validateConfig(u)
}

// Normalize applies compatibility fixes before validation.
func (u *UCANServerConfig) Normalize() {
	u.UCANService.Normalize()
}

type UCANServiceConfig struct {
	Services   ServicesConfig `mapstructure:"services" toml:"services"`
	ProofSetID uint64         `mapstructure:"proof_set" flag:"proof-set" toml:"proof_set"`
}

func (s UCANServiceConfig) Validate() error {
	return validateConfig(s)
}

// Normalize applies compatibility fixes before validation.
func (s *UCANServiceConfig) Normalize() {
	s.Services.Normalize()
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
