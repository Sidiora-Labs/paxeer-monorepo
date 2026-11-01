package app_test

import (
	"math/big"
	"testing"
	"time"

	spottypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
	tokenfactorytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/spot"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestV6120SpotServesThroughServedSet(t *testing.T) {
	w := app.NewTestWrapper(t, time.Unix(1_800_000_000, 0).UTC(), secp256k1.GenPrivKey().PubKey(), true)
	t.Cleanup(func() { require.NoError(t, w.App.Close()) })
	a, ctx := w.App, w.Ctx.WithBlockHeight(10)
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(1), app.ActivationUpgrade)
	a.UpgradeKeeper.SetDone(ctx, app.V6120Upgrade)
	versions := a.UpgradeKeeper.GetModuleVersionMap(ctx)
	versions[spottypes.ModuleName] = 1
	a.UpgradeKeeper.SetModuleVersionMap(ctx, versions)
	a.SpotKeeper.InitGenesis(ctx, *spottypes.DefaultGenesis())
	a.RefreshPrecompiles(ctx)
	contracts := a.EvmKeeper.CustomPrecompiles(ctx)
	for _, tc := range []struct {
		address, abi, method string
		want                 *big.Int
	}{
		{spottypes.PECORAddress, "spot_pecor", "BPS_DENOMINATOR", big.NewInt(spottypes.BpsDenominator)},
		{spottypes.PECOROrdersAddress, "spot_orders", "BPS_DENOMINATOR", big.NewInt(spottypes.BpsDenominator)},
		{spottypes.PECORStopOrdersAddress, "spot_stoporders", "BPS_DENOMINATOR", big.NewInt(spottypes.BpsDenominator)},
		{spottypes.PECORQuoterAddress, "spot_quoter", "vault", new(big.Int).SetBytes(common.HexToAddress(spottypes.PECORVaultAddress).Bytes())},
		{spottypes.PECORVaultAddress, "spot_vault", "BPS_DENOMINATOR", big.NewInt(spottypes.BpsDenominator)},
	} {
		t.Run(tc.abi, func(t *testing.T) {
			address := common.HexToAddress(tc.address)
			contract, ok := contracts[address].(*spot.Precompile)
			require.True(t, ok, "v6.12.0 must serve the spot dynamic gas wrapper at %s", address)
			input, err := spot.PrecompileABIs()[tc.abi].Pack(tc.method)
			require.NoError(t, err)
			const supplied = uint64(1_000_000)
			row := spot.GasTable[address][tc.method]
			cost := row.Base + spot.GasPerInputByte*uint64(len(input)-4)
			require.Positive(t, cost)
			var executionGas uint64
			require.NoError(t, a.EvmKeeper.RunWithOneOffEVMInstance(ctx, func(evm *vm.EVM) error {
				runtimeCtx := ctx.WithGasMeter(sdk.NewGasMeterWithMultiplier(ctx, a.EvmKeeper.GetCosmosGasLimitFromEVMGas(ctx, supplied)))
				method := contract.ABI.Methods[tc.method]
				ret, err := contract.GetExecutor().Execute(runtimeCtx, &method, common.Address{}, address, nil, new(big.Int), true, evm, nil)
				require.NoError(t, err)
				require.Equal(t, common.LeftPadBytes(tc.want.Bytes(), 32), ret)
				executionGas = supplied - a.EvmKeeper.GetEVMGasLimitFromCtx(runtimeCtx)
				return nil
			}, func(string, string) {}))
			require.NoError(t, a.EvmKeeper.RunWithOneOffEVMInstance(ctx, func(evm *vm.EVM) error {
				ret, left, err := contract.RunAndCalculateGas(evm, common.Address{}, address, input, supplied, new(big.Int), nil, true, false)
				require.NoError(t, err)
				require.Equal(t, common.LeftPadBytes(tc.want.Bytes(), 32), ret)
				require.Equal(t, supplied-cost-executionGas, left, "served spot reads charge exactly their gas row plus keeper execution")
				_, left, err = contract.RunAndCalculateGas(evm, common.Address{}, address, input, cost-1, new(big.Int), nil, true, false)
				require.ErrorIs(t, err, vm.ErrOutOfGas)
				require.Zero(t, left)
				return nil
			}, func(string, string) {}))
		})
	}
	require.NotContains(t, contracts, common.HexToAddress("0x1026"))
	require.NotContains(t, contracts, common.HexToAddress("0x1027"))
}

func TestGigaBankRefusesPausedTokenFactoryDenom(t *testing.T) {
	w := mergedAppWrapper(t)
	a, ctx := w.App, w.Ctx
	from := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	to := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	paused := "factory/" + from.String() + "/paused"
	active := "factory/" + from.String() + "/active"
	a.TokenFactoryKeeper.InitGenesis(ctx, tokenfactorytypes.GenesisState{
		Params: tokenfactorytypes.DefaultParams(),
		FactoryDenoms: []tokenfactorytypes.GenesisDenom{
			{Denom: paused, AuthorityMetadata: tokenfactorytypes.DenomAuthorityMetadata{Admin: from.String()}},
			{Denom: active, AuthorityMetadata: tokenfactorytypes.DenomAuthorityMetadata{Admin: from.String()}},
		},
	})
	w.FundAcc(from, sdk.NewCoins(sdk.NewInt64Coin(paused, 100), sdk.NewInt64Coin(active, 100)))
	amount := sdk.NewCoins(sdk.NewInt64Coin(paused, 10))
	require.NoError(t, a.GigaBankKeeper.SendCoins(ctx, from, to, amount))
	a.TokenFactoryKeeper.InitGenesis(ctx, tokenfactorytypes.GenesisState{
		Params: tokenfactorytypes.DefaultParams(),
		FactoryDenoms: []tokenfactorytypes.GenesisDenom{
			{Denom: paused, AuthorityMetadata: tokenfactorytypes.DenomAuthorityMetadata{Admin: from.String()}},
			{Denom: active, AuthorityMetadata: tokenfactorytypes.DenomAuthorityMetadata{Admin: from.String()}},
		},
		PausedDenoms: []string{paused},
	})
	require.False(t, a.GigaBankKeeper.IsSendEnabledCoin(ctx, amount[0]))
	require.ErrorIs(t, a.GigaBankKeeper.SendCoins(ctx, from, to, amount), banktypes.ErrSendDisabled)
	require.Equal(t, int64(90), a.GigaBankKeeper.GetBalance(ctx, from, paused).Amount.Int64())
	require.Equal(t, int64(10), a.GigaBankKeeper.GetBalance(ctx, to, paused).Amount.Int64())
	require.True(t, a.GigaBankKeeper.IsSendEnabledCoin(ctx, sdk.NewInt64Coin(active, 1)))
	require.NoError(t, a.GigaBankKeeper.SendCoins(ctx, from, to, sdk.NewCoins(sdk.NewInt64Coin(active, 10))))
	require.Equal(t, int64(10), a.GigaBankKeeper.GetBalance(ctx, to, active).Amount.Int64())
}
