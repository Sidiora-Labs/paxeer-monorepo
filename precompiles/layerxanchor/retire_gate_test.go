package layerxanchor_test

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	anchortypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxanchor"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func anchorRetirementWrites(h *harness) map[string][]interface{} {
	context := h.fixture["context"][0]
	id, err := context.Array32("sequencer_id")
	require.NoError(h.t, err)
	key, err := context.Array32("sequencer_public_key")
	require.NoError(h.t, err)
	return map[string][]interface{}{
		layerxanchor.SubmitCheckpointMethod:          h.checkpointArgs("batch_1_quorum"),
		layerxanchor.SubmitAvailabilityMethod:        {h.attestation("batch_1_attestation_1")},
		layerxanchor.FinalizeMethod:                  {uint64(1)},
		layerxanchor.RegisterGuarantorMethod:         {h.ids[0], h.signers[0]},
		layerxanchor.IncreaseBondMethod:              {h.ids[0]},
		layerxanchor.BeginUnbondMethod:               {h.ids[0], big.NewInt(bond / 2)},
		layerxanchor.CompleteUnbondMethod:            {h.ids[0]},
		layerxanchor.SubmitEquivocationMethod:        {h.attestation("batch_1_attestation_0"), h.attestation("batch_1_conflicting_attestation_0")},
		layerxanchor.OpenChallengeMethod:             {uint64(1), anchortypes.ChallengeFraud, [32]byte{1}},
		layerxanchor.ResolveChallengeMethod:          {uint64(1), false},
		layerxanchor.ActivateGuarantorMethod:         {h.ids[0]},
		layerxanchor.SetSequencerAuthorizationMethod: {id, key, uint64(1), uint64(8)},
	}
}

func TestRetiredWritesRevertAfterMerge(t *testing.T) {
	h := newHarness(t, nil)
	h.bootstrap()
	_, err := h.submit("batch_1_quorum")
	require.NoError(t, err)
	const mergeHeight = int64(20)
	h.app.UpgradeKeeper.SetDone(h.ctx().WithBlockHeight(mergeHeight), pcommon.MergeUpgrade)
	require.Equal(t, mergeHeight, h.app.UpgradeKeeper.GetDoneHeight(h.ctx(), pcommon.MergeUpgrade))
	writes := anchorRetirementWrites(h)
	require.Len(t, writes, 12)
	views := map[string][]interface{}{
		layerxanchor.LatestFinalizedMethod:      {},
		layerxanchor.CheckpointMethod:           {uint64(1)},
		layerxanchor.FinalizedStateRootMethod:   {uint64(1)},
		layerxanchor.FinalizedReceiptRootMethod: {uint64(1)},
		layerxanchor.GuarantorMethod:            {h.ids[0]},
		layerxanchor.ThresholdMethod:            {},
		layerxanchor.StatusOfMethod:             {uint64(1)},
		layerxanchor.CheckpointBatchMethod:      {h.vectorIDForRetirement()},
		layerxanchor.CheckpointGuarantorsMethod: {uint64(1)},
	}
	before := make(map[string][]byte)
	for name, args := range views {
		before[name], err = h.precompile.Run(h.evm, h.reporter.address, h.reporter.address, h.input(name, args...), nil, true, false, nil)
		require.NoError(t, err)
	}
	for _, height := range []int64{mergeHeight, mergeHeight + 1} {
		h.stateDB.WithCtx(h.ctx().WithBlockHeight(height))
		for name, method := range h.precompile.GetABI().Methods {
			t.Run(fmt.Sprintf("%d/%s", height, name), func(t *testing.T) {
				if method.StateMutability == "view" {
					args, ok := views[name]
					require.True(t, ok)
					res, err := h.precompile.Run(h.evm, h.reporter.address, h.reporter.address, h.input(name, args...), nil, true, false, nil)
					require.NoError(t, err)
					require.Equal(t, before[name], res)
					return
				}
				args, ok := writes[name]
				require.True(t, ok, "every write must be exercised")
				res, err := h.precompile.GetExecutor().Execute(h.ctx(), h.method(name), h.authority.address, h.authority.address, args, nil, false, h.evm, nil)
				require.ErrorIs(t, err, vm.ErrExecutionReverted)
				require.Equal(t, pcommon.RetiredRevert(layerxanchor.PrecompileName), res)
				reason, err := abi.UnpackRevert(res)
				require.NoError(t, err)
				require.Equal(t, "LXP_MODULE_DISABLED: module disabled; layerxAnchor retired", reason)
			})
		}
	}
}

func (h *harness) vectorIDForRetirement() [32]byte {
	h.t.Helper()
	id, err := h.vector("checkpoints", "batch_1_quorum").Array32("checkpoint_id")
	require.NoError(h.t, err)
	return id
}

