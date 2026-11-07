package evmrpc_test

import (
	stdbytes "bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/bytes"
	tenderminttypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/mock"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/legacyabci"
	"github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	bam "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	cosmostestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	txtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	receipt "github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/receipt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/fixtures/example/contracts/simplestorage"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/export"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func primeReceiptStore(t *testing.T, store receipt.ReceiptStore, latest int64) {
	t.Helper()
	if store == nil {
		return
	}
	if latest <= 0 {
		latest = 1
	}
	require.NoError(t, store.SetLatestVersion(latest))
	require.NoError(t, store.SetEarliestVersion(1))
	require.Equal(t, int64(1), store.EarliestVersion())
}

// bcAlwaysFailClient fails every Block call (header resolution uses a single block fetch).
type bcAlwaysFailClient struct{ *MockClient }

func (bc bcAlwaysFailClient) Block(ctx context.Context, h *int64) (*coretypes.ResultBlock, error) {
	return nil, fmt.Errorf("fail bc")
}

func TestEstimateGas(t *testing.T) {
	Ctx = Ctx.WithBlockHeight(1)
	// transfer
	_, from := testkeeper.MockAddressPair()
	_, to := testkeeper.MockAddressPair()
	txArgs := map[string]any{
		"from":    from.Hex(),
		"to":      to.Hex(),
		"value":   "0x10",
		"nonce":   "0x1",
		"chainId": fmt.Sprintf("%#x", EVMKeeper.ChainID(Ctx)),
	}
	amts := sdk.NewCoins(sdk.NewCoin(EVMKeeper.GetBaseDenom(Ctx), sdk.NewInt(20)))
	EVMKeeper.BankKeeper().MintCoins(Ctx, types.ModuleName, amts)
	EVMKeeper.BankKeeper().SendCoinsFromModuleToAccount(Ctx, types.ModuleName, sdk.AccAddress(from[:]), amts)
	resObj := sendRequestGood(t, "estimateGas", txArgs, nil, map[string]any{})
	require.Nil(t, resObj["error"], "estimate gas refused: %v", resObj["error"])
	result := resObj["result"].(string)
	require.Equal(t, "0x5208", result) // 21000
	resObj = sendRequestGood(t, "estimateGas", txArgs, "latest", map[string]any{})
	result = resObj["result"].(string)
	require.Equal(t, "0x5208", result) // 21000
	resObj = sendRequestGood(t, "estimateGas", txArgs, "0x1", map[string]any{})
	result = resObj["result"].(string)
	require.Equal(t, "0x5208", result) // 21000

	// contract call
	_, contractAddr := testkeeper.MockAddressPair()
	code, err := os.ReadFile("../tests/fixtures/example/contracts/simplestorage/SimpleStorage.bin")
	require.Nil(t, err)
	bz, err := hex.DecodeString(string(code))
	require.Nil(t, err)
	abi, err := simplestorage.SimplestorageMetaData.GetAbi()
	require.Nil(t, err)
	input, err := abi.Pack("set", big.NewInt(20))
	require.Nil(t, err)
	EVMKeeper.SetCode(Ctx, contractAddr, bz)
	txArgs = map[string]any{
		"from":    from.Hex(),
		"to":      contractAddr.Hex(),
		"value":   "0x0",
		"nonce":   "0x2",
		"chainId": fmt.Sprintf("%#x", EVMKeeper.ChainID(Ctx)),
		"input":   fmt.Sprintf("%#x", input),
	}
	resObj = sendRequestGood(t, "estimateGas", txArgs, nil, map[string]any{})
	result = resObj["result"].(string)
	require.Equal(t, "0x54ac", result) // 21497

	Ctx = Ctx.WithBlockHeight(8)
}

func TestChainConfigReflectsSstoreParam(t *testing.T) {
	testApp := app.Setup(t, false, false, false)
	baseCtx := testApp.GetContextForDeliverTx([]byte{})

	oldCtx, _ := baseCtx.CacheContext()
	oldCtx = oldCtx.WithBlockHeight(100).WithIsTracing(true)

	newCtx, _ := baseCtx.CacheContext()
	newCtx = newCtx.WithBlockHeight(200).WithIsTracing(true)
	params := testApp.EvmKeeper.GetParams(newCtx)
	params.PaxSstoreSetGasEip2200 = 72000
	testApp.EvmKeeper.SetParams(newCtx, params)
	primeReceiptStore(t, testApp.EvmKeeper.ReceiptStore(), newCtx.BlockHeight())

	oldHeight := oldCtx.BlockHeight()
	ctxProvider := func(height int64) sdk.Context {
		switch {
		case height == evmrpc.LatestCtxHeight:
			return newCtx
		case height >= newCtx.BlockHeight():
			return newCtx
		case height == oldHeight:
			return oldCtx
		default:
			return newCtx
		}
	}

	encodingCfg := app.MakeEncodingConfig()
	tmClient := &MockClient{}
	watermarks := evmrpc.NewWatermarkManager(tmClient, ctxProvider, nil, testApp.EvmKeeper.ReceiptStore())
	backend := evmrpc.NewBackend(
		ctxProvider,
		&testApp.EvmKeeper,
		legacyabci.BeginBlockKeepers{},
		func(int64) client.TxConfig { return encodingCfg.TxConfig },
		tmClient,
		&SConfig,
		testApp.BaseApp,
		testApp.TracerAnteHandler,
		evmrpc.NewBlockCache(3000),
		&sync.Mutex{},
		watermarks,
	)

	oldCfg := backend.ChainConfigAtHeight(oldHeight)
	require.NotNil(t, oldCfg.PaxSstoreSetGasEIP2200)
	require.Equal(t, types.DefaultPaxSstoreSetGasEIP2200, *oldCfg.PaxSstoreSetGasEIP2200)

	latestCfg := backend.ChainConfig()
	require.NotNil(t, latestCfg.PaxSstoreSetGasEIP2200)
	require.Equal(t, uint64(72000), *latestCfg.PaxSstoreSetGasEIP2200)
}

