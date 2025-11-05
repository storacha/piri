package pdp

import (
	"fmt"

	"github.com/storacha/go-ucanto/principal"

	"github.com/storacha/piri/pkg/store/receiptstore"
)

type Config struct {
}

type PDPService struct {
}

func NewRemote(cfg *Config, id principal.Signer, receiptStore receiptstore.ReceiptStore) (*PDPService, error) {
	return nil, fmt.Errorf("remote PDP not supported")
}
