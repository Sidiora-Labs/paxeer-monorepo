package metadata

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"github.com/jackc/pgx/v5/pgconn"
)

// LogFailure identifies dependency failures without logging driver messages,
// which can contain credentials, request bodies or connection addresses.
func LogFailure(logger *slog.Logger, message string, err error, attrs ...slog.Attr) {
	if logger == nil || err == nil {
		return
	}
	cause := err
	for errors.Unwrap(cause) != nil {
		cause = errors.Unwrap(cause)
	}
	attrs = append(attrs, slog.String("cause_type", fmt.Sprintf("%T", cause)))
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		attrs = append(attrs, slog.String("cause", "deadline_exceeded"))
	case errors.Is(err, context.Canceled):
		attrs = append(attrs, slog.String("cause", "canceled"))
	case errors.Is(err, ErrUnrepresentable):
		attrs = append(attrs, slog.String("cause", "unrepresentable_state"))
	default:
		var database *pgconn.PgError
		var network *net.OpError
		if errors.As(err, &database) {
			attrs = append(attrs, slog.String("sqlstate", database.Code))
		} else if errors.As(err, &network) {
			attrs = append(attrs, slog.Bool("network_timeout", network.Timeout()))
		}
	}
	logger.LogAttrs(context.Background(), slog.LevelError, message, attrs...)
}
