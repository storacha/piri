package publisher

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"sync"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/advertisement"
	"github.com/storacha/go-libstoracha/capabilities/assert"
	"github.com/storacha/go-libstoracha/capabilities/claim"
	ipnipub "github.com/storacha/go-libstoracha/ipnipublisher/publisher"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-libstoracha/metadata"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/ipld"
	"github.com/storacha/go-ucanto/core/receipt"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/core/result/ok"
	"github.com/storacha/go-ucanto/principal"
	"go.uber.org/fx"
)

var log = logging.Logger("publisher")

// Service provides content publishing functionality
type Service struct {
	id              principal.Signer
	store           store.PublisherStore
	publisher       ipnipub.Publisher
	provider        peer.AddrInfo
	indexingService client.Connection
	indexingProofs  delegation.Proofs
	mu              sync.Mutex
}

// Params defines the dependencies for Service
type Params struct {
	fx.In
	ID         principal.Signer
	Store      store.PublisherStore
	PublicAddr multiaddr.Multiaddr
	BlobAddr   multiaddr.Multiaddr

	AnnounceURLs    []url.URL           `optional:"true"`
	AnnounceAddr    multiaddr.Multiaddr `optional:"true"`
	IndexingService client.Connection   `optional:"true"`
	IndexingProofs  delegation.Proofs   `optional:"true"`
}

// NewService creates a new publisher service
func NewService(params Params) (*Service, error) {
	priv, err := crypto.UnmarshalEd25519PrivateKey(params.ID.Raw())
	if err != nil {
		return nil, fmt.Errorf("unmarshaling private key: %w", err)
	}

	announceAddr := params.AnnounceAddr
	if announceAddr == nil {
		announceAddr = params.PublicAddr
	}

	ipnipubOpts := []ipnipub.Option{ipnipub.WithAnnounceAddrs(announceAddr.String())}
	for _, u := range params.AnnounceURLs {
		log.Infof("Announcing new IPNI adverts to: %s", u.String())
		ipnipubOpts = append(ipnipubOpts, ipnipub.WithDirectAnnounce(u.String()))
	}
	publisher, err := ipnipub.New(priv, params.Store, ipnipubOpts...)
	if err != nil {
		return nil, fmt.Errorf("creating IPNI publisher instance: %w", err)
	}

	found := false
	for _, p := range params.PublicAddr.Protocols() {
		if p.Code == multiaddr.P_HTTPS || p.Code == multiaddr.P_HTTP {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("IPNI publisher address is not HTTP(S): %s", params.PublicAddr)
	}

	if params.IndexingService == nil {
		log.Warn("Indexing service is not configured - claims will not be cached")
	}

	peerid, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("creating libp2p peer ID from private key: %w", err)
	}

	return &Service{
		id:              params.ID,
		store:           params.Store,
		indexingService: params.IndexingService,
		indexingProofs:  params.IndexingProofs,

		publisher: publisher,
		provider:  providerInfo(peerid, params.PublicAddr, params.BlobAddr),
	}, nil
}

// Store returns the underlying publisher store
func (s *Service) Store() store.PublisherStore {
	return s.store
}

// Publish publishes content claims to the network
func (s *Service) Publish(ctx context.Context, dlg delegation.Delegation) error {
	ability := dlg.Capabilities()[0].Can()
	switch ability {
	case assert.LocationAbility:
		err := s.publishLocationCommitment(ctx, dlg)
		if err != nil {
			return err
		}
		return s.cacheClaim(ctx, dlg)
	default:
		return fmt.Errorf("unknown claim: %s", ability)
	}
}

