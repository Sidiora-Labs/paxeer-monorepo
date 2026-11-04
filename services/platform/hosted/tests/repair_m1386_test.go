package hostedtests

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	coretypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/bech32"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
	"gopkg.in/yaml.v3"
)

func repair1386Write(t *testing.T, path string, raw []byte) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, raw, 0o600))
}

func repair1386JSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	return raw
}

func repair1386Archive(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	for name, raw := range files {
		require.NoError(t, writer.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(raw))}))
		_, err := writer.Write(raw)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return buffer.Bytes()
}

func repair1386Unpack(t *testing.T, raw []byte) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	reader := tar.NewReader(bytes.NewReader(raw))
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return files
		}
		require.NoError(t, err)
		require.NotContains(t, files, header.Name)
		files[header.Name], err = io.ReadAll(reader)
		require.NoError(t, err)
	}
}

func TestRepairM_1386_DirtyImageProvenance(t *testing.T) {
	root := t.TempDir()
	repository, err := git.PlainInit(root, false)
	require.NoError(t, err)
	worktree, err := repository.Worktree()
	require.NoError(t, err)
	for _, name := range []string{"services/platform/hosted/input", "sdk/platform/input", "docker/kernel/Dockerfile"} {
		repair1386Write(t, filepath.Join(root, name), []byte("committed\n"))
		_, err := worktree.Add(name)
		require.NoError(t, err)
	}
	_, err = worktree.Commit("record build inputs", &git.CommitOptions{Author: &object.Signature{Name: "fixture", Email: "fixture@example.invalid", When: time.Unix(100, 0)}})
	require.NoError(t, err)
	var packed bytes.Buffer
	clean, err := SnapshotSource(root, &packed)
	require.NoError(t, err)
	require.False(t, clean.Dirty)
	require.Equal(t, clean.Revision[:12], clean.Tag)
	require.Equal(t, fmt.Sprintf("%x", sha256.Sum256(packed.Bytes())), clean.SourceTree)
	previous := clean
	for _, name := range []string{"services/platform/hosted/input", "sdk/platform/input"} {
		repair1386Write(t, filepath.Join(root, name), []byte("selected dirty input\n"))
		packed.Reset()
		dirty, err := SnapshotSource(root, &packed)
		require.NoError(t, err)
		require.True(t, dirty.Dirty, name)
		require.NotEqual(t, previous.SourceTree, dirty.SourceTree)
		require.Contains(t, dirty.Tag, "-dirty-"+dirty.SourceTree[:16])
		require.Equal(t, []byte("selected dirty input\n"), repair1386Unpack(t, packed.Bytes())[name])
		again, err := SnapshotSource(root, nil)
		require.NoError(t, err)
		require.Equal(t, dirty, again, "source metadata must describe the exact transported archive")
		previous = dirty
	}
}

