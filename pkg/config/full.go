package config

import (
	"fmt"

	"github.com/storacha/piri/pkg/config/app"
)

type FullServerConfig struct {
	Network     string            `mapstructure:"network" validate:"required" flag:"network" toml:"network"`
	Identity    IdentityConfig    `mapstructure:"identity" toml:"identity"`
	Repo        RepoConfig        `mapstructure:"repo" toml:"repo"`
	Server      ServerConfig      `mapstructure:"server" toml:"server"`
	PDPService  PDPServiceConfig  `mapstructure:"pdp" toml:"pdp"`
	UCANService UCANServiceConfig `mapstructure:"ucan" toml:"ucan"`
	Telemetry   TelemetryConfig   `mapstructure:"telemetry" toml:"telemetry"`
}

func (f FullServerConfig) Validate() error {
	return validateConfig(f)
}

// Normalize applies compatibility fixes before validation.
func (f *FullServerConfig) Normalize() {
	f.UCANService.Normalize()
}

func (f FullServerConfig) ToAppConfig() (app.AppConfig, error) {
	var (
		err error
		out app.AppConfig
	)

	//
	// user provided configuration
	//
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

	out.Telemetry = f.Telemetry.ToAppConfig()

	//
	// non-user configuration
	//
	out.Replicator = app.DefaultReplicatorConfig()

	return out, nil
}
