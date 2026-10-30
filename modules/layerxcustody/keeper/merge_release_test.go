package keeper_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"math"
	"testing"

	anchorkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/keeper"
	anchortypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachemulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func prepareCustodyMerge(t *testing.T) (*env, types.Claim, upgradetypes.Plan) {
	t.Helper()
	e := newEnv(t, 600)
	app := testkeeper.EVMTestApp
	anchorBefore := app.LayerXAnchorKeeper
	t.Cleanup(func() { app.LayerXAnchorKeeper = anchorBefore })
	stores := make(map[storetypes.StoreKey]storetypes.CacheWrapper)
	keys := make(map[string]storetypes.StoreKey)
	for _, key := range e.ctx.MultiStore().StoreKeys() {
		stores[key] = e.ctx.MultiStore().GetKVStore(key)
		keys[key.Name()] = key
	}
	anchorKey := sdk.NewKVStoreKey(anchortypes.StoreKey)
	anchorDB := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, anchorDB.Close()) })
	stores[anchorKey] = dbadapter.Store{DB: anchorDB}
	keys[anchorKey.Name()] = anchorKey
	cacheDB := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, cacheDB.Close()) })
	e.ctx = e.ctx.WithMultiStore(cachemulti.NewStore(cacheDB, stores, keys, nil, nil, nil, 0))
	app.LayerXAnchorKeeper = anchorkeeper.NewKeeper(anchorKey, app.AccountKeeper, app.BankKeeper)
	app.LayerXAnchorKeeper.InitGenesis(e.ctx, *anchortypes.DefaultGenesis())
	authority, _ := testkeeper.MockAddressPair()
	params := e.k.GetParams(e.ctx)
	params.Authority = authority.String()
	require.NoError(t, e.k.SetParams(e.ctx, params))
	e.deposit(e.withdrawal.Fields["asset"], 1_000)
	e.deposit(e.exit.Fields["asset"], 2_000)
	claim, err := e.k.RequestWithdrawal(e.ctx, e.evidence())
	require.NoError(t, err)
	require.Equal(t, types.ClaimStatus_CLAIM_STATUS_PENDING, claim.Status)

	const height = int64(20)
	info, err := json.Marshal(map[string]any{
		"leaders": [][]byte{bytes.Repeat([]byte{0x51}, 32)},
		"receipt_keys": seqkeeper.ReceiptKeys{{
			KeyId:       bytes.Repeat([]byte{0x44}, seqtypes.KeyIDLength),
			PublicKey:   ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x11}, ed25519.SeedSize)).Public().(ed25519.PublicKey),
			FirstHeight: uint64(height),
			LastHeight:  math.MaxUint64,
		}},
	})
	require.NoError(t, err)
	plan := upgradetypes.Plan{Name: "v7.0", Height: height, Info: string(info)}
	require.True(t, app.UpgradeKeeper.HasHandler(plan.Name))
	require.Zero(t, app.UpgradeKeeper.GetDoneHeight(e.ctx, plan.Name))
	require.NoError(t, app.UpgradeKeeper.ScheduleUpgrade(e.ctx, plan))
	require.Less(t, e.ctx.BlockHeight(), plan.Height)
	require.False(t, e.k.Retired(e.ctx))
	return e, claim, plan
}

