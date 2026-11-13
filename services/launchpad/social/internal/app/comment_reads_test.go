package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
)

func TestCommentReadsPreserveSnapshotAndDeletion(t *testing.T) {
	h := newDevHarness(t)
	actor := "0x" + strings.ReplaceAll(uuid.NewString(), "-", "") + "00000000"
	created := data(h.expect(h.do("POST", "/v2/pools/"+h.pool+"/comments", actor, map[string]any{"body": "visible comment", "clientCommentId": uuid.NewString()}, nil), http.StatusCreated, "CommentCreateResponse"))["comment"].(map[string]any)
	id := created["commentId"].(string)
	reader := &socialComments{pool: h.app.pool}
	view, err := reader.byID(context.Background(), id)
	if err != nil || view.ID != id || view.Body != "visible comment" || view.Author != actor {
		t.Fatalf("comment view = %+v: %v", view, err)
	}
	page, next, tag, err := reader.page(context.Background(), h.pool, 1, "")
	if err != nil || len(page) != 1 || page[0].ID != id || next != nil || tag == "" {
		t.Fatalf("comment page = %+v, next = %v, tag = %q: %v", page, next, tag, err)
	}
	if second := commentSnapshotTag(h.pool, "", 1, "1", page); second == tag {
		// The page tag includes the moderation overlay even when the rows agree.
		t.Fatal("overlay revision did not change the comment snapshot")
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	request := httptest.NewRequest(http.MethodGet, "/v2/pools/"+h.pool+"/comments?limit=1", nil)
	first := httptest.NewRecorder()
	reader.reads(nextHandler).ServeHTTP(first, request)
	if first.Code != http.StatusOK || first.Header().Get("ETag") != tag {
		t.Fatalf("first read = %d, tag = %q", first.Code, first.Header().Get("ETag"))
	}
	request.Header.Set("If-None-Match", tag)
	cached := httptest.NewRecorder()
	reader.reads(nextHandler).ServeHTTP(cached, request)
	if cached.Code != http.StatusNotModified {
		t.Fatalf("matching snapshot = %d", cached.Code)
	}
	invalid := httptest.NewRecorder()
	reader.reads(nextHandler).ServeHTTP(invalid, httptest.NewRequest(http.MethodGet, "/v2/pools/"+h.pool+"/comments?limit=0", nil))
	if invalid.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid page limit = %d", invalid.Code)
	}
	pass := httptest.NewRecorder()
	reader.reads(nextHandler).ServeHTTP(pass, httptest.NewRequest(http.MethodPost, "/v2/pools/"+h.pool+"/comments", nil))
	if pass.Code != http.StatusTeapot {
		t.Fatalf("non-read route = %d", pass.Code)
	}

	h.expect(h.do("DELETE", "/v2/comments/"+id, actor, nil, nil), http.StatusNoContent, "")
	if _, err = reader.byID(context.Background(), id); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("deleted comment remains visible: %v", err)
	}
}

func TestMessageVisibilityProjectsRemovalAndRestoration(t *testing.T) {
	base := fanout.DurableMessage{ID: "message-1", Sender: addr(7), Content: "visible", Revision: 2}
	removed := safety.Visibility{TargetType: messageAction, TargetID: base.ID, Status: removedState, Revision: 3}
	projected, err := projectSafetyMessage(base, removed, false, 1)
	if err != nil || !projected.Deleted || projected.Content != "" || projected.Revision != 4 {
		t.Fatalf("removed projection = %+v: %v", projected, err)
	}
	restored := safety.Visibility{TargetType: messageAction, TargetID: base.ID, Status: "restored", Revision: 4}
	projected, err = projectSafetyMessage(base, restored, false, 3)
	if err != nil || projected.Deleted || projected.Content != "visible" || projected.Revision != 3 {
		t.Fatalf("restored projection = %+v: %v", projected, err)
	}
	projected, err = projectSafetyMessage(base, restored, true, 3)
	if err != nil || !projected.Deleted || projected.Content != "" {
		t.Fatalf("erased projection = %+v: %v", projected, err)
	}
	wrongTarget := restored
	wrongTarget.TargetID = "another-message"
	if _, err = projectSafetyMessage(base, wrongTarget, false, 0); !errors.Is(err, safety.ErrInvalid) {
		t.Fatalf("different target accepted: %v", err)
	}
}

