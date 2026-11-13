package holdings

import (
	"io"
	"log/slog"
)

func closeResource(resource io.Closer) {
	if err := resource.Close(); err != nil {
		slog.Error("resource close failed", "error", err)
	}
}
