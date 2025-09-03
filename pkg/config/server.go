package config

import (
	"fmt"
	"net/url"

	"github.com/storacha/piri/pkg/config/app"
)

type ServerConfig struct {
	Port      uint   `mapstructure:"port" validate:"required,min=1,max=65535" flag:"port" toml:"port"`
	Host      string `mapstructure:"host" validate:"required" flag:"host" toml:"host"`
	PublicURL string `mapstructure:"public_url" validate:"omitempty,url" flag:"public-url" toml:"public_url"`
}

func (s ServerConfig) Validate() error {
	return validateConfig(s)
}

func (s ServerConfig) ToAppConfig() (app.ServerConfig, error) {
	var err error
	var publicURL *url.URL
	if s.PublicURL != "" {
		publicURL, err = url.Parse(s.PublicURL)
		if err != nil {
			return app.ServerConfig{}, fmt.Errorf("parsing public URL: %w", err)
		}
	} else {
		log.Warnf("public URL not set, using http://%s:%d", s.Host, s.Port)
		publicURL, err = url.Parse(fmt.Sprintf("http://%s:%d", s.Host, s.Port))
		if err != nil {
			return app.ServerConfig{}, fmt.Errorf("creating default public URL: %w", err)
		}
	}

	return app.ServerConfig{
		Host:      s.Host,
		Port:      s.Port,
		PublicURL: *publicURL,
	}, nil
}
