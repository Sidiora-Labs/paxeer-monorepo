package fanout_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/go-chi/chi/v5"
)

func TestConnectedRoutesExposeCommittedDirectConversation(t *testing.T) {
	f := newRepair1027Fixture(t)
	runtime := f.start(500, 20, 20)
	connection := runtime.dial(addr(0xaa))
	persisted := repair1027Send(t, connection, "dm", "connected-conversation", "readable private message")
	repair1027Persisted(t, persisted, "connected-conversation")
	measurement := runtime.hub.Measurement()
	if measurement.Connected != 1 || measurement.EnqueuedFrames < 1 || measurement.ObservedAt.IsZero() {
		t.Fatalf("connected hub measurement = %+v", measurement)
	}
	router := chi.NewRouter()
	if err := fanout.RegisterConnectedRoutes(router, runtime.store, runtime.hub, f.requestPrincipal); err != nil {
		t.Fatal(err)
	}
	request := func(handler http.Handler, method, path, wallet string, body []byte) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("X-Fixture-Session", wallet)
		rw := httptest.NewRecorder()
		handler.ServeHTTP(rw, req)
		return rw
	}
	conversations := request(router, http.MethodGet, "/v2/chat/conversations", addr(0xaa), nil)
	if conversations.Code != http.StatusOK {
		t.Fatalf("connected conversations = %d %s", conversations.Code, conversations.Body.String())
	}
	var reply struct {
		Conversations []struct {
			ID   string `json:"id"`
			Peer string `json:"peer"`
		} `json:"conversations"`
	}
	if err := json.Unmarshal(conversations.Body.Bytes(), &reply); err != nil || len(reply.Conversations) != 1 || reply.Conversations[0].Peer != addr(0xbb) {
		t.Fatalf("connected conversations = %+v: %v", reply, err)
	}
	classic := request(fanout.ConversationsHandler(runtime.store, f.requestPrincipal), http.MethodGet, "/v2/chat/conversations", addr(0xaa), nil)
	if classic.Code != http.StatusOK || !bytes.Contains(classic.Body.Bytes(), []byte(reply.Conversations[0].ID)) {
		t.Fatalf("conversation listing = %d %s", classic.Code, classic.Body.String())
	}
	read := request(router, http.MethodPost, "/v2/chat/read", addr(0xbb), []byte(`{"conversation_id":"`+reply.Conversations[0].ID+`","through_sequence":"1"}`))
	if read.Code != http.StatusOK || !bytes.Contains(read.Body.Bytes(), []byte(`"success":true`)) {
		t.Fatalf("read marker = %d %s", read.Code, read.Body.String())
	}
	var through int64
	if err := runtime.pool.QueryRow(context.Background(), `SELECT read_sequence FROM chat_members WHERE conversation_id=$1 AND wallet=$2`, reply.Conversations[0].ID, addr(0xbb)).Scan(&through); err != nil || through != 1 {
		t.Fatalf("stored read marker = %d: %v", through, err)
	}
	messageID, ok := persisted["id"].(string)
	if !ok || messageID == "" {
		t.Fatalf("persisted message identity = %v", persisted)
	}
	revised := request(router, http.MethodPost, "/v2/chat/revise", addr(0xaa), []byte(`{"message_id":"`+messageID+`","expected_revision":"1","content":"revised private message"}`))
	if revised.Code != http.StatusOK || !bytes.Contains(revised.Body.Bytes(), []byte(`"content":"revised private message"`)) || !bytes.Contains(revised.Body.Bytes(), []byte(`"revision":"2"`)) {
		t.Fatalf("revised message = %d %s", revised.Code, revised.Body.String())
	}
	stale := request(router, http.MethodPost, "/v2/chat/revise", addr(0xaa), []byte(`{"message_id":"`+messageID+`","expected_revision":"1","content":"stale message"}`))
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale revision = %d %s", stale.Code, stale.Body.String())
	}
	invalidRead := request(router, http.MethodPost, "/v2/chat/read", addr(0xbb), []byte(`{"conversation_id":"`+reply.Conversations[0].ID+`","through_sequence":"invalid"}`))
	if invalidRead.Code != http.StatusBadRequest {
		t.Fatalf("invalid read marker = %d %s", invalidRead.Code, invalidRead.Body.String())
	}
	historyPath := "/v2/chat/history?conversation_id=" + reply.Conversations[0].ID
	history := request(router, http.MethodGet, historyPath, addr(0xbb), nil)
	if history.Code != http.StatusOK || !bytes.Contains(history.Body.Bytes(), []byte("revised private message")) {
		t.Fatalf("connected message history = %d %s", history.Code, history.Body.String())
	}
	classicHistory := request(fanout.HistoryHandler(runtime.store, f.requestPrincipal), http.MethodGet, historyPath, addr(0xbb), nil)
	if classicHistory.Code != http.StatusOK || !bytes.Contains(classicHistory.Body.Bytes(), []byte(messageID)) {
		t.Fatalf("authenticated message history = %d %s", classicHistory.Code, classicHistory.Body.String())
	}
	invalidLimit := request(fanout.HistoryHandler(runtime.store, f.requestPrincipal), http.MethodGet, historyPath+"&limit=invalid", addr(0xbb), nil)
	if invalidLimit.Code != http.StatusBadRequest {
		t.Fatalf("invalid history page size = %d %s", invalidLimit.Code, invalidLimit.Body.String())
	}
	invalidCursor := request(fanout.HistoryHandler(runtime.store, f.requestPrincipal), http.MethodGet, historyPath+"&cursor=invalid", addr(0xbb), nil)
	if invalidCursor.Code != http.StatusBadRequest {
		t.Fatalf("invalid history cursor = %d %s", invalidCursor.Code, invalidCursor.Body.String())
	}
	unknownHistory := request(fanout.HistoryHandler(runtime.store, f.requestPrincipal), http.MethodGet, "/v2/chat/history?conversation_id=unknown", addr(0xbb), nil)
	if unknownHistory.Code != http.StatusNotFound {
		t.Fatalf("unknown conversation history = %d %s", unknownHistory.Code, unknownHistory.Body.String())
	}
	privateHistory := request(fanout.HistoryHandler(runtime.store, f.requestPrincipal), http.MethodGet, historyPath, "", nil)
	if privateHistory.Code != http.StatusUnauthorized || privateHistory.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("unverified history = %d %s", privateHistory.Code, privateHistory.Body.String())
	}
	unauthenticated := request(router, http.MethodGet, "/v2/chat/conversations", "", nil)
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unverified conversation read = %d %s", unauthenticated.Code, unauthenticated.Body.String())
	}
}
