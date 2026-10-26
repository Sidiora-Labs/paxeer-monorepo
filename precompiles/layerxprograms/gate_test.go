package layerxprograms

import (
	"math/big"
	"strconv"
	"testing"

	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/require"
)

func TestProgramsGateRefusesBeforeMergeHeight(t *testing.T) {
	app := testkeeper.EVMTestApp
	parent := app.GetContextForDeliverTx([]byte{})
	ctx, _ := parent.CacheContext()
	const planName = "programs-gate-merge-plan"
	const doneHeight int64 = 100
	gate := programsGate{upgrade: &app.UpgradeKeeper, planName: planName}
	require.Zero(t, app.UpgradeKeeper.GetDoneHeight(ctx, planName))

	assertDisabled := func(t *testing.T, ctx sdk.Context, gate programsGate) {
		t.Helper()
		err := gate.checkActive(ctx)
		require.ErrorIs(t, err, ErrModuleDisabled)
		require.Equal(t, pcommon.LXPRevert("LXP_MODULE_DISABLED", ""), pcommon.RevertReason(err))
	}
	for _, height := range []int64{0, doneHeight - 1, doneHeight, doneHeight + 1, 1000000} {
		t.Run("not-done/"+strconv.FormatInt(height, 10), func(t *testing.T) {
			assertDisabled(t, ctx.WithBlockHeight(height), gate)
		})
	}

	app.UpgradeKeeper.SetDone(ctx.WithBlockHeight(doneHeight), planName+"-other")
	assertDisabled(t, ctx.WithBlockHeight(doneHeight+1), gate)
	app.UpgradeKeeper.SetDone(ctx.WithBlockHeight(0), planName)
	assertDisabled(t, ctx.WithBlockHeight(doneHeight+1), gate)
	app.UpgradeKeeper.SetDone(ctx.WithBlockHeight(doneHeight), planName)
	require.Equal(t, doneHeight, app.UpgradeKeeper.GetDoneHeight(ctx, planName))

	require.NotEmpty(t, programsABI.Methods)
	for name, method := range programsABI.Methods {
		t.Run(name, func(t *testing.T) {
			for _, height := range []int64{0, doneHeight - 1} {
				assertDisabled(t, ctx.WithBlockHeight(height), gate)
			}
			for _, height := range []int64{doneHeight, doneHeight + 1, 1000000} {
				require.NoError(t, gate.checkActive(ctx.WithBlockHeight(height)))
				require.NoError(t, checkCall(&method, nil, false))
			}
		})
	}
	assertDisabled(t, ctx.WithBlockHeight(doneHeight), programsGate{})
	assertDisabled(t, ctx.WithBlockHeight(doneHeight), programsGate{upgrade: &app.UpgradeKeeper})
	assertDisabled(t, ctx.WithBlockHeight(doneHeight), programsGate{planName: planName})
	assertDisabled(t, sdk.Context{}, gate)
	require.Zero(t, app.UpgradeKeeper.GetDoneHeight(parent, planName))
}

func TestProgramsGateRefusesValueAndStaticWrites(t *testing.T) {
	writes := map[string]bool{
		"deploy":           true,
		"upgrade":          true,
		"call":             true,
		"sandboxExecute":   true,
		"retireStorage":    true,
		"registryRead":     true,
		"transfer":         true,
		"registerAccount":  true,
		"setExitRoute":     true,
		"deprecate":        true,
		"tombstone":        true,
		"exitValue":        true,
		"exitValueBounded": true,
		"sandboxFund":      true,
		"sandboxActivate":  true,
	}
	require.NotEmpty(t, programsABI.Methods)
	for name := range writes {
		require.Contains(t, programsABI.Methods, name)
	}
	require.Contains(t, programsABI.Methods, "simulateCall")
	maxValue := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	for name, method := range programsABI.Methods {
		t.Run(name, func(t *testing.T) {
			for _, readOnly := range []bool{false, true} {
				for _, value := range []*big.Int{big.NewInt(1), maxValue, big.NewInt(-1)} {
					err := checkCall(&method, value, readOnly)
					require.Error(t, err)
					reason, unpackErr := abi.UnpackRevert(pcommon.LXPReason(err))
					require.NoError(t, unpackErr)
					require.Equal(t, "LXP_PRECOMPILE_NON_PAYABLE: sending funds to a non-payable function", reason)
					require.Equal(t, pcommon.LXPRevert("LXP_PRECOMPILE_NON_PAYABLE", ""), pcommon.RevertReason(err))
				}
				for _, value := range []*big.Int{nil, big.NewInt(0)} {
					err := checkCall(&method, value, readOnly)
					if readOnly && writes[name] {
						require.ErrorIs(t, err, ErrReadOnlyWrite)
						require.Equal(t, pcommon.LXPRevert("LXP_PROGRAM_REFUSED", "write under staticcall"), pcommon.RevertReason(err))
					} else {
						require.NoError(t, err)
					}
				}
			}
		})
	}
	write := programsABI.Methods["deploy"]
	write.StateMutability = "view"
	require.ErrorIs(t, checkCall(&write, nil, true), ErrReadOnlyWrite)
	simulation := programsABI.Methods["simulateCall"]
	simulation.StateMutability = "nonpayable"
	require.NoError(t, checkCall(&simulation, nil, true))
	for _, readOnly := range []bool{false, true} {
		require.ErrorIs(t, checkCall(nil, nil, readOnly), errGateUnknownMethod)
		require.ErrorIs(t, checkCall(&abi.Method{Name: "unknown"}, nil, readOnly), errGateUnknownMethod)
	}
}
