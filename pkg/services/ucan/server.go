package ucan

import (
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/go-ucanto/server"
	"github.com/storacha/go-ucanto/transport"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/services/types"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

// Server wraps the UCAN server and registers capability handlers
type Server struct {
	id               principal.Signer
	blob             types.Blobs
	claim            types.Claims
	receiptStore     receiptstore.ReceiptStore
	replicator       types.Replicator
	uploadConnection client.Connection
	ucanServer       server.ServerView
}

// ServerParams defines all dependencies needed by the UCAN server
type ServerParams struct {
	fx.In
	ID               principal.Signer
	Blob             types.Blobs
	Claim            types.Claims
	ReceiptStore     receiptstore.ReceiptStore
	Replicator       types.Replicator
	UploadConnection client.Connection `name:"upload"`

	// Collect all server options from handlers
	Options []server.Option `group:"ucan-options"`
}

// NewServer creates a new UCAN server with all handlers
func NewServer(params ServerParams) (*Server, error) {
	ucanServer, err := server.NewServer(params.ID, params.Options...)
	if err != nil {
		return nil, err
	}

	return &Server{
		id:               params.ID,
		blob:             params.Blob,
		claim:            params.Claim,
		receiptStore:     params.ReceiptStore,
		replicator:       params.Replicator,
		uploadConnection: params.UploadConnection,
		ucanServer:       ucanServer,
	}, nil
}

// Request handles a UCAN request by delegating to the embedded UCAN server
func (s *Server) Request(req transport.HTTPRequest) (transport.HTTPResponse, error) {
	return s.ucanServer.Request(req)
}
