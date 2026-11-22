package app

import (
	"context"
	"errors"
	"fmt"
)

func accountingRuntimeError(ctx context.Context, err error) error {
	if errors.Is(err, context.Canceled) && ctx.Err() != nil {
		return nil
	}
	if err == nil {
		err = errors.New("inventory consumer stopped unexpectedly")
	}
	return fmt.Errorf("pnl: inventory consumer failed: %w", err)
}
