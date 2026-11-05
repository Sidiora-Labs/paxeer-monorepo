package rpctest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmnet "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/net"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/service"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/node"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"go.opentelemetry.io/otel/sdk/trace"
)

// Options helps with specifying some parameters for our RPC testing for greater
// control.
type Options struct {
	suppressStdout bool
	preserveRoot   bool
}

// WaitForRPC connects to the RPC service and waits for a decoded status result.
// Cancellation bounds both requests and the delay between connection attempts.
func WaitForRPC(ctx context.Context, conf *config.Config) error {
	client, err := rpchttp.New(conf.RPC.ListenAddress)
	if err != nil {
		return err
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := client.Status(ctx); err == nil {
			return nil
		} else if errors.Is(err, rpchttp.ErrInvalidStatusResponse) {
			return err
		}
		timer := time.NewTimer(time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func randPort() int {
	port, err := tmnet.GetFreePort()
	if err != nil {
		panic(err)
	}
	return port
}

// makeAddrs constructs local listener addresses for node services.  This
// implementation uses random ports so test instances can run concurrently.
func makeAddrs() (p2pAddr, rpcAddr string) {
	const addrTemplate = "tcp://127.0.0.1:%d"
	return fmt.Sprintf(addrTemplate, randPort()), fmt.Sprintf(addrTemplate, randPort())
}

func CreateConfig(t *testing.T, testName string) (*config.Config, error) {
	c, err := config.ResetTestRoot(t.TempDir(), testName)
	if err != nil {
		return nil, err
	}

	p2pAddr, rpcAddr := makeAddrs()
	c.P2P.ListenAddress = p2pAddr
	c.RPC.ListenAddress = rpcAddr
	c.RPC.EventLogWindowSize = 5 * time.Minute
	c.Consensus.WalPath = "rpc-test"
	c.RPC.CORSAllowedOrigins = []string{"https://tendermint.com/"}
	return c, nil
}

type ServiceCloser func(context.Context) error

func StartTendermint(
	ctx context.Context,
	conf *config.Config,
	app abci.Application,
	opts ...func(*Options),
) (service.Service, ServiceCloser, error) {
	ctx, cancel := context.WithCancel(ctx)

	if kvApp, ok := app.(*kvstore.Application); ok {
		genDoc, err := types.GenesisDocFromFile(conf.GenesisFile())
		if err != nil {
			return nil, func(_ context.Context) error { cancel(); return nil }, fmt.Errorf("types.GenesisDocFromFile(%q): %w", conf.GenesisFile(), err)
		}
		kvApp.SetValidators(genDoc.ValidatorUpdates())
	}

	nodeOpts := &Options{}
	for _, opt := range opts {
		opt(nodeOpts)
	}
	tmNode, err := node.New(
		ctx,
		conf,
		func() {},
		app,
		nil,
		[]trace.TracerProviderOption{},
		node.NoOpMetricsProvider(),
		types.DefaultConsensusPolicy(),
	)
	if err != nil {
		return nil, func(_ context.Context) error { cancel(); return nil }, fmt.Errorf("node.New(%q): %w", conf.Mode, err)
	}

	err = tmNode.Start(ctx)
	if err != nil {
		return nil, func(_ context.Context) error { cancel(); return nil }, err
	}

	if err := WaitForRPC(ctx, conf); err != nil {
		cancel()
		tmNode.Wait()
		return nil, func(_ context.Context) error { return nil }, err
	}

	if !nodeOpts.suppressStdout {
		fmt.Println("Tendermint running!")
	}

	return tmNode, func(ctx context.Context) error {
		cancel()
		tmNode.Wait()
		if !nodeOpts.preserveRoot {
			return os.RemoveAll(conf.RootDir)
		}
		return nil
	}, nil
}

// SuppressStdout is an option that tries to make sure the RPC test Tendermint
// node doesn't log anything to stdout.
func SuppressStdout(o *Options) {
	o.suppressStdout = true
}

// PreserveRoot keeps node data after shutdown for tests that reopen the node.
// The caller owns removal of the preserved directory.
func PreserveRoot(o *Options) {
	o.preserveRoot = true
}
