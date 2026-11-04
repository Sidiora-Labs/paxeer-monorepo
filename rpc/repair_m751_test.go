package evmrpc_test

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	rpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	rpcfixtures "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/tests"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	gethstate "github.com/ethereum/go-ethereum/core/state"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm/runtime"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/gasprice"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/export"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

// Task 22.33.751 repair record (spec/ is outside the authorized paths).
// accountless-silent-skip -> TestRepairM_751_AccountlessSilentSkip
// call-nonce-override-observability -> TestRepairM_751_CallNonceOverrideObservability
// chain-id-legacy-table -> TestRepairM_751_ChainIDLegacyTable
// feehistory-disabled-regressions -> TestRepairM_751_FeeHistoryDisabledRegressions
// gasprice-spec-neighbor -> TestRepairM_751_GasPriceSpecNeighbor
// hash-block-fee-skips -> TestRepairM_751_HashBlockFeeSkips
// number-block-fee-skips -> TestRepairM_751_NumberBlockFeeSkips
// receipts-dualvm-skip -> TestRepairM_751_ReceiptsDualVMSkip
// Previous attempt: six cases passed; AccountlessSilentSkip and
// FeeHistoryDisabledRegressions failed, as reported by the continuation ruling.
// Successful keyless Accounts now initializes an empty, non-nil result slice.
// Fee-history parity compares big.Int values rather than their internal storage.
// Qualification repair: ethclient allocates empty reward/base-fee slices; assert
// the oracle's nil slices and the consumer's empty slices at their own stages.
// Wiring request (domain 16): tests/integration/rpc_tests/scripts/run-ci.sh must start/stop an isolated non-live Pax node with an empty test keyring, unsafe keyring RPC enabled, and RPC_ACCOUNTLESS pointing to its loopback listener.
// Wiring request (domain 16): tests/integration/rpc_tests/utils/chainUtils.ts must propagate parameter-query errors rather than returning null, retaining the original setup diagnostics.
// Wiring request (domain 16): tests/integration/rpc_tests/utils/gasPriceUtils.ts must adopt the scoped spec's pinned-head/successor checks; the repaired spec no longer calls its neighbor-tolerant helper.
// Wiring request (domain 16): tests/integration/rpc_tests/eth/eth_getBlockReceipts.spec.ts must control native/EVM co-location, reject nonzero native execution codes, and fail bounded setup rather than skipping.
// Wiring request (domain 16): spec/layerx-merge/spec.kvx must carry this finding/test mapping and the final runner evidence; no baseline failures are claimed.

func repair751RPC(t *testing.T, service any) *gethrpc.Client {
	t.Helper()
	server := gethrpc.NewServer()
	t.Cleanup(server.Stop)
	require.NoError(t, server.RegisterName("eth", service))
	httpServer := httptest.NewServer(server)
	t.Cleanup(httpServer.Close)
	c, err := gethrpc.DialHTTP(httpServer.URL)
	require.NoError(t, err)
	t.Cleanup(c.Close)
	return c
}

func repair751Call(t *testing.T, c *gethrpc.Client, method string, args ...any) json.RawMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	var result json.RawMessage
	require.NoError(t, c.CallContext(ctx, &result, method, args...), method)
	return result
}

func TestRepairM_751_AccountlessSilentSkip(t *testing.T) {
	// Provision the actual Pax handler with a real, empty on-disk SDK keyring.
	home := t.TempDir()
	api := evmrpc.NewInfoAPI(nil, nil, nil, nil, home, 1024, evmrpc.ConnectionTypeHTTP, nil, nil, true)
	c := repair751RPC(t, api)
	raw := repair751Call(t, c, "eth_accounts")
	require.JSONEq(t, `[]`, string(raw), "a successful keyless response must serialize as [], never null")
	var accounts []common.Address
	require.NoError(t, json.Unmarshal(raw, &accounts))
	require.NotNil(t, accounts)
	require.Empty(t, accounts)
}

type repair751Fixture struct {
	app       *app.App
	consensus *rpcfixtures.MockClient
	rpc       *gethrpc.Client
	height    int64
}

