package evmrpc_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	chainskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/keeper"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	seqnode "github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	evmrpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	occutils "github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
	"go.opentelemetry.io/otel"
)

const (
	seqReceiptChainID9005  = "sequencer-receipt-test-1"
	seqReceiptModuleID9005 = uint16(12)
	seqReceiptFeeDenom9005 = "upax"
)

var seqReceiptSeqDomain9005 = []byte("PAXEER/v1/sequence\x00")

// seqReceiptChain9005 is a sequencer host: the signed receipts it hands out and the committed
// sequencer store whose MsgSequenceBatch history the API reads.
type seqReceiptChain9005 struct {
	t          *testing.T
	mu         sync.Mutex
	ms         storetypes.CommitMultiStore
	key        storetypes.StoreKey
	keeper     seqkeeper.Keeper
	latest     int64
	snapshots  map[int64]storetypes.CommitMultiStore
	seed       []byte
	keyID      [32]byte
	receiptKey *seqnode.ReceiptKey
	lastSeq    uint64
	seqHashes  map[uint64][32]byte
	activities map[uint64][32]byte
	committed  uint64
	sends      map[common.Hash]*evmrpc.SequencerSend
}

func newSeqReceiptChain9005(t *testing.T) *seqReceiptChain9005 {
	t.Helper()
	db := dbm.NewMemDB()
	ms := store.NewCommitMultiStore(db)
	key := sdk.NewKVStoreKey(seqtypes.StoreKey)
	chainsKey := sdk.NewKVStoreKey(chainstypes.StoreKey)
	ms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(chainsKey, sdk.StoreTypeIAVL, db)
	require.NoError(t, ms.LoadLatestVersion())

	seed := bytes.Repeat([]byte{0x21}, ed25519.SeedSize)
	public := [32]byte(ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey))
	var keyID [32]byte
	copy(keyID[:], bytes.Repeat([]byte{0x44}, 32))
	var leader [32]byte
	copy(leader[:], bytes.Repeat([]byte{0x55}, 32))

	dir := t.TempDir()
	keyFile := filepath.Join(dir, "receipt_key")
	require.NoError(t, os.WriteFile(keyFile, []byte(hex.EncodeToString(seed)+"\n"), 0o600))
	pv, err := privval.GenFilePV(filepath.Join(dir, "priv_validator_key.json"), filepath.Join(dir, "priv_validator_state.json"), "")
	require.NoError(t, err)
	require.NoError(t, pv.Key.Save())
	receiptKey, err := seqnode.LoadReceiptKey(keyFile, filepath.Join(dir, "priv_validator_key.json"),
		[]verify.SequencerAuthorization{{SequencerID: keyID, PublicKey: public, FirstBatchNumber: 1, LastBatchNumber: 1999}}, 7)
	require.NoError(t, err)

	c := &seqReceiptChain9005{
		t:          t,
		ms:         ms,
		key:        key,
		keeper:     seqkeeper.NewKeeper(key),
		snapshots:  map[int64]storetypes.CommitMultiStore{},
		seed:       seed,
		keyID:      keyID,
		receiptKey: receiptKey,
		seqHashes:  map[uint64][32]byte{0: {}},
		activities: map[uint64][32]byte{},
		sends:      map[common.Hash]*evmrpc.SequencerSend{},
	}
	chains, err := chainskeeper.NewKeeper(EVMKeeper.AccountKeeper().GetCodec(), chainsKey, EVMKeeper.BankKeeper(), EVMKeeper.GetBaseDenom)
	require.NoError(t, err)
	ctx := c.writeCtx9005(1)
	require.NoError(t, chains.InitGenesis(ctx, *chainstypes.DefaultGenesis()))
	root := chainstypes.Domain{Id: 0, Name: "root", Status: chainstypes.StatusActive, Bond: sdk.ZeroInt()}
	rootJSON, err := json.Marshal(root)
	require.NoError(t, err)
	ctx.KVStore(chainsKey).Set(chainstypes.DomainKey(root.Id), rootJSON)
	ctx.KVStore(chainsKey).Set(chainstypes.NameIndexKey(root.Name), binary.BigEndian.AppendUint32(nil, root.Id))
	require.NoError(t, c.keeper.InitGenesis(c.writeCtx9005(1), seqtypes.GenesisState{
		Leaders:     [][32]byte{leader},
		ReceiptKeys: []seqtypes.ReceiptKey{{KeyId: keyID, PublicKey: public, FirstHeight: 1, LastHeight: 1999}},
	}))
	for height := int64(1); height <= 6; height++ {
		c.commit9005(height)
		domain, found := chains.GetDomain(c.ctxProvider(height), 0)
		require.True(t, found)
		require.Equal(t, root, domain)
	}
	return c
}

// commit9005 commits the block at height and keeps a copy of the committed sequencer store as
// the state at that height: the legacy root store serves only its latest version.
func (c *seqReceiptChain9005) commit9005(height int64) {
	c.t.Helper()
	require.Equal(c.t, height, c.ms.Commit(true).Version)
	db := dbm.NewMemDB()
	snapshot := store.NewCommitMultiStore(db)
	for _, key := range c.ms.StoreKeys() {
		snapshot.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	require.NoError(c.t, snapshot.LoadLatestVersion())
	for _, key := range c.ms.StoreKeys() {
		src, dst := c.ms.GetKVStore(key), snapshot.GetKVStore(key)
		it := src.Iterator(nil, nil)
		for ; it.Valid(); it.Next() {
			dst.Set(bytes.Clone(it.Key()), bytes.Clone(it.Value()))
		}
		require.NoError(c.t, it.Close())
	}
	snapshot.Commit(true)
	c.snapshots[height], c.latest = snapshot, height
}

func (c *seqReceiptChain9005) writeCtx9005(height int64) sdk.Context {
	return sdk.NewContext(c.ms, tmproto.Header{ChainID: seqReceiptChainID9005, Height: height}, false).
		WithChainID(seqReceiptChainID9005).WithBlockHeight(height).WithTxIndex(0)
}

// ctxProvider serves the committed sequencer store at a height, the latest one for LatestCtxHeight.
func (c *seqReceiptChain9005) ctxProvider(height int64) sdk.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	if height == evmrpc.LatestCtxHeight {
		height = c.latest
	}
	snapshot, ok := c.snapshots[height]
	if !ok {
		panic(fmt.Sprintf("no committed sequencer state at height %d", height))
	}
	return sdk.NewContext(snapshot.CacheMultiStore(), tmproto.Header{ChainID: seqReceiptChainID9005, Height: height}, false).
		WithChainID(seqReceiptChainID9005).WithBlockHeight(height)
}

func (c *seqReceiptChain9005) SequencerSend(txHash common.Hash) (*evmrpc.SequencerSend, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sends[txHash], nil
}

func seqReceiptNextHash9005(prev [32]byte, seq uint64, activity [32]byte) [32]byte {
	preimage := append(bytes.Clone(seqReceiptSeqDomain9005), prev[:]...)
	preimage = binary.BigEndian.AppendUint64(preimage, seq)
	return sha256.Sum256(append(preimage, activity[:]...))
}

// seqReceiptTx9005 describes one executed transaction of a test receipt.
type seqReceiptTx9005 struct {
	nonce      uint64
	height     int64
	txIndex    int
	status     uint32
	returnData []byte
	logs       []*types.Log
	vmError    string
	create     bool
}

func seqReceiptTxHash9005(height int64, nonce uint64) common.Hash {
	return sha256.Sum256([]byte(fmt.Sprintf("sequencer receipt tx %d/%d", height, nonce)))
}

