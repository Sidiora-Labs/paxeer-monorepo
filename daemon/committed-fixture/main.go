package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"math/big"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
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
	p2pAddress       string
	consensusAddress string
	client           *rpc.Client
	ctx              context.Context
	isStandby        bool
	standby          *node
	standbyPrepared  bool
	bootstrapped     bool
	stopProcess      func()
	processDone      <-chan struct{}
	observer         bool
	standbyAddress   string
	gasPrice         *big.Int
	bootstrapHeight  int64
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

func run(ctx context.Context, args []string, stderr io.Writer) (err error) {
	if len(args) == 0 || args[0] != "record" {
		return errors.New("usage: committed-fixture record --paxd PATH --out DIRECTORY")
	}
	flags := flag.NewFlagSet("record", flag.ContinueOnError)
	flags.SetOutput(stderr)
	paxd := flags.String("paxd", "paxd", "paxd executable")
	out := flags.String("out", "", "directory for recorded JSON vectors")
	vector := flags.String("vector", "", "record only the named registered vector")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if *paxd == "" || *out == "" || flags.NArg() != 0 {
		return errors.New("record requires --paxd and --out and accepts no positional arguments")
	}
	if len(vectors) == 0 {
		return errors.New("no fixture vectors registered")
	}
	if *vector != "" && vectors[*vector] == nil {
		return fmt.Errorf("unknown fixture vector: %s", *vector)
	}
	n, err := prepareHome(ctx, *paxd)
	if err != nil {
		return err
	}
	defer os.RemoveAll(n.home)
	defer func() {
		if err != nil {
			n.printLogTail(stderr)
		}
	}()
	if *vector == vectorV6Name {
		if err := n.configureV6Genesis(); err != nil {
			return err
		}
	}
	standby, err := n.prepareStandby()
	if err != nil {
		return err
	}
	defer os.RemoveAll(standby.home)
	if *vector == "" || *vector == vectorV2Name || *vector == v1ProgramsName {
		if err := n.configureV2Genesis(); err != nil {
			return err
		}
	}
	defer func() {
		if err != nil {
			standby.printLogTail(stderr)
		}
	}()
	observer, err := n.prepareObserver(standby)
	if err != nil {
		return err
	}
	defer os.RemoveAll(observer.home)
	defer func() {
		if err != nil {
			observer.printLogTail(stderr)
		}
	}()
	if err := n.bootstrap(standby, observer); err != nil {
		return err
	}
	stopObserver, err := observer.start()
	if err != nil {
		return err
	}
	defer stopObserver()
	stopStandby, err := standby.start()
	if err != nil {
		return err
	}
	defer stopStandby()
	if err := n.prime(); err != nil {
		return err
	}
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
		if *vector == "" || name == *vector {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		value, err := vectors[name](n)
		if err != nil {
			return fmt.Errorf("record %s: %w", name, err)
		}
		if name == v1ProgramsName {
			v, ok := value.(*v1ProgramsVector)
			if !ok {
				return errors.New("V1 Programs recorder returned another vector type")
			}
			if err := writeV1ProgramsCopies(*out, v); err != nil {
				return err
			}
			continue
		}
		if name == vectorV2Name {
			v, ok := value.(*vectorV2)
			if !ok {
				return errors.New("V2 recorder returned another vector type")
			}
			if err := writeVectorV2Copies(*out, v); err != nil {
				return err
			}
			continue
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
	n, err := prepareNodeHome(ctx, paxd, false, "")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(n.home)
		}
	}()
	n.standby, err = prepareNodeHome(ctx, paxd, true, n.home)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(n.receiptPublicKey, n.standby.receiptPublicKey) {
		return nil, errors.New("standby requires its own receipt key")
	}
	genesis, err := os.ReadFile(filepath.Join(n.home, "config", "genesis.json"))
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(n.standby.home, "config", "genesis.json"), genesis, 0600); err != nil {
		return nil, err
	}
	if err := n.configure(); err != nil {
		return nil, err
	}
	if err := n.configureReplicationTLS(); err != nil {
		return nil, err
	}
	return n, nil
}

