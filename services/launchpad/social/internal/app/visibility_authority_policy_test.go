package app

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestVisibilityAuthorityReadsCommittedModerationPolicy(t *testing.T) {
	h := newPlatformHarness(t)
	ctx := context.Background()
	actor := h.wire.session("Bearer visibility-policy", addr(0xa11ce))
	runtime := &socialConnectedRuntime{pool: h.app.pool}
	caseID := uuid.NewString()
	if _, err := h.dbConn.Exec(ctx, `INSERT INTO chat.safety_cases(id,target_type,target_id,pool_id,affected_wallet,reporter,category,description) VALUES($1,'pool',$2,$2,$3,$4,'spam','community report')`, caseID, h.pool, h.creator, actor.Wallet); err != nil {
		t.Fatal(err)
	}
	if _, err := h.dbConn.Exec(ctx, `INSERT INTO chat.safety_visibility(target_type,target_id,revision,status,case_id) VALUES('pool',$1,1,'removed',$2)`, h.pool, caseID); err != nil {
		t.Fatal(err)
	}
	value, err := runtime.visibilityAuthority(ctx, "pool", h.pool, actor.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Visible  bool
		Revision uint64
	}
	if err := json.Unmarshal(raw, &result); err != nil || result.Visible || result.Revision != 1 {
		t.Fatalf("moderated pool = %+v: %v", result, err)
	}
	ranking, err := runtime.rankingVisibility(ctx)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = json.Marshal(ranking)
	if err != nil {
		t.Fatal(err)
	}
	var listed struct{ Pools map[string]bool }
	if err := json.Unmarshal(raw, &listed); err != nil || listed.Pools[h.pool] {
		t.Fatalf("moderated ranking pool = %+v: %v", listed, err)
	}
	if _, err := h.dbConn.Exec(ctx, `UPDATE chat.safety_visibility SET status='restored',revision=2 WHERE target_type='pool' AND target_id=$1`, h.pool); err != nil {
		t.Fatal(err)
	}
	value, err = runtime.visibilityAuthority(ctx, "pool", h.pool, actor.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &result); err != nil || !result.Visible || result.Revision != 2 {
		t.Fatalf("restored pool = %+v: %v", result, err)
	}
	tokenCase := uuid.NewString()
	if _, err := h.dbConn.Exec(ctx, `INSERT INTO chat.safety_cases(id,target_type,target_id,pool_id,affected_wallet,reporter,category,description) VALUES($1,'token',$2,$3,$4,$5,'spam','token report')`, tokenCase, h.token, h.pool, h.creator, actor.Wallet); err != nil {
		t.Fatal(err)
	}
	if _, err := h.dbConn.Exec(ctx, `INSERT INTO chat.safety_visibility(target_type,target_id,revision,status,case_id) VALUES('token',$1,1,'removed',$2)`, h.token, tokenCase); err != nil {
		t.Fatal(err)
	}
	token, err := runtime.visibilityAuthority(ctx, "token", h.token, actor.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = json.Marshal(token)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &result); err != nil || result.Visible || result.Revision != 1 {
		t.Fatalf("moderated token = %+v: %v", result, err)
	}
	h.wire.session("Bearer visibility-creator", h.creator)
	content, err := runtime.contentVisibility(ctx, "token", h.pool)
	if err != nil || content.Status != "allowed" {
		t.Fatalf("pool content authority = %+v: %v", content, err)
	}
	if _, err := h.dbConn.Exec(ctx, `INSERT INTO chat.safety_bans(id,wallet,scope,resource,actor) VALUES($1,$2,'global','',$3)`, uuid.NewString(), h.creator, actor.Wallet); err != nil {
		t.Fatal(err)
	}
	content, err = runtime.contentVisibility(ctx, "token", h.pool)
	if err == nil || content.Status != "" {
		t.Fatalf("banned creator content = %+v: %v", content, err)
	}
	ranking, err = runtime.rankingVisibility(ctx)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = json.Marshal(ranking)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &listed); err != nil || listed.Pools[h.pool] {
		t.Fatalf("moderated token ranking = %+v: %v", listed, err)
	}
}