func repair751Chain(t *testing.T) *repair751Fixture {
	t.Helper()
	a, c := rpcfixtures.SetupCommittedHeaderFixture(t)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	// The existing fixture peer stores only real FinalizeBlock/Commit outputs.
	// Preserve its genesis block before adding subsequent committed transcripts.
	one := int64(1)
	block, err := c.Block(t.Context(), &one)
	require.NoError(t, err)
	results, err := c.BlockResults(t.Context(), &one)
	require.NoError(t, err)
	c.RecordCommittedBlock(block, results)
	cfg := rpcconfig.DefaultConfig
	cfg.HTTPPort, cfg.WSPort, cfg.IPRateLimitRPS = 0, 0, 0
	cfg.EnableUnsafeKeyringRPC = true // isolated "pax-test", never a live chain
	service, err := evmrpc.NewEVMHTTPServer(cfg, c, &a.EvmKeeper, a.BeginBlockKeepers, a.BaseApp,
		a.TracerAnteHandler, a.RPCContextProvider, func(int64) client.TxConfig { return a.GetTxConfig() }, t.TempDir(), nil)
	require.NoError(t, err)
	t.Cleanup(service.Stop)
	server := httptest.NewServer(service.(http.Handler))
	t.Cleanup(server.Close)
	rpcClient, err := gethrpc.DialHTTP(server.URL)
	require.NoError(t, err)
	t.Cleanup(rpcClient.Close)
	return &repair751Fixture{a, c, rpcClient, 1}
}

func (f *repair751Fixture) commit(t *testing.T, txs [][]byte, initialize func(sdk.Context)) *coretypes.ResultBlock {
	t.Helper()
	previous := f.height
	parent, err := f.consensus.Block(t.Context(), &previous)
	require.NoError(t, err)
	height := previous + 1
	data := tmtypes.Data{}
	for _, tx := range txs {
		data.Txs = append(data.Txs, tmtypes.Tx(tx))
	}
	header := tmtypes.Header{
		ChainID: f.app.ChainID, Height: height, Time: parent.Block.Time.Add(time.Second),
		LastBlockID: parent.BlockID, AppHash: f.app.LastCommitID().Hash,
		DataHash: data.Txs.Hash(), ProposerAddress: parent.Block.ProposerAddress,
		ValidatorsHash: parent.Block.ValidatorsHash, NextValidatorsHash: parent.Block.NextValidatorsHash,
	}
	block := &tmtypes.Block{Header: header, Data: data, LastCommit: &tmtypes.Commit{Height: previous}}
	hash := block.Hash()
	response, err := f.app.FinalizeBlock(t.Context(), &abci.RequestFinalizeBlock{Hash: hash, Header: header.ToProto(), Txs: txs})
	require.NoError(t, err)
	require.Len(t, response.TxResults, len(txs))
	for _, result := range response.TxResults {
		require.Equal(t, abci.CodeTypeOK, result.Code, result.Log)
	}
	if initialize != nil {
		initialize(f.app.GetContextForDeliverTx(nil))
	}
	_, err = f.app.Commit(t.Context())
	require.NoError(t, err)
	result := &coretypes.ResultBlock{BlockID: tmtypes.BlockID{Hash: hash}, Block: block}
	f.consensus.RecordCommittedBlock(result, &coretypes.ResultBlockResults{Height: height, TxsResults: response.TxResults})
	store := f.app.EvmKeeper.ReceiptStore()
	require.NotNil(t, store)
	require.NoError(t, store.SetEarliestVersion(1))
	require.NoError(t, store.SetLatestVersion(height))
	// Receipt persistence is asynchronous; require readiness before consuming it.
	for _, raw := range txs {
		tx, err := f.app.GetTxConfig().TxDecoder()(raw)
		require.NoError(t, err)
		for _, message := range tx.GetMsgs() {
			if evm, ok := message.(*evmtypes.MsgEVMTransaction); ok {
				transaction, _ := evm.AsTransaction()
				require.NotNil(t, transaction)
				require.Eventually(t, func() bool {
					receipt, err := f.app.EvmKeeper.GetReceipt(f.app.RPCContextProvider(evmrpc.LatestCtxHeight), transaction.Hash())
					return err == nil && receipt != nil && receipt.BlockNumber == uint64(height)
				}, 5*time.Second, time.Millisecond, "required committed EVM receipt %s", transaction.Hash())
			}
		}
	}
	f.height = height
	return result
}

