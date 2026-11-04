package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	evmcli "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/client/cli"
	spotabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/spot/abi"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkquery "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/query"
	staking "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tools/benchmark/loadtest/operations"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func m859Read(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func m859Write(t *testing.T, path string, raw []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, raw, mode); err != nil {
		t.Fatal(err)
	}
}
func m859JSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func m859Proto(t *testing.T, value interface {
	Reset()
	String() string
	ProtoMessage()
}) []byte {
	t.Helper()
	raw, err := codec.ProtoMarshalJSON(value, nil)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// This is the real in-memory keyring producer used by the CLI. The repair's
// consumer receives its serialized KeyOutput, including the generated mnemonic.
type m859Keys struct {
	t         *testing.T
	mutex     sync.Mutex
	names     map[string]bool
	outputs   map[string]operations.Account
	fail      string
	duplicate bool
	first     *operations.Account
}

func (p *m859Keys) Run(ctx context.Context, c operations.Command) ([]byte, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(c.Args) != 10 || !reflect.DeepEqual(c.Args[:3], []string{"paxd", "keys", "add"}) || !reflect.DeepEqual(c.Args[4:6], []string{"--home", c.Args[5]}) || !reflect.DeepEqual(c.Args[6:], []string{"--output", "json", "--keyring-backend", "test"}) {
		return nil, fmt.Errorf("unexpected key command: %q", c.Args)
	}
	name := c.Args[3]
	if p.names == nil {
		p.names = map[string]bool{}
		p.outputs = map[string]operations.Account{}
	}
	if p.names[name] {
		return nil, errors.New("duplicate key command")
	}
	p.names[name] = true
	if name == p.fail {
		return nil, context.Canceled
	}
	info, mnemonic, err := keyring.NewInMemory().NewMnemonic(name, keyring.English, sdk.FullFundraiserPath, "", hd.Secp256k1)
	if err != nil {
		return nil, err
	}
	output, err := keyring.MkAccKeyOutput(info)
	if err != nil {
		return nil, err
	}
	output.Mnemonic = mnemonic
	account := operations.Account{Address: output.Address, Mnemonic: mnemonic}
	if p.duplicate && p.first != nil {
		output.Address = p.first.Address
		output.Mnemonic = p.first.Mnemonic
		account = *p.first
	}
	if p.first == nil {
		p.first = &account
	}
	p.outputs[name] = account
	return json.Marshal(output)
}

func m859Genesis(t *testing.T) (string, string, []byte) {
	t.Helper()
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, "config"), 0700); err != nil {
		t.Fatal(err)
	}
	// The bank/auth producer's state shape is retained, including an existing account.
	raw := []byte(`{"app_state":{"bank":{"balances":[{"address":"existing","coins":[]}]},"auth":{"accounts":[{"address":"existing"}]}},"chain_id":"preserved"}`)
	m859Write(t, filepath.Join(home, "config", "genesis.json"), raw, 0600)
	return home, filepath.Join(home, "private accounts"), raw
}

func TestRepairM_859_GenesisAccountCount(t *testing.T) {
	for _, count := range []int{0, 1, 63, 64, 65, 129} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			home, accounts, _ := m859Genesis(t)
			peer := &m859Keys{t: t}
			options := operations.GenesisOptions{Count: count, Home: home, Accounts: accounts, Paxd: "paxd", Local: true}
			if err := operations.GenerateGenesis(context.Background(), peer, options); err != nil {
				t.Fatal(err)
			}
			// Decode independently to assert contiguous ordering and preservation.
			var result map[string]any
			if err := json.Unmarshal(m859Read(t, filepath.Join(home, "config", "genesis.json")), &result); err != nil {
				t.Fatal(err)
			}
			app := result["app_state"].(map[string]any)
			balances := app["bank"].(map[string]any)["balances"].([]any)
			auth := app["auth"].(map[string]any)["accounts"].([]any)
			if len(peer.names) != count || len(balances) != count+1 || len(auth) != count+1 || result["chain_id"] != "preserved" {
				t.Fatalf("inventory mismatch: %d commands, %d balances, %d accounts", len(peer.names), len(balances), len(auth))
			}
			if balances[0].(map[string]any)["address"] != "existing" || auth[0].(map[string]any)["address"] != "existing" {
				t.Fatal("existing state was changed")
			}
			files, err := os.ReadDir(accounts)
			if err != nil {
				t.Fatal(err)
			}
			if len(files) != count {
				t.Fatalf("private inventory = %d, want %d", len(files), count)
			}
			for index := 0; index < count; index++ {
				name := "ta" + strconv.Itoa(index)
				var saved operations.Account
				path := filepath.Join(accounts, name+".json")
				if err := json.Unmarshal(m859Read(t, path), &saved); err != nil {
					t.Fatal(err)
				}
				info, err := os.Stat(path)
				if err != nil || info.Mode().Perm() != 0600 {
					t.Fatalf("private seed mode: %v %v", info, err)
				}
				if saved != peer.outputs[name] || balances[index+1].(map[string]any)["address"] != saved.Address || auth[index+1].(map[string]any)["address"] != saved.Address || auth[index+1].(map[string]any)["account_number"] != strconv.Itoa(index) {
					t.Fatalf("account %d was not persisted and funded contiguously", index)
				}
			}
		})
	}
	for _, kind := range []string{"participant-error", "duplicate-address", "existing-seed"} {
		t.Run(kind, func(t *testing.T) {
			home, accounts, before := m859Genesis(t)
			peer := &m859Keys{t: t}
			if kind == "participant-error" {
				peer.fail = "ta17"
			}
			if kind == "duplicate-address" {
				peer.duplicate = true
			}
			if kind == "existing-seed" {
				if err := os.Mkdir(accounts, 0700); err != nil {
					t.Fatal(err)
				}
				m859Write(t, filepath.Join(accounts, "ta17.json"), []byte("do not overwrite"), 0600)
			}
			err := operations.GenerateGenesis(context.Background(), peer, operations.GenesisOptions{Count: 129, Home: home, Accounts: accounts, Paxd: "paxd", Local: true})
			if err == nil {
				t.Fatal("invalid inventory was accepted")
			}
			if !bytes.Equal(before, m859Read(t, filepath.Join(home, "config", "genesis.json"))) {
				t.Fatal("failed inventory published genesis")
			}
			if len(peer.names) != 129 {
				t.Fatalf("not all participants were collected: %d", len(peer.names))
			}
			if kind == "existing-seed" && string(m859Read(t, filepath.Join(accounts, "ta17.json"))) != "do not overwrite" {
				t.Fatal("existing seed was overwritten")
			}
		})
	}
}

