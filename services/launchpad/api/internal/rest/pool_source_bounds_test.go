package rest_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"
)

func TestPoolReadsRejectInvalidSourceAndContinuation(t *testing.T) {
	f := newPoolReadsFixture(t, "")
	ctx := context.Background()
	for _, tc := range []struct {
		query  string
		status int
		code   string
	}{
		{"?extra=1", 400, "QUERY_INVALID"},
	} {
		assertV2Error(t, getV2(t, f.srv.URL+"/v2/pools/"+f.first+tc.query), tc.status, tc.code)
	}
	first := assertV2Success(t, getV2(t, f.srv.URL+"/v2/pools?sort=new&limit=1"), "PoolListResponse")
	_ = first
	for _, offset := range []int{1000} {
		// The snapshot identity is obtained from the published ranking response.
		ranking := assertV2Success(t, getV2(t, f.srv.URL+"/v2/rankings/new"), "RankingsResponse")
		snapshot := ranking["data"].(map[string]any)["snapshot"].(map[string]any)["snapshotId"].(string)
		cursor := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"snapshotId":%q,"offset":%d}`, snapshot, offset)))
		assertV2Error(t, getV2(t, f.srv.URL+"/v2/pools?sort=new&cursor="+cursor), 400, "CURSOR_INVALID")
	}
	for _, tc := range []struct{ name, change, restore, code string }{
		{"unqualified fee", `UPDATE stats.accounting_market_commits SET fill=jsonb_set(fill,'{execution,quality}','"unavailable"')`, ``, "DEPENDENCY_UNAVAILABLE"},
		{"fee", `UPDATE stats.accounting_market_commits SET fill=jsonb_set(fill,'{execution,feeBPS}','"invalid"')`, ``, "INTEGRITY_QUARANTINED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Preserve each committed document while validating source conversion.
			rows, err := f.pool.Query(ctx, `SELECT publication_id,fill FROM stats.accounting_market_commits`)
			if err != nil {
				t.Fatal(err)
			}
			saved := map[string][]byte{}
			for rows.Next() {
				var sequence string
				var fill []byte
				if err := rows.Scan(&sequence, &fill); err != nil {
					t.Fatal(err)
				}
				saved[sequence] = fill
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			if _, err := f.pool.Exec(ctx, tc.change); err != nil {
				t.Fatal(err)
			}
			assertV2Error(t, getV2(t, f.srv.URL+"/v2/pools/"+f.first), 503, tc.code)
			for sequence, fill := range saved {
				if _, err := f.pool.Exec(ctx, `UPDATE stats.accounting_market_commits SET fill=$1 WHERE publication_id=$2`, fill, sequence); err != nil {
					t.Fatal(err)
				}
			}
		})
	}

	if _, err := f.pool.Exec(ctx, `DELETE FROM indexer.pool_state_snapshots`); err != nil {
		t.Fatal(err)
	}
	assertV2Error(t, getV2(t, f.srv.URL+"/v2/pools/"+f.first), 503, "DEPENDENCY_UNAVAILABLE")
	if _, err := f.pool.Exec(ctx, `DROP TABLE indexer.pool_state_snapshots`); err != nil {
		t.Fatal(err)
	}
	assertV2Error(t, getV2(t, f.srv.URL+"/v2/pools/"+f.first), 503, "DEPENDENCY_UNAVAILABLE")
	assertV2Error(t, getV2(t, f.srv.URL+"/v2/pools?sort=new"), 503, "DEPENDENCY_UNAVAILABLE")
}