func TestRepairM_751_ChainIDLegacyTable(t *testing.T) {
	f := repair751Chain(t)
	// Exercise the real keeper mapping through HTTP and the typed Ethereum consumer.
	var id string
	require.NoError(t, json.Unmarshal(repair751Call(t, f.rpc, "eth_chainId"), &id))
	require.Equal(t, "0x7d", id)
	chainID, err := ethclient.NewClient(f.rpc).ChainID(t.Context())
	require.NoError(t, err)
	require.Equal(t, big.NewInt(125), chainID)
	var version string
	require.NoError(t, json.Unmarshal(repair751Call(t, f.rpc, "net_version"), &version))
	require.Equal(t, "125", version)
}

func TestRepairM_751_CallNonceOverrideObservability(t *testing.T) {
	f := repair751Chain(t)
	sender := common.HexToAddress("0x7510000000000000000000000000000000000001")
	// Real constructor bytecode exposes ADDRESS, hence the sender's CREATE nonce.
	call := map[string]any{"from": sender, "data": "0x3060005260206000f3", "gas": "0x186a0"}
	tag := hexutil.EncodeUint64(uint64(f.height))
	before := repair751Call(t, f.rpc, "eth_getTransactionCount", sender, tag)
	require.JSONEq(t, `"0x0"`, string(before))
	var observed []hexutil.Bytes
	for _, nonce := range []uint64{0, 255} {
		override := map[string]any{}
		if nonce != 0 {
			override[sender.Hex()] = map[string]any{"nonce": hexutil.EncodeUint64(nonce)}
		}
		var result hexutil.Bytes
		require.NoError(t, json.Unmarshal(repair751Call(t, f.rpc, "eth_call", call, tag, override), &result))
		address := crypto.CreateAddress(sender, nonce)
		require.Equal(t, common.LeftPadBytes(address.Bytes(), 32), []byte(result))
		// Drive geth's actual override decoder, trie state and CREATE execution
		// in-process as the reference; no reference node or binary is required.
		reference, err := gethstate.New(ethtypes.EmptyRootHash, gethstate.NewDatabaseForTesting())
		require.NoError(t, err)
		encoded, err := json.Marshal(override)
		require.NoError(t, err)
		var gethOverride export.StateOverride
		require.NoError(t, json.Unmarshal(encoded, &gethOverride))
		require.NoError(t, gethOverride.Apply(reference, nil))
		gethResult, gethAddress, _, err := runtime.Create(common.FromHex("0x3060005260206000f3"), &runtime.Config{
			Origin: sender, State: reference, GasLimit: 100_000,
		})
		require.NoError(t, err)
		require.Equal(t, address, gethAddress)
		require.Equal(t, gethResult, []byte(result), "real geth CREATE parity")
		observed = append(observed, result)
		require.JSONEq(t, string(before), string(repair751Call(t, f.rpc, "eth_getTransactionCount", sender, tag)))
		require.JSONEq(t, `"0x"`, string(repair751Call(t, f.rpc, "eth_getCode", address, "latest")))
		require.JSONEq(t, string(before), string(repair751Call(t, f.rpc, "eth_getTransactionCount", sender, "latest")))
	}
	require.NotEqual(t, observed[0], observed[1], "ignoring the override must fail the observable result")
}

