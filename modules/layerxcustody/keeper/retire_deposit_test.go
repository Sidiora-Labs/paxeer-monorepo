package keeper_test

import (
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
)

func TestDepositIsRefusedFromTheMergePlanHeight(t *testing.T) {
	e := newEnvWithoutCheckpoint(t, 0)
	app := testkeeper.EVMTestApp
	assetID := vectorArray(t, e.withdrawal, "asset")
	beneficiary := [32]byte{0xbe, 0xef}
	amount := sdk.NewInt(1_000)
	denom := sdk.MustGetBaseDenom()
	const planHeight int64 = 20
	payerBefore := e.balance(e.payerAcc, denom)
	moduleBefore := e.balance(e.k.ModuleAddress(), denom)
	countBefore := e.k.GetDepositCount(e.ctx)
	require.True(t, payerBefore.GTE(amount))
	app.UpgradeKeeper.SetDone(e.ctx.WithBlockHeight(planHeight), types.MergePlanUpgrade)
	require.Equal(t, planHeight, app.UpgradeKeeper.GetDoneHeight(e.ctx, types.MergePlanUpgrade))

	t.Run("below merge plan height", func(t *testing.T) {
		ctx := e.ctx.WithBlockHeight(planHeight - 1).WithEventManager(sdk.NewEventManager())
		deposit, err := e.k.Deposit(ctx, e.payer, e.payerAcc, assetID, beneficiary, amount)
		require.NoError(t, err)
		require.Equal(t, planHeight-1, deposit.Height)
		require.Equal(t, payerBefore.Sub(amount), e.balance(e.payerAcc, denom))
		require.Equal(t, moduleBefore.Add(amount), e.balance(e.k.ModuleAddress(), denom))
		require.Equal(t, countBefore+1, e.k.GetDepositCount(e.ctx))
	})
	payerBefore = e.balance(e.payerAcc, denom)
	moduleBefore = e.balance(e.k.ModuleAddress(), denom)
	countBefore = e.k.GetDepositCount(e.ctx)
	nonceBefore := e.k.GetDepositNonce(e.ctx, e.payer, assetID)
	totalsBefore := e.k.GetTotals(e.ctx, assetID)

	for _, height := range []int64{planHeight, planHeight + 1} {
		for _, tc := range []struct {
			name        string
			assetID     [32]byte
			beneficiary [32]byte
			amount      sdk.Int
		}{
			{name: "valid deposit", assetID: assetID, beneficiary: beneficiary, amount: amount},
			{name: "unknown asset", assetID: [32]byte{}, beneficiary: beneficiary, amount: amount},
			{name: "invalid deposit", assetID: assetID, amount: sdk.ZeroInt()},
		} {
			t.Run(fmt.Sprintf("height %d/%s", height, tc.name), func(t *testing.T) {
				ctx := e.ctx.WithBlockHeight(height).WithEventManager(sdk.NewEventManager()).WithGasMeter(sdk.NewGasMeter(1, 1, 1))
				deposit, err := e.k.Deposit(ctx, e.payer, e.payerAcc, tc.assetID, tc.beneficiary, tc.amount)
				require.ErrorIs(t, err, types.ErrModuleRetired)
				require.Contains(t, err.Error(), fmt.Sprintf("%s from height %d", types.MergePlanUpgrade, height))
				require.Equal(t, types.Deposit{}, deposit)
				require.Zero(t, ctx.GasMeter().GasConsumed())
				require.Equal(t, payerBefore, e.balance(e.payerAcc, denom))
				require.Equal(t, moduleBefore, e.balance(e.k.ModuleAddress(), denom))
				require.Equal(t, countBefore, e.k.GetDepositCount(e.ctx))
				require.Equal(t, nonceBefore, e.k.GetDepositNonce(e.ctx, e.payer, assetID))
				require.Equal(t, totalsBefore, e.k.GetTotals(e.ctx, assetID))
				require.Empty(t, ctx.EventManager().Events(), "no custody deposit event")
			})
		}
	}
}
