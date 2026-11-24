package records

import (
	"context"
	"database/sql"
	"time"
)

// WindowPrice carries the index and mark prices at the start of a window.
// Each value is null until its own source has a row at or before the bound.
type WindowPrice struct {
	Price     *float64
	MarkPrice *float64
}

// priceWindowSQL locates each producer's last block by block time, then seeks
// each market by market and block. Block times follow canonical block order.
// The two index candidates use the same ordering as latestMarkSQL.
const priceWindowSQL = `WITH mark_bound AS MATERIALIZED (
 SELECT block FROM perps.mark_prices WHERE block_time <= $1
 ORDER BY block_time DESC, block DESC LIMIT 1
), index_bound AS MATERIALIZED (
 SELECT block FROM perps.index_prices WHERE block_time <= $1
 ORDER BY block_time DESC, block DESC LIMIT 1
)
SELECT m.market_id::text, i.index_price / $2::numeric, mp.mark_price / $2::numeric
FROM perps.markets m
LEFT JOIN LATERAL (
 SELECT block FROM perps.mark_prices
 WHERE market_id = m.market_id AND block <= (SELECT block FROM mark_bound)
 ORDER BY block DESC LIMIT 1
) mb ON true
LEFT JOIN LATERAL (
 SELECT block, tx_index, log_index, sub, mark_price, index_price
 FROM perps.mark_prices WHERE market_id = m.market_id AND block = mb.block
 ORDER BY tx_index DESC, log_index DESC, sub DESC LIMIT 1
) mp ON true
LEFT JOIN LATERAL (
 SELECT index_price FROM (
  SELECT mp.block, mp.tx_index, mp.log_index, mp.sub, mp.index_price
  UNION ALL
  (SELECT block, tx_index, log_index, sub, index_price FROM perps.index_prices
   WHERE market_id = m.market_id AND block <= (SELECT block FROM index_bound)
    AND kind = 'aggregate' AND index_price IS NOT NULL
   ORDER BY block DESC, tx_index DESC, log_index DESC, sub DESC LIMIT 1)
 ) candidates WHERE index_price IS NOT NULL
 ORDER BY block DESC, tx_index DESC, log_index DESC, sub DESC LIMIT 1
) i ON true
ORDER BY m.market_id`

// PriceWindow reads both prices for every market in one statement, at now
// less 24 hours, without scanning the intervening window.
func PriceWindow(ctx context.Context, db *sql.DB, now time.Time) (out map[string]WindowPrice, resultErr error) {
	rows, err := db.QueryContext(ctx, priceWindowSQL, now.Add(-Window24h), USDScale)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); resultErr == nil {
			resultErr = err
		}
	}()
	out = make(map[string]WindowPrice)
	for rows.Next() {
		var id string
		var index, mark sql.NullString
		if err := rows.Scan(&id, &index, &mark); err != nil {
			return nil, err
		}
		var price WindowPrice
		if price.Price, err = Decimal(index); err != nil {
			return nil, err
		}
		if price.MarkPrice, err = Decimal(mark); err != nil {
			return nil, err
		}
		out[id] = price
	}
	return out, rows.Err()
}
