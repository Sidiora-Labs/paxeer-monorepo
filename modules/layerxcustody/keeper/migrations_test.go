package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
)

func TestMigrate1to2SeedsExitCheckpoint(t *testing.T) {
	e := newEnv(t, 600)
	e.deposit(e.withdrawal.Fields["asset"], 1_000)
	e.deposit(e.exit.Fields["asset"], 6_000_000)
	_, err := e.k.RequestWithdrawal(e.ctx, e.evidence())
	require.NoError(t, err)

	params := e.k.GetParams(e.ctx)
	params.ForcedExitDelaySeconds = 120
	require.NoError(t, e.k.SetParams(e.ctx, params))
	exitBatch := vectorNumber(t, e.withdrawal, "batch_number") + 1
	require.NoError(t, e.k.RegisterCheckpoint(e.ctx, exitBatch, vectorArray(t, e.exit, "state_root"), [32]byte{7}))
	require.NoError(t, e.k.SetEmergency(e.ctx, true))
	exit, err := e.k.RequestForcedExit(e.ctx, e.exitEvidence(exitBatch))
	require.NoError(t, err)
	require.Equal(t, types.ClaimStatus_CLAIM_STATUS_PENDING, exit.Status)
	require.Equal(t, exitBatch, exit.BatchNumber)

	store := e.ctx.KVStore(testkeeper.EVMTestApp.GetKey(types.StoreKey))
	require.True(t, store.Has(types.ExitCheckpointKey))
	store.Delete(types.ExitCheckpointKey)
	_, frozen := e.k.ExitCheckpoint(e.ctx)
	require.False(t, frozen)

	require.NoError(t, keeper.NewMigrator(e.k).Migrate1to2(e.ctx))
	frozenAt, frozen := e.k.ExitCheckpoint(e.ctx)
	require.True(t, frozen)
	require.Equal(t, exitBatch, frozenAt)

	assetID := vectorArray(t, e.exit, "asset")
	_, err = e.k.Deposit(e.ctx, e.payer, e.payerAcc, assetID, [32]byte{0xbe, 0xef}, sdk.NewInt(1_000))
	require.ErrorIs(t, err, types.ErrDepositFrozen)
	require.Equal(t, sdk.NewInt(6_000_000), e.balance(e.k.ModuleAddress(), tokenDenom))
}
