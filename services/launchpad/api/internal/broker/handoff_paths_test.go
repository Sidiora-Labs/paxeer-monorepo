package broker

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
)

func TestStreamOriginAndControlContracts(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/v2/snapshots", nil)
	origin := "https://" + request.Host
	allowed := map[string]bool{origin: true}
	request.Header.Set("Origin", origin)
	got, err := TrustedHTTPOrigin(request, allowed, nil)
	if err != nil || got != origin {
		t.Fatalf("allowed origin: %q %v", got, err)
	}
	request.Header.Set("Origin", origin+"/other")
	if _, err := TrustedHTTPOrigin(request, allowed, nil); !errors.Is(err, sharedauth.ErrScopeDenied) {
		t.Fatalf("untrusted origin: %v", err)
	}
	request.Header.Del("Origin")
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	request.TLS = &tls.ConnectionState{}
	got, err = TrustedHTTPOrigin(request, allowed, nil)
	if err != nil || got != origin {
		t.Fatalf("same origin TLS request: %q %v", got, err)
	}
	request.TLS = nil
	if _, err := TrustedHTTPOrigin(request, allowed, nil); !errors.Is(err, sharedauth.ErrScopeDenied) {
		t.Fatalf("insecure same origin request: %v", err)
	}
	request.RemoteAddr = "127.0.0.1:1245"
	request.Header.Set("X-Forwarded-Proto", "https")
	if got, err := TrustedHTTPOrigin(request, allowed, []string{"127.0.0.0/8"}); err != nil || got != origin {
		t.Fatalf("trusted proxy origin: %q %v", got, err)
	}
	for _, check := range []struct {
		name, remote, cidr, site, protocol string
	}{
		{"untrusted peer", "127.0.0.1:1245", "192.0.2.0/24", "same-origin", "https"},
		{"invalid peer", "invalid", "127.0.0.0/8", "same-origin", "https"},
		{"invalid proxy range", "127.0.0.1:1245", "invalid", "same-origin", "https"},
		{"other fetch site", "127.0.0.1:1245", "127.0.0.0/8", "cross-site", "https"},
		{"insecure proxy", "127.0.0.1:1245", "127.0.0.0/8", "same-origin", "http"},
	} {
		t.Run(check.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v2/snapshots", nil)
			req.RemoteAddr = check.remote
			req.Header.Set("Sec-Fetch-Site", check.site)
			req.Header.Set("X-Forwarded-Proto", check.protocol)
			if _, err := TrustedHTTPOrigin(req, allowed, []string{check.cidr}); !errors.Is(err, sharedauth.ErrScopeDenied) {
				t.Fatalf("untrusted same-origin request: %v", err)
			}
		})
	}
	for _, tc := range []struct {
		cause error
		code  string
	}{
		{sharedauth.ErrScopeDenied, "AUTH_REQUIRED"},
		{ErrReplayLimit, "REPLAY_LIMIT"},
		{ErrExpired, "RESUME_EXPIRED"},
		{ErrCursor, "RESUME_INVALID"},
		{ErrSequence, "RESUME_INVALID"},
		{ErrFrame, "INTEGRITY_QUARANTINED"},
		{errors.New("upstream unavailable"), "DEPENDENCY_UNAVAILABLE"},
	} {
		if code := StreamFailure(tc.cause); code != tc.code {
			t.Errorf("%v: %s, want %s", tc.cause, code, tc.code)
		}
	}
	topic := "market.price.0x" + strings.Repeat("1", 40)
	control, err := StreamControl("resnapshot_required", "subscription-1", "cursor-1", "RESUME_EXPIRED", []string{topic}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Op          string `json:"op"`
		SnapshotURL string `json:"snapshotUrl"`
	}
	if err := json.Unmarshal(control, &document); err != nil || document.Op != "resnapshot_required" || document.SnapshotURL != SnapshotRoute([]string{topic}) {
		t.Fatalf("resnapshot control: %+v, %v", document, err)
	}
	if _, err := StreamControl("resnapshot_required", "", "", "", []string{topic}, map[string]any{"op": "override"}); !errors.Is(err, ErrFrame) {
		t.Fatalf("reserved control field: %v", err)
	}
}

func TestTopicFanoutNotifiesOnlyWatchedTopics(t *testing.T) {
	f := NewTopicFanout()
	if !f.Degraded() || f.Interruptions() != 0 {
		t.Fatalf("new fanout status: degraded=%v interruptions=%d", f.Degraded(), f.Interruptions())
	}
	first := "market.price.0x" + strings.Repeat("1", 40)
	second := "market.price.0x" + strings.Repeat("2", 40)
	ch, stop, err := f.Watch([]string{first})
	if err != nil {
		t.Fatal(err)
	}
	f.committed(second)
	select {
	case <-ch:
		t.Fatal("unwatched topic notified subscriber")
	default:
	}
	f.committed(first)
	select {
	case <-ch:
	default:
		t.Fatal("committed topic did not notify subscriber")
	}
	stop()
	stop()
	f.committed(first)
	select {
	case <-ch:
		t.Fatal("stopped subscriber received notification")
	default:
	}
	if _, _, err := f.Watch([]string{"market.*"}); err == nil {
		t.Fatal("invalid topic accepted")
	}
}

func TestPostgresStreamConstructorAndCheckpoint(t *testing.T) {
	ctx, storage := postgresStreamStorage(t)
	if _, err := NewPostgresStream(storage.pool, "invalid.schema", storage.key, sharedauth.HTTPStreamAuthority{}); !errors.Is(err, ErrStreamUnavailable) {
		t.Fatalf("invalid stream schema accepted: %v", err)
	}
	stream, err := NewPostgresStream(storage.pool, strings.Trim(storage.schema, `"`), storage.key, sharedauth.HTTPStreamAuthority{})
	if err != nil {
		t.Fatal(err)
	}
	frame := publicPriceFrame(t, "1")
	commitStreamFrame(t, ctx, stream, frame)
	token := snapshotCursor(t, ctx, stream, []string{frame.Topic})
	replay, err := stream.PublicResume(ctx, []string{frame.Topic}, token)
	if err != nil {
		t.Fatal(err)
	}
	defer replay.Close()
	if batch, err := replay.Batch(ctx); err != nil || len(batch) != 0 {
		t.Fatalf("initial replay: %d frames, %v", len(batch), err)
	}
	checkpoint, err := replay.Checkpoint(ctx)
	if err != nil || checkpoint == "" {
		t.Fatalf("live checkpoint: %q %v", checkpoint, err)
	}
	if _, err := stream.AuthenticateHTTP(ctx, ""); err == nil {
		t.Fatal("empty stream bearer accepted")
	}
	request := httptest.NewRequest(http.MethodGet, "/v2/snapshots", nil)
	if _, err := stream.AuthenticateRequest(ctx, request); err == nil {
		t.Fatal("unauthenticated stream request accepted")
	}
	if err := stream.ConsumeCommitted(context.Background(), nil, NewTopicFanout()); !errors.Is(err, ErrStreamUnavailable) {
		t.Fatalf("missing consumer accepted: %v", err)
	}
	if err := stream.RunCommitted(context.Background(), nil, natsdriver.ConsumerConfig{}, NewTopicFanout()); !errors.Is(err, ErrStreamUnavailable) {
		t.Fatalf("missing committed broker accepted: %v", err)
	}
}
