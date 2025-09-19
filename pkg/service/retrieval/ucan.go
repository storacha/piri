package retrieval

import (
	"github.com/storacha/go-ucanto/server"
	"github.com/storacha/go-ucanto/server/retrieval"
	"github.com/storacha/piri/pkg/service/retrieval/ucan"
)

func NewUCANServer(retrievalService Service, options ...retrieval.Option) (server.ServerView[retrieval.Service], error) {
	options = append(
		options,
		ucan.SpaceContentRetrieve(retrievalService),
	)

	return retrieval.NewServer(retrievalService.ID(), options...)
}
