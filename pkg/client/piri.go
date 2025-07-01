package client

import (
	"context"
	"net/url"

	"resty.dev/v3"

	"github.com/storacha/piri/pkg/pdp/types"
)

type Piri struct {
	endpoint *url.URL
	client   *resty.Client
}

func NewPiriClient(endpoint *url.URL) *Piri {
	return &Piri{
		endpoint: endpoint,
		client:   resty.New(),
	}
}

func (p *Piri) TaskHistory(ctx context.Context, filter *types.TaskHistoryFilter) (*types.TaskHistoryResponse, error) {
	res, err := p.client.R().
		SetContext(ctx).
		SetContentType("application/json").
		SetQueryParamsFromValues(filter.ToQueryParams()).
		SetResult(&types.TaskHistoryResponse{}).
		Get(p.endpoint.JoinPath("pdp", "task").String())
	if err != nil {
		return nil, err
	}
	return res.Result().(*types.TaskHistoryResponse), nil
}
