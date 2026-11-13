package broker

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/constants"
)

func TestResumeMachinesRejectInvalidTransitions(t *testing.T) {
	m := NewResumeMachine()
	m.Authenticate(false)
	if m.State != Closed {
		t.Fatalf("rejected identity state: %s", m.State)
	}
	m.Authenticate(true)
	if m.State != Closed {
		t.Fatal("closed subscription reopened")
	}
	if err := m.Begin("0", "1"); !errors.Is(err, ErrCursor) || m.State != ResyncRequired {
		t.Fatalf("unauthorized replay: %s %v", m.State, err)
	}
	m = NewResumeMachine()
	m.Authenticate(true)
	if err := m.Begin("1", "2"); err != nil {
		t.Fatal(err)
	}
	if err := m.Complete(); !errors.Is(err, ErrCursor) || m.State != ResyncRequired {
		t.Fatalf("incomplete replay became live: %s %v", m.State, err)
	}
	m.Close()
	if m.State != Closed {
		t.Fatalf("closed replay: %s", m.State)
	}
	for _, tc := range []struct{ snapshot, barrier map[string]string }{
		{map[string]string{"p": "1"}, map[string]string{"q": "2"}},
		{map[string]string{"p": "3"}, map[string]string{"p": "2"}},
		{map[string]string{"p": "01"}, map[string]string{"p": "2"}},
	} {
		if got, err := NewPartitionReplay(tc.snapshot, tc.barrier); got != nil || !errors.Is(err, ErrCursor) {
			t.Fatalf("invalid partition boundary: %+v %v", got, err)
		}
	}
	partitions, err := NewPartitionReplay(map[string]string{"p": "1"}, map[string]string{"p": "2"})
	if err != nil {
		t.Fatal(err)
	}
	if err := partitions.Replay("unknown", "2", 1); !errors.Is(err, ErrCursor) || partitions.State != ResyncRequired {
		t.Fatalf("unknown partition: %s %v", partitions.State, err)
	}
	if err := partitions.Complete(); !errors.Is(err, ErrCursor) {
		t.Fatalf("invalid replay completed: %v", err)
	}
}

func TestStreamCursorKeyAndOffsetBounds(t *testing.T) {
	now := time.Unix(1700000000, 0)
	key := []byte(strings.Repeat("k", 32))
	legacy := Cursor{Scope: CursorScope{Chain: "125", Subject: "public", PermissionEpoch: "1", ProjectionEpoch: "1", Schema: "1", FilterHash: "filter"}, Offsets: map[string]string{"p": "1"}, IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Hour).Unix()}
	v2 := CursorV2{FormatVersion: 1, Scope: CursorScopeV2{Chain: "125", Subject: "public", SessionID: "session", PermissionEpoch: "1", ProjectionEpoch: "1", SchemaMajor: 2, FilterHash: "filter", SnapshotID: "snapshot"}, Offsets: map[string]string{"p": "1"}, IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Hour).Unix()}
	if _, err := SignCursor(legacy, key[:31], now); !errors.Is(err, ErrCursor) {
		t.Fatalf("short legacy key: %v", err)
	}
	if _, err := SignCursorV2(v2, key[:31], now); !errors.Is(err, ErrCursor) {
		t.Fatalf("short v2 key: %v", err)
	}
	for _, offsets := range []map[string]string{{"": "1"}, {"p": "01"}, {strings.Repeat("p", 201): "1"}} {
		old, current := legacy, v2
		old.Offsets, current.Offsets = offsets, offsets
		if _, err := SignCursor(old, key, now); !errors.Is(err, ErrCursor) {
			t.Fatalf("legacy offset admitted: %v", err)
		}
		if _, err := SignCursorV2(current, key, now); !errors.Is(err, ErrCursor) {
			t.Fatalf("v2 offset admitted: %v", err)
		}
	}
	for _, change := range []func(*CursorV2){
		func(c *CursorV2) { c.Scope.SessionID = "" },
		func(c *CursorV2) { c.Scope.PermissionEpoch = "01" },
	} {
		c := v2
		change(&c)
		if _, err := SignCursorV2(c, key, now); !errors.Is(err, ErrCursor) {
			t.Fatalf("invalid v2 scope admitted: %v", err)
		}
	}
	for _, token := range []string{"", "x", "x.!", "x.x", strings.Repeat("x", 2049)} {
		if _, err := VerifyCursor(token, key, legacy.Scope, now); !errors.Is(err, ErrCursor) {
			t.Fatalf("legacy token admitted: %q %v", token, err)
		}
		if _, err := VerifyCursorV2(token, key, v2.Scope, now); !errors.Is(err, ErrCursor) {
			t.Fatalf("v2 token admitted: %q %v", token, err)
		}
	}
	large := map[string]string{}
	for i := 0; i < 20; i++ {
		large[strings.Repeat("p", 190)+string(rune('a'+i))] = "1"
	}
	legacy.Offsets, v2.Offsets = large, large
	if token, err := SignCursor(legacy, key, now); token != "" || !errors.Is(err, ErrCursor) {
		t.Fatalf("oversized legacy token: %q %v", token, err)
	}
	if token, err := SignCursorV2(v2, key, now); token != "" || !errors.Is(err, ErrCursor) {
		t.Fatalf("oversized v2 token: %q %v", token, err)
	}
}