var (
	seqReceiptFrom9005     = common.HexToAddress("0x1000000000000000000000000000000000000001")
	seqReceiptTo9005       = common.HexToAddress("0x2000000000000000000000000000000000000002")
	seqReceiptSponsor9005  = common.HexToAddress("0x3000000000000000000000000000000000000003")
	seqReceiptCreated9005  = common.HexToAddress("0x4000000000000000000000000000000000000004")
	seqReceiptLogAddr9005  = common.HexToAddress("0x5000000000000000000000000000000000000005")
	seqReceiptTopicA9005   = common.HexToHash("0x00000000000000000000000000000000000000000000000000000000000000a1")
	seqReceiptTopicB9005   = common.HexToHash("0x00000000000000000000000000000000000000000000000000000000000000b2")
	seqReceiptFeeTotal9005 = big.NewInt(4_200_000)
)

// signReceipt9005 signs the receipt of tx at seq chained from prev without recording it.
func (c *seqReceiptChain9005) signReceipt9005(tx seqReceiptTx9005, seq uint64, prev [32]byte, activity [32]byte) []byte {
	c.t.Helper()
	evm := &types.Receipt{
		Status:            tx.status,
		GasUsed:           21_000 + uint64(len(tx.logs))*375,
		CumulativeGasUsed: 63_000 + uint64(len(tx.logs))*375,
		EffectiveGasPrice: 200,
		To:                seqReceiptTo9005.Hex(),
		Logs:              tx.logs,
		VmError:           tx.vmError,
	}
	to := seqReceiptTo9005
	if tx.create {
		evm.To, evm.ContractAddress, to = "", seqReceiptCreated9005.Hex(), common.Address{}
	}
	signed, err := c.receiptKey.SignReceipt(&seqnode.ReceiptInput{
		Accepted: &seqnode.Accepted{
			Seq:         seq,
			PrevSeqHash: prev,
			SeqHash:     seqReceiptNextHash9005(prev, seq, activity),
			ActivityID:  activity,
			Height:      tx.height,
			TxIndex:     tx.txIndex,
			TimestampMs: 1_700_000_000_000 + tx.height*400,
		},
		EVM:               evm,
		Operation:         2,
		ReturnData:        tx.returnData,
		PaymentDenom:      evmkeeper.BaseDenom,
		FeeDenom:          seqReceiptFeeDenom9005,
		FeeCharged:        seqReceiptFeeTotal9005,
		Sponsor:           seqReceiptSponsor9005,
		Amount:            big.NewInt(1_000),
		From:              seqReceiptFrom9005,
		FromBalanceBefore: big.NewInt(10_000_000),
		FromBalanceAfter:  big.NewInt(5_799_000),
		FromSequence:      tx.nonce,
		To:                to,
		ToBalanceBefore:   big.NewInt(0),
		ToBalanceAfter:    big.NewInt(1_000),
		ModuleID:          seqReceiptModuleID9005,
	})
	require.NoError(c.t, err)
	return signed
}

func seqReceiptLogData9005(logs []*types.Log) [][]byte {
	data := make([][]byte, len(logs))
	for i, log := range logs {
		data[i] = log.Data
	}
	return data
}

// accept9005 sequences tx as the next seq, signs its receipt and records the send.
func (c *seqReceiptChain9005) accept9005(tx seqReceiptTx9005) (common.Hash, uint64) {
	c.t.Helper()
	hash := seqReceiptTxHash9005(tx.height, tx.nonce)
	c.mu.Lock()
	seq, prev := c.lastSeq+1, c.seqHashes[c.lastSeq]
	c.mu.Unlock()
	signed := c.signReceipt9005(tx, seq, prev, hash)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastSeq = seq
	c.seqHashes[seq] = seqReceiptNextHash9005(prev, seq, hash)
	c.activities[seq] = hash
	c.sends[hash] = &evmrpc.SequencerSend{
		Nonce:      tx.nonce,
		Receipt:    signed,
		ReturnData: tx.returnData,
		LogData:    seqReceiptLogData9005(tx.logs),
		VMError:    tx.vmError,
	}
	return hash, seq
}

func (c *seqReceiptChain9005) submit9005(hash common.Hash, nonce uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sends[hash] = &evmrpc.SequencerSend{Nonce: nonce}
}

// commitBatch9005 delivers the MsgSequenceBatch of height covering every seq above the committed
// one up to toSeq at tx index 0, and commits the block.
func (c *seqReceiptChain9005) commitBatch9005(height int64, toSeq uint64) {
	c.t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	from := c.committed + 1
	prev, last := c.seqHashes[c.committed], c.seqHashes[toSeq]
	msg := seqtypes.MsgSequenceBatch{
		Height:      uint64(height),
		FromSeq:     from,
		ToSeq:       toSeq,
		PrevSeqHash: bytes.Clone(prev[:]),
		LastSeqHash: bytes.Clone(last[:]),
		TxRoot:      bytes.Clone(seqtypes.EmptyTxRoot[:]),
		KeyId:       bytes.Clone(c.keyID[:]),
	}
	if !msg.IsEmpty() {
		var ids []byte
		for seq := from; seq <= toSeq; seq++ {
			id := c.activities[seq]
			ids = append(ids, id[:]...)
		}
		root := sha256.Sum256(ids)
		msg.TxRoot = root[:]
	}
	digest, err := msg.SignBytes(seqReceiptChainID9005)
	require.NoError(c.t, err)
	msg.Sig = ed25519.Sign(ed25519.NewKeyFromSeed(c.seed), digest[:])
	require.NoError(c.t, c.keeper.DeliverSequenceBatch(c.writeCtx9005(height), msg))
	exported, err := c.keeper.ExportGenesis(c.writeCtx9005(height))
	require.NoError(c.t, err)
	require.Equal(c.t, toSeq, exported.LastSeq)
	require.Equal(c.t, last, exported.LastSeqHash)
	c.commit9005(height)
	c.committed = toSeq
}

func seqReceiptFreePort9005(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", TestAddr+":0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	return port
}

// startSeqReceiptServer9005 starts an EVM HTTP server with the sequencer receipt option and
// returns its port.
func startSeqReceiptServer9005(t *testing.T, testApp *app.App, ctxProvider func(int64) sdk.Context, option evmrpc.SequencerReceiptOption) int {
	t.Helper()
	config := evmrpcconfig.DefaultConfig
	config.HTTPAddress = TestAddr
	config.HTTPPort = seqReceiptFreePort9005(t)
	server, err := evmrpc.NewEVMHTTPServer(config, &MockClient{}, EVMKeeper, testApp.BeginBlockKeepers, testApp.BaseApp,
		testApp.TracerAnteHandler, ctxProvider, func(int64) client.TxConfig { return TxConfig }, "", nil, option)
	require.NoError(t, err)
	require.NoError(t, server.Start())
	t.Cleanup(server.Stop)
	return config.HTTPPort
}

type seqReceiptRPCError9005 struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// callSeqReceipt9005 posts one JSON-RPC call and returns its raw result or its error.
func callSeqReceipt9005(t *testing.T, port int, method string, param ...any) (json.RawMessage, *seqReceiptRPCError9005) {
	t.Helper()
	params, err := json.Marshal(append([]any{}, param...))
	require.NoError(t, err)
	body := fmt.Sprintf(`{"jsonrpc":"2.0","method":%q,"params":%s,"id":1}`, method, params)
	res, err := http.Post(fmt.Sprintf("http://%s:%d", TestAddr, port), "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	var answer struct {
		Result json.RawMessage         `json:"result"`
		Error  *seqReceiptRPCError9005 `json:"error"`
	}
	require.NoError(t, json.Unmarshal(raw, &answer), string(raw))
	return answer.Result, answer.Error
}

func seqReceiptResult9005(t *testing.T, port int, hash common.Hash) (map[string]any, *evmrpc.SequencerReceiptResult, json.RawMessage) {
	t.Helper()
	raw, rpcErr := callSeqReceipt9005(t, port, evmrpc.SequencerReceiptMethod, hash)
	require.Nil(t, rpcErr)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(raw, &fields))
	var result *evmrpc.SequencerReceiptResult
	require.NoError(t, json.Unmarshal(raw, &result))
	return fields, result, raw
}

