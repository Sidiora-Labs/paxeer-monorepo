package sequencer

import (
	"log/slog"
	"time"
)

func logLeaseFence(logger *slog.Logger, epoch uint64, expiry, now time.Time, cause error) {
	if logger != nil {
		logger.Error("sequencer fenced by lease", "epoch", epoch, "expiry", expiry, "time", now, "cause", cause)
	}
}