func TestFrameAndTransformRejectMalformedRepresentations(t *testing.T) {
	for _, raw := range [][]byte{[]byte(`{} {}`), []byte(`[]`), []byte(`{"topic":1}`)} {
		if err := validateFrozenFrame(raw); !errors.Is(err, ErrFrame) {
			t.Fatalf("malformed frame accepted: %s %v", raw, err)
		}
	}
	if _, err := ParseFrame("2", testFixtures(t)["CandleEvent"]); !errors.Is(err, ErrFrame) {
		t.Fatalf("unsupported frame schema: %v", err)
	}
	for _, raw := range [][]byte{[]byte(`{`), []byte(`{"open":1}`)} {
		if _, ok := DefaultTransform(constants.ChannelCandleUpdate, raw); ok {
			t.Fatalf("malformed candle transformed: %s", raw)
		}
		if _, ok := candleFrame(raw); ok {
			t.Fatalf("malformed candle encoded: %s", raw)
		}
	}
	message, ok := DefaultTransform("custom:state", []byte(`{"poolAddress":"0xABC","value":"ready"}`))
	var frame map[string]any
	if !ok || json.Unmarshal(message.bytes, &frame) != nil || frame["type"] != "custom:state" || message.pool != "0xabc" || message.coalesceKey != "" {
		t.Fatalf("custom channel: %+v %s", message, message.bytes)
	}
	for _, raw := range []string{"invalid", strings.Repeat("9", 400)} {
		if fprice(raw) != 0 || fvol(raw) != 0 {
			t.Fatalf("unrepresentable amount accepted: %q", raw)
		}
	}
	tracker := NewRevisionTracker(1)
	if err := tracker.Apply(Frame{Sequence: "01", Revision: "1"}, "state"); !errors.Is(err, ErrFrame) {
		t.Fatalf("noncanonical progress: %v", err)
	}
	if err := tracker.Apply(Frame{Topic: "topic", Sequence: "1", Revision: "1"}, "state"); err != nil {
		t.Fatal(err)
	}
	if err := tracker.Apply(Frame{Topic: "topic", Sequence: "1", Revision: "2"}, "state"); !errors.Is(err, ErrSequence) {
		t.Fatalf("backwards topic: %v", err)
	}
	if err := tracker.Apply(Frame{Topic: "other", Sequence: "2", Revision: "2"}, "other"); err == nil {
		t.Fatal("tracker capacity exceeded")
	}
}

func TestLifecycleAdmissionRefillsAndBoundsBurst(t *testing.T) {
	l := NewLifecycle()
	l.admissionAt = time.Now().Add(-time.Hour)
	_, leave, err := l.Join()
	if err != nil || l.Connections() != 1 {
		t.Fatalf("refilled admission: %d %v", l.Connections(), err)
	}
	leave()
	l.admissionAt = time.Now().Add(time.Hour)
	l.admissionTokens = 0
	if _, _, err := l.Join(); !errors.Is(err, ErrStreamUnavailable) || l.Connections() != 0 {
		t.Fatalf("exhausted admission: %d %v", l.Connections(), err)
	}
}
