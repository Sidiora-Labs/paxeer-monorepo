package layerxanchor_test

import (
	"fmt"
	"math/big"
	"testing"

	anchortypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxanchor"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func anchorRetirementReads(h *harness) map[string][]interface{} {
	return map[string][]interface{}{
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
}

func TestReadsServedUntilStoreDelete(t *testing.T) {
	h := newHarness(t, nil)
	h.bootstrap()
	_, err := h.submit("batch_1_quorum")
	require.NoError(t, err)
	reads := anchorRetirementReads(h)
	require.Len(t, reads, 9)
	viewCount := 0
	for name, method := range h.precompile.GetABI().Methods {
		if method.StateMutability == "view" {
			viewCount++
			require.Contains(t, reads, name)
		}
	}
	require.Equal(t, len(reads), viewCount)

	vector := h.vector("checkpoints", "batch_1_quorum")
	stateRoot, err := vector.Array32("resulting_state_root")
	require.NoError(t, err)
	receiptRoot, err := vector.Array32("receipt_root")
	require.NoError(t, err)
	require.Equal(t, []interface{}{uint64(1), true}, h.view(layerxanchor.LatestFinalizedMethod))
	require.Equal(t, []interface{}{stateRoot, true}, h.view(layerxanchor.FinalizedStateRootMethod, uint64(1)))
	require.Equal(t, []interface{}{receiptRoot, true}, h.view(layerxanchor.FinalizedReceiptRootMethod, uint64(1)))
	require.Equal(t, []interface{}{uint32(2)}, h.view(layerxanchor.ThresholdMethod))
	require.Equal(t, []interface{}{anchortypes.CheckpointFinal}, h.view(layerxanchor.StatusOfMethod, uint64(1)))
	require.Equal(t, []interface{}{uint64(1), anchortypes.CheckpointFinal}, h.view(layerxanchor.CheckpointBatchMethod, h.vectorIDForRetirement()))
	require.Equal(t, []interface{}{h.ids}, h.view(layerxanchor.CheckpointGuarantorsMethod, uint64(1)))
	checkpoint := abi.ConvertType(h.view(layerxanchor.CheckpointMethod, uint64(1))[0], new(layerxanchor.CheckpointView)).(*layerxanchor.CheckpointView)
	require.Equal(t, h.vectorIDForRetirement(), checkpoint.CheckpointId)
	require.Equal(t, stateRoot, checkpoint.StateRoot)
	require.Equal(t, receiptRoot, checkpoint.ReceiptRoot)
	require.Equal(t, anchortypes.CheckpointFinal, checkpoint.Status)
	guarantor := abi.ConvertType(h.view(layerxanchor.GuarantorMethod, h.ids[0])[0], new(layerxanchor.GuarantorView)).(*layerxanchor.GuarantorView)
	require.Equal(t, h.ids[0], guarantor.GuarantorId)
	require.Equal(t, h.signers[0], guarantor.Signer)
	require.Equal(t, h.operators[0].address, guarantor.Operator)
	require.Equal(t, big.NewInt(bond), guarantor.Bond)
	require.True(t, guarantor.Eligible)

	before := make(map[string][]byte, len(reads))
	for name, args := range reads {
		before[name], err = h.precompile.Run(h.evm, h.reporter.address, h.reporter.address, h.input(name, args...), nil, true, false, nil)
		require.NoError(t, err, name)
	}
	const mergeHeight, deleteHeight = int64(20), int64(40)
	h.app.UpgradeKeeper.SetDone(h.ctx().WithBlockHeight(mergeHeight), pcommon.MergeUpgrade)
	require.NoError(t, h.app.UpgradeKeeper.ScheduleUpgrade(h.ctx(), upgradetypes.Plan{
		Name: anchortypes.StoreDeleteUpgrade, Height: deleteHeight,
	}))
	for _, applied := range []bool{false, true} {
		heights := []int64{mergeHeight, deleteHeight - 1, deleteHeight, deleteHeight + 1}
		if applied {
			h.app.UpgradeKeeper.SetDone(h.ctx().WithBlockHeight(deleteHeight), anchortypes.StoreDeleteUpgrade)
			heights = heights[:2]
		}
		for _, height := range heights {
			h.stateDB.WithCtx(h.ctx().WithBlockHeight(height))
			for name, args := range reads {
				for _, readOnly := range []bool{false, true} {
					t.Run(fmt.Sprintf("applied=%t/%d/%s/static=%t", applied, height, name, readOnly), func(t *testing.T) {
						res, err := h.precompile.Run(h.evm, h.reporter.address, h.reporter.address, h.input(name, args...), nil, readOnly, false, nil)
						require.NoError(t, err)
						require.Equal(t, before[name], res)
					})
				}
			}
		}
	}
}

func TestReadsRevertFromStoreDelete(t *testing.T) {
	h := newHarness(t, nil)
	h.bootstrap()
	_, err := h.submit("batch_1_quorum")
	require.NoError(t, err)
	const mergeHeight, deleteHeight = int64(20), int64(40)
	h.app.UpgradeKeeper.SetDone(h.ctx().WithBlockHeight(mergeHeight), pcommon.MergeUpgrade)
	h.app.UpgradeKeeper.SetDone(h.ctx().WithBlockHeight(deleteHeight), anchortypes.StoreDeleteUpgrade)
	require.Equal(t, anchortypes.StoreDeleteUpgrade, layerxanchor.StoreDeleteUpgrade)

	keepers, ok := h.app.GetPrecompileKeepers().(*app.PrecompileKeepers)
	require.True(t, ok)
	keepers.AnchorKeeper = nil
	withoutAnchor, err := layerxanchor.NewPrecompile(keepers)
	require.NoError(t, err)
	precompiles := map[string]*pcommon.Precompile{"wired": h.precompile, "unwired": withoutAnchor}
	reads := anchorRetirementReads(h)
	require.Len(t, reads, 9)
	for _, height := range []int64{deleteHeight, deleteHeight + 1} {
		h.stateDB.WithCtx(h.ctx().WithBlockHeight(height))
		for wiring, precompile := range precompiles {
			for name, args := range reads {
				for _, readOnly := range []bool{false, true} {
					t.Run(fmt.Sprintf("%d/%s/%s/static=%t", height, wiring, name, readOnly), func(t *testing.T) {
						res, err := precompile.GetExecutor().Execute(h.ctx(), h.method(name), h.reporter.address, h.reporter.address, args, nil, readOnly, h.evm, nil)
						require.ErrorIs(t, err, vm.ErrExecutionReverted)
						require.Equal(t, pcommon.RetiredRevert(layerxanchor.PrecompileName), res)
						res, err = precompile.Run(h.evm, h.reporter.address, h.reporter.address, h.input(name, args...), nil, readOnly, false, nil)
						require.ErrorIs(t, err, vm.ErrExecutionReverted)
						require.Equal(t, pcommon.RetiredRevert(layerxanchor.PrecompileName), res)
						reason, err := abi.UnpackRevert(res)
						require.NoError(t, err)
						require.Equal(t, "LXP_MODULE_DISABLED: module disabled; layerxAnchor retired", reason)
					})
				}
			}
		}
	}
}
