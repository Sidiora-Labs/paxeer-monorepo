package util

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"sync"
	"time"
)

type PprofProfiler struct {
	logger   *slog.Logger
	address  string
	token    string
	mu       sync.Mutex
	server   *http.Server
	listener net.Listener
}

// PprofConfig requires bearer authentication for any non-loopback listener.
type PprofConfig struct {
	Address     string
	BearerToken string
}

func NewPprofProfiler(httpPort string, logger *slog.Logger) *PprofProfiler {
	return &PprofProfiler{logger: logger.With("component", "PprofProfiler"), address: net.JoinHostPort("127.0.0.1", httpPort)}
}

func NewPprofProfilerWithConfig(config PprofConfig, logger *slog.Logger) (*PprofProfiler, error) {
	host, _, err := net.SplitHostPort(config.Address)
	if err != nil {
		return nil, fmt.Errorf("invalid pprof address: %w", err)
	}
	ip := net.ParseIP(host)
	if (ip == nil || !ip.IsLoopback()) && len(config.BearerToken) < 32 {
		return nil, fmt.Errorf("non-loopback pprof requires a bearer token of at least 32 bytes")
	}
	return &PprofProfiler{logger: logger.With("component", "PprofProfiler"), address: config.Address, token: config.BearerToken}, nil
}

// Start preserves the original blocking API. Context-aware callers use StartContext.
func (p *PprofProfiler) Start() {
	if err := p.StartContext(context.Background()); err != nil {
		p.logger.Error("pprof server failed", "error", err)
	}
}

func (p *PprofProfiler) StartContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	var handler http.Handler = mux
	if p.token != "" {
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+p.token)) != 1 {
				w.Header().Set("WWW-Authenticate", "Bearer")
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			mux.ServeHTTP(w, r)
		})
	}
	p.mu.Lock()
	if p.server != nil {
		p.mu.Unlock()
		return fmt.Errorf("pprof already started")
	}
	listener, err := net.Listen("tcp", p.address)
	if err != nil {
		p.mu.Unlock()
		return err
	}
	server := &http.Server{Addr: listener.Addr().String(), Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	p.server, p.listener = server, listener
	p.mu.Unlock()
	done, watcherDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(watcherDone)
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := p.Shutdown(shutdownCtx); err != nil {
				p.logger.Error("pprof shutdown failed", "error", err)
			}
		case <-done:
		}
	}()
	err = server.Serve(listener)
	close(done)
	<-watcherDone
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (p *PprofProfiler) Addr() net.Addr {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.listener == nil {
		return nil
	}
	return p.listener.Addr()
}

func (p *PprofProfiler) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	server := p.server
	p.mu.Unlock()
	if server == nil {
		return nil
	}
	if err := server.Shutdown(ctx); err != nil {
		_ = server.Close()
		return err
	}
	return nil
}