func TestRepairM_1386_ImageBuildContexts(t *testing.T) {
	root, err := filepath.Abs("../../../..")
	require.NoError(t, err)
	// The real explorer recipe's COPY paths are rooted in the backend tree.
	recipe, err := os.ReadFile(filepath.Join(root, "docker/explorer-backend/Dockerfile"))
	require.NoError(t, err)
	mix, err := os.ReadFile(filepath.Join(root, "services/explorer/backend/mix.exs"))
	require.NoError(t, err)
	source := repair1386Archive(t, map[string][]byte{"docker/explorer-backend/Dockerfile": recipe, "services/explorer/backend/mix.exs": mix, "services/platform/unrelated": []byte("excluded")})
	plan, err := PlanImage("explorer-backend", "docker/explorer-backend/Dockerfile", "services/explorer/backend", "", "explorer.tar")
	require.NoError(t, err)
	var archive bytes.Buffer
	projected, err := ProjectImageContext(plan, bytes.NewReader(source), &archive)
	require.NoError(t, err)
	files := repair1386Unpack(t, archive.Bytes())
	require.Equal(t, recipe, files[projected.Dockerfile])
	require.Equal(t, mix, files["mix.exs"])
	require.NotContains(t, files, "services/platform/unrelated")
	require.Equal(t, fmt.Sprintf("%x", sha256.Sum256(archive.Bytes())), projected.SourceTree)
	prepared := t.TempDir()
	for _, name := range []string{"Dockerfile", "Dockerfile.dockerignore", "package.json", "package-lock.json", "rust-downloads.lock", "install-rust.sh", "cargo-config.toml", "layerx-rustc", "layerx-build"} {
		source := filepath.Join(root, "services/platform/hosted/registry/builder-environment", name)
		if strings.HasPrefix(name, "Dockerfile") {
			source = filepath.Join(root, "docker/platform-registry-builder", name)
		}
		raw, err := os.ReadFile(source)
		require.NoError(t, err)
		repair1386Write(t, filepath.Join(prepared, name), raw)
	}
	vendor, err := os.ReadFile(filepath.Join(root, "programs/vendor/wasm-instrument/src/lib.rs"))
	require.NoError(t, err)
	repair1386Write(t, filepath.Join(prepared, "vendor/wasm-instrument/src/lib.rs"), vendor)
	plan, err = PlanImage("platform-registry-builder", "docker/platform-registry-builder/Dockerfile", "@registry-builder", prepared, "builder.tar")
	require.NoError(t, err)
	archive.Reset()
	projected, err = ProjectImageContext(plan, bytes.NewReader(source), &archive)
	require.NoError(t, err)
	files = repair1386Unpack(t, archive.Bytes())
	require.Contains(t, string(files[projected.Dockerfile]), "COPY")
	require.Contains(t, files, "layerx-build")
	require.Equal(t, vendor, files["vendor/wasm-instrument/src/lib.rs"])
	require.NotContains(t, files, "mix.exs")
	require.NoError(t, os.Remove(filepath.Join(prepared, "layerx-build")))
	_, err = ProjectImageContext(plan, bytes.NewReader(source), io.Discard)
	require.ErrorContains(t, err, "layerx-build")
	_, err = PlanImage("platform-registry-builder", "docker/platform-registry-builder/Dockerfile", "@registry-builder", "", "bad")
	require.Error(t, err)
}

func repair1386Home(t *testing.T) (string, TreasuryPolicy) {
	t.Helper()
	home := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(home, "config"), 0o700))
	require.NoError(t, os.Mkdir(filepath.Join(home, "data"), 0o700))
	validator, err := privval.GenFilePV(filepath.Join(home, "config/priv_validator_key.json"), filepath.Join(home, "data/priv_validator_state.json"), "")
	require.NoError(t, err)
	require.NoError(t, validator.Save())
	require.NoError(t, tmtypes.GenNodeKey().SaveAs(filepath.Join(home, "config/node_key.json")))
	policy := TreasuryPolicy{Version: 1, NetworkID: 125, AssetID: strings.Repeat("12", 32), Recipient: strings.Repeat("34", 20)}
	account, err := bech32.ConvertAndEncode("pax", bytes.Repeat([]byte{0x34}, 20))
	require.NoError(t, err)
	genesis := map[string]any{"chain_id": NativeChainID, "app_state": map[string]any{
		"bank": map[string]any{"balances": []any{map[string]any{"address": account, "coins": []any{map[string]string{"denom": "uhpx", "amount": "100000000000000000000"}}}}}, "lxphost": map[string]any{}, "evm": map[string]any{},
	}}
	repair1386Write(t, filepath.Join(home, "config/genesis.json"), repair1386JSON(t, genesis))
	repair1386Write(t, filepath.Join(home, "config/receipt.key"), []byte(hex.EncodeToString(bytes.Repeat([]byte{3}, 32))))
	repair1386Write(t, filepath.Join(home, "config/config.toml"), []byte("mode = \"validator\"\ndb-dir = \"data\"\ngenesis-file = \"config/genesis.json\"\nnode-key-file = \"config/node_key.json\"\n[priv-validator]\nkey-file = \"config/priv_validator_key.json\"\nstate-file = \"data/priv_validator_state.json\"\nladdr = \"\"\n"))
	repair1386Write(t, filepath.Join(home, "config/app.toml"), []byte("minimum-gas-prices = \"0.01uhpx\"\n"))
	require.NoError(t, ConfigureNativeHome(home, account, policy))
	return home, policy
}

