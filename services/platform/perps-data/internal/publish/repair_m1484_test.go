package publish

import (
	"context"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/delivery"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/stream"
)

func TestRepairM_1484_EnvelopeBoundary(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, err := store.Open(testDatabase(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	policy := testPolicy(t)
	outboxStore, err := delivery.NewStore(db, "perps", policy)
	if err != nil {
		t.Fatal(err)
	}
	nc := startCluster(t)
	if err := EnsureStream(ctx, nc, chain); err != nil {
		t.Fatal(err)
	}
	perps, err := jsPub(t, nc).Stream(ctx, stream.Name)
	if err != nil {
		t.Fatal(err)
	}
	checkEnvelopeBoundary(t, ctx, db, nc, policy, outboxStore, perps)
}

func TestRepairM_1484_HeaderBudget(t *testing.T) {
	limit := configuredPayloadLimit(t)
	policy := testPolicy(t)
	e := padded(t, policy, 605, delivery.MaxEnvelopeBytes)
	s := &sink{policy: policy}
	msg, id, err := s.message(delivery.Lease{EventID: e.env.EventID, Subject: e.subject, Wire: wireOf(t, policy, e), PerpsSeq: math.MaxInt64})
	if err != nil || id != e.env.EventID {
		t.Fatalf("production publisher message identity %q: %v", id, err)
	}
	msg.Header.Set(jetstream.MsgIDHeader, id)
	msg.Header.Set("Nats-Expected-Stream", stream.Name)
	msg.Header.Set("Nats-Expected-Last-Sequence", strconv.FormatUint(math.MaxUint64, 10))
	if size := msg.Size() - len(msg.Subject); size > int(limit) {
		t.Fatalf("maximum admitted body with production headers = %d bytes, limit=%d", size, limit)
	}
	// Hold the reserved budget to the actual client's serialization at every
	// header field's maximum, including generic stream and identity tokens.
	maxima := &nats.Msg{Header: nats.Header{}}
	maxima.Header.Set(HeaderSeq, strconv.FormatInt(math.MaxInt64, 10))
	maxima.Header.Set(HeaderBlock, strconv.FormatInt(math.MaxInt64, 10))
	maxima.Header.Set(HeaderKind, strings.Repeat("k", 128))
	maxima.Header.Set(jetstream.MsgIDHeader, strings.Repeat("i", 128))
	maxima.Header.Set("Nats-Expected-Stream", strings.Repeat("s", 128))
	maxima.Header.Set("Nats-Expected-Last-Sequence", strconv.FormatUint(math.MaxUint64, 10))
	if got := maxima.Size(); got != delivery.MaxTransportHeaderBytes || got+delivery.MaxEnvelopeBytes != int(limit) {
		t.Fatalf("header reservation=%d actual=%d admission=%d broker=%d", delivery.MaxTransportHeaderBytes, got, delivery.MaxEnvelopeBytes, limit)
	}
}