func seqReceiptTwoLogs9005() []*types.Log {
	return []*types.Log{
		{Address: seqReceiptLogAddr9005.Hex(), Topics: []string{seqReceiptTopicA9005.Hex(), seqReceiptTopicB9005.Hex()}, Data: []byte("transfer")},
		{Address: seqReceiptTo9005.Hex(), Topics: []string{}, Data: nil},
	}
}

func TestReceiptKeysNotServedNamesRequestedMethod(t *testing.T) {
	chain := newSeqReceiptChain9005(t)
	testApp := app.Setup(t, false, false, false)
	port := startSeqReceiptServer9005(t, testApp, chain.ctxProvider, evmrpc.SequencerReceiptOption{})
	unavailableURL, err := url.Parse(fmt.Sprintf("http://%s:%d", TestAddr, port))
	require.NoError(t, err)
	forwardPort := startSeqReceiptServer9005(t, testApp, chain.ctxProvider, evmrpc.SequencerReceiptOption{ForwardURL: unavailableURL})

	for _, node := range []struct {
		name string
		port int
	}{
		{name: "direct", port: port},
		{name: "forwarded", port: forwardPort},
	} {
		t.Run(node.name, func(t *testing.T) {
			_, rpcErr := callSeqReceipt9005(t, node.port, evmrpc.ReceiptKeysMethod)
			require.NotNil(t, rpcErr)
			require.Equal(t, -32000, rpcErr.Code)
			require.Equal(t, "px_getReceiptKeys served by sequencer", rpcErr.Message)
		})
	}

	_, rpcErr := callSeqReceipt9005(t, port, evmrpc.SequencerReceiptMethod, common.HexToHash("0x77"))
	require.NotNil(t, rpcErr)
	require.Equal(t, -32000, rpcErr.Code)
	require.Equal(t, "pax_sequencerReceipt served by sequencer", rpcErr.Message)
}

