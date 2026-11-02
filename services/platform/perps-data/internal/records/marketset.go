package records

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"
)

type MarketSet struct {
	db          *sql.DB
	ttl         time.Duration
	mu          sync.Mutex
	ids         map[string]string
	loadedAt    time.Time
	lastAttempt time.Time
	lastMiss    time.Time
	refreshErr  error
}

func NewMarketSet(db *sql.DB, ttl time.Duration) *MarketSet {
	return &MarketSet{db: db, ttl: ttl}
}

func (s *MarketSet) ID(ctx context.Context, symbol string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	id, ok := s.ids[symbol]
	expired := s.ids == nil || now.Sub(s.loadedAt) >= s.ttl
	miss := !ok && (s.lastMiss.IsZero() || now.Sub(s.lastMiss) >= s.ttl)
	if expired || miss {
		if s.refreshErr != nil && now.Sub(s.lastAttempt) < s.ttl {
			return "", false, s.refreshErr
		}
		if !ok && s.ids != nil {
			s.lastMiss = now
		}
		s.lastAttempt = now
		s.refreshErr = s.reload(ctx)
		if s.refreshErr != nil {
			return "", false, s.refreshErr
		}
		id, ok = s.ids[symbol]
	}
	return id, ok, nil
}

func (s *MarketSet) reload(ctx context.Context) error {
	if s.db == nil {
		return errors.New("records: nil database")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT symbol, market_id::text FROM perps.markets`)
	if err != nil {
		return err
	}
	defer rows.Close()
	ids := make(map[string]string)
	for rows.Next() {
		var symbol, id string
		if err := rows.Scan(&symbol, &id); err != nil {
			return err
		}
		ids[symbol] = id
	}
	if err := rows.Err(); err != nil {
		return err
	}
	s.ids = ids
	s.loadedAt = time.Now()
	return nil
}
