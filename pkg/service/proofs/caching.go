package proofs

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/storacha/go-libstoracha/capabilities/access"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/did"
	ucan_http "github.com/storacha/go-ucanto/transport/http"
	"github.com/storacha/go-ucanto/ucan"
)

// defaultMinTTL is the minimum time a cached delegation should still be valid
// for before it expires. If the TTL is less than this then consider it expired.
var defaultMinTTL = time.Second * 5

type CachingProofService struct {
	cache      map[did.DID]map[ucan.Ability]delegation.Delegation
	cacheMutex sync.RWMutex
}

func NewCachingProofService() *CachingProofService {
	service := CachingProofService{
		cache: map[did.DID]map[ucan.Ability]delegation.Delegation{},
	}
	return &service
}

// Request access to be granted from the service for the passed ability.
// A cached delegation may be returned if not expired.
func (ps *CachingProofService) RequestAccess(
	ctx context.Context,
	issuer ucan.Signer,
	audience ucan.Principal,
	ability ucan.Ability,
	cause invocation.Invocation,
	options ...Option,
) (delegation.Delegation, error) {
	cfg := requestConfig{}
	for _, opt := range options {
		opt(&cfg)
	}

	conn := cfg.conn
	if conn == nil {
		serviceURL := cfg.url
		if serviceURL == nil {
			if strings.HasPrefix(audience.DID().String(), "did:web:") {
				u, err := url.Parse("https://" + strings.TrimPrefix(audience.DID().String(), "did:web:"))
				if err != nil {
					return nil, err
				}
				serviceURL = u
			} else {
				return nil, errors.New("non-did web audience and no service URL provided")
			}
		}
		ch := ucan_http.NewChannel(serviceURL, ucan_http.WithClient(cfg.httpClient))
		c, err := client.NewConnection(audience, ch)
		if err != nil {
			return nil, fmt.Errorf("creating connection to %s: %w", audience.DID(), err)
		}
		conn = c
	}

	ps.cacheMutex.RLock()
	serviceProofs, ok := ps.cache[audience.DID()]
	if ok {
		d, ok := serviceProofs[ability]
		if ok {
			exp := d.Expiration()
			// if no expiration we can reuse
			if exp == nil {
				ps.cacheMutex.RUnlock()
				return d, nil
			}
			minTTL := defaultMinTTL
			if cfg.minTTL != nil {
				minTTL = *cfg.minTTL
			}
			// if not expired, we can reuse
			if ucan.Now()+ucan.UTCUnixTimestamp(minTTL.Seconds()) < *exp {
				ps.cacheMutex.RUnlock()
				return d, nil
			}
		}
	}
	ps.cacheMutex.RUnlock()

	// if not in cache we need to fetch
	d, err := requestDelegation(ctx, conn, issuer, audience, ability, cause)
	if err != nil {
		return nil, fmt.Errorf("requesting %s access from %s", ability, audience)
	}

	ps.cacheMutex.Lock()
	serviceProofs, ok = ps.cache[audience.DID()]
	if !ok {
		serviceProofs = map[ucan.Ability]delegation.Delegation{}
		ps.cache[audience.DID()] = serviceProofs
	}
	serviceProofs[ability] = d
	ps.cacheMutex.Unlock()

	return d, nil
}

func requestDelegation(
	ctx context.Context,
	conn client.Connection,
	issuer ucan.Signer,
	audience ucan.Principal,
	ability ucan.Ability,
	cause invocation.Invocation,
) (delegation.Delegation, error) {
	nb := access.GrantCaveats{Att: []access.CapabilityRequest{{Can: ability}}}
	if cause != nil {
		nb.Cause = cause.Link()
	}
	inv, err := access.Grant.Invoke(issuer, audience, issuer.DID().String(), nb)
	if err != nil {
		return nil, fmt.Errorf("creating %s (%s) invocation: %w", access.GrantAbility, ability, err)
	}
	if cause != nil {
		for b, err := range cause.Export() {
			if err != nil {
				return nil, fmt.Errorf("exporting cause blocks for %s (%s) invocation: %w", access.GrantAbility, ability, err)
			}
			if err = inv.Attach(b); err != nil {
				return nil, fmt.Errorf("attaching cause blocks for %s (%s): %w", access.GrantAbility, ability, err)
			}
		}
	}

	resp, err := client.Execute(ctx, []invocation.Invocation{inv}, conn)
	if err != nil {
		return nil, fmt.Errorf("executing %s (%s) invocation: %w", access.GrantAbility, ability, err)
	}

	rcptLink, ok := resp.Get(inv.Link())
	if !ok {
		return nil, fmt.Errorf("missing %s receipt: %s", access.GrantAbility, inv.Link())
	}

	rcptReader, err := access.NewGrantReceiptReader()
	if err != nil {
		return nil, err
	}

	rcpt, err := rcptReader.Read(rcptLink, resp.Blocks())
	if err != nil {
		return nil, fmt.Errorf("reading %s receipt: %w", access.GrantAbility, err)
	}

	return result.MatchResultR2(
		rcpt.Out(),
		func(o access.GrantOk) (delegation.Delegation, error) {
			dlgBytes := o.Delegations.Values[o.Delegations.Keys[0]]
			return delegation.Extract(dlgBytes)
		},
		func(x access.GrantError) (delegation.Delegation, error) {
			return nil, x
		},
	)
}
