package store_test

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
	sharedconfig "github.com/Sidiora-Technologies/KindleLaunch/shared/config"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const holderIndexDefinition = "CREATE INDEX idx_ph_balance_numeric ON stats.pool_holders USING btree (pool_address, ((balance)::numeric) DESC, holder_address)"

func TestRuntimeRoleHolderIndex(t *testing.T) {
	ctx := context.Background()
	database := coordinatortest.Bootstrap(t, os.Getenv("COORDINATOR_TEST_POSTGRES_DSN"), "stats_runtime")
	runtime := database.Runtimes["stats_runtime"]
	pool, err := pgxpool.New(ctx, runtime.DSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	owner, err := pgx.Connect(ctx, database.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := owner.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	if _, err = owner.Exec(ctx, "SET ROLE "+pgx.Identifier{database.Rename("stats_owner")}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	want, err := production.LoadExpectedSchema(runtime.ExpectedSchema, runtime.ExpectedSchemaSHA256)
	if err != nil {
		t.Fatal(err)
	}
	if err = production.CheckReadiness(ctx, pool, want); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{BaseEnv: sharedconfig.BaseEnv{NodeEnv: "production"}, ExpectedSchemaDocument: base64.StdEncoding.EncodeToString(runtime.ExpectedSchema), ExpectedSchemaSHA256: runtime.ExpectedSchemaSHA256}
	if err := cfg.CheckRuntimeSchema(ctx, pool); err != nil {
		t.Fatalf("production startup readiness: %v", err)
	}
	var definition string
	var valid, ready, live bool
	if err = pool.QueryRow(ctx, `SELECT pg_get_indexdef(i.indexrelid),i.indisvalid,i.indisready,i.indislive FROM pg_index i WHERE i.indexrelid='stats.idx_ph_balance_numeric'::regclass`).Scan(&definition, &valid, &ready, &live); err != nil || definition != holderIndexDefinition || !valid || !ready || !live {
		t.Fatalf("registered holder index: definition=%q valid=%t ready=%t live=%t: %v", definition, valid, ready, live, err)
	}
	if database.Applied[150007] == "" || want.Migrations[150007] != database.Applied[150007] {
		t.Fatal("numeric holder index migration is absent from runtime readiness")
	}
	for _, holder := range []struct{ address, balance string }{{"0xc", "9"}, {"0xb", "100"}, {"0xa", "100"}, {"0xd", "100000000000000000000000000000000000001"}} {
		insertHolder(t, ctx, pool, "0xpool", holder.address, holder.balance, "0")
	}
	insertHolder(t, ctx, pool, "0xother", "0xa", "999999999999999999999999999999999999999", "0")
	st := store.New(pool)
	read, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	defer read.Rollback(ctx)
	if err = st.WithTx(read).CheckHolderBalanceIndex(ctx); err != nil {
		t.Fatalf("runtime startup check: %v", err)
	}
	holders, err := st.WithTx(read).ListHolders(ctx, "0xpool", 4, 0)
	if err != nil || len(holders) != 4 {
		t.Fatalf("runtime numeric holder read: %+v, %v", holders, err)
	}
	for i, address := range []string{"0xd", "0xa", "0xb", "0xc"} {
		if holders[i].HolderAddress != address {
			t.Fatalf("numeric balance and address order: %+v", holders)
		}
	}
	if err = read.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = production.CheckReadiness(ctx, pool, want); err != nil {
		t.Fatalf("startup check changed the catalog: %v", err)
	}
	_, err = pool.Exec(ctx, "CREATE INDEX runtime_holder_index ON stats.pool_holders (pool_address)")
	var denied *pgconn.PgError
	if !errors.As(err, &denied) || denied.Code != "42501" {
		t.Fatalf("runtime CREATE INDEX must be denied with SQLSTATE 42501: %v", err)
	}
	create := `CREATE INDEX idx_ph_balance_numeric ON stats.pool_holders (pool_address, (balance::numeric) DESC, holder_address ASC)`
	for _, tc := range []struct{ name, statement string }{
		{"missing", ""},
		{"text balance", `CREATE INDEX idx_ph_balance_numeric ON stats.pool_holders (pool_address, balance DESC, holder_address)`},
		{"ascending balance", `CREATE INDEX idx_ph_balance_numeric ON stats.pool_holders (pool_address, (balance::numeric), holder_address)`},
		{"wrong tie order", `CREATE INDEX idx_ph_balance_numeric ON stats.pool_holders (pool_address, (balance::numeric) DESC, holder_address DESC)`},
		{"partial", create + " WHERE balance <> '0'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := owner.Exec(ctx, "DROP INDEX stats.idx_ph_balance_numeric"); err != nil {
				t.Fatal(err)
			}
			if tc.statement != "" {
				if _, err := owner.Exec(ctx, tc.statement); err != nil {
					t.Fatal(err)
				}
			}
			if err := st.CheckHolderBalanceIndex(ctx); err == nil || !strings.Contains(err.Error(), "idx_ph_balance_numeric") {
				t.Fatalf("startup must name the missing or mismatched index: %v", err)
			}
			if err := cfg.CheckRuntimeSchema(ctx, pool); err == nil || !strings.Contains(err.Error(), "idx_ph_balance_numeric") {
				t.Fatalf("production startup must name the missing or mismatched index: %v", err)
			}
			if tc.statement != "" {
				if _, err := owner.Exec(ctx, "DROP INDEX stats.idx_ph_balance_numeric"); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := owner.Exec(ctx, create); err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Run("invalid numeric index", func(t *testing.T) {
		if _, err := owner.Exec(ctx, `DROP INDEX stats.idx_ph_balance_numeric; INSERT INTO stats.pool_holders VALUES('0xinvalid','0xinvalid','invalid','0',1)`); err != nil {
			t.Fatal(err)
		}
		_, err := owner.Exec(ctx, strings.Replace(create, "CREATE INDEX", "CREATE INDEX CONCURRENTLY", 1))
		var invalid *pgconn.PgError
		if !errors.As(err, &invalid) || invalid.Code != "22P02" {
			t.Fatalf("invalid numeric balance must prevent index validation: %v", err)
		}
		if err := pool.QueryRow(ctx, `SELECT pg_get_indexdef(i.indexrelid),i.indisvalid FROM pg_index i WHERE i.indexrelid='stats.idx_ph_balance_numeric'::regclass`).Scan(&definition, &valid); err != nil || definition != holderIndexDefinition || valid {
			t.Fatalf("invalid index catalog: definition=%q valid=%t: %v", definition, valid, err)
		}
		if err := st.CheckHolderBalanceIndex(ctx); err == nil || !strings.Contains(err.Error(), "idx_ph_balance_numeric") {
			t.Fatalf("startup must name the invalid index: %v", err)
		}
	})
}
