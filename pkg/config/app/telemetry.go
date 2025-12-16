package app

import "time"

type TelemetryConfig struct {
	Metrics                  []TelemetryCollectorConfig
	Traces                   []TelemetryCollectorConfig
	DisableStorachaAnalytics bool
}

type TelemetryCollectorConfig struct {
	Endpoint        string
	Insecure        bool
	Headers         map[string]string
	PublishInterval time.Duration
}
