package keeper_test

import (
	"encoding/json"
	"testing"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
)

func TestRepair_338_GovernanceMerge(t *testing.T) {
	for _, authority := range []string{"", authtypes.NewModuleAddress(govtypes.ModuleName).String()} {
		t.Run("authority="+authority, func(t *testing.T) {
			e, queued, plan := prepareCustodyMerge(t)
			params := e.k.GetParams(e.ctx)
			params.Authority = authority
			require.NoError(t, e.k.SetParams(e.ctx, params))
			gov := authtypes.NewModuleAddress(govtypes.ModuleName)
			require.Equal(t, gov.String(), e.k.Authority(e.ctx))
			require.True(t, testkeeper.EVMTestApp.BankKeeper.BlockedAddr(gov))
			assertCustodyMergeRelease(t, e, queued, plan)
			// The same blocked-recipient protection still applies to ordinary
			// module-to-account transfers after the governed release succeeds.
			require.ErrorIs(t, testkeeper.EVMTestApp.BankKeeper.SendCoinsFromModuleToAccount(
				e.ctx, types.ModuleName, gov, sdk.NewCoins(sdk.NewInt64Coin(tokenDenom, 1))), sdkerrors.ErrUnauthorized)
		})
	}
}

func TestRepair_338_ExternalAuthorityAndBlockedModule(t *testing.T) {
	t.Run("external authority", TestMergePlanReleasesCustodyToTheAuthority)
	t.Run("blocked module and atomic rollback", TestReleaseRetiredCancelsPendingClaims)
}

func TestRepair_338_RetirementRecovery(t *testing.T) {
	e, queued, plan := prepareCustodyMerge(t)
	assertCustodyMergeRelease(t, e, queued, plan)
	exported, err := e.k.ExportGenesisWithRetirement(e.ctx)
	require.NoError(t, err)
	require.Equal(t, plan.Height, exported.RetiredAt)
	wire, err := json.Marshal(exported)
	require.NoError(t, err)

	// Reopen a fresh real application twice. No upgrade done-height store is
	// carried across, and the second import deliberately resets block height.
	for _, height := range []int64{plan.Height + 1, 0} {
		app := paxapp.Setup(t, false, true, false)
		ctx, _ := app.NewContext(false, tmtypes.Header{}).
			WithBlockHeight(height).WithBlockTime(e.ctx.BlockTime()).CacheContext()
		k := app.LayerXCustodyKeeper
		require.Zero(t, app.UpgradeKeeper.GetDoneHeight(ctx, types.MergePlanUpgrade))
		require.False(t, k.Retired(ctx))
		var recovered keeper.RetirementGenesis
		require.NoError(t, json.Unmarshal(wire, &recovered))
		require.NoError(t, k.InitGenesisWithRetirement(ctx, recovered))
		require.True(t, k.Retired(ctx))
		got, err := k.ExportGenesisWithRetirement(ctx)
		require.NoError(t, err)
		require.Equal(t, exported, got)
		withoutUpgrade := keeper.NewKeeper(app.AppCodec(), app.GetKey(types.StoreKey),
			app.AccountKeeper, app.BankKeeper, &app.EvmKeeper, nil)
		require.True(t, withoutUpgrade.Retired(ctx))
		for _, replacementHeight := range []int64{0, -1, plan.Height + 1} {
			invalid := recovered
			invalid.RetiredAt = replacementHeight
			require.ErrorIs(t, k.InitGenesisWithRetirement(ctx, invalid), types.ErrInvalidGenesis)
			require.True(t, k.Retired(ctx))
			unchanged, err := k.ExportGenesisWithRetirement(ctx)
			require.NoError(t, err)
			require.Equal(t, got, unchanged)
		}

		assetID := vectorArray(t, e.withdrawal, "asset")
		asset, found := k.GetAsset(ctx, assetID)
		require.True(t, found)
		pointerAsset, found := k.GetAssetByPointer(ctx, tokenPointer)
		require.True(t, found)
		require.Equal(t, tokenDenom, pointerAsset.Denom)
		for _, call := range []func(sdk.Context) error{
			func(ctx sdk.Context) error {
				_, err := k.Deposit(ctx, e.payer, e.payerAcc, assetID, [32]byte{1}, sdk.NewInt(10))
				return err
			},
			func(ctx sdk.Context) error { return k.SetParams(ctx, recovered.State.Params) },
			func(ctx sdk.Context) error { return k.SetAsset(ctx, asset) },
			func(ctx sdk.Context) error { return k.SetEmergency(ctx, true) },
			func(ctx sdk.Context) error { return k.ReleaseRetired(ctx) },
		} {
			callCtx := ctx.WithGasMeter(sdk.NewGasMeter(1, 1, 1)).WithEventManager(sdk.NewEventManager())
			require.ErrorIs(t, call(callCtx), types.ErrModuleRetired)
			require.Zero(t, callCtx.GasMeter().GasConsumed())
			require.Empty(t, callCtx.EventManager().Events())
		}
		require.Equal(t, recovered.State, *k.ExportGenesis(ctx))
		// Reinitialization of historical records also works when retirement
		// has already been restored, without reopening runtime setters.
		require.NotPanics(t, func() { k.InitGenesis(ctx, recovered.State) })
		require.True(t, k.Retired(ctx))
		got, err = k.ExportGenesisWithRetirement(ctx)
		require.NoError(t, err)
		wire, err = json.Marshal(got)
		require.NoError(t, err)
		require.NoError(t, app.Close())
	}
}

func TestRepair_338_RestoredUpgradeBeforeHistoricalGenesis(t *testing.T) {
	TestGenesisRoundTripAfterRetiredRelease(t)
}

func TestRepair_338_InvalidLifecycleIsAtomic(t *testing.T) {
	e := newEnv(t, 600)
	e.deposit(e.withdrawal.Fields["asset"], 1_000)
	live, err := e.k.ExportGenesisWithRetirement(e.ctx)
	require.NoError(t, err)
	require.Zero(t, live.RetiredAt)
	require.ErrorIs(t, e.k.InitGenesisWithRetirement(e.ctx,
		keeper.RetirementGenesis{State: live.State, RetiredAt: 20}), types.ErrInvalidGenesis)
	require.Equal(t, live.State, *e.k.ExportGenesis(e.ctx))
	e.seedClaim(types.ClaimKind_CLAIM_KIND_WITHDRAWAL, vectorNumber(t, e.withdrawal, "batch_number"))
	before := e.k.ExportGenesis(e.ctx)
	for _, height := range []int64{-1, 20} {
		state := keeper.RetirementGenesis{State: *before, RetiredAt: height}
		require.ErrorIs(t, e.k.InitGenesisWithRetirement(e.ctx, state), types.ErrInvalidGenesis)
		require.False(t, e.k.Retired(e.ctx))
		require.Equal(t, before, e.k.ExportGenesis(e.ctx))
	}
}
