package scheduler_test

import (
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	agentkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/scheduler"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachemulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

const (
	schedulerPlan   = "scheduler-test-merge"
	schedulerHeight = int64(100)
	schedulerGas    = uint64(100_000)
	callGas         = uint64(2_000_000)
)

type schedulerHarness struct {
	t          *testing.T
	keeper     agentkeeper.Keeper
	precompile *scheduler.SchedulerPrecompile
	stateDB    *state.DBImpl
	evm        *vm.EVM
	creator    common.Address
	other      common.Address
	target     common.Address
}

func newSchedulerHarness(t *testing.T) *schedulerHarness {
	t.Helper()
	app := testkeeper.EVMTestApp
	ctx, _ := app.GetContextForDeliverTx(nil).CacheContext()
	key := sdk.NewKVStoreKey(agentkeeper.StoreKey)
	stores := map[sdk.StoreKey]sdk.CacheWrapper{}
	keys := map[string]sdk.StoreKey{}
	for _, existing := range ctx.MultiStore().StoreKeys() {
		stores[existing] = ctx.MultiStore().GetStore(existing)
		keys[existing.Name()] = existing
	}
	stores[key] = dbadapter.Store{DB: dbm.NewMemDB()}
	keys[key.Name()] = key
	ctx = ctx.WithMultiStore(cachemulti.NewStore(dbm.NewMemDB(), stores, keys, nil, nil, nil, 0))
	ctx = ctx.WithBlockHeight(schedulerHeight).WithBlockTime(time.Unix(1_800_000_000, 0))
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx)).WithEventManager(sdk.NewEventManager())
	app.UpgradeKeeper.SetDone(ctx, schedulerPlan)
	app.EvmKeeper.SetNextBaseFeePerGas(ctx, sdk.NewDec(1))
	k := agentkeeper.NewKeeper(key, &app.EvmKeeper, schedulerPlan)
	p, err := scheduler.NewPrecompile(k, &app.EvmKeeper, &app.UpgradeKeeper, schedulerPlan)
	require.NoError(t, err)
	h := &schedulerHarness{t: t, keeper: k, precompile: p,
		creator: common.HexToAddress("0xa111"), other: common.HexToAddress("0xb222"), target: common.HexToAddress("0xc333")}
	for _, address := range []common.Address{h.creator, h.other} {
		account := app.EvmKeeper.GetPaxAddressOrDefault(ctx, address)
		coins := sdk.NewCoins(sdk.NewCoin(sdk.MustGetBaseDenom(), sdk.NewInt(100)))
		require.NoError(t, app.BankKeeper.MintCoins(ctx, "evm", coins))
		require.NoError(t, app.BankKeeper.SendCoinsFromModuleToAccount(ctx, "evm", account, coins))
	}
	h.stateDB = state.NewDBImpl(ctx, &app.EvmKeeper, false)
	block, err := app.EvmKeeper.GetVMBlockContext(h.stateDB.Ctx(), core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	chain := evmtypes.DefaultChainConfig().EthereumConfig(app.EvmKeeper.ChainID(h.stateDB.Ctx()))
	h.evm = vm.NewEVM(*block, h.stateDB, chain, vm.Config{}, map[common.Address]vm.PrecompiledContract{p.Address(): p})
	return h
}

func (h *schedulerHarness) input(name string, args ...interface{}) []byte {
	h.t.Helper()
	a := h.precompile.GetABI()
	input, err := a.Pack(name, args...)
	require.NoError(h.t, err)
	return input
}

func (h *schedulerHarness) call(from common.Address, value *big.Int, name string, args ...interface{}) ([]byte, error) {
	h.t.Helper()
	amount, overflow := uint256.FromBig(value)
	require.False(h.t, overflow)
	ret, _, err := h.evm.Call(from, h.precompile.Address(), h.input(name, args...), callGas, amount)
	require.NoError(h.t, h.stateDB.Err())
	return ret, err
}

func (h *schedulerHarness) write(value *big.Int, name string, args ...interface{}) []interface{} {
	h.t.Helper()
	ret, err := h.call(h.creator, value, name, args...)
	require.NoError(h.t, err)
	a := h.precompile.GetABI()
	output, err := a.Methods[name].Outputs.Unpack(ret)
	require.NoError(h.t, err)
	return output
}

func (h *schedulerHarness) view(name string, args ...interface{}) []interface{} {
	h.t.Helper()
	logs := len(h.stateDB.GetAllLogs())
	ret, _, err := h.evm.StaticCall(h.other, h.precompile.Address(), h.input(name, args...), callGas)
	require.NoError(h.t, err)
	require.Len(h.t, h.stateDB.GetAllLogs(), logs)
	a := h.precompile.GetABI()
	output, err := a.Methods[name].Outputs.Unpack(ret)
	require.NoError(h.t, err)
	return output
}

func (h *schedulerHarness) balance(address common.Address) *big.Int {
	return h.stateDB.GetBalance(address).ToBig()
}

func (h *schedulerHarness) escrow() *big.Int {
	return testkeeper.EVMTestApp.EvmKeeper.GetBalance(h.stateDB.Ctx(), h.keeper.ModuleAddress())
}

func (h *schedulerHarness) schedule() *big.Int {
	h.t.Helper()
	return h.write(big.NewInt(200_000), "schedule", h.target, []byte{1}, uint64(schedulerHeight+10), schedulerGas)[0].(*big.Int)
}

func requireRevert(t *testing.T, ret []byte, err error, code string) {
	t.Helper()
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, decodeErr := abi.UnpackRevert(ret)
	require.NoError(t, decodeErr)
	require.True(t, reason == code || len(reason) > len(code)+2 && reason[:len(code)+2] == code+": ", reason)
}

func (h *schedulerHarness) log(name, topic string, id *big.Int, creator *common.Address, values ...interface{}) {
	h.t.Helper()
	logs := h.stateDB.GetAllLogs()
	require.NotEmpty(h.t, logs)
	entry := logs[len(logs)-1]
	require.Equal(h.t, h.precompile.Address(), entry.Address)
	topics := []common.Hash{common.HexToHash(topic), common.BigToHash(id)}
	if creator != nil {
		topics = append(topics, common.BytesToHash(creator.Bytes()))
	}
	require.Equal(h.t, topics, entry.Topics)
	a := h.precompile.GetABI()
	data, err := a.Events[name].Inputs.NonIndexed().Pack(values...)
	require.NoError(h.t, err)
	require.Equal(h.t, data, entry.Data)
}
