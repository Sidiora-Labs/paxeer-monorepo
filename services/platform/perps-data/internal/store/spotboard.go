package store

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math/big"
	"slices"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/decode"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/pnl"
)

// Native swaps have no attributed stablecoin cost basis (decision.349).
// Order fills use the gross execution price, as in rest.spotTradesCTE.
const spotBoardLegsSQL = `WITH stable AS (
    SELECT token FROM perps.spot_token_registry
    WHERE kind = 'token_registered' AND is_stablecoin AND block <= $2
), raw AS (
    SELECT s.block, s.tx_index, s.log_index, s.sub, s.block_time,
        CASE WHEN s.is_buy THEN s.stablecoin ELSE s.token END AS token_in,
        CASE WHEN s.is_buy THEN s.token ELSE s.stablecoin END AS token_out,
        (CASE WHEN s.is_buy THEN s.stablecoin_amount ELSE s.token_amount END)::numeric AS amount_in,
        (CASE WHEN s.is_buy THEN s.token_amount ELSE s.stablecoin_amount END)::numeric AS amount_out
    FROM perps.spot_swaps s WHERE s.kind = 'market_order' AND s."user" = $1 AND s.block <= $2
    UNION ALL
    SELECT s.block, s.tx_index, s.log_index, s.sub, s.block_time,
        s.token_in, s.token_out, s.amount_in::numeric, s.amount_out::numeric
    FROM perps.spot_swaps s WHERE s.kind = 'simple_swap' AND s."user" = $1 AND s.block <= $2
    UNION ALL
    SELECT e.block, e.tx_index, e.log_index, e.sub, e.block_time,
        CASE WHEN o.side = 'buy' THEN o.stablecoin ELSE o.token END,
        CASE WHEN o.side = 'buy' THEN o.token ELSE o.stablecoin END,
        o.amount::numeric,
        trunc(CASE WHEN o.side = 'buy' THEN o.amount * 1000000000000000000 / e.execution_price
            ELSE o.amount * e.execution_price / 1000000000000000000 END)
    FROM perps.spot_order_events e
    JOIN perps.spot_orders o ON o.order_id = e.order_id AND e.kind = o.kind || '_executed'
    WHERE e.execution_price > 0 AND o.account = $1 AND e.block <= $2
), legs AS (
    SELECT raw.*, token_in IN (SELECT token FROM stable) AS in_stable,
        token_out IN (SELECT token FROM stable) AS out_stable FROM raw
)
SELECT CASE WHEN in_stable THEN token_out ELSE token_in END, in_stable,
    trunc(CASE WHEN in_stable THEN amount_in ELSE amount_out END)::text,
    trunc(CASE WHEN in_stable THEN amount_out ELSE amount_in END)::text, block_time
FROM legs WHERE in_stable <> out_stable
    AND CASE WHEN in_stable THEN amount_out ELSE amount_in END >= 1
ORDER BY block, tx_index, log_index, sub`

type spotBoardAccount struct {
	realized, volume     big.Int
	trades, wins, losses int64
}

func foldSpotAccount(ctx context.Context, tx *sql.Tx, account common.Address, block int64) (spotBoardAccount, error, error) {
	var total spotBoardAccount
	rows, err := tx.QueryContext(ctx, spotBoardLegsSQL, account.Bytes(), block)
	if err != nil {
		return total, nil, err
	}
	defer rows.Close()
	books := map[common.Address]pnl.Position{}
	var refusal error
	for rows.Next() {
		var token []byte
		var trade pnl.Trade
		var at time.Time
		if err := rows.Scan(&token, &trade.IsBuy, &trade.UsdlAmount, &trade.TokenAmount, &at); err != nil {
			return total, nil, err
		}
		if refusal != nil {
			continue
		}
		trade.Ts = at.Unix()
		key := common.BytesToAddress(token)
		before := books[key]
		after, err := pnl.Fold(before, trade)
		if err != nil {
			refusal = err
			continue
		}
		books[key] = after
		var volume, prior, realized big.Int
		if _, ok := volume.SetString(trade.UsdlAmount, 10); !ok {
			return total, nil, fmt.Errorf("invalid spot volume %q", trade.UsdlAmount)
		}
		total.volume.Add(&total.volume, &volume)
		total.trades++
		if before.RealizedPnlUsdl != "" {
			if _, ok := prior.SetString(before.RealizedPnlUsdl, 10); !ok {
				return total, nil, fmt.Errorf("invalid prior spot realized %q", before.RealizedPnlUsdl)
			}
		}
		if _, ok := realized.SetString(after.RealizedPnlUsdl, 10); !ok {
			return total, nil, fmt.Errorf("invalid spot realized %q", after.RealizedPnlUsdl)
		}
		delta := new(big.Int).Sub(&realized, &prior)
		total.realized.Add(&total.realized, delta)
		if !trade.IsBuy {
			switch delta.Sign() {
			case 1:
				total.wins++
			case -1:
				total.losses++
			}
		}
	}
	return total, refusal, rows.Err()
}

