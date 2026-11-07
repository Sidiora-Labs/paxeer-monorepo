package bringup

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/eth/catalyst"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
)

// The sender uses the native signing codec, the real value-loop process and
// journal, HTTP JSON-RPC, and geth's actual transaction pool and state engine.
// The proxy only withholds a real acceptance response to expose the crash gap.
type repair143Chain struct {
	backend   *eth.Ethereum
	beacon    *catalyst.SimulatedBeacon
	client    *ethclient.Client
	url       string
	key       *ecdsa.PrivateKey
	sender    common.Address
	recipient common.Address
	chainID   *big.Int
}

func repair143NewChain(t *testing.T) *repair143Chain {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	sender := crypto.PubkeyToAddress(key.PublicKey)
	genesis := core.DeveloperGenesisBlock(10_000_000, &sender)
	stack, err := node.New(&node.Config{
		DataDir: t.TempDir(), HTTPHost: "127.0.0.1", HTTPPort: 0,
		HTTPModules: []string{"eth"}, HTTPVirtualHosts: []string{"localhost"},
		P2P: p2p.Config{NoDiscovery: true, MaxPeers: 0, ListenAddr: "127.0.0.1:0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := stack.Close(); err != nil {
			t.Error(err)
		}
	})
	config := ethconfig.Defaults
	config.Genesis = genesis
	config.SyncMode = ethconfig.FullSync
	config.TrieCleanCache, config.TrieDirtyCache, config.SnapshotCache = 16, 16, 16
	config.TxPool.Journal = ""
	config.BlobPool.Datadir = ""
	backend, err := eth.New(stack, &config)
	if err != nil {
		t.Fatal(err)
	}
	beacon, err := catalyst.NewSimulatedBeacon(0, common.Address{}, backend)
	if err != nil {
		t.Fatal(err)
	}
	stack.RegisterLifecycle(beacon)
	if err := stack.Start(); err != nil {
		t.Fatal(err)
	}
	backend.SetSynced()
	// At genesis geth has not started indexing transaction history yet. Advance
	// the real chain and wait for its index before asking whether a hash exists.
	beacon.Commit()
	indexDeadline := time.Now().Add(10 * time.Second)
	for {
		progress, err := backend.BlockChain().TxIndexProgress()
		if err != nil {
			t.Fatal(err)
		}
		if progress.Done() {
			break
		}
		if time.Now().After(indexDeadline) {
			t.Fatalf("transaction index not ready: %+v", progress)
		}
		time.Sleep(10 * time.Millisecond)
	}
	client, err := ethclient.Dial(stack.HTTPEndpoint())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	return &repair143Chain{backend: backend, beacon: beacon, client: client,
		url: stack.HTTPEndpoint(), key: key, sender: sender,
		recipient: common.HexToAddress("0x3210000000000000000000000000000000000123"),
		chainID:   genesis.Config.ChainID}
}

func repair143Signed(t *testing.T, key *ecdsa.PrivateKey, chainID *big.Int, nonce uint64, recipient common.Address) (*types.Transaction, []byte) {
	t.Helper()
	transaction, err := types.SignTx(types.NewTx(&types.DynamicFeeTx{
		ChainID: chainID, Nonce: nonce, GasTipCap: big.NewInt(2_000_000_000),
		GasFeeCap: big.NewInt(5_000_000_000), Gas: 21000, To: &recipient, Value: big.NewInt(1),
	}), types.LatestSignerForChainID(chainID), key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := transaction.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	return transaction, raw
}

func repair143Write(t *testing.T, state, name string, data []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(state, name), data, 0600); err != nil {
		t.Fatal(err)
	}
}

func repair143Command(t *testing.T, ctx context.Context, state, endpoint, chainID, operation string, args ...string) *exec.Cmd {
	t.Helper()
	script, err := filepath.Abs("value-loop.sh")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, "bash", "-c", `
source "$1"
state=$2
rpc=$3
network_id=$4
explicit_asset=0
operation=$5
shift 5
if [ "$operation" = transfer ]; then
    send_transfer
else
    send_journal "$operation" "$@"
fi`, "--", script, state, endpoint, chainID, operation)
	command.Args = append(command.Args, args...)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return command
}

func repair143Run(t *testing.T, state, endpoint, chainID, operation string, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	output, err := repair143Command(t, ctx, state, endpoint, chainID, operation, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("value-loop %s: %v\n%s", operation, err, output)
	}
	return output
}

