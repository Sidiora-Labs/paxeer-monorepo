package production

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db"
)

func TestRuntimeGrantsRequireExactObjectAuthority(t *testing.T) {
	for name, grant := range map[string]Grant{
		"identifier":          {Schema: "bad schema", Object: "entry", Privileges: []string{"SELECT"}},
		"object kind":         {Schema: "l07_stats", Object: "entry", ObjectKind: "index", Privileges: []string{"SELECT"}},
		"duplicate privilege": {Schema: "l07_stats", Object: "entry", Privileges: []string{"SELECT", "SELECT"}},
		"function privilege":  {Schema: "l07_stats", Object: "lookup", ObjectKind: "function", Privileges: []string{"SELECT"}},
		"function columns":    {Schema: "l07_stats", Object: "lookup", ObjectKind: "function", Privileges: []string{"EXECUTE"}, Columns: []string{"id"}},
		"function argument":   {Schema: "l07_stats", Object: "lookup", ObjectKind: "function", Privileges: []string{"EXECUTE"}, ArgumentTypes: []string{"unsafe"}},
		"sequence privilege":  {Schema: "l07_stats", Object: "counter", ObjectKind: "sequence", Privileges: []string{"SELECT"}},
		"table signature":     {Schema: "l07_stats", Object: "entry", Privileges: []string{"SELECT"}, ArgumentTypes: []string{"text"}},
		"duplicate column":    {Schema: "l07_stats", Object: "entry", Privileges: []string{"SELECT"}, Columns: []string{"id", "id"}},
		"column insert":       {Schema: "l07_stats", Object: "entry", Privileges: []string{"INSERT"}, Columns: []string{"id"}},
		"history":             {Schema: "migration_control", Object: "applied_migrations", Privileges: []string{"SELECT"}},
	} {
		t.Run(name, func(t *testing.T) {
			m := manifest()
			m.Grants = []Grant{grant}
			if err := m.Validate(); err == nil {
				t.Fatal("unreviewed object authority admitted")
			}
		})
	}
	m := manifest()
	m.Grants = append(m.Grants, m.Grants[0])
	if err := m.Validate(); err == nil {
		t.Fatal("duplicate grant admitted")
	}
	m = manifest()
	m.OwnerReadGrants = []Grant{{Schema: m.Schema, Object: "entries_v1", View: true, ObjectKind: "view", Columns: []string{"id"}, Privileges: []string{"SELECT"}}}
	if err := m.Validate(); err == nil {
		t.Fatal("local owner read interface admitted")
	}
	m.OwnerReadGrants[0].Schema = "foreign"
	m.OwnerReadGrants[0].Columns = []string{"bad column"}
	if err := m.Validate(); err == nil {
		t.Fatal("invalid owner read column admitted")
	}
	m = manifest()
	m.OwnerReadGrants = []Grant{m.Grants[1]}
	m.OwnerReadGrants[0].ObjectKind = "view"
	if _, err := m.GrantSQL(); err != nil {
		t.Fatal(err)
	}
	m.Schema = "invalid schema"
	if _, err := m.ProvisionSQL(); err == nil {
		t.Fatal("invalid provisioning identity admitted")
	}
	m = RoleManifest{Service: "core/stats-workers", Schema: "stats", OwnerRole: "stats_owner", RuntimeRole: "stats_runtime", SchemaCapabilities: []SchemaCapability{{Service: "core/stats-workers/holdings", Schema: "stats_transfer_holdings", OwnerRole: "stats_transfer_holdings_owner"}}}
	if sql, err := m.ProvisionSQL(); err != nil || len(sql) == 0 {
		t.Fatalf("reviewed schema capability: %v", err)
	}
	m.SchemaCapabilities[0].Service = "unknown"
	if _, err := m.ProvisionSQL(); err == nil {
		t.Fatal("unreviewed schema capability admitted")
	}
	if err := ValidateFleet(0, 0, nil); err == nil {
		t.Fatal("missing fleet capacity admitted")
	}
}

