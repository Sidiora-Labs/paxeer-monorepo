package keeper_test

import (
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
)

func TestWithdrawalIsRefusedFromTheMergePlanHeight(t *testing.T) {
	e := newEnv(t, 0)
	e.deposit(e.withdrawal.Fields["asset"], 1_000)
	evidence := e.evidence()
	claim, err := e.k.RequestWithdrawal(e.ctx, evidence)
	require.NoError(t, err)
	require.Equal(t, types.ClaimStatus_CLAIM_STATUS_PENDING, claim.Status)
	claimID, err := types.ParseHash32(claim.ClaimId)
	require.NoError(t, err)
	moduleBefore := e.balance(e.k.ModuleAddress(), sdk.MustGetBaseDenom())
	recipientBefore := e.balance(e.recipient(), sdk.MustGetBaseDenom())

	testkeeper.EVMTestApp.UpgradeKeeper.SetDone(e.ctx.WithBlockHeight(20), types.MergePlanUpgrade)
	for _, height := range []int64{20, 21} {
		t.Run(fmt.Sprintf("height %d", height), func(t *testing.T) {
			ctx := e.ctx.WithBlockHeight(height).WithEventManager(sdk.NewEventManager())
			for _, evidence := range []keeper.WithdrawalEvidence{evidence, {}} {
				_, err := e.k.RequestWithdrawal(ctx, evidence)
				require.ErrorIs(t, err, types.ErrModuleRetired)
				require.Contains(t, err.Error(), fmt.Sprintf("%s from height %d", types.MergePlanUpgrade, height))
				_, err = e.k.FinaliseWithdrawal(ctx, evidence)
				require.ErrorIs(t, err, types.ErrModuleRetired)
			}
			require.Equal(t, moduleBefore, e.balance(e.k.ModuleAddress(), sdk.MustGetBaseDenom()))
			require.Equal(t, recipientBefore, e.balance(e.recipient(), sdk.MustGetBaseDenom()))
			stored, found := e.k.GetClaim(ctx, claimID)
			require.True(t, found)
			require.Equal(t, claim, stored)
			require.Empty(t, ctx.EventManager().Events(), "no claim queued, claim finalised or custody release event")
		})
	}
}

func TestForcedExitIsRefusedFromTheMergePlanHeight(t *testing.T) {
	e := newEnv(t, 0)
	e.deposit(e.exit.Fields["asset"], 6_000_000)
	params := e.k.GetParams(e.ctx)
	params.ForcedExitDelaySeconds = 0
	require.NoError(t, e.k.SetParams(e.ctx, params))
	exitBatch := vectorNumber(t, e.withdrawal, "batch_number") + 1
	require.NoError(t, e.k.RegisterCheckpoint(e.ctx, exitBatch, vectorArray(t, e.exit, "state_root"), [32]byte{7}))
	require.NoError(t, e.k.SetEmergency(e.ctx, true))
	evidence := e.exitEvidence(exitBatch)
	claim, err := e.k.RequestForcedExit(e.ctx, evidence)
	require.NoError(t, err)
	require.Equal(t, types.ClaimStatus_CLAIM_STATUS_PENDING, claim.Status)
	claimID, err := types.ParseHash32(claim.ClaimId)
	require.NoError(t, err)
	recipient := testkeeper.EVMTestApp.EvmKeeper.GetPaxAddressOrDefault(e.ctx, evidence.Recipient)
	moduleBefore := e.balance(e.k.ModuleAddress(), tokenDenom)
	recipientBefore := e.balance(recipient, tokenDenom)

	testkeeper.EVMTestApp.UpgradeKeeper.SetDone(e.ctx.WithBlockHeight(20), types.MergePlanUpgrade)
	for _, height := range []int64{20, 21} {
		t.Run(fmt.Sprintf("height %d", height), func(t *testing.T) {
			ctx := e.ctx.WithBlockHeight(height).WithEventManager(sdk.NewEventManager())
			for _, evidence := range []keeper.ExitEvidence{evidence, {}} {
				_, err := e.k.RequestForcedExit(ctx, evidence)
				require.ErrorIs(t, err, types.ErrModuleRetired)
				_, err = e.k.ExecuteForcedExit(ctx, evidence)
				require.ErrorIs(t, err, types.ErrModuleRetired)
			}
			require.Equal(t, moduleBefore, e.balance(e.k.ModuleAddress(), tokenDenom))
			require.Equal(t, recipientBefore, e.balance(recipient, tokenDenom))
			stored, found := e.k.GetClaim(ctx, claimID)
			require.True(t, found)
			require.Equal(t, claim, stored)
			require.Empty(t, ctx.EventManager().Events(), "no forced exit executed or custody release event")
		})
	}
}

func TestCancelClaimIsRefusedFromTheMergePlanHeight(t *testing.T) {
	e := newEnv(t, 600)
	e.deposit(e.withdrawal.Fields["asset"], 1_000)
	claim, err := e.k.RequestWithdrawal(e.ctx, e.evidence())
	require.NoError(t, err)
	require.Equal(t, types.ClaimStatus_CLAIM_STATUS_PENDING, claim.Status)
	claimID, err := types.ParseHash32(claim.ClaimId)
	require.NoError(t, err)
	nullifier := vectorArray(t, e.withdrawal, "nullifier")
	reserved, found := e.k.GetNullifier(e.ctx, nullifier)
	require.True(t, found)
	require.Equal(t, types.NullifierStatus_NULLIFIER_STATUS_RESERVED, reserved.Status)

	testkeeper.EVMTestApp.UpgradeKeeper.SetDone(e.ctx.WithBlockHeight(20), types.MergePlanUpgrade)
	for _, height := range []int64{20, 21} {
		t.Run(fmt.Sprintf("height %d", height), func(t *testing.T) {
			ctx := e.ctx.WithBlockHeight(height).WithEventManager(sdk.NewEventManager())
			for _, id := range [][32]byte{claimID, {}} {
				_, err := e.k.CancelClaim(ctx, id)
				require.ErrorIs(t, err, types.ErrModuleRetired)
			}
			stored, found := e.k.GetClaim(ctx, claimID)
			require.True(t, found)
			require.Equal(t, types.ClaimStatus_CLAIM_STATUS_PENDING, stored.Status)
			require.Equal(t, claim, stored)
			record, found := e.k.GetNullifier(ctx, nullifier)
			require.True(t, found)
			require.Equal(t, reserved, record)
			require.Empty(t, ctx.EventManager().Events(), "no claim cancelled event")
		})
	}
}