func prepareNodeHome(ctx context.Context, paxd string, standby bool, parent string) (_ *node, err error) {
	home, err := os.MkdirTemp(parent, "pax-committed-fixture-")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(home)
		}
	}()
	n := &node{home: home, paxd: paxd, ctx: ctx, isStandby: standby, receiptKeyFile: filepath.Join(home, "config", "receipt-key.hex")}
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
	if standby {
		return n, nil
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
	var state struct {
		ReceiptKeys seqkeeper.ReceiptKeys `json:"receipt_keys"`
	}
	if err := json.Unmarshal(app[seqtypes.ModuleName], &state); err != nil {
		return err
	}
	if len(state.ReceiptKeys) != 1 || !bytes.Equal(state.ReceiptKeys[0].PublicKey, n.receiptPublicKey) {
		return errors.New("add-genesis-receipt-key did not pin the generated key")
	}
	keys := state.ReceiptKeys
	if err := keys.Validate(); err != nil {
		return err
	}
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
	listeners := make([]net.Listener, 2, 3)
	defer func() {
		for _, listener := range listeners {
			if listener != nil {
				listener.Close()
			}
		}
	}()
	for i := 2; i < 3; i++ {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return err
		}
		listeners = append(listeners, listener)
	}
	n.rpcURL = ""
	n.p2pAddress = listeners[2].Addr().String()
	n.consensusAddress = "127.0.0.4:0"
	if n.standby != nil {
		n.standbyAddress = "127.0.0.2:0"
		n.standby.standbyAddress = n.standbyAddress
	}
	role := "primary"
	if n.isStandby {
		role = "standby"
	}
	if err := editTOML(filepath.Join(n.home, "config", "app.toml"), func(cfg map[string]any) {
		cfg["sequencer"] = map[string]any{
			"enable": !n.observer, "mode": "sequential", "role": role,
			"wal-dir": filepath.Join(n.home, "data", "sequencer-wal"), "receipt-key-file": n.receiptKeyFile,
			"standby-address": n.standbyAddress, "replication-timeout": "5s",
			"wal-retain-blocks": int64(2), "nonce-queue-max-per-sender": int64(64),
			"nonce-queue-max-total": int64(1024), "lease-duration": "3s",
		}
		evm := cfg["evm"].(map[string]any)
		evm["http_enabled"], evm["http_address"] = !n.observer, "127.0.0.3"
		evm["http_port"], evm["ws_enabled"] = 0, false
		grpc := cfg["grpc"].(map[string]any)
		grpc["enable"], grpc["address"] = true, "127.0.0.1:0"
		cfg["grpc-web"].(map[string]any)["enable"] = false
		cfg["api"].(map[string]any)["address"] = "tcp://127.0.0.1:0"
	}); err != nil {
		return err
	}
	if err := editTOML(filepath.Join(n.home, "config", "config.toml"), func(cfg map[string]any) {
		if n.observer {
			cfg["mode"] = "full"
		}
		cfg["rpc"].(map[string]any)["laddr"] = "tcp://127.0.0.4:0"
		cfg["consensus"].(map[string]any)["unsafe-commit-timeout-override"] = "10s"
		p2p := cfg["p2p"].(map[string]any)
		p2p["allow-duplicate-ip"] = true
		p2p["laddr"], p2p["seeds"], p2p["persistent-peers"], p2p["pex"] = "tcp://"+listeners[2].Addr().String(), "", "", false
	}); err != nil {
		return err
	}
	if n.standby != nil {
		if err := n.standby.configure(); err != nil {
			return err
		}
		primaryID, err := tmconfig.DefaultConfig().SetRoot(n.home).LoadNodeKeyID()
		if err != nil {
			return err
		}
		return editTOML(filepath.Join(n.standby.home, "config", "config.toml"), func(cfg map[string]any) {
			cfg["p2p"].(map[string]any)["persistent-peers"] = string(primaryID) + "@" + n.p2pAddress
		})
	}
	return nil
}

