package retrievaljournal_test

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/stretchr/testify/require"

	"github.com/storacha/piri/pkg/store/local/retrievaljournal"
)

func TestPeriodicRotator(t *testing.T) {
	batches := []cid.Cid{
		testutil.RandomCID(t).(cidlink.Link).Cid,
		cid.Undef, // signal for no rotation due to empty batch
		testutil.RandomCID(t).(cidlink.Link).Cid,
		testutil.RandomCID(t).(cidlink.Link).Cid,
		testutil.RandomCID(t).(cidlink.Link).Cid,
		cid.Undef,
		testutil.RandomCID(t).(cidlink.Link).Cid,
		testutil.RandomCID(t).(cidlink.Link).Cid,
	}
	i := 0
	rj := mockRetrievalJournal{
		forceRotateFunc: func() (bool, cid.Cid, error) {
			if i >= len(batches) {
				return false, cid.Undef, nil
			}
			batch := batches[i]
			rotated := batch != cid.Undef
			i++
			return rotated, batch, nil
		},
	}
	pr := retrievaljournal.NewPeriodicRotator(&rj, time.Millisecond)

	// collect the rotation batches
	actualBatches := []cid.Cid{}
	pr.RotateFunc = func(batchID cid.Cid) {
		actualBatches = append(actualBatches, batchID)
		t.Logf("Rotated batch: %s", batchID)
	}

	pr.Start()
	time.Sleep(30 * time.Millisecond) // allow some time for rotations to occur
	err := pr.Stop(t.Context())
	require.NoError(t, err)

	var expectedBatches []cid.Cid
	for _, batch := range batches {
		if batch != cid.Undef {
			expectedBatches = append(expectedBatches, batch)
		}
	}
	require.Equal(t, actualBatches, expectedBatches)
}

type mockRetrievalJournal struct {
	forceRotateFunc func() (bool, cid.Cid, error)
}

func (m *mockRetrievalJournal) ForceRotate(ctx context.Context) (bool, cid.Cid, error) {
	return m.forceRotateFunc()
}
