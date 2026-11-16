package analytics_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/analytics"
	"github.com/jackc/pgx/v5"
)

func assertProjectionCatalog(t *testing.T, ctx context.Context, adminDSN string) {
	t.Helper()
	conn, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "CREATE SCHEMA analytics_reference"); err != nil {
		t.Fatal(err)
	}
	ddl, err := (analytics.ReactionAuthority{Schema: "analytics_reference"}).ProjectionSQL()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	const catalog = `SELECT jsonb_build_object(
 'columns',(SELECT jsonb_agg(jsonb_build_array(a.attname,format_type(a.atttypid,a.atttypmod),a.attnotnull,a.attidentity,pg_get_expr(d.adbin,d.adrelid)) ORDER BY a.attnum) FROM pg_attribute a LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum WHERE a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped),
 'keys',(SELECT jsonb_agg(pg_get_constraintdef(k.oid) ORDER BY pg_get_constraintdef(k.oid)) FROM pg_constraint k WHERE k.conrelid=c.oid),
 'indexes',(SELECT jsonb_agg(jsonb_build_array(i.indisunique,i.indisprimary,i.indisvalid,i.indisready,substring(pg_get_indexdef(i.indexrelid) FROM ' USING .*$')) ORDER BY substring(pg_get_indexdef(i.indexrelid) FROM ' USING .*$')) FROM pg_index i WHERE i.indrelid=c.oid)
 )::text FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=$1 AND c.relname=$2`
	for _, table := range []string{"analytics_progress", "analytics_trades", "analytics_outbox", "analytics_risk_inputs"} {
		var registered, reference string
		if err := tx.QueryRow(ctx, catalog, "stats", table).Scan(&registered); err != nil {
			t.Fatal(err)
		}
		if err := tx.QueryRow(ctx, catalog, "analytics_reference", table).Scan(&reference); err != nil {
			t.Fatal(err)
		}
		reference = strings.ReplaceAll(reference, "analytics_reference.", "stats.")
		if registered != reference {
			t.Fatalf("%s catalog differs from ProjectionSQL: registered=%s reference=%s", table, registered, reference)
		}
	}
}