func (n *node) prepareStandby() (_ *node, err error) {
	s := n.standby
	if s == nil {
		return nil, errors.New("primary has no prepared standby home")
	}
	if n.standbyPrepared {
		return s, nil
	}
	var validator struct {
		Address string `json:"address"`
		PubKey  struct {
			Type  string `json:"type"`
			Value []byte `json:"value"`
		} `json:"pub_key"`
	}
	if err := readJSON(filepath.Join(s.home, "config", "priv_validator_key.json"), &validator); err != nil {
		return nil, err
	}
	if len(validator.PubKey.Value) != 32 || bytes.Equal(s.receiptPublicKey, n.receiptPublicKey) {
		return nil, errors.New("standby requires its own consensus and receipt keys")
	}
	if err := n.addStandbyGenesisValidator(s); err != nil {
		return nil, err
	}
	var doc map[string]json.RawMessage
	if err := readJSON(filepath.Join(n.home, "config", "genesis.json"), &doc); err != nil {
		return nil, err
	}
	var primaryValidator struct {
		Address string          `json:"address"`
		PubKey  json.RawMessage `json:"pub_key"`
	}
	if err := readJSON(filepath.Join(n.home, "config", "priv_validator_key.json"), &primaryValidator); err != nil {
		return nil, err
	}
	doc["validators"], err = json.Marshal([]map[string]any{
		{
			"address": primaryValidator.Address, "pub_key": primaryValidator.PubKey,
			"power": "7000000000", "name": "committed-fixture",
		},
		{
			"address": validator.Address, "pub_key": map[string]any{
				"type": validator.PubKey.Type, "value": validator.PubKey.Value,
			},
			"power": "1", "name": "committed-fixture-standby",
		},
	})
	if err != nil {
		return nil, err
	}
	var app map[string]json.RawMessage
	if err := json.Unmarshal(doc["app_state"], &app); err != nil {
		return nil, err
	}
	var state struct {
		Leaders          [][]byte              `json:"leaders"`
		ReceiptKeys      seqkeeper.ReceiptKeys `json:"receipt_keys"`
		ActivationHeight uint64                `json:"activation_height"`
	}
	if err := json.Unmarshal(app[seqtypes.ModuleName], &state); err != nil {
		return nil, err
	}
	if len(state.Leaders) != 1 || len(state.ReceiptKeys) != 1 || bytes.Equal(state.Leaders[0], validator.PubKey.Value) {
		return nil, errors.New("standby requires a distinct pinned primary leader and receipt key")
	}
	state.Leaders = append(state.Leaders, validator.PubKey.Value)
	state.ActivationHeight = 2
	id := sha256.Sum256(s.receiptPublicKey)
	state.ReceiptKeys = append(state.ReceiptKeys, seqkeeper.ReceiptKeyEntry{
		KeyId: id[:], PublicKey: s.receiptPublicKey, FirstHeight: 1, LastHeight: uint64(math.MaxInt64),
	})
	if err := state.ReceiptKeys.Validate(); err != nil {
		return nil, err
	}
	app[seqtypes.ModuleName], err = json.Marshal(state)
	if err != nil {
		return nil, err
	}
	doc["app_state"], err = json.Marshal(app)
	if err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	for _, peer := range []*node{n, s} {
		if err := os.WriteFile(filepath.Join(peer.home, "config", "genesis.json"), append(data, '\n'), 0600); err != nil {
			return nil, err
		}
	}
	n.standbyPrepared = true
	return s, nil
}