func TestRepairM_1386_RetainedNativeMaterial(t *testing.T) {
	home, policy := repair1386Home(t)
	directory := filepath.Join(t.TempDir(), "material")
	material, err := SaveNativeMaterial(home, directory, policy)
	require.NoError(t, err)
	secret, err := material.Secret("layerx-testnet")
	require.NoError(t, err)
	transport := repair1386JSON(t, secret)
	var decoded struct {
		Data map[string]string `json:"data"`
	}
	require.NoError(t, json.Unmarshal(transport, &decoded))
	genesis, err := base64.StdEncoding.DecodeString(decoded.Data["genesis.json"])
	require.NoError(t, err)
	require.Contains(t, string(genesis), NativeChainID)
	pins, err := base64.StdEncoding.DecodeString(decoded.Data["receipt-keys.json"])
	require.NoError(t, err)
	keys, err := LoadGatewayReceiptKeys(pins)
	require.NoError(t, err)
	receipt, err := os.ReadFile(repair142Receipt(t, t.TempDir(), 3, keys[0], 1))
	require.NoError(t, err)
	_, err = keys.VerifyReceipt(receipt)
	require.NoError(t, err)
	original := repair1386JSON(t, material)
	material.Close()
	_, err = material.Secret("layerx-testnet")
	require.Error(t, err)
	// Discard the original producer home: reopen uses only saved native state.
	require.NoError(t, os.RemoveAll(home))
	reopened, err := OpenNativeMaterial(directory, policy)
	require.NoError(t, err)
	defer reopened.Close()
	require.Equal(t, original, repair1386JSON(t, reopened))
	// Consume the actual Secret bytes with the validator's real retained-file
	// decoder. Advance and persist its anti-double-sign state, then reopen it.
	installed := t.TempDir()
	for _, name := range []string{"priv_validator_key.json", "priv_validator_state.json"} {
		raw, err := base64.StdEncoding.DecodeString(decoded.Data[name])
		require.NoError(t, err)
		repair1386Write(t, filepath.Join(installed, name), raw)
	}
	keyFile, stateFile := filepath.Join(installed, "priv_validator_key.json"), filepath.Join(installed, "priv_validator_state.json")
	validator, err := privval.LoadFilePV(keyFile, stateFile)
	require.NoError(t, err)
	vote := &tmproto.Vote{Type: tmproto.PrecommitType, Height: 5, Round: 0, Timestamp: time.Unix(100, 0), ValidatorAddress: validator.Key.Address, ValidatorIndex: 0}
	require.NoError(t, validator.SignVote(context.Background(), NativeChainID, vote))
	validator, err = privval.LoadFilePV(keyFile, stateFile)
	require.NoError(t, err)
	require.Equal(t, int64(5), validator.LastSignState.Height)
	vote.Height = 4
	require.ErrorContains(t, validator.SignVote(context.Background(), NativeChainID, vote), "regression")

	_, err = os.Stat(filepath.Join(directory, "publication-authorization.json"))
	require.ErrorIs(t, err, os.ErrNotExist)
	changed := policy
	changed.Recipient = strings.Repeat("56", 20)
	_, err = OpenNativeMaterial(directory, changed)
	require.ErrorContains(t, err, "treasury policy changed")
	repair1386Write(t, filepath.Join(directory, "genesis.json"), bytes.ReplaceAll(genesis, []byte(NativeChainID), []byte("retired-chain")))
	_, err = OpenNativeMaterial(directory, policy)
	require.ErrorContains(t, err, "genesis.json")
}

