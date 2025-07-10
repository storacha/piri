package principalresolver

import (
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/validator"
)

type CachedResolver struct {
	wrapped validator.PrincipalResolver
	cache   *cache.Cache
}

func NewCachedResolver(wrapped validator.PrincipalResolver, ttl time.Duration) (*CachedResolver, error) {
	// items remain in the cache for `ttl`, expired items are purged every hour.
	return &CachedResolver{wrapped: wrapped, cache: cache.New(ttl, time.Hour)}, nil
}

func (c *CachedResolver) ResolveDIDKey(input did.DID) (did.DID, validator.UnresolvedDID) {
	if out, found := c.cache.Get(input.String()); found {
		return out.(did.DID), nil
	}
	out, err := c.wrapped.ResolveDIDKey(input)
	if err != nil {
		return did.Undef, err
	}
	c.cache.Set(input.String(), out, cache.DefaultExpiration)

	return out, nil
}
