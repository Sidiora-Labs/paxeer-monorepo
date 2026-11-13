package store

import (
	"context"
	"encoding/json"

	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

// DevelopmentTrades serves one page, at most MaxCanonicalTrades (100), newest
// chain position first, as production does. Development issues no trade cursor.
// Every matching source row must have a canonical commit, including rows beyond
// the page boundary; a bounded page cannot hide an unprojected legacy row.
func (s *Store) DevelopmentTrades(ctx context.Context, r AccountingPageRequest) ([]json.RawMessage, error) {
	if pool, err := Address(r.Pool); err != nil || pool != r.Pool || r.Side != "" && r.Side != literalBuy && r.Side != literalSell || r.Limit < 1 || r.Limit > MaxCanonicalTrades {
		return nil, ErrDevelopmentScope
	}
	ctx, cancel, tx, err := s.beginLegacyRead(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()
	defer rollbackRead(tx)
	items := []json.RawMessage{}
	var recorded, installed bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM indexer.swaps WHERE pool_address=$1 AND ($2='' OR is_buy=($2='buy'))),to_regclass('api_stream_dev.trade_projection') IS NOT NULL`, r.Pool, r.Side).Scan(&recorded, &installed); err != nil {
		return nil, err
	}
	if !recorded {
		return items, tx.Commit(ctx)
	}
	if !installed {
		return nil, ErrDataIntegrity
	}
	var unprojected bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM indexer.swaps s
 LEFT JOIN api_stream_dev.trade_projection p ON p.swap_id=s.id
 WHERE s.pool_address=$1 AND ($2='' OR s.is_buy=($2='buy')) AND
 (p.swap_id IS NULL OR p.pool_address<>s.pool_address OR p.block_number<>s.block_number OR p.log_index<>s.log_index OR NOT
 (convert_from(p.payload,'UTF8')::jsonb @> jsonb_build_object(
 'poolAddress',s.pool_address,'sender',s.sender,'side',CASE WHEN s.is_buy THEN 'buy' ELSE 'sell' END,
 'tokenAmountRaw',CASE WHEN s.is_buy THEN s.amount_out ELSE s.amount_in END,
 'usdlAmountRaw',CASE WHEN s.is_buy THEN s.amount_in ELSE s.amount_out END,'feeRaw',s.fee,
 'position',jsonb_build_object('transactionHash',s.tx_hash,'height',s.block_number::text,'logIndex',s.log_index::text)))))`, r.Pool, r.Side).Scan(&unprojected)
	if err != nil {
		return nil, err
	}
	if unprojected {
		return nil, ErrDataIntegrity
	}
	rows, err := tx.Query(ctx, `SELECT p.payload FROM indexer.swaps s JOIN api_stream_dev.trade_projection p ON p.swap_id=s.id
 WHERE s.pool_address=$1 AND ($2='' OR s.is_buy=($2='buy'))
 ORDER BY p.block_number DESC,p.transaction_index DESC,p.log_index DESC,p.event_id DESC LIMIT $3`, r.Pool, r.Side, r.Limit)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			rows.Close()
			return nil, err
		}
		if len(raw) > PublicJSONMaxBytes || interfaces.ValidateJSON("Trade", raw) != nil {
			rows.Close()
			return nil, ErrDataIntegrity
		}
		items = append(items, json.RawMessage(raw))
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return items, tx.Commit(ctx)
}