// FoldSpotBoard refolds the stored history of the accounts traded in rows and
// ranks their running totals with the other accounts, in the block transaction.
func FoldSpotBoard(ctx context.Context, tx *sql.Tx, rows []decode.Row) error {
	if len(rows) == 0 {
		return nil
	}
	block, err := blockArg(rows[0].Common().Block)
	if err != nil {
		return err
	}
	var accounts set[common.Address]
	for _, row := range rows {
		if row.Common().Block != uint64(block) {
			return fmt.Errorf("store: FoldSpotBoard rows span blocks")
		}
		switch r := row.(type) {
		case decode.SpotMarketOrderRow:
			accounts.add(r.User)
		case decode.SpotSimpleSwapRow:
			accounts.add(r.User)
		case decode.SpotOrderExecutedRow:
			var account []byte
			kind := strings.TrimSuffix(r.Kind, "_executed")
			if err := tx.QueryRowContext(ctx, `SELECT account FROM perps.spot_orders WHERE kind = $1 AND order_id = $2`, kind, r.OrderID).Scan(&account); err != nil {
				return fmt.Errorf("store: spot board order owner: %w", err)
			}
			accounts.add(common.BytesToAddress(account))
		}
	}
	if len(accounts.list) == 0 {
		return nil
	}
	winRates := map[common.Address]*big.Rat{}
	stored, err := tx.QueryContext(ctx, `SELECT account, value::text FROM perps.spot_leaderboard WHERE kind = 'win-rate'`)
	if err != nil {
		return fmt.Errorf("store: read spot win rates: %w", err)
	}
	for stored.Next() {
		var account []byte
		var value string
		if err := stored.Scan(&account, &value); err != nil {
			stored.Close()
			return err
		}
		rate, ok := new(big.Rat).SetString(value)
		if !ok {
			stored.Close()
			return fmt.Errorf("store: invalid spot win rate %q", value)
		}
		winRates[common.BytesToAddress(account)] = rate
	}
	err = stored.Err()
	if closeErr := stored.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	for _, account := range accounts.list {
		total, refusal, err := foldSpotAccount(ctx, tx, account, block)
		if err != nil {
			return fmt.Errorf("store: fold spot account %s: %w", account.Hex(), err)
		}
		delete(winRates, account)
		if refusal != nil || total.trades == 0 {
			if refusal != nil {
				slog.Warn("spot board: account left out", "account", account.Hex(), "block", block, "reason", refusal.Error())
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM perps.spot_account_legs WHERE account = $1`, account.Bytes()); err != nil {
				return err
			}
			continue
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO perps.spot_account_legs (account, realized_usd, volume_usd, trades, updated_block)
VALUES ($1, $2, $3, $4, $5) ON CONFLICT (account) DO UPDATE SET
realized_usd = EXCLUDED.realized_usd, volume_usd = EXCLUDED.volume_usd,
trades = EXCLUDED.trades, updated_block = EXCLUDED.updated_block`, account.Bytes(), total.realized.String(), total.volume.String(), total.trades, block)
		if err != nil {
			return fmt.Errorf("store: write spot account: %w", err)
		}
		if decided := total.wins + total.losses; decided > 0 {
			rate := new(big.Rat).Mul(new(big.Rat).SetFrac(big.NewInt(total.wins), big.NewInt(decided)), big.NewRat(100, 1))
			// Compare refreshed and stored percentages at the same precision.
			storedRate, ok := new(big.Rat).SetString(rate.FloatString(40))
			if !ok {
				return fmt.Errorf("store: invalid computed spot win rate for %s", account.Hex())
			}
			winRates[account] = storedRate
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM perps.spot_leaderboard`); err != nil {
		return fmt.Errorf("store: clear spot leaderboard: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO perps.spot_leaderboard (kind, rank, account, value, updated_block)
SELECT kind, row_number() OVER (PARTITION BY kind ORDER BY value DESC, account), account, value, $1
FROM (SELECT 'pnl' AS kind, account, realized_usd AS value FROM perps.spot_account_legs
    UNION ALL SELECT 'volume', account, volume_usd FROM perps.spot_account_legs) scores`, block)
	if err != nil {
		return fmt.Errorf("store: rank spot leaderboard: %w", err)
	}
	ranked := make([]common.Address, 0, len(winRates))
	for account := range winRates {
		ranked = append(ranked, account)
	}
	slices.SortFunc(ranked, func(a, b common.Address) int {
		if cmp := winRates[b].Cmp(winRates[a]); cmp != 0 {
			return cmp
		}
		return bytes.Compare(a.Bytes(), b.Bytes())
	})
	for i, account := range ranked {
		// Forty places preserve ordering of percentages whose counts fit bigint.
		_, err := tx.ExecContext(ctx, `INSERT INTO perps.spot_leaderboard (kind, rank, account, value, updated_block)
VALUES ('win-rate', $1, $2, $3, $4)`, i+1, account.Bytes(), winRates[account].FloatString(40), block)
		if err != nil {
			return fmt.Errorf("store: rank spot win rate: %w", err)
		}
	}
	return nil
}