func (n *node) addStandbyGenesisValidator(s *node) error {
	output, err := s.command("keys", "add", "fixture-standby", "--keyring-backend", "test", "--output", "json")
	if err != nil {
		return err
	}
	var account struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal(output, &account); err != nil {
		return err
	}
	if _, err := n.command("add-genesis-account", account.Address, "1000000uhpx"); err != nil {
		return err
	}
	genesis, err := os.ReadFile(filepath.Join(n.home, "config", "genesis.json"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(s.home, "config", "genesis.json"), genesis, 0600); err != nil {
		return err
	}
	if _, err := s.command("gentx", "fixture-standby", "1000000uhpx", "--chain-id", fixtureChainID, "--keyring-backend", "test"); err != nil {
		return err
	}
	gentxDir := filepath.Join(s.home, "config", "gentx")
	entries, err := os.ReadDir(gentxDir)
	if err != nil {
		return err
	}
	if len(entries) != 1 || entries[0].IsDir() {
		return errors.New("standby must generate exactly one genesis validator transaction")
	}
	gentx, err := os.ReadFile(filepath.Join(gentxDir, entries[0].Name()))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(n.home, "config", "gentx", entries[0].Name()), gentx, 0600); err != nil {
		return err
	}
	_, err = n.command("collect-gentxs")
	return err
}

func (n *node) prepareObserver(standby *node) (_ *node, err error) {
	o, err := prepareNodeHome(n.ctx, n.paxd, true, "")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(o.home)
		}
	}()
	o.observer = true
	genesis, err := os.ReadFile(filepath.Join(n.home, "config", "genesis.json"))
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(o.home, "config", "genesis.json"), genesis, 0600); err != nil {
		return nil, err
	}
	if err := o.configure(); err != nil {
		return nil, err
	}
	peers := []*node{n, standby, o}
	addresses := make([]string, len(peers))
	for i, peer := range peers {
		id, err := tmconfig.DefaultConfig().SetRoot(peer.home).LoadNodeKeyID()
		if err != nil {
			return nil, err
		}
		addresses[i] = string(id) + "@" + peer.p2pAddress
	}
	for i, peer := range peers[1:] {
		own := i + 1
		var others []string
		for j, address := range addresses {
			if j != own {
				others = append(others, address)
			}
		}
		if err := editTOML(filepath.Join(peer.home, "config", "config.toml"), func(cfg map[string]any) {
			cfg["p2p"].(map[string]any)["persistent-peers"] = strings.Join(others, ",")
		}); err != nil {
			return nil, err
		}
	}
	return o, nil
}

func (n *node) bootstrap(peers ...*node) error {
	ctx, cancel := context.WithTimeout(n.ctx, 60*time.Second)
	defer cancel()
	// A multi-validator genesis enters block sync until two peers report their
	// heights. Keep both peers serving until the primary has halted, rather
	// than requiring them to receive the primary's commit before it exits.
	var stopOnce sync.Once
	var peerStops []func()
	stopPeers := func() {
		stopOnce.Do(func() {
			for _, stop := range peerStops {
				stop()
			}
		})
	}
	defer stopPeers()
	for _, peer := range peers {
		log, err := os.OpenFile(filepath.Join(peer.home, "bootstrap.log"), os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		cmd := exec.CommandContext(ctx, peer.paxd, "start", "--home", peer.home, "--chain-id", fixtureChainID,
			"--sequencer.enable=false")
		cmd.Stdout, cmd.Stderr = log, log
		if err := cmd.Start(); err != nil {
			log.Close()
			return err
		}
		done := make(chan struct{})
		go func() {
			cmd.Wait()
			close(done)
		}()
		peerStops = append(peerStops, func() {
			cmd.Process.Signal(os.Interrupt)
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				cmd.Process.Kill()
				<-done
			}
			log.Close()
		})
	}
	logPath := filepath.Join(n.home, "bootstrap.log")
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	height := n.bootstrapHeight
	if height == 0 {
		height = 1
	}
	cmd := exec.CommandContext(ctx, n.paxd, "start", "--home", n.home, "--chain-id", fixtureChainID,
		"--sequencer.enable=false", "--halt-height", fmt.Sprint(height))
	cmd.Stdout, cmd.Stderr = log, log
	err = cmd.Run()
	log.Close()
	stopPeers()
	if err != nil {
		data, readErr := os.ReadFile(logPath)
		return fmt.Errorf("bootstrap fixture chain: %w (read log: %v)\n%s", err, readErr, data)
	}
	entries, err := os.ReadDir(filepath.Join(n.home, "data"))
	if err != nil {
		return err
	}
	// The primary alone has quorum and halts at the bootstrap height. Copy its bootstrap
	// state so peers cannot miss the commit when the primary shuts down.
	for _, peer := range peers {
		for _, entry := range entries {
			if entry.Name() == "priv_validator_state.json" {
				continue
			}
			src := filepath.Join(n.home, "data", entry.Name())
			dst := filepath.Join(peer.home, "data", entry.Name())
			if entry.IsDir() {
				if err := os.RemoveAll(dst); err != nil {
					return err
				}
				if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
					return err
				}
			} else {
				data, err := os.ReadFile(src)
				if err != nil {
					return err
				}
				if err := os.WriteFile(dst, data, 0600); err != nil {
					return err
				}
			}
		}
	}
	n.bootstrapped = true
	return nil
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

