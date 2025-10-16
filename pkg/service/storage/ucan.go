package storage

import (
	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/go-ucanto/server"

	"github.com/storacha/piri/pkg/service/storage/ucan"
)

var log = logging.Logger("storage")

func NewUCANServer(storageService Service, options ...server.Option) (server.ServerView[server.Service], error) {
	options = append(
		options,
		ucan.AccessGrant(storageService),
		ucan.BlobAllocate(storageService),
		ucan.BlobAccept(storageService),
		ucan.PDPInfo(storageService),
		ucan.ReplicaAllocate(storageService),
	)

	return server.NewServer(storageService.ID(), options...)
}
