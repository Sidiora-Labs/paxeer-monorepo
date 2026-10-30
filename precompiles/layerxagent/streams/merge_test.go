package streams_test

import (
	"fmt"
	"math/big"
	"testing"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/streams"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxpay"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestStreamsMergeConstructorNeedsPayKeeper(t *testing.T) {
	application := testkeeper.EVMTestApp
	keepers := app.NewPrecompileKeepers(application)
	var _ layerxpay.LayerXPayKeepers = keepers
	withoutPay := *application
	withoutPay.LayerXPayKeeper = nil
	missingEVM := app.NewPrecompileKeepers(application)
	missingEVM.EVMKeeper = nil
	nilEVM := app.NewPrecompileKeepers(application)
	nilEVM.EVMKeeper = (*evmkeeper.Keeper)(nil)
	for _, tc := range []struct {
		name    string
		keepers utils.Keepers
	}{
		{"nil keepers", nil},
		{"empty keepers", &utils.EmptyKeepers{}},
		{"missing pay capability", struct{ utils.Keepers }{keepers}},
		{"nil pay keeper", app.NewPrecompileKeepers(&withoutPay)},
		{"missing evm", missingEVM},
		{"nil evm pointer", nilEVM},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := streams.NewMergePrecompile(tc.keepers, precompiles.MergeUpgrade)
			require.Nil(t, p)
			require.ErrorIs(t, err, agentcommon.ErrNoContext)
		})
	}
	p, err := streams.NewMergePrecompile(keepers, precompiles.MergeUpgrade)
	require.NoError(t, err)
	address := common.HexToAddress("0x0000000000000000000000000000000000000906")
	require.Equal(t, address, p.Address())
	require.Equal(t, "streams", p.GetName())
	for _, missing := range precompiles.MissingMergeConstructors() {
		require.NotEqual(t, address, missing.Address)
	}
	served, err := precompiles.GetCustomPrecompilesE(app.LatestUpgrade, keepers)
	require.NoError(t, err)
	require.Contains(t, served, address)
	require.IsType(t, p, served[address][precompiles.MergeUpgrade])
	require.EqualError(t, precompiles.RegisterMergeConstructor(address, streams.NewMergePrecompile),
		"merge constructor for "+address.Hex()+" already registered")
	p, err = streams.NewMergePrecompile(keepers, "")
	require.Nil(t, p)
	require.ErrorIs(t, err, agentcommon.ErrNotActive)
}

func TestStreamsMergePrecompileHeightGated(t *testing.T) {
	h := newStreamsHarness(t)
	application := *testkeeper.EVMTestApp
	application.LayerXPayKeeper = h.keeper
	const plan = "streams-constructor-merge"
	built, err := streams.NewMergePrecompile(app.NewPrecompileKeepers(&application), plan)
	require.NoError(t, err)
	h.precompile = built.(*agentcommon.Precompile)
	h.evm.SetPrecompiles(map[common.Address]vm.PrecompiledContract{built.Address(): built})
	ctx := h.db.Ctx()
	application.UpgradeKeeper.SetDone(ctx, plan)
	before := h.keeper.ExportGenesis(ctx)
	logs := len(h.db.GetAllLogs())
	calls := []struct {
		name string
		args []interface{}
	}{
		{"open", []interface{}{h.payee.caller, h.token, big.NewInt(2), streamsStart, streamsStart + 100, big.NewInt(100)}},
		{"settle", []interface{}{big.NewInt(1)}},
		{"close", []interface{}{big.NewInt(1)}},
		{"updateRate", []interface{}{big.NewInt(1), big.NewInt(3)}},
		{"accrued", []interface{}{big.NewInt(1)}},
		{"getStream", []interface{}{big.NewInt(1)}},
	}
	for _, height := range []int64{1, ctx.BlockHeight() - 1} {
		for _, call := range calls {
			t.Run(fmt.Sprintf("%s/height_%d", call.name, height), func(t *testing.T) {
				db := state.NewDBImpl(ctx.WithBlockHeight(height), &application.EvmKeeper, false)
				out, err := built.Run(&vm.EVM{StateDB: db}, h.payer.caller, built.Address(), h.input(call.name, call.args...), nil, built.GetABI().Methods[call.name].IsConstant(), false, nil)
				requireStreamsRevert(t, out, err, "LXP_AGENT_NOT_ACTIVE")
				require.ErrorIs(t, db.GetPrecompileError(), agentcommon.ErrNotActive)
				require.Equal(t, before, h.keeper.ExportGenesis(db.Ctx()))
				require.Empty(t, db.GetAllLogs())
			})
		}
		for _, input := range [][]byte{nil, {0xff, 0xff, 0xff, 0xff}} {
			db := state.NewDBImpl(ctx.WithBlockHeight(height), &application.EvmKeeper, false)
			out, err := built.Run(&vm.EVM{StateDB: db}, h.payer.caller, built.Address(), input, nil, true, false, nil)
			requireStreamsRevert(t, out, err, "LXP_AGENT_NOT_ACTIVE")
			require.ErrorIs(t, db.GetPrecompileError(), agentcommon.ErrNotActive)
		}
	}
	require.Equal(t, before, h.keeper.ExportGenesis(ctx))
	require.Len(t, h.db.GetAllLogs(), logs)
	zero := &streamOutput{Id: new(big.Int), RatePerSecond: new(big.Int), Cap: new(big.Int), Settled: new(big.Int)}
	missing := abi.ConvertType(h.view("getStream", big.NewInt(1))[0], new(streamOutput)).(*streamOutput)
	requireStreamTuple(t, h, zero, missing)
	id := h.open()
	require.Positive(t, id.Sign())
	require.False(t, h.record(id).Closed)
	require.NotEmpty(t, h.view("getStream", id))
	h.advance(1)
	require.Equal(t, big.NewInt(2), h.view("accrued", id)[0])
}