func repair143Journal(t *testing.T, state string, transaction *types.Transaction, raw []byte, status string) []byte {
	t.Helper()
	encoded, err := os.ReadFile(filepath.Join(state, "send.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved struct {
		Transaction string `json:"transaction"`
		Hash        string `json:"hash"`
		Nonce       string `json:"nonce"`
		ChainID     string `json:"chain_id"`
		Status      string `json:"status"`
	}
	if err := json.Unmarshal(encoded, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Transaction != fmt.Sprintf("0x%x", raw) || saved.Hash != transaction.Hash().Hex() ||
		saved.Nonce != fmt.Sprint(transaction.Nonce()) || saved.ChainID != transaction.ChainId().String() || saved.Status != status {
		t.Fatalf("journal lost immutable transaction facts or status: %s", encoded)
	}
	retained, err := os.ReadFile(filepath.Join(state, "send.bin"))
	if err != nil || !bytes.Equal(raw, retained) {
		t.Fatalf("signed bytes changed: %v", err)
	}
	activity, err := os.ReadFile(filepath.Join(state, "send.activity-id"))
	if err != nil || strings.TrimSpace(string(activity)) != transaction.Hash().Hex()[2:] {
		t.Fatalf("compatibility activity identity differs: %q, %v", activity, err)
	}
	info, err := os.Stat(filepath.Join(state, "send.json"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("journal permissions: %v, %v", info, err)
	}
	return encoded
}

func repair143ObservePayment(t *testing.T, chain *repair143Chain, transaction *types.Transaction) {
	t.Helper()
	if err := chain.backend.TxPool().Sync(); err != nil {
		t.Fatal(err)
	}
	chain.beacon.Commit()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	receipt, err := chain.client.TransactionReceipt(ctx, transaction.Hash())
	if err != nil || receipt.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("real transfer receipt: %+v, %v", receipt, err)
	}
	balance, err := chain.client.BalanceAt(ctx, chain.recipient, nil)
	if err != nil || balance.Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("recipient must receive exactly one unit: %v, %v", balance, err)
	}
	nonce, err := chain.client.NonceAt(ctx, chain.sender, nil)
	if err != nil || nonce != 1 {
		t.Fatalf("sender must consume exactly one nonce: %d, %v", nonce, err)
	}
}

func TestRepair_143_CrashAfterAcceptance(t *testing.T) {
	for _, committed := range []bool{false, true} {
		t.Run(fmt.Sprintf("committed=%t", committed), func(t *testing.T) {
			chain := repair143NewChain(t)
			transaction, raw := repair143Signed(t, chain.key, chain.chainID, 0, chain.recipient)
			state := t.TempDir()
			repair143Write(t, state, "send.bin", raw)
			target, err := url.Parse(chain.url)
			if err != nil {
				t.Fatal(err)
			}
			proxy := httputil.NewSingleHostReverseProxy(target)
			accepted, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			var sends atomic.Int32
			proxy.ModifyResponse = func(response *http.Response) error {
				if response.Request.Header.Get("X-Repair143-Send") == "yes" {
					body, err := io.ReadAll(response.Body)
					response.Body.Close()
					if err != nil {
						return err
					}
					response.Body = io.NopCloser(bytes.NewReader(body))
					var answer struct {
						Result string `json:"result"`
					}
					if err := json.Unmarshal(body, &answer); err != nil {
						return err
					}
					if answer.Result != transaction.Hash().Hex() {
						return fmt.Errorf("real node refused: %s", body)
					}
					close(accepted)
					<-release
				}
				return nil
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
					return
				}
				r.Body.Close()
				r.Body = io.NopCloser(bytes.NewReader(body))
				var request struct {
					Method string `json:"method"`
				}
				if err := json.Unmarshal(body, &request); err != nil {
					t.Error(err)
					return
				}
				if request.Method == "eth_sendRawTransaction" {
					sends.Add(1)
					r.Header.Set("X-Repair143-Send", "yes")
				}
				proxy.ServeHTTP(w, r)
			}))
			t.Cleanup(func() {
				releaseOnce.Do(func() { close(release) })
				server.Close()
			})
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			command := repair143Command(t, ctx, state, server.URL, chain.chainID.String(), "transfer")
			var output bytes.Buffer
			command.Stdout, command.Stderr = &output, &output
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL) })
			done := make(chan error, 1)
			go func() { done <- command.Wait() }()
			select {
			case <-accepted:
			case err := <-done:
				t.Fatalf("sender exited before acceptance: %v\n%s", err, output.String())
			case <-ctx.Done():
				t.Fatal("node did not accept the transfer before deadline")
			}
			// Acceptance happened, but no acknowledgement reached the journal.
			repair143Journal(t, state, transaction, raw, "prepared")
			if chain.backend.TxPool().Get(transaction.Hash()) == nil {
				t.Fatal("node has not accepted signed bytes")
			}
			contender, contenderErr := repair143Command(t, ctx, state, server.URL, chain.chainID.String(), "transfer").CombinedOutput()
			if contenderErr == nil || !strings.Contains(string(contender), "holds the journal lock") {
				t.Fatalf("concurrent sender was not excluded: %v\n%s", contenderErr, contender)
			}
			if err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL); err != nil {
				t.Fatal(err)
			}
			if err := <-done; err == nil {
				t.Fatal("terminated sender exited successfully")
			}
			releaseOnce.Do(func() { close(release) })
			if committed {
				repair143ObservePayment(t, chain, transaction)
			}
			// A new process reopens the same on-disk journal. No producer runs.
			for restart := 0; restart < 2; restart++ {
				got := repair143Run(t, state, server.URL, chain.chainID.String(), "transfer")
				if strings.TrimSpace(string(got)) != transaction.Hash().Hex()[2:] {
					t.Fatalf("recovered hash: %s", got)
				}
				repair143Journal(t, state, transaction, raw, "submitted")
			}
			if sends.Load() != 1 {
				t.Fatalf("restart broadcast %d transfers, want 1", sends.Load())
			}
			repair143ObservePayment(t, chain, transaction)
		})
	}
}