func TestRepairM_1386_NativeBootstrapCommittedReadiness(t *testing.T) {
	home, policy := repair1386Home(t)
	material, err := SaveNativeMaterial(home, filepath.Join(t.TempDir(), "native"), policy)
	require.NoError(t, err)
	defer material.Close()
	keysRaw, err := os.ReadFile(filepath.Join(material.directory, "receipt-keys.json"))
	require.NoError(t, err)
	keys, err := LoadGatewayReceiptKeys(keysRaw)
	require.NoError(t, err)
	wire, err := os.ReadFile(repair142Receipt(t, t.TempDir(), 3, keys[0], 1))
	require.NoError(t, err)
	signed, err := keys.VerifyReceipt(wire)
	require.NoError(t, err)
	transaction := "0x" + hex.EncodeToString(signed.Receipt.ActivityID[:])
	blockHash := fmt.Sprintf("%x", sha256.Sum256([]byte("recorded consensus block")))
	validator, err := privval.LoadFilePV(filepath.Join(home, "config/priv_validator_key.json"), filepath.Join(home, "data/priv_validator_state.json"))
	require.NoError(t, err)
	vote := &tmproto.Vote{Type: tmproto.PrecommitType, Height: 2, Round: 0, Timestamp: time.Unix(1000, 0), ValidatorAddress: validator.Key.Address, ValidatorIndex: 0, BlockID: tmproto.BlockID{Hash: must1386Hex(t, blockHash), PartSetHeader: tmproto.PartSetHeader{Total: 1, Hash: must1386Hex(t, blockHash)}}}
	require.NoError(t, validator.SignVote(context.Background(), NativeChainID, vote))
	height := uint64(2)
	rung := "committed"
	requests := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/", r.URL.Path)
		var request struct {
			ID     string `json:"id"`
			Method string `json:"method"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		requests[request.Method]++
		var result any
		switch request.Method {
		case "status":
			result = &coretypes.ResultStatus{NodeInfo: tmtypes.NodeInfo{Network: NativeChainID}, SyncInfo: coretypes.SyncInfo{LatestBlockHeight: int64(height), LastCommittedBlockHeight: int64(height), LatestBlockHash: must1386Hex(t, blockHash), CatchingUp: false}}
		case "commit":
			signedVote, err := tmtypes.VoteFromProto(vote)
			require.NoError(t, err)
			result = coretypes.NewResultCommit(&tmtypes.Header{ChainID: NativeChainID, Height: int64(height), AppHash: must1386Hex(t, blockHash)}, &tmtypes.Commit{Height: int64(height), Round: 0, BlockID: tmtypes.BlockID{Hash: must1386Hex(t, blockHash)}, Signatures: []tmtypes.CommitSig{signedVote.CommitSig()}}, true)
		case "eth_chainId":
			result = "0x7d"
		case "eth_getBlockByNumber":
			result = map[string]string{"number": fmt.Sprintf("0x%x", height), "hash": "0x" + blockHash}
		case "pax_sequencerReceipt":
			result = map[string]string{"rung": rung, "txHash": transaction, "height": "0x1", "receipt": "0x" + hex.EncodeToString(wire), "blockNumber": fmt.Sprintf("0x%x", height), "blockHash": "0x" + blockHash}
		default:
			t.Errorf("unexpected RPC %s", request.Method)
			w.WriteHeader(400)
			return
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result}))
	}))
	checkpoint := filepath.Join(t.TempDir(), "ready.json")
	readiness, err := NewNativeReadiness(material, checkpoint)
	require.NoError(t, err)
	state, err := readiness.Probe(context.Background(), server.URL, server.URL, transaction)
	require.NoError(t, err)
	require.Equal(t, uint64(2), state.Height)
	for _, method := range []string{"status", "commit", "eth_chainId", "eth_getBlockByNumber", "pax_sequencerReceipt"} {
		require.Positive(t, requests[method], method)
	}
	readiness.Close()
	server.Close()
	readiness, err = NewNativeReadiness(material, checkpoint)
	require.NoError(t, err)
	defer readiness.Close()
	// Closed real loopback transport cannot become cached healthy after restart.
	_, err = readiness.Probe(context.Background(), server.URL, server.URL, transaction)
	require.Error(t, err)
	// Reopen the transport with the same recorded peer transcript.
	server = httptest.NewServer(server.Config.Handler)
	defer server.Close()
	recovered, err := readiness.Probe(context.Background(), server.URL, server.URL, transaction)
	require.NoError(t, err)
	require.Equal(t, state, recovered)
	rung = "receipted"
	_, err = readiness.Probe(context.Background(), server.URL, server.URL, transaction)
	require.ErrorContains(t, err, "not reached committed")
	rung = "committed"
	height = 1
	_, err = readiness.Probe(context.Background(), server.URL, server.URL, transaction)
	require.ErrorContains(t, err, "rolled back")
	height = 2
	blockHash = strings.Repeat("ab", 32)
	_, err = readiness.Probe(context.Background(), server.URL, server.URL, transaction)
	require.ErrorContains(t, err, "diverged")
	_, err = readiness.Probe(context.Background(), server.URL+"/rpc", server.URL, transaction)
	require.ErrorContains(t, err, "root JSON-RPC")
}

func TestRepairM_1386_NativeImageSelection(t *testing.T) {
	inventory, err := os.ReadFile("beta-images.sh")
	require.NoError(t, err)
	list := strings.Split(strings.Split(string(inventory), "IMAGE_NAMES=(")[1], ")")[0]
	names := strings.Fields(list)
	for _, retired := range []string{"layerx-node", "layerx-core-boundary", "layerx-receipt-authority"} {
		require.NotContains(t, names, retired)
	}
	require.Contains(t, names, "kernel")
	require.Contains(t, names, "paxd-node")
	manifest, err := os.ReadFile("../node/deployment.yaml")
	require.NoError(t, err)
	decoder := yaml.NewDecoder(bytes.NewReader(manifest))
	found := false
	for {
		var document map[string]any
		err := decoder.Decode(&document)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		if document["kind"] != "StatefulSet" {
			continue
		}
		found = true
		pod := document["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)
		containers := pod["containers"].([]any)
		require.Len(t, containers, 1)
		runtime := containers[0].(map[string]any)
		require.Equal(t, "paxd", runtime["name"])
		require.Equal(t, "ghcr.io/sidiora-labs/kernel:0.1.0", runtime["image"])
		require.Contains(t, fmt.Sprint(runtime["command"]), "kernel_runtime.py")
		require.Contains(t, fmt.Sprint(runtime["readinessProbe"]), "/readyz")
		require.NotContains(t, string(manifest), "bootstrap.sh")
	}
	require.True(t, found)
	home, policy := repair1386Home(t)
	material, err := SaveNativeMaterial(home, filepath.Join(t.TempDir(), "inputs"), policy)
	require.NoError(t, err)
	defer material.Close()
	secret, err := material.Secret("layerx-testnet")
	require.NoError(t, err)
	require.Equal(t, "layerx-native-inputs", secret["metadata"].(map[string]string)["name"])
	require.Contains(t, string(manifest), "secretName: layerx-native-inputs")
}

func TestRepairM_1386_MirrorKeypairDerived(t *testing.T) {
	seed, err := hex.DecodeString("9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60")
	require.NoError(t, err)
	private := ed25519.NewKeyFromSeed(seed)
	require.Equal(t, "d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a", hex.EncodeToString(private[32:]))
	values := make([]int, len(private))
	for i, b := range private {
		values[i] = int(b)
	}
	raw := repair1386JSON(t, values)
	key, err := SolanaPublicKey(raw)
	require.NoError(t, err)
	require.NotEmpty(t, key)
	require.Len(t, key, 44)
	message := []byte("mirror deployment identity")
	require.True(t, ed25519.Verify(private.Public().(ed25519.PublicKey), message, ed25519.Sign(private, message)))
	values[63] ^= 1
	_, err = SolanaPublicKey(repair1386JSON(t, values))
	require.ErrorContains(t, err, "does not match")
	for _, value := range []string{"true", "1.0", "null", "-1", "256", "\"1\""} {
		bad := []byte("[" + value + "," + strings.SplitN(string(raw), ",", 2)[1])
		_, err = SolanaPublicKey(bad)
		require.Error(t, err, value)
	}
	_, err = SolanaPublicKey(raw[:len(raw)-2])
	require.Error(t, err)
}

func TestRepairM_1386_GatewayPinsPersistedRotation(t *testing.T) {
	directory := t.TempDir()
	database, err := dbm.NewGoLevelDB("pins", directory)
	require.NoError(t, err)
	mount := sdk.NewKVStoreKey(seqtypes.StoreKey)
	multi := store.NewCommitMultiStore(database)
	multi.MountStoreWithDB(mount, sdk.StoreTypeIAVL, database)
	require.NoError(t, multi.LoadLatestVersion())
	ctx := sdk.NewContext(multi, tmproto.Header{ChainID: NativeChainID, Height: 1}, false)
	keeper := seqkeeper.NewKeeper(mount)
	primary := repair142Pin(3, 1, math.MaxUint64)
	require.NoError(t, keeper.SetReceiptKeys(ctx, seqkeeper.ReceiptKeys{primary}))
	multi.Commit(true)
	rotated := primary
	rotated.LastHeight = 6999
	standby := repair142Pin(4, 7000, math.MaxUint64)
	require.NoError(t, seqkeeper.ValidateRotation(seqkeeper.ReceiptKeys{primary}, seqkeeper.ReceiptKeys{rotated, standby}, 6000))
	require.NoError(t, keeper.SetReceiptKeys(ctx, seqkeeper.ReceiptKeys{rotated, standby}))
	multi.Commit(true)
	stored, err := keeper.ReceiptKeys(ctx)
	require.NoError(t, err)
	pins := make(seqkeeper.ReceiptKeys, len(stored))
	for i, pin := range stored {
		pins[i] = seqkeeper.ReceiptKeyEntry{KeyId: pin.KeyId[:], PublicKey: pin.PublicKey[:], FirstHeight: pin.FirstHeight, LastHeight: pin.LastHeight}
	}
	projected, err := GatewayReceiptKeys(pins)
	require.NoError(t, err)
	path := filepath.Join(directory, "receipt-keys.json")
	repair1386Write(t, path, projected)
	require.NoError(t, multi.Close())
	database, err = dbm.NewGoLevelDB("pins", directory)
	require.NoError(t, err)
	multi = store.NewCommitMultiStore(database)
	defer multi.Close()
	multi.MountStoreWithDB(mount, sdk.StoreTypeIAVL, database)
	require.NoError(t, multi.LoadLatestVersion())
	ctx = sdk.NewContext(multi, tmproto.Header{ChainID: NativeChainID, Height: 7000}, false)
	recovered, err := keeper.ReceiptKeys(ctx)
	require.NoError(t, err)
	require.Equal(t, stored, recovered)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	consumer, err := LoadGatewayReceiptKeys(raw)
	require.NoError(t, err)
	for _, test := range []struct {
		seed   byte
		pin    seqkeeper.ReceiptKeyEntry
		height int64
		accept bool
	}{{3, primary, 1, true}, {3, primary, 6999, true}, {4, standby, 7000, true}, {3, primary, 7000, false}, {4, standby, 6999, false}} {
		receipt, err := os.ReadFile(repair142Receipt(t, t.TempDir(), test.seed, test.pin, test.height))
		require.NoError(t, err)
		_, err = consumer.VerifyReceipt(receipt)
		if test.accept {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}
	_, err = LoadGatewayReceiptKeys(bytes.Replace(raw, []byte(`"firstHeight":1`), []byte(`"firstHeight":2,"firstHeight":1`), 1))
	require.Error(t, err)
}

func TestRepairM_1386_TopologyShortDNSRefusals(t *testing.T) {
	var topology TopologyInput
	raw := []byte(`{"workloads":[{"kind":"Deployment","ns":"beta","name":"caller","labels":{"app":"caller"}},{"kind":"Deployment","ns":"beta","name":"native","labels":{"app":"native"},"ports":{"rpc":["8545","TCP"]}}],"services":[{"ns":"beta","name":"native","selector":{"app":"native"},"ports":[{"port":"8545","targetPort":"rpc","protocol":"TCP"}]}],"policies":[{"ns":"beta","name":"native-deny","selector":{"matchLabels":{"app":"native"}},"types":["Ingress"],"ingress":[]}]}`)
	require.NoError(t, json.Unmarshal(raw, &topology))
	caller := topology.Workloads[0]
	for _, host := range []string{"native", "native.beta", "native.beta.svc", "native.beta.svc.cluster.local."} {
		topology.Edges = []TopologyEdge{{Caller: caller, Host: host, Port: "8545", Source: "env RPC"}}
		result := CheckTopology(topology)
		require.Len(t, result, 1)
		require.Equal(t, "FAIL", result[0][0], host)
		require.Contains(t, result[0][2], "ingress NetworkPolicy")
		topology.Policies = nil
		require.Equal(t, "ok", CheckTopology(topology)[0][0], host)
		topology.Policies = []TopologyPolicy{{NS: "beta", Name: "caller-deny", Selector: TopologySelector{Labels: map[string]string{"app": "caller"}}, Types: []string{"Egress"}}}
		require.Contains(t, CheckTopology(topology)[0][2], "egress NetworkPolicy")
		topology.Policies[0].Egress = []TopologyRule{{}}
		require.Equal(t, "ok", CheckTopology(topology)[0][0])
		// Allow RPC only: DNS must still be admitted explicitly.
		require.NoError(t, json.Unmarshal([]byte(`[{"ns":"beta","name":"rpc-only","selector":{"matchLabels":{"app":"caller"}},"types":["Egress"],"egress":[{"ports":[{"port":8545,"protocol":"TCP"}]}]}]`), &topology.Policies))
		require.Contains(t, CheckTopology(topology)[0][2], "does not admit DNS")
		require.NoError(t, json.Unmarshal(raw, &topology))
	}
	topology.Edges = []TopologyEdge{{Caller: caller, Host: "missing", Port: "8545"}}
	require.Contains(t, CheckTopology(topology)[0][2], "no Service beta/missing")
	topology.Edges[0].Host = "rpc.external.example"
	require.Equal(t, "FAIL", CheckTopology(topology)[0][0])
	topology.ExternalHosts = []string{"rpc.external.example"}
	require.Equal(t, "note", CheckTopology(topology)[0][0])
}

func must1386Hex(t *testing.T, value string) []byte {
	t.Helper()
	raw, err := hex.DecodeString(value)
	require.NoError(t, err)
	return raw
}
