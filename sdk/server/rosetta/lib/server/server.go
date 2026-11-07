package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	assert "github.com/coinbase/rosetta-sdk-go/asserter"
	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/rosetta/lib/internal/service"
	crgtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/rosetta/lib/types"
)

const DefaultRetries = 5
const DefaultRetryWait = 5 * time.Second

// Settings define the rosetta server settings
type Settings struct {
	// Network contains the information regarding the network
	Network *types.NetworkIdentifier
	// Client is the online API handler
	Client crgtypes.Client
	// Listen is the address the handler will listen at
	Listen string
	// Offline defines if the rosetta service should be exposed in offline mode
	Offline bool
	// Retries is the number of readiness checks that will be attempted when instantiating the handler
	// valid only for online API
	Retries int
	// RetryWait is the time that will be waited between retries
	RetryWait time.Duration
}

type Server struct {
	lifecycle *serverLifecycle
}

type serverLifecycle struct {
	http        *http.Server
	client      io.Closer
	shutdown    sync.Once
	shutdownErr error
}

func (h Server) Start() error {
	if h.lifecycle == nil {
		return fmt.Errorf("rosetta server is not initialized")
	}
	if err := h.lifecycle.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return errors.Join(err, h.Shutdown(context.Background()))
	}
	return nil
}

// Shutdown drains HTTP requests before closing the client connection, once across
// all copies of Server. If draining expires, remaining connections are closed.
func (h Server) Shutdown(ctx context.Context) error {
	if h.lifecycle == nil {
		return nil
	}
	lifecycle := h.lifecycle
	lifecycle.shutdown.Do(func() {
		lifecycle.shutdownErr = lifecycle.http.Shutdown(ctx)
		if lifecycle.shutdownErr != nil {
			lifecycle.shutdownErr = errors.Join(lifecycle.shutdownErr, lifecycle.http.Close())
		}
		if lifecycle.client != nil {
			lifecycle.shutdownErr = errors.Join(lifecycle.shutdownErr, lifecycle.client.Close())
		}
	})
	return lifecycle.shutdownErr
}

// NewServer takes ownership of clients that implement io.Closer, including on
// initialization failure. Clients must retain and close their own connections.
func NewServer(settings Settings) (result Server, err error) {
	if settings.Client == nil {
		return Server{}, fmt.Errorf("client is nil")
	}
	client, _ := settings.Client.(io.Closer)
	defer func() {
		if err != nil && client != nil {
			err = errors.Join(err, client.Close())
		}
	}()
	asserter, err := assert.NewServer(
		settings.Client.SupportedOperations(),
		true,
		[]*types.NetworkIdentifier{settings.Network},
		nil,
		false,
		"",
	)
	if err != nil {
		return Server{}, fmt.Errorf("cannot build asserter: %w", err)
	}

	var (
		adapter crgtypes.API
	)
	switch settings.Offline {
	case true:
		adapter, err = newOfflineAdapter(settings)
	case false:
		adapter, err = newOnlineAdapter(settings)
	}
	if err != nil {
		return Server{}, err
	}
	h := server.NewRouter(
		server.NewAccountAPIController(adapter, asserter),
		server.NewBlockAPIController(adapter, asserter),
		server.NewNetworkAPIController(adapter, asserter),
		server.NewMempoolAPIController(adapter, asserter),
		server.NewConstructionAPIController(adapter, asserter),
	)

	return Server{
		lifecycle: &serverLifecycle{
			http: &http.Server{
				Addr:              settings.Listen,
				Handler:           h,
				ReadHeaderTimeout: 10 * time.Second,
			},
			client: client,
		},
	}, nil
}

func newOfflineAdapter(settings Settings) (crgtypes.API, error) {
	if settings.Client == nil {
		return nil, fmt.Errorf("client is nil")
	}
	return service.NewOffline(settings.Network, settings.Client)
}

func newOnlineAdapter(settings Settings) (crgtypes.API, error) {
	if settings.Client == nil {
		return nil, fmt.Errorf("client is nil")
	}
	if settings.Retries <= 0 {
		settings.Retries = DefaultRetries
	}
	if settings.RetryWait == 0 {
		settings.RetryWait = DefaultRetryWait
	}

	var err error
	err = settings.Client.Bootstrap()
	if err != nil {
		return nil, err
	}

	for i := 0; i < settings.Retries; i++ {
		err = settings.Client.Ready()
		if err != nil {
			time.Sleep(settings.RetryWait)
			continue
		}
		return service.NewOnlineNetwork(settings.Network, settings.Client)
	}
	return nil, fmt.Errorf("maximum number of retries exceeded, last error: %w", err)
}
