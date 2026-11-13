package httpapi

import (
	"context"
	"strings"
	"testing"
)

func TestPostgresHistoryRevisionRejectsUntrustedCandleRows(t *testing.T) {
	ctx := context.Background()
	pool := historyPostgres(t)
	if _, err := pool.Exec(ctx, `CREATE SCHEMA revision_authority;
CREATE TABLE revision_authority.chart_exact_v1(chain_id text,pool text,interval text,start bigint,snapshot jsonb)`); err != nil {
		t.Fatal(err)
	}
	reader := PostgresHistoryRevision{Pool: pool, Schema: "revision_authority"}
	request := HistoryRequest{Symbol: udfPool, Timeframe: "1m", From: 0, To: 120}
	baseline, err := reader.Read(ctx, request)
	if err != nil || len(baseline) != 64 {
		t.Fatalf("empty revision authority: %q, %v", baseline, err)
	}
	for _, tc := range []struct {
		name, rows, want string
	}{
		{"zero revision", `(60,'{"revision":"0"}')`, "invalid revision source"},
		{"noninteger revision", `(60,'{"revision":"abc"}')`, "invalid revision source"},
		{"missing revision", `(60,'{}')`, "cannot scan NULL"},
		{"duplicate candle", `(60,'{"revision":"1"}'),(60,'{"revision":"2"}')`, "invalid revision candle identity"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := pool.Exec(ctx, `INSERT INTO revision_authority.chart_exact_v1(chain_id,pool,interval,start,snapshot) VALUES `+strings.ReplaceAll(tc.rows, "(", "('125','"+udfPool+"','1m',")); err != nil {
				t.Fatal(err)
			}
			_, err := reader.Read(ctx, request)
			if _, err := pool.Exec(ctx, `TRUNCATE revision_authority.chart_exact_v1`); err != nil {
				t.Fatal(err)
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("untrusted revision row: %v", err)
			}
		})
	}
	large := request
	large.To = 300120
	if _, err := pool.Exec(ctx, `INSERT INTO revision_authority.chart_exact_v1 SELECT '125',$1,'1m',i*60,jsonb_build_object('revision','1') FROM generate_series(0,5000) AS i`, udfPool); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Read(ctx, large); err == nil || !strings.Contains(err.Error(), "history revision range exceeds bound") {
		t.Fatalf("unbounded revision history: %v", err)
	}
}