func TestRepairM_751_FeeHistoryDisabledRegressions(t *testing.T) {
	c := repair751RPC(t, evmrpc.NewInfoAPI(nil, nil, nil, nil, "", 1024, evmrpc.ConnectionTypeHTTP, nil, nil))
	oracle := new(gasprice.Oracle)
	for _, count := range []any{"0x0", 0} {
		for _, tag := range []string{"latest", "pending", "0x1"} {
			raw := repair751Call(t, c, "eth_feeHistory", count, tag, []float64{})
			require.JSONEq(t, `{"oldestBlock":"0x0","gasUsedRatio":null}`, string(raw))
		}
	}
	oldest, rewards, base, ratios, _, _, err := oracle.FeeHistory(t.Context(), 0, gethrpc.LatestBlockNumber, nil)
	require.NoError(t, err)
	history, err := ethclient.NewClient(c).FeeHistory(t.Context(), 0, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, history.OldestBlock)
	require.NotNil(t, oldest)
	require.Zero(t, oldest.Cmp(history.OldestBlock), "oldest block matches the geth reference numerically")
	// The oracle returns nil for an empty range; ethclient.FeeHistory allocates
	// both slices while decoding. Assert each stage's exact representation.
	require.Nil(t, rewards)
	require.Nil(t, base)
	require.Equal(t, [][]*big.Int{}, history.Reward)
	require.Equal(t, []*big.Int{}, history.BaseFee)
	require.Equal(t, ratios, history.GasUsedRatio)
}

// repair751FeeBlocks sets real governed parameters in a committed fixture, then
// executes two signed EVM calls together above the target and commits their child.
func repair751FeeBlocks(t *testing.T, f *repair751Fixture) (*coretypes.ResultBlock, *coretypes.ResultBlock) {
	t.Helper()
	f.commit(t, nil, func(ctx sdk.Context) {
		params := f.app.EvmKeeper.GetParams(ctx)
		params.TargetGasUsedPerBlock = 20_000
		require.NoError(t, params.Validate())
		f.app.EvmKeeper.SetParams(ctx, params)
		f.app.EvmKeeper.SetNextBaseFeePerGas(ctx, sdk.NewDec(500_000_000))
	})
	_, txs := rpcfixtures.CommittedHeaderTransactions(t, f.app)
	parent := f.commit(t, txs, nil)
	child := f.commit(t, nil, nil)
	return parent, child
}

func TestRepairM_751_GasPriceSpecNeighbor(t *testing.T) {
	f := repair751Chain(t)
	f.commit(t, nil, func(ctx sdk.Context) {
		f.app.EvmKeeper.SetNextBaseFeePerGas(ctx, sdk.NewDec(500_000_007))
	})
	sample := f.height
	state := f.app.RPCContextProvider(sample)
	nextFee := f.app.EvmKeeper.GetNextBaseFeePerGas(state).TruncateInt().BigInt()
	var price hexutil.Big
	require.NoError(t, json.Unmarshal(repair751Call(t, f.rpc, "eth_gasPrice"), &price))
	expected := new(big.Int).Mul(new(big.Int).Set(nextFee), big.NewInt(110))
	expected.Div(expected, big.NewInt(100)).Add(expected, big.NewInt(1_000_000_000))
	require.Equal(t, expected, price.ToInt())
	child := f.commit(t, nil, nil)
	var block struct {
		BaseFee hexutil.Big `json:"baseFeePerGas"`
	}
	require.NoError(t, json.Unmarshal(repair751Call(t, f.rpc, "eth_getBlockByHash", common.BytesToHash(child.BlockID.Hash), false), &block))
	require.Equal(t, nextFee, block.BaseFee.ToInt(), "only the sampled successor fee qualifies")
	following := f.app.EvmKeeper.GetNextBaseFeePerGas(f.app.RPCContextProvider(f.height)).TruncateInt().BigInt()
	require.NotEqual(t, nextFee, following, "fixture must distinguish the neighboring fee")
	floor := f.app.EvmKeeper.GetMinimumFeePerGas(state).TruncateInt().BigInt()
	floor.Mul(floor, big.NewInt(110)).Div(floor, big.NewInt(100)).Add(floor, big.NewInt(1_000_000_000))
	require.GreaterOrEqual(t, price.ToInt().Cmp(floor), 0)
}

