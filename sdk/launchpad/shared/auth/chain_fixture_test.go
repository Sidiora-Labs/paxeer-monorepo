package auth

import (
	"context"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
)

func authChainClient(t *testing.T) *ethclient.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	endpoint := os.Getenv("L34_ANVIL_URL")
	if endpoint == "" {
		directory := t.TempDir()
		endpoint = filepath.Join(directory, "chain.ipc")
		output, err := os.Create(filepath.Join(directory, "chain.log"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = output.Close() })
		command := exec.Command("anvil", "--host", "127.0.0.1", "--port", "0", "--ipc", endpoint, "--chain-id", "125", "--accounts", "0", "--threads", "2", "--silent")
		command.Stdout, command.Stderr = output, output
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- command.Wait() }()
		t.Cleanup(func() {
			_ = command.Process.Signal(os.Interrupt)
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				_ = command.Process.Kill()
				<-done
			}
		})
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			if _, err := os.Stat(endpoint); err == nil {
				break
			}
			select {
			case err := <-done:
				done <- err
				t.Fatal("authorization chain exited", err)
			case <-ctx.Done():
				t.Fatal("authorization chain unavailable", ctx.Err())
			case <-ticker.C:
			}
		}
	}
	client, err := ethclient.DialContext(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	chain, err := client.ChainID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if chain.Cmp(big.NewInt(125)) != 0 {
		t.Fatalf("authorization chain ID = %s, want 125", chain)
	}
	return client
}
