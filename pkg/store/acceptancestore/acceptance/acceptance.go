package acceptance

import (
	"bytes"
	"errors"

	// for go:embed
	_ "embed"
	"fmt"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/fluent/qp"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/ucan"
)

type Acceptance struct {
	// Space is the DID of the space this data was accepted for.
	Space did.DID
	// Blob is the details of the data that was accepted.
	Blob Blob
	// PDPAccept is the promise of the `pdp/accept` task completion.
	PDPAccept *Promise
	// ExecutedAt is the approximate time (in seconds since unix epoch) that the
	// `blob/accept` invocation was executed.
	ExecutedAt uint64
	// Cause is a link to the `blob/accept` invocation requested the acceptance.
	Cause ucan.Link
}

func (a Acceptance) ToIPLD() (datamodel.Node, error) {
	sizeHint := 4
	if a.PDPAccept != nil {
		sizeHint = 5
	}
	var acceptNode datamodel.Node
	if a.PDPAccept != nil {
		nd, err := a.PDPAccept.ToIPLD()
		if err != nil {
			return nil, err
		}
		acceptNode = nd
	}
	blobNode, err := a.Blob.ToIPLD()
	if err != nil {
		return nil, err
	}
	return qp.BuildMap(basicnode.Prototype.Map, int64(sizeHint), func(ma datamodel.MapAssembler) {
		qp.MapEntry(ma, "space", qp.Bytes(a.Space.Bytes()))
		qp.MapEntry(ma, "blob", qp.Node(blobNode))
		if acceptNode != nil {
			qp.MapEntry(ma, "pdpAccept", qp.Node(acceptNode))
		}
		qp.MapEntry(ma, "executedAt", qp.Int(int64(a.ExecutedAt)))
		qp.MapEntry(ma, "cause", qp.Link(a.Cause))
	})
}

func Encode(acceptance Acceptance, enc codec.Encoder) ([]byte, error) {
	n, err := acceptance.ToIPLD()
	if err != nil {
		return nil, fmt.Errorf("encoding to IPLD: %w", err)
	}
	if enc == nil {
		enc = dagcbor.Encode
	}
	var buf bytes.Buffer
	err = enc(n, &buf)
	if err != nil {
		return nil, fmt.Errorf("encoding to data format: %w", err)
	}
	return buf.Bytes(), nil
}

func Decode(data []byte, dec codec.Decoder) (Acceptance, error) {
	if dec == nil {
		dec = dagcbor.Decode
	}
	np := basicnode.Prototype.Map
	nb := np.NewBuilder()
	err := dec(nb, bytes.NewReader(data))
	if err != nil {
		return Acceptance{}, fmt.Errorf("decoding CBOR: %w", err)
	}
	return FromIPLD(nb.Build())
}

func FromIPLD(n datamodel.Node) (Acceptance, error) {
	a := Acceptance{}

	sn, err := n.LookupByString("space")
	if err != nil {
		return Acceptance{}, fmt.Errorf("looking up space key: %w", err)
	}
	spaceBytes, err := sn.AsBytes()
	if err != nil {
		return Acceptance{}, fmt.Errorf("reading space bytes: %w", err)
	}
	space, err := did.Decode(spaceBytes)
	if err != nil {
		return Acceptance{}, fmt.Errorf("decoding space bytes: %w", err)
	}
	a.Space = space

	bn, err := n.LookupByString("blob")
	if err != nil {
		return Acceptance{}, fmt.Errorf("looking up blob key: %w", err)
	}
	blob := Blob{}
	if err := blob.FromIPLD(bn); err != nil {
		return Acceptance{}, fmt.Errorf("decoding blob: %w", err)
	}
	a.Blob = blob

	an, err := n.LookupByString("pdpAccept")
	if err == nil {
		pdpAccept := Promise{}
		if err := pdpAccept.FromIPLD(an); err != nil {
			return Acceptance{}, fmt.Errorf("decoding PDP accept promise: %w", err)
		}
		a.PDPAccept = &pdpAccept
	}

	en, err := n.LookupByString("executedAt")
	if err != nil {
		return Acceptance{}, fmt.Errorf("looking up executedAt key: %w", err)
	}
	executedAt, err := en.AsInt()
	if err != nil {
		return Acceptance{}, fmt.Errorf("reading executed at integer: %w", err)
	}
	if executedAt < 0 {
		return Acceptance{}, errors.New("negative executed at value")
	}
	a.ExecutedAt = uint64(executedAt)

	cn, err := n.LookupByString("cause")
	if err != nil {
		return Acceptance{}, fmt.Errorf("looking up cause key: %w", err)
	}
	cause, err := cn.AsLink()
	if err != nil {
		return Acceptance{}, fmt.Errorf("reading cause link: %w", err)
	}
	a.Cause = cause

	return a, nil
}