func TestEstimateGasAfterCalls(t *testing.T) {
	Ctx = Ctx.WithBlockHeight(1)
	// estimate get after set
	_, from := testkeeper.MockAddressPair()
	amts := sdk.NewCoins(sdk.NewCoin(EVMKeeper.GetBaseDenom(Ctx), sdk.NewInt(20)))
	EVMKeeper.BankKeeper().MintCoins(Ctx, types.ModuleName, amts)
	EVMKeeper.BankKeeper().SendCoinsFromModuleToAccount(Ctx, types.ModuleName, sdk.AccAddress(from[:]), amts)
	_, contractAddr := testkeeper.MockAddressPair()
	code, err := os.ReadFile("../tests/fixtures/example/contracts/simplestorage/SimpleStorage.bin")
	require.Nil(t, err)
	bz, err := hex.DecodeString(string(code))
	require.Nil(t, err)
	abi, err := simplestorage.SimplestorageMetaData.GetAbi()
	require.Nil(t, err)
	call, err := abi.Pack("set", big.NewInt(20))
	require.Nil(t, err)
	input, err := abi.Pack("get")
	require.Nil(t, err)
	EVMKeeper.SetCode(Ctx, contractAddr, bz)
	txArgs := map[string]any{
		"from":    from.Hex(),
		"to":      contractAddr.Hex(),
		"value":   "0x0",
		"nonce":   "0x2",
		"chainId": fmt.Sprintf("%#x", EVMKeeper.ChainID(Ctx)),
		"input":   fmt.Sprintf("%#x", input),
	}
	callArgs := map[string]any{
		"from":    from.Hex(),
		"to":      contractAddr.Hex(),
		"value":   "0x0",
		"nonce":   "0x2",
		"chainId": fmt.Sprintf("%#x", EVMKeeper.ChainID(Ctx)),
		"input":   fmt.Sprintf("%#x", call),
	}
	resObj := sendRequestGood(t, "estimateGasAfterCalls", txArgs, []any{callArgs}, nil, map[string]any{})
	result := resObj["result"].(string)
	require.Equal(t, "0x536d", result) // 21357 for get

	Ctx = Ctx.WithBlockHeight(8)
}

func TestCreateAccessList(t *testing.T) {
	Ctx = Ctx.WithBlockHeight(1)

	_, from := testkeeper.MockAddressPair()
	_, contractAddr := testkeeper.MockAddressPair()
	code, err := os.ReadFile("../tests/fixtures/example/contracts/simplestorage/SimpleStorage.bin")
	require.Nil(t, err)
	bz, err := hex.DecodeString(string(code))
	require.Nil(t, err)
	abi, err := simplestorage.SimplestorageMetaData.GetAbi()
	require.Nil(t, err)
	input, err := abi.Pack("set", big.NewInt(20))
	require.Nil(t, err)
	EVMKeeper.SetCode(Ctx, contractAddr, bz)
	txArgs := map[string]any{
		"from":    from.Hex(),
		"to":      contractAddr.Hex(),
		"value":   "0x0",
		"nonce":   "0x1",
		"chainId": fmt.Sprintf("%#x", EVMKeeper.ChainID(Ctx)),
		"input":   fmt.Sprintf("%#x", input),
	}
	amts := sdk.NewCoins(sdk.NewCoin(EVMKeeper.GetBaseDenom(Ctx), sdk.NewInt(2000000)))
	EVMKeeper.BankKeeper().MintCoins(Ctx, types.ModuleName, amts)
	EVMKeeper.BankKeeper().SendCoinsFromModuleToAccount(Ctx, types.ModuleName, sdk.AccAddress(from[:]), amts)
	resObj := sendRequestGood(t, "createAccessList", txArgs, "latest")
	result := resObj["result"].(map[string]any)
	require.Equal(t, []any{}, result["accessList"]) // the code uses MSTORE which does not trace access list

	resObj = sendRequestBad(t, "createAccessList", txArgs, "latest")
	result = resObj["error"].(map[string]any)
	require.Equal(t, "error block", result["message"])

	Ctx = Ctx.WithBlockHeight(8)
}

func TestCall(t *testing.T) {
	Ctx = Ctx.WithBlockHeight(1)

	_, from := testkeeper.MockAddressPair()
	_, contractAddr := testkeeper.MockAddressPair()
	code, err := os.ReadFile("../tests/fixtures/example/contracts/simplestorage/SimpleStorage.bin")
	require.Nil(t, err)
	bz, err := hex.DecodeString(string(code))
	require.Nil(t, err)
	abi, err := simplestorage.SimplestorageMetaData.GetAbi()
	require.Nil(t, err)
	input, err := abi.Pack("set", big.NewInt(20))
	require.Nil(t, err)
	EVMKeeper.SetCode(Ctx, contractAddr, bz)
	txArgs := map[string]any{
		"from":    from.Hex(),
		"to":      contractAddr.Hex(),
		"value":   "0x0",
		"nonce":   "0x2",
		"chainId": fmt.Sprintf("%#x", EVMKeeper.ChainID(Ctx)),
		"input":   fmt.Sprintf("%#x", input),
	}
	resObj := sendRequestGood(t, "call", txArgs, nil, map[string]any{}, map[string]any{})
	result := resObj["result"].(string)
	require.Equal(t, "0x608060405234801561000f575f80fd5b506004361061003f575f3560e01c806360fe47b1146100435780636d4ce63c1461005f5780639c3674fc1461007d575b5f80fd5b61005d6004803603810190610058919061010a565b610087565b005b6100676100c7565b6040516100749190610144565b60405180910390f35b6100856100cf565b005b805f819055507f0de2d86113046b9e8bb6b785e96a6228f6803952bf53a40b68a36dce316218c1816040516100bc9190610144565b60405180910390a150565b5f8054905090565b5f80fd5b5f80fd5b5f819050919050565b6100e9816100d7565b81146100f3575f80fd5b50565b5f81359050610104816100e0565b92915050565b5f6020828403121561011f5761011e6100d3565b5b5f61012c848285016100f6565b91505092915050565b61013e816100d7565b82525050565b5f6020820190506101575f830184610135565b9291505056fea2646970667358221220bb55137839ea2afda11ab2d30ad07fee30bb9438caaa46e30ccd1053ed72439064736f6c63430008150033", result)

	Ctx = Ctx.WithBlockHeight(8)
}

