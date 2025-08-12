package app

import (
	"net/url"
)

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	Host      string
	Port      uint
	PublicURL url.URL
}
