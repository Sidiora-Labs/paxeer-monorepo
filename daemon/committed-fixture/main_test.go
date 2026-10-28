package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	paxdcmd "github.com/Sidiora-Labs/Paxeer-X-Network/daemon/paxd/cmd"
	seqmodule "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/params"
	seqconfig "github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	servercmd "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/cmd"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"
)

func TestMain(m *testing.M) {
	if os.Getenv("COMMITTED_FIXTURE_EXEC_PAXD") == "1" {
		params.SetAddressPrefixes()
		root, _ := paxdcmd.NewRootCmd()
		root.SetArgs(os.Args[1:])
		if err := servercmd.Execute(root, ""); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestRecorderHomePinsGeneratedReceiptKey(t *testing.T) {
	paxd, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("COMMITTED_FIXTURE_EXEC_PAXD", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var previousKey []byte
	var previousHome string
	for run := 0; run < 2; run++ {
		n, err := prepareHome(ctx, paxd)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.RemoveAll(n.home) })
		if n.home == previousHome {
			t.Fatal("recorder reused its previous home")
		}
		previousHome = n.home
		info, err := os.Stat(n.home)
		if err != nil || info.Mode().Perm() != 0700 {
			t.Fatalf("temporary home must be private: %v", err)
		}
		keyInfo, err := os.Stat(n.receiptKeyFile)
		if err != nil || keyInfo.Mode().Perm() != 0600 {
			t.Fatalf("receipt key must have mode 0600: %v", err)
		}
		seedHex, err := os.ReadFile(n.receiptKeyFile)
		if err != nil {
			t.Fatal(err)
		}
		seed, err := hex.DecodeString(strings.TrimSpace(string(seedHex)))
		if err != nil || len(seed) != ed25519.SeedSize {
			t.Fatal("receipt key file must contain an ed25519 seed")
		}
		private := ed25519.NewKeyFromSeed(seed)
		public := private.Public().(ed25519.PublicKey)
		if bytes.Equal(public, previousKey) {
			t.Fatal("recorder reused the receipt key across homes")
		}
		previousKey = bytes.Clone(public)
		if !bytes.Equal(public, n.receiptPublicKey) {
			t.Fatal("generated public key differs from the configured signing seed")
		}
		var genesis struct {
			ChainID  string                     `json:"chain_id"`
			AppState map[string]json.RawMessage `json:"app_state"`
		}
		if err := readJSON(filepath.Join(n.home, "config", "genesis.json"), &genesis); err != nil {
			t.Fatal(err)
		}
		if genesis.ChainID != fixtureChainID {
			t.Fatalf("wrong fixture chain id: %s", genesis.ChainID)
		}
		var state struct {
			ReceiptKeys      seqkeeper.ReceiptKeys `json:"receipt_keys"`
			Leaders          [][]byte              `json:"leaders"`
			ActivationHeight uint64                `json:"activation_height"`
		}
		if err := json.Unmarshal(genesis.AppState["sequencer"], &state); err != nil {
			t.Fatal(err)
		}
		if len(state.ReceiptKeys) != 1 {
			t.Fatalf("want exactly one generated receipt pin, got %d", len(state.ReceiptKeys))
		}
		pin := state.ReceiptKeys[0]
		keyID := sha256.Sum256(public)
		if !bytes.Equal(pin.PublicKey, public) || !bytes.Equal(pin.KeyId, keyID[:]) {
			t.Fatal("genesis does not pin the generated receipt public key and its SHA-256 id")
		}
		if pin.FirstHeight != 1 || pin.LastHeight != uint64(1<<63-1) || state.ActivationHeight != 1 {
			t.Fatalf("unexpected receipt key height range [%d, %d] or activation %d", pin.FirstHeight, pin.LastHeight, state.ActivationHeight)
		}
		if err := (seqmodule.AppModuleBasic{}).ValidateGenesis(nil, nil, genesis.AppState["sequencer"]); err != nil {
			t.Fatalf("prepared sequencer genesis must be accepted by the real module: %v", err)
		}
		var consensus struct {
			PubKey struct {
				Value []byte `json:"value"`
			} `json:"pub_key"`
		}
		if err := readJSON(filepath.Join(n.home, "config", "priv_validator_key.json"), &consensus); err != nil {
			t.Fatal(err)
		}
		if len(state.Leaders) != 1 || !bytes.Equal(state.Leaders[0], consensus.PubKey.Value) {
			t.Fatal("genesis does not pin this node's consensus key as the sole leader")
		}
		if bytes.Equal(consensus.PubKey.Value, public) {
			t.Fatal("receipt and consensus signing keys must be distinct")
		}
		app := viper.New()
		app.SetConfigFile(filepath.Join(n.home, "config", "app.toml"))
		if err := app.ReadInConfig(); err != nil {
			t.Fatal(err)
		}
		cfg, err := seqconfig.ReadConfig(app)
		if err != nil {
			t.Fatalf("prepared sequencer configuration must be usable: %v", err)
		}
		if !cfg.Enable || cfg.ReceiptKeyFile != n.receiptKeyFile || cfg.WALDir != filepath.Join(n.home, "data", "sequencer-wal") {
			t.Fatalf("incorrect sequencer paths or enable flag: %+v", cfg)
		}
		if cfg.Role != seqconfig.RolePrimary || cfg.StandbyAddress == "" || cfg.ReplicationTimeout <= 0 {
			t.Fatalf("primary must have a standby and a positive replication timeout: %+v", cfg)
		}
		if cfg.ReplicationTimeout != 5*time.Second {
			t.Fatalf("replication timeout must be 5s, got %s", cfg.ReplicationTimeout)
		}
		if n.standby == nil || n.standby.home == n.home {
			t.Fatal("primary must have a distinct prepared standby home")
		}
		standbyApp := viper.New()
		standbyApp.SetConfigFile(filepath.Join(n.standby.home, "config", "app.toml"))
		if err := standbyApp.ReadInConfig(); err != nil {
			t.Fatal(err)
		}
		standbyCfg, err := seqconfig.ReadConfig(standbyApp)
		if err != nil {
			t.Fatalf("prepared standby configuration must be usable: %v", err)
		}
		if !standbyCfg.Enable || standbyCfg.Role != seqconfig.RoleStandby ||
			standbyCfg.StandbyAddress != cfg.StandbyAddress || standbyCfg.ReplicationTimeout != cfg.ReplicationTimeout ||
			standbyCfg.WALDir != filepath.Join(n.standby.home, "data", "sequencer-wal") ||
			standbyCfg.ReceiptKeyFile != n.standby.receiptKeyFile {
			t.Fatalf("standby must replicate with its own paths and matching address: %+v", standbyCfg)
		}
		standbySeedHex, err := os.ReadFile(standbyCfg.ReceiptKeyFile)
		if err != nil {
			t.Fatal(err)
		}
		standbySeed, err := hex.DecodeString(strings.TrimSpace(string(standbySeedHex)))
		if err != nil || len(standbySeed) != ed25519.SeedSize {
			t.Fatal("standby receipt key file must contain an ed25519 seed")
		}
		standbyPrivate := ed25519.NewKeyFromSeed(standbySeed)
		standbyPublic := standbyPrivate.Public().(ed25519.PublicKey)
		if !bytes.Equal(standbyPublic, n.standby.receiptPublicKey) || bytes.Equal(standbyPublic, public) {
			t.Fatal("standby must use its own generated receipt key")
		}
		clear(standbySeed)
		clear(standbyPrivate)
		primaryGenesis, err := os.ReadFile(filepath.Join(n.home, "config", "genesis.json"))
		if err != nil {
			t.Fatal(err)
		}
		standbyGenesis, err := os.ReadFile(filepath.Join(n.standby.home, "config", "genesis.json"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(primaryGenesis, standbyGenesis) {
			t.Fatal("standby must copy the primary genesis")
		}
		primaryID, err := tmconfig.DefaultConfig().SetRoot(n.home).LoadNodeKeyID()
		if err != nil {
			t.Fatal(err)
		}
		var standbyTM struct {
			P2P struct {
				PersistentPeers string `toml:"persistent-peers"`
			} `toml:"p2p"`
		}
		if _, err := toml.DecodeFile(filepath.Join(n.standby.home, "config", "config.toml"), &standbyTM); err != nil {
			t.Fatal(err)
		}
		if standbyTM.P2P.PersistentPeers != string(primaryID)+"@"+n.p2pAddress {
			t.Fatal("standby must peer with the primary")
		}
		if n.rpcURL == n.standby.rpcURL || n.consensusAddress == n.standby.consensusAddress || n.p2pAddress == n.standby.p2pAddress {
			t.Fatal("standby must have its own EVM, RPC and P2P listeners")
		}
		if _, err := os.Stat(filepath.Join(n.standby.home, "paxd.log")); !os.IsNotExist(err) {
			t.Fatal("preparing the standby home must not start paxd")
		}
		if filepath.Dir(cfg.ReceiptKeyFile) != filepath.Join(n.home, "config") || !filepath.IsAbs(cfg.WALDir) {
			t.Fatal("sequencer paths must refer to this temporary home")
		}
		var tm struct {
			Mode string `toml:"mode"`
		}
		if _, err := toml.DecodeFile(filepath.Join(n.home, "config", "config.toml"), &tm); err != nil {
			t.Fatal(err)
		}
		if tm.Mode != "validator" {
			t.Fatalf("single-node sequencer must run as a validator, got %q", tm.Mode)
		}
		if _, err := os.Stat(filepath.Join(n.home, "paxd.log")); !os.IsNotExist(err) {
			t.Fatal("preparing the home must not start paxd")
		}
		clear(seed)
		clear(private)
	}
}

func TestRecorderHomeStartsSequencerReceiptRPC(t *testing.T) {
	paxd, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("COMMITTED_FIXTURE_EXEC_PAXD", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	n, err := prepareHome(ctx, paxd)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(n.home) })
	stop, err := n.start()
	if err != nil {
		for _, peer := range []*node{n, n.standby} {
			var tail bytes.Buffer
			peer.printLogTail(&tail)
			t.Log(tail.String())
		}
		t.Fatalf("start prepared sequencer home: %v", err)
	}
	t.Cleanup(stop)
	probe, cancelProbe := context.WithTimeout(ctx, time.Second)
	defer cancelProbe()
	var receipt json.RawMessage
	if err := n.client.CallContext(probe, &receipt, "pax_sequencerReceipt", common.Hash{}); err != nil {
		t.Fatalf("primary sequencer receipt RPC must answer after start: %v", err)
	}
	if string(receipt) != "null" {
		t.Fatalf("unknown transaction must have no receipt, got %s", receipt)
	}
	for _, peer := range []*node{n, n.standby} {
		if peer.processDone == nil {
			t.Fatal("both primary and standby must have started")
		}
		select {
		case <-peer.processDone:
			t.Fatalf("paxd in %s exited before stop", peer.home)
		default:
		}
		if info, err := os.Stat(filepath.Join(peer.home, "paxd.log")); err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("each paxd must have its own private log: %v", err)
		}
	}
	conn, err := net.DialTimeout("tcp", n.standbyAddress, time.Second)
	if err != nil {
		t.Fatalf("standby must serve the configured replication address: %v", err)
	}
	conn.Close()
	stop()
	for _, peer := range []*node{n, n.standby} {
		select {
		case <-peer.processDone:
		default:
			t.Fatalf("stop did not terminate paxd in %s", peer.home)
		}
	}
}
