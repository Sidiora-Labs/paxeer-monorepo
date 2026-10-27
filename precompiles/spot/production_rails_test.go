package spot_test

import (
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/spot"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

type productionSpotRails struct {
	t         *testing.T
	app       *app.App
	ctx       sdk.Context
	contracts map[common.Address]vm.PrecompiledContract
}

func newProductionSpotRails(t *testing.T) *productionSpotRails {
	t.Helper()
	a := app.Setup(t, false, true, false)
	ctx := a.GetContextForDeliverTx([]byte{}).WithBlockHeight(1)
	a.UpgradeKeeper.SetDone(ctx, app.V6120Upgrade)
	production := v6120Contracts{app: a}
	production.RefreshPrecompiles(ctx)
	ctx = ctx.WithBlockHeight(2)
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	return &productionSpotRails{t: t, app: a, ctx: ctx, contracts: production.CustomPrecompiles(ctx)}
}

func (f *productionSpotRails) contract(address common.Address) *spot.Precompile {
	f.t.Helper()
	require.Contains(f.t, f.contracts, address)
	contract, ok := f.contracts[address].(*spot.Precompile)
	require.True(f.t, ok, "%s must serve the production dynamic-gas spot wrapper", address.Hex())
	require.NotNil(f.t, contract)
	return contract
}

func (f *productionSpotRails) RunAndCalculateGas(address common.Address, input []byte, gas uint64, value *big.Int) (ret []byte, left uint64, callErr error) {
	f.t.Helper()
	contract := f.contract(address)
	require.NoError(f.t, f.app.EvmKeeper.RunWithOneOffEVMInstance(f.ctx, func(evm *vm.EVM) error {
		ret, left, callErr = contract.RunAndCalculateGas(evm, common.HexToAddress("0x3100"), address, input, gas, value, nil, false, false)
		return nil
	}, func(string, string) {}))
	return
}

func TestProductionPECORReceiveRevertsNativeTransferFailed(t *testing.T) {
	f := newProductionSpotRails(t)
	ret, _, err := f.RunAndCalculateGas(common.HexToAddress("0x1021"), nil, 1_000_000, big.NewInt(1))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.Equal(t, types.ErrNativeTransferFailed.Data(), ret)
}

func TestProductionVaultReceiveRevertsWethOnly(t *testing.T) {
	f := newProductionSpotRails(t)
	ret, _, err := f.RunAndCalculateGas(common.HexToAddress("0x1025"), nil, 1_000_000, big.NewInt(1))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.Equal(t, []byte{0xcf, 0xab, 0x65, 0x17}, ret)
}

func TestProductionSyncAllReservesChargesEveryToken(t *testing.T) {
	f := newProductionSpotRails(t)
	params, err := f.app.SpotKeeper.GetParams(f.ctx)
	require.NoError(t, err)
	params.Tokens = types.OpeningTokens(common.HexToAddress("0x4100"), common.HexToAddress("0x4200"))
	require.NoError(t, f.app.SpotKeeper.UpdateParams(f.ctx, f.app.SpotKeeper.Authority(), params))
	registered, err := f.app.SpotKeeper.GetParams(f.ctx)
	require.NoError(t, err)
	require.Len(t, registered.Tokens, 3)
	require.Equal(t, params.Tokens, registered.Tokens)

	address := common.HexToAddress("0x1025")
	contract := f.contract(address)
	input := packed(t, contract.GetABI(), "syncAllReserves")
	require.Equal(t, uint64(20_000), contract.RequiredGas(input))
	for _, gas := range []uint64{20_000, 3*20_000 - 1} {
		ret, left, err := f.RunAndCalculateGas(address, input, gas, new(big.Int))
		require.ErrorIs(t, err, vm.ErrOutOfGas, "supplied gas %d", gas)
		require.Zero(t, left)
		require.Empty(t, ret)
	}
}

func TestProductionMulticallChargesEveryElement(t *testing.T) {
	f := newProductionSpotRails(t)
	address := common.HexToAddress("0x1021")
	contract := f.contract(address).GetABI()
	paused := packed(t, contract, "paused")
	one := packed(t, contract, "multicall", [][]byte{paused})
	two := packed(t, contract, "multicall", [][]byte{paused, paused})
	const suppliedGas uint64 = 1_000_000
	used := make([]uint64, 0, 2)
	for i, input := range [][]byte{one, two} {
		ret, left, err := f.RunAndCalculateGas(address, input, suppliedGas, new(big.Int))
		require.NoError(t, err)
		require.Less(t, left, suppliedGas)
		values, err := contract.Unpack("multicall", ret)
		require.NoError(t, err)
		require.Len(t, values, 1)
		results, ok := values[0].([][]byte)
		require.True(t, ok)
		require.Len(t, results, i+1)
		for _, result := range results {
			value, err := contract.Unpack("paused", result)
			require.NoError(t, err)
			require.Equal(t, []interface{}{false}, value)
		}
		used = append(used, suppliedGas-left)
	}
	require.GreaterOrEqual(t, used[1], used[0]+3_000)
	additionalInputGas := spot.GasPerInputByte * uint64(len(two)-len(one))
	require.GreaterOrEqual(t, used[1], used[0]+additionalInputGas+3_000)
}