func TestEthCallHighAmount(t *testing.T) {
	Ctx = Ctx.WithBlockHeight(1)
	_, from := testkeeper.MockAddressPair()
	_, to := testkeeper.MockAddressPair()
	txArgs := map[string]any{
		"from":    from.Hex(),
		"to":      to.Hex(),
		"value":   "0x0",
		"nonce":   "0x2",
		"chainId": fmt.Sprintf("%#x", EVMKeeper.ChainID(Ctx)),
	}

	overrides := map[string]map[string]any{
		from.Hex(): {"balance": "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"},
	}
	resObj := sendRequestGood(t, "call", txArgs, "latest", overrides)
	errMap := resObj["error"].(map[string]any)
	result := errMap["message"]
	require.Equal(t, result, "error: balance override overflow")

	Ctx = Ctx.WithBlockHeight(8)
}

func TestNewRevertError(t *testing.T) {
	err := evmrpc.NewRevertError(&core.ExecutionResult{})
	require.NotNil(t, err)
	require.Equal(t, 3, err.ErrorCode())
	require.Equal(t, "0x", err.ErrorData())
}

func TestConvertBlockNumber(t *testing.T) {
	tmClient := &MockClient{}
	watermarks := evmrpc.NewWatermarkManager(tmClient, func(i int64) sdk.Context {
		if i == evmrpc.LatestCtxHeight {
			return sdk.Context{}.WithBlockHeight(1000)
		}
		return sdk.Context{}
	}, nil, nil)
	backend := evmrpc.NewBackend(func(i int64) sdk.Context {
		if i == evmrpc.LatestCtxHeight {
			return sdk.Context{}.WithBlockHeight(1000)
		}
		return sdk.Context{}
	}, nil, legacyabci.BeginBlockKeepers{}, nil, &MockClient{}, nil, nil, nil, evmrpc.NewBlockCache(3000), &sync.Mutex{}, watermarks)
	converted, err := backend.ConvertBlockNumber(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, int64(10), converted)
	converted, err = backend.ConvertBlockNumber(context.Background(), 0)
	require.NoError(t, err)
	require.Equal(t, int64(1), converted)
	converted, err = backend.ConvertBlockNumber(context.Background(), rpc.SafeBlockNumber)
	require.NoError(t, err)
	require.Equal(t, int64(1000), converted)
	converted, err = backend.ConvertBlockNumber(context.Background(), rpc.FinalizedBlockNumber)
	require.NoError(t, err)
	require.Equal(t, int64(1000), converted)
	converted, err = backend.ConvertBlockNumber(context.Background(), rpc.LatestBlockNumber)
	require.NoError(t, err)
	require.Equal(t, int64(1000), converted)
	_, err = backend.ConvertBlockNumber(context.Background(), rpc.PendingBlockNumber)
	require.ErrorContains(t, err, "pending block")
}

// Header gasLimit comes from the active SDK ConsensusParams at the block's height.
func TestGasLimitUsesConsensusOrConfig(t *testing.T) {
	testApp := app.Setup(t, false, false, false)
	baseCtx := testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(1).
		WithConsensusParams(&tenderminttypes.ConsensusParams{Block: &tenderminttypes.BlockParams{MaxGas: 200_000_000}})

	ctxProvider := func(h int64) sdk.Context { return baseCtx.WithBlockHeight(h) }
	cfg := &evmrpc.SimulateConfig{GasCap: 10_000_000, EVMTimeout: time.Second}

	tmClient := &MockClient{}
	watermarks := evmrpc.NewWatermarkManager(tmClient, ctxProvider, nil, testApp.EvmKeeper.ReceiptStore())
	backend := evmrpc.NewBackend(ctxProvider, &testApp.EvmKeeper,
		legacyabci.BeginBlockKeepers{},
		func(int64) client.TxConfig { return TxConfig },
		tmClient, cfg, testApp.BaseApp, testApp.TracerAnteHandler, evmrpc.NewBlockCache(3000), &sync.Mutex{}, watermarks)

	header, err := backend.HeaderByNumber(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, uint64(200_000_000), header.GasLimit)

	header2, err := backend.HeaderByNumber(context.Background(), 0)
	require.NoError(t, err)
	require.Equal(t, uint64(200_000_000), header2.GasLimit)
}

