package evm_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestExportImportAccountFeeDenom(t *testing.T) {
	source := app.Setup(t, false, false, false)
	ctx := source.GetContextForDeliverTx(nil).WithBlockHeight(8)
	k := &source.EvmKeeper
	account := common.HexToAddress("0x1234")
	other := common.HexToAddress("0x5678")
	const denom = "usid"

	params := types.DefaultParams()
	params.FeeTokenEnabled = true
	params.AllowedFeeDenoms = []types.AllowedFeeDenom{{
		Denom: denom, Rate: sdk.NewDec(types.InitialSidioraBaseUnitsPerPax), RateUpdateHeight: ctx.BlockHeight(),
	}}
	k.SetParams(ctx, params)
	require.NoError(t, k.SetAccountFeeDenom(ctx, account, denom))

	genesis := evm.ExportGenesis(ctx, k)
	require.NoError(t, genesis.Validate())
	require.Contains(t, genesis.Serialized, &types.Serialized{
		Prefix: []byte{0x23}, Key: account.Bytes(), Value: []byte(denom),
	})

	target := app.Setup(t, false, false, false)
	targetCtx := target.GetContextForDeliverTx(nil).WithBlockHeight(8)
	restored := &target.EvmKeeper
	require.False(t, targetCtx.KVStore(restored.GetStoreKey()).Has(types.AccountFeeDenomKey(account)))
	require.Equal(t, restored.GetBaseDenom(targetCtx), restored.GetAccountFeeDenom(targetCtx, account))

	evm.InitGenesis(targetCtx, restored, *genesis)
	require.Equal(t, denom, restored.GetAccountFeeDenom(targetCtx, account))
	require.Equal(t, []byte(denom), targetCtx.KVStore(restored.GetStoreKey()).Get(types.AccountFeeDenomKey(account)))
	require.Equal(t, restored.GetBaseDenom(targetCtx), restored.GetAccountFeeDenom(targetCtx, other))
	require.Equal(t, params.AllowedFeeDenoms, restored.GetParams(targetCtx).AllowedFeeDenoms)
	require.True(t, restored.GetFeeTokenEnabled(targetCtx))

	for _, entry := range []*types.Serialized{
		{Prefix: types.AccountFeeDenomKeyPrefix, Key: []byte{1}, Value: []byte(denom)},
		{Prefix: types.AccountFeeDenomKeyPrefix, Key: account.Bytes(), Value: []byte("bad denom")},
	} {
		err := types.ValidateAccountFeeDenomGenesisEntry(entry.Prefix, entry.Key, entry.Value)
		require.Error(t, err)
		invalid := *genesis
		invalid.Serialized = []*types.Serialized{entry}
		invalidCtx, _ := targetCtx.CacheContext()
		require.PanicsWithError(t, err.Error(), func() {
			evm.InitGenesis(invalidCtx, restored, invalid)
		})
	}
}
