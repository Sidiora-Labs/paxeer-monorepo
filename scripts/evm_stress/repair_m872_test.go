package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

type m872RPCFixture struct {
	mu               sync.Mutex
	transactions     []*types.Transaction
	refusals         map[common.Address]string
	hangSends        bool
	hangFirstBalance bool
	zeroBalance      bool
	balanceCalls     int
	started          chan string
	canceled         chan string
	release          chan struct{}
}

func (f *m872RPCFixture) wait(ctx context.Context, method string) error {
	select {
	case <-ctx.Done():
		f.canceled <- method
		return ctx.Err()
	case <-f.release:
		return errors.New("fixture closed")
	}
}

func (f *m872RPCFixture) SendRawTransaction(ctx context.Context, raw hexutil.Bytes) (common.Hash, error) {
	var tx types.Transaction
	if err := tx.UnmarshalBinary(raw); err != nil {
		return common.Hash{}, err
	}
	sender, err := types.Sender(signer, &tx)
	if err != nil {
		return common.Hash{}, err
	}
	f.mu.Lock()
	f.transactions = append(f.transactions, &tx)
	f.mu.Unlock()
	f.started <- "send"
	if f.hangSends {
		return common.Hash{}, f.wait(ctx, "send")
	}
	if reason := f.refusals[sender]; reason != "" {
		return common.Hash{}, errors.New(reason)
	}
	return tx.Hash(), nil
}

func (f *m872RPCFixture) GetBalance(ctx context.Context, _ common.Address, block string) (*hexutil.Big, error) {
	if block != "latest" {
		return nil, fmt.Errorf("unexpected block %q", block)
	}
	f.mu.Lock()
	f.balanceCalls++
	call := f.balanceCalls
	f.mu.Unlock()
	f.started <- "balance"
	if f.hangFirstBalance && call == 1 {
		return nil, f.wait(ctx, "balance")
	}
	balance := big.NewInt(1)
	if f.zeroBalance {
		balance.SetInt64(0)
	}
	return (*hexutil.Big)(balance), nil
}

func m872Client(t *testing.T, fixture *m872RPCFixture) *ethclient.Client {
	t.Helper()
	fixture.started = make(chan string, 32)
	fixture.canceled = make(chan string, 32)
	fixture.release = make(chan struct{})
	server := rpc.NewServer()
	if err := server.RegisterName("eth", fixture); err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server)
	client, err := ethclient.DialContext(context.Background(), httpServer.URL)
	if err != nil {
		httpServer.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		close(fixture.release)
		client.Close()
		httpServer.Close()
		server.Stop()
	})
	return client
}

func m872Queue(n int) (<-chan *ecdsa.PrivateKey, <-chan time.Time) {
	keys := make(chan *ecdsa.PrivateKey, n)
	ticks := make(chan time.Time, n)
	for i := 1; i <= n; i++ {
		keys <- nextKey(uint64(i))
		ticks <- time.Now()
	}
	close(keys)
	return keys, ticks
}

