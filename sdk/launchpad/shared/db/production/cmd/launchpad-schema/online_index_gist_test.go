package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting/sourcebootstrap/indexedtest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
)

func TestBootstrapIndexedCommitsApplyAndReadiness(t *testing.T) {
	if os.Getenv("COORDINATOR_TEST_POSTGRES_DSN") == "" {
		t.Fatal("COORDINATOR_TEST_POSTGRES_DSN is required")
	}
	directory := filepath.Join(coordinatortest.RepositoryRoot(t), "migrations")
	bundle, err := loadBundle(directory)
	if err != nil {
		t.Fatal(err)
	}
	registered := false
	for _, prerequisite := range bundle.registry.Extensions {
		if prerequisite.Extension == "btree_gist" {
			if prerequisite.Schema != "public" || prerequisite.Version != nil || prerequisite.RuntimeInstall {
				t.Fatal("GiST operator classes require an administrator-installed public extension")
			}
			registered = true
		}
	}
	if !registered {
		t.Fatal("GiST operator classes require a registered extension prerequisite")
	}
	roles := append([]production.RoleManifest(nil), bundle.roles...)
	for _, owner := range bundle.owners {
		roles = append(roles, production.RoleManifest{OwnerRole: owner, RuntimeRole: owner})
	}
	dsn, conn := emptyDatabase(t, testTag(t), roles)
	major, err := serverMajorVersion(t.Context(), conn)
	if err != nil || major != 18 {
		t.Fatalf("PostgreSQL major version: %d %v", major, err)
	}
	var installed bool
	if err := conn.QueryRow(t.Context(), "SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname='btree_gist')").Scan(&installed); err != nil || installed {
		t.Fatalf("empty database extension state: %t %v", installed, err)
	}
	t.Setenv("LAUNCHPAD_ADMIN_DATABASE_URL", dsn)
	command(t, "apply", "--migrations", directory)
	var schema string
	var administratorOwned bool
	if err := conn.QueryRow(t.Context(), "SELECT n.nspname,pg_get_userbyid(e.extowner)=current_user FROM pg_extension e JOIN pg_namespace n ON n.oid=e.extnamespace WHERE e.extname='btree_gist'").Scan(&schema, &administratorOwned); err != nil || schema != "public" || !administratorOwned {
		t.Fatalf("administrator-installed GiST prerequisite: %s %t %v", schema, administratorOwned, err)
	}
	command(t, "region", "--migrations", directory, "--region", "local-region")
	for _, service := range []struct{ service, role string }{
		{"core/stats-workers", "stats_runtime"},
		{"core/trading-charts", "candles_runtime"},
		{"core/api", "api_rest_runtime"},
	} {
		t.Run(service.role, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "expected.json")
			digest, err := bundle.expected(t.Context(), conn, service.service, service.role, "local-region", path)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			want, err := production.LoadExpectedSchema(raw, digest)
			if err != nil || want.Migrations[60009] != bundle.catalog[60009].SHA256 {
				t.Fatalf("source commit index readiness: %v", err)
			}
		})
	}
	command(t, "apply", "--migrations", directory)
}

func TestBootstrapIndexedCommitsOnlineIndex(t *testing.T) {
	f := indexedtest.Open(t, "COORDINATOR_TEST_POSTGRES_DSN", "api_rest_runtime")
	conn, err := f.Admin.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	columns := []string{"chain_id", "stream_id", "previous_height", "last_complete_height", "block_hash", "record_count", "batch_id"}
	plan := production.OnlineIndexPlan{Method: "gist", Schema: "indexer", Table: "indexer_ingest_commits", Name: "indexer_ingest_commits_height", Columns: columns}
	state, err := production.InspectOnlineIndex(t.Context(), conn.Conn(), plan)
	if err != nil || !state.Exists || !state.Valid || !state.Ready || !state.DefinitionMatches || state.Method != "gist" || !reflect.DeepEqual(state.Columns, columns) {
		t.Fatalf("source height index: %+v %v", state, err)
	}
	var schema string
	if err := conn.QueryRow(t.Context(), "SELECT n.nspname FROM pg_extension e JOIN pg_namespace n ON n.oid=e.extnamespace WHERE e.extname='btree_gist'").Scan(&schema); err != nil || schema != "public" {
		t.Fatalf("GiST prerequisite: %s %v", schema, err)
	}
}

func TestOnlineIndexGiSTRequiresPlainColumns(t *testing.T) {
	plan := production.OnlineIndexPlan{Method: "gist", Schema: "indexer", Table: "indexer_ingest_commits", Name: "by_height", Columns: []string{"previous_height", "last_complete_height"}}
	if _, err := plan.CreateSQL(); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []func(*production.OnlineIndexPlan){
		func(p *production.OnlineIndexPlan) { p.Method = "hash" },
		func(p *production.OnlineIndexPlan) { p.Unique = true },
		func(p *production.OnlineIndexPlan) {
			p.Columns = nil
			p.Keys = []production.OnlineIndexKey{{Column: "previous_height", Numeric: true}}
		},
		func(p *production.OnlineIndexPlan) {
			p.Columns = nil
			p.Keys = []production.OnlineIndexKey{{Column: "previous_height", Descending: true}}
		},
		func(p *production.OnlineIndexPlan) {
			position := 0
			p.Columns = nil
			p.Keys = []production.OnlineIndexKey{{Column: "topics", JSONTextPosition: &position}}
		},
	} {
		candidate := plan
		mutation(&candidate)
		if _, err := candidate.CreateSQL(); err == nil {
			t.Fatal("unsupported online index definition admitted")
		}
	}
}
