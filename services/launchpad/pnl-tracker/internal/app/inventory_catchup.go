package app

import (
	"context"
	"log/slog"
	"time"

	inventory "github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l16"
)

func (a *App) startAccountingJobs(ctx context.Context, runtimeErr chan<- error, cancel context.CancelFunc, pass inventory.InventoryCatchUp) bool {
	if a.accounting == nil {
		return false
	}
	go func() {
		release, err := a.tracker.Acquire()
		if err != nil {
			return
		}
		defer release()
		runInventoryCatchUp(ctx, a.logger, pass)
	}()
	go func() {
		release, err := a.tracker.Acquire()
		if err != nil {
			return
		}
		defer release()
		if err := accountingRuntimeError(ctx, runInventoryAgain(ctx, a.logger, a.runAccounting, waitInventoryRun, time.Now)); err != nil {
			runtimeErr <- err
			a.logger.Error("inventory consumer failed", slog.Any("err", err))
			a.tracker.BeginDrain()
			cancel()
		}
	}()
	return true
}

func runInventoryCatchUp(ctx context.Context, logger *slog.Logger, pass inventory.InventoryCatchUp) {
	ticker := time.NewTicker(pass.Period)
	defer ticker.Stop()
	lastClass, lastText := "", ""
	failing := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			applied, err := pass.RunOnce(ctx)
			if ctx.Err() != nil {
				return
			}
			if applied > 0 {
				logger.InfoContext(ctx, "inventory catch up", slog.Int("applied", applied))
			}
			if err == nil {
				failing = false
				continue
			}
			class, _ := inventoryRunErrorClass(err)
			text := inventoryRunErrorText(err)
			if !failing || class != lastClass || text != lastText {
				logger.InfoContext(ctx, "inventory catch up", slog.String("state", "failing"), slog.String("class", class), slog.String("error", text))
				lastClass, lastText = class, text
				failing = true
			}
		}
	}
}
