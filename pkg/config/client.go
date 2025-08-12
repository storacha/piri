package config

type Client struct {
	Identity Identity `mapstructure:"identity"`
	API      API      `mapstructure:"api"`
}

func (c Client) Validate() error {
	return validateConfig(c)
}

type API struct {
	// The URL of the node to establish an API connection with
	Endpoint string `mapstructure:"endpoint" validate:"required"`

	// NB: We could derive both of these (DID, Proof) from the Identity iff the client
	// is talking to the server run by the same operator. In practice this
	// will almost always be the case

	// The DID of the node to establish an API connection with
	DID string `mapstructure:"did" validate:"required"`
	// The Proof use to authenticate API interactions (optional?)
	Proof string `json:"proof"`
}