func TestPublicPxReceiptKeysCanonical(t *testing.T) {
	chain := newSeqReceiptChain9005(t)
	testApp := app.Setup(t, false, false, false)
	port := startSeqReceiptServer9005(t, testApp, chain.ctxProvider, evmrpc.SequencerReceiptOption{Source: chain, Keeper: &chain.keeper})
	unavailablePort := startSeqReceiptServer9005(t, testApp, chain.ctxProvider, evmrpc.SequencerReceiptOption{})
	_, unavailable := callSeqReceipt9005(t, unavailablePort, evmrpc.ReceiptKeysMethod)
	require.NotNil(t, unavailable)
	require.Equal(t, -32000, unavailable.Code)
	require.Equal(t, "px_getReceiptKeys served by sequencer", unavailable.Message)
	require.Nil(t, unavailable.Data)

	root, err := filepath.Abs("..")
	require.NoError(t, err)
	sdkPath := filepath.Join(root, "services", "agent", "sdk")
	const typescript = `
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire, registerHooks } from "node:module";
import { pathToFileURL, fileURLToPath } from "node:url";
const sdk = process.argv[1];
const require = createRequire(pathToFileURL(sdk + "/typescript/package.json"));
const ts = require("typescript");
const sourceRoot = pathToFileURL(sdk + "/typescript/src/").href;
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (context.parentURL?.startsWith(sourceRoot) && specifier.startsWith(".") && specifier.endsWith(".js"))
      specifier = specifier.slice(0, -3) + ".ts";
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (url.startsWith(sourceRoot) && url.endsWith(".ts")) {
      const source = ts.transpileModule(readFileSync(fileURLToPath(url), "utf8"), {
        compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext },
        fileName: fileURLToPath(url),
      }).outputText;
      return { format: "module", source, shortCircuit: true };
    }
    return nextLoad(url, context);
  },
});
const { PxClient, decodePxReceiptKeys } = await import(sourceRoot + "px.ts");
const { JsonRpcError, decodeJsonRpcResponse } = await import(sourceRoot + "rpc.ts");
const input = JSON.parse(readFileSync(0, "utf8"));
const normalize = keys => keys.map(key => {
  assert.equal(typeof key.firstHeight, "bigint");
  assert.equal(typeof key.lastHeight, "bigint");
  assert.equal(Object.isFrozen(key), true);
  assert.equal(Buffer.from(key.keyId, "hex").length, 32);
  assert.equal(Buffer.from(key.publicKey, "hex").length, 32);
  return { ...key, firstHeight: key.firstHeight.toString(), lastHeight: key.lastHeight.toString() };
});
const keys = await new PxClient({ endpoint: input.url }).getReceiptKeys();
assert.equal(Object.isFrozen(keys), true);
assert.deepEqual(normalize(keys), input.expected);
assert.deepEqual(normalize(decodePxReceiptKeys(input.document)), input.expected);
await assert.rejects(new PxClient({ endpoint: input.errorUrl }).getReceiptKeys(), error => {
  assert.ok(error instanceof JsonRpcError);
  assert.equal(error.code, input.error.code);
  assert.equal(error.message, input.error.message);
  assert.deepEqual(error.data ?? null, input.error.data);
  return true;
});
for (const mutation of input.mutations)
  assert.throws(() => decodePxReceiptKeys(mutation.value), { message: mutation.ts }, mutation.name);
for (const mutation of input.unsafe)
  assert.throws(() => decodePxReceiptKeys(mutation), { message: "Invalid gateway quantity" });
for (const mutation of input.valid)
  assert.deepEqual(normalize(decodePxReceiptKeys(mutation.value)), mutation.expected);
const envelope = { jsonrpc: "2.0", id: "1", result: input.document };
assert.deepEqual(decodeJsonRpcResponse(envelope, "1"), input.document);
for (const invalid of [
  { ...envelope, jsonrpc: "1.0" }, { ...envelope, id: "2" }, { ...envelope, id: 1 },
  { ...envelope, error: input.error }, { jsonrpc: "2.0", id: "1" },
]) assert.throws(() => decodeJsonRpcResponse(invalid, "1"));
console.log(JSON.stringify(normalize(keys)));
`
	const python = `
import json
import sys
from dataclasses import FrozenInstanceError
sys.path.insert(0, sys.argv[1] + "/python")
from layerx_sdk.px import PxClient, PxRpcError, decode_px_receipt_keys

document = json.load(sys.stdin)
def normalize(keys):
    assert isinstance(keys, tuple)
    result = []
    for key in keys:
        assert type(key.first_height) is int
        assert type(key.last_height) is int
        assert len(bytes.fromhex(key.key_id)) == 32
        assert len(bytes.fromhex(key.public_key)) == 32
        try:
            key.first_height = 0
        except FrozenInstanceError:
            pass
        else:
            raise AssertionError("mutable receipt key")
        result.append({"keyId": key.key_id, "publicKey": key.public_key,
                       "firstHeight": str(key.first_height), "lastHeight": str(key.last_height)})
    return result
keys = PxClient(document["url"]).get_receipt_keys()
assert normalize(keys) == document["expected"]
assert normalize(decode_px_receipt_keys(document["document"])) == document["expected"]
try:
    PxClient(document["errorUrl"]).get_receipt_keys()
except PxRpcError as error:
    assert error.code == document["error"]["code"]
    assert error.message == document["error"]["message"]
    assert error.data == document["error"]["data"]
else:
    raise AssertionError("non-serving node accepted")
for mutation in document["mutations"]:
    try:
        decode_px_receipt_keys(mutation["value"])
    except ValueError as error:
        assert str(error) == mutation["py"], (mutation["name"], str(error))
    else:
        raise AssertionError(mutation["name"])
for mutation in document["valid"]:
    assert normalize(decode_px_receipt_keys(mutation["value"])) == mutation["expected"]
print(json.dumps(normalize(keys)))
`
	public := chain.receiptKey.PublicKey()
	want := seqkeeper.ReceiptKeys{{KeyId: bytes.Clone(chain.keyID[:]), PublicKey: bytes.Clone(public[:]), FirstHeight: 1, LastHeight: 1999}}
	for _, phase := range []string{"initial", "rotated", "empty"} {
		t.Run(phase, func(t *testing.T) {
			if phase == "rotated" {
				secondPublic := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x31}, ed25519.SeedSize)).Public().(ed25519.PublicKey)
				want = append(want, seqkeeper.ReceiptKeyEntry{KeyId: bytes.Repeat([]byte{0x22}, 32), PublicKey: secondPublic, FirstHeight: 2000, LastHeight: 9007199254740991})
				require.NoError(t, chain.keeper.SetReceiptKeys(chain.writeCtx9005(7), want))
				chain.commit9005(7)
			} else if phase == "empty" {
				want = seqkeeper.ReceiptKeys{}
				require.NoError(t, chain.keeper.SetReceiptKeys(chain.writeCtx9005(8), want))
				chain.commit9005(8)
			}
			raw, rpcErr := callSeqReceipt9005(t, port, evmrpc.ReceiptKeysMethod)
			require.Nil(t, rpcErr)
			var captured map[string]any
			require.NoError(t, json.Unmarshal(raw, &captured))
			entries, ok := captured["keys"].([]any)
			require.True(t, ok, "the producer must emit an array, including when empty")
			require.Len(t, entries, len(want))
			expected := make([]map[string]string, 0, len(want))
			for _, key := range want {
				expected = append(expected, map[string]string{"keyId": hex.EncodeToString(key.KeyId), "publicKey": hex.EncodeToString(key.PublicKey), "firstHeight": fmt.Sprint(key.FirstHeight), "lastHeight": fmt.Sprint(key.LastHeight)})
			}
			mutations := make([]map[string]any, 0)
			unsafe := make([]any, 0)
			valid := make([]map[string]any, 0)
			mutate := func(change func(map[string]any, map[string]any)) map[string]any {
				var document map[string]any
				require.NoError(t, json.Unmarshal(raw, &document))
				entry := document["keys"].([]any)[0].(map[string]any)
				change(document, entry)
				return document
			}
			add := func(name string, value any, ts, py string) {
				mutations = append(mutations, map[string]any{"name": name, "value": value, "ts": ts, "py": py})
			}
			if len(entries) > 0 {
				for _, field := range []string{"keyId", "publicKey", "firstHeight", "lastHeight"} {
					add("missing "+field, mutate(func(_ map[string]any, entry map[string]any) { delete(entry, field) }), "Incomplete gateway document", "incomplete-px-document")
				}
				add("extra field", mutate(func(_ map[string]any, entry map[string]any) { entry["extra"] = true }), "Invalid gateway receipt key", "invalid-px-receipt-key")
				add("duplicate id", mutate(func(document, entry map[string]any) { document["keys"] = append(document["keys"].([]any), entry) }), "Invalid gateway receipt key", "invalid-px-receipt-key")
				add("duplicate mixed case id", mutate(func(document, entry map[string]any) {
					duplicate := make(map[string]any)
					for key, value := range entry {
						duplicate[key] = value
					}
					duplicate["keyId"] = "0x" + strings.ToUpper(entry["keyId"].(string)[2:])
					document["keys"] = append(document["keys"].([]any), duplicate)
				}), "Invalid gateway receipt key", "invalid-px-receipt-key")
				add("reversed range", mutate(func(_ map[string]any, entry map[string]any) { entry["firstHeight"] = entry["lastHeight"].(float64) + 1 }), "Invalid gateway receipt key range", "invalid-px-receipt-key-range")
				for _, field := range []string{"keyId", "publicKey"} {
					original := entries[0].(map[string]any)[field].(string)
					for _, value := range []any{nil, 1, original[2:], original[:len(original)-2], original + "00", "0x" + strings.Repeat("gg", 32), " " + original, original + "\n", original + "\r", original + "\r\n", strings.Repeat("A", 43) + "="} {
						add("malformed "+field, mutate(func(_ map[string]any, entry map[string]any) { entry[field] = value }), "Invalid gateway receipt key bytes", "invalid-px-receipt-key-bytes")
					}
					add("zero "+field, mutate(func(_ map[string]any, entry map[string]any) { entry[field] = "0x" + strings.Repeat("00", 32) }), "Invalid gateway receipt key", "invalid-px-receipt-key")
				}
				for _, field := range []string{"firstHeight", "lastHeight"} {
					for _, value := range []any{nil, true, -1, 1.5, "-1", "01", "0xgg", ""} {
						add("invalid "+field, mutate(func(_ map[string]any, entry map[string]any) { entry[field] = value }), "Invalid gateway quantity", "invalid-px-quantity")
					}
					add("overflow "+field, mutate(func(_ map[string]any, entry map[string]any) { entry[field] = "18446744073709551616" }), "Invalid gateway count", "invalid-px-count")
					unsafe = append(unsafe, mutate(func(_ map[string]any, entry map[string]any) { entry[field] = uint64(9007199254740992) }))
				}
				for _, value := range []any{nil, true, []any{}} {
					add("invalid entry", mutate(func(document, _ map[string]any) { document["keys"] = []any{value} }), "Invalid gateway document", "invalid-px-document")
				}
				for _, bounds := range [][2]string{{"0", "0"}, {"0x1", "0x7cf"}, {"0", "18446744073709551615"}} {
					value := mutate(func(document, entry map[string]any) {
						entry["firstHeight"], entry["lastHeight"] = bounds[0], bounds[1]
						document["keys"] = []any{entry}
					})
					first, ok := new(big.Int).SetString(bounds[0], 0)
					require.True(t, ok)
					last, ok := new(big.Int).SetString(bounds[1], 0)
					require.True(t, ok)
					valid = append(valid, map[string]any{"value": value, "expected": []map[string]string{{"keyId": expected[0]["keyId"], "publicKey": expected[0]["publicKey"], "firstHeight": first.String(), "lastHeight": last.String()}}})
				}
			}
			for _, value := range []any{nil, entries, map[string]any{}, map[string]any{"keys": nil}, map[string]any{"keys": captured}, map[string]any{"keys": entries, "extra": true}} {
				add("invalid result shape", value, "Invalid gateway receipt keys", "invalid-px-receipt-keys")
			}
			input, err := json.Marshal(map[string]any{
				"url": fmt.Sprintf("http://%s:%d", TestAddr, port), "errorUrl": fmt.Sprintf("http://%s:%d", TestAddr, unavailablePort),
				"document": captured, "expected": expected, "error": unavailable, "mutations": mutations, "unsafe": unsafe, "valid": valid,
			})
			require.NoError(t, err)
			for _, language := range []string{"typescript", "python"} {
				t.Run(language, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					var command *exec.Cmd
					if language == "typescript" {
						command = exec.CommandContext(ctx, "node", "--input-type=module", "-e", typescript, sdkPath)
					} else {
						command = exec.CommandContext(ctx, "python3", "-B", "-c", python, sdkPath)
					}
					command.Stdin = bytes.NewReader(input)
					var stderr bytes.Buffer
					command.Stderr = &stderr
					output, err := command.Output()
					require.NoError(t, err, "%s", stderr.String())
					expectedJSON, err := json.Marshal(expected)
					require.NoError(t, err)
					require.JSONEq(t, string(expectedJSON), string(output))
				})
			}
		})
	}
}

