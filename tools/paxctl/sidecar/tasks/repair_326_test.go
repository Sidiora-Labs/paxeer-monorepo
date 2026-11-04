package tasks

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/sei-protocol/sei-chain/sei-cosmos/codec"
	"github.com/sei-protocol/sei-chain/sei-cosmos/crypto/keys/secp256k1"
	"github.com/sei-protocol/sei-chain/sei-cosmos/store"
	sdk "github.com/sei-protocol/sei-chain/sei-cosmos/types"
	authmodule "github.com/sei-protocol/sei-chain/sei-cosmos/x/auth"
	"github.com/sei-protocol/sei-chain/sei-cosmos/x/auth/ante"
	authkeeper "github.com/sei-protocol/sei-chain/sei-cosmos/x/auth/keeper"
	authsigning "github.com/sei-protocol/sei-chain/sei-cosmos/x/auth/signing"
	authtypes "github.com/sei-protocol/sei-chain/sei-cosmos/x/auth/types"
	vestingtypes "github.com/sei-protocol/sei-chain/sei-cosmos/x/auth/vesting/types"
	bankkeeper "github.com/sei-protocol/sei-chain/sei-cosmos/x/bank/keeper"
	banktypes "github.com/sei-protocol/sei-chain/sei-cosmos/x/bank/types"
	"github.com/sei-protocol/sei-chain/sei-cosmos/x/genutil"
	genutiltypes "github.com/sei-protocol/sei-chain/sei-cosmos/x/genutil/types"
	govtypes "github.com/sei-protocol/sei-chain/sei-cosmos/x/gov/types"
	paramskeeper "github.com/sei-protocol/sei-chain/sei-cosmos/x/params/keeper"
	paramstypes "github.com/sei-protocol/sei-chain/sei-cosmos/x/params/types"
	stakingmodule "github.com/sei-protocol/sei-chain/sei-cosmos/x/staking"
	stakingkeeper "github.com/sei-protocol/sei-chain/sei-cosmos/x/staking/keeper"
	stakingtypes "github.com/sei-protocol/sei-chain/sei-cosmos/x/staking/types"
	abci "github.com/sei-protocol/sei-chain/sei-tendermint/abci/types"
	tmproto "github.com/sei-protocol/sei-chain/sei-tendermint/proto/tendermint/types"
	coretypes "github.com/sei-protocol/sei-chain/sei-tendermint/rpc/coretypes"
	rpcserver "github.com/sei-protocol/sei-chain/sei-tendermint/rpc/jsonrpc/server"
	tmtypes "github.com/sei-protocol/sei-chain/sei-tendermint/types"
	dbm "github.com/tendermint/tm-db"

	"github.com/sei-protocol/seictl/sidecar/engine"
	"github.com/sei-protocol/seictl/sidecar/rpc"
	seis3 "github.com/sei-protocol/seictl/sidecar/s3"
)

const repairChainID = "repair-326"
const repairSignState = `{"height":"987","round":2,"step":3,"signature":"retained"}`

// The transport uses the real AWS client and transfer manager against a local
// filesystem HTTP service. All genesis and identity objects come from the real
// ceremony producers below, including their actual serialized bytes.
func repairObjectTransport(t *testing.T) (string, S3ClientFactory, seis3.UploaderFactory, *httptest.Server) {
	t.Helper()
	root := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			name := strings.TrimPrefix(r.URL.Path, "/")
			if !filepath.IsLocal(name) {
				http.Error(w, "invalid path", 400)
				return
			}
			path := filepath.Join(root, name)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			_, copyErr := io.Copy(f, r.Body)
			closeErr := f.Close()
			if copyErr != nil || closeErr != nil {
				http.Error(w, "write failed", 500)
				return
			}
			w.Header().Set("ETag", `"local-file"`)
			w.WriteHeader(200)
			return
		}
		http.FileServer(http.Dir(root)).ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	client := awss3.NewFromConfig(aws.Config{Region: "local", Credentials: aws.AnonymousCredentials{}, HTTPClient: srv.Client()}, func(o *awss3.Options) {
		o.BaseEndpoint = aws.String(srv.URL)
		o.UsePathStyle = true
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})
	return root, func(context.Context, string) (S3GetObjectAPI, error) { return client, nil }, func(context.Context, string) (seis3.Uploader, error) { return transfermanager.New(client), nil }, srv
}