func TestGasLimitRequiresCanonicalConsensusParams(t *testing.T) {
	testApp := app.Setup(t, false, false, false)
	cfg := &evmrpc.SimulateConfig{GasCap: 20_000_000, EVMTimeout: time.Second}

	nilParamsCtx := testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(1).WithConsensusParams(nil)
	ctxProvider1 := func(h int64) sdk.Context { return nilParamsCtx.WithBlockHeight(h) }
	tmClient1 := &MockClient{}
	watermarks1 := evmrpc.NewWatermarkManager(tmClient1, ctxProvider1, nil, testApp.EvmKeeper.ReceiptStore())
	backend1 := evmrpc.NewBackend(ctxProvider1, &testApp.EvmKeeper, legacyabci.BeginBlockKeepers{}, func(int64) client.TxConfig { return TxConfig }, tmClient1, cfg, testApp.BaseApp, testApp.TracerAnteHandler, evmrpc.NewBlockCache(3000), &sync.Mutex{}, watermarks1)
	_, err := backend1.HeaderByNumber(context.Background(), 1)
	require.ErrorContains(t, err, "consensus block parameters are unavailable")

	// Case 2: Block fails — resolution errors out entirely.
	baseCtx := testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(1)
	ctxProvider2 := func(h int64) sdk.Context { return baseCtx.WithBlockHeight(h) }
	bcClient := &bcAlwaysFailClient{MockClient: &MockClient{}}
	watermarks2 := evmrpc.NewWatermarkManager(bcClient, ctxProvider2, nil, testApp.EvmKeeper.ReceiptStore())
	backend2 := evmrpc.NewBackend(ctxProvider2, &testApp.EvmKeeper, legacyabci.BeginBlockKeepers{}, func(int64) client.TxConfig { return TxConfig }, bcClient, cfg, testApp.BaseApp, testApp.TracerAnteHandler, evmrpc.NewBlockCache(3000), &sync.Mutex{}, watermarks2)
	_, err = backend2.HeaderByNumber(context.Background(), 1)
	require.Error(t, err)
}

// Exercises CurrentHeader: block fetch + getHeader vs fallback when Block RPC fails.
// HeaderByNumber / BlockByNumber cover getHeader with an already-resolved tmBlock.
func TestSimulateBackendBlockResolutionCoverage(t *testing.T) {
	testApp := app.Setup(t, false, false, false)
	baseCtx := testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(1)
	ctxProvider := func(h int64) sdk.Context {
		if h == evmrpc.LatestCtxHeight {
			return baseCtx
		}
		return baseCtx.WithBlockHeight(h)
	}
	cfg := &evmrpc.SimulateConfig{GasCap: 10_000_000, EVMTimeout: time.Second}
	primeReceiptStore(t, testApp.EvmKeeper.ReceiptStore(), 1)
	tmClient := &MockClient{}
	watermarks := evmrpc.NewWatermarkManager(tmClient, ctxProvider, nil, testApp.EvmKeeper.ReceiptStore())
	backend := evmrpc.NewBackend(ctxProvider, &testApp.EvmKeeper,
		legacyabci.BeginBlockKeepers{}, func(int64) client.TxConfig { return TxConfig },
		tmClient, cfg, testApp.BaseApp, testApp.TracerAnteHandler, evmrpc.NewBlockCache(3000), &sync.Mutex{}, watermarks)

	t.Run("CurrentHeader_fetches_block_then_getHeader", func(t *testing.T) {
		h := backend.CurrentHeader()
		require.NotNil(t, h)
		require.Equal(t, int64(1), h.Number.Int64())
		require.Equal(t, common.BytesToHash(mockBlockHeader(1).LastBlockID.Hash), h.ParentHash)
	})

	t.Run("CurrentHeader_does_not_fabricate_when_block_unavailable", func(t *testing.T) {
		bcClient := &bcAlwaysFailClient{MockClient: &MockClient{}}
		wm := evmrpc.NewWatermarkManager(bcClient, ctxProvider, nil, testApp.EvmKeeper.ReceiptStore())
		b2 := evmrpc.NewBackend(ctxProvider, &testApp.EvmKeeper,
			legacyabci.BeginBlockKeepers{}, func(int64) client.TxConfig { return TxConfig },
			bcClient, cfg, testApp.BaseApp, testApp.TracerAnteHandler, evmrpc.NewBlockCache(3000), &sync.Mutex{}, wm)
		h := b2.CurrentHeader()
		require.Nil(t, h)
	})
}

// repairM1297StorageTrace coordinates real SLOADs through the SDK trace store.
// It observes storage without replacing the EVM, keeper, store or RPC handler.
type repairM1297StorageTrace struct {
	key       []byte
	mu        sync.Mutex
	remaining int
	entered   chan struct{}
	release   chan struct{}
}

func (w *repairM1297StorageTrace) Write(p []byte) (int, error) {
	var operation struct {
		Operation string `json:"operation"`
		Key       []byte `json:"key"`
	}
	if err := json.Unmarshal(p, &operation); err != nil {
		return 0, err
	}
	if operation.Operation == "read" && stdbytes.Equal(operation.Key, w.key) {
		w.mu.Lock()
		hold := w.remaining > 0
		if hold {
			w.remaining--
		}
		w.mu.Unlock()
		if hold {
			w.entered <- struct{}{}
			<-w.release
		}
	}
	return len(p), nil
}

