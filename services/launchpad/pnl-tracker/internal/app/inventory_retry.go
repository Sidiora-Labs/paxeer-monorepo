package app

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

var inventoryServiceAddress = regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://[^\p{Z}\t\n\f\r\v]+`)

func inventoryRunErrorText(err error) string {
	text := []rune(inventoryServiceAddress.ReplaceAllString(err.Error(), "[redacted]"))
	if len(text) > 300 {
		text = text[:300]
	}
	return string(text)
}

func inventoryRunErrorClass(err error) (string, bool) {
	switch {
	case errors.Is(err, delivery.ErrAuthorization), errors.Is(err, nats.ErrAuthorization), errors.Is(err, nats.ErrPermissionViolation):
		return "authorization", true
	case errors.Is(err, delivery.ErrIntegrity):
		return "integrity", true
	case errors.Is(err, delivery.ErrSchema):
		return "schema", true
	case errors.Is(err, natsdriver.ErrClosed), errors.Is(err, nats.ErrConnectionClosed), errors.Is(err, nats.ErrConnectionDraining), errors.Is(err, jetstream.ErrConnectionClosed), errors.Is(err, jetstream.ErrConsumerDeleted):
		return "closed", true
	}
	var integrity *delivery.IntegrityFailure
	if errors.As(err, &integrity) {
		return "integrity", true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout", false
	}
	return "transient", false
}

func waitInventoryRun(ctx context.Context, delay time.Duration) {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func runInventoryAgain(ctx context.Context, logger *slog.Logger, run func(context.Context) error, wait func(context.Context, time.Duration), now func() time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	const stable = 30 * time.Second
	delay := time.Second
	failing := false
	lastClass, lastText := "", ""
	for {
		started := now()
		recovered := false
		monitor, stop := context.WithCancel(ctx)
		done := make(chan struct{})
		if failing {
			go func() {
				defer close(done)
				wait(monitor, stable)
				if monitor.Err() == nil && now().Sub(started) >= stable {
					logger.InfoContext(ctx, "inventory consumer", slog.String("job", "inventory consumer"), slog.String("state", "passing"))
					recovered = true
				}
			}()
		} else {
			close(done)
		}
		err := run(ctx)
		stop()
		<-done
		if ctx.Err() != nil {
			return err
		}
		if recovered {
			failing = false
		}
		if now().Sub(started) >= stable {
			delay = time.Second
			if failing {
				logger.InfoContext(ctx, "inventory consumer", slog.String("job", "inventory consumer"), slog.String("state", "passing"))
				failing = false
			}
		}
		class, permanent := inventoryRunErrorClass(err)
		if err == nil || permanent {
			return err
		}
		text := inventoryRunErrorText(err)
		if !failing || class != lastClass || text != lastText {
			logger.InfoContext(ctx, "inventory consumer", slog.String("job", "inventory consumer"), slog.String("state", "failing"), slog.String("class", class), slog.String("error", text))
			lastClass, lastText = class, text
			failing = true
		}
		wait(ctx, delay)
		if ctx.Err() != nil {
			return err
		}
		delay = min(delay*2, stable)
	}
}
