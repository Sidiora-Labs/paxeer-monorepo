package app

import (
	"log/slog"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/consumerhealth"
)

func newConsumerReadiness(cfg config.Config, name string, logger *slog.Logger, now func() time.Time) (*consumerhealth.State, error) {
	bound, err := cfg.ConsumerReadinessBound()
	if err != nil {
		return nil, err
	}
	return consumerhealth.New(name, bound, logger, now), nil
}
