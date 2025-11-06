package dedup_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/storacha/piri/lib/jobqueue/dedup"
	"github.com/storacha/piri/lib/jobqueue/queue"
	"github.com/storacha/piri/pkg/database/sqlitedb"
)

type envelope struct {
	Name    string `json:"Name"`
	Message []byte `json:"Message"`
}

func newTestQueue(t *testing.T, opts dedup.NewOpts) (*dedup.Queue, context.Context) {
	t.Helper()

	db := opts.DB
	if db == nil {
		var err error
		db, err = sqlitedb.NewMemory()
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = db.Close()
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	require.NoError(t, dedup.Setup(ctx, db))

	opts.DB = db
	if opts.Name == "" {
		opts.Name = "test"
	}
	q, err := dedup.New(opts)
	require.NoError(t, err)
	return q, context.Background()
}

func encodeEnvelope(t *testing.T, name string, payload []byte) []byte {
	t.Helper()
	b, err := json.Marshal(envelope{Name: name, Message: payload})
	require.NoError(t, err)
	return b
}

func TestQueue_DedupSkipsCompletedPayloads(t *testing.T) {
	q, ctx := newTestQueue(t, dedup.NewOpts{})

	body := encodeEnvelope(t, "job", []byte("payload"))
	id, err := q.SendAndGetID(ctx, queue.Message{Body: body})
	require.NoError(t, err)
	require.NotEmpty(t, id)

	msg, err := q.Receive(ctx)
	require.NoError(t, err)
	require.NotNil(t, msg)
	require.Equal(t, id, msg.ID)
	require.Equal(t, 1, msg.Received)

	require.NoError(t, q.Delete(ctx, msg.ID))

	dupID, err := q.SendAndGetID(ctx, queue.Message{Body: body})
	require.NoError(t, err)
	require.Empty(t, dupID, "duplicate payload should be skipped")

	next, err := q.Receive(ctx)
	require.NoError(t, err)
	require.Nil(t, next, "no job should be available after dedupe skip")
}

func TestQueue_DedupDisabledAllowsReenqueue(t *testing.T) {
	enabled := false
	q, ctx := newTestQueue(t, dedup.NewOpts{
		DedupeEnabled: &enabled,
	})

	body := encodeEnvelope(t, "job", []byte("payload"))
	_, err := q.SendAndGetID(ctx, queue.Message{Body: body})
	require.NoError(t, err)

	msg, err := q.Receive(ctx)
	require.NoError(t, err)
	require.NotNil(t, msg)

	require.NoError(t, q.Delete(ctx, msg.ID))

	_, err = q.SendAndGetID(ctx, queue.Message{Body: body})
	require.NoError(t, err)

	msg2, err := q.Receive(ctx)
	require.NoError(t, err)
	require.NotNil(t, msg2, "payload should be delivered again when dedupe disabled")
}

func TestQueue_MoveToDeadLetterBlocksWhenConfigured(t *testing.T) {
	q, ctx := newTestQueue(t, dedup.NewOpts{})

	body := encodeEnvelope(t, "job", []byte("payload"))
	_, err := q.SendAndGetID(ctx, queue.Message{Body: body})
	require.NoError(t, err)

	msg, err := q.Receive(ctx)
	require.NoError(t, err)
	require.NotNil(t, msg)

	require.NoError(t, q.MoveToDeadLetter(ctx, msg.ID, "job", "failed", "boom"))

	dupID, err := q.SendAndGetID(ctx, queue.Message{Body: body})
	require.NoError(t, err)
	require.Empty(t, dupID, "payload should remain blocked after DLQ move when blocking enabled")
}

func TestQueue_MoveToDeadLetterAllowsReenqueueWhenBlockingDisabled(t *testing.T) {
	block := false
	q, ctx := newTestQueue(t, dedup.NewOpts{
		BlockRepeatsOnDLQ: &block,
	})

	body := encodeEnvelope(t, "job", []byte("payload"))
	_, err := q.SendAndGetID(ctx, queue.Message{Body: body})
	require.NoError(t, err)

	msg, err := q.Receive(ctx)
	require.NoError(t, err)
	require.NotNil(t, msg)

	require.NoError(t, q.MoveToDeadLetter(ctx, msg.ID, "job", "failed", "boom"))

	_, err = q.SendAndGetID(ctx, queue.Message{Body: body})
	require.NoError(t, err)

	next, err := q.Receive(ctx)
	require.NoError(t, err)
	require.NotNil(t, next, "payload should be allowed when DLQ blocking disabled")
}

func TestQueue_DuplicateWhileInFlight(t *testing.T) {
	q, ctx := newTestQueue(t, dedup.NewOpts{})

	body := encodeEnvelope(t, "job", []byte("payload"))

	id, err := q.SendAndGetID(ctx, queue.Message{Body: body})
	require.NoError(t, err)
	require.NotEmpty(t, id)

	dupID, err := q.SendAndGetID(ctx, queue.Message{Body: body})
	require.NoError(t, err, "second enqueue of in-flight payload should not error")
	require.Empty(t, dupID, "second enqueue should be ignored while job is in-flight")

	received, err := q.Receive(ctx)
	require.NoError(t, err)
	require.NotNil(t, received)
	require.Equal(t, id, received.ID)

	next, err := q.Receive(ctx)
	require.NoError(t, err)
	require.Nil(t, next, "only one job should be present for duplicated payload")
}

func TestQueue_DedupeScopedPerJobName(t *testing.T) {
	q, ctx := newTestQueue(t, dedup.NewOpts{})

	payload := []byte("shared-payload")
	bodyA := encodeEnvelope(t, "job-a", payload)
	bodyB := encodeEnvelope(t, "job-b", payload)

	idA, err := q.SendAndGetID(ctx, queue.Message{Body: bodyA})
	require.NoError(t, err)
	require.NotEmpty(t, idA)

	idB, err := q.SendAndGetID(ctx, queue.Message{Body: bodyB})
	require.NoError(t, err)
	require.NotEmpty(t, idB, "same payload in different namespace should enqueue")

	msg1, err := q.Receive(ctx)
	require.NoError(t, err)
	require.NotNil(t, msg1)

	msg2, err := q.Receive(ctx)
	require.NoError(t, err)
	require.NotNil(t, msg2, "both namespaces should deliver a job")
	require.NotEqual(t, msg1.ID, msg2.ID)
}

func TestQueue_InvalidEnvelopeRejected(t *testing.T) {
	q, ctx := newTestQueue(t, dedup.NewOpts{})

	// Missing Name field
	badBody, err := json.Marshal(struct {
		Message []byte
	}{
		Message: []byte("payload"),
	})
	require.NoError(t, err)

	_, err = q.SendAndGetID(ctx, queue.Message{Body: badBody})
	require.Error(t, err)
	require.Contains(t, err.Error(), "message envelope missing name")

	// Not JSON at all
	_, err = q.SendAndGetID(ctx, queue.Message{Body: []byte("not-json")})
	require.Error(t, err)
	require.Contains(t, err.Error(), "decode message envelope")
}
