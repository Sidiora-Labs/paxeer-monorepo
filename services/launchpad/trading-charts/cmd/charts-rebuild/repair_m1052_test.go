package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	dbproduction "github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/jackc/pgx/v5/pgxpool"
)

func repair1052Bind(t *testing.T, name string, value any, inline bool) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(name+"_SHA256", dbproduction.Checksum(raw))
	t.Setenv(name+"_FILE", "")
	t.Setenv(name+"_DOCUMENT", "")
	if inline {
		t.Setenv(name+"_DOCUMENT", base64.StdEncoding.EncodeToString(raw))
	} else {
		path := filepath.Join(t.TempDir(), name+".json")
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		t.Setenv(name+"_FILE", path)
	}
}

func repair1052Expected() dbproduction.ExpectedSchema {
	return dbproduction.ExpectedSchema{
		Service: "core/trading-charts", RuntimeRole: "candles_runtime",
		RegistryDigest: strings.Repeat("a", 64), InterfaceDigest: strings.Repeat("b", 64),
		ReaderVersion: 1, Migrations: map[int64]string{1: strings.Repeat("c", 64)},
		Schemas: map[string]string{"candles": strings.Repeat("d", 64)}, Epoch: 1, Region: "local",
		Privileges: []dbproduction.Grant{{Schema: "candles", Object: "chart_exact", Privileges: []string{"SELECT", "INSERT", "UPDATE"}}},
	}
}

func repair1052Environment(t *testing.T, inline bool) {
	t.Helper()
	var clear func(reflect.Type)
	clear = func(typ reflect.Type) {
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.Anonymous {
				clear(field.Type)
			}
			if name := strings.Split(field.Tag.Get("env"), ",")[0]; name != "" {
				t.Setenv(name, "")
			}
		}
	}
	clear(reflect.TypeOf(config.Config{}))
	for key, value := range map[string]string{
		"NODE_ENV": "production", "RUNTIME_PROFILE": "container", "PORT": "8080", "BIND_ADDRESS": "0.0.0.0",
		"DATABASE_URL": "postgres://candles_runtime:fixture@127.0.0.1:1/charts?sslmode=disable",
		"REDIS_URL":    "redis://127.0.0.1:1", "REDIS_BULL_URL": "redis://127.0.0.1:1/1", "RPC_URL": "http://127.0.0.1:1",
		"WEBHOOK_HMAC_SECRET": strings.Repeat("s", 32), "NATS_CREDENTIALS": "unused-by-rebuild.creds",
	} {
		t.Setenv(key, value)
	}
	roles := deployment.Current().Roles
	for key, role := range map[string]string{
		"EVENT_EMITTER_ADDRESS": "EventEmitter", "POOL_REGISTRY_ADDRESS": "PoolRegistry", "ROUTER_ADDRESS": "Router",
		"FACTORY_ADDRESS": "SidioraFactory", "QUOTER_ADDRESS": "Quoter", "PROTOCOL_CONFIG_ADDRESS": "ProtocolConfig",
		"FEE_ACCUMULATOR_ADDRESS": "FeeAccumulator", "SIDIORA_NFT_ADDRESS": "SidioraNFT", "FEES_ROUTER_ADDRESS": "FeesRouter", "POOL_BEACON_ADDRESS": "PoolBeacon",
	} {
		t.Setenv(key, roles[role].Address)
	}
	repair1052Bind(t, "DELIVERY_CONFIG", map[string]any{"schema": "candles", "sourceStream": "canonical_fills"}, inline)
	repair1052Bind(t, "EXPECTED_SCHEMA", repair1052Expected(), inline)
}

func repair1052Args(manifest string) []string {
	return []string{"--manifests", manifest, "--pool", "0x" + strings.Repeat("1", 40), "--run-id", "repair-m1052", "--from", "0", "--to", "60", "--promote"}
}

func TestRepairM_1052_DefaultTargetsProductionTables(t *testing.T) {
	for _, inline := range []bool{false, true} {
		name := "file"
		if inline {
			name = "inline"
		}
		t.Run(name, func(t *testing.T) {
			repair1052Environment(t, inline)
			o, err := parseRebuildOptions(nil)
			if err != nil || o.schema != "candles" {
				t.Fatalf("default schema = %q, error = %v", o.schema, err)
			}
			cfg, err := loadRebuildConfig(o.schema)
			if err != nil {
				t.Fatal(err)
			}
			if history, err := dbproduction.HistoryTable("core/trading-charts", o.schema); err != nil || history != "migration_control.trading_charts_versions" {
				t.Fatalf("schema does not resolve the production migration binding: %q, %v", history, err)
			}
			pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			exact, err := store.NewExact(pool, o.schema)
			if err != nil {
				t.Fatal(err)
			}
			if exact.Schema != "candles" || exact.Pool != pool {
				t.Fatal("rebuild store lost the admitted database/schema binding")
			}
			for _, ddl := range []struct {
				build  func(string) (string, error)
				tables []string
			}{
				{store.ExactSchemaDDL, []string{"candles.chart_exact", "candles.chart_inbox", "candles.chart_outbox"}},
				{store.RebuildSchemaDDL, []string{"candles.chart_rebuild_runs", "candles.chart_rebuild_candles"}},
			} {
				sql, err := ddl.build(exact.Schema)
				if err != nil {
					t.Fatal(err)
				}
				for _, table := range ddl.tables {
					if !strings.Contains(sql, "CREATE TABLE "+table+"(") {
						t.Fatalf("store does not target production table %s", table)
					}
				}
			}
		})
	}
}

