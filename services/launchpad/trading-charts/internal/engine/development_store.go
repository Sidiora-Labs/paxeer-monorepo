package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DevelopmentSchemaDDL must be applied by the migration owner before enabling
// development ingestion. It is deliberately not an automatic runtime migration.
const DevelopmentSchemaDDL = `CREATE TABLE IF NOT EXISTS "candles"."development_state" (
    "key" text PRIMARY KEY,
    "payload" bytea NOT NULL
);`

type developmentSQLStore struct{ pool *pgxpool.Pool }
type developmentSQLTx struct {
	ctx context.Context
	tx  pgx.Tx
}

func (s *developmentSQLStore) Transaction(ctx context.Context, pool, identity string, apply func(developmentKV) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	if identity != "" {
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "development:identity:"+identity); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "development:pool:"+pool); err != nil {
		return err
	}
	if err = apply(&developmentSQLTx{ctx: ctx, tx: tx}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (t *developmentSQLTx) Get(key string) ([]byte, error) {
	var raw []byte
	err := t.tx.QueryRow(t.ctx, `SELECT payload FROM candles.development_state WHERE key=$1`, key).Scan(&raw)
	if !errors.Is(err, pgx.ErrNoRows) {
		return raw, err
	}
	kind, parts, err := developmentParts(key)
	if err != nil {
		return nil, err
	}
	if kind == "candle" {
		// An old projection has neither source ordering nor durable membership.
		// Reconstruct it from source rather than silently resetting its totals.
		start, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return nil, err
		}
		var exists bool
		err = t.tx.QueryRow(t.ctx, `SELECT EXISTS(SELECT 1 FROM candles.candles WHERE pool_address=$1 AND timeframe=$2 AND candle_start=$3)`, parts[0], parts[1], start).Scan(&exists)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, errors.New("engine: development candle requires source reconstruction")
		}
	}
	if kind == "cursor" {
		var c store.CursorRow
		err = t.tx.QueryRow(t.ctx, `SELECT pool_address,timeframe,last_close,last_candle_start,last_sequence_num FROM candles.candle_cursors WHERE pool_address=$1 AND timeframe=$2`, parts[0], parts[1]).Scan(&c.PoolAddress, &c.Timeframe, &c.LastClose, &c.LastCandleStart, &c.LastSequenceNum)
		if err == nil {
			return json.Marshal(c)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	return nil, nil
}
func developmentParts(key string) (string, []string, error) {
	kind, wire, ok := strings.Cut(key, "/")
	var parts []string
	if !ok || json.Unmarshal([]byte(wire), &parts) != nil {
		return "", nil, errors.New("engine: invalid development key")
	}
	want := map[string]int{"candle": 3, "cursor": 2, "outbox": 1, "claim": 2}[kind]
	if want == 0 || len(parts) != want {
		return "", nil, errors.New("engine: invalid development key arity")
	}
	return kind, parts, nil
}
func (t *developmentSQLTx) Put(key string, raw []byte) error {
	kind, _, err := developmentParts(key)
	if err != nil {
		return err
	}
	if kind == "candle" {
		var state developmentCandle
		if err = json.Unmarshal(raw, &state); err != nil {
			return err
		}
		c := state.Row
		_, err = t.tx.Exec(t.ctx, `INSERT INTO candles.candles (
            pool_address,timeframe,candle_start,open,high,low,close,
            volume_usdl,volume_token,buy_volume_usdl,sell_volume_usdl,
            trade_count,unique_traders,large_trade_count,last_trade_ts,sequence_num,
            mcap_open,mcap_high,mcap_low,mcap_close
        ) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
        ON CONFLICT(pool_address,timeframe,candle_start) DO UPDATE SET
            open=EXCLUDED.open,high=EXCLUDED.high,low=EXCLUDED.low,close=EXCLUDED.close,
            volume_usdl=EXCLUDED.volume_usdl,volume_token=EXCLUDED.volume_token,
            buy_volume_usdl=EXCLUDED.buy_volume_usdl,sell_volume_usdl=EXCLUDED.sell_volume_usdl,
            trade_count=EXCLUDED.trade_count,unique_traders=EXCLUDED.unique_traders,
            large_trade_count=EXCLUDED.large_trade_count,last_trade_ts=EXCLUDED.last_trade_ts,
            sequence_num=EXCLUDED.sequence_num,mcap_open=EXCLUDED.mcap_open,
            mcap_high=EXCLUDED.mcap_high,mcap_low=EXCLUDED.mcap_low,mcap_close=EXCLUDED.mcap_close`,
			c.PoolAddress, c.Timeframe, c.CandleStart, c.Open, c.High, c.Low, c.Close,
			c.VolumeUsdl, c.VolumeToken, c.BuyVolumeUsdl, c.SellVolumeUsdl,
			c.TradeCount, c.UniqueTraders, c.LargeTradeCount, c.LastTradeTs, c.SequenceNum,
			c.McapOpen, c.McapHigh, c.McapLow, c.McapClose)
		if err != nil {
			return err
		}
	}
	if kind == "cursor" {
		var c store.CursorRow
		if err = json.Unmarshal(raw, &c); err != nil {
			return err
		}
		_, err = t.tx.Exec(t.ctx, `INSERT INTO candles.candle_cursors AS current_cursor(pool_address,timeframe,last_close,last_candle_start,last_sequence_num)
            VALUES($1,$2,$3,$4,$5) ON CONFLICT(pool_address,timeframe) DO UPDATE SET
            last_close=EXCLUDED.last_close,last_candle_start=EXCLUDED.last_candle_start,last_sequence_num=EXCLUDED.last_sequence_num
            WHERE current_cursor.last_candle_start <= EXCLUDED.last_candle_start`,
			c.PoolAddress, c.Timeframe, c.LastClose, c.LastCandleStart, c.LastSequenceNum)
		if err != nil {
			return err
		}
	}
	_, err = t.tx.Exec(t.ctx, `INSERT INTO candles.development_state(key,payload) VALUES($1,$2) ON CONFLICT(key) DO UPDATE SET payload=EXCLUDED.payload`, key, raw)
	return err
}
func (t *developmentSQLTx) Claim(swap SwapEvent) (bool, error) {
	key, raw, err := developmentClaim(swap)
	if err != nil {
		return false, err
	}
	tag, err := t.tx.Exec(t.ctx, `INSERT INTO candles.processed_swaps(tx_hash,log_index) VALUES($1,$2) ON CONFLICT DO NOTHING`, swap.TxHash, swap.LogIndex)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		old, err := t.Get(key)
		if err != nil {
			return false, err
		}
		if len(old) > 0 && !bytes.Equal(old, raw) {
			return false, errors.New("engine: conflicting swap identity")
		}
		return false, nil
	}
	return true, t.Put(key, raw)
}
func developmentClaim(swap SwapEvent) (string, []byte, error) {
	raw, err := json.Marshal(swap)
	if err != nil {
		return "", nil, fmt.Errorf("engine: encode source swap: %w", err)
	}
	return developmentKey("claim", swap.TxHash, fmt.Sprint(swap.LogIndex)), raw, nil
}
