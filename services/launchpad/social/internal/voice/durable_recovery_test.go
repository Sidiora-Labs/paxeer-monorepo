package voice

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func withVoiceWriteConstraint(t *testing.T, c *Control, table, expression string, run func()) {
	t.Helper()
	name := pgx.Identifier{table}.Sanitize()
	if _, err := c.DB.Exec(context.Background(), "ALTER TABLE "+name+" ADD CONSTRAINT voice_write_guard CHECK ("+expression+") NOT VALID"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := c.DB.Exec(context.Background(), "ALTER TABLE "+name+" DROP CONSTRAINT voice_write_guard"); err != nil {
			t.Fatal(err)
		}
	}()
	run()
}

func assertVoiceWriteRejected(t *testing.T, err error) {
	t.Helper()
	var postgres *pgconn.PgError
	if !errors.As(err, &postgres) || postgres.Code != "23514" {
		t.Fatalf("rejected voice write = %v", err)
	}
}

func TestVoiceLifecycleRecoversWhenDurableTransitionsAreRejected(t *testing.T) {
	c, ctx := fixture(t)
	provider := localLiveKit(t)
	c.Provider = provider
	c.Gates = Gates{true, true, true, true}
	room, err := c.Create(ctx, "known-pool")
	if err != nil {
		t.Fatal(err)
	}
	withVoiceWriteConstraint(t, c, "voice_rooms", "state <> 'live'", func() {
		_, err := c.Start(ctx, room.ID, 1)
		assertVoiceWriteRejected(t, err)
	})
	stored, err := c.Read(ctx, room.ID)
	if err != nil || stored.State != draftState || stored.Revision != 1 || stored.ProviderSession != "" {
		t.Fatalf("uncommitted start = %+v: %v", stored, err)
	}
	session, found, err := provider.Find(ctx, room.StartKey)
	if err != nil || !found || session == "" {
		t.Fatalf("recoverable provider session = %q/%t: %v", session, found, err)
	}
	live, err := c.Start(ctx, room.ID, 1)
	if err != nil || live.ProviderSession != session || live.Revision != 2 {
		t.Fatalf("recovered start = %+v: %v", live, err)
	}
	if err := c.Join(ctx, room.ID); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, table, expression string
	}{
		{"ending intent", "voice_rooms", "state <> 'ending'"},
		{"participant revocation", "voice_participants", "NOT revoked"},
		{"ended state", "voice_rooms", "state <> 'ended'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			current, err := c.Read(ctx, room.ID)
			if err != nil {
				t.Fatal(err)
			}
			withVoiceWriteConstraint(t, c, tc.table, tc.expression, func() {
				_, err := c.End(ctx, room.ID, current.Revision)
				assertVoiceWriteRejected(t, err)
			})
			stored, err := c.Read(ctx, room.ID)
			if err != nil {
				t.Fatal(err)
			}
			wantState, wantRevision := endingState, int64(3)
			if tc.name == "ending intent" {
				wantState, wantRevision = liveState, 2
			}
			var revoked, muted bool
			var revision int64
			if err := c.DB.QueryRow(ctx, `SELECT revoked,muted,revision FROM voice_participants WHERE room_id=$1 AND wallet=$2`, room.ID, host).Scan(&revoked, &muted, &revision); err != nil {
				t.Fatal(err)
			}
			if stored.State != wantState || stored.Revision != wantRevision || revoked || muted || revision != 1 {
				t.Fatalf("partial end: room=%+v revoked=%t muted=%t revision=%d", stored, revoked, muted, revision)
			}
		})
	}
	restarted := &Control{DB: c.DB, Provider: provider}
	ended, err := restarted.End(ctx, room.ID, 3)
	if err != nil || ended.State != "ended" || ended.Revision != 4 {
		t.Fatalf("recovered end = %+v: %v", ended, err)
	}
	var revoked bool
	var grant string
	if err := c.DB.QueryRow(ctx, `SELECT revoked,grant_state FROM voice_participants WHERE room_id=$1 AND wallet=$2`, room.ID, host).Scan(&revoked, &grant); err != nil || !revoked || grant != "revoked" {
		t.Fatalf("recovered participant = %t/%s: %v", revoked, grant, err)
	}
}

