package internaltest

import (
	"context"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type Anvil struct {
	URL    string
	client *ethclient.Client
	t      *testing.T
}

func NewAnvil(t *testing.T) *Anvil {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	logs, err := os.Create(filepath.Join(t.TempDir(), "anvil.log"))
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, "anvil", "--host", "127.0.0.1", "--port", "0", "--chain-id", "125", "--no-mining", "--accounts", "0")
	cmd.Stdout, cmd.Stderr = logs, logs
	if err = cmd.Start(); err != nil {
		cancel()
		_ = logs.Close()
		t.Fatal(err)
	}
	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(exited) }()
	t.Cleanup(func() { cancel(); <-exited; _ = logs.Close() })
	var client *ethclient.Client
	endpoint := ""
	listening := regexp.MustCompile(`Listening on (127\.0\.0\.1:[0-9]+)`)
	ready, done := context.WithTimeout(ctx, 10*time.Second)
	defer done()
	for {
		if client == nil {
			raw, err := os.ReadFile(logs.Name())
			if err != nil {
				t.Fatal(err)
			}
			if address := listening.FindSubmatch(raw); len(address) == 2 {
				endpoint = "http://" + string(address[1])
				client, err = ethclient.DialContext(ready, endpoint)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(client.Close)
			}
		}
		if client != nil {
			id, err := client.ChainID(ready)
			if err != nil {
				t.Fatal(err)
			}
			if id.Cmp(big.NewInt(125)) != 0 {
				t.Fatal("chain id", id)
			}
			return &Anvil{URL: endpoint, client: client, t: t}
		}
		select {
		case <-exited:
			t.Fatal("chain exited before readiness")
		case <-ready.Done():
			t.Fatal("chain readiness", ready.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func (a *Anvil) Client() *ethclient.Client { return a.client }

func (a *Anvil) Commit() common.Hash {
	a.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.client.Client().CallContext(ctx, nil, "evm_mine"); err != nil {
		a.t.Fatal(err)
	}
	head, err := a.client.HeaderByNumber(ctx, nil)
	if err != nil {
		a.t.Fatal(err)
	}
	return head.Hash()
}

func (a *Anvil) AdjustTime(delta time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.client.Client().CallContext(ctx, nil, "evm_increaseTime", int64(delta/time.Second)); err != nil {
		return err
	}
	a.Commit()
	return nil
}