func m872Event(t *testing.T, events <-chan string, want string) {
	t.Helper()
	select {
	case got := <-events:
		if got != want {
			t.Fatalf("RPC event = %q, want %q", got, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("RPC did not emit %q within the bound", want)
	}
}

func TestRepairM_872_F005_SubmissionAccountingAndReasons(t *testing.T) {
	recipient := keyAddr(nextKey(0))
	for _, mode := range []string{workloadModeTransfer, workloadModeContractStorage} {
		for _, refused := range []int{0, 2, 4} {
			t.Run(fmt.Sprintf("%s/refused_%d", mode, refused), func(t *testing.T) {
				fixture := &m872RPCFixture{refusals: make(map[common.Address]string)}
				for i := 1; i <= refused; i++ {
					fixture.refusals[keyAddr(nextKey(uint64(i)))] = fmt.Sprintf("LXP_SUBMISSION_REFUSED_%d", i)
				}
				client := m872Client(t, fixture)
				makeTx := func(key *ecdsa.PrivateKey) *types.Transaction {
					if mode == workloadModeContractStorage {
						return deployStorageContract(0, key)
					}
					return transfer(0, recipient, key)
				}
				keys, ticks := m872Queue(4)
				var output bytes.Buffer
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				counts := runStress(ctx, client, keys, ticks, 3, time.Second, makeTx, &output)
				want := submissionCounts{attempted: 4, accepted: uint64(4 - refused), refused: uint64(refused)}
				if counts != want {
					t.Fatalf("counts = %+v, want %+v; output: %s", counts, want, output.String())
				}
				status := "completed"
				if refused > 0 {
					status = "degraded"
				}
				summary := fmt.Sprintf("run %s: attempted=4 accepted=%d refused=%d\n", status, 4-refused, refused)
				if !strings.HasSuffix(output.String(), summary) {
					t.Fatalf("missing exact summary %q in %q", summary, output.String())
				}
				fixture.mu.Lock()
				txs := append([]*types.Transaction(nil), fixture.transactions...)
				fixture.mu.Unlock()
				if len(txs) != 4 {
					t.Fatalf("RPC received %d transactions, want 4", len(txs))
				}
				seen := make(map[common.Address]bool)
				for _, tx := range txs {
					sender, err := types.Sender(signer, tx)
					if err != nil || seen[sender] || tx.Nonce() != 0 || tx.ChainId().Cmp(bigChainID) != 0 {
						t.Fatalf("invalid or repeated signed transaction: sender=%s err=%v", sender, err)
					}
					seen[sender] = true
					if mode == workloadModeTransfer {
						if tx.To() == nil || *tx.To() != recipient || tx.Gas() != 21_000 || tx.Value().Cmp(txValue) != 0 {
							t.Fatal("transport changed the transfer workload")
						}
					} else if tx.To() != nil || !bytes.Equal(tx.Data(), storageContractInitCode) || tx.Gas() != 200_000 {
						t.Fatal("transport changed the contract-storage workload")
					}
					if reason := fixture.refusals[sender]; reason != "" {
						line := fmt.Sprintf("submission refused tx=%s: %s\n", tx.Hash().Hex(), reason)
						if strings.Count(output.String(), line) != 1 {
							t.Fatalf("missing unique refusal reason and transaction: %q in %q", line, output.String())
						}
					}
				}
				for i := uint64(1); i <= 4; i++ {
					if !seen[keyAddr(nextKey(i))] {
						t.Fatalf("RPC never consumed sender %d", i)
					}
				}
				if strings.Count(output.String(), "submission refused tx=") != refused {
					t.Fatalf("wrong number of refusal reports: %q", output.String())
				}
			})
		}
	}
}

func TestRepairM_872_F005_SendTimeoutReleasesWorkers(t *testing.T) {
	fixture := &m872RPCFixture{hangSends: true}
	client := m872Client(t, fixture)
	keys, ticks := m872Queue(3)
	var output bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	counts := runStress(ctx, client, keys, ticks, 2, 250*time.Millisecond,
		func(key *ecdsa.PrivateKey) *types.Transaction { return deployStorageContract(0, key) }, &output)
	if counts != (submissionCounts{attempted: 3, refused: 3}) || ctx.Err() != nil {
		t.Fatalf("per-call timeouts did not release workers: counts=%+v context=%v output=%s", counts, ctx.Err(), output.String())
	}
	if strings.Count(output.String(), context.DeadlineExceeded.Error()) != 3 ||
		!strings.HasSuffix(output.String(), "run degraded: attempted=3 accepted=0 refused=3\n") {
		t.Fatalf("timeouts were not reported as refused submissions: %s", output.String())
	}
	for range 3 {
		m872Event(t, fixture.started, "send")
		m872Event(t, fixture.canceled, "send")
	}
}

func TestRepairM_872_F005_ParentCancellation(t *testing.T) {
	t.Run("in_flight_send", func(t *testing.T) {
		fixture := &m872RPCFixture{hangSends: true}
		client := m872Client(t, fixture)
		keys, ticks := m872Queue(2)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var output bytes.Buffer
		done := make(chan submissionCounts, 1)
		go func() {
			done <- runStress(ctx, client, keys, ticks, 1, rpcTimeout,
				func(key *ecdsa.PrivateKey) *types.Transaction { return transfer(0, keyAddr(nextKey(0)), key) }, &output)
		}()
		m872Event(t, fixture.started, "send")
		cancel()
		select {
		case counts := <-done:
			if counts != (submissionCounts{attempted: 1, refused: 1}) ||
				!strings.Contains(output.String(), context.Canceled.Error()) ||
				!strings.HasSuffix(output.String(), "run canceled: attempted=1 accepted=0 refused=1\n") {
				t.Fatalf("canceled run misreported: %+v %s", counts, output.String())
			}
		case <-time.After(3 * time.Second):
			t.Fatal("parent cancellation did not interrupt the in-flight RPC")
		}
		m872Event(t, fixture.canceled, "send")
	})
	t.Run("rate_limiter", func(t *testing.T) {
		fixture := &m872RPCFixture{}
		client := m872Client(t, fixture)
		keys, _ := m872Queue(1)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan submissionCounts, 1)
		var output bytes.Buffer
		go func() {
			done <- runStress(ctx, client, keys, make(chan time.Time), 1, rpcTimeout,
				func(key *ecdsa.PrivateKey) *types.Transaction { return deployStorageContract(0, key) }, &output)
		}()
		cancel()
		select {
		case counts := <-done:
			if counts != (submissionCounts{}) || output.String() != "run canceled: attempted=0 accepted=0 refused=0\n" {
				t.Fatalf("unsent work was counted as attempted: %+v %s", counts, output.String())
			}
		case <-time.After(3 * time.Second):
			t.Fatal("rate limiter ignored cancellation")
		}
	})
}

func TestRepairM_872_F005_BalanceRPCTimeoutAndRecovery(t *testing.T) {
	fixture := &m872RPCFixture{hangFirstBalance: true}
	client := m872Client(t, fixture)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := waitForBalance(ctx, client, keyAddr(nextKey(1)), 250*time.Millisecond); err != nil {
		t.Fatalf("balance retry did not recover after a bounded RPC: %v", err)
	}
	fixture.mu.Lock()
	calls := fixture.balanceCalls
	fixture.mu.Unlock()
	if calls != 2 {
		t.Fatalf("balance calls = %d, want one timeout then one successful retry", calls)
	}
	m872Event(t, fixture.canceled, "balance")
}

func TestRepairM_872_F005_BalanceCancellation(t *testing.T) {
	for _, hang := range []bool{false, true} {
		t.Run(fmt.Sprintf("in_flight_%t", hang), func(t *testing.T) {
			fixture := &m872RPCFixture{hangFirstBalance: hang, zeroBalance: true}
			client := m872Client(t, fixture)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- waitForBalance(ctx, client, keyAddr(nextKey(1)), rpcTimeout) }()
			m872Event(t, fixture.started, "balance")
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("balance cancellation = %v, want context.Canceled", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("balance wait ignored cancellation")
			}
			if hang {
				m872Event(t, fixture.canceled, "balance")
			}
		})
	}
}
