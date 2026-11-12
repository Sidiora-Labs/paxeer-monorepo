package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"

	"github.com/jackc/pgx/v5"
)

func closeResource(resource io.Closer) {
	if err := resource.Close(); err != nil {
		slog.Error("metadata resource close", "error", err)
	}
}

func closeConnection(ctx context.Context, connection *pgx.Conn) {
	if err := connection.Close(ctx); err != nil {
		slog.Error("metadata database connection close", "error", err)
	}
}

func rollbackTransaction(ctx context.Context, tx pgx.Tx) {
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		slog.Error("metadata transaction rollback", "error", err)
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("metadata response write", "error", err)
	}
}

func checkedImageSize(length int) (int32, error) {
	if length < 0 || length > math.MaxInt32 {
		return 0, ErrValidation
	}
	//nolint:gosec // The checked bound preserves the database's int32 image size field.
	return int32(length), nil
}