func TestReceiptKeysServedByTheNode(t *testing.T) {
	chain := newSeqReceiptChain9005(t)
	testApp := app.Setup(t, false, false, false)
	port := startSeqReceiptServer9005(t, testApp, chain.ctxProvider, evmrpc.SequencerReceiptOption{Source: chain, Keeper: &chain.keeper})
	proposerURL, err := url.Parse(fmt.Sprintf("http://%s:%d", TestAddr, port))
	require.NoError(t, err)
	// The forwarder's local store is empty: it must answer the proposer's pins.
	other := newSeqReceiptChain9005(t)
	require.NoError(t, other.keeper.SetReceiptKeys(other.writeCtx9005(7), seqkeeper.ReceiptKeys{}))
	other.commit9005(7)
	forwardPort := startSeqReceiptServer9005(t, testApp, other.ctxProvider, evmrpc.SequencerReceiptOption{Keeper: &other.keeper, ForwardURL: proposerURL})

	checkKeys := func(want seqkeeper.ReceiptKeys) seqkeeper.ReceiptKeys {
		t.Helper()
		raw, rpcErr := callSeqReceipt9005(t, port, evmrpc.ReceiptKeysMethod)
		require.Nil(t, rpcErr)
		var fields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(raw, &fields))
		require.Len(t, fields, 1)
		var entries []map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(fields["keys"], &entries))
		require.NotNil(t, entries, "an empty key list is [], never null")
		require.Len(t, entries, len(want))
		for i, entry := range entries {
			require.Len(t, entry, 4)
			entryJSON, err := json.Marshal(entry)
			require.NoError(t, err)
			require.JSONEq(t, fmt.Sprintf(`{"keyId":%q,"publicKey":%q,"firstHeight":%d,"lastHeight":%d}`,
				"0x"+hex.EncodeToString(want[i].KeyId), "0x"+hex.EncodeToString(want[i].PublicKey), want[i].FirstHeight, want[i].LastHeight), string(entryJSON))
		}
		var result evmrpc.ReceiptKeysResult
		require.NoError(t, json.Unmarshal(raw, &result))
		pins := make(seqkeeper.ReceiptKeys, len(result.Keys))
		for i, key := range result.Keys {
			pins[i] = seqkeeper.ReceiptKeyEntry{KeyId: bytes.Clone(key.KeyID[:]), PublicKey: bytes.Clone(key.PublicKey), FirstHeight: key.FirstHeight, LastHeight: key.LastHeight}
		}
		require.Equal(t, want, pins)
		forwarded, rpcErr := callSeqReceipt9005(t, forwardPort, evmrpc.ReceiptKeysMethod)
		require.Nil(t, rpcErr)
		require.JSONEq(t, string(raw), string(forwarded))
		return pins
	}

	public := chain.receiptKey.PublicKey()
	want := seqkeeper.ReceiptKeys{{KeyId: bytes.Clone(chain.keyID[:]), PublicKey: bytes.Clone(public[:]), FirstHeight: 1, LastHeight: 1999}}
	pins := checkKeys(want)
	hash, _ := chain.accept9005(seqReceiptTx9005{nonce: 1, height: 7, txIndex: 1, status: 1, returnData: []byte{0x01}})
	_, receipt, _ := seqReceiptResult9005(t, port, hash)
	require.NotNil(t, receipt)
	require.NoError(t, receipt.Verify(pins))

	// An update after server startup must be read at the latest height, in pinned order.
	secondID := bytes.Repeat([]byte{0x66}, 32)
	secondPublic := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x31}, ed25519.SeedSize)).Public().(ed25519.PublicKey)
	want = append(want, seqkeeper.ReceiptKeyEntry{KeyId: secondID, PublicKey: secondPublic, FirstHeight: 8, LastHeight: 2999})
	require.NoError(t, chain.keeper.SetReceiptKeys(chain.writeCtx9005(7), want))
	chain.commit9005(7)
	pins = checkKeys(want)
	require.NoError(t, receipt.Verify(pins))

	require.NoError(t, chain.keeper.SetReceiptKeys(chain.writeCtx9005(8), seqkeeper.ReceiptKeys{}))
	chain.commit9005(8)
	checkKeys(seqkeeper.ReceiptKeys{})

	_, rpcErr := callSeqReceipt9005(t, TestPort, evmrpc.ReceiptKeysMethod)
	require.NotNil(t, rpcErr)
	require.Equal(t, -32000, rpcErr.Code)

	t.Run("sent transaction", func(t *testing.T) {
		sender := occutils.NewSigner()
		h := newSendSeqHarness(t, sender)
		nodeCtxProvider := resolutionCtxProvider1315(h)
		public := h.key.PublicKey()
		keyID, err := h.key.KeyID(sendSeqOpenHeight)
		require.NoError(t, err)
		want := seqkeeper.ReceiptKeys{{KeyId: keyID[:], PublicKey: public[:], FirstHeight: 1, LastHeight: 1000}}
		committedStore := h.app.CommitMultiStore()
		header := tmproto.Header{ChainID: sendSeqChainID, Height: sendSeqOpenHeight}
		ctx := sdk.NewContext(committedStore, header, false)
		require.NoError(t, h.app.SequencerKeeper.SetReceiptKeys(ctx, want))
		require.Equal(t, sendSeqOpenHeight, committedStore.Commit(true).Version)
		ctxProvider := func(height int64) sdk.Context {
			if height == evmrpc.PendingCtxHeight {
				return nodeCtxProvider(height)
			}
			return sdk.NewContext(committedStore.CacheMultiStore(), header, false)
		}
		committedPins, err := h.app.SequencerKeeper.ReceiptKeys(ctxProvider(evmrpc.LatestCtxHeight))
		require.NoError(t, err)
		require.Equal(t, []seqtypes.ReceiptKey{{KeyId: keyID, PublicKey: public, FirstHeight: 1, LastHeight: 1000}}, committedPins)

		log, err := wal.OpenWAL(t.TempDir(), wal.DefaultConfig())
		require.NoError(t, err)
		t.Cleanup(log.Close)
		replicator, err := seqnode.NewReplicator(seqnode.ReplicatorConfig{WAL: log, Standalone: true})
		require.NoError(t, err)
		t.Cleanup(replicator.Close)
		_, err = replicator.Release(context.Background(), &wal.Open{
			Height: uint64(sendSeqOpenHeight), OpenTimeMs: uint64(time.Now().UnixMilli()), BaseFee: uint256.NewInt(0),
		})
		require.NoError(t, err)
		metrics, err := seqnode.NewMetrics(otel.Meter("receipt-keys-test"))
		require.NoError(t, err)
		source := seqnode.NewReceiptSource(8)
		accepter := seqnode.NewAccepter(h.accepter.r, replicator, source, metrics)
		config := evmrpcconfig.DefaultConfig
		config.HTTPAddress, config.HTTPPort = TestAddr, seqReceiptFreePort9005(t)
		server, err := evmrpc.NewEVMHTTPServer(config, h.client, &h.app.EvmKeeper, h.app.BeginBlockKeepers, h.app.BaseApp,
			h.app.TracerAnteHandler, ctxProvider, func(int64) client.TxConfig { return h.app.GetTxConfig() }, "", nil,
			evmrpc.NewSequencerOption(accepter, evmrpc.NewReceiptLogFeed(sendSeqCommittedTip)),
			evmrpc.SequencerReceiptOption{Source: source, Keeper: &h.app.SequencerKeeper})
		require.NoError(t, err)
		require.NoError(t, server.Start())
		t.Cleanup(server.Stop)

		tx, rawTx := sendSeqTransfer(t, sender, 0, seqReceiptTo9005, big.NewInt(1), sendSeqTransferGas)
		raw, rpcErr := callSeqReceipt9005(t, config.HTTPPort, "eth_sendRawTransaction", rawTx)
		require.Nil(t, rpcErr)
		var hash common.Hash
		require.NoError(t, json.Unmarshal(raw, &hash))
		require.Equal(t, tx.Hash(), hash)
		raw, rpcErr = callSeqReceipt9005(t, config.HTTPPort, evmrpc.ReceiptKeysMethod)
		require.Nil(t, rpcErr)
		var result evmrpc.ReceiptKeysResult
		require.NoError(t, json.Unmarshal(raw, &result))
		require.Len(t, result.Keys, 1)
		key := result.Keys[0]
		pins := seqkeeper.ReceiptKeys{{KeyId: key.KeyID[:], PublicKey: key.PublicKey, FirstHeight: key.FirstHeight, LastHeight: key.LastHeight}}
		require.Equal(t, want, pins)
		_, receipt, _ := seqReceiptResult9005(t, config.HTTPPort, hash)
		require.NotNil(t, receipt)
		require.Equal(t, evmrpc.RungReceipted, receipt.Rung)
		require.NoError(t, receipt.Verify(pins))
		require.Zero(t, h.client.broadcastCount())
	})
}