func TestCommentModerationVisibilityProjectsFromStore(t *testing.T) {
	h := newDevHarness(t)
	ctx := context.Background()
	actor := "0x" + strings.ReplaceAll(uuid.NewString(), "-", "") + "00000000"
	created := data(h.expect(h.do("POST", "/v2/pools/"+h.pool+"/comments", actor, map[string]any{"body": "visible until moderated", "clientCommentId": uuid.NewString()}, nil), http.StatusCreated, "CommentCreateResponse"))["comment"].(map[string]any)
	id := created["commentId"].(string)
	caseID := uuid.NewString()
	if _, err := h.dbConn.Exec(ctx, `INSERT INTO chat.safety_cases(id,target_type,target_id,pool_id,affected_wallet,reporter,category,description) VALUES($1,'comment',$2,$3,$4,$5,'spam','reported content')`, caseID, id, h.pool, actor, addr(0xb0b)); err != nil {
		t.Fatal(err)
	}
	if _, err := h.dbConn.Exec(ctx, `INSERT INTO chat.safety_visibility(target_type,target_id,revision,status,case_id) VALUES('comment',$1,1,'removed',$2)`, id, caseID); err != nil {
		t.Fatal(err)
	}
	runtime := &socialConnectedRuntime{pool: h.app.pool}
	if err := runtime.visibilityOnce(ctx); err != nil {
		t.Fatal(err)
	}
	var revision int64
	var removed bool
	if err := h.dbConn.QueryRow(ctx, `SELECT revision,removed FROM chat.interaction_feed_projection WHERE generation=$1 AND aggregate_id=$2`, safetyProjectionGeneration, "comment:"+id).Scan(&revision, &removed); err != nil {
		t.Fatal(err)
	}
	if revision != 1 || !removed {
		t.Fatalf("removal projection = revision %d, removed %t", revision, removed)
	}
	reader := &socialComments{pool: h.app.pool}
	if _, err := reader.byID(ctx, id); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("removed comment remains visible: %v", err)
	}
	if _, err := h.dbConn.Exec(ctx, `UPDATE chat.safety_visibility SET revision=2,status='restored' WHERE target_type='comment' AND target_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if err := runtime.visibilityOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if err := h.dbConn.QueryRow(ctx, `SELECT revision,removed FROM chat.interaction_feed_projection WHERE generation=$1 AND aggregate_id=$2`, safetyProjectionGeneration, "comment:"+id).Scan(&revision, &removed); err != nil {
		t.Fatal(err)
	}
	if revision != 2 || removed {
		t.Fatalf("restoration projection = revision %d, removed %t", revision, removed)
	}
	view, err := reader.byID(ctx, id)
	if err != nil || view.Body != "visible until moderated" {
		t.Fatalf("restored comment = %+v: %v", view, err)
	}
}

func TestMessageModerationVisibilityProjectsCommittedChat(t *testing.T) {
	h := newDevHarness(t)
	ctx := context.Background()
	actor := "0x" + strings.ReplaceAll(uuid.NewString(), "-", "") + "00000000"
	message, err := h.app.dev.Chat.Send(ctx, fanout.SendInput{Actor: actor, Target: addr(0xb0b), Kind: "dm", ClientID: "visibility-message", Content: "visible chat body"})
	if err != nil {
		t.Fatal(err)
	}
	caseID := uuid.NewString()
	if _, err := h.dbConn.Exec(ctx, `INSERT INTO chat.safety_cases(id,target_type,target_id,pool_id,affected_wallet,reporter,category,description) VALUES($1,'message',$2,$3,$4,$5,'spam','reported chat')`, caseID, message.ID, h.pool, actor, addr(0xb0b)); err != nil {
		t.Fatal(err)
	}
	if _, err := h.dbConn.Exec(ctx, `INSERT INTO chat.safety_visibility(target_type,target_id,revision,status,case_id) VALUES('message',$1,1,'removed',$2)`, message.ID, caseID); err != nil {
		t.Fatal(err)
	}
	runtime := &socialConnectedRuntime{pool: h.app.pool, chat: h.app.dev.Chat}
	if err := runtime.visibilityOnce(ctx); err != nil {
		t.Fatal(err)
	}
	initialRevision := message.Revision
	if err := h.dbConn.QueryRow(ctx, `SELECT revision FROM chat.chat_messages WHERE id=$1`, message.ID).Scan(&message.Revision); err != nil || message.Revision != initialRevision+1 {
		t.Fatalf("stored removal revision = %d: %v", message.Revision, err)
	}
	removed, err := runtime.projectMessage(ctx, message)
	if err != nil || !removed.Deleted || removed.Content != "" || removed.Revision != message.Revision {
		t.Fatalf("removed message = %+v: %v", removed, err)
	}
	var events int
	if err := h.dbConn.QueryRow(ctx, `SELECT count(*) FROM chat.chat_outbox WHERE event_id=$1`, "safety_message_"+message.ID+"_1").Scan(&events); err != nil || events != 1 {
		t.Fatalf("removal outbox = %d: %v", events, err)
	}
	if _, err := h.dbConn.Exec(ctx, `UPDATE chat.safety_visibility SET revision=2,status='restored' WHERE target_type='message' AND target_id=$1`, message.ID); err != nil {
		t.Fatal(err)
	}
	if err := runtime.visibilityOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if err := h.dbConn.QueryRow(ctx, `SELECT revision FROM chat.chat_messages WHERE id=$1`, message.ID).Scan(&message.Revision); err != nil || message.Revision != initialRevision+2 {
		t.Fatalf("stored restoration revision = %d: %v", message.Revision, err)
	}
	restored, err := runtime.projectMessage(ctx, message)
	if err != nil || restored.Deleted || restored.Content != message.Content || restored.Revision != message.Revision {
		t.Fatalf("restored message = %+v: %v", restored, err)
	}
}