func repair751AssertFeeTransition(t *testing.T, byHash bool) {
	t.Helper()
	f := repair751Chain(t)
	parent, child := repair751FeeBlocks(t, f)
	var blocks [2]struct {
		Hash       common.Hash    `json:"hash"`
		ParentHash common.Hash    `json:"parentHash"`
		Number     hexutil.Uint64 `json:"number"`
		GasUsed    hexutil.Uint64 `json:"gasUsed"`
		BaseFee    hexutil.Big    `json:"baseFeePerGas"`
	}
	for i, b := range []*coretypes.ResultBlock{parent, child} {
		method := "eth_getBlockByNumber"
		var identifier any = hexutil.EncodeUint64(uint64(b.Block.Height))
		if byHash {
			method, identifier = "eth_getBlockByHash", common.BytesToHash(b.BlockID.Hash)
		}
		require.NoError(t, json.Unmarshal(repair751Call(t, f.rpc, method, identifier, false), &blocks[i]))
		require.Equal(t, uint64(b.Block.Height), uint64(blocks[i].Number))
		require.Equal(t, common.BytesToHash(b.BlockID.Hash), blocks[i].Hash)
	}
	require.Equal(t, blocks[0].Hash, blocks[1].ParentHash)
	ctx := f.app.RPCContextProvider(parent.Block.Height)
	target := f.app.EvmKeeper.GetTargetGasUsedPerBlock(ctx)
	require.Greater(t, uint64(blocks[0].GasUsed), target, "mandatory workload exceeded the target")
	next := f.app.EvmKeeper.GetNextBaseFeePerGas(ctx).TruncateInt().BigInt()
	require.Equal(t, next, blocks[1].BaseFee.ToInt())
	require.Positive(t, blocks[1].BaseFee.ToInt().Cmp(blocks[0].BaseFee.ToInt()))
	// Advancing the real chain must not alter either pinned committed view.
	f.commit(t, nil, nil)
	for i, b := range []*coretypes.ResultBlock{parent, child} {
		method := "eth_getBlockByNumber"
		var identifier any = hexutil.EncodeUint64(uint64(b.Block.Height))
		if byHash {
			method, identifier = "eth_getBlockByHash", blocks[i].Hash
		}
		var again struct {
			BaseFee hexutil.Big `json:"baseFeePerGas"`
		}
		require.NoError(t, json.Unmarshal(repair751Call(t, f.rpc, method, identifier, false), &again))
		require.Equal(t, blocks[i].BaseFee.ToInt(), again.BaseFee.ToInt())
	}
}

func TestRepairM_751_HashBlockFeeSkips(t *testing.T)   { repair751AssertFeeTransition(t, true) }
func TestRepairM_751_NumberBlockFeeSkips(t *testing.T) { repair751AssertFeeTransition(t, false) }

func TestRepairM_751_ReceiptsDualVMSkip(t *testing.T) {
	f := repair751Chain(t)
	bank, evm := rpcfixtures.CommittedHeaderTransactions(t, f.app)
	// Co-location is controlled by a single real FinalizeBlock request. commit
	// requires every native and EVM result code to succeed before RPC assertions.
	mixed := f.commit(t, append([][]byte{bank}, evm...), nil)
	var block struct {
		Transactions []common.Hash `json:"transactions"`
	}
	require.NoError(t, json.Unmarshal(repair751Call(t, f.rpc, "eth_getBlockByNumber", hexutil.EncodeUint64(uint64(f.height)), false), &block))
	require.Len(t, block.Transactions, len(evm))
	var receipts []struct {
		Hash      common.Hash    `json:"transactionHash"`
		BlockHash common.Hash    `json:"blockHash"`
		Height    hexutil.Uint64 `json:"blockNumber"`
		Status    hexutil.Uint64 `json:"status"`
	}
	require.NoError(t, json.Unmarshal(repair751Call(t, f.rpc, "eth_getBlockReceipts", hexutil.EncodeUint64(uint64(f.height))), &receipts))
	require.Len(t, receipts, len(evm))
	bankHash := common.Hash(tmtypes.Tx(bank).Hash())
	for i, receipt := range receipts {
		require.Equal(t, block.Transactions[i], receipt.Hash)
		require.NotEqual(t, bankHash, receipt.Hash)
		require.Equal(t, common.BytesToHash(mixed.BlockID.Hash), receipt.BlockHash)
		require.Equal(t, uint64(f.height), uint64(receipt.Height))
		require.Equal(t, hexutil.Uint64(1), receipt.Status)
	}
	require.JSONEq(t, `null`, string(repair751Call(t, f.rpc, "eth_getTransactionReceipt", bankHash)))
	require.JSONEq(t, `null`, string(repair751Call(t, f.rpc, "eth_getTransactionByHash", bankHash)))
}
