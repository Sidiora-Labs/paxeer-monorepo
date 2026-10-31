package layerxanchor_test

import (
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/testvectors"
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
	h.fixture, err = testvectors.LoadAnchor()
	require.NoError(t, err)
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
	writes := anchorRetirementWrites(h)
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
	reads := anchorRetirementReads(h)
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
