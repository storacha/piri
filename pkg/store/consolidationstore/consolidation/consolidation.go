package consolidation

import (
	"bytes"
	"fmt"
	"io"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/fluent/qp"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
)

// Consolidation holds the track invocation and consolidate invocation CID
// for a batch of egress receipts.
type Consolidation struct {
	// TrackInvocation is the space/egress/track invocation that was sent to the
	// egress tracker service.
	TrackInvocation invocation.Invocation
	// ConsolidateInvocationCID is the CID of the space/egress/consolidate
	// invocation from the receipt's fork effect.
	ConsolidateInvocationCID cid.Cid
}

// ToIPLD encodes the Consolidation as an IPLD map node.
// The track invocation is archived to CAR bytes and stored as a bytes field.
func (c Consolidation) ToIPLD() (datamodel.Node, error) {
	trackBytes, err := io.ReadAll(c.TrackInvocation.Archive())
	if err != nil {
		return nil, fmt.Errorf("archiving track invocation: %w", err)
	}

	return qp.BuildMap(basicnode.Prototype.Map, 2, func(ma datamodel.MapAssembler) {
		qp.MapEntry(ma, "trackInvocation", qp.Bytes(trackBytes))
		qp.MapEntry(ma, "consolidateInvocationCID", qp.Link(cidlink.Link{Cid: c.ConsolidateInvocationCID}))
	})
}

// FromIPLD decodes a Consolidation from an IPLD map node.
func FromIPLD(n datamodel.Node) (Consolidation, error) {
	c := Consolidation{}

	// Extract track invocation bytes
	tn, err := n.LookupByString("trackInvocation")
	if err != nil {
		return Consolidation{}, fmt.Errorf("looking up trackInvocation: %w", err)
	}
	trackBytes, err := tn.AsBytes()
	if err != nil {
		return Consolidation{}, fmt.Errorf("reading trackInvocation bytes: %w", err)
	}
	inv, err := delegation.Extract(trackBytes)
	if err != nil {
		return Consolidation{}, fmt.Errorf("extracting invocation: %w", err)
	}
	c.TrackInvocation = inv

	// Extract consolidate CID
	cn, err := n.LookupByString("consolidateInvocationCID")
	if err != nil {
		return Consolidation{}, fmt.Errorf("looking up consolidateInvocationCID: %w", err)
	}
	lnk, err := cn.AsLink()
	if err != nil {
		return Consolidation{}, fmt.Errorf("reading consolidateInvocationCID link: %w", err)
	}
	cl, ok := lnk.(cidlink.Link)
	if !ok {
		return Consolidation{}, fmt.Errorf("consolidateInvocationCID is not a CID link")
	}
	c.ConsolidateInvocationCID = cl.Cid

	return c, nil
}

// Encode serializes a Consolidation to CBOR bytes.
func Encode(c Consolidation) ([]byte, error) {
	n, err := c.ToIPLD()
	if err != nil {
		return nil, fmt.Errorf("encoding to IPLD: %w", err)
	}
	var buf bytes.Buffer
	if err := dagcbor.Encode(n, &buf); err != nil {
		return nil, fmt.Errorf("encoding to CBOR: %w", err)
	}
	return buf.Bytes(), nil
}

// Decode deserializes a Consolidation from CBOR bytes.
func Decode(data []byte) (Consolidation, error) {
	nb := basicnode.Prototype.Map.NewBuilder()
	if err := dagcbor.Decode(nb, bytes.NewReader(data)); err != nil {
		return Consolidation{}, fmt.Errorf("decoding CBOR: %w", err)
	}
	return FromIPLD(nb.Build())
}

// Codec implements genericstore.Codec for Consolidation values.
type Codec struct{}

func (Codec) Encode(c Consolidation) ([]byte, error) {
	return Encode(c)
}

func (Codec) Decode(data []byte) (Consolidation, error) {
	return Decode(data)
}
