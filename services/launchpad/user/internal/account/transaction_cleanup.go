package account

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
)

func rollbackAccountTx(ctx context.Context, tx pgx.Tx) {
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		slog.Warn("account transaction rollback", "error", err)
	}
}
