package httpapi

// Logging
type (
	ListLogLevelsResponse struct {
		Loggers map[string]string `json:"loggers"`
	}
	SetLogLevelRequest struct {
		System string `json:"system"`
		Level  string `json:"level"`
	}

	SetLogLevelRegexRequest struct {
		Expression string `json:"expression"`
		Level      string `json:"level"`
	}
)

// Dynamic Configuration
type (
	// ConfigResponse returns all dynamic configuration values as key-value pairs.
	ConfigResponse struct {
		Values map[string]any `json:"values"`
	}

	// UpdateConfigRequest updates one or more configuration values.
	// Keys are dot-notation paths like "pdp.aggregation.manager.poll_interval".
	// Values are raw JSON values that will be parsed and validated by the registry.
	UpdateConfigRequest struct {
		// Updates maps config key paths to their new values.
		Updates map[string]any `json:"updates"`
		// Persist writes changes to config file if true.
		Persist bool `json:"persist"`
	}
)
