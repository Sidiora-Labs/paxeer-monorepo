package keeper_test

import (
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
)

func scheduleCustodyRelease(t *testing.T, e *env) {
	t.Helper()
	require.NoError(t, testkeeper.EVMTestApp.UpgradeKeeper.ScheduleUpgrade(e.ctx,
		upgradetypes.Plan{Name: types.MergePlanUpgrade, Height: 20}))
	e.ctx = e.ctx.WithBlockHeight(20)
	require.False(t, e.k.Retired(e.ctx))
}

func custodyReleaseEvents(t *testing.T, ctx sdk.Context) map[string]*types.EventRetiredRelease {
	t.Helper()
	events := make(map[string]*types.EventRetiredRelease)
	for _, event := range ctx.EventManager().Events() {
		if event.Type != "paxprotocol.paxchain.layerxcustody.EventRetiredRelease" {
			continue
		}
		parsed, err := sdk.ParseTypedEvent(abci.Event(event))
		require.NoError(t, err)
		release, ok := parsed.(*types.EventRetiredRelease)
		require.True(t, ok)
		require.NotContains(t, events, release.AssetId)
		events[release.AssetId] = release
	}
	return events
}

func TestReleaseRetiredMovesEveryCustodiedBalanceToTheAuthority(t *testing.T) {
	e := newEnv(t, 0)
	app := testkeeper.EVMTestApp
	params := e.k.GetParams(e.ctx)
	params.Authority = e.payerAcc.String()
	require.NoError(t, e.k.SetParams(e.ctx, params))
	e.deposit(e.withdrawal.Fields["asset"], 1_000)
	e.deposit(e.exit.Fields["asset"], 2_000)
	paid, err := e.k.FinaliseWithdrawal(e.ctx, e.evidence())
	require.NoError(t, err)
	require.Equal(t, types.ClaimStatus_CLAIM_STATUS_PAID, paid.Claim.Status)
	donation := sdk.NewCoins(sdk.NewInt64Coin(tokenDenom, 17))
	require.NoError(t, app.BankKeeper.SendCoinsFromAccountToModule(e.ctx, e.payerAcc, types.ModuleName, donation))
	empty := types.AssetMapping{AssetId: types.Hash32([32]byte{0xff}), Denom: "uempty"}
	require.NoError(t, e.k.SetAsset(e.ctx, empty))
	assetID := vectorArray(t, e.exit, "asset")
	asset, found := e.k.GetAsset(e.ctx, assetID)
	require.True(t, found)
	asset.Enabled, asset.Paused = false, true
	require.NoError(t, e.k.SetAsset(e.ctx, asset))
	scheduleCustodyRelease(t, e)
	e.solvent()
	before := make(map[string]types.AssetTotals)
	balances := make(map[string]sdk.Int)
	authorityBefore := make(map[string]sdk.Int)
	e.k.IterateAssets(e.ctx, func(asset types.AssetMapping) bool {
		id, err := types.ParseHash32(asset.AssetId)
		require.NoError(t, err)
		before[asset.AssetId] = e.k.GetTotals(e.ctx, id)
		balances[asset.AssetId] = e.balance(e.k.ModuleAddress(), asset.Denom)
		authorityBefore[asset.AssetId] = e.balance(e.payerAcc, asset.Denom)
		return false
	})
	require.NoError(t, e.k.ReleaseRetired(e.ctx))
	events := custodyReleaseEvents(t, e.ctx)
	require.Len(t, events, 3)
	e.k.IterateAssets(e.ctx, func(asset types.AssetMapping) bool {
		id, err := types.ParseHash32(asset.AssetId)
		require.NoError(t, err)
		require.True(t, e.balance(e.k.ModuleAddress(), asset.Denom).IsZero())
		require.Equal(t, authorityBefore[asset.AssetId].Add(balances[asset.AssetId]), e.balance(e.payerAcc, asset.Denom))
		custodied, ok := sdk.NewIntFromString(before[asset.AssetId].Custodied)
		require.True(t, ok)
		released, ok := sdk.NewIntFromString(before[asset.AssetId].Released)
		require.True(t, ok)
		require.Equal(t, types.AssetTotals{AssetId: asset.AssetId, Custodied: "0",
			Released: released.Add(custodied).String(), Pending: "0"}, e.k.GetTotals(e.ctx, id))
		require.Equal(t, &types.EventRetiredRelease{AssetId: asset.AssetId, Recipient: params.Authority,
			Amount: balances[asset.AssetId].String(), Denom: asset.Denom}, events[asset.AssetId])
		return false
	})
	claimID, err := types.ParseHash32(paid.Claim.ClaimId)
	require.NoError(t, err)
	claim, found := e.k.GetClaim(e.ctx, claimID)
	require.True(t, found)
	require.Equal(t, paid.Claim, claim)
	nullifier, found := e.k.GetNullifier(e.ctx, vectorArray(t, e.withdrawal, "nullifier"))
	require.True(t, found)
	require.Equal(t, types.NullifierStatus_NULLIFIER_STATUS_CONSUMED, nullifier.Status)
	e.solvent()
	app.UpgradeKeeper.SetDone(e.ctx, types.MergePlanUpgrade)
	state := e.k.ExportGenesis(e.ctx)
	eventCount := len(e.ctx.EventManager().Events())
	err = e.k.ReleaseRetired(e.ctx)
	require.ErrorIs(t, err, types.ErrModuleRetired)
	require.ErrorContains(t, err, "already released")
	require.Equal(t, state, e.k.ExportGenesis(e.ctx))
	require.Len(t, e.ctx.EventManager().Events(), eventCount)
	e.solvent()
}

