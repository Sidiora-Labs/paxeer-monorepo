package layerxanchor_test

import (
	"fmt"
	"math/big"
	"testing"

	anchortypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxanchor"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

type retiredWrapperKeepers struct {
	utils.Keepers
	anchor utils.AnchorKeeper
}

func (k retiredWrapperKeepers) AnchorK() utils.AnchorKeeper { return k.anchor }

func TestAnchorRetirementSurvivesTheWrapper(t *testing.T) {
	h := newHarness(t, nil)
	var err error
	h.precompile, err = layerxanchor.NewPrecompile(retiredWrapperKeepers{
		Keepers: h.app.GetPrecompileKeepers(),
		anchor:  h.anchor,
	})
	require.NoError(t, err)
	const mergeHeight, deleteHeight = int64(20), int64(40)
	h.app.UpgradeKeeper.SetDone(h.ctx().WithBlockHeight(mergeHeight), pcommon.MergeUpgrade)
	h.app.UpgradeKeeper.SetDone(h.ctx().WithBlockHeight(deleteHeight), layerxanchor.StoreDeleteUpgrade)
	h.stateDB.WithCtx(h.ctx().WithBlockHeight(deleteHeight + 1))
	require.Equal(t, mergeHeight, h.app.UpgradeKeeper.GetDoneHeight(h.ctx(), pcommon.MergeUpgrade))
	require.Equal(t, deleteHeight, h.app.UpgradeKeeper.GetDoneHeight(h.ctx(), layerxanchor.StoreDeleteUpgrade))

	want := pcommon.RetiredRevert(layerxanchor.PrecompileName)
	require.Len(t, want, 132)
	check := func(name string, args []interface{}) {
		t.Helper()
		input := h.input(name, args...)
		for _, readOnly := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/readOnly=%t", name, readOnly), func(t *testing.T) {
				data, err := h.precompile.Run(h.evm, h.reporter.address, h.reporter.address, input, nil, readOnly, false, nil)
				require.Equal(t, vm.ErrExecutionReverted, err)
				require.Equal(t, want, data)
				require.Len(t, data, 132)
				reason, err := abi.UnpackRevert(data)
				require.NoError(t, err)
				require.Equal(t, "LXP_MODULE_DISABLED: module disabled; layerxAnchor retired", reason)
			})
		}
	}
	writes := map[string][]interface{}{
		layerxanchor.SubmitCheckpointMethod:          h.checkpointArgs("batch_1_quorum"),
		layerxanchor.SubmitAvailabilityMethod:        {h.attestation("batch_1_attestation_1")},
		layerxanchor.FinalizeMethod:                  {uint64(1)},
		layerxanchor.RegisterGuarantorMethod:         {h.ids[0], h.signers[0]},
		layerxanchor.IncreaseBondMethod:              {h.ids[0]},
		layerxanchor.BeginUnbondMethod:               {h.ids[0], big.NewInt(bond / 2)},
		layerxanchor.CompleteUnbondMethod:            {h.ids[0]},
		layerxanchor.SubmitEquivocationMethod:        {[]byte{5}, []byte{6}},
		layerxanchor.OpenChallengeMethod:             {uint64(1), anchortypes.ChallengeFraud, [32]byte{1}},
		layerxanchor.ResolveChallengeMethod:          {uint64(1), false},
		layerxanchor.ActivateGuarantorMethod:         {h.ids[0]},
		layerxanchor.SetSequencerAuthorizationMethod: {[32]byte{1}, [32]byte{2}, uint64(1), uint64(8)},
	}
	writeCount := 0
	for name, method := range h.precompile.GetABI().Methods {
		if method.StateMutability == "view" || method.StateMutability == "pure" {
			continue
		}
		args, ok := writes[name]
		require.True(t, ok, "every ABI write must be exercised: %s", name)
		check(name, args)
		writeCount++
	}
	require.Equal(t, 12, writeCount)
	checkpointID, err := h.vector("checkpoints", "batch_1_quorum").Array32("checkpoint_id")
	require.NoError(t, err)
	reads := anchorRetirementReads(h, checkpointID)
	viewCount := 0
	for name, method := range h.precompile.GetABI().Methods {
		if method.StateMutability != "view" && method.StateMutability != "pure" {
			continue
		}
		args, ok := reads[name]
		require.True(t, ok, "every ABI view must be exercised: %s", name)
		check(name, args)
		viewCount++
	}
	require.Equal(t, 9, viewCount)
}

