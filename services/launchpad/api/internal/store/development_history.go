package store

import (
	"context"
	"errors"
)

// Development-mode history scope checks read the indexer's legacy tables.
// Committed trades and candles are served only with the development stream
// projector's canonical receipt completion; unprojected legacy data refuses.

// ErrDevelopmentScope reports a development read called with an invalid scope.
var ErrDevelopmentScope = errors.New("development read scope invalid")

// DevelopmentPoolIndexed reports whether the development indexer recorded the
// pool's creation (indexer.pools, written by core/indexer InsertPool).
func (s *Store) DevelopmentPoolIndexed(ctx context.Context, pool string) (bool, error) {
	if normalized, err := Address(pool); err != nil || normalized != pool {
		return false, ErrDevelopmentScope
	}
	ctx, cancel, tx, err := s.beginLegacyRead(ctx)
	if err != nil {
		return false, err
	}
	defer cancel()
	defer rollbackRead(tx)
	var found bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM indexer.pools WHERE pool_address=$1)`, pool).Scan(&found); err != nil {
		return false, err
	}
	return found, tx.Commit(ctx)
}

// DevelopmentPoolsIndexed returns the subset of pools (canonical lowercase
// addresses, at most 100) the development indexer recorded in indexer.pools.
func (s *Store) DevelopmentPoolsIndexed(ctx context.Context, pools []string) (map[string]bool, error) {
	if len(pools) == 0 || len(pools) > 100 {
		return nil, ErrDevelopmentScope
	}
	for _, pool := range pools {
		if normalized, err := Address(pool); err != nil || normalized != pool {
			return nil, ErrDevelopmentScope
		}
	}
	ctx, cancel, tx, err := s.beginLegacyRead(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()
	defer rollbackRead(tx)
	rows, err := tx.Query(ctx, `SELECT DISTINCT lower(pool_address) FROM indexer.pools WHERE lower(pool_address)=ANY($1)`, pools)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	found := map[string]bool{}
	for rows.Next() {
		var pool string
		if err = rows.Scan(&pool); err != nil {
			return nil, err
		}
		found[pool] = true
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	return found, tx.Commit(ctx)
}

// DevelopmentSwapsRecorded reports whether indexer.swaps holds any swap for the
// pool, optionally restricted to one side (literalBuy or literalSell).
func (s *Store) DevelopmentSwapsRecorded(ctx context.Context, pool, side string) (bool, error) {
	if normalized, err := Address(pool); err != nil || normalized != pool || side != "" && side != literalBuy && side != literalSell {
		return false, ErrDevelopmentScope
	}
	ctx, cancel, tx, err := s.beginLegacyRead(ctx)
	if err != nil {
		return false, err
	}
	defer cancel()
	defer rollbackRead(tx)
	var found bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM indexer.swaps WHERE pool_address=$1 AND ($2='' OR is_buy=($2='buy')))`, pool, side).Scan(&found); err != nil {
		return false, err
	}
	return found, tx.Commit(ctx)
}

// DevelopmentCandlesRecorded reports whether candles.candles holds a bucket for
// the pool and timeframe whose start lies in the half-open second range
// [fromSeconds, toSeconds).
func (s *Store) DevelopmentCandlesRecorded(ctx context.Context, pool, timeframe string, fromSeconds, toSeconds int64) (bool, error) {
	if normalized, err := Address(pool); err != nil || normalized != pool || timeframe == "" || fromSeconds < 0 || toSeconds <= fromSeconds {
		return false, ErrDevelopmentScope
	}
	ctx, cancel, tx, err := s.beginLegacyRead(ctx)
	if err != nil {
		return false, err
	}
	defer cancel()
	defer rollbackRead(tx)
	var found bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM candles.candles WHERE pool_address=$1 AND timeframe=$2 AND candle_start>=$3 AND candle_start<$4)`, pool, timeframe, fromSeconds, toSeconds).Scan(&found); err != nil {
		return false, err
	}
	return found, tx.Commit(ctx)
}
