package layerxbridge_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxbridge"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/require"
)

func TestBridgeRevertCodesAgreeWithTheLXPRegistry(t *testing.T) {
	cases := []struct {
		err  error
		code string
	}{
		{types.ErrUnauthorized, "LXP_BRIDGE_UNAUTHORIZED"},
		{types.ErrUnknownChain, "LXP_BRIDGE_UNKNOWN_CHAIN"},
		{types.ErrChainDisabled, "LXP_BRIDGE_CHAIN_DISABLED"},
		{types.ErrUnknownAsset, "LXP_BRIDGE_UNKNOWN_ASSET"},
		{types.ErrPaused, "LXP_BRIDGE_PAUSED"},
		{types.ErrNullified, "LXP_BRIDGE_NULLIFIED"},
		{types.ErrCapExceeded, "LXP_BRIDGE_CAP_EXCEEDED"},
		{types.ErrBadSignature, "LXP_BRIDGE_BAD_SIGNATURE"},
		{types.ErrBelowThreshold, "LXP_BRIDGE_BELOW_THRESHOLD"},
		{types.ErrVaultMismatch, "LXP_BRIDGE_VAULT_MISMATCH"},
		{types.ErrInvalidRequest, "LXP_BRIDGE_INVALID_REQUEST"},
	}
	require.Len(t, cases, 11)
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			code := layerxbridge.RevertCode(tc.err)
			require.Equal(t, tc.code, code)
			reason, err := abi.UnpackRevert(pcommon.LXPReason(tc.err))
			require.NoError(t, err)
			registryCode, message, found := strings.Cut(reason, ": ")
			require.True(t, found, "registry must resolve the registered Go error")
			require.Equal(t, tc.code, registryCode)
			require.Equal(t, code, registryCode)
			require.Equal(t, tc.err.Error(), message)
			want := pcommon.RevertReason(errors.New(code + ": " + tc.err.Error()))
			require.Equal(t, want, pcommon.LXPReason(tc.err))
			require.Equal(t, want, pcommon.LXPRevert(code, ""))
		})
	}

	t.Run("failed-call", func(t *testing.T) {
		h := newHarness(t)
		h.activate()
		testkeeper.EVMTestApp.EvmKeeper.UpgradeKeeper().SetDone(h.ctx, pcommon.MergeUpgrade)
		args := h.bridgeInArgs(1, 100, h.attestors[0], h.attestors[1])
		args[0] = uint64(9)
		res := h.revert(layerxbridge.BridgeInMethod, false, args...)
		code := layerxbridge.RevertCode(types.ErrUnknownChain)
		want := pcommon.RevertReason(errors.New(code + ": " + types.ErrUnknownChain.Error()))
		require.Equal(t, want, res)
		require.Equal(t, pcommon.LXPReason(types.ErrUnknownChain), res)
	})
}