func TestRepair_143_RestartBeforeBroadcast(t *testing.T) {
	chain := repair143NewChain(t)
	transaction, raw := repair143Signed(t, chain.key, chain.chainID, 0, chain.recipient)
	state := t.TempDir()
	repair143Write(t, state, "send.bin", raw)
	// This process exits after persisting the producer output, before transport.
	repair143Run(t, state, chain.url, chain.chainID.String(), "prepare", transaction.Hash().Hex()[2:])
	repair143Journal(t, state, transaction, raw, "prepared")
	if chain.backend.TxPool().Get(transaction.Hash()) != nil {
		t.Fatal("prepare broadcast a transaction")
	}
	repair143Run(t, state, chain.url, chain.chainID.String(), "transfer")
	repair143Journal(t, state, transaction, raw, "submitted")
	repair143ObservePayment(t, chain, transaction)
}

func TestRepair_143_RejectChangedOrIncompleteIdentity(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	chainID := big.NewInt(125)
	transaction, raw := repair143Signed(t, key, chainID, 0, common.HexToAddress("0x1234"))
	for _, fault := range []string{"bytes", "nonce", "hash", "status", "activity", "missing-bytes", "truncated", "wrong-chain", "producer-hash"} {
		t.Run(fault, func(t *testing.T) {
			state := t.TempDir()
			repair143Write(t, state, "send.bin", raw)
			repair143Run(t, state, "http://127.0.0.1:1", chainID.String(), "prepare")
			before := repair143Journal(t, state, transaction, raw, "prepared")
			operation := "transfer"
			var arguments []string
			switch fault {
			case "bytes":
				_, other := repair143Signed(t, key, chainID, 1, common.HexToAddress("0x1234"))
				repair143Write(t, state, "send.bin", other)
			case "nonce", "hash", "status":
				var changed map[string]any
				if err := json.Unmarshal(before, &changed); err != nil {
					t.Fatal(err)
				}
				changed[fault] = "corrupt"
				encoded, err := json.Marshal(changed)
				if err != nil {
					t.Fatal(err)
				}
				repair143Write(t, state, "send.json", encoded)
			case "activity":
				repair143Write(t, state, "send.activity-id", []byte(strings.Repeat("0", 64)))
			case "missing-bytes":
				if err := os.Remove(filepath.Join(state, "send.bin")); err != nil {
					t.Fatal(err)
				}
			case "truncated":
				repair143Write(t, state, "send.bin", raw[:len(raw)-1])
			case "wrong-chain":
				operation = "prepare"
			case "producer-hash":
				operation, arguments = "prepare", []string{strings.Repeat("0", 64)}
			}
			expected := "identity"
			switch fault {
			case "bytes", "nonce", "hash", "status":
				expected = "saved transaction identity or status changed"
			case "activity":
				expected = "retained activity identity differs"
			case "missing-bytes":
				expected = "retained identity has no signed transaction"
			case "truncated":
				expected = "truncated signed transaction"
			case "wrong-chain":
				expected = "signed transaction chain or shape mismatch"
			case "producer-hash":
				expected = "producer identity differs"
			}
			network := chainID.String()
			if fault == "wrong-chain" {
				network = "126"
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			output, err := repair143Command(t, ctx, state, "http://127.0.0.1:1", network, operation, arguments...).CombinedOutput()
			if err == nil || !strings.Contains(string(output), expected) {
				t.Fatalf("fault %s was not refused at journal boundary: %v\n%s", fault, err, output)
			}
			if strings.Contains(string(output), "write-send") {
				t.Fatalf("recovery attempted a new payment: %s", output)
			}
			after, err := os.ReadFile(filepath.Join(state, "send.json"))
			if err != nil {
				t.Fatal(err)
			}
			if fault != "nonce" && fault != "hash" && fault != "status" && !bytes.Equal(before, after) {
				t.Fatal("refusal replaced the retained journal")
			}
		})
	}
}

func TestRepair_143_LegacyIdentityWithoutBytes(t *testing.T) {
	state := t.TempDir()
	identity := []byte(strings.Repeat("a", 64) + "\n")
	repair143Write(t, state, "send.activity-id", identity)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, err := repair143Command(t, ctx, state, "http://127.0.0.1:1", "125", "transfer").CombinedOutput()
	if err == nil || !strings.Contains(string(output), "reconcile manually") {
		t.Fatalf("ambiguous legacy identity regenerated: %v\n%s", err, output)
	}
	retained, err := os.ReadFile(filepath.Join(state, "send.activity-id"))
	if err != nil || !bytes.Equal(retained, identity) {
		t.Fatalf("legacy identity lost: %q, %v", retained, err)
	}
	if _, err := os.Stat(filepath.Join(state, "send.bin")); !os.IsNotExist(err) {
		t.Fatalf("unexpected replacement transaction: %v", err)
	}
}
