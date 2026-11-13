package projectionstats

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

func TestDurablePublicationCarriesCommittedTrade(t *testing.T) {
	ctx, repository := repository(t)
	trade := input(fixture("buy", "7"), 119)
	result, err := repository.Apply(ctx, trade)
	if err != nil || !result.Accepted {
		t.Fatalf("committed trade: %+v %v", result, err)
	}
	if _, err := repository.PublishDurable(ctx, nil, "stats.minute", 1); err != ErrInvalid {
		t.Fatalf("missing publisher: %v", err)
	}
	nc := internaltest.ConnectNATS(t, os.Getenv("L20_NATS_URL"))
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	stream := "STATS_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	subject := "stats.minute." + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := js.AddStream(&nats.StreamConfig{Name: stream, Subjects: []string{subject}, Storage: nats.FileStorage, MaxBytes: 1048576}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := js.DeleteStream(stream); err != nil {
			t.Error(err)
		}
	})
	publisher, err := natsdriver.NewPublisher(nc, stream)
	if err != nil {
		t.Fatal(err)
	}
	count, err := repository.PublishDurable(ctx, publisher, subject, 10)
	if err != nil || count != 1 {
		t.Fatalf("published trade: %d %v", count, err)
	}
	message, err := js.GetMsg(stream, 1)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := delivery.Decode(message.Data, delivery.Policy{Subjects: map[string]bool{subject: true}, EventTypes: map[string]bool{"stats.minute.v2": true}}, subject)
	if err != nil || envelope.EventType != "stats.minute.v2" || envelope.SourceOrder.Height != trade.Trade.Position.Height {
		t.Fatalf("published envelope: %+v %v", envelope, err)
	}
	var payload json.RawMessage
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil || len(payload) == 0 {
		t.Fatalf("published payload: %s %v", envelope.Payload, err)
	}
	count, err = repository.PublishDurable(ctx, publisher, subject, 10)
	if err != nil || count != 0 {
		t.Fatalf("republication: %d %v", count, err)
	}
	second := input(fixture("buy", "8"), 119)
	if result, err := repository.Apply(ctx, second); err != nil || !result.Accepted {
		t.Fatalf("second committed trade: %+v %v", result, err)
	}
	runCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	worker := DurablePublisher{Repository: repository, Publisher: publisher, Subject: subject, Batch: 10, AttemptTimeout: time.Second}
	if err := worker.Run(runCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("publisher stop: %v", err)
	}
	info, err := js.StreamInfo(stream)
	if err != nil || info.State.Msgs != 2 {
		t.Fatalf("published stream state: %+v %v", info, err)
	}
	if err := (DurablePublisher{}).Run(context.Background()); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid publisher: %v", err)
	}
}
