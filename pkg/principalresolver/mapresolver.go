package principalresolver

import (
	"context"
	"errors"

	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/validator"
)

type MapResolver struct {
	mapping map[did.DID]did.DID
}

var _ validator.PrincipalResolver = (*MapResolver)(nil)

func (r *MapResolver) ResolveDIDKey(_ context.Context, input did.DID) (did.DID, validator.UnresolvedDID) {
	// ctx is unused; this implementation only looks in a local mapping.
	dk, ok := r.mapping[input]
	if !ok {
		return did.Undef, validator.NewDIDKeyResolutionError(input, errors.New("not found in mapping"))
	}
	return dk, nil
}

func NewMapResolver(smap map[string]string) (*MapResolver, error) {
	dmap := map[did.DID]did.DID{}
	for k, v := range smap {
		dk, err := did.Parse(k)
		if err != nil {
			return nil, err
		}
		dv, err := did.Parse(v)
		if err != nil {
			return nil, err
		}
		dmap[dk] = dv
	}
	return &MapResolver{dmap}, nil
}