func (n *node) start() (_ func(), err error) {
	if n.stopProcess != nil {
		return nil, errors.New("paxd already started")
	}
	var stopStandby, stopObserver func()
	defer func() {
		if err != nil && stopObserver != nil {
			stopObserver()
		}
	}()
	if n.standby != nil {
		if !n.bootstrapped {
			if _, err := n.prepareStandby(); err != nil {
				return nil, err
			}
			observer, err := n.prepareObserver(n.standby)
			if err != nil {
				return nil, err
			}
			stopObserver = func() { os.RemoveAll(observer.home) }
			if err := n.bootstrap(n.standby, observer); err != nil {
				return nil, err
			}
			stop, err := observer.start()
			if err != nil {
				return nil, err
			}
			stopObserver = func() {
				stop()
				os.RemoveAll(observer.home)
			}
		}
		stopStandby = n.standby.stopProcess
		if stopStandby == nil {
			stopStandby, err = n.standby.start()
			if err != nil {
				return nil, err
			}
		}
		defer func() {
			if err != nil {
				stopStandby()
			}
		}()
		if n.gasPrice == nil {
			if err := n.prime(); err != nil {
				return nil, fmt.Errorf("prime prepared sequencer home: %w", err)
			}
		}
	}
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
	n.processDone = done
	var processErr error
	go func() {
		processErr = cmd.Wait()
		close(done)
	}()
	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() {
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
			if stopStandby != nil {
				stopStandby()
			}
			if stopObserver != nil {
				stopObserver()
			}
		})
	}
	ctx, cancel := context.WithTimeout(n.ctx, 60*time.Second)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	for {
		probe, cancel := context.WithTimeout(ctx, time.Second)
		if n.isStandby || n.observer {
			address := n.standbyAddress
			if n.isStandby {
				address, lastErr = processBoundTCPAddress(cmd.Process.Pid, "127.0.0.2")
				if lastErr == nil {
					n.standbyAddress = address
				}
			}
			if n.observer {
				address, lastErr = processBoundTCPAddress(cmd.Process.Pid, "127.0.0.4")
				if lastErr == nil {
					n.consensusAddress = address
				}
			}
			var conn net.Conn
			conn, lastErr = (&net.Dialer{}).DialContext(probe, "tcp", address)
			if lastErr == nil {
				conn.Close()
			}
		} else {
			if n.client == nil {
				var address string
				address, lastErr = processBoundTCPAddress(cmd.Process.Pid, "127.0.0.3")
				if lastErr == nil {
					n.rpcURL = "http://" + address
				}
				if lastErr == nil {
					n.client, lastErr = rpc.DialContext(probe, n.rpcURL)
				}
			}
			var result json.RawMessage
			if n.client != nil {
				lastErr = n.client.CallContext(probe, &result, "pax_sequencerReceipt", common.Hash{})
			}
		}
		cancel()
		if lastErr == nil {
			address, e := processBoundTCPAddress(cmd.Process.Pid, "127.0.0.4")
			if e == nil {
				n.consensusAddress = address
				n.stopProcess = stop
				return stop, nil
			}
			lastErr = e
		}
		select {
		case <-done:
			stop()
			return nil, fmt.Errorf("paxd exited before readiness: %v (last RPC error: %v)", processErr, lastErr)
		case <-ctx.Done():
			stop()
			return nil, fmt.Errorf("wait for pax_sequencerReceipt: %w (last RPC error: %v)", ctx.Err(), lastErr)
		case <-ticker.C:
		}
	}
}