type Blob struct {
	Digest multihash.Multihash
	Size   uint64
}

func (b Blob) ToIPLD() (datamodel.Node, error) {
	return qp.BuildMap(basicnode.Prototype.Map, 2, func(ma datamodel.MapAssembler) {
		qp.MapEntry(ma, "digest", qp.Bytes(b.Digest))
		qp.MapEntry(ma, "size", qp.Int(int64(b.Size)))
	})
}

func (b *Blob) FromIPLD(n datamodel.Node) error {
	dn, err := n.LookupByString("digest")
	if err != nil {
		return fmt.Errorf("looking up digest key: %w", err)
	}
	digestBytes, err := dn.AsBytes()
	if err != nil {
		return fmt.Errorf("reading digest bytes: %w", err)
	}
	_, digest, err := multihash.MHFromBytes(digestBytes)
	if err != nil {
		return fmt.Errorf("decoding digest multihash: %w", err)
	}
	b.Digest = digest

	sn, err := n.LookupByString("size")
	if err != nil {
		return fmt.Errorf("looking up size key: %w", err)
	}
	size, err := sn.AsInt()
	if err != nil {
		return fmt.Errorf("reading size integer: %w", err)
	}
	b.Size = uint64(size)

	return nil
}

type Await struct {
	Selector string
	Link     ipld.Link
}

func (a Await) ToIPLD() (datamodel.Node, error) {
	return qp.BuildList(basicnode.Prototype.List, 2, func(la datamodel.ListAssembler) {
		qp.ListEntry(la, qp.String(a.Selector))
		qp.ListEntry(la, qp.Link(a.Link))
	})
}

func (a *Await) FromIPLD(n datamodel.Node) error {
	sn, err := n.LookupByIndex(0)
	if err != nil {
		return fmt.Errorf("looking up selector at index 0: %w", err)
	}
	selector, err := sn.AsString()
	if err != nil {
		return fmt.Errorf("reading selector string: %w", err)
	}
	a.Selector = selector

	ln, err := n.LookupByIndex(1)
	if err != nil {
		return fmt.Errorf("looking up link at index 1: %w", err)
	}
	link, err := ln.AsLink()
	if err != nil {
		return fmt.Errorf("reading link: %w", err)
	}
	a.Link = link

	return nil
}

type Promise struct {
	UcanAwait Await
}

func (p Promise) ToIPLD() (datamodel.Node, error) {
	awaitNode, err := p.UcanAwait.ToIPLD()
	if err != nil {
		return nil, err
	}
	return qp.BuildMap(basicnode.Prototype.Map, 1, func(ma datamodel.MapAssembler) {
		qp.MapEntry(ma, "ucan/await", qp.Node(awaitNode))
	})
}

func (p *Promise) FromIPLD(n datamodel.Node) error {
	un, err := n.LookupByString("ucan/await")
	if err != nil {
		return fmt.Errorf("looking up ucan/await key: %w", err)
	}
	await := Await{}
	if err := await.FromIPLD(un); err != nil {
		return fmt.Errorf("decoding await: %w", err)
	}
	p.UcanAwait = await

	return nil
}
