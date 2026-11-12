package store

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestOperationalWritesRejectUnavailableDurableAuthority(t *testing.T) {
	ctx := context.Background()
	st, pool := lifecycleStore(t)
	scope := authorizeStoreOperation(t, pool, "backfill")
	waiver := authorizeStoreOperation(t, pool, "waive")
	repair := authorizeStoreOperation(t, pool, "repair")
	raw := lifecycleRaw()
	digest, err := RawDigest(raw)
	if err != nil {
		t.Fatal(err)
	}
	poison := PoisonRecord{ChainID: 125, StreamID: "canonical", Raw: raw, ErrorCode: "arity", ABIRevision: "v1", Requirement: "Swap", Required: true}
	if err = st.RetainPoison(ctx, poison); err != nil {
		t.Fatal(err)
	}
	request := ReplayRequest{JobID: "atomic-authority", SourceStream: "canonical", Target: scope.Target, Generation: scope.Generation, ProjectionRevision: scope.Revision, Mode: scope.Action, Requester: scope.Operator, AuthorizationID: scope.AuthorizationID, FromHeight: 101, ToHeight: 101, CheckpointHeight: 101, CheckpointHash: raw.BlockHash}
	if err = st.AdmitReplay(ctx, request); err != nil {
		t.Fatal(err)
	}
	block := BlockCompleteness{Height: 101, Hash: raw.BlockHash, RawCount: 1, FetchComplete: true, RequiredDecodeComplete: true}
	job, err := st.AcquireReplay(ctx, request.JobID, "worker", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	segment := ReplaySegment{FormatVersion: "raw-replay-v2", ChainID: 125, SourceStream: "canonical", FromHeight: 101, ToHeight: 101, Blocks: []BlockCompleteness{block}, Records: []RawRecord{raw}, Events: []ReplayEvent{}}
	calls := map[string]func(*Store) error{
		"admit":      func(s *Store) error { return s.AdmitReplay(ctx, request) },
		"acquire":    func(s *Store) error { _, e := s.AcquireReplay(ctx, request.JobID, "worker", time.Second); return e },
		"checkpoint": func(s *Store) error { return s.BindReplayCheckpoint(ctx, request, block) },
		"conflict":   func(s *Store) error { return s.RecordReplayConflict(ctx, request, segment, "checkpoint differs") },
		"transition": func(s *Store) error {
			return s.TransitionReplay(ctx, scope, request.JobID, "paused", "unavailable", nil, nil)
		},
		"resume": func(s *Store) error { return s.ResumeReplay(ctx, scope, request.JobID) },
		"pause":  func(s *Store) error { return s.PauseReplayLease(ctx, job, "unavailable", 101, 101) },
		"waive": func(s *Store) error {
			return s.WaivePoison(ctx, waiver, digest, "Swap", "reviewed", time.Now().Add(time.Minute))
		},
		"repair": func(s *Store) error { return s.ResolvePoison(ctx, repair, digest, raw.BlockHash) },
		"poison": func(s *Store) error { return s.RetainPoison(ctx, poison) },
	}
	cases := []struct{ name, op, sql string }{
		{"admission job schema", "admit", `ALTER TABLE indexer.indexer_replay_jobs RENAME TO unavailable_jobs`},
		{"admission acknowledgement", "admit", `DELETE FROM indexer.indexer_replay_jobs; CREATE FUNCTION pg_temp.reject_insert() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NULL; END $$; CREATE TRIGGER reject_insert BEFORE INSERT ON indexer.indexer_replay_jobs FOR EACH ROW EXECUTE FUNCTION pg_temp.reject_insert()`},
		{"acquire absent", "acquire", `DELETE FROM indexer.indexer_replay_jobs`},
		{"acquire malformed request", "acquire", `UPDATE indexer.indexer_replay_jobs SET request='[]'`},
		{"acquire update unavailable", "acquire", `CREATE FUNCTION pg_temp.reject_update() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'lease unavailable'; END $$; CREATE TRIGGER reject_update BEFORE UPDATE ON indexer.indexer_replay_jobs FOR EACH ROW EXECUTE FUNCTION pg_temp.reject_update()`},
		{"checkpoint schema", "checkpoint", `ALTER TABLE indexer.indexer_replay_checkpoints RENAME TO unavailable_checkpoints`},
		{"checkpoint acknowledgement", "checkpoint", `CREATE FUNCTION pg_temp.reject_insert() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NULL; END $$; CREATE TRIGGER reject_insert BEFORE INSERT ON indexer.indexer_replay_checkpoints FOR EACH ROW EXECUTE FUNCTION pg_temp.reject_insert()`},
		{"checkpoint disagreement", "checkpoint", `INSERT INTO indexer.indexer_replay_checkpoints VALUES('canonical',101,'0x` + strings.Repeat("9", 64) + `')`},
		{"conflict schema", "conflict", `ALTER TABLE indexer.indexer_replay_checkpoint_conflicts RENAME TO unavailable_conflicts`},
		{"transition schema", "transition", `ALTER TABLE indexer.indexer_replay_jobs RENAME TO unavailable_jobs`},
		{"resume schema", "resume", `ALTER TABLE indexer.indexer_replay_jobs RENAME TO unavailable_jobs`},
		{"pause schema", "pause", `ALTER TABLE indexer.indexer_replay_jobs RENAME TO unavailable_jobs`},
		{"waiver schema", "waive", `ALTER TABLE indexer.indexer_quarantine_waivers RENAME TO unavailable_waivers`},
		{"repair schema", "repair", `ALTER TABLE indexer.indexer_quarantine RENAME TO unavailable_poison`},
		{"poison raw schema", "poison", `ALTER TABLE indexer.indexer_raw_logs RENAME TO unavailable_raw`},
		{"poison schema", "poison", `ALTER TABLE indexer.indexer_quarantine RENAME TO unavailable_poison`},
	}
	for op := range calls {
		if op != "poison" {
			cases = append(cases, struct{ name, op, sql string }{op + " authorization", op, `UPDATE indexer.indexer_replay_authorizations SET revoked_at=clock_timestamp()`})
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tx, e := pool.Begin(ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(ctx)
			if _, e = tx.Exec(ctx, tc.sql); e != nil {
				t.Fatal(e)
			}
			if e = calls[tc.op](NewFromDBTX(tx)); e == nil {
				t.Fatal("unavailable durable authority admitted operation")
			}
		})
	}
	if _, err = pool.Exec(ctx, `ALTER TABLE indexer.indexer_capability_degradation RENAME TO unavailable_degradation`); err != nil {
		t.Fatal(err)
	}
	optional := poison
	optional.Required = false
	optional.Raw.LogIndex++
	if err = st.RetainPoison(ctx, optional); err == nil {
		t.Fatal("optional degradation acknowledged without durable record")
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM indexer.indexer_raw_logs`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("failed retention persisted extra raw rows %d: %v", count, err)
	}
}