func TestRepairM_859_GenesisMnemonicPermissions(t *testing.T) {
	peer := &m859Keys{t: t}
	raw, err := peer.Run(context.Background(), operations.Command{Args: []string{"paxd", "keys", "add", "seed", "--home", "unused", "--output", "json", "--keyring-backend", "test"}})
	if err != nil {
		t.Fatal(err)
	}
	var account operations.Account
	if err = json.Unmarshal(raw, &account); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	dir := filepath.Join(root, "private")
	if err = operations.SaveAccount(dir, "ta0", account); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatalf("directory permissions: %v, %v", info, err)
	}
	file := filepath.Join(dir, "ta0.json")
	before := m859Read(t, file)
	info, err = os.Stat(file)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("file permissions: %v %v", info, err)
	}
	if err = operations.SaveAccount(dir, "ta0", account); err == nil {
		t.Fatal("overwrote an existing seed")
	}
	if !bytes.Equal(before, m859Read(t, file)) {
		t.Fatal("existing seed changed")
	}
	target := filepath.Join(root, "victim")
	m859Write(t, target, []byte("untouched"), 0600)
	if err = os.Symlink(target, filepath.Join(dir, "ta1.json")); err != nil {
		t.Fatal(err)
	}
	if err = operations.SaveAccount(dir, "ta1", account); err == nil {
		t.Fatal("followed seed symlink")
	}
	if string(m859Read(t, target)) != "untouched" {
		t.Fatal("symlink target changed")
	}
	link := filepath.Join(root, "linked-directory")
	if err = os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	if err = operations.SaveAccount(link, "ta2", account); err == nil {
		t.Fatal("followed account directory symlink")
	}
	shared := filepath.Join(root, "shared")
	if err = os.Mkdir(shared, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(shared, 0777); err != nil {
		t.Fatal(err)
	}
	if err = operations.SaveAccount(shared, "ta2", account); err == nil {
		t.Fatal("accepted a shared account directory")
	}
	if err = operations.SaveAccount(dir, "../escape", account); err == nil {
		t.Fatal("accepted path traversal")
	}
	home, accounts, original := m859Genesis(t)
	if err = os.Mkdir(accounts, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(target, filepath.Join(accounts, "ta0.json")); err != nil {
		t.Fatal(err)
	}
	if err = operations.GenerateGenesis(context.Background(), &m859Keys{t: t}, operations.GenesisOptions{Count: 1, Home: home, Accounts: accounts, Paxd: "paxd", Local: true}); err == nil {
		t.Fatal("funded an account whose seed could not be stored safely")
	}
	if !bytes.Equal(original, m859Read(t, filepath.Join(home, "config", "genesis.json"))) {
		t.Fatal("unsafe seed persistence published funding")
	}
}

// This peer replays committed generator output through the generator's actual
// file transport. No abigen binary, toolchain, or subprocess runs in this gate.
type m859Bindings struct {
	t      *testing.T
	mode   string
	called int
	abi    string
	output []byte
}

func (p *m859Bindings) Run(ctx context.Context, c operations.Command) ([]byte, error) {
	p.called++
	if len(c.Args) != 4 || c.Args[0] != "abigen" || c.Args[1] != "--abi="+p.abi || c.Args[2] != "--pkg=erc20" || !strings.HasPrefix(c.Args[3], "--out=") {
		return nil, fmt.Errorf("invalid generator command %q", c.Args)
	}
	output := strings.TrimPrefix(c.Args[3], "--out=")
	if !filepath.IsAbs(output) || filepath.Dir(filepath.Dir(output)) != filepath.Dir(p.abi) {
		return nil, errors.New("generator output is not staged beside the binding")
	}
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		return nil, errors.New("generator output was not fresh")
	}
	switch p.mode {
	case "exit":
		return nil, errors.New("abigen failed")
	case "missing":
		return nil, nil
	case "symlink":
		return nil, os.Symlink(p.abi, output)
	case "empty":
		return nil, os.WriteFile(output, nil, 0600)
	case "syntax":
		return nil, os.WriteFile(output, []byte("package erc20\nfunc ("), 0600)
	case "package":
		return nil, os.WriteFile(output, []byte("package stale\nconst Stale=1\n"), 0600)
	case "changed-abi":
		if err := os.WriteFile(p.abi, []byte("[]"), 0600); err != nil {
			return nil, err
		}
	}
	return nil, os.WriteFile(output, p.output, 0600)
}