func TestSequencerReceiptRungs(t *testing.T) {
	chain := newSeqReceiptChain9005(t)
	port := startSeqReceiptServer9005(t, app.Setup(t, false, false, false), chain.ctxProvider, evmrpc.SequencerReceiptOption{Source: chain, Keeper: &chain.keeper})

	unknown, rpcErr := callSeqReceipt9005(t, port, evmrpc.SequencerReceiptMethod, common.HexToHash("0x99"))
	require.Nil(t, rpcErr)
	require.Equal(t, "null", string(unknown))

	submitted := seqReceiptTxHash9005(0, 9)
	chain.submit9005(submitted, 9)
	fields, result, _ := seqReceiptResult9005(t, port, submitted)
	require.Equal(t, map[string]any{"rung": "submitted", "txHash": submitted.Hex(), "nonce": "0x9"}, fields)
	require.NoError(t, result.Verify())

	committedHash, _ := chain.accept9005(seqReceiptTx9005{nonce: 1, height: 7, txIndex: 1, status: 1, returnData: []byte{0x01}, logs: seqReceiptTwoLogs9005()})
	chain.commitBatch9005(7, 1)
	receiptedHash, _ := chain.accept9005(seqReceiptTx9005{nonce: 2, height: 8, txIndex: 1, status: 1, returnData: []byte{0x02}})

	fields, result, _ = seqReceiptResult9005(t, port, receiptedHash)
	require.Equal(t, "receipted", fields["rung"])
	require.NotContains(t, fields, "blockHash")
	require.NotContains(t, fields, "blockNumber")
	require.Equal(t, "0x8", fields["height"])
	require.NoError(t, result.Verify())
	receiptedBytes := []byte(result.Receipt)
	verified, err := verify.ReceiptSignature(receiptedBytes, chain.receiptKey.PublicKey())
	require.NoError(t, err)
	require.Equal(t, [32]byte(receiptedHash), verified.Receipt.ActivityID)

	fields, result, _ = seqReceiptResult9005(t, port, committedHash)
	require.Equal(t, "committed", fields["rung"])
	require.Equal(t, "0x7", fields["blockNumber"])
	require.Equal(t, TestBlockHash, fields["blockHash"])
	require.NoError(t, result.Verify())

	chain.commitBatch9005(8, 2)
	fields, result, _ = seqReceiptResult9005(t, port, receiptedHash)
	require.Equal(t, "committed", fields["rung"])
	require.Equal(t, "0x8", fields["blockNumber"])
	require.Equal(t, TestBlockHash, fields["blockHash"])
	require.Equal(t, receiptedBytes, []byte(result.Receipt), "receipt bytes are identical at the receipted and committed rungs")
	require.NoError(t, result.Verify())
}

func TestStandardReceiptStaysCommittedOnly(t *testing.T) {
	chain := newSeqReceiptChain9005(t)
	port := startSeqReceiptServer9005(t, app.Setup(t, false, false, false), chain.ctxProvider, evmrpc.SequencerReceiptOption{Source: chain, Keeper: &chain.keeper})
	hash, _ := chain.accept9005(seqReceiptTx9005{nonce: 1, height: 7, txIndex: 1, status: 1, returnData: []byte{0x0a}})
	chain.submit9005(seqReceiptTxHash9005(0, 2), 2)

	fields, _, _ := seqReceiptResult9005(t, port, hash)
	require.Equal(t, "receipted", fields["rung"])
	standard := sendRequestGood(t, "getTransactionReceipt", hash.Hex())
	require.Nil(t, standard["error"])
	require.Contains(t, standard, "result")
	require.Nil(t, standard["result"], "eth_getTransactionReceipt never answers a receipted transaction")

	submittedStandard := sendRequestGood(t, "getTransactionReceipt", seqReceiptTxHash9005(0, 2).Hex())
	require.Nil(t, submittedStandard["error"])
	require.Nil(t, submittedStandard["result"], "eth_getTransactionReceipt never answers a submitted transaction")

	chain.commitBatch9005(7, 1)
	fields, _, _ = seqReceiptResult9005(t, port, hash)
	require.Equal(t, "committed", fields["rung"])
	require.Nil(t, sendRequestGood(t, "getTransactionReceipt", hash.Hex())["result"],
		"the sequencer rung never feeds eth_getTransactionReceipt; only the committed EVM receipt does")

	committed := sendRequestGood(t, "getTransactionReceipt", multiTxBlockTx1.Hash().Hex())
	require.Nil(t, committed["error"])
	receipt, ok := committed["result"].(map[string]any)
	require.True(t, ok, "a committed transaction keeps its standard receipt")
	require.Equal(t, MultiTxBlockHash, receipt["blockHash"])
	require.Equal(t, "0x2", receipt["blockNumber"])
	require.Equal(t, multiTxBlockTx1.Hash().Hex(), receipt["transactionHash"])
}