func TestExpectedSchemaPrivilegeMatrix(t *testing.T) {
	digest := Checksum([]byte("catalog"))
	base := ExpectedSchema{Service: "core/stats-workers", RuntimeRole: "stats_runtime", RegistryDigest: digest, InterfaceDigest: digest, ReaderVersion: 1, Migrations: map[int64]string{70001: digest}, Schemas: map[string]string{"stats": digest}, Epoch: 1, Region: "us-east", Privileges: []Grant{{Schema: "stats", Object: "entries", Privileges: []string{"SELECT"}}}}
	if err := validateExpectedSchema(base); err != nil {
		t.Fatal(err)
	}
	for name, edit := range map[string]func(*ExpectedSchema){
		"migration":          func(w *ExpectedSchema) { w.Migrations = map[int64]string{0: digest} },
		"schema":             func(w *ExpectedSchema) { w.Schemas = map[string]string{"public": digest} },
		"grant":              func(w *ExpectedSchema) { w.Privileges[0].GrantOption = true },
		"kind":               func(w *ExpectedSchema) { w.Privileges[0].ObjectKind = "index" },
		"history":            func(w *ExpectedSchema) { w.Privileges[0].Schema = "migration_control" },
		"duplicate":          func(w *ExpectedSchema) { w.Privileges = append(w.Privileges, w.Privileges[0]) },
		"function privilege": func(w *ExpectedSchema) { w.Privileges[0].ObjectKind = "function" },
		"sequence privilege": func(w *ExpectedSchema) { w.Privileges[0].ObjectKind = "sequence" },
		"table privilege":    func(w *ExpectedSchema) { w.Privileges[0].Privileges = []string{"TRUNCATE"} },
		"column":             func(w *ExpectedSchema) { w.Privileges[0].Columns = []string{"bad column"} },
		"column privilege": func(w *ExpectedSchema) {
			w.Privileges[0].Columns = []string{"id"}
			w.Privileges[0].Privileges = []string{"INSERT"}
		},
		"table arguments": func(w *ExpectedSchema) { w.Privileges[0].ArgumentTypes = []string{"text"} },
		"function argument": func(w *ExpectedSchema) {
			w.Privileges[0].ObjectKind = "function"
			w.Privileges[0].Privileges = []string{"EXECUTE"}
			w.Privileges[0].ArgumentTypes = []string{"invalid"}
		},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := base
			candidate.Privileges = append([]Grant(nil), base.Privileges...)
			edit(&candidate)
			if err := validateExpectedSchema(candidate); err == nil {
				t.Fatal("invalid schema requirement admitted")
			}
		})
	}
	base.Privileges = []Grant{{Schema: "stats", Object: "lookup", ObjectKind: "function", Privileges: []string{"EXECUTE"}, ArgumentTypes: []string{"text", "bigint", "uuid", "bytea", "integer", "boolean", "jsonb"}}, {Schema: "stats", Object: "counter", ObjectKind: "sequence", Privileges: []string{"USAGE"}}}
	if err := validateExpectedSchema(base); err != nil {
		t.Fatal(err)
	}
}