func TestRepairM_1052_RejectMismatchedBindingsBeforeManifest(t *testing.T) {
	for _, tc := range []struct {
		name   string
		args   []string
		change func(*testing.T)
		want   string
	}{
		{name: "legacy_override", args: []string{"--schema", "charts"}, want: "admitted candles schema"},
		{name: "other_override", args: []string{"--schema", "indexer"}, want: "admitted candles schema"},
		{name: "unsafe_override", args: []string{"--schema", "candles;DROP SCHEMA candles"}, want: "admitted candles schema"},
		{name: "missing_profile", change: func(t *testing.T) { t.Setenv("RUNTIME_PROFILE", "") }, want: "runtime profile"},
		{name: "unknown_profile", change: func(t *testing.T) { t.Setenv("RUNTIME_PROFILE", "unreviewed") }, want: "runtime profile"},
		{name: "container_address", change: func(t *testing.T) { t.Setenv("BIND_ADDRESS", "127.0.0.1") }, want: "container_bind_mismatch"},
		{name: "container_port", change: func(t *testing.T) { t.Setenv("PORT", "3000") }, want: "container_bind_mismatch"},
		{name: "owned_address", change: func(t *testing.T) { t.Setenv("RUNTIME_PROFILE", "owned_local") }, want: "owned_loopback_required"},
		{name: "nonproduction", change: func(t *testing.T) { t.Setenv("NODE_ENV", "development") }, want: "production runtime profile"},
		{name: "delivery_schema", change: func(t *testing.T) { repair1052Bind(t, "DELIVERY_CONFIG", map[string]any{"schema": "charts"}, true) }, want: "delivery schema binding mismatch"},
		{name: "missing_delivery_schema", change: func(t *testing.T) { repair1052Bind(t, "DELIVERY_CONFIG", map[string]any{}, true) }, want: "delivery schema binding mismatch"},
		{name: "wrong_delivery_type", change: func(t *testing.T) { repair1052Bind(t, "DELIVERY_CONFIG", map[string]any{"schema": 125}, true) }, want: "delivery schema binding mismatch"},
		{name: "delivery_digest", change: func(t *testing.T) { t.Setenv("DELIVERY_CONFIG_SHA256", strings.Repeat("0", 64)) }, want: "binding_mismatch"},
		{name: "expected_digest", change: func(t *testing.T) { t.Setenv("EXPECTED_SCHEMA_SHA256", strings.Repeat("0", 64)) }, want: "binding_mismatch"},
		{name: "wrong_service", change: func(t *testing.T) {
			expected := repair1052Expected()
			expected.Service = "core/stats-workers"
			repair1052Bind(t, "EXPECTED_SCHEMA", expected, true)
		}, want: "runtime schema binding mismatch"},
		{name: "wrong_schema", change: func(t *testing.T) {
			expected := repair1052Expected()
			expected.Schemas = map[string]string{"charts": strings.Repeat("d", 64)}
			repair1052Bind(t, "EXPECTED_SCHEMA", expected, true)
		}, want: "runtime schema binding mismatch"},
		{name: "missing_binding", change: func(t *testing.T) { t.Setenv("EXPECTED_SCHEMA_FILE", "") }, want: "binding_missing"},
		{name: "ambiguous_binding", change: func(t *testing.T) { t.Setenv("DELIVERY_CONFIG_DOCUMENT", "e30=") }, want: "both set"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repair1052Environment(t, false)
			if tc.change != nil {
				tc.change(t)
			}
			manifest := filepath.Join(t.TempDir(), "must-not-be-opened.json")
			err := runArgs(append(repair1052Args(manifest), tc.args...))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q before manifest access, got %v", tc.want, err)
			}
			var pathErr *os.PathError
			if errors.As(err, &pathErr) && pathErr.Path == manifest {
				t.Fatal("mismatched runtime binding reached manifest loading")
			}
		})
	}
}

func TestRepairM_1052_AdmittedProfilesReachManifest(t *testing.T) {
	for _, profile := range []string{"container", "owned_local"} {
		for _, explicit := range []bool{false, true} {
			name := profile + "/default"
			if explicit {
				name = profile + "/explicit"
			}
			t.Run(name, func(t *testing.T) {
				repair1052Environment(t, true)
				t.Setenv("RUNTIME_PROFILE", profile)
				if profile == "owned_local" {
					t.Setenv("BIND_ADDRESS", "127.0.0.1")
				}
				manifest := filepath.Join(t.TempDir(), "approved-manifests.json")
				args := repair1052Args(manifest)
				if explicit {
					args = append(args, "--schema", "candles")
				}
				err := runArgs(args)
				var pathErr *os.PathError
				if !errors.As(err, &pathErr) || pathErr.Path != manifest || !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("admitted command did not reach manifest loading: %v", err)
				}
			})
		}
	}
}

func TestRepairM_1052_RuntimeReadinessPrecedesReconstruction(t *testing.T) {
	repair1052Environment(t, false)
	expected := repair1052Expected()
	expected.RuntimeRole = ""
	repair1052Bind(t, "EXPECTED_SCHEMA", expected, false)
	manifest := filepath.Join(t.TempDir(), "manifests.json")
	if err := os.WriteFile(manifest, []byte("[]"), 0600); err != nil {
		t.Fatal(err)
	}
	err := runArgs(repair1052Args(manifest))
	if err == nil || !strings.Contains(err.Error(), "rebuild runtime schema readiness: complete trusted expected-schema requirements required") {
		t.Fatalf("unready schema was not rejected before database/RPC/archive reconstruction: %v", err)
	}
}
