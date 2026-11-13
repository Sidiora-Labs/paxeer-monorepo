package readpolicy_test

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/chain/readpolicy"
	"github.com/ethereum/go-ethereum/ethclient"
)

type localChainHead struct{ client *ethclient.Client }

func (h localChainHead) Head(ctx context.Context) (int64, error) {
	height, err := h.client.BlockNumber(ctx)
	return int64(height), err
}

func TestHeadWorkerDeliversCommittedChainObservation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("anvil", "--host", "127.0.0.1", "--port", strconv.Itoa(port), "--chain-id", "125", "--silent")
	command.Stdout, command.Stderr = io.Discard, io.Discard
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Signal(os.Interrupt)
		_ = command.Wait()
	})
	address := "http://127.0.0.1:" + strconv.Itoa(port)
	var client *ethclient.Client
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		client, err = ethclient.Dial(address)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			_, err = client.BlockNumber(ctx)
			cancel()
			if err == nil {
				break
			}
			client.Close()
		}
	}
	if err != nil {
		t.Fatalf("local chain did not serve committed heights: %v", err)
	}
	t.Cleanup(client.Close)
	var worker readpolicy.HeadWorker
	if err := worker.Run(context.Background(), nil, func(context.Context, readpolicy.HeadObservation) error { return nil }); err == nil {
		t.Fatal("worker admitted a missing chain reader")
	}

	denied := errors.New("durable head delivery rejected")
	if err := worker.Run(context.Background(), localChainHead{client}, func(context.Context, readpolicy.HeadObservation) error { return denied }); !errors.Is(err, denied) {
		t.Fatalf("head delivery error: %v", err)
	}
	cancelled, cancelBefore := context.WithCancel(context.Background())
	cancelBefore()
	if err := worker.Run(cancelled, localChainHead{client}, func(context.Context, readpolicy.HeadObservation) error {
		t.Fatal("cancelled worker delivered a head")
		return nil
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled worker: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var observed int
	err = worker.Run(ctx, localChainHead{client}, func(ctx context.Context, head readpolicy.HeadObservation) error {
		if head.Err != nil || head.Height < 0 || head.ObservedAt.IsZero() || head.Latency < 0 {
			t.Fatalf("chain head observation: %+v", head)
		}
		observed++
		if err := worker.Run(ctx, localChainHead{client}, func(context.Context, readpolicy.HeadObservation) error { return nil }); err == nil {
			t.Fatal("running head worker admitted a second loop")
		}
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) || observed != 1 {
		t.Fatalf("head worker delivered %d observations: %v", observed, err)
	}
}

func TestHeadWorkerReportsUnavailableEndpoint(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := "http://" + listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	client, err := ethclient.Dial(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var worker readpolicy.HeadWorker
	observed := 0
	err = worker.Run(ctx, localChainHead{client}, func(_ context.Context, head readpolicy.HeadObservation) error {
		if head.Err == nil {
			t.Fatal("unavailable endpoint returned a committed head")
		}
		observed++
		if observed == 2 {
			cancel()
		}
		return nil
	})
	if !errors.Is(err, context.Canceled) || observed != 2 {
		t.Fatalf("unavailable head delivery: count=%d error=%v", observed, err)
	}
}