func TestRepairM_859_BindingsGeneratorExit(t *testing.T) {
	for _, mode := range []string{"success", "exit", "missing", "symlink", "empty", "syntax", "package", "changed-abi"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "erc20")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			abi := filepath.Join(dir, "abi.json")
			m859Write(t, abi, m859Read(t, "contracts/evm/bindings/erc20/abi.json"), 0600)
			target := filepath.Join(dir, "erc20.go")
			old := []byte("package erc20\nconst Previous=1\n")
			m859Write(t, target, old, 0600)
			generated := m859Read(t, "contracts/evm/bindings/erc20/erc20.go")
			peer := &m859Bindings{t: t, mode: mode, abi: abi, output: generated}
			err := operations.RunCLI(context.Background(), []string{"generate-bindings", "--bindings", root, "--abigen", "abigen"}, nil, io.Discard, peer)
			if mode == "success" {
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(m859Read(t, target), generated) {
					t.Fatal("generated output was not published")
				}
			} else {
				if err == nil {
					t.Fatal("invalid generator result reported success")
				}
				if !bytes.Equal(old, m859Read(t, target)) {
					t.Fatal("failed generation replaced the existing binding")
				}
			}
			if peer.called != 1 {
				t.Fatalf("generator called %d times", peer.called)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 2 {
				t.Fatalf("staging files leaked: %v", entries)
			}
		})
	}
	t.Run("missing-abi", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, "erc20"), 0700); err != nil {
			t.Fatal(err)
		}
		peer := &m859Bindings{}
		if err := operations.GenerateBindings(context.Background(), peer, root, "abigen"); err == nil || peer.called != 0 {
			t.Fatal("missing ABI did not fail before invoking generator")
		}
	})
}

func m859BlockServer(t *testing.T, stamp string) (*httptest.Server, *int) {
	t.Helper()
	template := m859Read(t, "scripts/fixtures/consensus_block_populated.json")
	var body map[string]any
	if err := json.Unmarshal(template, &body); err != nil {
		t.Fatal(err)
	}
	block := body["result"].(map[string]any)["block"].(map[string]any)
	block["header"].(map[string]any)["time"] = stamp
	raw := m859JSON(t, body)
	calls := new(int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls++
		w.Header().Set("Content-Type", "application/json")
		w.Write(raw)
	}))
	t.Cleanup(server.Close)
	return server, calls
}

func TestRepairM_859_BenchmarkMetricDeadlines(t *testing.T) {
	for _, stamp := range []string{"2026-11-04T02:47:17Z", "2026-11-04T02:47:17.1Z", "2026-11-04T02:47:17.123456789Z", "2026-11-04T04:47:17.123456789+02:00", "2026-11-03T21:17:17.123456789-05:30"} {
		t.Run(stamp, func(t *testing.T) {
			server, _ := m859BlockServer(t, stamp)
			client := operations.MetricsClient{RPC: server.URL}
			info, err := client.BlockInfo(context.Background(), 33716909)
			if err != nil {
				t.Fatal(err)
			}
			expected, err := time.Parse(time.RFC3339Nano, stamp)
			if err != nil {
				t.Fatal(err)
			}
			if !info.Timestamp.Equal(expected) || info.Timestamp.Nanosecond() != expected.Nanosecond() || info.NumberOfTxs != 2 {
				t.Fatalf("complete timestamp was not retained: %+v", info)
			}
			mapping, err := client.Breakdown(context.Background(), 33716909)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(mapping, map[string]int{"staking": 1, "bank": 1}) {
				t.Fatalf("recorded transactions decoded incorrectly: %v", mapping)
			}
		})
	}
	for _, stamp := range []string{"2026-11-04T02:47:17", "2026-11-04T02:47:17Zgarbage", "2026-11-04T02:47:17+25:00"} {
		t.Run("invalid-"+stamp, func(t *testing.T) {
			server, _ := m859BlockServer(t, stamp)
			if _, err := (operations.MetricsClient{RPC: server.URL}).BlockInfo(context.Background(), 1); err == nil {
				t.Fatal("invalid complete timestamp accepted")
			}
		})
	}
	for _, delta := range []time.Duration{0, -time.Second, 100 * time.Nanosecond, 500 * time.Microsecond} {
		t.Run(delta.String(), func(t *testing.T) {
			initial := time.Date(2026, 11, 4, 2, 47, 17, 123456789, time.UTC)
			server, _ := m859BlockServer(t, initial.Add(delta).Format(time.RFC3339Nano))
			client := operations.MetricsClient{RPC: server.URL}
			result, err := client.BestStats(context.Background(), []operations.BlockInfo{{Height: 1, Timestamp: initial, NumberOfTxs: 2}})
			if err != nil {
				t.Fatal(err)
			}
			if delta <= 0 {
				if !reflect.DeepEqual(result, []any{nil, nil, nil}) {
					t.Fatalf("nonpositive duration must be unavailable: %v", result)
				}
			} else {
				want := 2 / delta.Seconds()
				if result[0] != want || result[1] != 1 || math.Abs(result[2].(float64)-float64(delta)/float64(time.Millisecond)) > 1e-12 {
					t.Fatalf("submillisecond precision lost: %v", result)
				}
			}
		})
	}
	for _, operation := range []string{"block", "height"} {
		t.Run("deadline-"+operation, func(t *testing.T) {
			released := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done(); close(released) }))
			defer server.Close()
			client := operations.MetricsClient{RPC: server.URL, REST: server.URL}
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			started := time.Now()
			var err error
			if operation == "block" {
				_, err = client.BlockInfo(ctx, 1)
			} else {
				_, err = client.Height(ctx, "hash")
			}
			elapsed := time.Since(started)
			if err == nil || elapsed > operations.RequestTimeout+time.Second {
				t.Fatalf("request was not bounded at %v: %v, %v", operations.RequestTimeout, elapsed, err)
			}
			select {
			case <-released:
			case <-time.After(time.Second):
				t.Fatal("request did not cancel the server's context")
			}
		})
	}
	t.Run("summary-unavailable", func(t *testing.T) {
		stamp := "2026-11-04T02:47:17Z"
		template := m859Read(t, "scripts/fixtures/consensus_block_populated.json")
		var body map[string]any
		if err := json.Unmarshal(template, &body); err != nil {
			t.Fatal(err)
		}
		body["result"].(map[string]any)["block"].(map[string]any)["header"].(map[string]any)["time"] = stamp
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/txs/") {
				height := strings.TrimPrefix(r.URL.Path, "/txs/")
				json.NewEncoder(w).Encode(map[string]string{"height": height})
				return
			}
			json.NewEncoder(w).Encode(body)
		}))
		defer server.Close()
		hashes := filepath.Join(t.TempDir(), "hashes")
		m859Write(t, hashes, []byte("1\n2\n3\n"), 0600)
		result, err := (operations.MetricsClient{RPC: server.URL, REST: server.URL}).Metrics(context.Background(), hashes)
		if err != nil {
			t.Fatal(err)
		}
		summary := result["Summary (excl. edge block)"].(map[string]any)
		best := result["Best block"].(map[string]any)
		if summary["average_throughput_per_sec"] != nil || summary["average_block_time"] != nil || best["height"] != nil || best["tx_mapping"] != nil {
			t.Fatalf("invalid duration produced available metrics: %v", result)
		}
	})
}