func repairWaitResult(t *testing.T, e *engine.Engine, id string) *engine.TaskResult {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		result := e.GetResult(id)
		if result != nil && result.Status != engine.TaskStatusRunning {
			return result
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("task did not finish")
	return nil
}

func repairHeldStore(t *testing.T, home string) {
	t.Helper()
	s, err := engine.NewSQLiteStore(filepath.Join(home, "sidecar.db"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e := engine.NewEngine(ctx, map[engine.TaskType]engine.TaskHandler{
		engine.TaskMarkNotReady: NewMarkNotReadier(s).Handler(),
		engine.TaskStopSeid:     NewStopSeider().Handler(),
	}, s)
	for _, typ := range []engine.TaskType{engine.TaskMarkNotReady, engine.TaskStopSeid} {
		id := uuid.NewString()
		if _, err := e.Submit(engine.Task{ID: id, Type: typ}); err != nil {
			t.Fatal(err)
		}
		if result := repairWaitResult(t, e, id); result.Status != engine.TaskStatusCompleted {
			t.Fatalf("hold/stop failed: %+v", result)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// The status and block responses are serialized using the node's real JSON
// contracts and independently generated public node identities.
func repairWitness(t *testing.T, chainID, hash string, height int64) *httptest.Server {
	t.Helper()
	id := tmtypes.GenNodeKey().ID
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/status":
			result := rpc.StatusResult{NodeInfo: rpc.NodeInfo{ID: string(id), Network: chainID}, SyncInfo: rpc.SyncInfo{LatestBlockHeight: fmt.Sprint(height)}}
			if err := json.NewEncoder(w).Encode(result); err != nil {
				t.Error(err)
			}
		case "/block":
			result := struct {
				BlockID rpc.BlockID `json:"block_id"`
				Block   struct {
					Header struct {
						ChainID string `json:"chain_id"`
						Height  string `json:"height"`
					} `json:"header"`
				} `json:"block"`
			}{}
			result.BlockID.Hash = hash
			result.Block.Header.ChainID = chainID
			result.Block.Header.Height = r.URL.Query().Get("height")
			if err := json.NewEncoder(w).Encode(result); err != nil {
				t.Error(err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func repairWitnessPair(t *testing.T, home string) []string {
	t.Helper()
	hash := fmt.Sprintf("%X", sha256.Sum256([]byte("independently witnessed trust point")))
	first := repairWitness(t, repairChainID, hash, 5000)
	second := repairWitness(t, repairChainID, hash, 5000)
	eps := []string{strings.TrimPrefix(first.URL, "http://"), strings.TrimPrefix(second.URL, "http://")}
	peers := []string{"one@" + eps[0], "two@" + eps[1]}
	setupConfigFile(t, home, "mode = \"full\"\n[p2p]\npersistent-peers = \""+strings.Join(peers, ",")+"\"\n")
	// Explicit witness ports are part of the pre-reset request contract. Existing
	// peer-derived witnesses retain their documented fixed RPC port.
	return eps
}

func repairResetHome(t *testing.T, home string) []string {
	t.Helper()
	t.Setenv("SEI_CHAIN_ID", repairChainID)
	eps := repairWitnessPair(t, home)
	doc := &tmtypes.GenesisDoc{ChainID: repairChainID, GenesisTime: time.Now().UTC(), ConsensusParams: tmtypes.DefaultConsensusParams(), AppState: json.RawMessage(`{}`)}
	if err := genutil.ExportGenesisFile(doc, filepath.Join(home, "config", "genesis.json")); err != nil {
		t.Fatal(err)
	}
	repairHeldStore(t, home)
	return eps
}

func TestRepair_326_ResetLiveOrValidatorState(t *testing.T) {
	home := t.TempDir()
	eps := repairResetHome(t, home)
	mustWrite(t, filepath.Join(home, "data", privValidatorStateFile), repairSignState)
	mustWrite(t, filepath.Join(home, "data", "application.db", "state"), "chain state")
	d := NewResetDataer(home)
	// Neither unreachable RPC nor a forged request substitutes for persisted hold.
	s, err := engine.NewSQLiteStore(filepath.Join(home, "sidecar.db"))
	if err != nil {
		t.Fatal(err)
	}
	hold, err := s.LatestByType(string(engine.TaskMarkNotReady))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Delete(hold.ID); err != nil {
		t.Fatal(err)
	}
	s.Close()
	params := map[string]any{"rpcServers": eps}
	if _, err := d.Handler()(context.Background(), params); err == nil {
		t.Fatal("reset without durable hold succeeded")
	}
	if !exists(filepath.Join(home, "data", "application.db", "state")) {
		t.Fatal("refusal erased chain data")
	}
	repairHeldStore(t, home)
	if _, err := d.Handler()(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(home, "data", "application.db")) {
		t.Fatal("chain data survived authorized reset")
	}
	for _, path := range []string{filepath.Join(home, "data", privValidatorStateFile), filepath.Join(home, preservedSignStateFile)} {
		b, err := os.ReadFile(path)
		if err != nil || string(b) != repairSignState {
			t.Fatalf("last-sign state changed at %s: %q %v", path, b, err)
		}
	}
	// Reopen the durable ledger and the reset handler, with last-sign state lost
	// from data/ in the intervening crash, and recover the exact original bytes.
	if err := os.Remove(filepath.Join(home, "data", privValidatorStateFile)); err != nil {
		t.Fatal(err)
	}
	d = NewResetDataer(home)
	if _, err := d.Handler()(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(home, "data", privValidatorStateFile))
	if err != nil || string(b) != repairSignState {
		t.Fatalf("restart lost anti-double-sign state: %q %v", b, err)
	}
	mustWrite(t, filepath.Join(home, "config", "config.toml"), "mode = \"validator\"\n")
	mustWrite(t, filepath.Join(home, "data", "sentinel"), "retained")
	if _, err := d.Handler()(context.Background(), params); err == nil {
		t.Fatal("validator reset accepted")
	}
	if !exists(filepath.Join(home, "data", "sentinel")) {
		t.Fatal("validator data erased")
	}
	// A full-mode node whose consensus identity occurs in genesis is also refused.
	setupConfigFile(t, home, "mode = \"full\"\n")
	key := tmtypes.GenNodeKey()
	pub := key.PubKey()
	doc := &tmtypes.GenesisDoc{ChainID: repairChainID, GenesisTime: time.Now().UTC(), ConsensusParams: tmtypes.DefaultConsensusParams(), AppState: json.RawMessage(`{}`), Validators: []tmtypes.GenesisValidator{{Address: pub.Address(), PubKey: pub, Power: 1}}}
	if err := genutil.ExportGenesisFile(doc, filepath.Join(home, "config", "genesis.json")); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(home, "config", "priv_validator_key.json"), fmt.Sprintf(`{"address":%q}`, pub.Address().String()))
	if _, err := d.Handler()(context.Background(), params); err == nil {
		t.Fatal("active validator identity accepted")
	}
}

func TestRepair_326_WitnessDuplication(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SEI_CHAIN_ID", repairChainID)
	eps := repairWitnessPair(t, home)
	p := StateSyncRequest{RpcServers: eps}
	c := NewStateSyncConfigurer(home, nil)
	if err := c.Configure(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	ss := readTOML(t, filepath.Join(home, "config", "config.toml"))["statesync"].(map[string]any)
	if ss["rpc-servers"] != strings.Join(eps, ",") || ss["trust-height"] != int64(3000) {
		t.Fatalf("wrong quorum config: %+v", ss)
	}
	for _, bad := range [][]string{{eps[0]}, {eps[0], eps[0]}, {eps[0], "127.0.0.1:1"}} {
		if err := c.Configure(context.Background(), StateSyncRequest{RpcServers: bad}); err == nil {
			t.Fatalf("under-witnessed request accepted on marker replay: %v", bad)
		}
	}
	different := repairWitness(t, repairChainID, strings.Repeat("a", 64), 5000)
	wrongChain := repairWitness(t, "another-chain", strings.Repeat("a", 64), 5000)
	for _, server := range []*httptest.Server{different, wrongChain} {
		if _, err := c.Preflight(context.Background(), StateSyncRequest{RpcServers: []string{eps[0], strings.TrimPrefix(server.URL, "http://")}}); err == nil {
			t.Fatal("mismatched witness accepted")
		}
	}
	// Distinct addresses serving the same public node identity are one authority.
	original := repairWitness(t, repairChainID, strings.Repeat("a", 64), 5000)
	duplicate := httptest.NewServer(original.Config.Handler)
	defer duplicate.Close()
	if _, err := c.Preflight(context.Background(), StateSyncRequest{RpcServers: []string{strings.TrimPrefix(original.URL, "http://"), strings.TrimPrefix(duplicate.URL, "http://")}}); err == nil {
		t.Fatal("duplicate public witness identity accepted")
	}
	// Run the destructive consumer with a valid durable hold, but one witness.
	resetHome := t.TempDir()
	repairResetHome(t, resetHome)
	mustWrite(t, filepath.Join(resetHome, "data", "sentinel"), "untouched")
	if _, err := NewResetDataer(resetHome).Handler()(context.Background(), map[string]any{"rpcServers": []string{eps[0]}}); err == nil {
		t.Fatal("reset erased before establishing quorum")
	}
	if !exists(filepath.Join(resetHome, "data", "sentinel")) {
		t.Fatal("failed preflight changed data")
	}
}

func TestRepair_326_ConfigPathTraversal(t *testing.T) {
	home := t.TempDir()
	path := setupConfigFile(t, home, "mode = \"full\"\n")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	patch := map[string]any{"mode": "validator"}
	for _, name := range []string{"../outside.toml", "/tmp/outside.toml", "sub/config.toml", "..\\config.toml", "unknown.toml"} {
		_, err := NewConfigPatcher(home).Handler()(context.Background(), map[string]any{"files": map[string]any{"config.toml": patch, name: patch}})
		if err == nil {
			t.Fatalf("invalid path accepted: %q", name)
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(after, original) {
			t.Fatalf("invalid batch partially wrote config: %q %v", after, err)
		}
	}
	outside := filepath.Join(t.TempDir(), "victim")
	mustWrite(t, outside, "pruning = \"nothing\"\n")
	if err := os.Symlink(outside, filepath.Join(home, "config", "app.toml")); err != nil {
		t.Fatal(err)
	}
	if err := NewConfigPatcher(home).PatchFiles(context.Background(), map[string]any{"config.toml": patch, "app.toml": patch}); err == nil {
		t.Fatal("symlink destination accepted")
	}
	got, err := os.ReadFile(outside)
	if err != nil || string(got) != "pruning = \"nothing\"\n" {
		t.Fatal("symlink target modified")
	}
	other := t.TempDir()
	if err := os.Symlink(filepath.Join(home, "config"), filepath.Join(other, "config")); err != nil {
		t.Fatal(err)
	}
	if err := NewConfigPatcher(other).PatchFiles(context.Background(), map[string]any{"config.toml": patch}); err == nil {
		t.Fatal("symlink parent accepted")
	}
	if err := NewConfigPatcher(home).PatchFiles(context.Background(), map[string]any{"config.toml": map[string]any{"mode": "full", "moniker": "repaired"}}); err != nil {
		t.Fatal(err)
	}
	if got := readTOML(t, path)["moniker"]; got != "repaired" {
		t.Fatalf("valid confined patch was not consumed: %v", got)
	}
}

func repairArchive(t *testing.T, headers []*tar.Header, contents [][]byte) []byte {
	t.Helper()
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	for i, h := range headers {
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if len(contents) > i {
			if _, err := tw.Write(contents[i]); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestRepair_326_SnapshotExtractionSymlinks(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "data", "snapshots")
	outside := filepath.Join(root, "identity.json")
	mustWrite(t, outside, "private identity")
	mustWrite(t, filepath.Join(dest, "old"), "old snapshot")
	for _, header := range []*tar.Header{
		{Name: "escape", Typeflag: tar.TypeSymlink, Linkname: outside},
		{Name: "escape", Typeflag: tar.TypeSymlink, Linkname: "../../identity.json"},
		{Name: "escape", Typeflag: tar.TypeLink, Linkname: outside},
		{Name: "../identity.json", Typeflag: tar.TypeReg, Size: 0},
		{Name: "/identity.json", Typeflag: tar.TypeReg, Size: 0},
	} {
		if err := extractTarStream(context.Background(), bytes.NewReader(repairArchive(t, []*tar.Header{header}, nil)), dest); err == nil {
			t.Fatalf("hostile archive accepted: %+v", header)
		}
		got, err := os.ReadFile(outside)
		if err != nil || string(got) != "private identity" {
			t.Fatal("archive changed node identity")
		}
		if !exists(filepath.Join(dest, "old")) {
			t.Fatal("invalid stage replaced original snapshot")
		}
	}
	if err := os.Symlink(filepath.Dir(outside), filepath.Join(dest, "linked")); err != nil {
		t.Fatal(err)
	}
	data := []byte("snapshot chunk")
	archive := repairArchive(t, []*tar.Header{{Name: "linked/identity.json", Typeflag: tar.TypeReg, Size: int64(len(data)), Mode: 0600}}, [][]byte{data})
	if err := extractTarStream(context.Background(), bytes.NewReader(archive), dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(outside)
	if err != nil || string(got) != "private identity" {
		t.Fatal("pre-existing symlink was followed")
	}
	got, err = os.ReadFile(filepath.Join(dest, "linked", "identity.json"))
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal("staged snapshot was not adopted")
	}
	// A large advertised member must be refused before any body is allocated.
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "huge", Typeflag: tar.TypeReg, Size: maxSnapshotFileBytes + 1, Mode: 0600}); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := extractTarStream(context.Background(), bytes.NewReader(b.Bytes()), dest); err == nil {
		t.Fatal("unbounded expanded member accepted")
	}
	if !exists(filepath.Join(dest, "linked", "identity.json")) {
		t.Fatal("oversized stage replaced valid snapshot")
	}
	f, err := os.CreateTemp(t.TempDir(), "compressed")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := (boundedSnapshotWriter{file: f}).WriteAt([]byte{1}, maxSnapshotCompressedBytes); err == nil {
		t.Fatal("unbounded compressed download accepted")
	}
}

func TestRepair_326_NativeFeeDenom(t *testing.T) {
	// Verify the explicit nested-module contract against the deployed source.
	contract, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "node", "params", "config.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(contract, []byte(`BaseCoinUnit  = "uhpx"`)) || !bytes.Contains(contract, []byte(`Bech32PrefixAccAddr = "pax"`)) {
		t.Fatal("Paxeer node contract changed; update the explicit sidecar contract")
	}
	kb, addr := testKeyring(t)
	cfg := engine.ExecutionConfig{Keyring: kb}
	vote, err := buildVoteMsg(cfg, GovVoteRequest{KeyName: "node_admin", ProposalID: 17, Option: "yes"})
	if err != nil {
		t.Fatal(err)
	}
	upgradeRequest := GovSoftwareUpgradeRequest{KeyName: "node_admin", Title: "Upgrade", Description: "Apply release", UpgradeName: "repair", UpgradeHeight: 100, InitialDeposit: "1000000uhpx"}
	upgrade, err := buildSoftwareUpgradeMsg(cfg, upgradeRequest)
	if err != nil {
		t.Fatal(err)
	}
	paramRequest := GovParamChangeRequest{KeyName: "node_admin", Title: "Params", Description: "Update parameters", Changes: []paramChange{{Subspace: "staking", Key: "MaxValidators", Value: json.RawMessage(`100`)}}, InitialDeposit: "1000000uhpx"}
	param, err := buildParamChangeMsg(cfg, paramRequest)
	if err != nil {
		t.Fatal(err)
	}
	_, cdc, txCodec := makeSignTxCodec()
	key := sdk.NewKVStoreKey("params")
	transient := sdk.NewTransientStoreKey("transient_params")
	db := dbm.NewMemDB()
	defer db.Close()
	cms := store.NewCommitMultiStore(db)
	cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, nil)
	cms.MountStoreWithDB(transient, sdk.StoreTypeTransient, nil)
	authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	cms.MountStoreWithDB(authKey, sdk.StoreTypeIAVL, nil)
	if err := cms.LoadLatestVersion(); err != nil {
		t.Fatal(err)
	}
	ctx := sdk.NewContext(cms, tmproto.Header{ChainID: repairChainID, Height: 1}, true)
	params := paramskeeper.NewKeeper(cdc, codec.NewLegacyAmino(), key, transient)
	minPrice := sdk.NewDecCoins(sdk.NewDecCoinFromDec("uhpx", sdk.NewDecWithPrec(1, 3)))
	params.SetFeesParams(ctx, paramstypes.FeesParams{GlobalMinimumGasPrices: minPrice, AllowedFeeDenoms: []string{"uhpx"}})
	ctx = ctx.WithMinGasPrices(minPrice)
	for _, msg := range []sdk.Msg{vote, upgrade, param} {
		in := SignAndBroadcastInput{ChainID: repairChainID, KeyName: "node_admin", Msg: msg, Fees: "4000uhpx", Gas: 200000, TaskID: uuid.NewString()}
		in.Memo = appendTaskIDToMemo("native fees", in.TaskID)
		if err := checkFeesDenom(in.Fees); err != nil {
			t.Fatal(err)
		}
		encoded, err := signTransaction(cfg, in, 17, 42)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := txCodec.TxDecoder()(encoded)
		if err != nil {
			t.Fatal(err)
		}
		if len(decoded.GetMsgs()) != 1 || sdk.MsgTypeURL(decoded.GetMsgs()[0]) != sdk.MsgTypeURL(msg) {
			t.Fatal("governance message did not survive the transaction codec")
		}
		coins, _, err := ante.CheckTxFeeWithValidatorMinGasPrices(ctx, decoded, false, params)
		if err != nil || coins.String() != "4000uhpx" {
			t.Fatalf("real keeper fee admission rejected native transaction: %s %v", coins, err)
		}
		if !bytes.Equal(decoded.(authsigning.SigVerifiableTx).GetSigners()[0], addr) {
			t.Fatal("decoded signer differs from governance producer")
		}
	}
	ak := authkeeper.NewAccountKeeper(cdc, authKey, params.Subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, map[string][]string{})
	ak.SetAccount(ctx, authtypes.NewBaseAccount(addr, nil, 17, 42))
	repairGovernanceTransport(t, cfg, ctx, params, ak)
	for _, fees := range []string{"4000usei", "4000hpx", "1uatom", "0uhpx"} {
		if err := checkFeesDenom(fees); err == nil {
			t.Fatalf("fee policy accepted %q", fees)
		}
	}
	upgradeRequest.InitialDeposit = "100usei"
	if _, err := buildSoftwareUpgradeMsg(cfg, upgradeRequest); !IsTerminal(err) {
		t.Fatal("upgrade accepted retired deposit denom")
	}
	paramRequest.InitialDeposit = "100usei"
	if _, err := buildParamChangeMsg(cfg, paramRequest); !IsTerminal(err) {
		t.Fatal("parameter change accepted retired deposit denom")
	}
}

// The task producers and production SDK RPC client cross the actual JSON-RPC
// transport. The committed peer runs real account queries and fee admission.
func repairGovernanceTransport(t *testing.T, cfg engine.ExecutionConfig, ctx sdk.Context, params paramskeeper.Keeper, ak authkeeper.AccountKeeper) {
	t.Helper()
	_, cdc, txCodec := makeSignTxCodec()
	var mu sync.Mutex
	transactions := map[string]*coretypes.ResultTx{}
	mux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, map[string]*rpcserver.RPCFunc{
		"status": rpcserver.NewRPCFunc(func(context.Context) (*rpc.StatusResult, error) {
			return &rpc.StatusResult{NodeInfo: rpc.NodeInfo{Network: repairChainID}, SyncInfo: rpc.SyncInfo{LatestBlockHeight: "1"}}, nil
		}),
		"abci_query": rpcserver.NewRPCFunc(func(_ context.Context, req *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
			mu.Lock()
			defer mu.Unlock()
			if req.Path != "/cosmos.auth.v1beta1.Query/Account" {
				return nil, fmt.Errorf("unexpected query %q", req.Path)
			}
			var query authtypes.QueryAccountRequest
			if err := cdc.Unmarshal(req.Data, &query); err != nil {
				return nil, err
			}
			response, err := ak.Account(sdk.WrapSDKContext(ctx), &query)
			if err != nil {
				return nil, err
			}
			encoded, err := cdc.Marshal(response)
			if err != nil {
				return nil, err
			}
			return &coretypes.ResultABCIQuery{Response: abci.ResponseQuery{Value: encoded, Height: 1}}, nil
		}),
		"broadcast_tx_sync": rpcserver.NewRPCFunc(func(_ context.Context, req *coretypes.RequestBroadcastTx) (*coretypes.ResultBroadcastTx, error) {
			mu.Lock()
			defer mu.Unlock()
			decoded, err := txCodec.TxDecoder()(req.Tx)
			if err != nil {
				return nil, err
			}
			for _, msg := range decoded.GetMsgs() {
				if err := msg.ValidateBasic(); err != nil {
					return nil, err
				}
			}
			coins, _, err := ante.CheckTxFeeWithValidatorMinGasPrices(ctx, decoded, false, params)
			if err != nil {
				return nil, err
			}
			if coins.String() != "4000uhpx" {
				return nil, fmt.Errorf("unexpected admitted fees %s", coins)
			}
			verify := sdk.ChainAnteDecorators(ante.NewSetPubKeyDecorator(ak), ante.NewSigVerificationDecorator(ak, txCodec.SignModeHandler()), ante.NewIncrementSequenceDecorator(ak))
			if _, err := verify(ctx, decoded, false); err != nil {
				return nil, err
			}
			hash := sha256.Sum256(req.Tx)
			var resultData []byte
			if _, ok := decoded.GetMsgs()[0].(*govtypes.MsgSubmitProposal); ok {
				proposal, err := (&govtypes.MsgSubmitProposalResponse{ProposalId: uint64(len(transactions) + 1)}).Marshal()
				if err != nil {
					return nil, err
				}
				resultData, err = (&sdk.TxMsgData{Data: []*sdk.MsgData{{MsgType: submitProposalMsgType, Data: proposal}}}).Marshal()
				if err != nil {
					return nil, err
				}
			}
			transactions[fmt.Sprintf("%X", hash)] = &coretypes.ResultTx{Hash: hash[:], Height: 1, Tx: req.Tx, TxResult: abci.ExecTxResult{Data: resultData, GasWanted: 200000, GasUsed: 1}}
			return &coretypes.ResultBroadcastTx{Hash: hash[:]}, nil
		}),
		"tx": rpcserver.NewRPCFunc(func(_ context.Context, req *coretypes.RequestTx) (*coretypes.ResultTx, error) {
			mu.Lock()
			defer mu.Unlock()
			result := transactions[fmt.Sprintf("%X", []byte(req.Hash))]
			if result == nil {
				return nil, fmt.Errorf("tx not found")
			}
			return result, nil
		}),
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	t.Setenv("SEI_CHAIN_ID", repairChainID)
	cfg.RPC = rpc.NewClient(server.URL, server.Client())
	checkpointer, err := engine.NewSQLiteStore(filepath.Join(t.TempDir(), "governance.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer checkpointer.Close()
	cfg.Checkpointer = checkpointer
	cases := []struct {
		name    string
		handler engine.TaskHandler
		request any
	}{
		{"vote", NewGovVoter(cfg).Handler(), GovVoteRequest{ChainID: repairChainID, KeyName: "node_admin", ProposalID: 17, Option: "yes", Fees: "4000uhpx", Gas: 200000}},
		{"upgrade", NewGovSoftwareUpgrader(cfg).Handler(), GovSoftwareUpgradeRequest{ChainID: repairChainID, KeyName: "node_admin", Title: "Upgrade", Description: "Apply release", UpgradeName: "repair", UpgradeHeight: 100, InitialDeposit: "1000000uhpx", Fees: "4000uhpx", Gas: 200000}},
		{"params", NewGovParamChanger(cfg).Handler(), GovParamChangeRequest{ChainID: repairChainID, KeyName: "node_admin", Title: "Params", Description: "Update parameters", Changes: []paramChange{{Subspace: "staking", Key: "MaxValidators", Value: json.RawMessage(`100`)}}, InitialDeposit: "1000000uhpx", Fees: "4000uhpx", Gas: 200000}},
	}
	for _, tc := range cases {
		encoded, err := json.Marshal(tc.request)
		if err != nil {
			t.Fatal(err)
		}
		var request map[string]any
		if err := json.Unmarshal(encoded, &request); err != nil {
			t.Fatal(err)
		}
		id := uuid.NewString()
		result, err := tc.handler(engine.WithTaskID(context.Background(), id), request)
		if err != nil {
			t.Fatalf("%s production handler/transport/fee admission: %v", tc.name, err)
		}
		var outcome struct {
			TxHash          string `json:"txHash"`
			InclusionStatus string `json:"inclusionStatus"`
			ProposalID      uint64 `json:"proposalId"`
		}
		if err := json.Unmarshal(result, &outcome); err != nil {
			t.Fatal(err)
		}
		if outcome.InclusionStatus != "committed_ok" || outcome.TxHash == "" {
			t.Fatalf("%s result did not round trip: %s", tc.name, result)
		}
		if tc.name != "vote" && outcome.ProposalID == 0 {
			t.Fatalf("%s lost proposal response", tc.name)
		}
		marker, err := checkpointer.GetTxMarker(id)
		if err != nil || marker == nil || marker.TxHash != outcome.TxHash {
			t.Fatalf("%s transaction identity not durable: %+v %v", tc.name, marker, err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(transactions) != 3 {
		t.Fatalf("expected three admitted governance transactions, got %d", len(transactions))
	}
}

// repairCeremony starts with module defaults, as paxd init does. The identity
// task alone intentionally writes only a bare consensus genesis document.
func repairCeremony(t *testing.T, home string) {
	t.Helper()
	if _, err := NewIdentityGenerator(home).Handler()(context.Background(), map[string]any{"chainId": repairChainID, "moniker": "repair-validator"}); err != nil {
		t.Fatal(err)
	}
	cdc, _ := makeCodec()
	path := filepath.Join(home, "config", "genesis.json")
	doc, err := tmtypes.GenesisDocFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	staking := stakingtypes.DefaultGenesisState()
	staking.Params.BondDenom = feeDenom
	state := map[string]json.RawMessage{
		authtypes.ModuleName:    cdc.MustMarshalJSON(authtypes.DefaultGenesisState()),
		banktypes.ModuleName:    cdc.MustMarshalJSON(banktypes.DefaultGenesisState()),
		stakingtypes.ModuleName: cdc.MustMarshalJSON(staking),
		genutiltypes.ModuleName: cdc.MustMarshalJSON(genutiltypes.DefaultGenesisState()),
	}
	doc.AppState, err = json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := genutil.ExportGenesisFile(doc, path); err != nil {
		t.Fatal(err)
	}
	if _, err := NewGentxGenerator(home).Handler()(context.Background(), map[string]any{"chainId": repairChainID, "stakingAmount": "1000000uhpx", "accountBalance": "100000000uhpx"}); err != nil {
		t.Fatal(err)
	}
}

// Execute the same auth/bank/staking/genutil initialization functions used by
// InitChain, with production keepers and a committed in-memory multistore.
func repairInitGenesis(t *testing.T, state map[string]json.RawMessage, gentx json.RawMessage, external, vesting string) {
	t.Helper()
	cdc, txCodec := makeCodec()
	db := dbm.NewMemDB()
	defer db.Close()
	cms := store.NewCommitMultiStore(db)
	keys := sdk.NewKVStoreKeys("params", authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey)
	for _, key := range keys {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, nil)
	}
	transient := sdk.NewTransientStoreKey("transient_params")
	cms.MountStoreWithDB(transient, sdk.StoreTypeTransient, nil)
	if err := cms.LoadLatestVersion(); err != nil {
		t.Fatal(err)
	}
	ctx := sdk.NewContext(cms, tmproto.Header{ChainID: repairChainID, Time: time.Now().UTC()}, false).
		WithConsensusParams(&tmproto.ConsensusParams{Validator: &tmproto.ValidatorParams{PubKeyTypes: []string{"ed25519"}}})
	params := paramskeeper.NewKeeper(cdc, codec.NewLegacyAmino(), keys["params"], transient)
	ak := authkeeper.NewAccountKeeper(cdc, keys[authtypes.StoreKey], params.Subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount,
		map[string][]string{authtypes.FeeCollectorName: nil, stakingtypes.BondedPoolName: {authtypes.Burner, authtypes.Staking}, stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking}})
	bk := bankkeeper.NewBaseKeeper(cdc, keys[banktypes.StoreKey], ak, params.Subspace(banktypes.ModuleName), map[string]bool{})
	sk := stakingkeeper.NewKeeper(cdc, keys[stakingtypes.StoreKey], ak, bk, params.Subspace(stakingtypes.ModuleName))
	authState := authtypes.GetGenesisStateFromAppState(cdc, state)
	bankState := banktypes.GetGenesisStateFromAppState(cdc, state)
	stakingState := stakingtypes.GetGenesisStateFromAppState(cdc, state)
	authmodule.InitGenesis(ctx, ak, authState)
	bk.InitGenesis(ctx, bankState)
	stakingmodule.InitGenesis(ctx, sk, ak, bk, stakingState)
	genesis := genutiltypes.NewGenesisState([]json.RawMessage{gentx})
	if err := genutiltypes.ValidateGenesis(genesis, txCodec.TxJSONDecoder()); err != nil {
		t.Fatal(err)
	}
	server := stakingkeeper.NewMsgServerImpl(sk)
	validators, err := genutil.InitGenesis(ctx, sk, func(ctx sdk.Context, req abci.RequestDeliverTxV2, tx sdk.Tx, checksum [32]byte) abci.ResponseDeliverTx {
		if sha256.Sum256(req.Tx) != checksum {
			t.Fatal("genesis transaction bytes changed in transport")
		}
		if len(tx.GetMsgs()) != 1 {
			t.Fatal("gentx message count changed")
		}
		msg, ok := tx.GetMsgs()[0].(*stakingtypes.MsgCreateValidator)
		if !ok {
			t.Fatal("gentx did not reach staking consumer")
		}
		verify := sdk.ChainAnteDecorators(ante.NewSetPubKeyDecorator(ak), ante.NewSigVerificationDecorator(ak, txCodec.SignModeHandler()))
		if _, err := verify(ctx, tx, false); err != nil {
			return abci.ResponseDeliverTx{Code: 1, Log: err.Error()}
		}
		if _, err := server.CreateValidator(sdk.WrapSDKContext(ctx), msg); err != nil {
			return abci.ResponseDeliverTx{Code: 1, Log: err.Error()}
		}
		return abci.ResponseDeliverTx{}
	}, *genesis, txCodec)
	if err != nil || len(validators) != 1 || validators[0].Power != 1 {
		t.Fatalf("genesis validator initialization: %+v %v", validators, err)
	}
	for _, address := range []string{external, vesting} {
		addr, err := sdk.AccAddressFromBech32(address)
		if err != nil {
			t.Fatal(err)
		}
		if ak.GetAccount(ctx, addr) == nil {
			t.Fatalf("genesis consumer omitted account %s", address)
		}
		var expected sdk.Coins
		for _, balance := range bankState.Balances {
			if balance.Address == address {
				expected = balance.Coins
			}
		}
		if !bk.GetAllBalances(ctx, addr).IsEqual(expected) {
			t.Fatalf("genesis consumer changed balance for %s", address)
		}
	}
	if _, ok := ak.GetAccount(ctx, sdk.MustAccAddressFromBech32(vesting)).(*vestingtypes.ContinuousVestingAccount); !ok {
		t.Fatal("genesis keeper lost vesting account type")
	}
	commit := cms.Commit(true)
	if commit.Version != 1 || len(commit.Hash) == 0 {
		t.Fatalf("genesis state was not committed: %+v", commit)
	}
	reopened := store.NewCommitMultiStore(db)
	for _, key := range keys {
		reopened.MountStoreWithDB(key, sdk.StoreTypeIAVL, nil)
	}
	reopened.MountStoreWithDB(transient, sdk.StoreTypeTransient, nil)
	if err := reopened.LoadLatestVersion(); err != nil {
		t.Fatal(err)
	}
	if recovered := reopened.LastCommitID(); recovered.Version != commit.Version || !bytes.Equal(recovered.Hash, commit.Hash) {
		t.Fatalf("genesis commit changed after reopening: got %+v, want %+v", recovered, commit)
	}
	recoveredCtx := ctx.WithMultiStore(reopened)
	recoveredValidators := sk.GetAllValidators(recoveredCtx)
	if len(recoveredValidators) != 1 || recoveredValidators[0].GetConsensusPower(sk.PowerReduction(recoveredCtx)) != validators[0].Power {
		t.Fatalf("committed validator did not recover: %+v", recoveredValidators)
	}
	for _, address := range []string{external, vesting} {
		addr := sdk.MustAccAddressFromBech32(address)
		if ak.GetAccount(recoveredCtx, addr) == nil || !bk.GetAllBalances(recoveredCtx, addr).IsEqual(bk.GetAllBalances(ctx, addr)) {
			t.Fatalf("committed account or balance did not recover: %s", address)
		}
	}
	if _, ok := ak.GetAccount(recoveredCtx, sdk.MustAccAddressFromBech32(vesting)).(*vestingtypes.ContinuousVestingAccount); !ok {
		t.Fatal("committed vesting account type did not recover")
	}
}

func TestRepair_326_GenesisAddressPrefix(t *testing.T) {
	home := t.TempDir()
	repairCeremony(t, home)
	cdc, txCodec := makeCodec()
	entries, err := os.ReadDir(filepath.Join(home, "config", "gentx"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("gentx producer output: %v %v", entries, err)
	}
	data, err := os.ReadFile(filepath.Join(home, "config", "gentx", entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := txCodec.TxJSONDecoder()(data)
	if err != nil {
		t.Fatal(err)
	}
	msg, ok := decoded.GetMsgs()[0].(*stakingtypes.MsgCreateValidator)
	if !ok {
		t.Fatal("gentx did not produce a validator message")
	}
	if !strings.HasPrefix(msg.DelegatorAddress, "pax1") || !strings.HasPrefix(msg.ValidatorAddress, "paxvaloper1") {
		t.Fatalf("gentx address contract: %s %s", msg.DelegatorAddress, msg.ValidatorAddress)
	}
	if err := msg.ValidateBasic(); err != nil {
		t.Fatal(err)
	}
	external := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
	vesting := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
	a := NewGenesisAssembler(home, "bucket", "local", repairChainID, nil, nil)
	accounts := []GenesisAccountEntry{{Address: external, Balance: "500uhpx"}, {Address: vesting, Balance: "1000uhpx", Vesting: &GenesisAccountVesting{Amount: "900uhpx", EndTime: time.Now().Add(24 * time.Hour).Unix()}}}
	if err := a.addExternalGenesisAccounts(accounts); err != nil {
		t.Fatal(err)
	}
	appState, _, err := genutiltypes.GenesisStateFromGenFile(filepath.Join(home, "config", "genesis.json"))
	if err != nil {
		t.Fatal(err)
	}
	auth := authtypes.GetGenesisStateFromAppState(cdc, appState)
	unpacked, err := authtypes.UnpackAccounts(auth.Accounts)
	if err != nil {
		t.Fatal(err)
	}
	foundExternal, foundVesting := false, false
	for _, acc := range unpacked {
		if err := acc.Validate(); err != nil {
			t.Fatal(err)
		}
		switch acc.GetAddress().String() {
		case external:
			foundExternal = true
		case vesting:
			_, foundVesting = acc.(*vestingtypes.ContinuousVestingAccount)
		}
	}
	if !foundExternal || !foundVesting {
		t.Fatal("external and vesting accounts did not survive the real genesis codec")
	}
	bank := banktypes.GetGenesisStateFromAppState(cdc, appState)
	for _, balance := range bank.Balances {
		if _, err := sdk.AccAddressFromBech32(balance.Address); err != nil {
			t.Fatal(err)
		}
	}
	repairInitGenesis(t, appState, data, external, vesting)
	for _, size := range []int{0, 1, 19, 21, 31, 33} {
		if err := sdk.VerifyAddressFormat(make([]byte, size)); err == nil {
			t.Fatalf("invalid address byte length admitted: %d", size)
		}
	}
	for _, size := range []int{20, 32} {
		addr := sdk.AccAddress(bytes.Repeat([]byte{1}, size))
		if _, err := sdk.AccAddressFromBech32(addr.String()); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.addExternalGenesisAccounts([]GenesisAccountEntry{{Address: "sei1zg69v7y6hn00qy352euf40x77qfrg4nclsjzp9", Balance: "1uhpx"}}); err == nil {
		t.Fatal("retired prefix accepted")
	}
}

func TestRepair_326_GenesisKeyLeak(t *testing.T) {
	root, s3Factory, uploaderFactory, _ := repairObjectTransport(t)
	home := t.TempDir()
	if _, err := NewIdentityGenerator(home).Handler()(context.Background(), map[string]any{"chainId": repairChainID, "moniker": "public-only"}); err != nil {
		t.Fatal(err)
	}
	private, err := os.ReadFile(filepath.Join(home, "config", "node_key.json"))
	if err != nil {
		t.Fatal(err)
	}
	client, err := uploaderFactory(context.Background(), "local")
	if err != nil {
		t.Fatal(err)
	}
	u := NewGenesisArtifactUploader(home, "bucket", "local", repairChainID, uploaderFactory)
	if err := u.uploadIdentity(context.Background(), client, "bucket", repairChainID+"/node/"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "bucket", repairChainID, "node", "identity.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("priv_key")) || bytes.Contains(raw, []byte("secret")) {
		t.Fatal("private fields crossed the upload boundary")
	}
	identity, err := decodePublicIdentity(raw)
	if err != nil {
		t.Fatal(err)
	}
	key, err := tmtypes.LoadNodeKey(filepath.Join(home, "config", "node_key.json"))
	if err != nil {
		t.Fatal(err)
	}
	if identity.NodeKey.ID != string(key.ID) || !bytes.Equal(identity.NodeKey.PublicKey, key.PubKey().Bytes()) {
		t.Fatal("public identity differs from real node key")
	}
	a := NewGenesisAssembler(home, "bucket", "local", repairChainID, s3Factory, uploaderFactory)
	if err := a.uploadPeers(context.Background(), AssembleGenesisRequest{Namespace: "test"}, []string{"node"}); err != nil {
		t.Fatal(err)
	}
	peers, err := os.ReadFile(filepath.Join(root, "bucket", repairChainID, "peers.json"))
	if err != nil || !bytes.Contains(peers, []byte(identity.NodeKey.ID+"@node-0.node.test.svc.cluster.local:26656")) {
		t.Fatalf("assembler did not consume public identity: %s %v", peers, err)
	}
	if _, err := decodePublicIdentity(append([]byte(`{"private_key":"forbidden",`), raw[1:]...)); err == nil {
		t.Fatal("consumer accepted secret-bearing identity")
	}
	if _, err := decodePublicIdentity(append([]byte(`{"node_key":`), append(private, []byte(`}`)...)...)); err == nil {
		t.Fatal("consumer accepted the legacy private node key")
	}
	identity.NodeKey.ID = strings.Repeat("0", 40)
	bad, err := json.Marshal(identity)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodePublicIdentity(bad); err == nil {
		t.Fatal("consumer accepted mismatched node ID and public key")
	}
	after, err := os.ReadFile(filepath.Join(home, "config", "node_key.json"))
	if err != nil || !bytes.Equal(after, private) {
		t.Fatal("upload altered local P2P authentication key")
	}
}

func TestRepair_326_GenesisHashRecovery(t *testing.T) {
	root, s3Factory, uploaderFactory, server := repairObjectTransport(t)
	home := t.TempDir()
	repairCeremony(t, home)
	if _, err := NewGenesisArtifactUploader(home, "bucket", "local", repairChainID, uploaderFactory).Handler()(context.Background(), map[string]any{"nodeName": "node"}); err != nil {
		t.Fatal(err)
	}
	cfg := AssembleGenesisRequest{AccountBalance: "100000000uhpx", Namespace: "test", Nodes: []AssembleNodeEntry{{Name: "node"}}}
	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var params map[string]any
	if err := json.Unmarshal(encoded, &params); err != nil {
		t.Fatal(err)
	}
	id := uuid.NewString()
	path := filepath.Join(home, "sidecar.db")
	s, err := engine.NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Save(&engine.TaskResult{ID: id, Type: string(engine.TaskAssembleAndUploadGenesis), Status: engine.TaskStatusRunning, Run: 1, Params: params, SubmittedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	a := NewGenesisAssembler(home, "bucket", "local", repairChainID, s3Factory, uploaderFactory)
	if _, err := a.Handler()(engine.WithTaskID(context.Background(), id), params); err != nil {
		t.Fatal(err)
	}
	// Crash window: uploaded and locally completed, but engine.Result is absent.
	before, err := s.Get(id)
	if err != nil || len(before.Result) != 0 {
		t.Fatalf("crash-window fixture persisted a result: %+v %v", before, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	server.Close()
	s, err = engine.NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reopened := NewGenesisAssembler(home, "bucket", "local", repairChainID, s3Factory, uploaderFactory)
	e := engine.NewEngine(ctx, map[engine.TaskType]engine.TaskHandler{engine.TaskAssembleAndUploadGenesis: reopened.Handler()}, s)
	e.RehydrateStaleTasks()
	result := repairWaitResult(t, e, id)
	if result.Status != engine.TaskStatusCompleted {
		t.Fatalf("recovery failed: %+v", result)
	}
	var output AssembleGenesisResult
	if err := json.Unmarshal(result.Result, &output); err != nil {
		t.Fatal(err)
	}
	published, err := os.ReadFile(filepath.Join(root, "bucket", repairChainID, "genesis.json"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(published)
	if output.GenesisHash != hex.EncodeToString(sum[:]) {
		t.Fatalf("recovery lost published genesis digest: %+v", output)
	}
	cfg.Namespace = "different"
	if _, err := reopened.recoverAssembly(cfg); err == nil {
		t.Fatal("assembly completion adopted a different request")
	}
}

func TestRepair_326_UnverifiedGenesis(t *testing.T) {
	root, s3Factory, uploaderFactory, server := repairObjectTransport(t)
	source := t.TempDir()
	if _, err := NewIdentityGenerator(source).Handler()(context.Background(), map[string]any{"chainId": repairChainID, "moniker": "assembler"}); err != nil {
		t.Fatal(err)
	}
	a := NewGenesisAssembler(source, "bucket", "local", repairChainID, s3Factory, uploaderFactory)
	hash, err := a.uploadGenesis(context.Background(), AssembleGenesisRequest{})
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	fetcher := NewGenesisFetcher(home, repairChainID, "bucket", "local", s3Factory)
	for _, digest := range []string{"", strings.ToUpper(hash), "sha256:" + hash, strings.Repeat("x", 64)} {
		if _, err := fetcher.Handler()(context.Background(), map[string]any{"expectedGenesisHash": digest}); err == nil {
			t.Fatalf("noncanonical/missing digest accepted: %q", digest)
		}
	}
	if exists(filepath.Join(home, "config", "genesis.json")) {
		t.Fatal("unverified trust root published")
	}
	params := map[string]any{"expectedGenesisHash": hash}
	if _, err := fetcher.Handler()(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	published, err := os.ReadFile(filepath.Join(root, "bucket", repairChainID, "genesis.json"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(home, "config", "genesis.json"))
	if err != nil || !bytes.Equal(got, published) {
		t.Fatal("follower did not adopt assembler's exact bytes")
	}
	server.Close()
	fetcher = NewGenesisFetcher(home, repairChainID, "bucket", "local", s3Factory)
	if _, err := fetcher.Handler()(context.Background(), params); err != nil {
		t.Fatalf("verified replay unexpectedly needs storage: %v", err)
	}
	mustWrite(t, filepath.Join(home, "config", "genesis.json"), `{"chain_id":"poisoned"}`)
	if _, err := fetcher.Handler()(context.Background(), params); err == nil {
		t.Fatal("marker replay trusted modified genesis")
	}
	if _, err := fetcher.Handler()(context.Background(), map[string]any{"expectedGenesisHash": strings.Repeat("0", 64)}); err == nil {
		t.Fatal("marker accepted changed authenticated digest")
	}
	if err := fetcher.Fetch(context.Background(), GenesisS3Config{Bucket: "bucket", Key: "genesis", Region: "local"}); err == nil {
		t.Fatal("legacy Fetch bypassed digest requirement")
	}
	// A failed replacement must preserve previously installed bytes.
	otherRoot, otherS3, _, _ := repairObjectTransport(t)
	mustWrite(t, filepath.Join(otherRoot, "bucket", repairChainID, "genesis.json"), `{"chain_id":"attacker"}`)
	oldHome := t.TempDir()
	mustWrite(t, filepath.Join(oldHome, "config", "genesis.json"), string(published))
	other := NewGenesisFetcher(oldHome, repairChainID, "bucket", "local", otherS3)
	if _, err := other.Handler()(context.Background(), params); err == nil {
		t.Fatal("mismatched download accepted")
	}
	retained, err := os.ReadFile(filepath.Join(oldHome, "config", "genesis.json"))
	if err != nil || !bytes.Equal(retained, published) {
		t.Fatal("failed verification replaced existing genesis")
	}
	if markerExists(oldHome, genesisMarkerFile) {
		t.Fatal("failed download wrote completion")
	}
}

func TestRepair_326_TxMarkerAdoptsDifferentRequest(t *testing.T) {
	kb, addr := testKeyring(t)
	path := filepath.Join(t.TempDir(), "sidecar.db")
	s, err := engine.NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg := engine.ExecutionConfig{Keyring: kb, Checkpointer: s}
	id := uuid.NewString()
	msg, err := buildVoteMsg(cfg, GovVoteRequest{KeyName: "node_admin", ProposalID: 17, Option: "yes"})
	if err != nil {
		t.Fatal(err)
	}
	in := SignAndBroadcastInput{ChainID: repairChainID, KeyName: "node_admin", Msg: msg, Fees: "4000uhpx", Gas: 200000, TaskID: id, Memo: appendTaskIDToMemo("original", id)}
	signed, err := signTransaction(cfg, in, 17, 42)
	if err != nil {
		t.Fatal(err)
	}
	marker := &engine.TxMarker{TaskID: id, TxHash: fmt.Sprintf("%X", sha256.Sum256(signed)), TxBytes: signed, ChainID: repairChainID, AccountNumber: 17, Sequence: 42}
	if err := s.SaveTxMarker(marker); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(&engine.TaskResult{ID: id, Type: string(engine.TaskGovVote), Status: engine.TaskStatusCompleted, SubmittedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if deleted, err := s.Delete(id); err != nil || !deleted {
		t.Fatalf("delete task: %v %v", deleted, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = engine.NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	cfg.Checkpointer = s
	recovered, err := s.GetTxMarker(id)
	if err != nil || recovered == nil {
		t.Fatalf("delete lost transaction tombstone: %+v %v", recovered, err)
	}
	if err := verifyMarkerRequest(cfg, recovered, in, addr); err != nil {
		t.Fatalf("same request failed to recover: %v", err)
	}
	alteredMarker := *recovered
	alteredMarker.AccountNumber++
	if err := verifyMarkerRequest(cfg, &alteredMarker, in, addr); err == nil {
		t.Fatal("different account identity adopted marker")
	}
	alteredMarker = *recovered
	alteredMarker.ChainID = "other-chain"
	alteredInput := in
	alteredInput.ChainID = alteredMarker.ChainID
	if err := verifyMarkerRequest(cfg, &alteredMarker, alteredInput, addr); err == nil {
		t.Fatal("marker metadata rebound signed bytes to another chain")
	}
	mutations := []func(*SignAndBroadcastInput){
		func(v *SignAndBroadcastInput) { v.ChainID = "other-chain" },
		func(v *SignAndBroadcastInput) { v.Msg = govtypes.NewMsgVote(addr, 18, govtypes.OptionYes) },
		func(v *SignAndBroadcastInput) { v.Msg = govtypes.NewMsgVote(addr, 17, govtypes.OptionNo) },
		func(v *SignAndBroadcastInput) { v.Fees = "5000uhpx" },
		func(v *SignAndBroadcastInput) { v.Gas++ },
		func(v *SignAndBroadcastInput) { v.Memo = appendTaskIDToMemo("different", id) },
	}
	for i, mutate := range mutations {
		changed := in
		mutate(&changed)
		if err := verifyMarkerRequest(cfg, recovered, changed, addr); err == nil {
			t.Fatalf("conflicting UUID mutation %d adopted old transaction", i)
		}
	}
	// The actual signing entry point must reject before attempting chain I/O,
	// signing, querying an old outcome, or broadcasting under the new payload.
	changed := in
	changed.Memo = "original"
	changed.Msg = govtypes.NewMsgVote(addr, 18, govtypes.OptionYes)
	if result, err := signAndBroadcast(context.Background(), cfg, nil, changed, addr); !IsTerminal(err) || result != nil {
		t.Fatalf("signing consumer adopted different request: %+v %v", result, err)
	}
	upgrade := GovSoftwareUpgradeRequest{ChainID: repairChainID, KeyName: "node_admin", Title: "Upgrade", Description: "Apply release", UpgradeName: "repair", UpgradeHeight: 100, InitialDeposit: "1000000uhpx", Fees: "4000uhpx", Gas: 200000, Memo: "original"}
	rawUpgrade, err := json.Marshal(upgrade)
	if err != nil {
		t.Fatal(err)
	}
	var upgradeParams map[string]any
	if err := json.Unmarshal(rawUpgrade, &upgradeParams); err != nil {
		t.Fatal(err)
	}
	if result, err := NewGovSoftwareUpgrader(cfg).Handler()(engine.WithTaskID(context.Background(), id), upgradeParams); !IsTerminal(err) || result != nil {
		t.Fatalf("different task type adopted deleted vote transaction: %s %v", result, err)
	}
	after, err := s.GetTxMarker(id)
	if err != nil || !bytes.Equal(after.TxBytes, signed) {
		t.Fatal("conflicting reuse replaced transaction evidence")
	}
	another := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	if err := verifyMarkerRequest(cfg, recovered, in, another); err == nil {
		t.Fatal("different signer adopted marker")
	}
}

// This committed peer runs in the test executable itself, with the deployed
// comm/argv/home contract and a loopback status transcript. Process discovery,
// signaling, exit detection, restart and RPC transport all remain production code.
// Full paxd application linkage belongs to the node module (see repair_326.kvx).
func init() {
	if len(os.Args) != 7 || filepath.Base(os.Args[0]) != "paxd" || os.Args[1] != "start" || os.Args[2] != "--home" || os.Args[4] != "--rpc" || os.Args[6] != "repair-326-peer" {
		return
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(rpc.StatusResult{NodeInfo: rpc.NodeInfo{Network: repairChainID}, SyncInfo: rpc.SyncInfo{LatestBlockHeight: "1"}}); err != nil {
			panic(err)
		}
	})
	if err := http.ListenAndServe(os.Args[5], mux); err != nil {
		panic(err)
	}
	os.Exit(0)
}

func repairPaxdBinary(t *testing.T) string {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "paxd")
	if err := os.Symlink(executable, binary); err != nil {
		t.Fatal(err)
	}
	return binary
}

func repairAvailablePorts(t *testing.T, n int) []string {
	t.Helper()
	var listeners []net.Listener
	var addresses []string
	for i := 0; i < n; i++ {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		listeners = append(listeners, l)
		addresses = append(addresses, l.Addr().String())
	}
	for _, l := range listeners {
		if err := l.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return addresses
}

func repairWaitRPC(t *testing.T, endpoint string) {
	t.Helper()
	client := rpc.NewStatusClient(endpoint, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for {
		if _, err := client.Status(ctx); err == nil {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("real paxd did not serve status at %s", endpoint)
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func TestRepair_326_PaxdProcessInvisible(t *testing.T) {
	binary := repairPaxdBinary(t)
	home := t.TempDir()
	repairCeremony(t, home)
	a := NewGenesisAssembler(home, "bucket", "local", repairChainID, nil, nil)
	external := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
	vesting := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
	if err := a.addExternalGenesisAccounts([]GenesisAccountEntry{{Address: external, Balance: "100uhpx"}, {Address: vesting, Balance: "1000uhpx", Vesting: &GenesisAccountVesting{Amount: "900uhpx", EndTime: time.Now().Add(24 * time.Hour).Unix()}}}); err != nil {
		t.Fatal(err)
	}
	cdc, txCodec := makeCodec()
	genFile := filepath.Join(home, "config", "genesis.json")
	appState, doc, err := genutiltypes.GenesisStateFromGenFile(genFile)
	if err != nil {
		t.Fatal(err)
	}
	gentxs, _, err := genutil.CollectTxs(cdc, txCodec.TxJSONDecoder(), "repair", filepath.Join(home, "config", "gentx"), *doc, banktypes.GenesisBalancesIterator{})
	if err != nil || len(gentxs) != 1 {
		t.Fatalf("real collect-gentxs: %v %v", gentxs, err)
	}
	gentx, err := txCodec.TxJSONEncoder()(gentxs[0])
	if err != nil {
		t.Fatal(err)
	}
	repairInitGenesis(t, appState, gentx, external, vesting)
	ports := repairAvailablePorts(t, 2)
	endpoint := "http://" + ports[0]
	if err := NewConfigPatcher(home).PatchFiles(context.Background(), map[string]any{
		"config.toml": map[string]any{"mode": "full", "rpc": map[string]any{"laddr": "tcp://" + ports[0]}, "p2p": map[string]any{"laddr": "tcp://" + ports[1]}},
		"app.toml":    map[string]any{"api": map[string]any{"enable": false}, "grpc": map[string]any{"enable": false}, "grpc-web": map[string]any{"enable": false}},
	}); err != nil {
		t.Fatal(err)
	}
	log, err := os.Create(filepath.Join(home, "paxd.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	start := func() (*exec.Cmd, chan error, error) {
		cmd := exec.Command(binary, "start", "--home", home, "--rpc", ports[0], "repair-326-peer")
		cmd.Stdout = log
		cmd.Stderr = log
		if err := cmd.Start(); err != nil {
			return nil, nil, err
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait(); close(done) }()
		t.Cleanup(func() {
			_ = cmd.Process.Signal(syscall.SIGTERM)
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				_ = cmd.Process.Kill()
				<-done
			}
		})
		return cmd, done, nil
	}
	resetHome := t.TempDir()
	resetWitnesses := repairResetHome(t, resetHome)
	mustWrite(t, filepath.Join(resetHome, "data", "sentinel"), "retained while process is live")
	first, firstDone, err := start()
	if err != nil {
		t.Fatal(err)
	}
	repairWaitRPC(t, endpoint)
	config := NodeProcessConfig{Name: "paxd", HomeDir: home, RPCEndpoint: endpoint}
	pid, err := (seidStartFinder{config: config}).FindPID("paxd")
	if err != nil || pid != first.Process.Pid {
		t.Fatalf("actual paxd child invisible: pid=%d want=%d err=%v", pid, first.Process.Pid, err)
	}
	// Its RPC is on a non-default port. A failed default probe must not grant
	// reset authority when the production finder can see the live process.
	if _, err := NewResetDataer(resetHome).Handler()(context.Background(), map[string]any{"rpcServers": resetWitnesses}); err == nil {
		t.Fatal("reset accepted a live paxd process with unavailable default RPC")
	}
	if !exists(filepath.Join(resetHome, "data", "sentinel")) {
		t.Fatal("live-process refusal erased data")
	}
	if _, err := NewStopSeider(config).Handler()(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	select {
	case <-firstDone:
	case <-time.After(5 * time.Second):
		t.Fatal("stop task did not terminate paxd")
	}
	second, secondDone, err := start()
	if err != nil {
		t.Fatal(err)
	}
	repairWaitRPC(t, endpoint)
	// A supervisor owns process creation; the production restart handler stops
	// the real child and awaits its replacement through the RPC transport.
	next := make(chan *exec.Cmd, 1)
	restartErr := make(chan error, 1)
	go func() {
		<-secondDone
		cmd := exec.Command(binary, "start", "--home", home, "--rpc", ports[0], "repair-326-peer")
		cmd.Stdout = log
		cmd.Stderr = log
		if err := cmd.Start(); err != nil {
			restartErr <- err
			return
		}
		next <- cmd
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	_, restartFailure := NewRestartSeider(config).Handler()(ctx, nil)
	var third *exec.Cmd
	select {
	case third = <-next:
	case err := <-restartErr:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal("supervisor did not restart paxd")
	}
	defer func() { _ = third.Process.Signal(syscall.SIGTERM); _ = third.Wait() }()
	if restartFailure != nil {
		t.Fatal(restartFailure)
	}
	if third.Process.Pid == second.Process.Pid {
		t.Fatal("restart did not replace the real daemon")
	}
	// The published await-condition action uses the same configured discovery.
	waiter := NewConditionWaiter(nil, config)
	if waiter.rpc.Endpoint() != endpoint {
		t.Fatal("await-condition ignored explicit RPC configuration")
	}
	if _, err := waiter.Handler()(ctx, map[string]any{"condition": conditionHeight, "targetHeight": 1, "action": actionSIGTERM}); err != nil {
		t.Fatal(err)
	}
}
