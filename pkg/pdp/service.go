package pdp

import (
	"context"
	"errors"
	"net/url"

	"github.com/ipfs/go-datastore"
	"github.com/storacha/go-ucanto/principal"

	"github.com/storacha/piri/pkg/pdp/aggregator"
	"github.com/storacha/piri/pkg/pdp/pieceadder"
	"github.com/storacha/piri/pkg/pdp/piecefinder"
	"github.com/storacha/piri/pkg/pdp/piecereader"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

type Config struct {
	PDPDatastore datastore.Datastore
	PDPServerURL *url.URL
	ProofSet     uint64
	DatabasePath string
}

type PDPService struct {
	aggregator  aggregator.Aggregator
	pieceFinder piecefinder.PieceFinder
	pieceAdder  pieceadder.PieceAdder
	pieceReader piecereader.PieceReader
	startFuncs  []func(ctx context.Context) error
	closeFuncs  []func(ctx context.Context) error
}

func (p *PDPService) Aggregator() aggregator.Aggregator {
	return p.aggregator
}

func (p *PDPService) PieceAdder() pieceadder.PieceAdder {
	return p.pieceAdder
}

func (p *PDPService) PieceFinder() piecefinder.PieceFinder {
	return p.pieceFinder
}

func (p *PDPService) PieceReader() piecereader.PieceReader {
	return p.pieceReader
}

func (p *PDPService) Startup(ctx context.Context) error {
	var err error
	for _, startFunc := range p.startFuncs {
		err = errors.Join(startFunc(ctx))
	}
	return err
}

func (p *PDPService) Shutdown(ctx context.Context) error {
	var err error
	for _, closeFunc := range p.closeFuncs {
		err = errors.Join(closeFunc(ctx))
	}
	return err
}

var _ PDP = (*PDPService)(nil)

func NewRemote(cfg *Config, id principal.Signer, receiptStore receiptstore.ReceiptStore) (*PDPService, error) {
	panic("debt")
	/*
		api, err := client.New(cfg.PDPServerURL, client.WithBearerFromSigner(id))
		if err != nil {
			return nil, fmt.Errorf("creating PDP client api: %w", err)
		}
		agg, err := aggregator.NewLocal(cfg.PDPDatastore, cfg.DatabasePath, api, cfg.ProofSet, id, receiptStore)
		if err != nil {
			return nil, fmt.Errorf("creating aggregator: %w", err)
		}
		return &PDPService{
			aggregator:  agg,
			pieceFinder: piecefinder.New(api, cfg.PDPServerURL),
			pieceAdder:  pieceadder.New(api, cfg.PDPServerURL),
			pieceReader: piecereader.New(api, cfg.PDPServerURL),
			startFuncs: []func(ctx context.Context) error{
				func(ctx context.Context) error {
					return agg.Startup(ctx)
				},
			},
			closeFuncs: []func(context.Context) error{
				func(ctx context.Context) error { agg.Shutdown(ctx); return nil },
			},
		}, nil

	*/
}
