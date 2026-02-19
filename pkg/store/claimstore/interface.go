package claimstore

import (
	"github.com/storacha/piri/pkg/store/delegationstore"
)

// TODO a glorified type alias, remove this
type ClaimStore interface {
	delegationstore.DelegationStore
}
