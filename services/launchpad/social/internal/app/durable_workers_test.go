package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/interactions"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/voice"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSocialSafetyOutboxStagesCommittedPolicyEvent(t *testing.T) {
	h := newPlatformHarness(t)
	ctx := context.Background()
	pool := socialWorkerPool(t, h.db.AdminDSN)
	policy := delivery.Policy{
		Subjects:   map[string]bool{"social.policy": true},
		EventTypes: map[string]bool{socialPolicyChangedEvent: true},
	}
	store, err := delivery.NewStore(pool, "chat", policy)
	if err != nil {
		t.Fatal(err)
	}
	runtime := &socialConnectedRuntime{
		pool: pool, delivery: store,
		sourceOrder: interactions.CanonicalSourceOrder{Positions: pgx.Identifier{"indexer", "indexer_raw_logs_v1"}},
	}
	runtime.binding.PolicySubject = "social.policy"
	if _, err := h.dbConn.Exec(ctx, `INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,0,101,0,$2,$3,'[]','\x'::bytea)`, "0x"+strings.Repeat("a", 64), "0x"+strings.Repeat("b", 64), h.creator); err != nil {
		t.Fatal(err)
	}
	if _, err := h.dbConn.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,'social',101,$1,1,true,true)`, "0x"+strings.Repeat("b", 64)); err != nil {
		t.Fatal(err)
	}
	caseID := uuid.NewString()
	if _, err := h.dbConn.Exec(ctx, `INSERT INTO chat.safety_cases(id,target_type,target_id,reporter,category,description) VALUES($1,'message','policy-message',$2,'spam','reported message')`, caseID, h.creator); err != nil {
		t.Fatal(err)
	}
	visibility := map[string]any{"target_type": "message", "target_id": "policy-message", "status": "removed", "revision": 2, "case_id": caseID}
	payload, err := json.Marshal(visibility)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.NewString()
	if _, err := h.dbConn.Exec(ctx, `INSERT INTO chat.safety_outbox(id,kind,entity_id,revision,payload) VALUES($1,'visibility_changed','policy-message',2,$2)`, id, payload); err != nil {
		t.Fatal(err)
	}
	if err := runtime.stageSafetyOutbox(ctx); err != nil {
		t.Fatal(err)
	}
	var published bool
	if err := h.dbConn.QueryRow(ctx, `SELECT published_at IS NOT NULL FROM chat.safety_outbox WHERE id=$1`, id).Scan(&published); err != nil || !published {
		t.Fatalf("policy outbox publication = %t: %v", published, err)
	}
	leases, err := store.Claim(ctx, "social_test", 1, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(leases) != 1 || leases[0].EventID != "social_policy_"+strings.ReplaceAll(id, "-", "") || leases[0].Subject != "social.policy" {
		t.Fatalf("queued policy event = %+v", leases)
	}
	queued, err := delivery.Decode(leases[0].Wire, policy, "social.policy")
	if err != nil || queued.EventType != socialPolicyChangedEvent {
		t.Fatalf("policy envelope = %+v: %v", queued, err)
	}
	if err := runtime.stageSafetyOutbox(ctx); err != nil {
		t.Fatal(err)
	}
	stats, err := store.Stats(ctx)
	if err != nil || stats.PendingRows != 1 {
		t.Fatalf("policy events after replay = %+v: %v", stats, err)
	}
}

func socialWorkerPool(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = "chat,public"
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestSocialErasureWorkerAdvancesCommittedJob(t *testing.T) {
	h := newPlatformHarness(t)
	ctx := context.Background()
	pool := socialWorkerPool(t, h.db.AdminDSN)
	caseID := uuid.NewString()
	wallet := addr(0xe2a5e)
	if _, err := h.dbConn.Exec(ctx, `INSERT INTO chat.interaction_erasure_jobs(deletion_case_id,wallet,policy_version) VALUES($1,$2,'account-v2')`, caseID, wallet); err != nil {
		t.Fatal(err)
	}
	runtime := &socialConnectedRuntime{
		pool:  pool,
		store: interactions.Store{DB: pool},
		voice: &voice.Control{DB: pool},
	}
	if err := runtime.eraseOnce(ctx); err != nil {
		t.Fatal(err)
	}
	var stage int
	if err := h.dbConn.QueryRow(ctx, `SELECT stage FROM chat.interaction_erasure_jobs WHERE deletion_case_id=$1`, caseID).Scan(&stage); err != nil || stage != 1 {
		t.Fatalf("erasure stage = %d: %v", stage, err)
	}
	roomID := uuid.NewString()
	if _, err := h.dbConn.Exec(ctx, `INSERT INTO chat.voice_rooms(id,pool_id,host_wallet,start_key,state) VALUES($1,$2,$3,$4,'live')`, roomID, h.pool, wallet, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if err := runtime.eraseOnce(ctx); !errors.Is(err, voice.ErrUnavailable) {
		t.Fatalf("erasure with active provider grant = %v", err)
	}
	if err := h.dbConn.QueryRow(ctx, `SELECT stage FROM chat.interaction_erasure_jobs WHERE deletion_case_id=$1`, caseID).Scan(&stage); err != nil || stage != 1 {
		t.Fatalf("blocked erasure stage = %d: %v", stage, err)
	}
	if _, err := h.dbConn.Exec(ctx, `DELETE FROM chat.voice_rooms WHERE id=$1`, roomID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 15; i++ {
		if err := runtime.eraseOnce(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var state string
	if err := h.dbConn.QueryRow(ctx, `SELECT state FROM chat.interaction_erasure_jobs WHERE deletion_case_id=$1`, caseID).Scan(&state); err != nil || state != "completed" {
		t.Fatalf("resumed erasure state = %q: %v", state, err)
	}
}
