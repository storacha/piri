package publisher

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/capabilities/assert"
	"github.com/storacha/go-libstoracha/capabilities/claim"
	ipnipub "github.com/storacha/go-libstoracha/ipnipublisher/publisher"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-libstoracha/metadata"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/core/result/ok"
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/piri/lib"

	"github.com/storacha/go-libstoracha/advertisement"
)

var log = logging.Logger("publisher")

type PublisherService struct {
	id                    principal.Signer
	store                 store.PublisherStore
	publisher             ipnipub.Publisher
	provider              peer.AddrInfo
	indexingService       client.Connection
	indexingServiceProofs delegation.Proofs
	mutex                 sync.Mutex
}

func (pub *PublisherService) Store() store.PublisherStore {
	return pub.store
}

func (pub *PublisherService) Publish(ctx context.Context, claim delegation.Delegation) error {
	ability := claim.Capabilities()[0].Can()
	switch ability {
	case assert.LocationAbility:
		err := PublishLocationCommitment(ctx, &pub.mutex, pub.publisher, pub.provider, claim)
		if err != nil {
			return err
		}
		return CacheClaim(ctx, pub.id, pub.indexingService, pub.indexingServiceProofs, claim, pub.provider.Addrs)
	default:
		return fmt.Errorf("unknown claim: %s", ability)
	}
}

func PublishLocationCommitment(
	ctx context.Context,
	mutex *sync.Mutex,
	publisher ipnipub.Publisher,
	provider peer.AddrInfo,
	locationCommitment delegation.Delegation,
) error {
	log := log.With("claim", locationCommitment.Link())

	capability := locationCommitment.Capabilities()[0]
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
	if locationCommitment.Expiration() != nil {
		exp = *locationCommitment.Expiration()
	}

	shardCid, err := advertisement.ShardCID(provider, nb)
	if err != nil {
		return fmt.Errorf("failed to extract shard CID for provider: %s locationCommitment %s: %w", provider, capability, err)
	}

	meta := metadata.MetadataContext.New(
		&metadata.LocationCommitmentMetadata{
			Shard:      shardCid,
			Claim:      asCID(locationCommitment.Link()),
			Expiration: int64(exp),
		},
	)

	mutex.Lock()
	defer mutex.Unlock()

	adlink, err := publisher.Publish(ctx, provider, string(contextid), slices.Values(digests), meta)
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

func CacheClaim(
	ctx context.Context,
	id principal.Signer,
	indexingService client.Connection,
	invocationProofs delegation.Proofs,
	clm delegation.Delegation,
	providerAddresses []multiaddr.Multiaddr,
) error {
	log := log.With("claim", clm.Link())

	if indexingService == nil {
		log.Warnf("Cannot cache claim - indexing service is not configured")
		return nil
	}

	inv, err := claim.Cache.Invoke(
		id,
		indexingService.ID(),
		indexingService.ID().DID().String(),
		claim.CacheCaveats{
			Claim:    clm.Link(),
			Provider: claim.Provider{Addresses: providerAddresses},
		},
		delegation.WithProof(invocationProofs...),
	)
	if err != nil {
		return fmt.Errorf("creating invocation: %w", err)
	}

	for b, err := range clm.Blocks() {
		if err != nil {
			return fmt.Errorf("iterating claim blocks: %w", err)
		}
		err = inv.Attach(b)
		if err != nil {
			return fmt.Errorf("attaching block: %s: %w", b.Link(), err)
		}
	}

	res, err := client.Execute(ctx, []invocation.Invocation{inv}, indexingService)
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

var _ Publisher = (*PublisherService)(nil)

// New creates a [Publisher] that publishes content claims/commitments to IPNI
// and caches them with the indexing service.
//
// The publicAddr parameter is the base public address where adverts and claims
// can be read from. When publishing, the address is suffixed with a
// /http-path/<path> multiaddr, where "path" is the URI encoded version of the
// configured claim path.
//
// Note: publicAddr address must be HTTP(S).
func New(
	id principal.Signer,
	publisherStore store.PublisherStore,
	publicAddr multiaddr.Multiaddr,
	opts ...Option,
) (*PublisherService, error) {
	o := &options{}
	for _, opt := range opts {
		err := opt(o)
		if err != nil {
			return nil, err
		}
	}

	priv, err := crypto.UnmarshalEd25519PrivateKey(id.Raw())
	if err != nil {
		return nil, fmt.Errorf("unmarshaling private key: %w", err)
	}

	announceAddr := o.announceAddr
	if announceAddr == nil {
		announceAddr = publicAddr
	}

	ipnipubOpts := []ipnipub.Option{ipnipub.WithAnnounceAddrs(announceAddr.String())}
	for _, u := range o.announceURLs {
		log.Infof("Announcing new IPNI adverts to: %s", u.String())
		ipnipubOpts = append(ipnipubOpts, ipnipub.WithDirectAnnounce(u.String()))
	}
	publisher, err := ipnipub.New(priv, publisherStore, ipnipubOpts...)
	if err != nil {
		return nil, fmt.Errorf("creating IPNI publisher instance: %w", err)
	}

	found := false
	for _, p := range publicAddr.Protocols() {
		if p.Code == multiaddr.P_HTTPS || p.Code == multiaddr.P_HTTP {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("IPNI publisher address is not HTTP(S): %s", publicAddr)
	}

	peerid, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("creating libp2p peer ID from private key: %w", err)
	}
	provInfo, err := providerInfo(peerid, publicAddr, o.blobAddr)
	if err != nil {
		return nil, fmt.Errorf("building provider info: %w", err)
	}

	if o.indexingService == nil {
		log.Errorf("Indexing service is not configured - claims will not be cached")
	}

	return &PublisherService{
		id:                    id,
		store:                 publisherStore,
		publisher:             publisher,
		provider:              provInfo,
		indexingService:       o.indexingService,
		indexingServiceProofs: o.indexingServiceProofs,
	}, nil
}

func providerInfo(peerID peer.ID, publicAddr multiaddr.Multiaddr, blobAddr multiaddr.Multiaddr) (peer.AddrInfo, error) {
	provider := peer.AddrInfo{ID: peerID}
	if blobAddr == nil {
		addr, err := lib.JoinHTTPPath(publicAddr, "blob/{blob}")
		if err != nil {
			return peer.AddrInfo{}, fmt.Errorf("joining blob pattern path to public multiaddr: %w", err)
		}
		blobAddr = addr
	}
	provider.Addrs = append(provider.Addrs, blobAddr)

	claimAddr, err := lib.JoinHTTPPath(publicAddr, "claim/{claim}")
	if err != nil {
		return peer.AddrInfo{}, fmt.Errorf("joining claim pattern path to public multiaddr: %w", err)
	}
	provider.Addrs = append(provider.Addrs, claimAddr)

	return provider, nil
}

func asCID(link ipld.Link) cid.Cid {
	if cl, ok := link.(cidlink.Link); ok {
		return cl.Cid
	}
	return cid.MustParse(link.String())
}