func repairM1297EstimateLimiter(t *testing.T) {
	t.Helper()
	for _, scenario := range []struct{ name, method string }{
		{"TestEthCallRateLimiting", "call"},
		{"TestEstimateGasRateLimiting", "estimateGas"},
		{"TestEstimateGasAfterCallsRateLimiting", "estimateGasAfterCalls"},
		{"TestSequentialRequestsAfterLoad", "call"},
		{"TestDifferentMethodsShareSameLimiter", "mixed"},
		{"TestRateLimitErrorFormat", "call"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			a := app.Setup(t, false, false, false)
			base := a.GetContextForDeliverTx(nil).WithBlockHeight(MockHeight8)
			base = base.WithConsensusParams(a.GetConsensusParams(base))
			from := common.HexToAddress("0x129701")
			to := common.HexToAddress("0x129702")
			// SLOAD(0), MSTORE(0), RETURN(0,32).
			a.EvmKeeper.SetCode(base, to, common.FromHex("0x60005460005260206000f3"))
			a.EvmKeeper.SetState(base, to, common.Hash{}, common.BigToHash(big.NewInt(42)))
			coins := sdk.NewCoins(sdk.NewInt64Coin(a.EvmKeeper.GetBaseDenom(base), 2_000_000_000))
			require.NoError(t, a.BankKeeper.MintCoins(base, types.ModuleName, coins))
			require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(base, types.ModuleName, a.EvmKeeper.GetPaxAddressOrDefault(base, from), coins))
			trace := &repairM1297StorageTrace{
				key: append(types.StateKey(to), make([]byte, 32)...), remaining: 2,
				entered: make(chan struct{}, 2), release: make(chan struct{}),
			}
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(trace.release) }) }
			defer release()
			var providerMu sync.Mutex
			provider := func(int64) sdk.Context {
				providerMu.Lock()
				defer providerMu.Unlock()
				traced := base.MultiStore().CacheMultiStore().SetTracer(trace).CacheMultiStore()
				return base.WithMultiStore(traced).WithIsTracing(true)
			}
			peer := &MockClient{latestOverride: MockHeight8}
			wm := evmrpc.NewWatermarkManager(peer, provider, nil, nil)
			api := evmrpc.NewSimulationAPI(provider, &a.EvmKeeper, a.BeginBlockKeepers,
				func(int64) client.TxConfig { return a.GetTxConfig() }, peer,
				&evmrpc.SimulateConfig{GasCap: 1_000_000, EVMTimeout: 5 * time.Second, MaxConcurrentSimulationCalls: 2},
				a.BaseApp, a.TracerAnteHandler, evmrpc.ConnectionTypeHTTP, evmrpc.NewBlockCache(10), &sync.Mutex{}, wm)
			args := export.TransactionArgs{From: &from, To: &to}
			latest := rpc.BlockNumberOrHashWithNumber(rpc.LatestBlockNumber)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			run := func(name string) error {
				switch name {
				case "call":
					data, err := api.Call(ctx, args, &latest, nil, nil)
					if err == nil && !stdbytes.Equal(data, common.BigToHash(big.NewInt(42)).Bytes()) {
						return fmt.Errorf("SLOAD output: %x", data)
					}
					return err
				case "estimateGas":
					gas, err := api.EstimateGas(ctx, args, &latest, nil)
					if err == nil && gas <= 21000 {
						return fmt.Errorf("SLOAD estimate %d omitted execution", gas)
					}
					return err
				default:
					gas, err := api.EstimateGasAfterCalls(ctx, args, []export.TransactionArgs{args}, &latest, nil)
					if err == nil && gas <= 21000 {
						return fmt.Errorf("SLOAD estimate after calls %d omitted execution", gas)
					}
					return err
				}
			}
			results := make(chan error, 2)
			for i := 0; i < 2; i++ {
				method := scenario.method
				if method == "mixed" {
					if i == 0 {
						method = "call"
					} else {
						method = "estimateGas"
					}
				}
				go func() { results <- run(method) }()
				select {
				case <-trace.entered:
				case err := <-results:
					t.Fatalf("execution ended before SLOAD: %v", err)
				case <-ctx.Done():
					t.Fatal("execution did not reach SLOAD: ", ctx.Err())
				}
			}
			require.EqualError(t, run("estimateGas"), "eth_estimateGas rejected due to rate limit: server busy")
			require.EqualError(t, run("estimateGasAfterCalls"), "eth_estimateGasAfterCalls rejected due to rate limit: server busy")
			require.EqualError(t, run("call"), "eth_call rejected due to rate limit: server busy")
			release()
			for i := 0; i < 2; i++ {
				select {
				case err := <-results:
					require.NoError(t, err)
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
			}
			for _, next := range []string{"call", "estimateGas", "estimateGasAfterCalls"} {
				require.NoError(t, run(next), "permit reuse by %s", next)
			}
		})
	}
}

func TestSimulationAPIRequestLimiter(t *testing.T) {
	repairM1297EstimateLimiter(t)
}

func repairM1297EstimateLimiterProof(t *testing.T) {
	repairM1297EstimateLimiter(t)
}

// fixedBlockClient is a local test client that returns a single block for
// every Block(height) call, ignoring the requested height. Lets the test
// pin a specific Block.Header / BlockID combination without dragging in the
// rest of the mock infrastructure.
type fixedBlockClient struct {
	mock.Client
	block *coretypes.ResultBlock
}

func (c *fixedBlockClient) EvmNextPendingNonce(common.Address) uint64 {
	return 0
}

func (c *fixedBlockClient) EvmTxByHash(common.Hash) (tmtypes.Tx, bool) {
	return nil, false
}

func (c *fixedBlockClient) EvmProxy(common.Address) (*url.URL, bool) {
	return nil, false
}

func (c *fixedBlockClient) Block(_ context.Context, _ *int64) (*coretypes.ResultBlock, error) {
	return c.block, nil
}

func (c *fixedBlockClient) BlockResults(_ context.Context, _ *int64) (*coretypes.ResultBlockResults, error) {
	txResults := make([]*abci.ExecTxResult, len(c.block.Block.Txs))
	for i := range txResults {
		txResults[i] = &abci.ExecTxResult{}
	}
	return &coretypes.ResultBlockResults{
		Height:     c.block.Block.Height,
		TxsResults: txResults,
	}, nil
}

func (c *fixedBlockClient) Status(_ context.Context) (*coretypes.ResultStatus, error) {
	return &coretypes.ResultStatus{
		SyncInfo: coretypes.SyncInfo{
			LatestBlockHeight:   c.block.Block.Height,
			EarliestBlockHeight: 1,
		},
	}, nil
}

