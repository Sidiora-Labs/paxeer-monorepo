package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestPauseRefreshRetainsSnapshotWhenDurableEvidenceIsIncomplete(t *testing.T) {
	for _, name := range []string{"creation query", "event query", "invalid pool", "missing block", "base hash", "base height", "event hash", "event topic", "pause arity", "pause data"} {
		t.Run(name, func(t *testing.T) {
			c := newPauseChain(t)
			address, contract, created := c.create(t, "Pause integrity")
			c.retain(t, created)
			c.coverRaw(address)
			c.refresh(t)
			previous := c.observation.snapshot.Load()
			paused := c.transact(t, contract, "pause")
			c.retain(t, paused)
			ctx := c.ctx
			pool := c.observation.pool
			exec := func(sql string, args ...any) {
				t.Helper()
				if _, err := pool.Exec(ctx, sql, args...); err != nil {
					t.Fatal(err)
				}
			}
			switch name {
			case "creation query":
				exec(`ALTER TABLE indexer.indexer_canonical_creations RENAME TO unavailable_creations`)
			case "event query":
				exec(`ALTER TABLE indexer.indexer_raw_logs RENAME TO unavailable_events`)
			case "invalid pool":
				exec(`UPDATE indexer.indexer_canonical_creations SET pool=$1`, "0x"+strings.Repeat("0", 40))
			case "missing block":
				exec(`DELETE FROM indexer.indexer_block_completeness WHERE height=$1`, previous.base.block.height)
			case "base hash":
				base := *previous.base
				base.block.hash = "0x" + strings.Repeat("a", 64)
				snapshot := *previous
				snapshot.base = &base
				previous = &snapshot
				c.observation.snapshot.Store(previous)
			case "base height":
				base := *previous.base
				base.block.height = -1
				snapshot := *previous
				snapshot.base = &base
				previous = &snapshot
				c.observation.snapshot.Store(previous)
			case "event hash":
				exec(`UPDATE indexer.indexer_raw_logs SET block_hash=$1 WHERE height=$2 AND topics->>0=$3`, "0x"+strings.Repeat("a", 64), paused.BlockNumber.Int64(), strings.ToLower(pauseTopic.Hex()))
			case "event topic":
				exec(`UPDATE indexer.indexer_raw_logs SET topics=jsonb_build_array($1::text,'invalid') WHERE height=$2 AND topics->>0=$1`, strings.ToLower(pauseTopic.Hex()), paused.BlockNumber.Int64())
			case "pause arity":
				exec(`UPDATE indexer.indexer_raw_logs SET topics=jsonb_build_array($1::text,$1::text) WHERE height=$2 AND topics->>0=$1`, strings.ToLower(pauseTopic.Hex()), paused.BlockNumber.Int64())
			case "pause data":
				exec(`UPDATE indexer.indexer_raw_logs SET data=$1 WHERE height=$2 AND topics->>0=$3`, []byte{1}, paused.BlockNumber.Int64(), strings.ToLower(pauseTopic.Hex()))
			}
			err := c.observation.refresh(ctx)
			if strings.HasSuffix(name, "query") {
				var databaseError *pgconn.PgError
				if !errors.As(err, &databaseError) || databaseError.Code != "42P01" {
					t.Fatalf("unavailable pause query: %v", err)
				}
			} else if !errors.Is(err, errPauseUnavailable) {
				t.Fatalf("inconsistent pause evidence: %v", err)
			}
			if c.observation.snapshot.Load() != previous {
				t.Fatal("failed refresh replaced retained snapshot")
			}
		})
	}
}
