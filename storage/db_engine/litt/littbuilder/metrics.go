package littbuilder

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	commonmetrics "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/metrics"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/metrics"
)

// Storage initializes one process-wide provider. Its registry and instruments
// remain live for the process lifetime, including when no DB is open. Individual
// DBs own only their HTTP listeners and collection workers.
var processMetrics struct {
	once     sync.Once
	registry *prometheus.Registry
	err      error
}

type metricsServer struct {
	ctx     context.Context
	cancel  context.CancelFunc
	address net.Addr
	done    chan struct{}
	err     error
}

func buildMetrics(config *litt.Config, runtimeConfig *litt.RuntimeConfig) (*metrics.LittDBMetrics, *metricsServer, error) {
	if !config.MetricsEnabled {
		return nil, nil, nil
	}
	if config.MetricsUpdateInterval <= 0 {
		return nil, nil, fmt.Errorf("metrics update interval must be positive")
	}
	if err := runtimeConfig.CTX.Err(); err != nil {
		return nil, nil, fmt.Errorf("metrics context: %w", err)
	}
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", config.MetricsPort))
	if err != nil {
		return nil, nil, fmt.Errorf("bind metrics listener: %w", err)
	}
	processMetrics.once.Do(func() {
		processMetrics.registry, _, processMetrics.err = commonmetrics.SetupOtelPrometheus()
	})
	if processMetrics.err != nil {
		return nil, nil, errors.Join(processMetrics.err, listener.Close())
	}

	ctx, cancel := context.WithCancel(runtimeConfig.CTX)
	serving := &metricsServer{
		ctx: ctx, cancel: cancel, address: listener.Addr(), done: make(chan struct{}),
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(processMetrics.registry, promhttp.HandlerOpts{}))
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- server.Serve(listener)
	}()
	go func() {
		defer close(serving.done)
		var serveErr error
		serveStopped := false
		select {
		case <-ctx.Done():
		case serveErr = <-serveDone:
			serveStopped = true
			cancel()
		}
		shutdownCTX, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		serving.err = server.Shutdown(shutdownCTX)
		if serving.err != nil {
			serving.err = errors.Join(serving.err, server.Close())
		}
		if !serveStopped {
			serveErr = <-serveDone
		}
		if !errors.Is(serveErr, http.ErrServerClosed) {
			serving.err = errors.Join(serving.err, serveErr)
		}
	}()
	return metrics.NewLittDBMetrics(), serving, nil
}

func (s *metricsServer) close() error {
	s.cancel()
	<-s.done
	return s.err
}
