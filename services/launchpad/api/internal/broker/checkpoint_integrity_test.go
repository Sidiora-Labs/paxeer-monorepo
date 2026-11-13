package broker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/nats-io/nats.go/jetstream"
)

func TestWrittenCheckpointRejectsInvalidProgressAndKeepsCurrentToken(t *testing.T) {
	ctx, stream := postgresStreamStorage(t)
	first := publicPriceFrame(t, "1")
	commitStreamFrame(t, ctx, stream, first)
	cursor := snapshotCursor(t, ctx, stream, []string{first.Topic})
	second := publicPriceFrame(t, "2")
	commitStreamFrame(t, ctx, stream, second)
	replay, err := stream.PublicResume(ctx, []string{first.Topic}, cursor)
	if err != nil {
		t.Fatal(err)
	}
	defer replay.Close()
	tracker := replay.WrittenCheckpoints()
	initial, err := tracker.Token()
	if err != nil || initial == "" {
		t.Fatalf("initial written token: %q, %v", initial, err)
	}
	batch, err := replay.DeliveryBatch(ctx)
	if err != nil || len(batch) != 1 {
		t.Fatalf("replay deliveries: %+v, %v", batch, err)
	}
	advanced, err := tracker.Advance(batch[0].Frame)
	if err != nil || advanced == "" {
		t.Fatalf("written delivery token: %q, %v", advanced, err)
	}
	if _, err := tracker.Advance(batch[0].Frame); !errors.Is(err, ErrSequence) {
		t.Fatalf("duplicate delivery advanced cursor: %v", err)
	}
	foreign := second
	foreign.Topic = "market.price.0x3333333333333333333333333333333333333333"
	var payload map[string]any
	if err := json.Unmarshal(foreign.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	payload["baseAsset"].(map[string]any)["address"] = "0x3333333333333333333333333333333333333333"
	foreign.Payload, err = json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(foreign)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseStreamV2(raw); err != nil {
		t.Fatalf("unrequested frame is invalid: %v", err)
	}
	if _, err := tracker.Advance(raw); !errors.Is(err, ErrFrame) {
		t.Fatalf("unrequested topic advanced cursor: %v", err)
	}
	if _, err := tracker.Advance([]byte(`{"not":"a-frame"}`)); !errors.Is(err, ErrFrame) {
		t.Fatalf("malformed frame advanced cursor: %v", err)
	}
	current, err := tracker.Token()
	if err != nil || current == initial {
		t.Fatalf("written cursor did not retain progress: %q, %v", current, err)
	}
	parsed, err := VerifyCursorV2(current, stream.key, replay.Cursor.Scope, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, offset := range parsed.Offsets {
		if offset != "2" {
			t.Fatalf("written sequence = %s, want 2", offset)
		}
	}
}

func TestCommittedConsumerRejectsInsufficientStreamDurability(t *testing.T) {
	ctx, stream := postgresStreamStorage(t)
	connection, js := committedBroker(t)
	if _, err := js.CreateStream(ctx, jetstream.StreamConfig{Name: "API_STREAM", Subjects: []string{"market.price.>"}, Storage: jetstream.FileStorage, Replicas: 1, MaxAge: 72 * time.Hour}); err != nil {
		t.Fatal(err)
	}
	config := natsdriver.ConsumerConfig{Stream: "API_STREAM", Durable: "projection_v1", Subject: "market.price.0x1111111111111111111111111111111111111111", Batch: 1, MaxAckPending: 4, MaxWaiting: 1, MaxDeliver: 3, FetchWait: 100 * time.Millisecond, AckWait: time.Second}
	fanout := NewTopicFanout()
	if err := stream.RunCommitted(context.Background(), connection, config, fanout); !errors.Is(err, ErrFrame) || !fanout.Degraded() {
		t.Fatalf("stream durability admission: degraded=%t err=%v", fanout.Degraded(), err)
	}
}

func TestStreamFrameCoalescingAndRevisionBounds(t *testing.T) {
	first, last := "1", "2"
	for _, tc := range []struct {
		name   string
		change func(*Frame)
		valid  bool
	}{
		{"ordinary frame", func(*Frame) {}, true},
		{"coalesced frame", func(f *Frame) { f.Coalesced = true; f.First = &first; f.Last = &last }, true},
		{"missing coalesced start", func(f *Frame) { f.Coalesced = true; f.Last = &last }, false},
		{"coalesced end differs from sequence", func(f *Frame) { f.Coalesced = true; f.First = &first; f.Last = &first }, false},
		{"coalesced range reversed", func(f *Frame) { f.Coalesced = true; f.First = &last; f.Last = &first }, false},
		{"uncoalesced range", func(f *Frame) { f.First = &first; f.Last = &last }, false},
		{"noncanonical revision", func(f *Frame) { f.Revision = "02" }, false},
		{"invalid watermark", func(f *Frame) { f.Watermark = json.RawMessage(`{"integrity":"unhealthy"}`) }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frame := publicPriceFrame(t, "2")
			tc.change(&frame)
			raw, err := json.Marshal(frame)
			if err != nil {
				t.Fatal(err)
			}
			got, err := parseStreamV2(raw)
			if tc.valid {
				if err != nil || got.Sequence != "2" || got.Topic != frame.Topic || got.Coalesced != frame.Coalesced {
					t.Fatalf("accepted frame: %+v, %v", got, err)
				}
			} else if !errors.Is(err, ErrFrame) {
				t.Fatalf("invalid frame: %+v, %v", got, err)
			}
		})
	}
}
