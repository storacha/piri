package config

import (
	"time"

	"github.com/storacha/piri/pkg/config/app"
)

type TelemetryCollectorConfig struct {
	Endpoint        string            `mapstructure:"endpoint" validate:"required" toml:"endpoint"`
	Insecure        bool              `mapstructure:"insecure" toml:"insecure,omitempty"`
	Headers         map[string]string `mapstructure:"headers" toml:"headers,omitempty"`
	PublishInterval time.Duration     `mapstructure:"publish_interval" toml:"publish_interval,omitempty"`
}

type TelemetryConfig struct {
	Metrics                  []TelemetryCollectorConfig `mapstructure:"metrics" toml:"metrics,omitempty"`
	Traces                   []TelemetryCollectorConfig `mapstructure:"traces" toml:"traces,omitempty"`
	DisableStorachaAnalytics bool                       `mapstructure:"disable_storacha_analytics" toml:"disable_storacha_analytics,omitempty"`
}

func (t TelemetryConfig) Validate() error {
	return validateConfig(t)
}

func (t TelemetryConfig) ToAppConfig() app.TelemetryConfig {
	convert := func(in []TelemetryCollectorConfig) []app.TelemetryCollectorConfig {
		out := make([]app.TelemetryCollectorConfig, 0, len(in))
		for _, c := range in {
			out = append(out, app.TelemetryCollectorConfig{
				Endpoint:        c.Endpoint,
				Insecure:        c.Insecure,
				Headers:         c.Headers,
				PublishInterval: c.PublishInterval,
			})
		}
		return out
	}

	return app.TelemetryConfig{
		Metrics:                  convert(t.Metrics),
		Traces:                   convert(t.Traces),
		DisableStorachaAnalytics: t.DisableStorachaAnalytics,
	}
}
