package integration_test

import (
	"context"
	"encoding/json"
	"math"
	"math/big"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	agenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/scheduler"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestAppExportsAgentGenesis(t *testing.T) {
	blockTime := time.Unix(1_800_000_000, 0).UTC()
	w := app.NewTestWrapper(t, blockTime, secp256k1.GenPrivKey().PubKey(), true)
	t.Cleanup(func() { require.NoError(t, w.App.Close()) })
	a, ctx := w.App, w.Ctx
	a.UpgradeKeeper.SetDone(ctx, app.MergeUpgrade)
	a.RefreshPrecompiles(ctx)
	a.EvmKeeper.SetNextBaseFeePerGas(ctx, sdk.NewDec(1))
	params := agenttypes.DefaultParams()
	params.MaxJobsPerCreator = 7
	require.NoError(t, a.LayerXAgentKeeper.SetParams(ctx, params))
	require.Equal(t, uint64(1), a.LayerXAgentKeeper.NextJobID(ctx))

	creator := common.HexToAddress("0xa111")
	target := common.HexToAddress("0xc333")
	callData := []byte{0x12, 0x34, 0, 0xff}
	const executeAt, gasLimit = uint64(100), uint64(100_000)
	deposit := big.NewInt(3_000_000_000_017)
	w.FundAcc(a.EvmKeeper.GetPaxAddressOrDefault(ctx, creator),
		sdk.NewCoins(sdk.NewInt64Coin(a.EvmKeeper.GetBaseDenom(ctx), 10)))
	address := common.HexToAddress(scheduler.SchedulerAddress)
	require.Equal(t, common.HexToAddress("0x0000000000000000000000000000000000000905"), address)
	served := a.EvmKeeper.CustomPrecompiles(ctx)
	require.Contains(t, served, address)
	precompile, ok := served[address].(*scheduler.SchedulerPrecompile)
	require.True(t, ok)
	contractABI := precompile.GetABI()
	input, err := contractABI.Pack("schedule", target, callData, executeAt, gasLimit)
	require.NoError(t, err)
	stateDB := state.NewDBImpl(ctx, &a.EvmKeeper, false)
	block, err := a.EvmKeeper.GetVMBlockContext(stateDB.Ctx(), core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	chain := evmtypes.DefaultChainConfig().EthereumConfig(a.EvmKeeper.ChainID(ctx))
	evm := vm.NewEVM(*block, stateDB, chain, vm.Config{}, served)
	value, overflow := uint256.FromBig(deposit)
	require.False(t, overflow)
	output, _, err := evm.Call(creator, address, input, 2_000_000, value)
	require.NoError(t, err)
	require.NoError(t, stateDB.Err())
	result, err := contractABI.Unpack("schedule", output)
	require.NoError(t, err)
	require.Len(t, result, 1)
	id, ok := result[0].(*big.Int)
	require.True(t, ok)
	require.Equal(t, big.NewInt(1), id)
	_, err = stateDB.Finalize()
	require.NoError(t, err)

	queryJob := func(application *app.App, queryCtx sdk.Context) *agenttypes.QueryJobResponse {
		t.Helper()
		client := agenttypes.NewQueryClient(&baseapp.QueryServiceTestHelper{
			GRPCQueryRouter: application.GRPCQueryRouter(), Ctx: queryCtx,
		})
		response, err := client.Job(context.Background(), &agenttypes.QueryJobRequest{Id: id.Uint64()})
		require.NoError(t, err)
		require.NotNil(t, response)
		return response
	}
	wantJob := agenttypes.Job{
		Id: 1, Creator: creator.Hex(), Target: target.Hex(), CallData: callData,
		ExecuteAtBlock: executeAt, GasLimit: gasLimit, Deposit: deposit.String(), Active: true,
	}
	before := queryJob(a, ctx)
	require.Equal(t, wantJob, before.Job)
	nextJobID := a.LayerXAgentKeeper.NextJobID(ctx)
	require.Equal(t, uint64(2), nextJobID)
	moduleAddress := a.LayerXAgentKeeper.ModuleAddress()
	coinsBefore := a.BankKeeper.GetAllBalances(ctx, moduleAddress)
	weiBefore := a.BankKeeper.GetWeiBalance(ctx, moduleAddress)
	depositCoins, depositWei := state.SplitUhpxWeiAmount(deposit)
	require.Equal(t, sdk.NewCoins(sdk.NewCoin(a.EvmKeeper.GetBaseDenom(ctx), depositCoins)), coinsBefore)
	require.Equal(t, depositWei, weiBefore)
	require.True(t, weiBefore.IsPositive())

	*a.DeliverContext() = ctx
	a.SetDeliverStateToCommit()
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	exported, err := a.ExportAppStateAndValidators(false, nil)
	require.NoError(t, err)
	var genesis map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(exported.AppState, &genesis))
	require.Contains(t, genesis, agenttypes.ModuleName)
	var agentGenesis agenttypes.GenesisState
	require.NoError(t, a.AppCodec().UnmarshalAsJSON(genesis[agenttypes.ModuleName], &agentGenesis))
	require.NoError(t, agentGenesis.Validate())
	require.Equal(t, []agenttypes.Job{wantJob}, agentGenesis.Jobs)
	require.Equal(t, uint64(2), agentGenesis.NextJobId)
	require.Equal(t, nextJobID, agentGenesis.NextJobId)
	require.Equal(t, params, agentGenesis.Params)

	restored := app.Setup(t, true, true, false)
	t.Cleanup(func() { require.NoError(t, restored.Close()) })
	_, err = restored.InitChain(context.Background(), &abci.RequestInitChain{
		Time: blockTime, ChainId: ctx.ChainID(), InitialHeight: exported.Height,
		ConsensusParams: exported.ConsensusParams, AppStateBytes: exported.AppState,
	})
	require.NoError(t, err)
	restoredCtx := restored.NewContext(false, tmproto.Header{
		Height: exported.Height, ChainID: ctx.ChainID(), Time: blockTime,
	})
	after := queryJob(restored, restoredCtx)
	require.Equal(t, before, after)
	require.Equal(t, agentGenesis.NextJobId, restored.LayerXAgentKeeper.NextJobID(restoredCtx))
	require.Equal(t, params, restored.LayerXAgentKeeper.GetParams(restoredCtx))
	require.Equal(t, []uint64{id.Uint64()}, restored.LayerXAgentKeeper.Pending(restoredCtx, creator))
	require.Equal(t, coinsBefore, restored.BankKeeper.GetAllBalances(restoredCtx, restored.LayerXAgentKeeper.ModuleAddress()))
	require.Equal(t, weiBefore, restored.BankKeeper.GetWeiBalance(restoredCtx, restored.LayerXAgentKeeper.ModuleAddress()))
}
