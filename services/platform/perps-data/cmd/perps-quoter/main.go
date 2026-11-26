// Command perps-quoter maintains bounded post-only orders for one funded account.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/quoter"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("perps-quoter stopped", "reason", err.Error())
		os.Exit(1)
	}
}
func run(logger *slog.Logger) error {
	cfg, err := config.Quoter.Load()
	if err != nil {
		return err
	}
	logger.Info("perps-quoter starting", "config", *cfg)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	node, err := quoter.NewRPC(ctx, cfg)
	if err != nil {
		return err
	}
	defer node.Close()
	engine, err := quoter.New(node, *cfg.Quoting, logger)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", ":"+strconv.FormatUint(uint64(cfg.Port), 10))
	if err != nil {
		return errors.New("quoter health listener unavailable")
	}
	return serve(ctx, stop, listener, engine, logger)
}

func serve(ctx context.Context, stop context.CancelFunc, listener net.Listener, engine *quoter.Engine, logger *slog.Logger) error {
	server := &http.Server{Handler: engine.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}
	httpErr := make(chan error, 1)
	go func() { httpErr <- server.Serve(listener) }()
	runErr := make(chan error, 1)
	go func() { runErr <- engine.Run(ctx, func() bool { return os.Getenv("PERPS_QUOTER_STOP") == "true" }) }()
	var result error
	select {
	case result = <-runErr:
	case err := <-httpErr:
		if !errors.Is(err, http.ErrServerClosed) {
			result = errors.New("quoter health server stopped")
		}
		stop()
		result = errors.Join(result, <-runErr)
	}
	stop()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdown); err != nil {
		result = errors.Join(result, errors.New("quoter health shutdown failed"))
	}
	health := engine.Health()
	logger.Info("perps-quoter shutdown", "mode", health.Mode, "ordersLeft", health.OrdersLeft)
	return result
}