func TestRepairM_859_MetricEnvelopePreserved(t *testing.T) {
	// Preserve the earlier repair's full envelope and validation guarantees.
	for _, name := range []string{"empty", "populated", "error"} {
		t.Run(name, func(t *testing.T) {
			raw := m859Read(t, "scripts/fixtures/consensus_block_"+name+".json")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(raw) }))
			defer server.Close()
			client := operations.MetricsClient{RPC: server.URL}
			block, err := client.Block(context.Background(), 1)
			if name == "error" {
				if err == nil || !strings.Contains(err.Error(), "Block JSON-RPC error -32603") {
					t.Fatalf("recorded JSON-RPC error lost: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			count := len(block["data"].(map[string]any)["txs"].([]any))
			if (name == "empty" && count != 0) || (name == "populated" && count != 2) {
				t.Fatalf("wrong tx count %d", count)
			}
		})
	}
	for _, status := range []int{400, 500, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
			defer server.Close()
			_, err := (operations.MetricsClient{RPC: server.URL}).Block(context.Background(), 1)
			var failure *operations.HTTPError
			if !errors.As(err, &failure) || failure.Status != status {
				t.Fatalf("HTTP status lost: %v", err)
			}
		})
	}
	invalid := []string{`not JSON`, `[]`, `{"jsonrpc":"1.0","result":{}}`, `{"jsonrpc":"2.0","result":null}`, `{"jsonrpc":"2.0","result":{"block":null}}`, `{"jsonrpc":"2.0","error":"invalid error"}`, `{"jsonrpc":"2.0","result":{"block":{"data":{}}}}`, `{"jsonrpc":"2.0","result":{"block":{"data":{"txs":"not-an-array"}}}}`, `{"jsonrpc":"2.0","result":{"block":{"data":{"txs":[42]}}}}`}
	for index, raw := range invalid {
		t.Run(fmt.Sprintf("invalid-%d", index), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(raw)) }))
			defer server.Close()
			client := operations.MetricsClient{RPC: server.URL}
			if _, err := client.Block(context.Background(), 1); err == nil {
				t.Fatal("malformed envelope accepted")
			}
			if _, err := client.Breakdown(context.Background(), 1); err == nil {
				t.Fatal("breakdown accepted malformed envelope")
			}
		})
	}
	t.Run("null-transactions", func(t *testing.T) {
		raw := bytes.Replace(m859Read(t, "scripts/fixtures/consensus_block_empty.json"), []byte(`"txs":[]`), []byte(`"txs":null`), 1)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(raw) }))
		defer server.Close()
		client := operations.MetricsClient{RPC: server.URL}
		block, err := client.Block(context.Background(), 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(block["data"].(map[string]any)["txs"].([]any)) != 0 {
			t.Fatal("null tx list was not normalized")
		}
	})
}

type m859ExchangePeer struct {
	t        *testing.T
	calls    int
	failAt   int
	hash     common.Hash
	lastData []byte
	methods  []string
	noHash   bool
}

func (p *m859ExchangePeer) Run(ctx context.Context, c operations.Command) ([]byte, error) {
	if len(c.Args) < 12 || !reflect.DeepEqual(c.Args[:3], []string{"paxd", "tx", "evm"}) {
		return nil, fmt.Errorf("unexpected exchange command: %q", c.Args)
	}
	flags := 0
	for index, arg := range c.Args {
		if arg == "--from" {
			flags = index
			break
		}
	}
	if flags == 0 || !reflect.DeepEqual(c.Args[flags:], []string{"--from", "owner", "--chain-id", "test-chain", "--evm-rpc", c.Args[flags+5], "--value", "0", "-y"}) {
		return nil, fmt.Errorf("invalid exchange flags: %q", c.Args)
	}
	var to common.Address
	var payload []byte
	switch c.Args[3] {
	case "call-precompile":
		info, encoded, err := evmcli.BuildPrecompileCall(c.Args[4], c.Args[5:flags])
		if err != nil {
			return nil, err
		}
		to = info.Address
		payload = encoded
		method, err := info.ABI.MethodById(payload[:4])
		if err != nil {
			return nil, err
		}
		decoded, err := method.Inputs.Unpack(payload[4:])
		if err != nil {
			return nil, err
		}
		if method.Name == "registerToken" {
			if len(decoded) != 2 {
				return nil, errors.New("wrong token registration ABI")
			}
		} else if method.Name == "placeLimitBuy" {
			if len(decoded) != 5 || decoded[2].(*big.Int).Cmp(big.NewInt(100)) != 0 {
				return nil, errors.New("wrong order ABI")
			}
		} else {
			return nil, errors.New("retired or unsupported exchange operation")
		}
		p.methods = append(p.methods, method.Name)
	case "call-contract":
		if flags != 6 {
			return nil, errors.New("wrong approval arguments")
		}
		to = common.HexToAddress(c.Args[4])
		var err error
		payload, err = hex.DecodeString(c.Args[5])
		if err != nil {
			return nil, err
		}
		if len(payload) != 68 || hex.EncodeToString(payload[:4]) != "095ea7b3" || common.BytesToAddress(payload[4:36]) != common.HexToAddress(operations.NativeOrdersAddress) || new(big.Int).SetBytes(payload[36:]).Cmp(big.NewInt(1000)) != 0 {
			return nil, errors.New("approval did not cover exactly ten native orders")
		}
		p.methods = append(p.methods, "approve")
	default:
		return nil, errors.New("retired exchange command")
	}
	p.calls++
	if p.failAt == p.calls {
		return nil, errors.New("CLI submission failed")
	}
	// A real signed transaction supplies the hash consumed by the receipt RPC.
	key, err := crypto.HexToECDSA(strings.Repeat("1", 64))
	if err != nil {
		return nil, err
	}
	tx, err := ethtypes.SignTx(ethtypes.NewTransaction(uint64(p.calls), to, big.NewInt(0), 500000, big.NewInt(1), payload), ethtypes.NewEIP155Signer(big.NewInt(125)), key)
	if err != nil {
		return nil, err
	}
	p.hash = tx.Hash()
	p.lastData = payload
	if p.noHash {
		return []byte("submitted without a hash\n"), nil
	}
	return []byte("Transaction hash: " + p.hash.Hex() + "\n"), nil
}

