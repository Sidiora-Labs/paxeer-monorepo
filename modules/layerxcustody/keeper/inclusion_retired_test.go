package keeper_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/testvectors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestWithdrawalInclusionAndExitBalanceRetired(t *testing.T) {
	e := newEnvWithoutCheckpoint(t, 0)
	require.False(t, e.k.Retired(e.ctx), "proof paths retire independently of the upgrade gate")
	e.ctx = e.ctx.WithEventManager(sdk.NewEventManager())
	moduleBefore := e.balance(e.k.ModuleAddress(), sdk.MustGetBaseDenom())
	for _, tc := range []struct {
		name     string
		evidence keeper.WithdrawalEvidence
	}{
		{name: "valid", evidence: e.evidence()},
		{name: "empty"},
		{name: "malformed", evidence: keeper.WithdrawalEvidence{
			Receipt: []byte{0xff}, Proof: []byte{0xff}, Header: []byte{0xff}, HeaderSignature: []byte{1}}},
	} {
		t.Run("withdrawal/"+tc.name, func(t *testing.T) {
			effect, header, err := e.k.VerifyWithdrawal(e.ctx, tc.evidence)
			require.ErrorIs(t, err, types.ErrModuleRetired)
			require.Nil(t, effect)
			require.Nil(t, header)
			claim, err := e.k.RequestWithdrawal(e.ctx, tc.evidence)
			require.ErrorIs(t, err, types.ErrModuleRetired)
			require.Zero(t, claim)
			result, err := e.k.FinaliseWithdrawal(e.ctx, tc.evidence)
			require.ErrorIs(t, err, types.ErrModuleRetired)
			require.Zero(t, result)
		})
	}
	for _, tc := range []struct {
		name     string
		evidence keeper.ExitEvidence
	}{
		{name: "valid", evidence: e.exitEvidence(vectorNumber(t, e.withdrawal, "batch_number"))},
		{name: "empty"},
		{name: "malformed", evidence: keeper.ExitEvidence{Witness: []byte{0xff}, RecipientSignature: []byte{1}}},
	} {
		t.Run("exit/"+tc.name, func(t *testing.T) {
			pending, err := e.k.VerifyForcedExit(e.ctx, tc.evidence)
			require.ErrorIs(t, err, types.ErrModuleRetired)
			require.Zero(t, pending)
			claim, err := e.k.RequestForcedExit(e.ctx, tc.evidence)
			require.ErrorIs(t, err, types.ErrModuleRetired)
			require.Zero(t, claim)
			result, err := e.k.ExecuteForcedExit(e.ctx, tc.evidence)
			require.ErrorIs(t, err, types.ErrModuleRetired)
			require.Zero(t, result)
		})
	}
	e.k.IterateClaims(e.ctx, func(types.Claim) bool {
		t.Error("retired proof path created a claim")
		return false
	})
	e.k.IterateNullifiers(e.ctx, func(types.Nullifier) bool {
		t.Error("retired proof path reserved a nullifier")
		return false
	})
	_, frozen := e.k.ExitCheckpoint(e.ctx)
	require.False(t, frozen)
	require.Equal(t, moduleBefore, e.balance(e.k.ModuleAddress(), sdk.MustGetBaseDenom()))
	require.Empty(t, e.ctx.EventManager().Events())
}

func TestWithdrawalNullifierUnchangedWithoutVerify(t *testing.T) {
	fixture, err := testvectors.Load()
	require.NoError(t, err)
	require.NotEmpty(t, fixture["withdrawal"])
	v := fixture["withdrawal"][0]
	require.True(t, v.Valid)
	t.Run("native withdrawal vector", func(t *testing.T) {
		digest := types.WithdrawalNullifier(uint32(vectorNumber(t, v, "network_id")),
			vectorArray(t, v, "withdrawal_id"), vectorArray(t, v, "account"), vectorArray(t, v, "asset"),
			codec.U128{Lo: vectorNumber(t, v, "amount")}, vectorArray(t, v, "anchor"))
		require.Equal(t, vectorArray(t, v, "nullifier"), digest)
	})
	var sequential [4][32]byte
	for i := range sequential {
		for j := range sequential[i] {
			sequential[i][j] = byte(i*32 + j)
		}
	}
	maximum := [32]byte(bytes.Repeat([]byte{0xff}, 32))
	for _, tc := range []struct {
		name    string
		network uint32
		ids     [4][32]byte
		amount  codec.U128
		want    string
	}{
		{name: "zero", want: "7b99d8f82df24c34cf164a24852b8a959b62885dd880103f90ded228088d3a5c"},
		{name: "byte order and high amount", network: 0x01020304, ids: sequential,
			amount: codec.U128{Hi: 0x0123456789abcdef, Lo: 0xfedcba9876543210},
			want:   "cce8b614cfd7f589c0a6143c2fe8b2a7ba8f9e52910cfcbb1bb917aa3ec530e5"},
		{name: "maximum", network: 0xffffffff, ids: [4][32]byte{maximum, maximum, maximum, maximum},
			amount: codec.U128{Hi: 0xffffffffffffffff, Lo: 0xffffffffffffffff},
			want:   "2abd885c11ad9453861f73091287e0b75db9360e84edad679adb7f200d0cb202"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want, err := hex.DecodeString(tc.want)
			require.NoError(t, err)
			digest := types.WithdrawalNullifier(tc.network, tc.ids[0], tc.ids[1], tc.ids[2], tc.amount, tc.ids[3])
			require.Equal(t, want, digest[:])
		})
	}
}