func TestSequencerReceiptJSONMatchesBytes(t *testing.T) {
	chain := newSeqReceiptChain9005(t)
	port := startSeqReceiptServer9005(t, app.Setup(t, false, false, false), chain.ctxProvider, evmrpc.SequencerReceiptOption{Source: chain, Keeper: &chain.keeper})
	returnData := []byte{0xde, 0xad, 0xbe, 0xef}
	hash, seq := chain.accept9005(seqReceiptTx9005{nonce: 4, height: 7, txIndex: 3, status: 1, returnData: returnData, logs: seqReceiptTwoLogs9005()})
	revertedHash, _ := chain.accept9005(seqReceiptTx9005{nonce: 5, height: 7, txIndex: 4, status: 0, returnData: []byte("revert"), vmError: "execution reverted", create: true})

	fields, result, _ := seqReceiptResult9005(t, port, hash)
	require.NoError(t, result.Verify())
	require.Equal(t, evmrpc.RungReceipted, result.Rung)
	require.Equal(t, hash, result.TxHash)
	require.Equal(t, uint64(4), uint64(result.Nonce))
	require.Equal(t, seq, uint64(result.Seq))
	require.Equal(t, uint64(7), uint64(result.Height))
	require.Equal(t, uint(3), uint(result.Index))
	require.Equal(t, uint64(1_700_000_000_000+7*400), uint64(result.Timestamp))
	require.Equal(t, uint64(1), uint64(result.Status))
	require.Equal(t, returnData, []byte(result.ReturnData))
	require.Equal(t, uint64(21_750), uint64(result.GasUsed))
	require.Equal(t, uint64(63_750), uint64(result.CumulativeGasUsed))
	require.Equal(t, uint64(200), uint64(result.EffectiveGasPrice))
	require.Equal(t, seqReceiptFeeDenom9005, result.FeeDenom)
	require.Equal(t, "0x401640", fields["feeAmount"])
	require.Equal(t, seqReceiptSponsor9005, result.Sponsor)
	require.Equal(t, seqReceiptFrom9005, result.From)
	require.Equal(t, seqReceiptTo9005, result.To)
	require.Equal(t, common.Address{}, result.ContractAddress)
	require.Equal(t, "", result.VMError)
	require.Equal(t, common.Hash(chain.keyID), result.KeyID)
	require.Equal(t, chain.sends[hash].Receipt, []byte(result.Receipt))
	require.Equal(t, []evmrpc.SequencerReceiptLog{
		{Address: seqReceiptLogAddr9005, Topics: []common.Hash{seqReceiptTopicA9005, seqReceiptTopicB9005}, Data: []byte("transfer"), LogIndex: 0},
		{Address: seqReceiptTo9005, Topics: []common.Hash{}, Data: []byte{}, LogIndex: 1},
	}, result.Logs)
	verified, err := verify.ReceiptSignature(result.Receipt, chain.receiptKey.PublicKey())
	require.NoError(t, err)
	require.Equal(t, seq, verified.Receipt.GlobalSequence)

	_, reverted, _ := seqReceiptResult9005(t, port, revertedHash)
	require.NoError(t, reverted.Verify())
	require.Equal(t, uint64(0), uint64(reverted.Status))
	require.Equal(t, "execution reverted", reverted.VMError)
	require.Equal(t, seqReceiptCreated9005, reverted.ContractAddress)
	require.Equal(t, common.Address{}, reverted.To)
	require.Empty(t, reverted.Logs)

	tampered := func(edit func(r *evmrpc.SequencerReceiptResult)) error {
		var copyOf *evmrpc.SequencerReceiptResult
		_, _, raw := seqReceiptResult9005(t, port, hash)
		require.NoError(t, json.Unmarshal(raw, &copyOf))
		edit(copyOf)
		return copyOf.Verify()
	}
	for name, edit := range map[string]func(r *evmrpc.SequencerReceiptResult){
		"returnData": func(r *evmrpc.SequencerReceiptResult) { r.ReturnData = []byte{0xde, 0xad, 0xbe, 0xee} },
		"log data":   func(r *evmrpc.SequencerReceiptResult) { r.Logs[0].Data = []byte("transfeR") },
		"log topic":  func(r *evmrpc.SequencerReceiptResult) { r.Logs[0].Topics[1] = seqReceiptTopicA9005 },
		"log index":  func(r *evmrpc.SequencerReceiptResult) { r.Logs[1].LogIndex = 7 },
		"gasUsed":    func(r *evmrpc.SequencerReceiptResult) { r.GasUsed++ },
		"index":      func(r *evmrpc.SequencerReceiptResult) { r.Index = 0 },
		"sponsor":    func(r *evmrpc.SequencerReceiptResult) { r.Sponsor = seqReceiptFrom9005 },
		"vmError":    func(r *evmrpc.SequencerReceiptResult) { r.VMError = "out of gas" },
		"feeAmount":  func(r *evmrpc.SequencerReceiptResult) { r.FeeAmount.ToInt().Add(r.FeeAmount.ToInt(), big.NewInt(1)) },
		"nonce":      func(r *evmrpc.SequencerReceiptResult) { r.Nonce++ },
		"txHash":     func(r *evmrpc.SequencerReceiptResult) { r.TxHash = revertedHash },
		"rung block": func(r *evmrpc.SequencerReceiptResult) { n := r.Height; r.BlockNumber = &n },
	} {
		require.ErrorIs(t, tampered(edit), evmrpc.ErrSequencerReceiptMismatch, name)
	}
	require.ErrorIs(t, tampered(func(r *evmrpc.SequencerReceiptResult) { r.Receipt = r.Receipt[:len(r.Receipt)-1] }), evmrpc.ErrSequencerReceiptInvalid)
}

func TestSequencerReceiptCommittedRungMatchesBatch(t *testing.T) {
	chain := newSeqReceiptChain9005(t)
	port := startSeqReceiptServer9005(t, app.Setup(t, false, false, false), chain.ctxProvider, evmrpc.SequencerReceiptOption{Source: chain, Keeper: &chain.keeper})
	first, seqFirst := chain.accept9005(seqReceiptTx9005{nonce: 1, height: 7, txIndex: 1, status: 1, returnData: []byte{0x01}})
	second, seqSecond := chain.accept9005(seqReceiptTx9005{nonce: 2, height: 7, txIndex: 2, status: 1, returnData: []byte{0x02}})
	chain.commitBatch9005(7, seqSecond)
	chain.commitBatch9005(8, seqSecond)
	reproposed, seqReproposed := chain.accept9005(seqReceiptTx9005{nonce: 3, height: 8, txIndex: 1, status: 1, returnData: []byte{0x03}})
	open, _ := chain.accept9005(seqReceiptTx9005{nonce: 4, height: 10, txIndex: 1, status: 1, returnData: []byte{0x04}})
	chain.commitBatch9005(9, seqReproposed)

	for _, want := range []struct {
		hash   common.Hash
		seq    uint64
		height uint64
	}{{first, seqFirst, 7}, {second, seqSecond, 7}, {reproposed, seqReproposed, 9}} {
		fields, result, _ := seqReceiptResult9005(t, port, want.hash)
		require.Equal(t, "committed", fields["rung"])
		require.Equal(t, want.height, uint64(*result.BlockNumber))
		require.Equal(t, common.HexToHash(TestBlockHash), *result.BlockHash)
		require.NoError(t, result.Verify())
		atBlock, err := chain.keeper.LastSeq(chain.ctxProvider(int64(want.height)))
		require.NoError(t, err)
		require.GreaterOrEqual(t, atBlock, want.seq, "the block's MsgSequenceBatch covers the seq")
		belowBlock, err := chain.keeper.LastSeq(chain.ctxProvider(int64(want.height) - 1))
		require.NoError(t, err)
		require.Less(t, belowBlock, want.seq, "no lower block covers the seq")
	}

	fields, _, _ := seqReceiptResult9005(t, port, open)
	require.Equal(t, "receipted", fields["rung"], "a receipt above the latest block is not committed")

	forkedActivity := seqReceiptTxHash9005(7, 77)
	chain.mu.Lock()
	forkPoint := chain.seqHashes[seqFirst]
	chain.mu.Unlock()
	forked := chain.signReceipt9005(seqReceiptTx9005{nonce: 77, height: 7, txIndex: 5, status: 1}, seqSecond, forkPoint, forkedActivity)
	chain.mu.Lock()
	chain.sends[forkedActivity] = &evmrpc.SequencerSend{Nonce: 77, Receipt: forked, LogData: [][]byte{}}
	chain.mu.Unlock()
	_, rpcErr := callSeqReceipt9005(t, port, evmrpc.SequencerReceiptMethod, forkedActivity)
	require.NotNil(t, rpcErr)
	require.Contains(t, rpcErr.Message, evmrpc.ErrSequencerReceiptCommit.Error())
}

