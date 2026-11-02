package layerxcustody_test

import (
	"math/big"
	"testing"

	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxcustody"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func custodyRetirementCalls(h *harness) map[string][]interface{} {
	return map[string][]interface{}{
		layerxcustody.DepositMethod:                   {beneficiary},
		layerxcustody.DepositTokenMethod:              {tokenPointer, big.NewInt(1), beneficiary},
		layerxcustody.RequestWithdrawalMethod:         h.withdrawalArgs(),
		layerxcustody.FinaliseWithdrawalMethod:        h.withdrawalArgs(),
		layerxcustody.RequestForcedExitMethod:         h.exitArgs(number(h.t, h.withdrawal, "batch_number") + 2),
		layerxcustody.ExecuteForcedExitMethod:         h.exitArgs(number(h.t, h.withdrawal, "batch_number") + 2),
		layerxcustody.DepositCountMethod:              {},
		layerxcustody.DepositNonceMethod:              {h.caller, array(h.t, h.withdrawal, "asset")},
		layerxcustody.GetDepositMethod:                {[32]byte{1}},
		layerxcustody.GetDepositByIndexMethod:         {uint64(1)},
		layerxcustody.GetClaimMethod:                  {[32]byte{1}},
		layerxcustody.NullifierStatusMethod:           {[32]byte{1}},
		layerxcustody.GetAssetMethod:                  {array(h.t, h.withdrawal, "asset")},
		layerxcustody.AssetByPointerMethod:            {tokenPointer},
		layerxcustody.NativeAssetIDMethod:             {},
		layerxcustody.ExitEligibleMethod:              {},
		layerxcustody.RegisterDepositRootMethod:       {[]byte{}, []byte{}, [][32]byte{}},
		layerxcustody.DepositRootAuthorityMethod:      {},
		layerxcustody.DepositRootRegisteredMethod:     {[32]byte{1}},
		layerxcustody.DepositRegistrationDigestMethod: {[32]byte{1}},
	}
}

func TestEveryCustodyMethodRevertsModuleDisabledFromTheMergePlanHeight(t *testing.T) {
	h := newHarness(t, 0)
	const planHeight = int64(20)
	_, err := h.call(layerxcustody.DepositCountMethod)
	require.NoError(t, err)
	testkeeper.EVMTestApp.UpgradeKeeper.SetDone(h.ctx.WithBlockHeight(planHeight), pcommon.MergeUpgrade)
	require.Equal(t, planHeight, testkeeper.EVMTestApp.UpgradeKeeper.GetDoneHeight(h.ctx, pcommon.MergeUpgrade))
	h.at(h.ctx.WithBlockHeight(planHeight - 1))
	_, err = h.call(layerxcustody.DepositCountMethod)
	require.NoError(t, err)

	calls := custodyRetirementCalls(h)
	methods := h.precompile.GetABI().Methods
	require.Len(t, methods, 20)
	require.Len(t, calls, len(methods))
	for _, height := range []int64{planHeight, planHeight + 1} {
		h.at(h.ctx.WithBlockHeight(height))
		for name := range methods {
			t.Run(big.NewInt(height).String()+"/"+name, func(t *testing.T) {
				args, ok := calls[name]
				require.True(t, ok, "every ABI method must have a call")
				res, err := h.precompile.Run(h.evm, h.caller, h.caller, h.input(name, args...), nil, false, false, nil)
				require.ErrorIs(t, err, vm.ErrExecutionReverted)
				require.Equal(t, pcommon.RetiredRevert(layerxcustody.PrecompileName), res)
				reason, err := abi.UnpackRevert(res)
				require.NoError(t, err)
				require.Equal(t, "LXP_MODULE_DISABLED: module disabled; layerxCustody retired", reason)
			})
		}
	}
}

func TestCustodyRetiredGateRunsBeforeTheCallShapeChecks(t *testing.T) {
	for _, tc := range []struct {
		name     string
		value    *big.Int
		delegate bool
		readOnly bool
		method   string
	}{
		{name: "nonpayable value", value: big.NewInt(1), method: layerxcustody.DepositCountMethod},
		{name: "delegatecall", delegate: true, method: layerxcustody.DepositCountMethod},
		{name: "static state change", readOnly: true, method: layerxcustody.DepositMethod},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, 0)
			testkeeper.EVMTestApp.UpgradeKeeper.SetDone(h.ctx, pcommon.MergeUpgrade)
			args := custodyRetirementCalls(h)[tc.method]
			res, err := h.precompile.Run(h.evm, h.caller, h.caller, h.input(tc.method, args...), tc.value, tc.readOnly, tc.delegate, nil)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			require.Equal(t, pcommon.RetiredRevert(layerxcustody.PrecompileName), res)
		})
	}
}

func TestCustodyViewsRevertModuleDisabledUnderStaticcall(t *testing.T) {
	h := newHarness(t, 0)
	testkeeper.EVMTestApp.UpgradeKeeper.SetDone(h.ctx, pcommon.MergeUpgrade)
	calls := custodyRetirementCalls(h)
	views := 0
	for name, method := range h.precompile.GetABI().Methods {
		if method.StateMutability != "view" {
			continue
		}
		views++
		t.Run(name, func(t *testing.T) {
			args, ok := calls[name]
			require.True(t, ok)
			res, err := h.precompile.Run(h.evm, h.caller, h.caller, h.input(name, args...), nil, true, false, nil)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			require.Equal(t, pcommon.RetiredRevert(layerxcustody.PrecompileName), res)
		})
	}
	require.Equal(t, 13, views)
}
