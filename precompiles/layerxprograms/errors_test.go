package layerxprograms

import (
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/require"
)

func TestProgramsKernelCodeRevert(t *testing.T) {
	require.NoError(t, kernelError(0, "call"))
	for _, tc := range []struct {
		code lxphost.Code
		name string
	}{
		{-1, "LXP_TRUNCATED"},
		{-3, "LXP_NON_CANONICAL"},
		{-103, "LXP_MODULE_DISABLED"},
		{-200, "LXP_UNKNOWN_DID"},
		{-736, "LXP_PROGRAM_REFUSED"},
		{-800, "LXP_BATCH_GAP"},
		{-999, "LXP_UNKNOWN"},
		{1, "LXP_UNKNOWN"},
	} {
		t.Run(tc.name+"/"+tc.code.Error(), func(t *testing.T) {
			const op = "programs.call"
			err := kernelError(tc.code, op)
			require.Error(t, err)
			require.ErrorIs(t, err, tc.code)
			var code lxphost.Code
			require.ErrorAs(t, err, &code)
			require.Equal(t, tc.code, code)
			require.Equal(t, tc.name, pcommon.LXPCodeName(int32(code)))
			require.Contains(t, err.Error(), op)
			require.Contains(t, err.Error(), tc.code.Error())
			registryReason, unpackErr := abi.UnpackRevert(pcommon.LXPRevert(tc.name, op))
			require.NoError(t, unpackErr)
			require.True(t, strings.HasPrefix(registryReason, tc.name+": "))
			for _, data := range [][]byte{pcommon.RevertReason(err), pcommon.LXPReason(err)} {
				require.Equal(t, []byte{0x08, 0xc3, 0x79, 0xa0}, data[:4])
				reason, unpackErr := abi.UnpackRevert(data)
				require.NoError(t, unpackErr)
				require.Equal(t, err.Error(), reason)
				require.True(t, strings.HasPrefix(reason, tc.name+": "), reason)
			}
		})
	}
	t.Run("fatalResultsHalt", func(t *testing.T) {
		for _, code := range []lxphost.Code{-1000, -1001, -1002, -1003} {
			require.PanicsWithValue(t, code, func() { kernelError(code, "call") })
		}
	})
}

func TestProgramsGateRevertCodes(t *testing.T) {
	for _, tc := range []struct {
		err    error
		code   string
		detail string
	}{
		{ErrModuleDisabled, "LXP_MODULE_DISABLED", ""},
		{ErrUnboundCaller, "LXP_ADDR_EVM_NOT_BOUND", ""},
		{ErrReadOnlyWrite, "LXP_PROGRAM_REFUSED", "write under staticcall"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			want := pcommon.LXPRevert(tc.code, tc.detail)
			require.Equal(t, want, pcommon.RevertReason(tc.err))
			require.Equal(t, want, pcommon.LXPReason(tc.err))
			reason, err := abi.UnpackRevert(want)
			require.NoError(t, err)
			require.True(t, strings.HasPrefix(reason, tc.code+": "), reason)
		})
	}

	require.NotEmpty(t, programsABI.Methods)
	for name, method := range programsABI.Methods {
		t.Run("nonpayable/"+name, func(t *testing.T) {
			require.Contains(t, []string{"nonpayable", "view"}, method.StateMutability)
			err := pcommon.ValidateNonPayable(big.NewInt(1))
			require.Error(t, err)
			reason, unpackErr := abi.UnpackRevert(pcommon.LXPReason(err))
			require.NoError(t, unpackErr)
			require.Equal(t, "LXP_PRECOMPILE_NON_PAYABLE: sending funds to a non-payable function", reason)
			require.Equal(t, pcommon.LXPRevert("LXP_PRECOMPILE_NON_PAYABLE", ""), pcommon.RevertReason(errors.New(reason)))
		})
	}
	require.NoError(t, pcommon.ValidateNonPayable(nil))
	require.NoError(t, pcommon.ValidateNonPayable(big.NewInt(0)))
}
