package config

type ServicesConfig struct {
	ServicePrincipalMapping map[string]string `mapstructure:"principal_mapping" flag:"service-principal-mapping"`

	Indexer   IndexingServiceConfig  `mapstructure:"indexer" validate:"required"`
	Upload    UploadServiceConfig    `mapstructure:"upload" validate:"required"`
	Publisher PublisherServiceConfig `mapstructure:"publisher" validate:"required"`
}

func (s ServicesConfig) Validate() error {
	return validateConfig(s)
}

type IndexingServiceConfig struct {
	DID   string `mapstructure:"did" validate:"required" flag:"indexing-service-did"`
	URL   string `mapstructure:"url" validate:"required,url" flag:"indexing-service-url"`
	Proof string `mapstructure:"proof" flag:"indexing-service-proof"`
}

func (s *IndexingServiceConfig) Validate() error {
	return validateConfig(s)
}

type UploadServiceConfig struct {
	DID string `mapstructure:"did" validate:"required" flag:"upload-service-did"`
	URL string `mapstructure:"url" validate:"required,url" flag:"upload-service-url"`
}

func (s *UploadServiceConfig) Validate() error {
	return validateConfig(s)
}

type PublisherServiceConfig struct {
	AnnounceURLs []string `mapstructure:"ipni_announce_urls" validate:"required,min=1,dive,url" flag:"ipni-announce-urls"`
}

func (s *PublisherServiceConfig) Validate() error {
	return validateConfig(s)
}