func TestAnchorRetiredWriteMalformedCalldata(t *testing.T) {
	h := newHarness(t, nil)
	var err error
	h.precompile, err = layerxanchor.NewPrecompile(retiredWrapperKeepers{
		Keepers: h.app.GetPrecompileKeepers(),
		anchor:  h.anchor,
	})
	require.NoError(t, err)
	const mergeHeight, deleteHeight = int64(20), int64(40)
	h.app.UpgradeKeeper.SetDone(h.ctx().WithBlockHeight(mergeHeight), pcommon.MergeUpgrade)
	h.app.UpgradeKeeper.SetDone(h.ctx().WithBlockHeight(deleteHeight), layerxanchor.StoreDeleteUpgrade)
	h.stateDB.WithCtx(h.ctx().WithBlockHeight(deleteHeight + 1))
	require.Equal(t, mergeHeight, h.app.UpgradeKeeper.GetDoneHeight(h.ctx(), pcommon.MergeUpgrade))
	require.Equal(t, deleteHeight, h.app.UpgradeKeeper.GetDoneHeight(h.ctx(), layerxanchor.StoreDeleteUpgrade))

	want := pcommon.RetiredRevert(layerxanchor.PrecompileName)
	require.Len(t, want, 132)
	writes := map[string][]interface{}{
		layerxanchor.SubmitCheckpointMethod:          h.checkpointArgs("batch_1_quorum"),
		layerxanchor.SubmitAvailabilityMethod:        {h.attestation("batch_1_attestation_1")},
		layerxanchor.FinalizeMethod:                  {uint64(1)},
		layerxanchor.RegisterGuarantorMethod:         {h.ids[0], h.signers[0]},
		layerxanchor.IncreaseBondMethod:              {h.ids[0]},
		layerxanchor.BeginUnbondMethod:               {h.ids[0], big.NewInt(bond / 2)},
		layerxanchor.CompleteUnbondMethod:            {h.ids[0]},
		layerxanchor.SubmitEquivocationMethod:        {[]byte{5}, []byte{6}},
		layerxanchor.OpenChallengeMethod:             {uint64(1), anchortypes.ChallengeFraud, [32]byte{1}},
		layerxanchor.ResolveChallengeMethod:          {uint64(1), false},
		layerxanchor.ActivateGuarantorMethod:         {h.ids[0]},
		layerxanchor.SetSequencerAuthorizationMethod: {[32]byte{1}, [32]byte{2}, uint64(1), uint64(8)},
	}
	writeCount := 0
	for name, method := range h.precompile.GetABI().Methods {
		if method.StateMutability == "view" || method.StateMutability == "pure" {
			continue
		}
		args, ok := writes[name]
		require.True(t, ok, "every ABI write must be exercised: %s", name)
		input := h.input(name, args...)
		require.Greater(t, len(input), 4, "every ABI write must have arguments: %s", name)
		for _, calldata := range []struct {
			name  string
			input []byte
		}{
			{name: "selector", input: input[:4]},
			{name: "truncated", input: input[:len(input)-1]},
		} {
			for _, readOnly := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/readOnly=%t", name, calldata.name, readOnly), func(t *testing.T) {
					data, err := h.precompile.Run(h.evm, h.reporter.address, h.reporter.address, calldata.input, nil, readOnly, false, nil)
					require.Equal(t, vm.ErrExecutionReverted, err)
					require.Equal(t, want, data)
					require.Len(t, data, 132)
					reason, err := abi.UnpackRevert(data)
					require.NoError(t, err)
					require.Equal(t, "LXP_MODULE_DISABLED: module disabled; layerxAnchor retired", reason)
				})
			}
		}
		writeCount++
	}
	require.Equal(t, 12, writeCount)
}
