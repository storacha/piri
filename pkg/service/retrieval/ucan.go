package retrieval

import (
	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/go-ucanto/server"
	"github.com/storacha/go-ucanto/server/retrieval"
	"github.com/storacha/piri/pkg/service/retrieval/ucan"
)

var log = logging.Logger("retrieval")

func NewUCANRetrievalServer(retrievalService Service, options ...retrieval.Option) (server.ServerView[retrieval.Service], error) {
	options = append(
		options,
		ucan.SpaceContentRetrieve(retrievalService),
	)

	return retrieval.NewServer(retrievalService.ID(), options...)
}
