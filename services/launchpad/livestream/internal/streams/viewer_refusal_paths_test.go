package streams

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestViewerRoutesBindLiveStateRestrictionsAndTokenAuthority(t *testing.T) {
	f := repair57New(t)
	ctx := context.Background()
	pending, _ := f.create("viewer-pending")
	live, _ := f.create("viewer-live")
	live, err := f.store.BindProvider(ctx, live.ID, "viewer_live_provider")
	if err != nil {
		t.Fatal(err)
	}
	if outcome, err := f.store.Observe(ctx, ProviderEvent{EventID: "viewer_live_event", ProviderID: live.ProviderID, State: stateLive, ObservedAt: time.Now()}); err != nil || outcome != "applied" {
		t.Fatalf("live provider observation = %s, %v", outcome, err)
	}
	viewers, err := NewViewerCounter(f.redis, "livestream", 100000)
	if err != nil {
		t.Fatal(err)
	}
	f.handlers.viewers = viewers
	f.handlers.viewerTokens = &ViewerTokenAuthority{Key: []byte(strings.Repeat("v", 32))}
	request := func(path, body string, want int) {
		t.Helper()
		r, err := http.NewRequest(http.MethodPost, f.server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Content-Type", "application/json")
		response, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil || response.StatusCode != want || !bytes.Contains(raw, []byte(`"code"`)) {
			t.Fatalf("viewer route %s = %d %s, %v", path, response.StatusCode, raw, err)
		}
	}
	request("/v2/streams/invalid/viewer-session", `{}`, http.StatusNotFound)
	if _, err := f.database.Exec(ctx, "INSERT INTO chat.safety_bans(wallet,scope) VALUES($1,'global')", f.wallet); err != nil {
		t.Fatal(err)
	}
	request("/v2/streams/"+live.ID+"/viewer-session", `{}`, http.StatusForbidden)
	if _, err := f.database.Exec(ctx, "DELETE FROM chat.safety_bans WHERE wallet=$1", f.wallet); err != nil {
		t.Fatal(err)
	}
	f.handlers.viewerTokens.Key = []byte("short")
	request("/v2/streams/"+live.ID+"/viewer-session", `{}`, http.StatusServiceUnavailable)
	f.handlers.viewerTokens.Key = []byte(strings.Repeat("v", 32))
	session, err := randomID()
	if err != nil {
		t.Fatal(err)
	}
	pendingToken, _, err := f.handlers.viewerTokens.IssuePolicyBound(pending.ID, session, 0, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	request("/v2/streams/"+pending.ID+"/heartbeat", fmt.Sprintf(`{"sessionId":%q,"token":%q}`, session, pendingToken), http.StatusConflict)
	liveToken, _, err := f.handlers.viewerTokens.IssuePolicyBound(live.ID, session, 0, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.database.Exec(ctx, "INSERT INTO chat.safety_bans(wallet,scope) VALUES($1,'global')", f.wallet); err != nil {
		t.Fatal(err)
	}
	request("/v2/streams/"+live.ID+"/heartbeat", fmt.Sprintf(`{"sessionId":%q,"token":%q}`, session, liveToken), http.StatusForbidden)
	if _, err := f.database.Exec(ctx, "DELETE FROM chat.safety_bans WHERE wallet=$1", f.wallet); err != nil {
		t.Fatal(err)
	}
	request("/v2/streams/invalid/heartbeat", `{"sessionId":"bad","token":"bad"}`, http.StatusUnprocessableEntity)
	missingID, err := randomID()
	if err != nil {
		t.Fatal(err)
	}
	missingToken, _, err := f.handlers.viewerTokens.IssuePolicyBound(missingID, session, 0, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	request("/v2/streams/"+missingID+"/heartbeat", fmt.Sprintf(`{"sessionId":%q,"token":%q}`, session, missingToken), http.StatusNotFound)
	if err := f.redis.Close(); err != nil {
		t.Fatal(err)
	}
	request("/v2/streams/"+live.ID+"/heartbeat", fmt.Sprintf(`{"sessionId":%q,"token":%q}`, session, liveToken), http.StatusServiceUnavailable)
	if _, err := f.database.Exec(ctx, "DROP TABLE livestream.stream_playback_privacy"); err != nil {
		t.Fatal(err)
	}
	request("/v2/streams/"+live.ID+"/heartbeat", fmt.Sprintf(`{"sessionId":%q,"token":%q}`, session, liveToken), http.StatusServiceUnavailable)
}

func TestPrivateViewerSessionRequiresOwnerAndCurrentPrivacyStore(t *testing.T) {
	f := repair57New(t)
	ctx := context.Background()
	o, _ := f.create("private-viewer-session")
	o, err := f.store.BindProvider(ctx, o.ID, "private_viewer_provider")
	if err != nil {
		t.Fatal(err)
	}
	if outcome, err := f.store.Observe(ctx, ProviderEvent{EventID: "private_viewer_live", ProviderID: o.ProviderID, State: stateLive, ObservedAt: time.Now()}); err != nil || outcome != "applied" {
		t.Fatalf("private viewer state = %s, %v", outcome, err)
	}
	o, err = f.store.Get(ctx, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := f.database.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.SetPlaybackPrivacyTx(ctx, tx, o, true); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	viewers, err := NewViewerCounter(f.redis, "livestream", 100000)
	if err != nil {
		t.Fatal(err)
	}
	f.handlers.viewers = viewers
	f.handlers.viewerTokens = &ViewerTokenAuthority{Key: []byte(strings.Repeat("v", 32))}
	path := "/v2/streams/" + o.ID + "/viewer-session"
	refused := func(headers map[string]string, status int, code string) {
		t.Helper()
		request, err := http.NewRequest(http.MethodPost, f.server.URL+path, strings.NewReader(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		for key, value := range headers {
			request.Header.Set(key, value)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		if closeErr := response.Body.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		if err != nil || response.StatusCode != status || !bytes.Contains(body, []byte(`"code":"`+code+`"`)) {
			t.Fatalf("viewer refusal = %d %s, %v", response.StatusCode, body, err)
		}
	}
	refused(nil, http.StatusForbidden, "SCOPE_DENIED")
	authorized := f.exchange(http.MethodPost, path, []byte(`{}`), map[string]string{"Content-Type": "application/json", "Authorization": "Bearer " + f.session(f.key)}, http.StatusOK)
	if !bytes.Contains(authorized, []byte(`"token"`)) {
		t.Fatalf("private viewer session = %s", authorized)
	}
	if err := f.redis.Close(); err != nil {
		t.Fatal(err)
	}
	refused(map[string]string{"Authorization": "Bearer " + f.session(f.key)}, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE")
	if _, err := f.database.Exec(ctx, "DROP TABLE livestream.stream_playback_privacy"); err != nil {
		t.Fatal(err)
	}
	refused(map[string]string{"Authorization": "Bearer " + f.session(f.key)}, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE")
}

func TestPlaybackWebhookRejectsUnboundRequestsAfterSignatureVerification(t *testing.T) {
	f := repair57New(t)
	ctx := context.Background()
	pending, _ := f.create("playback-pending")
	live, _ := f.create("playback-live")
	live, err := f.store.BindProvider(ctx, live.ID, "playback_provider")
	if err != nil {
		t.Fatal(err)
	}
	if outcome, err := f.store.Observe(ctx, ProviderEvent{EventID: "playback_live_event", ProviderID: live.ProviderID, State: stateLive, ObservedAt: time.Now()}); err != nil || outcome != "applied" {
		t.Fatalf("live provider observation = %s, %v", outcome, err)
	}
	f.handlers.playbackWebhookSecret = strings.Repeat("p", 32)
	f.handlers.viewerTokens = &ViewerTokenAuthority{Key: []byte(strings.Repeat("v", 32))}
	session, err := randomID()
	if err != nil {
		t.Fatal(err)
	}
	liveToken, _, err := f.handlers.viewerTokens.IssuePolicyBound(live.ID, session, 0, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	pendingToken, _, err := f.handlers.viewerTokens.IssuePolicyBound(pending.ID, session, 0, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().UnixMilli()
	check := func(body string, want int) {
		t.Helper()
		mac := hmac.New(sha256.New, []byte(f.handlers.playbackWebhookSecret))
		_, _ = mac.Write([]byte(body))
		request, err := http.NewRequest(http.MethodPost, f.server.URL+"/webhooks/livepeer/playback-access", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Livepeer-Signature", fmt.Sprintf("t=%d,v1=%s", stamp, hex.EncodeToString(mac.Sum(nil))))
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil || response.StatusCode != want || !bytes.Contains(raw, []byte(`"code"`)) {
			t.Fatalf("playback webhook = %d %s, %v", response.StatusCode, raw, err)
		}
	}
	body := func(id, token string) string {
		return fmt.Sprintf(`{"timestamp":%d,"context":{"operationId":%q},"accessKey":%q}`, stamp, id, token)
	}
	check(body("invalid", liveToken), http.StatusForbidden)
	check(body(live.ID, "!!!.invalid"), http.StatusForbidden)
	check(body(live.ID, base64.RawURLEncoding.EncodeToString([]byte("short.fields"))+".invalid"), http.StatusForbidden)
	check(body(pending.ID, pendingToken), http.StatusForbidden)
	check(body(live.ID, liveToken)+strings.Repeat(" ", 16385), http.StatusRequestEntityTooLarge)
	missingID, err := randomID()
	if err != nil {
		t.Fatal(err)
	}
	missingToken, _, err := f.handlers.viewerTokens.IssuePolicyBound(missingID, session, 0, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	check(body(missingID, missingToken), http.StatusNotFound)
	if _, err := f.database.Exec(ctx, "DROP VIEW chat.safety_bans_v1"); err != nil {
		t.Fatal(err)
	}
	check(body(live.ID, liveToken), http.StatusServiceUnavailable)
	if _, err := f.database.Exec(ctx, "DROP TABLE livestream.stream_playback_privacy"); err != nil {
		t.Fatal(err)
	}
	check(body(live.ID, liveToken), http.StatusServiceUnavailable)
}