func (s *Service) publishLocationCommitment(ctx context.Context, dlg delegation.Delegation) error {
	log := log.With("claim", dlg.Link())

	capability := dlg.Capabilities()[0]
	nb, rerr := assert.LocationCaveatsReader.Read(capability.Nb())
	if rerr != nil {
		return fmt.Errorf("reading location commitment data: %w", rerr)
	}

	digests := []multihash.Multihash{nb.Content.Hash()}
	contextid, err := advertisement.EncodeContextID(nb.Space, nb.Content.Hash())
	if err != nil {
		return fmt.Errorf("encoding advertisement context ID: %w", err)
	}

	var exp int
	if dlg.Expiration() != nil {
		exp = *dlg.Expiration()
	}

	shardCid, err := advertisement.ShardCID(s.provider, nb)
	if err != nil {
		return fmt.Errorf("failed to extract shard CID for provider: %s locationCommitment %s: %w", s.provider, capability, err)
	}

	meta := metadata.MetadataContext.New(
		&metadata.LocationCommitmentMetadata{
			Shard:      shardCid,
			Claim:      asCID(dlg.Link()),
			Expiration: int64(exp),
		},
	)

	s.mu.Lock()
	defer s.mu.Unlock()

	adlink, err := s.publisher.Publish(ctx, s.provider, string(contextid), slices.Values(digests), meta)
	if err != nil {
		if errors.Is(err, ipnipub.ErrAlreadyAdvertised) {
			log.Warnf("Skipping previously published claim")
			return nil
		}
		return fmt.Errorf("publishing claim: %w", err)
	}

	log.Infof("Published advertisement: %s", adlink)
	return nil
}

var claimCacheReceiptSchema = []byte(`
	type Result union {
		| Unit "ok"
		| Any "error"
	} representation keyed

	type Unit struct {}
`)
var claimCacheReceiptReader, _ = receipt.NewReceiptReader[ok.Unit, ipld.Node](claimCacheReceiptSchema)

func (s *Service) cacheClaim(ctx context.Context, dlg delegation.Delegation) error {
	log := log.With("claim", dlg.Link())

	if s.indexingService == nil {
		log.Warnf("Cannot cache claim - indexing service is not configured")
		return nil
	}

	var opts []delegation.Option
	if s.indexingProofs != nil {
		opts = append(opts, delegation.WithProof(s.indexingProofs...))
	}
	inv, err := claim.Cache.Invoke(
		s.id,
		s.indexingService.ID(),
		s.indexingService.ID().DID().String(),
		claim.CacheCaveats{
			Claim:    dlg.Link(),
			Provider: claim.Provider{Addresses: s.provider.Addrs},
		},
		opts...,
	)
	if err != nil {
		return fmt.Errorf("creating invocation: %w", err)
	}

	for b, err := range dlg.Blocks() {
		if err != nil {
			return fmt.Errorf("iterating claim blocks: %w", err)
		}
		err = inv.Attach(b)
		if err != nil {
			return fmt.Errorf("attaching block: %s: %w", b.Link(), err)
		}
	}

	res, err := client.Execute([]invocation.Invocation{inv}, s.indexingService)
	if err != nil {
		return fmt.Errorf("executing invocation: %w", err)
	}

	rcptLink, exists := res.Get(inv.Link())
	if !exists {
		return fmt.Errorf("getting receipt link: %w", err)
	}
	rcpt, err := claimCacheReceiptReader.Read(rcptLink, res.Blocks())
	if err != nil {
		return fmt.Errorf("reading receipt: %w", err)
	}
	return result.MatchResultR1(
		rcpt.Out(),
		func(ok ok.Unit) error {
			log.Info("Cached location commitment with indexing service")
			return nil
		},
		func(node ipld.Node) error {
			name := "UnknownError"
			message := "claim/cache invocation failed"
			nn, err := node.LookupByString("name")
			if err == nil {
				n, err := nn.AsString()
				if err == nil {
					name = n
				}
			}
			mn, err := node.LookupByString("message")
			if err == nil {
				m, err := mn.AsString()
				if err == nil {
					message = m
				}
			}
			return fmt.Errorf("%s: %s", name, message)
		},
	)

}

func asCID(link ipld.Link) cid.Cid {
	if cl, ok := link.(cidlink.Link); ok {
		return cl.Cid
	}
	return cid.MustParse(link.String())
}

func providerInfo(peerID peer.ID, publicAddr multiaddr.Multiaddr, blobAddr multiaddr.Multiaddr) peer.AddrInfo {
	provider := peer.AddrInfo{ID: peerID}
	if blobAddr == nil {
		blobSuffix, _ := multiaddr.NewMultiaddr("/http-path/" + url.PathEscape("blob/{blob}"))
		provider.Addrs = append(provider.Addrs, multiaddr.Join(publicAddr, blobSuffix))
	} else {
		provider.Addrs = append(provider.Addrs, blobAddr)
	}
	claimSuffix, _ := multiaddr.NewMultiaddr("/http-path/" + url.PathEscape("claim/{claim}"))
	provider.Addrs = append(provider.Addrs, multiaddr.Join(publicAddr, claimSuffix))

	return provider
}