func TestVoiceParticipantWritesPreserveAuthorityOnRejectedTransitions(t *testing.T) {
	c, ctx := fixture(t)
	c.Provider = localLiveKit(t)
	c.Gates = Gates{true, true, true, true}
	room, err := c.Create(ctx, "known-pool")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Start(ctx, room.ID, 1); err != nil {
		t.Fatal(err)
	}
	withVoiceWriteConstraint(t, c, "voice_participants", "false", func() {
		assertVoiceWriteRejected(t, c.Join(ctx, room.ID))
	})
	if err := c.Join(ctx, room.ID); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"speaker request", func() error { return c.RequestSpeaker(ctx, room.ID, 1) }},
		{"speaker approval", func() error { return c.ApproveSpeaker(ctx, room.ID, host, 1) }},
		{"mute", func() error { return c.Mute(ctx, room.ID, host, 1, false) }},
		{"leave", func() error { return c.Leave(ctx, room.ID) }},
		{"account erasure", func() error { return c.DeleteAccount(ctx, host) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			table, expression := "voice_participants", "false"
			if tc.name == "account erasure" {
				table, expression = "voice_rooms", "state <> 'ending'"
			}
			withVoiceWriteConstraint(t, c, table, expression, func() {
				assertVoiceWriteRejected(t, tc.run())
			})
			var role, grant string
			var muted, revoked bool
			var revision int64
			if err := c.DB.QueryRow(ctx, `SELECT role,grant_state,muted,revoked,revision FROM voice_participants WHERE room_id=$1 AND wallet=$2`, room.ID, host).Scan(&role, &grant, &muted, &revoked, &revision); err != nil {
				t.Fatal(err)
			}
			if role != "listener" || grant != "active" || muted || revoked || revision != 1 {
				t.Fatalf("rejected participant write: %s/%s muted=%t revoked=%t revision=%d", role, grant, muted, revoked, revision)
			}
		})
	}
	c.Policy = &safety.Store{DB: c.DB}
	withoutVoiceRelation(t, c, "safety_bans", func() {
		for _, tc := range []struct {
			name string
			run  func() error
		}{
			{"join", func() error { return c.Join(ctx, room.ID) }},
			{"speaker request", func() error { return c.RequestSpeaker(ctx, room.ID, 1) }},
			{"speaker approval", func() error { return c.ApproveSpeaker(ctx, room.ID, host, 1) }},
			{"ticket", func() error { _, err := c.Ticket(ctx, room.ID); return err }},
		} {
			t.Run("policy/"+tc.name, func(t *testing.T) {
				if err := tc.run(); !errors.Is(err, ErrUnavailable) {
					t.Fatalf("unavailable policy decision = %v", err)
				}
			})
		}
	})
	if err := c.Leave(ctx, room.ID); err != nil {
		t.Fatalf("recovered leave = %v", err)
	}
}

func TestVoicePolicyUpdatesRespectBanRevisionAndRecoverProviderRevocation(t *testing.T) {
	c, ctx := fixture(t)
	provider := localLiveKit(t)
	c.Provider = provider
	c.Gates = Gates{true, true, true, true}
	room, err := c.Create(ctx, "known-pool")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Start(ctx, room.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err := c.Join(ctx, room.ID); err != nil {
		t.Fatal(err)
	}
	ban := uuid.NewString()
	if _, err := c.DB.Exec(ctx, `INSERT INTO safety_bans(id,wallet,scope,resource,actor,revision)VALUES($1,$2,'voice',$3,$2,2)`, ban, host, room.ID); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []int64{0, 3} {
		if err := c.ApplyPolicyUpdate(ctx, ban, expected); !errors.Is(err, safety.ErrConflict) {
			t.Fatalf("invalid ban revision %d = %v", expected, err)
		}
	}
	if err := c.ApplyPolicyUpdate(ctx, ban, 1); err != nil {
		t.Fatalf("superseded ban event = %v", err)
	}
	withoutVoiceRelation(t, c, "voice_participants", func() {
		var postgres *pgconn.PgError
		if err := c.ApplyPolicyUpdate(ctx, ban, 2); !errors.As(err, &postgres) || postgres.Code != "42P01" {
			t.Fatalf("policy participant dependency = %v", err)
		}
	})
	invalid := *provider
	invalid.APIKey = ""
	c.Provider = &invalid
	if err := c.ApplyPolicyUpdate(ctx, ban, 2); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("provider revocation unavailable = %v", err)
	}
	var muted bool
	var grant string
	var revision int64
	if err := c.DB.QueryRow(ctx, `SELECT muted,grant_state,revision FROM voice_participants WHERE room_id=$1 AND wallet=$2`, room.ID, host).Scan(&muted, &grant, &revision); err != nil || !muted || grant != "revoking" || revision != 2 {
		t.Fatalf("pending policy revocation = %t/%s/%d: %v", muted, grant, revision, err)
	}
	c.Provider = provider
	if err := c.ApplyPolicyUpdate(ctx, ban, 2); err != nil {
		t.Fatal(err)
	}
	if err := c.DB.QueryRow(ctx, `SELECT grant_state,revision FROM voice_participants WHERE room_id=$1 AND wallet=$2`, room.ID, host).Scan(&grant, &revision); err != nil || grant != "revoked" || revision != 3 {
		t.Fatalf("recovered policy revocation = %s/%d: %v", grant, revision, err)
	}
	if _, err := c.DB.Exec(ctx, `UPDATE safety_bans SET revoked_at=$2 WHERE id=$1`, ban, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := c.ApplyPolicyUpdate(ctx, ban, 2); err != nil {
		t.Fatalf("revoked ban event = %v", err)
	}
}
