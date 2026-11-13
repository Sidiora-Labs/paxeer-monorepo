package app

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/google/uuid"
)

func TestPoolMessageVisibilityPublishesRemovalAndRestoration(t *testing.T) {
	h := newDevHarness(t)
	ctx := context.Background()
	actor := addr(0xa11ce)
	if err := h.app.dev.Chat.JoinPool(ctx, actor, h.pool); err != nil {
		t.Fatal(err)
	}
	message, err := h.app.dev.Chat.Send(ctx, fanout.SendInput{Actor: actor, Kind: "pool", Target: h.pool, ClientID: uuid.NewString(), Content: "community update"})
	if err != nil {
		t.Fatal(err)
	}
	caseID := uuid.NewString()
	if _, err := h.dbConn.Exec(ctx, `INSERT INTO chat.safety_cases(id,target_type,target_id,pool_id,affected_wallet,reporter,category,description) VALUES($1,'message',$2,$3,$4,$5,'spam','community report')`, caseID, message.ID, h.pool, actor, addr(0xb0b)); err != nil {
		t.Fatal(err)
	}
	if _, err := h.dbConn.Exec(ctx, `INSERT INTO chat.safety_visibility(target_type,target_id,revision,status,case_id) VALUES('message',$1,1,'removed',$2)`, message.ID, caseID); err != nil {
		t.Fatal(err)
	}
	runtime := &socialConnectedRuntime{pool: h.app.pool, chat: h.app.dev.Chat}
	for _, state := range []struct {
		status  string
		removed bool
		rev     int64
	}{
		{"removed", true, 1},
		{"restored", false, 2},
	} {
		if state.rev > 1 {
			if _, err := h.dbConn.Exec(ctx, `UPDATE chat.safety_visibility SET revision=$2,status=$3 WHERE target_type='message' AND target_id=$1`, message.ID, state.rev, state.status); err != nil {
				t.Fatal(err)
			}
		}
		if err := runtime.visibilityOnce(ctx); err != nil {
			t.Fatal(err)
		}
		var projectionRevision int64
		var removed bool
		if err := h.dbConn.QueryRow(ctx, `SELECT revision,removed FROM chat.interaction_feed_projection WHERE generation=$1 AND aggregate_id=$2`, safetyProjectionGeneration, "message:"+message.ID).Scan(&projectionRevision, &removed); err != nil || projectionRevision != state.rev || removed != state.removed {
			t.Fatalf("projection = revision %d removed %t: %v", projectionRevision, removed, err)
		}
		var payload []byte
		var channels []byte
		if err := h.dbConn.QueryRow(ctx, `SELECT payload,channels FROM chat.chat_outbox WHERE event_id=$1`, "safety_message_"+message.ID+"_"+strconv.FormatInt(state.rev, 10)).Scan(&payload, &channels); err != nil {
			t.Fatal(err)
		}
		var published fanout.DurableMessage
		var targets []string
		if err := json.Unmarshal(payload, &published); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(channels, &targets); err != nil {
			t.Fatal(err)
		}
		if published.ID != message.ID || published.Deleted != state.removed || published.PoolAddress != h.pool || len(targets) != 1 || targets[0] != "pool:"+h.pool {
			t.Fatalf("published pool visibility = %+v, targets %v", published, targets)
		}
		if state.removed && published.Content != "" || !state.removed && published.Content != message.Content {
			t.Fatalf("published body = %q for %s", published.Content, state.status)
		}
	}
	if err := runtime.visibilityOnce(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := h.dbConn.QueryRow(ctx, `SELECT count(*) FROM chat.chat_outbox WHERE message_id=$1 AND event_id LIKE 'safety_message_%'`, message.ID).Scan(&count); err != nil || count != 2 {
		t.Fatalf("idempotent visibility events = %d: %v", count, err)
	}
}
