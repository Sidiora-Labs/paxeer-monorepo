package policy

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Audit struct{ Actor, Action, Target, BeforeDigest, AfterDigest, PolicyVersion string }

// MutateAudited puts the business operation and immutable audit outbox insert
// in the same transaction. Table is an integrator-owned trusted identifier.
func MutateAudited(ctx context.Context, pool *pgxpool.Pool, table pgx.Identifier, record Audit, mutate func(pgx.Tx) error) (resultErr error) {
	if pool == nil || len(table) != 2 || mutate == nil || record.Actor == "" || record.Action == "" || record.PolicyVersion == "" {
		return ErrDenied
	}
	b, e := json.Marshal(record)
	if e != nil {
		return e
	}
	tx, e := pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer func() {
		c, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		if rollbackErr := tx.Rollback(c); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			resultErr = errors.Join(resultErr, rollbackErr)
		}
	}()
	if e = mutate(tx); e != nil {
		return e
	}
	// The exporter takes the exclusive side before reading a completed day.
	// An insert waiting behind it receives its timestamp after that snapshot.
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock_shared(hashtextextended($1,0))", table.Sanitize()); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "INSERT INTO "+table.Sanitize()+" (record) VALUES ($1::jsonb)", string(b)); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

func MutateAuditedV2(ctx context.Context, pool *pgxpool.Pool, table pgx.Identifier, record Audit, mutate func(pgx.Tx) error) error {
	if record.Target == "" || len(record.Target) > 512 || len(record.Actor) > 160 || len(record.Action) > 128 || len(record.PolicyVersion) > 64 || !digestPattern.MatchString(record.BeforeDigest) || !digestPattern.MatchString(record.AfterDigest) {
		return ErrDenied
	}
	return MutateAudited(ctx, pool, table, record, mutate)
}

var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
