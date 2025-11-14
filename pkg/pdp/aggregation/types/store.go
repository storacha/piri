package types

import (
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/ipld/go-ipld-prime/datamodel"
	captypes "github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/piri/internal/ipldstore"
	"go.uber.org/fx"
)

type Store ipldstore.KVStore[datamodel.Link, Aggregate]

type StoreParams struct {
	fx.In
	Datastore datastore.Datastore `name:"aggregator_datastore"`
}

const AggregatePrefix = "aggregates/"

func NewStore(params StoreParams) Store {
	return ipldstore.IPLDStore[datamodel.Link, Aggregate](
		store.SimpleStoreFromDatastore(namespace.Wrap(params.Datastore, datastore.NewKey(AggregatePrefix))),
		AggregateType(), captypes.Converters...,
	)

}
