package app

import (
	"testing"
	"time"

	anchortypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	custodytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/stretchr/testify/require"
)

func TestMergePlanReturnsGuarantorBonds(t *testing.T) {
	for _, name := range []string{"refunds and custody release", "bond return fails", "custody release fails"} {
		t.Run(name, func(t *testing.T) {
			w := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), true)
			a, ctx := w.App, w.Ctx.WithBlockHeight(mergeHeight)
			t.Cleanup(func() { require.NoError(t, a.Close()) })
			rewindBeforeMerge(t, a, ctx)
			params := a.LayerXAnchorKeeper.GetParams(ctx)
			operators := make([]sdk.AccAddress, 3)
			balances := make([]sdk.Int, len(operators))
			bonds := make([]sdk.Int, len(operators))
			total := sdk.ZeroInt()
			for i := range operators {
				operators[i] = sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
				bonds[i] = params.MinBond.AddRaw(int64(i + 1))
				funding := sdk.NewCoins(sdk.NewCoin(params.BondDenom, bonds[i].MulRaw(2)))
				require.NoError(t, a.BankKeeper.MintCoins(ctx, "evm", funding))
				require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "evm", operators[i], funding))
				_, err := a.LayerXAnchorKeeper.RegisterGuarantor(ctx, operators[i], [32]byte{byte(i + 1)},
					[20]byte{byte(i + 1)}, bonds[i])
				require.NoError(t, err)
				balances[i] = a.BankKeeper.GetBalance(ctx, operators[i], params.BondDenom).Amount
				total = total.Add(bonds[i])
			}
			_, err := a.LayerXAnchorKeeper.BeginUnbond(ctx, operators[0], [32]byte{1}, sdk.OneInt())
			require.NoError(t, err)
			require.Len(t, a.LayerXAnchorKeeper.GetUnbondings(ctx), 1)
			require.Equal(t, total, a.LayerXAnchorKeeper.ModuleBalance(ctx))
			if name == "bond return fails" {
				require.NoError(t, a.BankKeeper.SendCoinsFromAccountToModule(ctx, operators[0], anchortypes.ModuleName,
					sdk.NewCoins(sdk.NewInt64Coin(params.BondDenom, 1))))
			}
			authority := operators[1]
			custodyParams := a.LayerXCustodyKeeper.GetParams(ctx)
			custodyParams.Authority = authority.String()
			require.NoError(t, a.LayerXCustodyKeeper.SetParams(ctx, custodyParams))
			asset := custodytypes.AssetMapping{AssetId: custodytypes.Hash32([32]byte{1}), Denom: "umergeretired"}
			require.NoError(t, a.LayerXCustodyKeeper.SetAsset(ctx, asset))
			custodyCoins := sdk.NewCoins(sdk.NewInt64Coin(asset.Denom, 73))
			require.NoError(t, a.BankKeeper.MintCoins(ctx, "evm", custodyCoins))
			require.NoError(t, a.BankKeeper.SendCoinsFromModuleToModule(ctx, "evm", custodytypes.ModuleName, custodyCoins))
			supply := a.BankKeeper.GetSupply(ctx, params.BondDenom)
			plan := upgradetypes.Plan{Name: MergeUpgrade, Height: mergeHeight, Info: activationInfo(t)}
			if name != "custody release fails" {
				require.NoError(t, a.UpgradeKeeper.ScheduleUpgrade(ctx, plan))
			}
			before := a.LayerXAnchorKeeper.ExportGenesis(ctx)
			anchorBalance := a.LayerXAnchorKeeper.ModuleBalance(ctx)
			ctx = ctx.WithEventManager(sdk.NewEventManager())
			_, err = a.continueMergeUpgradePlan(ctx, plan, a.UpgradeKeeper.GetModuleVersionMap(ctx))
			if name == "bond return fails" {
				require.ErrorIs(t, err, anchortypes.ErrBond)
				require.ErrorContains(t, err, "retire anchor")
				require.Equal(t, before, a.LayerXAnchorKeeper.ExportGenesis(ctx))
				require.Equal(t, anchorBalance, a.LayerXAnchorKeeper.ModuleBalance(ctx))
				state, exportErr := a.SequencerKeeper.ExportGenesis(ctx)
				require.NoError(t, exportErr)
				require.Empty(t, state.Leaders)
				require.Zero(t, a.UpgradeKeeper.GetDoneHeight(ctx, MergeUpgrade))
				return
			}
			if name == "custody release fails" {
				require.ErrorIs(t, err, custodytypes.ErrModuleRetired)
				require.ErrorContains(t, err, "release custody")
				require.Equal(t, before, a.LayerXAnchorKeeper.ExportGenesis(ctx))
				require.Equal(t, anchorBalance, a.LayerXAnchorKeeper.ModuleBalance(ctx))
				require.Equal(t, custodyCoins, a.BankKeeper.GetAllBalances(ctx, a.LayerXCustodyKeeper.ModuleAddress()))
				state, exportErr := a.SequencerKeeper.ExportGenesis(ctx)
				require.NoError(t, exportErr)
				require.Empty(t, state.Leaders)
				require.False(t, a.Sequenced(ctx, mergeHeight+1))
				require.Zero(t, a.UpgradeKeeper.GetDoneHeight(ctx, MergeUpgrade))
				return
			}
			require.NoError(t, err)
			for i, operator := range operators {
				require.Equal(t, balances[i].Add(bonds[i]), a.BankKeeper.GetBalance(ctx, operator, params.BondDenom).Amount)
			}
			for _, guarantor := range a.LayerXAnchorKeeper.GetGuarantors(ctx) {
				require.True(t, guarantor.Bond.IsZero())
			}
			require.Empty(t, a.LayerXAnchorKeeper.GetUnbondings(ctx))
			require.True(t, a.LayerXAnchorKeeper.ModuleBalance(ctx).IsZero())
			require.Equal(t, supply, a.BankKeeper.GetSupply(ctx, params.BondDenom))
			require.Empty(t, a.BankKeeper.GetAllBalances(ctx, a.LayerXCustodyKeeper.ModuleAddress()))
			require.Equal(t, custodyCoins[0], a.BankKeeper.GetBalance(ctx, authority, asset.Denom))
			require.Contains(t, ctx.EventManager().Events(), sdk.NewEvent(anchortypes.EventAnchorRetired,
				sdk.NewAttribute(anchortypes.AttributeReturned, total.String())))
			require.True(t, a.Sequenced(ctx, mergeHeight+1))
		})
	}
}