func TestRepairM_859_DexDeployRetired(t *testing.T) {
	for _, mode := range []string{"success", "pending-then-success", "cli-failure", "cli-no-hash", "revert", "missing-hash", "uncommitted", "rpc-error"} {
		t.Run(mode, func(t *testing.T) {
			peer := &m859ExchangePeer{t: t, noHash: mode == "cli-no-hash"}
			if mode == "cli-failure" {
				peer.failAt = 5
			}
			receipts := 0
			pending := map[string]bool{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" {
					t.Error("incorrect receipt transport")
				}
				var request struct {
					Method string
					Params []string
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					return
				}
				if request.Method != "eth_getTransactionReceipt" || len(request.Params) != 1 || request.Params[0] != peer.hash.Hex() {
					t.Errorf("receipt does not bind the CLI transaction: %+v", request)
				}
				receipts++
				status := uint64(1)
				height := big.NewInt(10)
				hash := peer.hash
				if mode == "revert" && receipts == 5 {
					status = 0
				}
				if mode == "missing-hash" {
					hash = common.Hash{}
				}
				if mode == "uncommitted" {
					height = big.NewInt(0)
				}
				receipt := &ethtypes.Receipt{Type: ethtypes.LegacyTxType, Status: status, CumulativeGasUsed: 100000, Logs: []*ethtypes.Log{}, TxHash: hash, GasUsed: 100000, BlockHash: common.HexToHash("0x1"), BlockNumber: height}
				response := map[string]any{"jsonrpc": "2.0", "id": 1, "result": receipt}
				if mode == "pending-then-success" && !pending[peer.hash.Hex()] {
					response["result"] = nil
					pending[peer.hash.Hex()] = true
				}
				if mode == "rpc-error" {
					response["error"] = map[string]any{"code": -32000, "message": "unavailable"}
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			config := filepath.Join(t.TempDir(), "config.json")
			original := []byte(`{"contract_distribution":[{"contract_address":"previous","percentage":"1"}],"unrelated":"preserved"}`)
			m859Write(t, config, original, 0600)
			options := operations.ExchangeOptions{Paxd: "paxd", From: "owner", ChainID: "test-chain", RPC: server.URL, Config: config, Scenario: operations.ExchangeScenario{Stablecoin: "0x0000000000000000000000000000000000001234", Token: "0x0000000000000000000000000000000000005678", Amount: "100", TargetPrice: "1000000000000000000", ExpiresAt: "4102444800"}}
			err := (operations.ExchangeSetup{Commands: peer, Poll: time.Millisecond}).Deploy(context.Background(), options)
			if mode != "success" && mode != "pending-then-success" {
				if err == nil {
					t.Fatal("unsuccessful exchange setup reported success")
				}
				if !bytes.Equal(original, m859Read(t, config)) {
					t.Fatal("unconfirmed setup published contract_distribution")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			expectedReceipts := 13
			if mode == "pending-then-success" {
				expectedReceipts = 26
			}
			if receipts != expectedReceipts || peer.calls != 13 {
				t.Fatalf("not all native setup operations were committed: commands=%d receipts=%d", peer.calls, receipts)
			}
			if !reflect.DeepEqual(peer.methods[:3], []string{"registerToken", "registerToken", "approve"}) {
				t.Fatalf("native asset setup missing: %v", peer.methods)
			}
			for _, method := range peer.methods[3:] {
				if method != "placeLimitBuy" {
					t.Fatalf("retired order operation: %s", method)
				}
			}
			var result map[string]any
			if err = json.Unmarshal(m859Read(t, config), &result); err != nil {
				t.Fatal(err)
			}
			distribution := result["contract_distribution"].([]any)
			if len(distribution) != 10 || result["unrelated"] != "preserved" {
				t.Fatalf("incorrect config publication: %v", result)
			}
			for _, entry := range distribution {
				value := entry.(map[string]any)
				if value["contract_address"] != operations.NativeOrdersAddress || value["percentage"] != "0.1" {
					t.Fatal("config contains a retired contract")
				}
			}
		})
	}
	// The test also resolves the current native ABIs directly, independent of
	// command spelling, so an interface rename cannot silently pass.
	vault, err := spotabi.Vault()
	if err != nil {
		t.Fatal(err)
	}
	orders, err := spotabi.Orders()
	if err != nil {
		t.Fatal(err)
	}
	if vault.Methods["registerToken"].Sig != "registerToken(address,bool)" || orders.Methods["placeLimitBuy"].Sig != "placeLimitBuy(address,address,uint256,uint256,uint256)" {
		t.Fatal("native exchange interface changed")
	}
}

type m859Step struct {
	args   []string
	output []byte
	err    error
	after  func()
}
type m859Replay struct {
	t     *testing.T
	steps []m859Step
	index int
}

func (p *m859Replay) Run(ctx context.Context, c operations.Command) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("operation used a canceled cleanup context: %w", err)
	}
	if p.index >= len(p.steps) {
		return nil, fmt.Errorf("unexpected additional command %q", c.Args)
	}
	step := p.steps[p.index]
	p.index++
	if !reflect.DeepEqual(c.Args, step.args) || c.Input != "" {
		return nil, fmt.Errorf("command %d = %q input=%q; want %q without a shell or password", p.index, c.Args, c.Input, step.args)
	}
	if step.after != nil {
		step.after()
	}
	return step.output, step.err
}
func (p *m859Replay) done() {
	p.t.Helper()
	if p.index != len(p.steps) {
		p.t.Fatalf("consumed %d of %d transcript commands", p.index, len(p.steps))
	}
}

func m859Remote(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	var fixture map[string]json.RawMessage
	if err := json.Unmarshal(m859Read(t, "testdata/m859/remote.json"), &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}
func m859SSH(host string, args ...string) []string {
	return append([]string{"ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=10", "-i", "identity with spaces;$(false)", "--", "ubuntu@" + host}, args...)
}
func m859Q(host string) []string {
	return m859SSH(host, "sudo", "-n", "tc", "-j", "qdisc", "show", "dev", "ens5")
}
func m859Add(host string) []string {
	return m859SSH(host, "sudo", "-n", "tc", "qdisc", "add", "dev", "ens5", "root", "handle", "1f00:", "netem", "delay", "200ms", "10ms", "25%")
}
func m859Del(host string) []string {
	return m859SSH(host, "sudo", "-n", "tc", "qdisc", "del", "dev", "ens5", "root", "handle", "1f00:")
}
func m859Options(t *testing.T) operations.FaultOptions {
	t.Helper()
	return operations.FaultOptions{Paxd: "paxd", SSHKey: "identity with spaces;$(false)", ChainID: "test-chain", Validators: []string{"validator-a", "validator-b", "validator-c"}, Concurrent: 2, Recovery: filepath.Join(t.TempDir(), "controller", "recovery.json")}
}
func m859Applied(t *testing.T, fixture map[string]json.RawMessage, host string) []m859Step {
	t.Helper()
	return []m859Step{{args: m859Q(host), output: fixture["inactive"]}, {args: m859Add(host)}, {args: m859Q(host), output: fixture["delayed"]}}
}
func m859Restored(t *testing.T, fixture map[string]json.RawMessage, host string) []m859Step {
	t.Helper()
	return []m859Step{{args: m859Q(host), output: fixture["delayed"]}, {args: m859Del(host)}, {args: m859Q(host), output: fixture["inactive"]}}
}

func TestRepairM_859_ValidatorFaultCleanup(t *testing.T) {
	fixture := m859Remote(t)
	t.Run("odd-batch-and-finally", func(t *testing.T) {
		options := m859Options(t)
		peer := &m859Replay{t: t}
		for _, host := range options.Validators[:2] {
			peer.steps = append(peer.steps, m859Applied(t, fixture, host)...)
		}
		for _, host := range []string{"validator-b", "validator-a"} {
			peer.steps = append(peer.steps, m859Restored(t, fixture, host)...)
		}
		peer.steps = append(peer.steps, m859Applied(t, fixture, "validator-c")...)
		peer.steps = append(peer.steps, m859Restored(t, fixture, "validator-c")...)
		// A successful remote mutation must already have an actionable durable intent.
		peer.steps[1].after = func() {
			var record operations.RecoveryRecord
			if err := json.Unmarshal(m859Read(t, options.Recovery), &record); err != nil {
				t.Fatal(err)
			}
			if len(record.Actions) != 1 || record.Actions[0].Host != "validator-a" || record.Actions[0].Applied {
				t.Fatal("no durable intent before applying fault")
			}
		}
		controller := operations.NewFaultController(peer, options)
		if err := controller.SlowNetwork(context.Background()); err != nil {
			t.Fatal(err)
		}
		peer.done()
		if _, err := os.Stat(options.Recovery); !os.IsNotExist(err) {
			t.Fatalf("fully restored run retained recovery file: %v", err)
		}
	})
	t.Run("partial-application", func(t *testing.T) {
		options := m859Options(t)
		peer := &m859Replay{t: t, steps: m859Applied(t, fixture, "validator-a")}
		peer.steps = append(peer.steps, m859Step{args: m859Q("validator-b"), output: fixture["inactive"]}, m859Step{args: m859Add("validator-b"), err: errors.New("remote operation exited 255")}, m859Step{args: m859Q("validator-b"), output: fixture["inactive"]}, m859Step{args: m859Q("validator-b"), output: fixture["inactive"]})
		peer.steps = append(peer.steps, m859Restored(t, fixture, "validator-a")...)
		if err := operations.NewFaultController(peer, options).SlowNetwork(context.Background()); err == nil || !strings.Contains(err.Error(), "255") {
			t.Fatalf("partial batch did not report original failure: %v", err)
		}
		peer.done()
		if _, err := os.Stat(options.Recovery); !os.IsNotExist(err) {
			t.Fatal("successfully cleaned partial batch retained journal")
		}
	})
	t.Run("signal-stop-reopen-recover", func(t *testing.T) {
		options := m859Options(t)
		options.Hold = time.Hour
		peer := &m859Replay{t: t}
		for _, host := range options.Validators[:2] {
			peer.steps = append(peer.steps, m859Applied(t, fixture, host)...)
		}
		ctx, stop := operations.SignalContext()
		defer stop()
		peer.steps[len(peer.steps)-1].after = func() {
			if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
				t.Fatal(err)
			}
		}
		peer.steps = append(peer.steps, m859Step{args: m859Q("validator-b"), output: fixture["delayed"]}, m859Step{args: m859Del("validator-b"), err: errors.New("remote operation exited 255")})
		peer.steps = append(peer.steps, m859Restored(t, fixture, "validator-a")...)
		controller := operations.NewFaultController(peer, options)
		err := controller.SlowNetwork(ctx)
		if err == nil || !strings.Contains(err.Error(), "cleanup incomplete") || !strings.Contains(err.Error(), "--recover "+options.Recovery) {
			t.Fatalf("missing retained recovery instructions: %v", err)
		}
		peer.done()
		stop()
		info, e := os.Stat(options.Recovery)
		if e != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("recovery record is not private: %v %v", info, e)
		}
		var record operations.RecoveryRecord
		if e = json.Unmarshal(m859Read(t, options.Recovery), &record); e != nil {
			t.Fatal(e)
		}
		if len(record.Actions) != 1 || !record.Actions[0].Applied || record.Actions[0].Host != "validator-b" || record.Options.SSHKey != options.SSHKey {
			t.Fatalf("changed validators not retained: %+v", record)
		}
		// Stop the old controller and reopen only its durable record, with a fresh
		// peer/session and context. No state is copied from the old object.
		controller = nil
		peer = nil
		resumedPeer := &m859Replay{t: t}
		resumedPeer.steps = append(resumedPeer.steps, m859Restored(t, fixture, "validator-b")...)
		reopened, e := operations.OpenRecovery(resumedPeer, options.Recovery)
		if e != nil {
			t.Fatal(e)
		}
		if e = reopened.Cleanup(); e != nil {
			t.Fatal(e)
		}
		resumedPeer.done()
		if _, e = os.Stat(options.Recovery); !os.IsNotExist(e) {
			t.Fatalf("recovered controller did not clear completed journal: %v", e)
		}
	})
	t.Run("refuse-existing-record", func(t *testing.T) {
		options := m859Options(t)
		if err := os.Mkdir(filepath.Dir(options.Recovery), 0700); err != nil {
			t.Fatal(err)
		}
		before := []byte("operator recovery data")
		m859Write(t, options.Recovery, before, 0600)
		peer := &m859Replay{t: t}
		if err := operations.NewFaultController(peer, options).SlowNetwork(context.Background()); err == nil {
			t.Fatal("overwrote unresolved recovery record")
		}
		peer.done()
		if !bytes.Equal(before, m859Read(t, options.Recovery)) {
			t.Fatal("old recovery record changed")
		}
	})
}

