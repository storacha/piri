package storage

import (
	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/go-ucanto/server"
	"github.com/storacha/go-ucanto/server/retrieval"

	"github.com/storacha/piri/pkg/service/storage/ucan"
)

var log = logging.Logger("storage")

func NewUCANServer(storageService Service, options ...server.Option) (server.ServerView[server.Service], error) {
	options = append(
		options,
		ucan.BlobAllocate(storageService),
		ucan.BlobAccept(storageService),
		ucan.PDPInfo(storageService),
		ucan.ReplicaAllocate(storageService),
	)

	return server.NewServer(storageService.ID(), options...)
}

func NewUCANRetrievalServer(storageService Service, options ...retrieval.Option) (server.ServerView[retrieval.Service], error) {
	options = append(
		options,
		ucan.SpaceContentRetrieve(storageService),
	)

	return retrieval.NewServer(storageService.ID(), options...)
}
