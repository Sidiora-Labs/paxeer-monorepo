package internaltest

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v5"
)

// QueryCancellation cancels a selected PostgreSQL operation before it is sent.
// Other queries execute normally against the connected database.
type QueryCancellation struct {
	mu      sync.Mutex
	enabled bool
	target  int
	queries []string
}

func (q *QueryCancellation) Start(target int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.enabled = true
	q.target = target
	q.queries = nil
}

func (q *QueryCancellation) Stop() []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.enabled = false
	return append([]string(nil), q.queries...)
}

func (q *QueryCancellation) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.enabled {
		return ctx
	}
	q.queries = append(q.queries, data.SQL)
	if q.target > 0 && len(q.queries) == q.target {
		q.enabled = false
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		return canceled
	}
	return ctx
}

func (*QueryCancellation) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}
