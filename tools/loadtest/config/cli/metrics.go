package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
)

// MetricsServer owns a private registry, mux, listener and exporter. Construction
// binds synchronously so startup failure is returned before generating traffic.
type MetricsServer struct {
	listener net.Listener
	server   *http.Server
	provider *metric.MeterProvider
	once     sync.Once
	closeErr error
}

func NewMetricsServer(listenAddr string) (*MetricsServer, error) {
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("bind metrics %s: %w", listenAddr, err)
	}
	registry := promclient.NewRegistry()
	exporter, err := prometheus.New(prometheus.WithNamespace("paxload"), prometheus.WithRegisterer(registry))
	if err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("create metrics exporter: %w", err)
	}
	provider := metric.NewMeterProvider(metric.WithReader(exporter))
	otel.SetMeterProvider(provider)
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	return &MetricsServer{listener: listener, provider: provider, server: &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}}, nil
}

func (s *MetricsServer) Addr() string { return s.listener.Addr().String() }

// Serve belongs to the service scope; unexpected serve and shutdown failures
// reach the caller. Closing the run context releases the listener before return.
func (s *MetricsServer) Serve(ctx context.Context) error {
	stopped := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { _ = s.Close(); close(stopped) })
	err := s.server.Serve(s.listener)
	if !stop() {
		<-stopped
	}
	closeErr := s.Close()
	if errors.Is(err, http.ErrServerClosed) && ctx.Err() != nil {
		err = nil
	}
	return errors.Join(err, closeErr)
}

func (s *MetricsServer) Close() error {
	s.once.Do(func() {
		serverErr := s.server.Close()
		listenerErr := s.listener.Close()
		if errors.Is(listenerErr, net.ErrClosed) {
			listenerErr = nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.closeErr = errors.Join(serverErr, listenerErr, s.provider.Shutdown(ctx))
	})
	return s.closeErr
}
