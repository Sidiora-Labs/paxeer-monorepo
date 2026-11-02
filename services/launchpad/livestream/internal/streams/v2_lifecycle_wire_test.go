package streams

import (
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestAdaptV2StreamLifecycleEnvelopesMatchFrozenSchema(t *testing.T) {
	const requestID = "22222222-2222-4222-8222-222222222222"
	const streamID = "11111111-1111-4111-8111-111111111111"
	at := time.Date(2026, time.October, 26, 14, 20, 30, 123456789, time.FixedZone("offset", 2*60*60))
	checkEnvelope := func(t *testing.T, name string, raw []byte, revision *string) map[string]json.RawMessage {
		t.Helper()
		if err := v2.ValidateJSON(name, raw); err != nil {
			t.Fatalf("%s: %v; body: %s", name, err, raw)
		}
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(raw, &envelope); err != nil {
			t.Fatal(err)
		}
		if len(envelope) != 7 || string(envelope["requestId"]) != strconv.Quote(requestID) || string(envelope["quality"]) != `"complete"` {
			t.Fatalf("unexpected envelope: %s", raw)
		}
		for _, field := range []string{"watermark", "nextCursor"} {
			if string(envelope[field]) != "null" {
				t.Fatalf("%s must be explicit null: %s", field, raw)
			}
		}
		wantRevision := "null"
		if revision != nil {
			wantRevision = strconv.Quote(*revision)
		}
		if string(envelope["revision"]) != wantRevision {
			t.Fatalf("unexpected revision: %s", raw)
		}
		var generatedAt string
		if err := json.Unmarshal(envelope["generatedAt"], &generatedAt); err != nil {
			t.Fatal(err)
		}
		parsed, err := time.Parse(v2LifecycleTimeLayout, generatedAt)
		if err != nil || parsed.UTC().Format(v2LifecycleTimeLayout) != generatedAt {
			t.Fatalf("timestamp must be UTC with millisecond precision: %q (%v)", generatedAt, err)
		}
		return envelope
	}

	for _, adapter := range []struct {
		name  string
		adapt func(string, Operation) ([]byte, error)
	}{
		{"StreamStartResponse", AdaptV2StreamStart},
		{"StreamEndResponse", AdaptV2StreamEnd},
	} {
		t.Run(adapter.name, func(t *testing.T) {
			for _, state := range []string{"REQUESTED", "PROVISIONING", "READY", "LIVE", "ENDING", "ENDED", "FAILED", "SUSPENDED"} {
				for _, retryAt := range []*time.Time{nil, &at} {
					name := state + "/no_retry"
					if retryAt != nil {
						name = state + "/persisted_retry"
					}
					t.Run(name, func(t *testing.T) {
						o := Operation{ID: streamID, State: state, Revision: 9007199254740992, RetryAt: retryAt, ProviderID: "private-provider", Wallet: "private-wallet", Title: "private-title"}
						before := time.Now().UTC().Truncate(time.Millisecond)
						raw, err := adapter.adapt(requestID, o)
						if err != nil {
							t.Fatal(err)
						}
						after := time.Now().UTC()
						revision := strconv.FormatInt(o.Revision, 10)
						envelope := checkEnvelope(t, adapter.name, raw, &revision)
						var data v2.StreamStartData
						if err := json.Unmarshal(envelope["data"], &data); err != nil {
							t.Fatal(err)
						}
						want := v2.StreamOperation{OperationId: streamID, StreamId: streamID, Status: strings.ToLower(state), Revision: v2.PositiveUInt256(revision)}
						if retryAt != nil {
							retry := "2026-10-26T12:20:30.123Z"
							want.RetryAt = &retry
						}
						if !reflect.DeepEqual(data.Operation, want) {
							t.Fatalf("operation = %+v, want %+v", data.Operation, want)
						}
						var generatedAt string
						if err := json.Unmarshal(envelope["generatedAt"], &generatedAt); err != nil {
							t.Fatal(err)
						}
						generated, err := time.Parse(v2LifecycleTimeLayout, generatedAt)
						if err != nil || generated.Before(before) || generated.After(after) {
							t.Fatalf("generation time outside adapter call: %s (%v)", generatedAt, err)
						}
						if strings.Contains(string(raw), "private-") || !strings.Contains(string(raw), `"retryAt":`) {
							t.Fatalf("private field leaked or nullable retry omitted: %s", raw)
						}
					})
				}
			}
		})
		t.Run(adapter.name+"/invalid", func(t *testing.T) {
			for _, o := range []Operation{
				{ID: "invalid", State: "READY", Revision: 1},
				{ID: streamID, State: "UNKNOWN", Revision: 1},
				{ID: streamID, State: "READY", Revision: 0},
				{ID: streamID, State: "READY", Revision: -1},
			} {
				if raw, err := adapter.adapt(requestID, o); !errors.Is(err, ErrInvalid) || raw != nil {
					t.Fatalf("invalid operation returned %s, %v", raw, err)
				}
			}
			if raw, err := adapter.adapt("invalid", Operation{ID: streamID, State: "READY", Revision: 1}); err == nil || raw != nil {
				t.Fatalf("invalid request ID returned %s, %v", raw, err)
			}
		})
	}

	for _, viewers := range []int64{0, 42, 9007199254740991} {
		t.Run("StreamHeartbeatResponse/"+strconv.FormatInt(viewers, 10), func(t *testing.T) {
			raw, err := AdaptV2StreamHeartbeat(requestID, streamID, viewers, at)
			if err != nil {
				t.Fatal(err)
			}
			envelope := checkEnvelope(t, "StreamHeartbeatResponse", raw, nil)
			var data v2.StreamHeartbeatData
			if err := json.Unmarshal(envelope["data"], &data); err != nil {
				t.Fatal(err)
			}
			if int64(data.ViewerCount) != viewers || data.ViewerQuality != "verified" || string(envelope["generatedAt"]) != `"2026-10-26T12:20:30.123Z"` {
				t.Fatalf("heartbeat lost count, quality or observation time: %s", raw)
			}
		})
	}
	t.Run("StreamHeartbeatResponse/invalid", func(t *testing.T) {
		for _, tc := range []struct {
			requestID string
			streamID  string
			viewers   int64
			at        time.Time
		}{
			{"invalid", streamID, 0, at},
			{requestID, "invalid", 0, at},
			{requestID, streamID, -1, at},
			{requestID, streamID, 9007199254740992, at},
			{requestID, streamID, 0, time.Time{}},
			{requestID, streamID, 0, time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)},
		} {
			if raw, err := AdaptV2StreamHeartbeat(tc.requestID, tc.streamID, tc.viewers, tc.at); err == nil || raw != nil {
				t.Fatalf("invalid heartbeat returned %s, %v", raw, err)
			}
		}
	})
}
