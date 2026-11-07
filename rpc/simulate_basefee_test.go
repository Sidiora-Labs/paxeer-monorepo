package evmrpc_test

import (
	"context"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	rpclocal "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	rpctest "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/test"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestSimulateHeaderKeepsBaseFee(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	conf, err := rpctest.CreateConfig(t, t.Name())
	require.NoError(t, err)
	kv := kvstore.NewApplication()
	nodeService, closer, err := rpctest.StartTendermint(ctx, conf, kv, rpctest.SuppressStdout)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, closer(context.Background()))
		require.NoError(t, kv.Close())
	})
	nodeWithEnv, ok := nodeService.(rpclocal.NodeService)
	require.True(t, ok)
	tm, err := rpclocal.New(nodeWithEnv)
	require.NoError(t, err)
	const height = int64(2)
	for {
		status, err := tm.Status(ctx)
		require.NoError(t, err)
		if status.SyncInfo.LatestBlockHeight >= height {
			break
		}
		select {
		case <-ctx.Done():
			require.FailNow(t, "chain did not reach simulation height", "%v", ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}

	application := app.Setup(t, false, true, false)
	k := &application.EvmKeeper
	for _, chainID := range []string{"pacific-1", "paxeer-1"} {
		t.Run(chainID, func(t *testing.T) {
			baseCtx, _ := application.GetContextForDeliverTx(nil).CacheContext()
			baseCtx = baseCtx.WithChainID(chainID).WithBlockHeight(height).
				WithBlockTime(time.Now()).WithClosestUpgradeName("v6.0.0")
			application.UpgradeKeeper.SetDone(baseCtx.WithBlockHeight(height+1), "6.2.0")
			require.Greater(t, application.UpgradeKeeper.GetDoneHeight(baseCtx, "6.2.0"), height)

			contract := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
			k.SetCode(baseCtx, contract, common.FromHex("0x4860005260206000f3"))
			previousCtx, _ := baseCtx.WithBlockHeight(height - 1).CacheContext()
			currentCtx, _ := baseCtx.CacheContext()
			k.SetNextBaseFeePerGas(previousCtx, sdk.NewDec(123456789))
			k.SetNextBaseFeePerGas(currentCtx, sdk.NewDec(987654321))
			sender := common.HexToAddress("0xabcdef1234567890abcdef1234567890abcdef12")
			funds := sdk.NewCoins(sdk.NewCoin(k.GetBaseDenom(currentCtx), sdk.NewInt(1000000)))
			require.NoError(t, k.BankKeeper().MintCoins(currentCtx, types.ModuleName, funds))
			require.NoError(t, k.BankKeeper().SendCoinsFromModuleToAccount(currentCtx, types.ModuleName, sdk.AccAddress(sender.Bytes()), funds))
			want := k.GetNextBaseFeePerGas(previousCtx).TruncateInt().BigInt()
			require.Positive(t, want.Sign())
			require.NotEqual(t, want, k.GetNextBaseFeePerGas(currentCtx).TruncateInt().BigInt())
			ctxProvider := func(h int64) sdk.Context {
				switch h {
				case height - 1:
					return previousCtx
				case height, evmrpc.LatestCtxHeight:
					return currentCtx
				default:
					panic("unexpected simulation context height")
				}
			}
			txConfigProvider := func(int64) client.TxConfig { return TxConfig }
			watermarks := evmrpc.NewWatermarkManager(tm, ctxProvider, nil, nil)
			cache := evmrpc.NewBlockCache(3000)
			cacheMutex := &sync.Mutex{}
			backend := evmrpc.NewBackend(ctxProvider, k, application.BeginBlockKeepers, txConfigProvider,
				tm, &SConfig, application.BaseApp, application.TracerAnteHandler, cache, cacheMutex, watermarks)
			header, err := backend.HeaderByNumber(ctx, rpc.LatestBlockNumber)
			require.NoError(t, err)
			require.Equal(t, height, header.Number.Int64())
			require.Equal(t, want, header.BaseFee)

			api := evmrpc.NewSimulationAPI(ctxProvider, k, application.BeginBlockKeepers, txConfigProvider,
				tm, &SConfig, application.BaseApp, application.TracerAnteHandler,
				evmrpc.ConnectionTypeHTTP, cache, cacheMutex, watermarks)
			server := rpc.NewServer()
			t.Cleanup(server.Stop)
			require.NoError(t, server.RegisterName("eth", api))
			rpcClient := rpc.DialInProc(server)
			t.Cleanup(rpcClient.Close)
			var result hexutil.Bytes
			err = rpcClient.CallContext(ctx, &result, "eth_call", map[string]any{
				"from": sender.Hex(), "to": contract.Hex(), "gasPrice": hexutil.EncodeBig(want),
			}, "latest")
			require.NoError(t, err)
			require.Len(t, result, 32)
			require.Equal(t, want, new(big.Int).SetBytes(result))
		})
	}
}