func TestTraceBlockByNumberUsesCompatDecoderForHistoricalCosmosTx(t *testing.T) {
	const (
		blockHeight = int64(42)
		v65Height   = int64(100)
	)

	testApp := app.Setup(t, false, false, false)
	ctx := testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(v65Height).WithClosestUpgradeName("v6.5")
	testApp.UpgradeKeeper.SetDone(ctx, "v6.5")
	ctxProvider := func(height int64) sdk.Context {
		if height == evmrpc.LatestCtxHeight {
			return ctx
		}
		return ctx.WithBlockHeight(height)
	}

	_, fromAddr := testkeeper.MockAddressPair()
	_, toAddr := testkeeper.MockAddressPair()
	txBuilder := TxConfig.NewTxBuilder()
	require.NoError(t, txBuilder.SetMsgs(banktypes.NewMsgSend(
		sdk.AccAddress(fromAddr.Bytes()),
		sdk.AccAddress(toAddr.Bytes()),
		sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewInt(1))),
	)))
	txBz, err := Encoder(txBuilder.GetTx())
	require.NoError(t, err)

	tests := []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{
			name: "memo explicitly encoded as default empty string",
			mutate: func(bodyBytes []byte) []byte {
				return append(bodyBytes, 0x12, 0x00)
			},
		},
		{
			name: "timeout height explicitly encoded as default zero",
			mutate: func(bodyBytes []byte) []byte {
				return append(bodyBytes, 0x18, 0x00)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var raw txtypes.TxRaw
			require.NoError(t, raw.Unmarshal(txBz))
			raw.BodyBytes = tt.mutate(raw.BodyBytes)
			bloatedTxBz, err := raw.Marshal()
			require.NoError(t, err)

			_, err = TxConfig.TxDecoder()(bloatedTxBz)
			require.Error(t, err)
			require.Contains(t, err.Error(), "exceeds canonical size")

			makeBlock := func(height int64) *coretypes.ResultBlock {
				return &coretypes.ResultBlock{
					BlockID: tmtypes.BlockID{Hash: bytes.HexBytes(mustHexToBytes("0000000000000000000000000000000000000000000000000000000000000042"))},
					Block: &tmtypes.Block{
						Header: mockBlockHeader(height),
						Data:   tmtypes.Data{Txs: []tmtypes.Tx{bloatedTxBz}},
						LastCommit: &tmtypes.Commit{
							Height: height,
						},
					},
				}
			}
			tmClient := &fixedBlockClient{block: makeBlock(blockHeight)}
			watermarks := evmrpc.NewWatermarkManager(tmClient, ctxProvider, nil, testApp.EvmKeeper.ReceiptStore())
			backend := evmrpc.NewBackend(
				ctxProvider, &testApp.EvmKeeper, legacyabci.BeginBlockKeepers{},
				func(int64) client.TxConfig { return TxConfig }, tmClient, &SConfig,
				testApp.BaseApp, testApp.TracerAnteHandler,
				evmrpc.NewBlockCache(3000), &sync.Mutex{}, watermarks,
			)

			ethBlock, metadata, err := backend.BlockByNumber(context.Background(), rpc.BlockNumber(blockHeight))
			require.NoError(t, err)
			require.Len(t, ethBlock.Transactions(), 0)
			require.Len(t, metadata, 1)
			require.False(t, metadata[0].ShouldIncludeInTraceResult)
			require.NotNil(t, metadata[0].TraceRunnable)

			strictTmClient := &fixedBlockClient{block: makeBlock(v65Height)}
			strictWatermarks := evmrpc.NewWatermarkManager(strictTmClient, ctxProvider, nil, testApp.EvmKeeper.ReceiptStore())
			strictBackend := evmrpc.NewBackend(
				ctxProvider, &testApp.EvmKeeper, legacyabci.BeginBlockKeepers{},
				func(int64) client.TxConfig { return TxConfig }, strictTmClient, &SConfig,
				testApp.BaseApp, testApp.TracerAnteHandler,
				evmrpc.NewBlockCache(3000), &sync.Mutex{}, strictWatermarks,
			)
			_, _, err = strictBackend.BlockByNumber(context.Background(), rpc.BlockNumber(v65Height))
			require.Error(t, err)
			require.Contains(t, err.Error(), "exceeds canonical size")
		})
	}
}

// TestBlockByNumberNonTracedTxPassesTxBytes verifies that when BlockByNumber
// produces a TraceRunnable for a non-EVM (Cosmos) transaction, the runnable
// calls DeliverTx with req.Tx populated. An empty req.Tx causes ctx.TxBytes()
// to be nil inside the ante handler, which zeroes out TxSizeCostPerByte gas
// and diverges from what actually happened on-chain.
func TestBlockByNumberNonTracedTxPassesTxBytes(t *testing.T) {
	const blockHeight = int64(42)

	// Build a minimal BaseApp whose sole purpose is to capture ctx.TxBytes()
	// as seen by the ante handler. BaseApp sets ctx.TxBytes = req.Tx before
	// invoking the ante handler, so this is the correct interception point.
	var capturedTxBytes []byte
	minApp := bam.NewBaseApp("test", dbm.NewMemDB(), TxConfig.TxDecoder(), nil, &cosmostestutil.TestAppOpts{})
	minApp.SetAnteHandler(func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
		capturedTxBytes = ctx.TxBytes()
		return ctx, fmt.Errorf("stop") // short-circuit; we only need ante
	})
	minApp.Seal()

	testApp := app.Setup(t, false, false, false)
	_, fromAddr := testkeeper.MockAddressPair()
	_, toAddr := testkeeper.MockAddressPair()
	txBuilder := TxConfig.NewTxBuilder()
	require.NoError(t, txBuilder.SetMsgs(banktypes.NewMsgSend(
		sdk.AccAddress(fromAddr.Bytes()),
		sdk.AccAddress(toAddr.Bytes()),
		sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewInt(1))),
	)))
	rawTx, err := Encoder(txBuilder.GetTx())
	require.NoError(t, err)

	tmBlock := &coretypes.ResultBlock{
		BlockID: tmtypes.BlockID{Hash: bytes.HexBytes(mustHexToBytes("0000000000000000000000000000000000000000000000000000000000000042"))},
		Block: &tmtypes.Block{
			Header:     mockBlockHeader(blockHeight),
			Data:       tmtypes.Data{Txs: []tmtypes.Tx{rawTx}},
			LastCommit: &tmtypes.Commit{Height: blockHeight},
		},
	}
	ctx := testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(blockHeight)
	ctxProvider := func(height int64) sdk.Context {
		if height == evmrpc.LatestCtxHeight {
			return ctx
		}
		return ctx.WithBlockHeight(height)
	}

	tmClient := &fixedBlockClient{block: tmBlock}
	watermarks := evmrpc.NewWatermarkManager(tmClient, ctxProvider, nil, testApp.EvmKeeper.ReceiptStore())
	backend := evmrpc.NewBackend(
		ctxProvider, &testApp.EvmKeeper, legacyabci.BeginBlockKeepers{},
		func(int64) client.TxConfig { return TxConfig }, tmClient, &SConfig,
		minApp, testApp.TracerAnteHandler,
		evmrpc.NewBlockCache(3000), &sync.Mutex{}, watermarks,
	)

	_, metadata, err := backend.BlockByNumber(context.Background(), rpc.BlockNumber(blockHeight))
	require.NoError(t, err)
	require.Len(t, metadata, 1)
	require.NotNil(t, metadata[0].TraceRunnable)

	sdkCtx := testApp.GetContextForDeliverTx([]byte(rawTx)).WithBlockHeight(blockHeight)
	stateDB := state.NewDBImpl(sdkCtx, &testApp.EvmKeeper, false)
	metadata[0].TraceRunnable(stateDB)

	require.Equal(t, []byte(rawTx), capturedTxBytes, "DeliverTx must receive the raw tx bytes so ante handler gas charging is faithful to the original execution")
}

