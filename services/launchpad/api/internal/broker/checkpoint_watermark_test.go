package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
)

func TestWrittenCheckpointKeepsBoundedConsistentBlockHistory(t *testing.T) {
	ctx, stream := postgresStreamStorage(t)
	first := publicPriceFrame(t, "1")
	commitStreamFrame(t, ctx, stream, first)
	token := snapshotCursor(t, ctx, stream, []string{first.Topic})
	replay, err := stream.PublicResume(ctx, []string{first.Topic}, token)
	if err != nil {
		t.Fatal(err)
	}
	defer replay.Close()
	tracker := replay.WrittenCheckpoints()
	for sequence := 2; sequence < 1028; sequence++ {
		frame := publicPriceFrame(t, fmt.Sprint(sequence))
		frame.Watermark = json.RawMessage(fmt.Sprintf(`{"chainId":"125","height":"%d","blockHash":"0x%064x","projectionRevision":"%d","observedAt":"2026-10-09T00:00:00.000Z","integrity":"healthy"}`, sequence, sequence, sequence))
		raw, err := json.Marshal(frame)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tracker.Advance(raw); err != nil {
			t.Fatalf("sequence %d: %v", sequence, err)
		}
	}
	if len(tracker.hashes) != 1024 || len(tracker.heights) != 1024 {
		t.Fatalf("block history: %d %d", len(tracker.hashes), len(tracker.heights))
	}
	if _, retained := tracker.hashes["2"]; retained {
		t.Fatal("expired block retained")
	}
	conflict := publicPriceFrame(t, "1028")
	conflict.Watermark = json.RawMessage(`{"chainId":"125","height":"1027","blockHash":"0x` + strings.Repeat("f", 64) + `","projectionRevision":"1028","observedAt":"2026-10-09T00:00:00.000Z","integrity":"healthy"}`)
	raw, err := json.Marshal(conflict)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tracker.Advance(raw); !errors.Is(err, ErrFrame) {
		t.Fatalf("conflicting block accepted: %v", err)
	}
}

func TestTopicSetRequiresCurrentIdentityForPrivateTopics(t *testing.T) {
	topic := "private.notifications.0x" + strings.Repeat("1", 40)
	binding := sharedauth.StreamTicketBinding{Topics: []string{topic}, SchemaMajor: 2, Origin: "https://web.example", Endpoint: "/v2/stream"}
	set := &TopicSet{}
	principal := sharedauth.StreamPrincipal{AccountID: "account"}
	if _, err := set.Replace(context.Background(), principal, binding, nil); !errors.Is(err, sharedauth.ErrScopeDenied) {
		t.Fatalf("missing authority: %v", err)
	}
	if _, err := set.Replace(context.Background(), principal, binding, sharedauth.HTTPStreamAuthority{}); err == nil {
		t.Fatal("unavailable session authority admitted")
	}
	for _, topics := range [][]string{nil, {"market.price.*"}, {strings.Repeat("x", 201)}, {topic, topic}} {
		if _, err := CanonicalTopics(topics); err == nil {
			t.Fatalf("invalid topic set: %v", topics)
		}
	}
}
