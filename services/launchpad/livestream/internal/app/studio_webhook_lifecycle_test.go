package app

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/livepeer"
	"github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/streams"
)

const studioLifecycleSecret = "repair-m1002-local-webhook-signing-secret"

// These are Studio's callback and GET /stream response shapes, pinned by
// LivepeerWebhookSourceRevision and the committed stream contract webhook fixtures.
// Only delivery/event times and the event identity vary between observations.
func studioLifecyclePayload(id, event string, occurred, delivered time.Time) []byte {
	return []byte(fmt.Sprintf(`{"id":%q,"webhookId":"studio_callback","createdAt":%d,"timestamp":%d,"event":%q,"stream":{"id":"studio_provider"}}`, id, occurred.UnixMilli(), delivered.UnixMilli(), event))
}

// The loopback peer drives the production authentication and app codec. It has
// no lifecycle implementation: persistence and terminal/stale suppression remain
// owned by LifecycleStore, outside this task's scope.
func studioLifecycleReceiver(t *testing.T, now time.Time) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := streams.VerifyProviderWebhook(w, r, studioLifecycleSecret, now, true)
		if !ok {
			return
		}
		event, err := decodeStudioWebhook(raw)
		if err != nil {
			http.Error(w, "invalid provider event", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(event); err != nil {
			t.Errorf("encode decoded provider observation: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func studioLifecycleDeliver(t *testing.T, server *httptest.Server, raw []byte, delivered time.Time) streams.ProviderEvent {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(studioLifecycleSecret))
	_, _ = mac.Write(raw)
	req, err := http.NewRequest(http.MethodPost, server.URL+"/webhooks/livepeer", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Livepeer-Signature", fmt.Sprintf("t=%d,v1=%s", delivered.UnixMilli(), hex.EncodeToString(mac.Sum(nil))))
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("Studio callback status=%d body=%s", response.StatusCode, body)
	}
	var event streams.ProviderEvent
	if err := json.NewDecoder(response.Body).Decode(&event); err != nil {
		t.Fatal(err)
	}
	return event
}

func studioLifecycleRequireEvent(t *testing.T, event streams.ProviderEvent, id, state string, occurred time.Time) {
	t.Helper()
	if event.EventID != id || event.ProviderID != "studio_provider" || event.State != state || !event.ObservedAt.Equal(occurred) {
		t.Fatalf("decoded observation=%+v; want event=%s provider=studio_provider state=%s occurred=%s", event, id, state, occurred)
	}
}

func TestStudioLifecycleIdleThenStarted(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	server := studioLifecycleReceiver(t, now)
	for _, observation := range []struct {
		id, event, state string
		at               time.Time
	}{
		{"initial_live", "stream.started", "LIVE", now.Add(-3 * time.Minute)},
		{"disconnect", "stream.idle", "READY", now.Add(-2 * time.Minute)},
		{"reconnect", "stream.started", "LIVE", now.Add(-time.Minute)},
	} {
		event := studioLifecycleDeliver(t, server, studioLifecyclePayload(observation.id, observation.event, observation.at, now), now)
		studioLifecycleRequireEvent(t, event, observation.id, observation.state, observation.at)
		// Exercise the real creator operation projection with the decoded
		// observation. This checks representation, not store transitions.
		view, err := streams.AdaptV2Operation(streams.Operation{ID: "11111111-1111-4111-8111-111111111111", State: event.State, Revision: 7})
		if err != nil || view.Status != strings.ToLower(observation.state) || view.StreamID != view.OperationID || view.Revision != "7" {
			t.Fatalf("creator projection=%+v err=%v", view, err)
		}
	}
}

func TestStudioLifecycleReorderedPollWebhook(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	for _, active := range []bool{false, true} {
		t.Run(fmt.Sprintf("active_%t", active), func(t *testing.T) {
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/stream/studio_provider" || r.Header.Get("Authorization") != "Bearer local-provider-key" {
					http.Error(w, "unexpected provider request", http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintf(w, `{"id":"studio_provider","playbackId":"studio_playback","isActive":%t,"createdAt":1690000000000}`, active)
			}))
			defer provider.Close()
			client := livepeer.New("local-provider-key", provider.URL, nil, livepeer.WithHTTPClient(provider.Client()))
			receiver := studioLifecycleReceiver(t, now)
			eventName, state := "stream.idle", "READY"
			if active {
				eventName, state = "stream.started", "LIVE"
			}
			occurred := now.Add(-2 * time.Minute)
			raw := studioLifecyclePayload("reordered_event", eventName, occurred, now)
			before := studioLifecycleDeliver(t, receiver, raw, now)
			remote, err := client.GetStream(context.Background(), "studio_provider")
			if err != nil {
				t.Fatal(err)
			}
			if remote.IsActive != active || remote.ID != before.ProviderID {
				t.Fatalf("poll=%+v callback=%+v", remote, before)
			}
			// Exercise both arrival orders around an actual provider read. The
			// original event time, rather than delivery time, must reach the
			// lifecycle consumer so its stale-event guard remains effective.
			after := studioLifecycleDeliver(t, receiver, raw, now)
			studioLifecycleRequireEvent(t, before, "reordered_event", state, occurred)
			studioLifecycleRequireEvent(t, after, "reordered_event", state, occurred)
			if before != after || !after.ObservedAt.Before(now) {
				t.Fatalf("observation changed with arrival order: before=%+v after=%+v", before, after)
			}
			// A delayed idle after a newer start must retain its older time;
			// neither payload may fabricate terminal evidence.
			started := studioLifecycleDeliver(t, receiver, studioLifecyclePayload("newer_started", "stream.started", now.Add(-time.Minute), now), now)
			idle := studioLifecycleDeliver(t, receiver, studioLifecyclePayload("older_idle", "stream.idle", occurred, now), now)
			if started.State != "LIVE" || idle.State != "READY" || !idle.ObservedAt.Before(started.ObservedAt) {
				t.Fatalf("reordered start=%+v idle=%+v", started, idle)
			}
		})
	}
}

func TestStudioLifecycleDecoderRestart(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	idleAt := now.Add(-2 * time.Minute)
	idle := studioLifecyclePayload("restart_idle", "stream.idle", idleAt, now)
	first := studioLifecycleReceiver(t, now)
	before := studioLifecycleDeliver(t, first, idle, now)
	studioLifecycleRequireEvent(t, before, "restart_idle", "READY", idleAt)
	first.Close()
	// Reopen the authentication/decoder component and redeliver the same
	// provider bytes. The adapter has no process-local terminal latch.
	reopened := studioLifecycleReceiver(t, now)
	after := studioLifecycleDeliver(t, reopened, idle, now)
	if before != after {
		t.Fatalf("decoder restart changed observation: before=%+v after=%+v", before, after)
	}
	startedAt := now.Add(-time.Minute)
	started := studioLifecycleDeliver(t, reopened, studioLifecyclePayload("restart_started", "stream.started", startedAt, now), now)
	studioLifecycleRequireEvent(t, started, "restart_started", "LIVE", startedAt)
}

func TestStudioLifecycleRejectInvalidTerminationEvidence(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	for _, event := range []string{"stream.ended", "stream.deleted", "stream.terminated", "inactive", "", "stream.unknown"} {
		if decoded, err := decodeStudioWebhook(studioLifecyclePayload("unsupported", event, now, now)); !errors.Is(err, streams.ErrInvalid) || decoded != (streams.ProviderEvent{}) {
			t.Fatalf("unsupported event %q produced %+v err=%v", event, decoded, err)
		}
	}
	valid := studioLifecyclePayload("invalid", "stream.idle", now, now)
	for _, raw := range [][]byte{
		[]byte(`{"event":"stream.idle"}`),
		bytes.Replace(valid, []byte(`"id":"invalid"`), []byte(`"id":""`), 1),
		bytes.Replace(valid, []byte(`"webhookId":"studio_callback"`), []byte(`"webhookId":""`), 1),
		bytes.Replace(valid, []byte(`"id":"studio_provider"`), []byte(`"id":""`), 1),
		studioLifecyclePayload("invalid", "stream.idle", now, now.Add(-time.Millisecond)),
		append(append([]byte{}, valid...), []byte(`{}`)...),
	} {
		if decoded, err := decodeStudioWebhook(raw); !errors.Is(err, streams.ErrInvalid) || decoded != (streams.ProviderEvent{}) {
			t.Fatalf("invalid callback produced %+v err=%v", decoded, err)
		}
	}
	// Authentication must still reject altered bytes before state decoding.
	raw := studioLifecyclePayload("tampered", "stream.idle", now, now)
	mac := hmac.New(sha256.New, []byte(studioLifecycleSecret))
	_, _ = mac.Write(raw)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/livepeer", bytes.NewReader(bytes.Replace(raw, []byte("stream.idle"), []byte("stream.started"), 1)))
	req.Header.Set("Livepeer-Signature", fmt.Sprintf("t=%d,v1=%s", now.UnixMilli(), hex.EncodeToString(mac.Sum(nil))))
	response := httptest.NewRecorder()
	if _, ok := streams.VerifyProviderWebhook(response, req, studioLifecycleSecret, now, true); ok || response.Code != http.StatusUnauthorized {
		t.Fatalf("tampered callback accepted=%t status=%d", ok, response.Code)
	}
}