// TestGetTransactionUsesBlockIDHash pins down the GetTransaction → blockHash
// contract: callers (notably go-ethereum's tracers.API.TraceTransaction,
// which hands the returned blockHash to BlockByHash for cross-validation)
// must get the BlockID.Hash that the EVM receipt store recorded during
// FinalizeBlock. Two scenarios, each with its own teeth:
//
//   - CometBFT-shaped block: every Header field that contributes to the
//     Merkle root is populated. Subtest asserts the fixture invariant
//     (Header.Hash() == BlockID.Hash) AND that GetTransaction returns
//     that shared value. Catches a regression where the function
//     somehow stops returning a hash at all — the fix is value-
//     equivalent here, but a downstream change that, say, returns the
//     parent hash would still fail.
//   - Autobahn-shaped block: GigaRouter.translateGlobalBlock returns a
//     ResultBlock with a sparse Header (only ChainID/Height/Time set)
//     and BlockID.Hash explicitly carrying the Autobahn block hash.
//     Subtest asserts the fixture invariant (Header.Hash() != BlockID.Hash)
//     AND that GetTransaction returns BlockID.Hash. This is the case
//     the original code got wrong: Header.Hash() recomputes a Merkle
//     root over the sparse fields that doesn't match anything stored,
//     so debug_traceTransaction's blockByNumberAndHash check downstream
//     sends BlockByHash on a wild goose chase and fails with
//     ErrBlockNotFoundByHash.
func TestGetTransactionUsesBlockIDHash(t *testing.T) {
	const txHeight = int64(42)

	mkBlock := func(header tmtypes.Header, blockIDHash []byte, txBz []byte) *coretypes.ResultBlock {
		return &coretypes.ResultBlock{
			BlockID: tmtypes.BlockID{Hash: bytes.HexBytes(blockIDHash)},
			Block: &tmtypes.Block{
				Header: header,
				Data:   tmtypes.Data{Txs: []tmtypes.Tx{txBz}},
				LastCommit: &tmtypes.Commit{
					Height: header.Height,
				},
			},
		}
	}

	// fullHeader is what CometBFT produces — every field populated, so
	// Header.Hash() round-trips through any tendermint-aware consumer.
	fullHeader := mockBlockHeader(txHeight)

	// sparseHeader is what GigaRouter.translateGlobalBlock produces under
	// Autobahn: ChainID / Height / Time only. Header.Hash() over this
	// computes a different value than any stored BlockID.Hash.
	sparseHeader := tmtypes.Header{
		ChainID: "test",
		Height:  txHeight,
		Time:    time.Unix(1696941649, 0),
	}

	// Two distinct hashes so a buggy implementation that picks the wrong
	// source visibly fails — not just "happens to match by coincidence".
	autobahnHash := mustHexToBytes("00000000000000000000000000000000000000000000000000000000000000ab")

	type tcase struct {
		name        string
		header      tmtypes.Header
		blockIDHash []byte
		// fixtureInvariant asserts the structural relationship between
		// Header.Hash() and BlockID.Hash that this scenario simulates,
		// so the test fails loudly if the fixture itself stops
		// representing what it's supposed to.
		fixtureInvariant func(t *testing.T, headerHash, blockIDHash []byte)
	}

	tcases := []tcase{
		{
			name:        "CometBFT (full header)",
			header:      fullHeader,
			blockIDHash: fullHeader.Hash().Bytes(),
			fixtureInvariant: func(t *testing.T, headerHash, blockIDHash []byte) {
				require.Equal(t, headerHash, blockIDHash, "CometBFT shape: full Header.Hash() must equal BlockID.Hash")
			},
		},
		{
			name:        "Autobahn (sparse header)",
			header:      sparseHeader,
			blockIDHash: autobahnHash,
			fixtureInvariant: func(t *testing.T, headerHash, blockIDHash []byte) {
				require.NotEqual(t, headerHash, blockIDHash, "Autobahn shape: sparse Header.Hash() must differ from BlockID.Hash")
			},
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			tc.fixtureInvariant(t, tc.header.Hash().Bytes(), tc.blockIDHash)
			testApp := app.Setup(t, false, false, false)
			ctx := testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(txHeight)
			primeReceiptStore(t, testApp.EvmKeeper.ReceiptStore(), txHeight)

			// Build a real tx the test can probe by hash, encoded the same
			// way the real path encodes via b.txConfigProvider.
			_, fromAddr := testkeeper.MockAddressPair()
			_, toAddr := testkeeper.MockAddressPair()
			gp := sdk.NewInt(1)
			amt := sdk.NewInt(1)
			builder := TxConfig.NewTxBuilder()
			msg, err := types.NewMsgEVMTransaction(&ethtx.LegacyTx{
				Nonce:    0,
				GasPrice: &gp,
				GasLimit: 21000,
				To:       toAddr.Hex(),
				Amount:   &amt,
			})
			require.NoError(t, err)
			require.NoError(t, builder.SetMsgs(msg))
			signedTx := builder.GetTx()
			txBz, err := Encoder(signedTx)
			require.NoError(t, err)

			// The receipt is what GetTransaction's first call resolves to
			// the block; index 0 must match the encoded tx position.
			ethTx, _ := msg.AsTransaction()
			require.NotNil(t, ethTx)
			require.NoError(t, testApp.EvmKeeper.MockReceipt(ctx, ethTx.Hash(), &types.Receipt{
				BlockNumber:      uint64(txHeight),
				TransactionIndex: 0,
				From:             fromAddr.Hex(),
				TxHashHex:        ethTx.Hash().Hex(),
			}))

			block := mkBlock(tc.header, tc.blockIDHash, txBz)
			tmClient := &fixedBlockClient{block: block}

			ctxProvider := func(int64) sdk.Context { return ctx }
			watermarks := evmrpc.NewWatermarkManager(tmClient, ctxProvider, nil, testApp.EvmKeeper.ReceiptStore())
			backend := evmrpc.NewBackend(
				ctxProvider, &testApp.EvmKeeper, legacyabci.BeginBlockKeepers{},
				func(int64) client.TxConfig { return TxConfig }, tmClient, &SConfig,
				testApp.BaseApp, testApp.TracerAnteHandler,
				evmrpc.NewBlockCache(3000), &sync.Mutex{}, watermarks,
			)

			found, _, blockHash, blockNumber, idx, err := backend.GetTransaction(context.Background(), ethTx.Hash())
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, uint64(txHeight), blockNumber)
			require.Equal(t, uint64(0), idx)
			// The contract: GetTransaction returns BlockID.Hash (NOT a
			// fresh Header.Hash()).
			require.Equal(t, common.BytesToHash(tc.blockIDHash), blockHash)
		})
	}
}

