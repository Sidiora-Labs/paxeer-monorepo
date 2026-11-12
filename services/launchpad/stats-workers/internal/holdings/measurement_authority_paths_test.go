package holdings

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

func TestPhase4ObservationEnforcesPendingAndProjectedFreshness(t *testing.T) {
	ctx, pool, mint := canonicalLocalFixture(t)
	if _, err := pool.Exec(ctx, string(mustPhase4Migration(t, "60001"))); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_committed_heads(chain_id,stream_id,height,block_hash,integrity,source_id) VALUES(125,'canonical',10,$1,'healthy','indexer')`, mint.BlockHash); err != nil {
		t.Fatal(err)
	}
	ddl, err := SchemaSQL("observation_bounds")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	repo, err := NewRepository(pool, "observation_bounds")
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.DiscoverCanonical(ctx, mint.Token, "canonical"); err != nil {
		t.Fatal(err)
	}
	nc := internaltest.ConnectNATS(t, os.Getenv("L20_NATS_URL"))
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	stream := "FRESHNESS_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = js.AddStream(&nats.StreamConfig{Name: stream, Subjects: []string{stream + ".events"}, Storage: nats.FileStorage, MaxBytes: 1048576}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := js.DeleteStream(stream); err != nil {
			t.Error(err)
		}
	})
	if _, err = js.AddConsumer(stream, &nats.ConsumerConfig{Durable: "observe", AckPolicy: nats.AckExplicitPolicy}); err != nil {
		t.Fatal(err)
	}
	plan := Phase4Plan{SourceStream: "canonical", HolderSchema: "observation_bounds", Pools: []Phase4Pool{{Token: mint.Token}}, BrokerStream: stream, BrokerConsumer: "observe", MaxDatabaseLockWaiters: 1000000, MaxProjectionLagMS: 3600000}
	observe := func(p Phase4Plan, want string) {
		t.Helper()
		a := &phase4Artifacts{directory: t.TempDir(), files: map[string]*os.File{}}
		backlog, err := phase4Observe(ctx, pool, js, p, a, "normal", time.Now().Add(-time.Hour))
		if err := a.close(); err != nil {
			t.Fatal(err)
		}
		if want == "" {
			if err != nil || backlog != 0 {
				t.Fatalf("observed freshness: %d %v", backlog, err)
			}
		} else if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("freshness rejection: %v, want %s", err, want)
		}
	}
	observe(plan, "")
	bounded := plan
	bounded.MaxProjectionLagMS = 0
	observe(bounded, "unprojected accepted event freshness bound failed")
	if batch, err := repo.CatchUpCanonical(ctx, mint.Token, "canonical", 10); err != nil || batch.Applied != 1 {
		t.Fatalf("canonical projection: %+v %v", batch, err)
	}
	observe(plan, "")
	observe(bounded, "measured projection lag bound failed")
	bounded = plan
	bounded.MaxDatabaseLockWaiters = -1
	observe(bounded, "database lock or commit timestamp authority bound failed")
	bounded = plan
	bounded.BrokerConsumer = "absent"
	observe(bounded, "consumer not found")
	for _, relation := range []string{"indexer.indexer_ingest_watermarks_v1", "indexer.indexer_ingest_leases_v1", "observation_bounds.transfers", "indexer.indexer_token_transfers_v1"} {
		t.Run(relation, func(t *testing.T) {
			parts := strings.Split(relation, ".")
			kind := "VIEW"
			if parts[1] == "transfers" {
				kind = "TABLE"
			}
			if _, err := pool.Exec(ctx, "ALTER "+kind+" "+relation+" RENAME TO observation_saved"); err != nil {
				t.Fatal(err)
			}
			observe(plan, "does not exist")
			if _, err := pool.Exec(ctx, "ALTER "+kind+" "+parts[0]+".observation_saved RENAME TO "+parts[1]); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPhase4MeasurementRequiresBoundInputsAndFreshOutput(t *testing.T) {
	plan := phase4TestPlan(t, "http://127.0.0.1:8080")
	for _, tc := range []struct {
		name     string
		edit     func(*Phase4Plan)
		raw      string
		output   string
		approval bool
		want     string
	}{
		{name: "absent input", raw: "missing", want: "no such file"},
		{name: "invalid JSON", raw: "{", want: "unexpected end of JSON input"},
		{name: "invalid plan", edit: func(p *Phase4Plan) { p.Candidate = "invalid" }, want: "binding missing"},
		{name: "unapproved plan", want: "approved runtime"},
		{name: "changed source", edit: func(p *Phase4Plan) {
			for k := range p.Sources {
				p.Sources[k] = strings.Repeat("0", 64)
				break
			}
		}, approval: true, want: "checksum mismatch"},
		{name: "relative output", output: "measurement", approval: true, want: "exact private output"},
		{name: "existing output", output: "existing", approval: true, want: "file exists"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := plan
			if tc.edit != nil {
				raw, err := json.Marshal(plan)
				if err != nil {
					t.Fatal(err)
				}
				p = Phase4Plan{}
				if err = json.Unmarshal(raw, &p); err != nil {
					t.Fatal(err)
				}
				tc.edit(&p)
			}
			raw, err := json.Marshal(p)
			if err != nil {
				t.Fatal(err)
			}
			if tc.raw != "" {
				raw = []byte(tc.raw)
			}
			dir := t.TempDir()
			path := filepath.Join(dir, "plan.json")
			if tc.raw != "missing" {
				if err = os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			output := filepath.Join(dir, "measurement")
			if tc.output == "measurement" {
				output = tc.output
			}
			if tc.output == "existing" {
				output = dir
			}
			for k, v := range map[string]string{"L14_PHASE4_APPROVED_CANDIDATE": p.Candidate, "L14_PHASE4_APPROVED_INTERFACE": p.Interface, "L14_PHASE4_APPROVED_TOPOLOGY": p.Topology} {
				if !tc.approval {
					v = ""
				}
				t.Setenv(k, v)
			}
			if err := RunPhase4Measurement(context.Background(), path, phase4Hash(raw), output); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("measurement admission: %v, want %s", err, tc.want)
			}
		})
	}
}
