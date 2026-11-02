package layerxanchor_test

import (
	"fmt"
	"math/big"
	"testing"

	anchorkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/keeper"
	anchortypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxanchor"
	putils "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

type wiredAnchorKeepers struct {
	putils.Keepers
	anchor anchorkeeper.Keeper
}

func (k wiredAnchorKeepers) AnchorK() putils.AnchorKeeper { return k.anchor }

func anchorRetirementReads(h *harness, checkpointID [32]byte) map[string][]interface{} {
	return map[string][]interface{}{
		layerxanchor.LatestFinalizedMethod:      {},
		layerxanchor.CheckpointMethod:           {uint64(1)},
		layerxanchor.FinalizedStateRootMethod:   {uint64(1)},
		layerxanchor.FinalizedReceiptRootMethod: {uint64(1)},
		layerxanchor.GuarantorMethod:            {h.ids[0]},
		layerxanchor.ThresholdMethod:            {},
		layerxanchor.StatusOfMethod:             {uint64(1)},
		layerxanchor.CheckpointBatchMethod:      {checkpointID},
		layerxanchor.CheckpointGuarantorsMethod: {uint64(1)},
	}
}

func TestReadsServedUntilStoreDelete(t *testing.T) {
	h := newHarness(t, nil)
	h.bootstrap()
	checkpointState := importRetirementCheckpoint(h, anchortypes.CheckpointFinal)
	checkpointID := [32]byte(checkpointState.CheckpointID)
	reads := anchorRetirementReads(h, checkpointID)
	require.Len(t, reads, 9)
	viewCount := 0
	for name, method := range h.precompile.GetABI().Methods {
		if method.StateMutability == "view" {
			viewCount++
			require.Contains(t, reads, name)
		}
	}
	require.Equal(t, len(reads), viewCount)

	stateRoot, receiptRoot := [32]byte(checkpointState.StateRoot), [32]byte(checkpointState.ReceiptRoot)
	var err error
	require.Equal(t, []interface{}{uint64(1), true}, h.view(layerxanchor.LatestFinalizedMethod))
	require.Equal(t, []interface{}{stateRoot, true}, h.view(layerxanchor.FinalizedStateRootMethod, uint64(1)))
	require.Equal(t, []interface{}{receiptRoot, true}, h.view(layerxanchor.FinalizedReceiptRootMethod, uint64(1)))
	require.Equal(t, []interface{}{uint32(2)}, h.view(layerxanchor.ThresholdMethod))
	require.Equal(t, []interface{}{anchortypes.CheckpointFinal}, h.view(layerxanchor.StatusOfMethod, uint64(1)))
	require.Equal(t, []interface{}{uint64(1), anchortypes.CheckpointFinal}, h.view(layerxanchor.CheckpointBatchMethod, checkpointID))
	require.Equal(t, []interface{}{h.ids}, h.view(layerxanchor.CheckpointGuarantorsMethod, uint64(1)))
	checkpoint := abi.ConvertType(h.view(layerxanchor.CheckpointMethod, uint64(1))[0], new(layerxanchor.CheckpointView)).(*layerxanchor.CheckpointView)
	require.Equal(t, checkpointID, checkpoint.CheckpointId)
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
	vector := h.vector("checkpoints", "batch_1_quorum")
	checkpointID, err := vector.Array32("checkpoint_id")
	require.NoError(t, err)
	signers, err := vector.Uint64("signers")
	require.NoError(t, err)
	require.Equal(t, signers, uint64(len(h.ids)))
	stateRoot, err := vector.Array32("resulting_state_root")
	require.NoError(t, err)
	receiptRoot, err := vector.Array32("receipt_root")
	require.NoError(t, err)
	guarantors := make([]anchortypes.Hash32, len(h.ids))
	for i, id := range h.ids {
		guarantors[i] = id
	}
	require.Len(t, guarantors, len(h.ids))
	withAnchor, err := layerxanchor.NewPrecompile(wiredAnchorKeepers{
		Keepers: h.app.GetPrecompileKeepers(), anchor: h.app.LayerXAnchorKeeper,
	})
	require.NoError(t, err)
	h.precompile = withAnchor
	h.bootstrap()
	genesis := h.anchor.ExportGenesis(h.ctx())
	genesis.Checkpoints = []anchortypes.Checkpoint{{
		BatchNumber: 1, CheckpointID: checkpointID, HeaderDigest: [32]byte{4},
		ProtocolVersion: 1, NetworkID: 125, Epoch: 1, FirstSequence: 1, LastSequence: 2,
		PreviousStateRoot: [32]byte{5}, StateRoot: stateRoot, ReceiptRoot: receiptRoot,
		DataAvailabilityRoot: [32]byte{6}, SequencerID: [32]byte{1},
		TimestampMs: uint64(h.ctx().BlockTime().UnixMilli()),
		Status:      anchortypes.CheckpointFinal, DeclaredThreshold: 2, Guarantors: guarantors,
		Submitter: h.reporter.account.String(), SubmittedHeight: h.ctx().BlockHeight(),
		SubmittedTime: h.ctx().BlockTime().Unix(), FinalizedHeight: h.ctx().BlockHeight(),
		FinalizedTime: h.ctx().BlockTime().Unix(),
	}}
	genesis.HasLatestFinalized, genesis.LatestFinalized = true, 1
	require.NoError(t, genesis.Validate())
	h.anchor.InitGenesis(h.ctx(), *genesis)
	h.invariant()
	const mergeHeight, deleteHeight = int64(20), int64(40)
	h.app.UpgradeKeeper.SetDone(h.ctx().WithBlockHeight(mergeHeight), pcommon.MergeUpgrade)
	require.Equal(t, anchortypes.StoreDeleteUpgrade, layerxanchor.StoreDeleteUpgrade)

	keepers, ok := h.app.GetPrecompileKeepers().(*app.PrecompileKeepers)
	require.True(t, ok)
	withoutAnchor, err := layerxanchor.NewPrecompile(keepers)
	require.NoError(t, err)
	precompiles := map[string]*pcommon.Precompile{"wired": withAnchor, "unwired": withoutAnchor}
	reads := anchorRetirementReads(h, checkpointID)
	require.Len(t, reads, 9)
	h.stateDB.WithCtx(h.ctx().WithBlockHeight(deleteHeight - 1))
	require.Equal(t, []interface{}{uint64(1), true}, h.view(layerxanchor.LatestFinalizedMethod))
	require.Equal(t, []interface{}{stateRoot, true}, h.view(layerxanchor.FinalizedStateRootMethod, uint64(1)))
	require.Equal(t, []interface{}{receiptRoot, true}, h.view(layerxanchor.FinalizedReceiptRootMethod, uint64(1)))
	require.Equal(t, []interface{}{uint64(1), anchortypes.CheckpointFinal}, h.view(layerxanchor.CheckpointBatchMethod, checkpointID))
	require.Equal(t, []interface{}{h.ids}, h.view(layerxanchor.CheckpointGuarantorsMethod, uint64(1)))
	for name, args := range reads {
		for _, readOnly := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/wired/%s/static=%t", deleteHeight-1, name, readOnly), func(t *testing.T) {
				res, err := withAnchor.GetExecutor().Execute(h.ctx(), h.method(name), h.reporter.address, h.reporter.address, args, nil, readOnly, h.evm, nil)
				require.NoError(t, err)
				require.NotEqual(t, pcommon.RetiredRevert(layerxanchor.PrecompileName), res)
				res, err = withAnchor.Run(h.evm, h.reporter.address, h.reporter.address, h.input(name, args...), nil, readOnly, false, nil)
				require.NoError(t, err)
				require.NotEqual(t, pcommon.RetiredRevert(layerxanchor.PrecompileName), res)
			})
		}
	}
	h.app.UpgradeKeeper.SetDone(h.ctx().WithBlockHeight(deleteHeight), anchortypes.StoreDeleteUpgrade)
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