func TestStateAndHeaderUsesBlockTime(t *testing.T) {
	testApp := app.Setup(t, false, false, false)
	blockTime := time.Unix(1696941649, 0)
	ctxProvider := func(height int64) sdk.Context {
		return testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(height).WithBlockTime(blockTime.Add(20 * time.Second))
	}
	tmClient := &MockClient{}
	watermarks := evmrpc.NewWatermarkManager(tmClient, ctxProvider, nil, testApp.EvmKeeper.ReceiptStore())
	backend := evmrpc.NewBackend(ctxProvider, &testApp.EvmKeeper, legacyabci.BeginBlockKeepers{},
		func(int64) client.TxConfig { return TxConfig }, tmClient,
		&evmrpc.SimulateConfig{GasCap: 10000000, EVMTimeout: 30 * time.Second},
		testApp.BaseApp, testApp.TracerAnteHandler, evmrpc.NewBlockCache(3000), &sync.Mutex{}, watermarks)

	latest := rpc.LatestBlockNumber
	sdb, header, err := backend.StateAndHeaderByNumberOrHash(context.Background(), rpc.BlockNumberOrHash{BlockNumber: &latest})
	require.NoError(t, err)
	require.Equal(t, uint64(blockTime.Unix()), header.Time)
	require.True(t, blockTime.Equal(sdb.(*state.DBImpl).Ctx().BlockTime()))
}

func TestGetBlockContextUsesRequestedBlock(t *testing.T) {
	testApp := app.Setup(t, false, false, false)
	const height int64 = 100
	blockTime := time.Unix(1696941649, 0).UTC()
	stateTime := blockTime.Add(5 * time.Second)
	sdkCtx := testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(height + 1).WithBlockTime(stateTime)
	ctxProvider := func(int64) sdk.Context { return sdkCtx }
	watermarks := evmrpc.NewWatermarkManager(nil, ctxProvider, nil, testApp.EvmKeeper.ReceiptStore())
	backend := evmrpc.NewBackend(ctxProvider, &testApp.EvmKeeper, legacyabci.BeginBlockKeepers{},
		func(int64) client.TxConfig { return TxConfig }, nil,
		&evmrpc.SimulateConfig{GasCap: 10000000, EVMTimeout: 30 * time.Second},
		testApp.BaseApp, testApp.TracerAnteHandler, evmrpc.NewBlockCache(3000), &sync.Mutex{}, watermarks)
	stateDB := state.NewDBImpl(sdkCtx, &testApp.EvmKeeper, true)

	for _, tc := range []struct {
		name   string
		height int64
		time   time.Time
	}{
		{name: "historical block", height: height, time: blockTime},
		{name: "state context block", height: height + 1, time: stateTime},
	} {
		t.Run(tc.name, func(t *testing.T) {
			block := ethtypes.NewBlockWithHeader(&ethtypes.Header{
				Number: big.NewInt(tc.height),
				Time:   uint64(tc.time.Unix()),
			})
			blockCtx, err := backend.GetBlockContext(context.Background(), block, stateDB, nil)
			require.NoError(t, err)
			require.Equal(t, big.NewInt(tc.height), blockCtx.BlockNumber)
			require.Equal(t, uint64(tc.time.Unix()), blockCtx.Time)
			require.Equal(t, height+1, stateDB.Ctx().BlockHeight())
			require.True(t, stateTime.Equal(stateDB.Ctx().BlockTime()))
		})
	}
}

func init() {
	evmrpc.RepairM1297Proofs["estimate-limiter-not-proven"] = repairM1297EstimateLimiterProof
}
