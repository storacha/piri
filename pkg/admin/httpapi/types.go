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