func TestAllocationArithmeticAndSelectionBounds(t *testing.T) {
	for _, live := range []LiveProcesses{{Replicas: 0, Maximum: 1}, {Replicas: math.MaxInt64, Surge: 1, Maximum: math.MaxInt64}, {Replicas: 1, Terminating: math.MaxInt64, Maximum: math.MaxInt64}} {
		if err := live.Validate(); err == nil {
			t.Fatal("overflowing process envelope admitted")
		}
	}
	for _, wire := range [][]byte{nil, make([]byte, (1<<20)+1)} {
		if _, err := ParseConnectionBudget(wire); err == nil {
			t.Fatal("unbounded budget document admitted")
		}
	}
	for name, edit := range map[string]func(*ConnectionBudget){
		"identity": func(b *ConnectionBudget) { b.Identity = "" },
		"duplicate target": func(b *ConnectionBudget) {
			target := b.Targets[0]
			target.Member = "member_b"
			b.Targets = append(b.Targets, target)
		},
		"headroom":         func(b *ConnectionBudget) { b.Targets[0].ProtectedSlots = 600 },
		"duplicate pooler": func(b *ConnectionBudget) { b.Poolers = append(b.Poolers, b.Poolers[0]) },
		"pooler target":    func(b *ConnectionBudget) { b.Poolers[0].Target = "missing" },
		"pooler route":     func(b *ConnectionBudget) { b.Clients[0].Pooler = "missing" },
		"duplicate user":   func(b *ConnectionBudget) { b.Poolers[0].Users = append(b.Poolers[0].Users, b.Poolers[0].Users[0]) },
		"unused pair":      func(b *ConnectionBudget) { b.Poolers[0].Pairs = nil },
		"alias overflow": func(b *ConnectionBudget) {
			b.Poolers[0].Aliases = []AliasLimit{{Alias: "a", MaxConnections: math.MaxInt64}, {Alias: "b", MaxConnections: 1}}
		},
		"user overflow": func(b *ConnectionBudget) {
			b.Poolers[0].Users = []UserLimit{{User: "a", MaxConnections: math.MaxInt64}, {User: "b", MaxConnections: 1}}
		},
		"reserve overflow": func(b *ConnectionBudget) {
			b.Poolers[0].Pairs[0].PoolSize = math.MaxInt64
			b.Poolers[0].Pairs[0].ReservePoolSize = 1
		},
		"pair overflow":    func(b *ConnectionBudget) { b.Poolers[0].Pairs[0].PoolSize = math.MaxInt64 },
		"backend overflow": func(b *ConnectionBudget) { b.Operational[0].Connections = math.MaxInt64 },
	} {
		t.Run(name, func(t *testing.T) {
			b := allocationFixture()
			edit(&b)
			if err := b.Validate(); err == nil {
				t.Fatal("invalid allocation admitted")
			}
		})
	}
	b := allocationFixture()
	b.Targets[0].MaxConnections = 601
	if _, err := b.Reconcile(); err != nil {
		t.Fatal(err)
	}
	if _, err := (ConnectionBudget{}).BudgetPoolOptions("role", "pool", "instance"); err == nil {
		t.Fatal("unregistered pool admitted")
	}
	if _, err := (ConnectionBudget{}).MigrationPoolOptions("id", "alias", "user", "instance"); err == nil {
		t.Fatal("unregistered migration pool admitted")
	}
	if _, err := b.MigrationPoolOptions("unknown", "alias", "user", "instance"); err == nil {
		t.Fatal("unknown migration admitted")
	}
	for _, tc := range []struct{ interactive, batch RuntimePoolSelection }{
		{RuntimePoolSelection{PoolID: "same"}, RuntimePoolSelection{PoolID: "same"}},
		{RuntimePoolSelection{PoolID: "missing"}, RuntimePoolSelection{PoolID: "batch"}},
		{RuntimePoolSelection{PoolID: "interactive"}, RuntimePoolSelection{PoolID: "missing"}},
	} {
		if _, err := b.OpenRuntimePools(context.Background(), "apid:rest", "instance", tc.interactive, tc.batch); err == nil {
			t.Fatal("unregistered runtime selection admitted")
		}
	}
	extra := b.Clients[0]
	extra.PoolID = "second"
	b.Clients = append(b.Clients, extra)
	if _, err := b.OpenRuntimePools(context.Background(), "apid:rest", "instance", RuntimePoolSelection{PoolID: "interactive"}, RuntimePoolSelection{PoolID: "second"}); err == nil {
		t.Fatal("interactive allocation admitted as batch")
	}
	b.Clients[len(b.Clients)-1].QueryClass = db.Batch
	for _, wire := range []string{`{`, `{} {}`, `{"version":2}`, `{"version":1,"approvals":[{}]}`} {
		data := []byte(wire)
		if _, err := LoadIndexRepairRegistry(data, Checksum(data)); err == nil {
			t.Fatal("invalid index repair authority admitted")
		}
	}
	if _, err := LoadIndexRepairRegistry([]byte(`{}`), strings.Repeat("0", 64)); err == nil {
		t.Fatal("unbound repair registry admitted")
	}
	duplicate := IndexRepairRegistry{Version: 1, Approvals: []IndexRepairApproval{{MigrationID: 70001, Schema: "stats", Index: "entry_idx", Operation: "drop_recreate", FailedChecksum: Checksum([]byte("sql")), FailureClass: "interrupted"}}}
	duplicate.Approvals = append(duplicate.Approvals, duplicate.Approvals[0])
	data, err := json.Marshal(duplicate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LoadIndexRepairRegistry(data, Checksum(data)); err == nil {
		t.Fatal("duplicate repair authority admitted")
	}
	if _, err := b.BudgetPoolOptions("apid:rest", "interactive", ""); err == nil {
		t.Fatal("unidentified runtime instance admitted")
	}
	if _, err := b.OpenRuntimePools(context.Background(), "apid:rest", "instance", RuntimePoolSelection{PoolID: "interactive", DSN: "invalid"}, RuntimePoolSelection{PoolID: "second"}); err == nil {
		t.Fatal("invalid connection selection admitted")
	}

}