func TestReleaseRetiredCancelsPendingClaims(t *testing.T) {
	for _, failSend := range []bool{false, true} {
		name := "release"
		if failSend {
			name = "failed bank send rolls back"
		}
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, 600)
			app := testkeeper.EVMTestApp
			params := e.k.GetParams(e.ctx)
			params.Authority = e.payerAcc.String()
			params.ForcedExitDelaySeconds = 600
			if failSend {
				params.Authority = authtypes.NewModuleAddress("evm").String()
				address, err := sdk.AccAddressFromBech32(params.Authority)
				require.NoError(t, err)
				require.True(t, app.BankKeeper.BlockedAddr(address))
			}
			require.NoError(t, e.k.SetParams(e.ctx, params))
			e.deposit(e.withdrawal.Fields["asset"], 1_000)
			e.deposit(e.exit.Fields["asset"], 6_000_000)
			withdrawal, err := e.k.RequestWithdrawal(e.ctx, e.evidence())
			require.NoError(t, err)
			batch := vectorNumber(t, e.withdrawal, "batch_number") + 1
			require.NoError(t, e.k.RegisterCheckpoint(e.ctx, batch, vectorArray(t, e.exit, "state_root"), [32]byte{7}))
			require.NoError(t, e.k.SetEmergency(e.ctx, true))
			exit, err := e.k.RequestForcedExit(e.ctx, e.exitEvidence(batch))
			require.NoError(t, err)
			scheduleCustodyRelease(t, e)
			e.solvent()
			state := e.k.ExportGenesis(e.ctx)
			eventCount := len(e.ctx.EventManager().Events())
			baseBalance := e.balance(e.k.ModuleAddress(), sdk.MustGetBaseDenom())
			tokenBalance := e.balance(e.k.ModuleAddress(), tokenDenom)
			err = e.k.ReleaseRetired(e.ctx)
			if failSend {
				require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
				require.Equal(t, state, e.k.ExportGenesis(e.ctx))
				require.Len(t, e.ctx.EventManager().Events(), eventCount)
				require.Equal(t, baseBalance, e.balance(e.k.ModuleAddress(), sdk.MustGetBaseDenom()))
				require.Equal(t, tokenBalance, e.balance(e.k.ModuleAddress(), tokenDenom))
				e.solvent()
				return
			}
			require.NoError(t, err)
			for _, original := range []types.Claim{withdrawal, exit} {
				id, err := types.ParseHash32(original.ClaimId)
				require.NoError(t, err)
				claim, found := e.k.GetClaim(e.ctx, id)
				require.True(t, found)
				original.Status = types.ClaimStatus_CLAIM_STATUS_CANCELLED
				require.Equal(t, original, claim)
				nullifierID, err := types.ParseHash32(claim.Nullifier)
				require.NoError(t, err)
				nullifier, found := e.k.GetNullifier(e.ctx, nullifierID)
				require.True(t, found)
				require.Equal(t, types.NullifierStatus_NULLIFIER_STATUS_CANCELLED, nullifier.Status)
				require.Equal(t, claim.ClaimId, nullifier.ClaimId)
				withdrawalID, err := types.ParseHash32(claim.WithdrawalId)
				require.NoError(t, err)
				require.True(t, e.k.WithdrawalIDUsed(e.ctx, withdrawalID))
				assetID, err := types.ParseHash32(claim.AssetId)
				require.NoError(t, err)
				totals := e.k.GetTotals(e.ctx, assetID)
				require.Equal(t, "0", totals.Pending)
				require.Equal(t, "0", totals.Custodied)
				require.True(t, e.balance(e.k.ModuleAddress(), claim.Denom).IsZero())
			}
			_, frozen := e.k.ExitCheckpoint(e.ctx)
			require.False(t, frozen)
			events := custodyReleaseEvents(t, e.ctx)
			require.Len(t, events, 2)
			for _, event := range events {
				require.Equal(t, uint64(1), event.CancelledClaims)
			}
			e.solvent()
		})
	}
}

