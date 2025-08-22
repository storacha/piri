package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig
	Store     DynamoConfig
	Delegator DelegatorServiceConfig
}

type ServerConfig struct {
	Host string
	Port int
}

func NewConfig() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Host: viper.GetString("server.host"),
			Port: viper.GetInt("server.port"),
		},
		Store: DynamoConfig{
			Region:                viper.GetString("store.region"),
			AllowListTableName:    viper.GetString("store.allowlist_table_name"),
			ProviderInfoTableName: viper.GetString("store.providerinfo_table_name"),
			ProviderWeight:        viper.GetUint("store.providerweight"),
			Endpoint:              viper.GetString("store.endpoint"),
		},
		Delegator: DelegatorServiceConfig{
			KeyFile:               viper.GetString("delegator.key_file"),
			IndexingServiceWebDID: viper.GetString("delegator.indexing_service_web_did"),
			IndexingServiceProof:  viper.GetString("delegator.indexing_service_proof"),
			UploadServiceDID:      viper.GetString("delegator.upload_service_did"),
		},
	}

	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}

	if cfg.Store.Region == "" {
		return nil, fmt.Errorf("store region not set")
	}
	if cfg.Store.AllowListTableName == "" {
		return nil, fmt.Errorf("store allow list table not set")
	}
	if cfg.Store.ProviderInfoTableName == "" {
		return nil, fmt.Errorf("store provider info table not set")
	}

	if cfg.Delegator.KeyFile == "" {
		return nil, fmt.Errorf("delegator key file not set")
	}
	if cfg.Delegator.IndexingServiceWebDID == "" {
		return nil, fmt.Errorf("delegator indexing service did not set")
	}
	if cfg.Delegator.IndexingServiceProof == "" {
		return nil, fmt.Errorf("delegator indexing service proof not set")
	}
	if cfg.Delegator.UploadServiceDID == "" {
		return nil, fmt.Errorf("delegator upload did not set")
	}

	return cfg, nil
}

func (c *ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

type DynamoConfig struct {
	// Region of the dynamoDB instance
	Region string
	// Name of table we use for allowing users to register
	AllowListTableName string
	// Name of table we persist registered user data to
	ProviderInfoTableName string

	// ProviderWeight is the weight that will be assigned to a provider
	// when they are registered. This value will affect their odds of being
	// selected for an upload. `0` means they will not be selected.
	ProviderWeight uint

	// Endpoint may be set for local testing, usually with docker, e.g.
	// docker run -p 8000:8000 amazon/dynamodb-local -jar DynamoDBLocal.jar -sharedDb
	// then set endpoint to localhost:8080
	// Do not set for production.
	Endpoint string // for development
}

type DelegatorServiceConfig struct {
	KeyFile               string
	IndexingServiceWebDID string
	IndexingServiceProof  string
	UploadServiceDID      string
}