func m859Delegation(t *testing.T, admin, validator, amount string) []byte {
	t.Helper()
	value, ok := sdk.NewIntFromString(amount)
	if !ok {
		t.Fatal("invalid delegation fixture")
	}
	return m859Proto(t, &staking.DelegationResponse{Delegation: staking.Delegation{DelegatorAddress: admin, ValidatorAddress: validator, Shares: sdk.NewDecFromInt(value)}, Balance: sdk.NewCoin("uhpx", value)})
}
func m859StakingCommand(operation, validator, amount string) []string {
	return []string{"paxd", "tx", "staking", operation, validator, amount + "uhpx", "--from", "admin", "--chain-id", "test-chain", "--broadcast-mode", "block", "--output", "json", "-y"}
}
func m859DelegationQuery(admin, validator string) []string {
	return []string{"paxd", "q", "staking", "delegation", admin, validator, "--chain-id", "test-chain", "--output", "json"}
}

func TestRepairM_859_ValidatorFaultCommands(t *testing.T) {
	fixture := m859Remote(t)
	for _, mode := range []string{"restart", "restart-unconfirmed"} {
		t.Run(mode, func(t *testing.T) {
			options := m859Options(t)
			peer := &m859Replay{t: t}
			var before, after string
			if err := json.Unmarshal(fixture["restart_before"], &before); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(fixture["restart_after"], &after); err != nil {
				t.Fatal(err)
			}
			if mode == "restart-unconfirmed" {
				after = before
			}
			for _, host := range options.Validators {
				peer.steps = append(peer.steps, m859Step{args: m859SSH(host, "systemctl", "show", "paxd", "--property=InvocationID", "--value"), output: []byte(before)}, m859Step{args: m859SSH(host, "sudo", "-n", "systemctl", "restart", "paxd")}, m859Step{args: m859SSH(host, "systemctl", "is-active", "--quiet", "paxd")}, m859Step{args: m859SSH(host, "systemctl", "show", "paxd", "--property=InvocationID", "--value"), output: []byte(after)})
				if mode == "restart-unconfirmed" {
					break
				}
			}
			err := operations.NewFaultController(peer, options).Restarts(context.Background())
			if (mode == "restart" && err != nil) || (mode != "restart" && err == nil) {
				t.Fatalf("restart confirmation result: %v", err)
			}
			peer.done()
		})
	}
	t.Run("structured-host-refusal", func(t *testing.T) {
		options := m859Options(t)
		options.Validators = []string{"validator;touch /tmp/unsafe"}
		peer := &m859Replay{t: t}
		if err := operations.NewFaultController(peer, options).Restarts(context.Background()); err == nil {
			t.Fatal("unsafe SSH host accepted")
		}
		peer.done()
	})
	t.Run("positive-concurrency", func(t *testing.T) {
		options := m859Options(t)
		options.Concurrent = 0
		peer := &m859Replay{t: t}
		controller := operations.NewFaultController(peer, options)
		if controller.Restarts(context.Background()) == nil || controller.SlowNetwork(context.Background()) == nil {
			t.Fatal("nonpositive batch size accepted")
		}
		peer.done()
	})
	for _, mode := range []string{"delegation", "delegation-rejected", "delegation-unconfirmed", "delegation-recover", "delegation-large", "delegation-paginated"} {
		t.Run(mode, func(t *testing.T) {
			options := m859Options(t)
			peer := &m859Replay{t: t}
			info, _, err := keyring.NewInMemory().NewMnemonic("admin", keyring.English, sdk.FullFundraiserPath, "", hd.Secp256k1)
			if err != nil {
				t.Fatal(err)
			}
			adminKey, err := keyring.MkAccKeyOutput(info)
			if err != nil {
				t.Fatal(err)
			}
			admin := adminKey.Address
			baseline := "100"
			added := "5000"
			target := "5100"
			if mode == "delegation-large" {
				baseline = "1000000000000000000000000"
				added = "50000000000000000000000000"
				target = "51000000000000000000000000"
			}
			baseCoin, ok := sdk.NewIntFromString(baseline)
			if !ok {
				t.Fatal("invalid coin fixture")
			}
			responses := []staking.DelegationResponse{}
			validators := []string{}
			for index := 0; index < 5; index++ {
				address := sdk.ValAddress(bytes.Repeat([]byte{byte(index + 1)}, 20)).String()
				validators = append(validators, address)
				responses = append(responses, staking.DelegationResponse{Delegation: staking.Delegation{DelegatorAddress: admin, ValidatorAddress: address, Shares: sdk.NewDecFromInt(baseCoin)}, Balance: sdk.NewCoin("uhpx", baseCoin)})
			}
			validator := validators[0]
			output := m859Proto(t, &staking.QueryDelegatorDelegationsResponse{DelegationResponses: responses})
			nextKey := base64.StdEncoding.EncodeToString([]byte("page-two"))
			if mode == "delegation-paginated" {
				output = m859Proto(t, &staking.QueryDelegatorDelegationsResponse{DelegationResponses: responses[:4], Pagination: &sdkquery.PageResponse{NextKey: []byte("page-two")}})
			}
			committed := m859Proto(t, &sdk.TxResponse{Height: 11, TxHash: strings.Repeat("1", 64), Code: 0})
			rejected := m859Proto(t, &sdk.TxResponse{Height: 11, TxHash: strings.Repeat("2", 64), Code: 7})
			peer.steps = append(peer.steps, m859Step{args: []string{"paxd", "keys", "list", "--output", "json"}, output: m859JSON(t, []keyring.KeyOutput{adminKey})}, m859Step{args: []string{"paxd", "q", "staking", "delegations", admin, "--chain-id", "test-chain", "--output", "json"}, output: output})
			if mode == "delegation-paginated" {
				peer.steps = append(peer.steps, m859Step{args: []string{"paxd", "q", "staking", "delegations", admin, "--chain-id", "test-chain", "--output", "json", "--page-key", nextKey}, output: m859Proto(t, &staking.QueryDelegatorDelegationsResponse{DelegationResponses: responses[4:]})})
			}
			txOutput := committed
			if mode == "delegation-rejected" {
				txOutput = rejected
			}
			peer.steps = append(peer.steps, m859Step{args: m859StakingCommand("delegate", validator, added), output: txOutput})
			if mode == "delegation-rejected" {
				peer.steps = append(peer.steps, m859Step{args: m859DelegationQuery(admin, validator), output: m859Delegation(t, admin, validator, baseline)})
			} else if mode == "delegation-unconfirmed" {
				peer.steps = append(peer.steps, m859Step{args: m859DelegationQuery(admin, validator), output: m859Delegation(t, admin, validator, baseline)}, m859Step{args: m859DelegationQuery(admin, validator), output: m859Delegation(t, admin, validator, baseline)})
			} else {
				peer.steps = append(peer.steps, m859Step{args: m859DelegationQuery(admin, validator), output: m859Delegation(t, admin, validator, target)}, m859Step{args: m859DelegationQuery(admin, validator), output: m859Delegation(t, admin, validator, target)})
				unbond := m859Step{args: m859StakingCommand("unbond", validator, added), output: committed}
				if mode == "delegation-recover" {
					unbond.err = errors.New("unbond transport failed")
				}
				peer.steps = append(peer.steps, unbond)
				if mode != "delegation-recover" {
					peer.steps = append(peer.steps, m859Step{args: m859DelegationQuery(admin, validator), output: m859Delegation(t, admin, validator, baseline)})
				}
			}
			controller := operations.NewFaultController(peer, options)
			err = controller.DelegationChange(context.Background())
			if mode == "delegation" || mode == "delegation-large" || mode == "delegation-paginated" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("failed delegation reported success")
			}
			peer.done()
			if mode == "delegation-recover" {
				if !strings.Contains(err.Error(), "--recover") {
					t.Fatal("no recovery instructions for retained delegation")
				}
				controller = nil
				resumed := &m859Replay{t: t, steps: []m859Step{{args: m859DelegationQuery(admin, validator), output: m859Delegation(t, admin, validator, target)}, {args: m859StakingCommand("unbond", validator, added), output: committed}, {args: m859DelegationQuery(admin, validator), output: m859Delegation(t, admin, validator, baseline)}}}
				reopened, err := operations.OpenRecovery(resumed, options.Recovery)
				if err != nil {
					t.Fatal(err)
				}
				if err = reopened.Cleanup(); err != nil {
					t.Fatal(err)
				}
				resumed.done()
			}
			if _, err := os.Stat(options.Recovery); !os.IsNotExist(err) {
				t.Fatalf("restored delegation retained record: %v", err)
			}
		})
	}
}
