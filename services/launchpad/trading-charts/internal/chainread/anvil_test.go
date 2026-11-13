package chainread

import (
	"context"
	"errors"
	"io"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestPinnedBlockAndCallOnLocalChain(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "anvil", "--host", "127.0.0.1", "--port", strconv.Itoa(port), "--silent")
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		if err := command.Wait(); err != nil && ctx.Err() == nil {
			t.Error(err)
		}
	})
	url := "http://127.0.0.1:" + strconv.Itoa(port)
	var client *ethclient.Client
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		client, err = ethclient.DialContext(ctx, url)
		if err == nil {
			if _, err = client.BlockNumber(ctx); err == nil {
				break
			}
			client.Close()
		}
		time.Sleep(25 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("local chain readiness: %v", err)
	}
	defer client.Close()
	reader := Reader{RPC: client}
	block, err := reader.Block(ctx, 0)
	if err != nil || block.Height != 0 || !strings.HasPrefix(block.Hash, "0x") || block.Time <= 0 {
		t.Fatalf("genesis block: %+v, %v", block, err)
	}
	address := common.Address{}
	values, err := reader.CallAt(ctx, 0, block.Hash, []ethereum.CallMsg{{To: &address}})
	if err != nil || len(values) != 1 || len(values[0]) != 0 {
		t.Fatalf("pinned empty account call: %x, %v", values, err)
	}
	if _, err := reader.CallAt(ctx, 0, "0x"+strings.Repeat("f", 64), []ethereum.CallMsg{{To: &address}}); !errors.Is(err, ErrConflict) {
		t.Fatalf("wrong block hash: %v", err)
	}
	if _, err := reader.CallAt(ctx, 0, block.Hash, nil); err == nil {
		t.Fatal("empty pinned call was accepted")
	}
	if _, err := reader.Block(ctx, -1); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("negative block height: %v", err)
	}
	if _, err := reader.Block(ctx, 1<<30); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("uncommitted block height: %v", err)
	}
	cancelled, stop := context.WithCancel(ctx)
	stop()
	if _, err := reader.Block(cancelled, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled block read: %v", err)
	}
	if _, err := reader.CallAt(cancelled, 0, block.Hash, []ethereum.CallMsg{{To: &address}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled pinned read: %v", err)
	}
}
