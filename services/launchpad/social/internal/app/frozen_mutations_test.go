package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/devsocial"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/interactions"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestFrozenCommentMutationsUsePersistedInteractionStore(t *testing.T) {
	h := newDevHarness(t)
	runtime := &socialConnectedRuntime{
		pool:   h.app.pool,
		store:  interactions.Store{DB: h.app.pool, Authority: &devsocial.Policy{DB: h.app.pool, Pools: devsocial.LegacyPools}},
		frozen: h.app.dev,
		safety: &safety.Store{DB: h.app.pool, Targets: socialTargetResolver{pool: h.app.pool}},
	}
	router := chi.NewRouter()
	if err := runtime.registerFrozen(router); err != nil {
		t.Fatal(err)
	}
	request := func(method, path, wallet string, body any) *httptest.ResponseRecorder {
		t.Helper()
		var payload []byte
		if body != nil {
			var err error
			payload, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(payload))
		if wallet != "" {
			req.Header.Set(devsocial.ActorHeader, wallet)
			principal, err := devsocial.Authenticate(req)
			if err != nil {
				t.Fatal(err)
			}
			ctx := fanout.WithPrincipal(req.Context(), principal)
			req = req.WithContext(ctx)
		}
		rw := httptest.NewRecorder()
		router.ServeHTTP(rw, req)
		return rw
	}
	actor := addr(0xa11ce)
	peer := addr(0xb0b)
	create := request(http.MethodPost, "/v2/pools/"+h.pool+"/comments", actor, map[string]any{"body": "persisted comment", "clientCommentId": uuid.NewString()})
	if create.Code != http.StatusCreated {
		t.Fatalf("create comment = %d %s", create.Code, create.Body.String())
	}
	var created struct {
		Data struct {
			Comment struct {
				ID       string `json:"commentId"`
				Revision string `json:"revision"`
			} `json:"comment"`
		} `json:"data"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil || created.Data.Comment.ID == "" || created.Data.Comment.Revision != "1" {
		t.Fatalf("created comment = %+v: %v", created, err)
	}
	id := created.Data.Comment.ID
	reportBody, err := json.Marshal(map[string]string{"targetType": "comment", "targetId": id, "reason": "spam", "evidence": "repeated unsolicited content"})
	if err != nil {
		t.Fatal(err)
	}
	reportRequest := httptest.NewRequest(http.MethodPost, "/v2/reports", bytes.NewReader(reportBody))
	reportRequest.Header.Set(devsocial.ActorHeader, actor)
	fanoutPrincipal, err := devsocial.Authenticate(reportRequest)
	if err != nil {
		t.Fatal(err)
	}
	reportContext := fanout.WithPrincipal(reportRequest.Context(), fanoutPrincipal)
	reportContext = safety.WithPrincipal(reportContext, safety.Principal{Wallet: actor, SessionID: "comment-report", Role: "participant", Expires: time.Now().Add(time.Hour)})
	report := httptest.NewRecorder()
	runtime.createFrozenReport(report, reportRequest.WithContext(reportContext))
	if report.Code != http.StatusCreated || !bytes.Contains(report.Body.Bytes(), []byte(`"targetId":"`+id+`"`)) {
		t.Fatalf("comment report = %d %s", report.Code, report.Body.String())
	}
	var reports int
	if err := h.app.pool.QueryRow(context.Background(), `SELECT count(*) FROM chat.safety_cases WHERE target_type='comment' AND target_id=$1`, id).Scan(&reports); err != nil || reports != 1 {
		t.Fatalf("stored comment report count = %d: %v", reports, err)
	}
	if _, err := h.dbConn.Exec(context.Background(), `INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload) VALUES($1,$2,$3,$4,$5,1,100,$6,$7,0,0,$8,'{}','{}')`, h.token, h.pool, h.creator, addr(0xfac), addr(0xb7), "0x"+strings.Repeat("ab", 32), "0x"+strings.Repeat("cd", 32), time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	calloutID := uuid.NewString()
	calloutBody := map[string]string{"clientCalloutId": calloutID, "poolAddress": h.pool, "body": "researched pool view", "direction": "neutral", "disclosure": "organic"}
	callout := request(http.MethodPost, "/v2/callouts", actor, calloutBody)
	if callout.Code != http.StatusCreated || !bytes.Contains(callout.Body.Bytes(), []byte(`"body":"researched pool view"`)) {
		t.Fatalf("callout create = %d %s", callout.Code, callout.Body.String())
	}
	replayed := request(http.MethodPost, "/v2/callouts", actor, calloutBody)
	var firstCallout, secondCallout struct {
		Data struct {
			Callout struct {
				ID string `json:"calloutId"`
			} `json:"callout"`
		} `json:"data"`
	}
	if err := json.Unmarshal(callout.Body.Bytes(), &firstCallout); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(replayed.Body.Bytes(), &secondCallout); err != nil {
		t.Fatal(err)
	}
	if replayed.Code != http.StatusCreated || firstCallout.Data.Callout.ID == "" || secondCallout.Data.Callout.ID != firstCallout.Data.Callout.ID {
		t.Fatalf("callout replay = %d %s", replayed.Code, replayed.Body.String())
	}
	var callouts int
	if err := h.app.pool.QueryRow(context.Background(), `SELECT count(*) FROM chat.v2_callouts WHERE author=$1 AND client_callout_id=$2`, actor, calloutID).Scan(&callouts); err != nil || callouts != 1 {
		t.Fatalf("stored callout count = %d: %v", callouts, err)
	}
	like := request(http.MethodPut, "/v2/comments/"+id+"/likes/me", peer, nil)
	if like.Code != http.StatusOK || !bytes.Contains(like.Body.Bytes(), []byte(`"likeCount":"1"`)) {
		t.Fatalf("like comment = %d %s", like.Code, like.Body.String())
	}
	edit := request(http.MethodPatch, "/v2/comments/"+id, actor, map[string]any{"body": "revised comment", "expectedRevision": "1"})
	if edit.Code != http.StatusOK || !bytes.Contains(edit.Body.Bytes(), []byte(`"body":"revised comment"`)) {
		t.Fatalf("edit comment = %d %s", edit.Code, edit.Body.String())
	}
	for _, tc := range []struct {
		method, path, wallet string
		body                 any
		status               int
	}{
		{http.MethodPost, "/v2/pools/" + h.pool + "/comments", "", map[string]any{"body": "unverified comment", "clientCommentId": uuid.NewString()}, http.StatusUnauthorized},
		{http.MethodPost, "/v2/pools/" + h.pool + "/comments", actor, map[string]any{"body": "missing client id"}, http.StatusUnprocessableEntity},
		{http.MethodPatch, "/v2/comments/" + id, peer, map[string]any{"body": "peer edit", "expectedRevision": "2"}, http.StatusForbidden},
		{http.MethodPatch, "/v2/comments/" + id, actor, map[string]any{"body": "stale edit", "expectedRevision": "1"}, http.StatusConflict},
		{http.MethodDelete, "/v2/comments/" + id, peer, nil, http.StatusForbidden},
		{http.MethodPut, "/v2/comments/" + uuid.NewString() + "/likes/me", peer, nil, http.StatusNotFound},
		{http.MethodPost, "/v2/callouts", "", calloutBody, http.StatusUnauthorized},
		{http.MethodPost, "/v2/callouts", actor, map[string]any{"body": "missing pool"}, http.StatusUnprocessableEntity},
	} {
		response := request(tc.method, tc.path, tc.wallet, tc.body)
		if response.Code != tc.status {
			t.Fatalf("%s %s = %d, want %d: %s", tc.method, tc.path, response.Code, tc.status, response.Body.String())
		}
	}
	unlike := request(http.MethodDelete, "/v2/comments/"+id+"/likes/me", peer, nil)
	if unlike.Code != http.StatusNoContent {
		t.Fatalf("unlike comment = %d %s", unlike.Code, unlike.Body.String())
	}
	remove := request(http.MethodDelete, "/v2/comments/"+id, actor, nil)
	if remove.Code != http.StatusNoContent {
		t.Fatalf("remove comment = %d %s", remove.Code, remove.Body.String())
	}
	var removed bool
	if err := h.app.pool.QueryRow(context.Background(), `SELECT removed FROM chat.interaction_comments WHERE id=$1`, id).Scan(&removed); err != nil || !removed {
		t.Fatalf("stored comment removed = %t: %v", removed, err)
	}
}
