package app

// AppConfig is the root configuration for the entire application
type AppConfig struct {
	// Identity configuration
	Identity IdentityConfig

	// Server configuration
	Server ServerConfig

	// Storage paths and directories
	Storage StorageConfig

	// External service-specific configurations: indexer, upload, publisher
	ExternalServices ExternalServicesConfig

	// Configuration specific for PDP operations
	PDPService PDPServiceConfig
}
