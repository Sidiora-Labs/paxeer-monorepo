package app

import (
	"context"
	"fmt"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l18"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newRewardLedger admits the deployed storage contract before reward routes are mounted.
// Schema deployment remains the migration coordinator's responsibility, never pnld's.
func newRewardLedger(ctx context.Context, pool *pgxpool.Pool, schema, service string, authority l18.Authority, guard func(context.Context, pgx.Tx) error) (*l18.Ledger, error) {
	ledger, err := l18.NewRuntimeLedger(pool, schema, service, authority, guard)
	if err != nil {
		return nil, err
	}
	if err := checkRewardRequestSchema(ctx, pool, schema); err != nil {
		return nil, err
	}
	return ledger, nil
}

func checkRewardRequestSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, table := range []struct {
		name    string
		columns []string
		types   []uint32
		key     []string
	}{
		{"l18_requests", []string{actorKind, "request_key", "digest", "method", "path", "response", "expires_at", "release_digest", "interface_digest"}, []uint32{pgtype.TextOID, pgtype.UUIDOID, pgtype.TextOID, pgtype.TextOID, pgtype.TextOID, pgtype.ByteaOID, pgtype.TimestamptzOID, pgtype.TextOID, pgtype.TextOID}, []string{actorKind, "request_key"}},
		{"l18_audit", []string{actorIdentity, actorKind, "action", "reason", "payload"}, []uint32{pgtype.TextOID, pgtype.TextOID, pgtype.TextOID, pgtype.TextOID, pgtype.JSONBOID}, []string{actorIdentity}},
	} {
		qualified := pgx.Identifier{schema, table.name}.Sanitize()
		columns := ""
		for i, column := range table.columns {
			if i > 0 {
				columns += ","
			}
			columns += pgx.Identifier{column}.Sanitize()
		}
		rows, err := tx.Query(ctx, "SELECT "+columns+" FROM "+qualified+" LIMIT 0")
		if err != nil {
			return fmt.Errorf("pnl: reward migration 180001 required (%s): %w", table.name, err)
		}
		fields := rows.FieldDescriptions()
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for i, field := range fields {
			if field.DataTypeOID != table.types[i] {
				return fmt.Errorf("pnl: incompatible reward storage column %s.%s", table.name, field.Name)
			}
		}
		var writable, identity bool
		err = tx.QueryRow(ctx, `SELECT
			has_table_privilege($1,'SELECT') AND has_table_privilege($1,'INSERT'),
			EXISTS(SELECT 1 FROM pg_catalog.pg_index i
				WHERE i.indrelid=$1::regclass AND i.indisprimary AND i.indisvalid
				AND ARRAY(SELECT a.attname::text FROM unnest(i.indkey) WITH ORDINALITY k(attnum,position)
					JOIN pg_catalog.pg_attribute a ON a.attrelid=i.indrelid AND a.attnum=k.attnum
					WHERE k.position<=i.indnkeyatts ORDER BY k.position)=$2::text[])
		`, qualified, table.key).Scan(&writable, &identity)
		if err != nil {
			return err
		}
		if !writable || !identity {
			return fmt.Errorf("pnl: reward storage %s requires SELECT, INSERT and its deployed primary key", table.name)
		}
	}
	return tx.Commit(ctx)
}
