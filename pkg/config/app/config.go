package app

// AppConfig is the root configuration for the entire application
type AppConfig struct {
	// Identity configuration
	Identity IdentityConfig

	// Server configuration
	Server ServerConfig

	// Storage paths and directories
	Storage StorageConfig

	// Configuration specific for UCAN operations
	UCANService UCANServiceConfig

	// Configuration specific for PDP operations
	PDPService PDPServiceConfig
}
