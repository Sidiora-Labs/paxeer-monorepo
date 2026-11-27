package store

import (
	"testing"
	"time"
)

const accountMarginReadSQL = `SELECT balance::text, locked::text FROM perps.account_margin WHERE account = $1 AND token = $2`
const positionMarginReadSQL = `SELECT collateral_usd::text, collateral_amount::text, status FROM perps.positions_current WHERE position_id = $1`

func TestAccountMargin(t *testing.T) {
	db := testDB(t)
	migrate(t, db)
	at := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)

	t.Run("fees_transfers_and_collateral_refresh_open_positions", func(t *testing.T) {
		c := newChain(t, db)
		c.next(100, at)
		c.deposited(traderA, "1000000000", "1000000000")
		c.opened("1", traderA, "0", "20000000000000000000", "60000", "1000000000")
		c.fee("1", traderA, 0, "9000000000000000", "9000")
		c.project()
		c.want(accountMarginReadSQL, []string{"999991000", "2000000"}, traderA.Bytes(), usdlBytes)
		c.want(positionMarginReadSQL, []string{"999991000000000000000", "999991000", "open"}, "1")

		c.next(101, at.Add(time.Second))
		c.deposited(traderA, "25000000", "1024991000")
		c.project()
		c.want(positionMarginReadSQL, []string{"1024991000000000000000", "1024991000", "open"}, "1")

		c.next(102, at.Add(2*time.Second))
		c.withdrawn(traderA, "25000000", "999991000")
		c.fee("1", traderA, 4, "100000000000000", "100")
		c.transferred(traderA, traderB, "1000000")
		c.project()
		c.want(accountMarginReadSQL, []string{"998991100", "2000000"}, traderA.Bytes(), usdlBytes)
		c.want(accountMarginReadSQL, []string{"1000000", "0"}, traderB.Bytes(), usdlBytes)
		c.want(positionMarginReadSQL, []string{"998991100000000000000", "998991100", "open"}, "1")
	})

	t.Run("notional_shares_and_individual_margin_rounding", func(t *testing.T) {
		c := newChain(t, db)
		c.next(100, at)
		c.opened("1", traderA, "0", "30000000000000000001", "60000", "1000000001")
		c.opened("2", traderA, "9", "10000000000000000000", "1", "1000000001")
		c.project()
		c.want(accountMarginReadSQL, []string{"1000000001", "4000001"}, traderA.Bytes(), usdlBytes)
		c.want(positionMarginReadSQL, []string{"750000000750000000006", "750000000", "open"}, "1")
		c.want(positionMarginReadSQL, []string{"250000000249999999993", "250000000", "open"}, "2")
		c.next(101, at.Add(time.Second))
		c.closed("1", traderA, "30000000000000000001", "0", true)
		c.project()
		c.want(accountMarginReadSQL, []string{"1000000001", "1000000"}, traderA.Bytes(), usdlBytes)
		c.want(positionMarginReadSQL, []string{"1000000001000000000000", "1000000001", "closed"}, "1")
		c.want(positionMarginReadSQL, []string{"1000000001000000000000", "1000000001", "open"}, "2")
	})

	t.Run("funding_snapshots_and_settled_pnl_round_toward_venue", func(t *testing.T) {
		c := newChain(t, db)
		c.next(100, at)
		c.opened("1", traderA, "0", "20000000000000000000", "60000", "1000000000")
		c.fee("1", traderA, 0, "9000000000000000", "9000")
		c.funding("1", traderA, "-750000000000000000", "999241000000000000000", "999241000")
		c.modified("1", traderA, "10000000000000000000", "1000240999000000000000", "1000240999")
		c.fee("1", traderA, 0, "1000000000000", "1")
		c.settled("1", traderA, "10000000000000000000", "1000000999999999999", "1000000000000", "-750000000000000000", "1000000")
		c.project()
		c.want(accountMarginReadSQL, []string{"1000240999", "1000000"}, traderA.Bytes(), usdlBytes)

		c.next(101, at.Add(time.Second))
		c.closed("1", traderA, "10000000000000000000", "-1000000000001", true)
		c.fee("1", traderA, 0, "1000000000000", "1")
		c.settled("1", traderA, "10000000000000000000", "-1000000000001", "1000000000000", "0", "0")
		c.project()
		c.want(accountMarginReadSQL, []string{"1000240996", "0"}, traderA.Bytes(), usdlBytes)
		c.want(positionMarginReadSQL, []string{"1000240999000000000000", "1000240999", "closed"}, "1")
	})

	t.Run("flip_snapshot_includes_close_pnl_before_open", func(t *testing.T) {
		c := newChain(t, db)
		c.next(100, at)
		c.opened("1", traderA, "0", "20000000000000000000", "60000", "1000000000")
		c.project()
		c.next(101, at.Add(time.Second))
		c.closed("1", traderA, "20000000000000000000", "2000000000000000000", true)
		c.opened("2", traderA, "0", "10000000000000000000", "60000", "1002000000")
		c.fee("2", traderA, 0, "1000000000000", "1")
		c.settled("1", traderA, "20000000000000000000", "2000000000000000000", "1000000000000", "0", "1999999")
		c.settled("2", traderA, "10000000000000000000", "0", "0", "0", "0")
		c.project()
		c.want(accountMarginReadSQL, []string{"1001999999", "1000000"}, traderA.Bytes(), usdlBytes)
		c.want(positionMarginReadSQL, []string{"1000000000000000000000", "1000000000", "closed"}, "1")
		c.want(positionMarginReadSQL, []string{"1001999999000000000000", "1001999999", "open"}, "2")
	})

	t.Run("loss_saturates_before_later_rebate", func(t *testing.T) {
		c := newChain(t, db)
		c.next(100, at)
		c.deposited(traderA, "1", "1")
		c.settled("1", traderA, "1", "-2000000000000", "0", "0", "0")
		c.fee("1", traderA, 4, "1000000000000", "1")
		c.project()
		c.want(accountMarginReadSQL, []string{"1", "0"}, traderA.Bytes(), usdlBytes)
	})

	t.Run("rebuild_and_rewind_preserve_account_and_closed_position_margin", func(t *testing.T) {
		c := newChain(t, db)
		c.next(100, at)
		c.blockRow(100, c.at)
		c.opened("1", traderA, "0", "20000000000000000000", "60000", "1000000000")
		c.fee("1", traderA, 0, "9000000000000000", "9000")
		c.project()
		c.next(101, at.Add(time.Second))
		c.blockRow(101, c.at)
		c.deposited(traderA, "25000000", "1024991000")
		c.project()
		c.next(102, at.Add(2*time.Second))
		c.blockRow(102, c.at)
		c.closed("1", traderA, "20000000000000000000", "1000000000000000000", true)
		c.fee("1", traderA, 0, "1000000000000", "1")
		c.settled("1", traderA, "20000000000000000000", "1000000000000000000", "1000000000000", "0", "999999")
		c.project()
		wantAccount := []string{"1025990999", "0"}
		wantClosed := []string{"1000000000000000000000", "1000000000", "closed"}
		c.want(accountMarginReadSQL, wantAccount, traderA.Bytes(), usdlBytes)
		c.want(positionMarginReadSQL, wantClosed, "1")
		if err := Rebuild(c.ctx, c.tx, 101, 102); err != nil {
			t.Fatal(err)
		}
		c.want(accountMarginReadSQL, wantAccount, traderA.Bytes(), usdlBytes)
		c.want(positionMarginReadSQL, wantClosed, "1")

		recovery, err := CaptureRecovery(c.ctx, c.tx, 102, 102)
		if err != nil {
			t.Fatal(err)
		}
		for _, table := range []string{"position_events", "fees", "trade_settlements", "blocks"} {
			column := "block"
			if table == "blocks" {
				column = "number"
			}
			if _, err := c.tx.ExecContext(c.ctx, `DELETE FROM perps.`+table+` WHERE `+column+` = 102`); err != nil {
				t.Fatal(err)
			}
		}
		if err := Rebuild(c.ctx, c.tx, 102, 102, recovery); err != nil {
			t.Fatal(err)
		}
		c.want(accountMarginReadSQL, []string{"1024991000", "2000000"}, traderA.Bytes(), usdlBytes)
		c.want(positionMarginReadSQL, []string{"1024991000000000000000", "1024991000", "open"}, "1")
		c.next(102, at.Add(2*time.Second))
		c.blockRow(102, c.at)
		c.closed("1", traderA, "20000000000000000000", "1000000000000000000", true)
		c.fee("1", traderA, 0, "1000000000000", "1")
		c.settled("1", traderA, "20000000000000000000", "1000000000000000000", "1000000000000", "0", "999999")
		c.project()
		c.want(accountMarginReadSQL, wantAccount, traderA.Bytes(), usdlBytes)
		c.want(positionMarginReadSQL, wantClosed, "1")
	})
}