func TestMergePlanReleasesCustodyToTheAuthority(t *testing.T) {
	e, queued, plan := prepareCustodyMerge(t)
	app := testkeeper.EVMTestApp
	authority, err := sdk.AccAddressFromBech32(e.k.Authority(e.ctx))
	require.NoError(t, err)
	moduleBefore := app.BankKeeper.GetAllBalances(e.ctx, e.k.ModuleAddress())
	require.Equal(t, sdk.NewCoins(sdk.NewInt64Coin(sdk.MustGetBaseDenom(), 1_000),
		sdk.NewInt64Coin(tokenDenom, 2_000)), moduleBefore)
	authorityBefore := app.BankKeeper.GetAllBalances(e.ctx, authority)

	e.ctx = e.ctx.WithBlockHeight(plan.Height)
	app.UpgradeKeeper.ApplyUpgrade(e.ctx, plan)

	require.Equal(t, plan.Height, app.UpgradeKeeper.GetDoneHeight(e.ctx, plan.Name))
	require.True(t, e.k.Retired(e.ctx))
	require.Empty(t, app.BankKeeper.GetAllBalances(e.ctx, e.k.ModuleAddress()))
	require.Equal(t, authorityBefore.Add(moduleBefore...), app.BankKeeper.GetAllBalances(e.ctx, authority))
	claimID, err := types.ParseHash32(queued.ClaimId)
	require.NoError(t, err)
	claim, found := e.k.GetClaim(e.ctx, claimID)
	require.True(t, found)
	queued.Status = types.ClaimStatus_CLAIM_STATUS_CANCELLED
	require.Equal(t, queued, claim)
	nullifierID, err := types.ParseHash32(queued.Nullifier)
	require.NoError(t, err)
	nullifier, found := e.k.GetNullifier(e.ctx, nullifierID)
	require.True(t, found)
	require.Equal(t, types.NullifierStatus_NULLIFIER_STATUS_CANCELLED, nullifier.Status)
	require.Equal(t, queued.ClaimId, nullifier.ClaimId)

	events := custodyReleaseEvents(t, e.ctx)
	require.Len(t, events, 2)
	for _, asset := range []struct {
		id        string
		denom     string
		cancelled uint64
	}{
		{e.withdrawal.Fields["asset"], sdk.MustGetBaseDenom(), 1},
		{e.exit.Fields["asset"], tokenDenom, 0},
	} {
		require.Equal(t, &types.EventRetiredRelease{AssetId: asset.id, Recipient: authority.String(),
			Amount: moduleBefore.AmountOf(asset.denom).String(), Denom: asset.denom,
			CancelledClaims: asset.cancelled}, events[asset.id])
	}

	state := e.k.ExportGenesis(e.ctx)
	eventCount := len(e.ctx.EventManager().Events())
	require.ErrorIs(t, e.k.ReleaseRetired(e.ctx), types.ErrModuleRetired)
	require.Equal(t, state, e.k.ExportGenesis(e.ctx))
	require.Empty(t, app.BankKeeper.GetAllBalances(e.ctx, e.k.ModuleAddress()))
	require.Equal(t, authorityBefore.Add(moduleBefore...), app.BankKeeper.GetAllBalances(e.ctx, authority))
	require.Len(t, e.ctx.EventManager().Events(), eventCount)
}

func TestSolvencyInvariantHoldsAcrossTheMergePlan(t *testing.T) {
	e, queued, plan := prepareCustodyMerge(t)
	invariant := keeper.SolvencyInvariant(e.k)
	assetID := vectorArray(t, e.withdrawal, "asset")
	require.Equal(t, queued.Amount, e.k.GetTotals(e.ctx, assetID).Pending)
	message, broken := invariant(e.ctx)
	require.False(t, broken, message)

	e.ctx = e.ctx.WithBlockHeight(plan.Height)
	testkeeper.EVMTestApp.UpgradeKeeper.ApplyUpgrade(e.ctx, plan)

	require.Equal(t, plan.Height, testkeeper.EVMTestApp.UpgradeKeeper.GetDoneHeight(e.ctx, plan.Name))
	require.True(t, e.k.Retired(e.ctx))
	message, broken = invariant(e.ctx)
	require.False(t, broken, message)
	for _, asset := range []struct {
		id     [32]byte
		amount string
	}{
		{assetID, "1000"},
		{vectorArray(t, e.exit, "asset"), "2000"},
	} {
		require.Equal(t, types.AssetTotals{AssetId: types.Hash32(asset.id), Custodied: "0",
			Released: asset.amount, Pending: "0"}, e.k.GetTotals(e.ctx, asset.id))
	}
}