func (n *node) printLogTail(stderr io.Writer) {
	path := filepath.Join(n.home, "paxd.log")
	log, err := os.Open(path)
	if err != nil {
		return
	}
	defer log.Close()
	info, err := log.Stat()
	if err != nil {
		fmt.Fprintf(stderr, "stat recorder log %s: %v\n", path, err)
		return
	}
	const tailBytes = 16 * 1024
	if info.Size() > tailBytes {
		// A halt may precede the tail by several rounds of consensus logging.
		scanner := bufio.NewScanner(io.NewSectionReader(log, 0, info.Size()-tailBytes))
		scanner.Buffer(make([]byte, 4096), 1<<20)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "standby refused") || strings.Contains(line, "standby stopped") || strings.Contains(line, "fences this primary") {
				fmt.Fprintf(stderr, "recorder replication failure %s: %s\n", path, line)
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(stderr, "scan recorder log %s: %v\n", path, err)
		}
		if _, err := log.Seek(-tailBytes, io.SeekEnd); err != nil {
			fmt.Fprintf(stderr, "seek recorder log %s: %v\n", path, err)
			return
		}
	}
	fmt.Fprintf(stderr, "recorder log tail %s:\n", path)
	if _, err := io.Copy(stderr, log); err != nil {
		fmt.Fprintf(stderr, "read recorder log %s: %v\n", path, err)
	}
}

// A standby process owns its port-zero socket before its primary learns the
// address. Match socket ownership, not an unowned entry in the global TCP table.
func processBoundTCPAddress(pid int, host string) (string, error) {
	descriptors, err := os.ReadDir(fmt.Sprintf("/proc/%d/fd", pid))
	if err != nil {
		return "", err
	}
	owned := make(map[string]bool)
	for _, descriptor := range descriptors {
		target, err := os.Readlink(fmt.Sprintf("/proc/%d/fd/%s", pid, descriptor.Name()))
		if err == nil && strings.HasPrefix(target, "socket:[") {
			owned[strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")] = true
		}
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/net/tcp", pid))
	if err != nil {
		return "", err
	}
	var addresses []string
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 10 || fields[3] != "0A" || !owned[fields[9]] {
			continue
		}
		local := strings.Split(fields[1], ":")
		if len(local) != 2 {
			continue
		}
		ipBytes, err := hex.DecodeString(local[0])
		if err != nil || len(ipBytes) != 4 {
			continue
		}
		ip := net.IPv4(ipBytes[3], ipBytes[2], ipBytes[1], ipBytes[0]).String()
		if ip != host {
			continue
		}
		port, err := strconv.ParseUint(local[1], 16, 16)
		if err != nil || port == 0 {
			continue
		}
		addresses = append(addresses, net.JoinHostPort(ip, fmt.Sprint(port)))
	}
	if len(addresses) != 1 {
		return "", fmt.Errorf("process %d owns %d listening sockets on %s; expected one", pid, len(addresses), host)
	}
	return addresses[0], nil
}
