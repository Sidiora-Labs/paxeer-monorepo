package store

import (
	"bytes"
	"slices"
	"testing"
)

const (
	spot_leaderboard  = "spot_leaderboard"
	spot_account_legs = "spot_account_legs"
)

func TestSpotLeaderboardTables(t *testing.T) {
	db := testDB(t)
	migrate(t, db)

	t.Run("keys", func(t *testing.T) {
		if got := primaryKey(t, db, spot_leaderboard); got != "kind,rank" {
			t.Fatalf("spot_leaderboard primary key %s, want kind,rank", got)
		}
		if got := uniqueKeys(t, db, spot_leaderboard); !slices.Equal(got, []string{"kind,account"}) {
			t.Fatalf("spot_leaderboard unique keys %v, want [kind,account]", got)
		}
		if got := primaryKey(t, db, spot_account_legs); got != "account" {
			t.Fatalf("spot_account_legs primary key %s, want account", got)
		}
	})

	t.Run("columns", func(t *testing.T) {
		for table, want := range map[string][]column{
			spot_leaderboard: {
				{"kind", "text"}, {"rank", "integer"}, {"account", "bytea"},
				{"value", "numeric"}, {"updated_block", "bigint"},
			},
			spot_account_legs: {
				{"account", "bytea"}, {"realized_usd", "numeric"}, {"volume_usd", "numeric"},
				{"trades", "bigint"}, {"updated_block", "bigint"},
			},
		} {
			if got := columns(t, db, table); !slices.Equal(got, want) {
				t.Fatalf("%s columns %v, want %v", table, got, want)
			}
			if got := queryStrings(t, db, `SELECT column_name FROM information_schema.columns WHERE table_schema = 'perps' AND table_name = $1 AND is_nullable = 'YES' ORDER BY ordinal_position`, table); len(got) != 0 {
				t.Fatalf("%s nullable columns %v, want none", table, got)
			}
		}
	})

	t.Run("leaderboard_checks", func(t *testing.T) {
		insert := `INSERT INTO perps.spot_leaderboard (kind, rank, account, value, updated_block) VALUES ($1, $2, $3, -12.5, 7)`
		account := bytes.Repeat([]byte{0x11}, 20)
		for _, kind := range []string{"pnl", "volume", "win-rate"} {
			if err := execRolledBack(t, db, insert, kind, 1, account); err != nil {
				t.Fatalf("spot_leaderboard kind %s refused: %v", kind, err)
			}
		}
		wantCheckViolation(t, execRolledBack(t, db, insert, "unknown", 1, account), "spot_leaderboard_kind_check")
		for _, rank := range []int{0, -1} {
			wantCheckViolation(t, execRolledBack(t, db, insert, "pnl", rank, account), "spot_leaderboard_rank_check")
		}
		for _, size := range []int{19, 21} {
			wantCheckViolation(t, execRolledBack(t, db, insert, "pnl", 1, bytes.Repeat([]byte{0x11}, size)), "spot_leaderboard_account_check")
		}
	})
}