func TestSequencerReceiptForwardedFromFullNode(t *testing.T) {
	chain := newSeqReceiptChain9005(t)
	testApp := app.Setup(t, false, false, false)
	sequencerPort := startSeqReceiptServer9005(t, testApp, chain.ctxProvider, evmrpc.SequencerReceiptOption{Source: chain, Keeper: &chain.keeper})
	sequencerURL, err := url.Parse(fmt.Sprintf("http://%s:%d", TestAddr, sequencerPort))
	require.NoError(t, err)
	fullNodePort := startSeqReceiptServer9005(t, testApp, chain.ctxProvider, evmrpc.SequencerReceiptOption{ForwardURL: sequencerURL})

	committed, _ := chain.accept9005(seqReceiptTx9005{nonce: 1, height: 7, txIndex: 1, status: 1, returnData: []byte{0x01}, logs: seqReceiptTwoLogs9005()})
	chain.commitBatch9005(7, 1)
	receipted, _ := chain.accept9005(seqReceiptTx9005{nonce: 2, height: 8, txIndex: 1, status: 1, returnData: []byte{0x02}})
	submitted := seqReceiptTxHash9005(0, 3)
	chain.submit9005(submitted, 3)

	for _, hash := range []common.Hash{committed, receipted, submitted, common.HexToHash("0x77")} {
		fromSequencer, rpcErr := callSeqReceipt9005(t, sequencerPort, evmrpc.SequencerReceiptMethod, hash)
		require.Nil(t, rpcErr)
		fromFullNode, rpcErr := callSeqReceipt9005(t, fullNodePort, evmrpc.SequencerReceiptMethod, hash)
		require.Nil(t, rpcErr)
		require.JSONEq(t, string(fromSequencer), string(fromFullNode))
	}

	_, honest, raw := seqReceiptResult9005(t, sequencerPort, committed)
	require.NoError(t, honest.Verify())
	var altered map[string]any
	require.NoError(t, json.Unmarshal(raw, &altered))
	altered["returnData"] = "0x0f"
	lying := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": altered})
	}))
	t.Cleanup(lying.Close)
	lyingURL, err := url.Parse(lying.URL)
	require.NoError(t, err)
	refusingPort := startSeqReceiptServer9005(t, testApp, chain.ctxProvider, evmrpc.SequencerReceiptOption{ForwardURL: lyingURL})
	_, rpcErr := callSeqReceipt9005(t, refusingPort, evmrpc.SequencerReceiptMethod, committed)
	require.NotNil(t, rpcErr)
	require.Contains(t, rpcErr.Message, evmrpc.ErrSequencerReceiptMismatch.Error())

	_, rpcErr = callSeqReceipt9005(t, TestPort, evmrpc.SequencerReceiptMethod, committed)
	require.NotNil(t, rpcErr)
	require.Equal(t, -32000, rpcErr.Code)
	require.Equal(t, "pax_sequencerReceipt served by sequencer", rpcErr.Message)

	_, err = evmrpc.NewEVMHTTPServer(evmrpcconfig.DefaultConfig, &MockClient{}, EVMKeeper, testApp.BeginBlockKeepers, testApp.BaseApp,
		testApp.TracerAnteHandler, chain.ctxProvider, func(int64) client.TxConfig { return TxConfig }, "", nil,
		evmrpc.SequencerReceiptOption{Source: chain, Keeper: &chain.keeper, ForwardURL: sequencerURL})
	require.True(t, errors.Is(err, evmrpc.ErrSequencerReceiptOption))
	_, err = evmrpc.NewEVMHTTPServer(evmrpcconfig.DefaultConfig, &MockClient{}, EVMKeeper, testApp.BeginBlockKeepers, testApp.BaseApp,
		testApp.TracerAnteHandler, chain.ctxProvider, func(int64) client.TxConfig { return TxConfig }, "", nil,
		evmrpc.SequencerReceiptOption{Source: chain})
	require.True(t, errors.Is(err, evmrpc.ErrSequencerReceiptOption))
}

func TestSequencerReceiptViewCarriesPaymentDenom(t *testing.T) {
	chain := newSeqReceiptChain9005(t)
	sender, recipient := occutils.NewSigner(), occutils.NewSigner()
	producer := newPendingSequencer(t, 6, sender)
	receipter, err := seqnode.NewReceipter(producer.exec, producer.app, &producer.app.EvmKeeper, chain.receiptKey, seqReceiptModuleID9005)
	require.NoError(t, err)
	accepted, signed, err := receipter.Accept(pendingTransfer(t, sender, 0, recipient.EvmAddress, big.NewInt(pendingOnePaxWei)))
	require.NoError(t, err)
	source := seqnode.NewReceiptSource(1)
	hash := common.Hash(accepted.ActivityID)
	source.Record(hash, evmrpc.SequencerSend{Receipt: signed, LogData: [][]byte{}})
	port := startSeqReceiptServer9005(t, producer.app, chain.ctxProvider, evmrpc.SequencerReceiptOption{Source: source, Keeper: &chain.keeper})
	fields, result, raw := seqReceiptResult9005(t, port, hash)
	require.Equal(t, evmkeeper.BaseDenom, fields["paymentDenom"])
	require.Equal(t, evmkeeper.BaseDenom, result.PaymentDenom)
	require.Equal(t, evmkeeper.BaseDenom, fields["feeDenom"])
	require.NoError(t, result.Verify())
	for _, paymentDenom := range []any{nil, "", "different-payment-asset"} {
		var changed map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(raw, &changed))
		if paymentDenom == nil {
			delete(changed, "paymentDenom")
		} else {
			changed["paymentDenom"], err = json.Marshal(paymentDenom)
			require.NoError(t, err)
		}
		encoded, err := json.Marshal(changed)
		require.NoError(t, err)
		var decoded evmrpc.SequencerReceiptResult
		require.NoError(t, json.Unmarshal(encoded, &decoded))
		require.ErrorIs(t, decoded.Verify(), evmrpc.ErrSequencerReceiptMismatch)
	}
}

func repairM1297CommittedReceiptNilView(t *testing.T) {
	var missing *evmrpc.SequencerReceiptResult
	require.ErrorIs(t, missing.Verify(), evmrpc.ErrSequencerReceiptMismatch)
	block := common.HexToHash("0x1234")
	for _, body := range []string{
		`{"txHash":"` + common.HexToHash("0x5678").Hex() + `","rung":"committed","blockNumber":"0x7","blockHash":"` + block.Hex() + `"}`,
		`{"txHash":"` + common.HexToHash("0x5678").Hex() + `","rung":"committed","blockNumber":"0x7","blockHash":"` + block.Hex() + `","receipt":"0x"}`,
		`{"txHash":"` + common.HexToHash("0x5678").Hex() + `","rung":"receipted"}`,
	} {
		t.Run(body, func(t *testing.T) {
			var result evmrpc.SequencerReceiptResult
			require.NoError(t, json.Unmarshal([]byte(body), &result))
			require.NotPanics(t, func() { require.ErrorIs(t, result.Verify(), evmrpc.ErrSequencerReceiptMismatch) })
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					ID     json.RawMessage `json:"id"`
					Method string          `json:"method"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				if request.Method != evmrpc.SequencerReceiptMethod {
					http.Error(w, "unexpected forwarding method", http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": json.RawMessage(body)})
			}))
			defer upstream.Close()
			forwardURL, err := url.Parse(upstream.URL)
			require.NoError(t, err)
			a := app.Setup(t, false, false, false)
			ctx := a.GetContextForDeliverTx(nil)
			public := ed25519.NewKeyFromSeed(seqVecReceiptSeed).Public().(ed25519.PublicKey)
			require.NoError(t, a.SequencerKeeper.SetReceiptKeys(ctx, seqkeeper.ReceiptKeys{{
				KeyId: seqVecKeyID[:], PublicKey: public, FirstHeight: 1, LastHeight: 1000,
			}}))
			port := startSeqReceiptServer9005(t, a, func(int64) sdk.Context { return ctx }, evmrpc.SequencerReceiptOption{ForwardURL: forwardURL})
			answer, rpcErr := callSeqReceipt9005(t, port, evmrpc.SequencerReceiptMethod, common.HexToHash("0x5678"))
			require.NotNil(t, rpcErr, string(answer))
			require.Contains(t, rpcErr.Message, evmrpc.ErrSequencerReceiptMismatch.Error())
		})
	}
	t.Run("null receipt rejected by codec", func(t *testing.T) {
		var result evmrpc.SequencerReceiptResult
		require.ErrorContains(t, json.Unmarshal([]byte(`{"rung":"committed","receipt":null}`), &result),
			"cannot unmarshal non-string")
		require.ErrorIs(t, result.Verify(), evmrpc.ErrSequencerReceiptMismatch)
	})
	require.NoError(t, (&evmrpc.SequencerReceiptResult{Rung: evmrpc.RungSubmitted}).Verify())
	require.ErrorIs(t, (&evmrpc.SequencerReceiptResult{Rung: evmrpc.RungSubmitted, SequencerReceiptView: &evmrpc.SequencerReceiptView{}}).Verify(), evmrpc.ErrSequencerReceiptMismatch)
}

func init() {
	evmrpc.RepairM1297Proofs["committed-receipt-nil-view"] = repairM1297CommittedReceiptNilView
}
