package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/voice"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/secrets"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestRuntimeReadinessTracksConsumedSecrets(t *testing.T) {
	ctx := context.Background()
	const reference = "SOCIAL_READINESS_CREDENTIAL"
	t.Setenv(reference, "current-value")
	runtime := &socialConnectedRuntime{secrets: map[string]string{reference: "current-value"}}
	if err := runtime.secretsReady(ctx); err == nil || err.Error() != "social secret authority unavailable" {
		t.Fatalf("unbound credential authority = %v", err)
	}
	runtime.provider = secrets.EnvProvider{}
	if err := runtime.secretsReady(ctx); err != nil {
		t.Fatalf("current credential = %v", err)
	}
	for _, value := range []string{"rotated-value", ""} {
		t.Setenv(reference, value)
		if err := runtime.secretsReady(ctx); err == nil || err.Error() != "social secret rotated or unavailable" {
			t.Fatalf("changed credential readiness = %v", err)
		}
	}
	t.Setenv(reference, "current-value")
	if err := runtime.secretsReady(ctx); err != nil {
		t.Fatalf("restored credential = %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := runtime.secretsReady(canceled); err == nil {
		t.Fatal("canceled credential check admitted readiness")
	}
}

func TestVoiceReadinessRequiresProviderPolicyAndQualification(t *testing.T) {
	h := newPlatformHarness(t)
	ctx := context.Background()
	runtime := &socialConnectedRuntime{binding: config.SocialRuntimeBinding{Voice: &config.VoiceRuntimeBinding{}}}
	if err := runtime.voiceReady(ctx); !errors.Is(err, voice.ErrUnavailable) {
		t.Fatalf("missing voice control = %v", err)
	}
	runtime.voice = &voice.Control{DB: h.app.pool}
	if err := runtime.voiceReady(ctx); !errors.Is(err, voice.ErrUnavailable) {
		t.Fatalf("missing voice provider = %v", err)
	}
	provider, err := voice.NewLiveKit("https://"+strings.Join([]string{"fixture", "livekit", "cloud"}, "."), t.Name(), strings.Repeat("s", 32), voice.S3RecordingOutput{})
	if err != nil {
		t.Fatal(err)
	}
	runtime.voice.Provider = provider
	if err := runtime.voiceReady(ctx); !errors.Is(err, voice.ErrUnavailable) {
		t.Fatalf("missing voice policy = %v", err)
	}
	runtime.voice.Policy = &safety.Store{DB: h.app.pool}
	if err := runtime.voiceReady(ctx); !errors.Is(err, voice.ErrUnavailable) {
		t.Fatalf("unqualified voice control = %v", err)
	}
	runtime.voice.Gates = voice.Gates{ProviderQualified: true, EjectionQualified: true, ACLQualified: true, RetentionQualified: true}
	if err := runtime.voiceReady(ctx); err != nil {
		t.Fatalf("qualified voice control = %v", err)
	}
	pools := socialPools{DB: h.app.pool, Guard: pgx.Identifier{"chat", "interaction_pool_creator"}}
	for _, tc := range []struct {
		pool string
		want bool
	}{{h.pool, true}, {addr(0xdead), false}} {
		exists, err := pools.Exists(ctx, tc.pool)
		if err != nil || exists != tc.want {
			t.Fatalf("voice pool membership = %t: %v, want %t", exists, err, tc.want)
		}
	}
}

func TestVisibilityOverlayRejectsMismatchedAuthorityAndIgnoresReplay(t *testing.T) {
	h := newPlatformHarness(t)
	ctx := context.Background()
	runtime := &socialConnectedRuntime{pool: h.app.pool}
	if err := runtime.applyVisibilityOverlay(ctx, nil, safety.Visibility{}); !errors.Is(err, safety.ErrUnavailable) {
		t.Fatalf("unbound visibility transaction = %v", err)
	}
	tx, err := h.dbConn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil {
			t.Error(err)
		}
	}()
	for _, value := range []safety.Visibility{
		{TargetType: "message", TargetID: "message", Status: "removed"},
		{TargetType: "message", Revision: 1, Status: "removed"},
		{TargetType: "message", TargetID: "message", Revision: 1, Status: "unknown"},
	} {
		if err := runtime.applyVisibilityOverlay(ctx, tx, value); !errors.Is(err, safety.ErrInvalid) {
			t.Fatalf("invalid visibility = %v", err)
		}
	}
	if err := runtime.applyVisibilityOverlay(ctx, tx, safety.Visibility{TargetType: "pool", TargetID: h.pool, Status: "removed", Revision: 1}); err != nil {
		t.Fatalf("pool visibility delegation = %v", err)
	}
	value := safety.Visibility{TargetType: "message", TargetID: "message", Status: "removed", Revision: 1}
	if err := runtime.applyVisibilityOverlay(ctx, tx, value); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("uncommitted visibility authority = %v", err)
	}
	caseID := uuid.NewString()
	if _, err := tx.Exec(ctx, `INSERT INTO chat.safety_cases(id,target_type,target_id,reporter,category,description) VALUES($1,'message',$2,$3,'spam','reported message')`, caseID, value.TargetID, h.creator); err != nil {
		t.Fatal(err)
	}
	value.CaseID = caseID
	if _, err := tx.Exec(ctx, `INSERT INTO chat.safety_visibility(target_type,target_id,status,case_id,revision) VALUES($1,$2,$3,$4,2)`, value.TargetType, value.TargetID, value.Status, value.CaseID); err != nil {
		t.Fatal(err)
	}
	if err := runtime.applyVisibilityOverlay(ctx, tx, value); err != nil {
		t.Fatalf("stale visibility replay = %v", err)
	}
	value.Revision = 2
	value.Status = "restored"
	if err := runtime.applyVisibilityOverlay(ctx, tx, value); !errors.Is(err, safety.ErrConflict) {
		t.Fatalf("mismatched visibility authority = %v", err)
	}
	value.Status = "removed"
	if _, err := tx.Exec(ctx, `INSERT INTO chat.interaction_feed_projection(generation,aggregate_id,revision,actor,removed,payload) VALUES($1,$2,2,$3,true,'{}')`, safetyProjectionGeneration, "message:"+value.TargetID, h.creator); err != nil {
		t.Fatal(err)
	}
	if err := runtime.applyVisibilityOverlay(ctx, tx, value); err != nil {
		t.Fatalf("applied visibility replay = %v", err)
	}
	var outboxRows int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM chat.chat_outbox WHERE message_id=$1`, value.TargetID).Scan(&outboxRows); err != nil || outboxRows != 0 {
		t.Fatalf("replayed visibility outbox rows = %d: %v", outboxRows, err)
	}
}
