package config

import (
	"fmt"

	logging "github.com/ipfs/go-log/v2"

	"github.com/storacha/piri/pkg/config/app"
)

var log = logging.Logger("config")

type Full struct {
	Identity Identity       `mapstructure:"identity"`
	Repo     Repo           `mapstructure:"repo"`
	Server   Server         `mapstructure:"server"`
	Services Services       `mapstructure:"services"`
	LocalPDP LocalPDPConfig `mapstructure:"pdp"`
}

func (p Full) Validate() error {
	return validateConfig(p)
}

func (p Full) ToAppConfig() (app.AppConfig, error) {
	var (
		err error
		out app.AppConfig
	)

	out.Identity, err = p.Identity.ToAppConfig()
	if err != nil {
		return app.AppConfig{}, fmt.Errorf("converting identity to app config: %s", err)
	}

	out.Server, err = p.Server.ToAppConfig()
	if err != nil {
		return app.AppConfig{}, fmt.Errorf("converting server config to app config: %s", err)
	}

	out.Storage, err = p.Repo.ToAppConfig()
	if err != nil {
		return app.AppConfig{}, fmt.Errorf("converting repo to app config: %s", err)
	}

	out.ExternalServices, err = p.Services.ToAppConfig(out.Server.PublicURL)
	if err != nil {
		return app.AppConfig{}, fmt.Errorf("converting services to app config: %s", err)
	}

	out.PDPService.Local, err = p.LocalPDP.ToAppConfig()
	if err != nil {
		return app.AppConfig{}, fmt.Errorf("converting local pdp to app config: %s", err)
	}
	return out, nil
}