func TestWritesServedBeforeMerge(t *testing.T) {
	params := func(p *anchortypes.Params) { p.ChallengeWindowSeconds = 50 }
	baseline, planned := newHarness(t, params), newHarness(t, params)
	const mergeHeight = int64(20)
	planned.app.UpgradeKeeper.SetDone(planned.ctx().WithBlockHeight(mergeHeight), pcommon.MergeUpgrade)
	for _, h := range []*harness{baseline, planned} {
		h.stateDB.WithCtx(h.ctx().WithBlockHeight(mergeHeight - 1))
	}
	seen := make(map[string]bool)
	compare := func(name string, from func(*harness) party, units int64, args []interface{}, signatures uint64) []interface{} {
		t.Helper()
		var expected []byte
		var expectedGas uint64
		var result []interface{}
		for index, h := range []*harness{baseline, planned} {
			meter := sdk.NewInfiniteGasMeter(1, 1)
			h.stateDB.WithCtx(h.ctx().WithGasMeter(meter))
			input := h.input(name, args...)
			require.Equal(t, layerxanchor.WriteBaseGas+layerxanchor.GasPerByte*uint64(len(input)-4)+layerxanchor.GasPerSignature*signatures, h.precompile.RequiredGas(input))
			out, err := h.run(from(h), units, false, name, args...)
			require.NoError(t, err, name)
			gas := meter.GasConsumed()
			raw, err := h.method(name).Outputs.Pack(out...)
			require.NoError(t, err)
			if index == 0 {
				expected, expectedGas = raw, gas
			} else {
				require.Equal(t, expected, raw, name)
				require.Equal(t, expectedGas, gas, name)
			}
			result = out
		}
		seen[name] = true
		return result
	}
	for i := range baseline.ids {
		operator := func(h *harness) party { return h.operators[i] }
		compare(layerxanchor.RegisterGuarantorMethod, operator, bond, []interface{}{baseline.ids[i], baseline.signers[i]}, 0)
		compare(layerxanchor.ActivateGuarantorMethod, func(h *harness) party { return h.authority }, 0, []interface{}{baseline.ids[i]}, 0)
	}
	calls := anchorRetirementWrites(baseline)
	authority := func(h *harness) party { return h.authority }
	reporter := func(h *harness) party { return h.reporter }
	operator := func(h *harness) party { return h.operators[1] }
	compare(layerxanchor.SetSequencerAuthorizationMethod, authority, 0, calls[layerxanchor.SetSequencerAuthorizationMethod], 0)
	compare(layerxanchor.IncreaseBondMethod, func(h *harness) party { return h.operators[2] }, bond, []interface{}{baseline.ids[2]}, 0)
	compare(layerxanchor.SubmitCheckpointMethod, reporter, 0, baseline.checkpointArgs("batch_1_below_threshold"), 2)
	compare(layerxanchor.SubmitAvailabilityMethod, reporter, 0, calls[layerxanchor.SubmitAvailabilityMethod], 1)
	compare(layerxanchor.SubmitCheckpointMethod, reporter, 0, calls[layerxanchor.SubmitCheckpointMethod], 4)
	challenge := compare(layerxanchor.OpenChallengeMethod, operator, 1_000_000, calls[layerxanchor.OpenChallengeMethod], 0)
	compare(layerxanchor.ResolveChallengeMethod, authority, 0, []interface{}{challenge[0], false}, 0)
	for _, h := range []*harness{baseline, planned} {
		h.stateDB.WithCtx(h.ctx().WithBlockTime(h.ctx().BlockTime().Add(60 * time.Second)))
	}
	compare(layerxanchor.FinalizeMethod, reporter, 0, calls[layerxanchor.FinalizeMethod], 0)
	compare(layerxanchor.BeginUnbondMethod, operator, 0, []interface{}{baseline.ids[1], big.NewInt(bond / 2)}, 0)
	for _, h := range []*harness{baseline, planned} {
		h.stateDB.WithCtx(h.ctx().WithBlockTime(h.ctx().BlockTime().Add(time.Duration(unbondingDelay) * time.Second)))
	}
	compare(layerxanchor.CompleteUnbondMethod, operator, 0, []interface{}{baseline.ids[1]}, 0)
	compare(layerxanchor.SubmitEquivocationMethod, reporter, 0, calls[layerxanchor.SubmitEquivocationMethod], 2)
	require.Len(t, seen, 12)
	baseline.invariant()
	planned.invariant()
}

func TestRetiredPayableWriteKeepsValue(t *testing.T) {
	for _, name := range []string{layerxanchor.RegisterGuarantorMethod, layerxanchor.IncreaseBondMethod, layerxanchor.OpenChallengeMethod} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, func(p *anchortypes.Params) { p.ChallengeWindowSeconds = 50 })
			if name != layerxanchor.RegisterGuarantorMethod {
				h.bootstrap()
				_, err := h.submit("batch_1_quorum")
				require.NoError(t, err)
			}
			h.app.UpgradeKeeper.SetDone(h.ctx(), pcommon.MergeUpgrade)
			caller := h.operators[0]
			precompileAccount := h.app.EvmKeeper.GetPaxAddressOrDefault(h.ctx(), h.precompile.Address())
			callerBefore, precompileBefore := h.balance(caller.account), h.balance(precompileAccount)
			guarantors := h.anchor.GetGuarantors(h.ctx())
			checkpoints := h.anchor.GetCheckpoints(h.ctx())
			logs := len(h.stateDB.GetAllLogs())
			args := anchorRetirementWrites(h)[name]
			value := new(big.Int).Mul(big.NewInt(bond), weiPerUnit)
			res, err := h.precompile.GetExecutor().Execute(h.ctx(), h.method(name), caller.address, caller.address, args, value, false, h.evm, nil)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			require.Equal(t, pcommon.RetiredRevert(layerxanchor.PrecompileName), res)
			require.Equal(t, callerBefore, h.balance(caller.account))
			require.Equal(t, precompileBefore, h.balance(precompileAccount))
			_, err = h.run(caller, bond, false, name, args...)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			require.Equal(t, callerBefore, h.balance(caller.account))
			require.Equal(t, precompileBefore, h.balance(precompileAccount))
			require.Equal(t, guarantors, h.anchor.GetGuarantors(h.ctx()))
			require.Equal(t, checkpoints, h.anchor.GetCheckpoints(h.ctx()))
			require.Len(t, h.stateDB.GetAllLogs(), logs)
			h.invariant()
		})
	}
}
