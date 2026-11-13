package internaltest

import (
	"context"
	"math/big"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/chainread"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/node"
)

// NewChain starts an owned chain 125 RPC and returns its first committed block.
func NewChain(t *testing.T) (*ethclient.Client, chainread.Block) {
	return NewChainAt(t, 0)
}

// NewChainAt starts a chain whose genesis uses the supplied timestamp.
func NewChainAt(t *testing.T, timestamp uint64) (*ethclient.Client, chainread.Block) {
	return NewChainSpanAt(t, timestamp, 120*time.Second)
}

// NewChainSpanAt commits two blocks separated by the requested duration.
func NewChainSpanAt(t *testing.T, timestamp uint64, span time.Duration) (*ethclient.Client, chainread.Block) {
	rpc, block, _ := NewChainSpanAtWithURL(t, timestamp, span)
	return rpc, block
}

// NewChainSpanAtWithURL also returns the owned chain's HTTP endpoint.
func NewChainSpanAtWithURL(t *testing.T, timestamp uint64, span time.Duration) (*ethclient.Client, chainread.Block, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	backend := simulated.NewBackend(types.GenesisAlloc{}, func(n *node.Config, eth *ethconfig.Config) {
		chain := *eth.Genesis.Config
		chain.ChainID = big.NewInt(125)
		eth.Genesis.Config = &chain
		if timestamp > 0 {
			eth.Genesis.Timestamp = timestamp
		}
		n.HTTPHost = net.IPv4(127, 0, 0, 1).String()
		n.HTTPPort = port
		n.HTTPModules = []string{"eth"}
	})
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Error(err)
		}
	})
	if timestamp > 0 {
		if err = backend.AdjustTime(12 * time.Second); err != nil {
			t.Fatal(err)
		}
		if err = backend.AdjustTime(span); err != nil {
			t.Fatal(err)
		}
	} else {
		backend.Commit()
		backend.Commit()
	}
	endpoint := "http://127.0.0.1:" + strconv.Itoa(port)
	rpc, err := ethclient.Dial(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(rpc.Close)
	block, err := (chainread.Reader{RPC: rpc}).Block(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	return rpc, block, endpoint
}
