package orob_test

import (
	"math"
	"math/big"
	"testing"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/orob"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestOrobMergeConstructorRegistered(t *testing.T) {
	address := ethcommon.HexToAddress("0x0000000000000000000000000000000000000901")
	require.EqualError(t, precompiles.RegisterMergeConstructor(address, orob.NewMergePrecompile),
		"merge constructor for "+address.Hex()+" already registered")

	for _, test := range []struct {
		name    string
		keepers utils.Keepers
	}{
		{"nil keepers", nil},
		{"missing evm keeper", &app.PrecompileKeepers{}},
		{"nil evm keeper", &app.PrecompileKeepers{EVMKeeper: (*evmkeeper.Keeper)(nil)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			p, err := orob.NewMergePrecompile(test.keepers, precompiles.MergeUpgrade)
			require.Nil(t, p)
			require.ErrorIs(t, err, agentcommon.ErrNoContext)
		})
	}

	_, ctx, upgrade, evmKeeper := fixture(t, 3)
	keepers := &app.PrecompileKeepers{EVMKeeper: evmKeeper}
	upgrade.SetDone(ctx, precompiles.MergeUpgrade)
	p, err := orob.NewMergePrecompile(keepers, precompiles.MergeUpgrade)
	require.NoError(t, err)
	require.NotNil(t, p)
	require.Equal(t, address, p.Address())
	require.Equal(t, "orob", p.GetName())
	require.Len(t, p.GetABI().Methods, 3)
	input, err := p.GetABI().Pack(orob.ResolveOffsetMethod, big.NewInt(10000), int16(100))
	require.NoError(t, err)
	data, err := p.Run(evmContext(ctx), ethcommon.Address{}, address, input, nil, true, false, nil)
	require.NoError(t, err)
	values, err := p.GetABI().Unpack(orob.ResolveOffsetMethod, data)
	require.NoError(t, err)
	require.Equal(t, []interface{}{big.NewInt(10100)}, values)
	data, err = p.Run(evmContext(ctx.WithBlockHeight(99)), ethcommon.Address{}, address, input, nil, true, false, nil)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.Equal(t, "LXP_AGENT_NOT_ACTIVE: "+precompiles.MergeUpgrade+" not active at height 99", revertText(t, data))

	for _, test := range []struct {
		name    string
		keepers utils.Keepers
		plan    string
	}{
		{"missing upgrade keeper", &app.PrecompileKeepers{EVMKeeper: new(evmkeeper.Keeper)}, precompiles.MergeUpgrade},
		{"missing plan", keepers, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			p, err := orob.NewMergePrecompile(test.keepers, test.plan)
			require.Nil(t, p)
			require.ErrorIs(t, err, agentcommon.ErrNotActive)
		})
	}
}

func TestOrobMergeBatchCapAndOverflow(t *testing.T) {
	require.Equal(t, 64, orob.MergeMaxBatchSize)
	_, ctx, _, evmKeeper := fixture(t, 3)
	p, err := orob.NewMergePrecompile(&app.PrecompileKeepers{EVMKeeper: evmKeeper}, "merge-plan")
	require.NoError(t, err)

	offsets := make([]int16, orob.MergeMaxBatchSize)
	for i := range offsets {
		offsets[i] = int16(i - 32)
	}
	input, err := p.GetABI().Pack(orob.ResolveOffsetBatchMethod, big.NewInt(10000), offsets)
	require.NoError(t, err)
	data, err := p.Run(evmContext(ctx), ethcommon.Address{}, p.Address(), input, nil, true, false, nil)
	require.NoError(t, err)
	values, err := p.GetABI().Unpack(orob.ResolveOffsetBatchMethod, data)
	require.NoError(t, err)
	require.Len(t, values, 1)
	prices, ok := values[0].([]*big.Int)
	require.True(t, ok)
	require.Len(t, prices, orob.MergeMaxBatchSize)
	for i, offset := range offsets {
		require.Equal(t, big.NewInt(10000+int64(offset)), prices[i])
	}

	input, err = p.GetABI().Pack(orob.ToOffsetMethod, big.NewInt(10000), big.NewInt(42767))
	require.NoError(t, err)
	data, err = p.Run(evmContext(ctx), ethcommon.Address{}, p.Address(), input, nil, true, false, nil)
	require.NoError(t, err)
	values, err = p.GetABI().Unpack(orob.ToOffsetMethod, data)
	require.NoError(t, err)
	require.Equal(t, []interface{}{int16(math.MaxInt16)}, values)

	for _, test := range []struct {
		name   string
		method string
		args   []interface{}
		code   agentcommon.ErrorCode
		reason string
	}{
		{"above batch cap", orob.ResolveOffsetBatchMethod, []interface{}{big.NewInt(10000), make([]int16, orob.MergeMaxBatchSize+1)}, agentcommon.ErrBatchTooLarge, "LXP_AGENT_BATCH_TOO_LARGE: batch over the cap (cap set at activation rehearsal)"},
		{"empty batch", orob.ResolveOffsetBatchMethod, []interface{}{big.NewInt(10000), []int16{}}, agentcommon.ErrABIDecode, "LXP_AGENT_ABI_DECODE: argument decode failed"},
		{"first int16 overflow", orob.ToOffsetMethod, []interface{}{big.NewInt(10000), big.NewInt(42768)}, agentcommon.ErrOverflow, "LXP_AGENT_OVERFLOW: result out of range"},
		{"large offset overflow", orob.ToOffsetMethod, []interface{}{big.NewInt(1), maxPrice()}, agentcommon.ErrOverflow, "LXP_AGENT_OVERFLOW: result out of range"},
	} {
		t.Run(test.name, func(t *testing.T) {
			input, err := p.GetABI().Pack(test.method, test.args...)
			require.NoError(t, err)
			evm := evmContext(ctx)
			data, err := p.Run(evm, ethcommon.Address{}, p.Address(), input, nil, true, false, nil)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			require.ErrorIs(t, state.GetDBImpl(evm.StateDB).GetPrecompileError(), test.code)
			require.Equal(t, test.reason, revertText(t, data))
		})
	}
}
