package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
)

const fixtureChainID = "committed-fixture"

type node struct {
	home             string
	paxd             string
	receiptKeyFile   string
	receiptPublicKey []byte
	rpcURL           string
	client           *rpc.Client
	ctx              context.Context
}

var vectors = make(map[string]func(*node) (any, error))

func registerVector(name string, rec func(*node) (any, error)) {
	name = strings.TrimSuffix(name, ".json")
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name || rec == nil {
		panic("invalid fixture vector registration")
	}
	if _, exists := vectors[name]; exists {
		panic("duplicate fixture vector: " + name)
	}
	vectors[name] = rec
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stderr io.Writer) error {
	if len(args) == 0 || args[0] != "record" {
		return errors.New("usage: committed-fixture record --paxd PATH --out DIRECTORY")
	}
	flags := flag.NewFlagSet("record", flag.ContinueOnError)
	flags.SetOutput(stderr)
	paxd := flags.String("paxd", "paxd", "paxd executable")
	out := flags.String("out", "", "directory for recorded JSON vectors")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if *paxd == "" || *out == "" || flags.NArg() != 0 {
		return errors.New("record requires --paxd and --out and accepts no positional arguments")
	}
	if len(vectors) == 0 {
		return errors.New("no fixture vectors registered")
	}
	n, err := prepareHome(ctx, *paxd)
	if err != nil {
		return err
	}
	defer os.RemoveAll(n.home)
	stop, err := n.start()
	if err != nil {
		return err
	}
	defer stop()
	if err := os.MkdirAll(*out, 0755); err != nil {
		return err
	}
	names := make([]string, 0, len(vectors))
	for name := range vectors {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		value, err := vectors[name](n)
		if err != nil {
			return fmt.Errorf("record %s: %w", name, err)
		}
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return fmt.Errorf("encode %s: %w", name, err)
		}
		if err := os.WriteFile(filepath.Join(*out, name+".json"), append(data, '\n'), 0644); err != nil {
			return err
		}
	}
	return nil
}

func (n *node) command(args ...string) ([]byte, error) {
	args = append(args, "--home", n.home)
	cmd := exec.CommandContext(n.ctx, n.paxd, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("paxd %s: %w: %s", args[0], err, stderr.String())
	}
	return output, nil
}

func prepareHome(ctx context.Context, paxd string) (_ *node, err error) {
	home, err := os.MkdirTemp("", "pax-committed-fixture-")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(home)
		}
	}()
	n := &node{home: home, paxd: paxd, ctx: ctx, receiptKeyFile: filepath.Join(home, "config", "receipt-key.hex")}
	if _, err = n.command("init", "committed-fixture", "--chain-id", fixtureChainID, "--mode", "validator"); err != nil {
		return nil, err
	}
	output, err := n.command("sequencer", "gen-receipt-key", "--output", n.receiptKeyFile)
	if err != nil {
		return nil, err
	}
	var generated struct {
		PublicKey string `json:"public_key"`
	}
	if err = json.Unmarshal(output, &generated); err != nil {
		return nil, fmt.Errorf("decode generated receipt public key: %w", err)
	}
	n.receiptPublicKey, err = hex.DecodeString(generated.PublicKey)
	if err != nil || len(n.receiptPublicKey) != 32 {
		return nil, errors.New("gen-receipt-key did not return a 32-byte public key")
	}
	if _, err = n.command("add-genesis-receipt-key", "--public-key", generated.PublicKey,
		"--first-height", "1", "--last-height", fmt.Sprint(uint64(math.MaxInt64))); err != nil {
		return nil, err
	}
	if err = n.configureGenesis(); err != nil {
		return nil, err
	}
	var validator struct {
		PubKey struct {
			Value []byte `json:"value"`
		} `json:"pub_key"`
	}
	if err = readJSON(filepath.Join(home, "config", "priv_validator_key.json"), &validator); err != nil {
		return nil, err
	}
	if len(validator.PubKey.Value) != 32 {
		return nil, errors.New("init did not generate a 32-byte consensus public key")
	}
	if _, err = n.command("add-genesis-sequencer-leader", "--pubkey", hex.EncodeToString(validator.PubKey.Value)); err != nil {
		return nil, err
	}
	for _, args := range [][]string{
		{"keys", "add", "fixture", "--keyring-backend", "test", "--output", "json"},
		{"add-genesis-account", "fixture", "100000000000000000000uhpx", "--keyring-backend", "test"},
		{"gentx", "fixture", "7000000000000000uhpx", "--chain-id", fixtureChainID, "--keyring-backend", "test"},
		{"collect-gentxs"},
	} {
		if _, err = n.command(args...); err != nil {
			return nil, err
		}
	}
	if err = n.configure(); err != nil {
		return nil, err
	}
	return n, nil
}

func readJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, value); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func (n *node) configureGenesis() error {
	path := filepath.Join(n.home, "config", "genesis.json")
	var doc map[string]json.RawMessage
	if err := readJSON(path, &doc); err != nil {
		return err
	}
	var app map[string]json.RawMessage
	if err := json.Unmarshal(doc["app_state"], &app); err != nil {
		return err
	}
	var state seqtypes.GenesisState
	if err := json.Unmarshal(app[seqtypes.ModuleName], &state); err != nil {
		return err
	}
	if len(state.ReceiptKeys) != 1 || !bytes.Equal(state.ReceiptKeys[0].PublicKey[:], n.receiptPublicKey) {
		return errors.New("add-genesis-receipt-key did not pin the generated key")
	}
	key := state.ReceiptKeys[0]
	keys := seqkeeper.ReceiptKeys{{KeyId: key.KeyId[:], PublicKey: key.PublicKey[:], FirstHeight: key.FirstHeight, LastHeight: key.LastHeight}}
	if err := keys.Validate(); err != nil {
		return err
	}
	// The CLI writes types.GenesisState; the module consumes the snake_case wire form.
	var err error
	app[seqtypes.ModuleName], err = json.Marshal(map[string]any{"receipt_keys": keys, "activation_height": uint64(1)})
	if err != nil {
		return err
	}
	var staking struct {
		Params map[string]json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(app["staking"], &staking); err != nil {
		return err
	}
	staking.Params["max_voting_power_ratio"] = json.RawMessage(`"1.000000000000000000"`)
	var stakingState map[string]json.RawMessage
	if err := json.Unmarshal(app["staking"], &stakingState); err != nil {
		return err
	}
	stakingState["params"], err = json.Marshal(staking.Params)
	if err != nil {
		return err
	}
	app["staking"], err = json.Marshal(stakingState)
	if err != nil {
		return err
	}
	doc["app_state"], err = json.Marshal(app)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0600)
}

func (n *node) configure() error {
	listeners := make([]net.Listener, 0, 3)
	defer func() {
		for _, listener := range listeners {
			listener.Close()
		}
	}()
	for i := 0; i < 3; i++ {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return err
		}
		listeners = append(listeners, listener)
	}
	n.rpcURL = "http://" + listeners[0].Addr().String()
	if err := editTOML(filepath.Join(n.home, "config", "app.toml"), func(cfg map[string]any) {
		cfg["sequencer"] = map[string]any{
			"enable": true, "mode": "sequential", "role": "primary",
			"wal-dir": filepath.Join(n.home, "data", "sequencer-wal"), "receipt-key-file": n.receiptKeyFile,
			"wal-retain-blocks": int64(2), "nonce-queue-max-per-sender": int64(64),
			"nonce-queue-max-total": int64(1024), "lease-duration": "3s",
		}
		evm := cfg["evm"].(map[string]any)
		evm["http_enabled"], evm["http_address"] = true, "127.0.0.1"
		evm["http_port"], evm["ws_enabled"] = listeners[0].Addr().(*net.TCPAddr).Port, false
	}); err != nil {
		return err
	}
	return editTOML(filepath.Join(n.home, "config", "config.toml"), func(cfg map[string]any) {
		cfg["rpc"].(map[string]any)["laddr"] = "tcp://" + listeners[1].Addr().String()
		p2p := cfg["p2p"].(map[string]any)
		p2p["laddr"], p2p["seeds"], p2p["persistent-peers"], p2p["pex"] = "tcp://"+listeners[2].Addr().String(), "", "", false
	})
}

func editTOML(path string, edit func(map[string]any)) error {
	var cfg map[string]any
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return err
	}
	edit(cfg)
	var data bytes.Buffer
	if err := toml.NewEncoder(&data).Encode(cfg); err != nil {
		return err
	}
	return os.WriteFile(path, data.Bytes(), 0600)
}

func (n *node) start() (func(), error) {
	log, err := os.OpenFile(filepath.Join(n.home, "paxd.log"), os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(n.ctx, n.paxd, "start", "--home", n.home, "--chain-id", fixtureChainID)
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		log.Close()
		return nil, err
	}
	done := make(chan struct{})
	var processErr error
	go func() {
		processErr = cmd.Wait()
		close(done)
	}()
	stop := func() {
		if n.client != nil {
			n.client.Close()
		}
		cmd.Process.Signal(os.Interrupt)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			cmd.Process.Kill()
			<-done
		}
		log.Close()
	}
	n.client, err = rpc.DialContext(n.ctx, n.rpcURL)
	if err != nil {
		stop()
		return nil, err
	}
	ctx, cancel := context.WithTimeout(n.ctx, 60*time.Second)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	for {
		probe, cancel := context.WithTimeout(ctx, time.Second)
		var result json.RawMessage
		lastErr = n.client.CallContext(probe, &result, "pax_sequencerReceipt", common.Hash{})
		cancel()
		if lastErr == nil {
			return stop, nil
		}
		select {
		case <-done:
			stop()
			return nil, fmt.Errorf("paxd exited before pax_sequencerReceipt was ready: %v", processErr)
		case <-ctx.Done():
			stop()
			return nil, fmt.Errorf("wait for pax_sequencerReceipt: %w (last RPC error: %v)", ctx.Err(), lastErr)
		case <-ticker.C:
		}
	}
}