func TestReleaseRetiredIsRefusedBeforeTheMergePlanHeight(t *testing.T) {
	for _, name := range []string{"no plan", "before height", "after height", "different plan", "already done", "done without plan", "no upgrade reader"} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, 600)
			app := testkeeper.EVMTestApp
			params := e.k.GetParams(e.ctx)
			params.Authority = e.payerAcc.String()
			require.NoError(t, e.k.SetParams(e.ctx, params))
			e.deposit(e.withdrawal.Fields["asset"], 1_000)
			e.deposit(e.exit.Fields["asset"], 2_000)
			_, err := e.k.RequestWithdrawal(e.ctx, e.evidence())
			require.NoError(t, err)
			plan := upgradetypes.Plan{Name: types.MergePlanUpgrade, Height: 20}
			if name == "different plan" {
				plan.Name = "another-upgrade"
			}
			if name != "no plan" && name != "no upgrade reader" {
				require.NoError(t, app.UpgradeKeeper.ScheduleUpgrade(e.ctx, plan))
			}
			e.ctx = e.ctx.WithBlockHeight(20)
			switch name {
			case "before height":
				e.ctx = e.ctx.WithBlockHeight(19)
			case "after height":
				e.ctx = e.ctx.WithBlockHeight(21)
			case "already done", "done without plan":
				app.UpgradeKeeper.SetDone(e.ctx, types.MergePlanUpgrade)
				if name == "done without plan" {
					app.UpgradeKeeper.ClearUpgradePlan(e.ctx)
				}
			case "no upgrade reader":
				e.k = keeper.NewKeeper(app.AppCodec(), app.GetKey(types.StoreKey),
					app.AccountKeeper, app.BankKeeper, &app.EvmKeeper, nil)
			}
			e.solvent()
			state := e.k.ExportGenesis(e.ctx)
			eventCount := len(e.ctx.EventManager().Events())
			baseBalance := e.balance(e.k.ModuleAddress(), sdk.MustGetBaseDenom())
			tokenBalance := e.balance(e.k.ModuleAddress(), tokenDenom)
			authorityBase := e.balance(e.payerAcc, sdk.MustGetBaseDenom())
			authorityToken := e.balance(e.payerAcc, tokenDenom)
			err = e.k.ReleaseRetired(e.ctx)
			require.ErrorIs(t, err, types.ErrModuleRetired)
			if name == "already done" || name == "done without plan" {
				require.ErrorContains(t, err, "already released")
			} else {
				require.ErrorContains(t, err, "plan not running")
			}
			require.Equal(t, state, e.k.ExportGenesis(e.ctx))
			require.Len(t, e.ctx.EventManager().Events(), eventCount)
			require.Equal(t, baseBalance, e.balance(e.k.ModuleAddress(), sdk.MustGetBaseDenom()))
			require.Equal(t, tokenBalance, e.balance(e.k.ModuleAddress(), tokenDenom))
			require.Equal(t, authorityBase, e.balance(e.payerAcc, sdk.MustGetBaseDenom()))
			require.Equal(t, authorityToken, e.balance(e.payerAcc, tokenDenom))
			e.solvent()
		})
	}
}
