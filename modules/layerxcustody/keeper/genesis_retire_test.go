package keeper_test

import (
	"testing"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenesisRoundTripAfterRetiredRelease(t *testing.T) {
	e := newEnv(t, 600)
	params := e.k.GetParams(e.ctx)
	params.Authority = e.payerAcc.String()
	params.ForcedExitDelaySeconds = 600
	require.NoError(t, e.k.SetParams(e.ctx, params))
	e.deposit(e.withdrawal.Fields["asset"], 1_000)
	e.deposit(e.exit.Fields["asset"], 6_000_000)
	withdrawal, err := e.k.RequestWithdrawal(e.ctx, e.evidence())
	require.NoError(t, err)
	exitBatch := vectorNumber(t, e.withdrawal, "batch_number") + 1
	require.NoError(t, e.k.RegisterCheckpoint(e.ctx, exitBatch, vectorArray(t, e.exit, "state_root"), [32]byte{7}))
	require.NoError(t, e.k.SetEmergency(e.ctx, true))
	exit, err := e.k.RequestForcedExit(e.ctx, e.exitEvidence(exitBatch))
	require.NoError(t, err)

	claims := []types.Claim{withdrawal, exit}
	nullifiers := make(map[string]types.Nullifier)
	totals := make(map[string]types.AssetTotals)
	for _, claim := range claims {
		require.Equal(t, types.ClaimStatus_CLAIM_STATUS_PENDING, claim.Status)
		nullifierID, err := types.ParseHash32(claim.Nullifier)
		require.NoError(t, err)
		nullifier, found := e.k.GetNullifier(e.ctx, nullifierID)
		require.True(t, found)
		require.Equal(t, types.NullifierStatus_NULLIFIER_STATUS_RESERVED, nullifier.Status)
		nullifiers[claim.Nullifier] = nullifier
		assetID, err := types.ParseHash32(claim.AssetId)
		require.NoError(t, err)
		totals[claim.AssetId] = e.k.GetTotals(e.ctx, assetID)
	}
	e.solvent()
	scheduleCustodyRelease(t, e)
	require.NoError(t, e.k.ReleaseRetired(e.ctx))
	testkeeper.EVMTestApp.UpgradeKeeper.SetDone(e.ctx, types.MergePlanUpgrade)
	require.True(t, e.k.Retired(e.ctx))

	exported := e.k.ExportGenesis(e.ctx)
	require.NoError(t, exported.Validate())
	require.Len(t, exported.Claims, 2)
	require.Len(t, exported.Nullifiers, 2)
	require.Len(t, exported.Totals, 2)
	bz, err := testkeeper.EVMTestApp.AppCodec().MarshalAsJSON(exported)
	require.NoError(t, err)

	second := paxapp.Setup(t, false, true, false)
	fresh, _ := second.NewContext(false, tmtypes.Header{}).
		WithBlockHeight(e.ctx.BlockHeight()).WithBlockTime(e.ctx.BlockTime()).CacheContext()
	imported := second.LayerXCustodyKeeper
	require.Empty(t, imported.ExportGenesis(fresh).Claims)
	require.Empty(t, imported.ExportGenesis(fresh).Nullifiers)
	require.Empty(t, imported.ExportGenesis(fresh).Totals)
	var decoded types.GenesisState
	require.NoError(t, second.AppCodec().UnmarshalAsJSON(bz, &decoded))
	imported.InitGenesis(fresh, decoded)
	assert.Equal(t, exported, imported.ExportGenesis(fresh))

	for _, original := range claims {
		claimID, err := types.ParseHash32(original.ClaimId)
		require.NoError(t, err)
		claim, found := imported.GetClaim(fresh, claimID)
		require.True(t, found)
		original.Status = types.ClaimStatus_CLAIM_STATUS_CANCELLED
		assert.Equal(t, original, claim)
		sourceClaim, found := e.k.GetClaim(e.ctx, claimID)
		require.True(t, found)
		assert.Equal(t, sourceClaim, claim)

		nullifierID, err := types.ParseHash32(original.Nullifier)
		require.NoError(t, err)
		nullifier, found := imported.GetNullifier(fresh, nullifierID)
		require.True(t, found)
		expectedNullifier := nullifiers[original.Nullifier]
		expectedNullifier.Status = types.NullifierStatus_NULLIFIER_STATUS_CANCELLED
		assert.Equal(t, expectedNullifier, nullifier)
		sourceNullifier, found := e.k.GetNullifier(e.ctx, nullifierID)
		require.True(t, found)
		assert.Equal(t, sourceNullifier, nullifier)
		withdrawalID, err := types.ParseHash32(original.WithdrawalId)
		require.NoError(t, err)
		assert.True(t, imported.WithdrawalIDUsed(fresh, withdrawalID))

		assetID, err := types.ParseHash32(original.AssetId)
		require.NoError(t, err)
		before := totals[original.AssetId]
		custodied, ok := sdk.NewIntFromString(before.Custodied)
		require.True(t, ok)
		released, ok := sdk.NewIntFromString(before.Released)
		require.True(t, ok)
		assert.Equal(t, types.AssetTotals{AssetId: original.AssetId, Custodied: "0",
			Released: released.Add(custodied).String(), Pending: "0"}, imported.GetTotals(fresh, assetID))
		assert.Equal(t, e.k.GetTotals(e.ctx, assetID), imported.GetTotals(fresh, assetID))
		assert.True(t, second.BankKeeper.GetBalance(fresh, imported.ModuleAddress(), original.Denom).IsZero())
	}
	_, frozen := imported.ExitCheckpoint(fresh)
	assert.False(t, frozen)
	message, broken := keeper.SolvencyInvariant(imported)(fresh)
	assert.False(t, broken, message)
}
