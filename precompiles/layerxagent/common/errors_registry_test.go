package common_test

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	common "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/eip712"
	"github.com/ethereum/go-ethereum/accounts/abi"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestAgentErrorCodesHaveRegistryRows(t *testing.T) {
	for _, code := range []common.ErrorCode{
		common.ErrInputTooShort,
		common.ErrUnknownMethod,
		common.ErrArgCount,
		common.ErrNonPayable,
		common.ErrNoContext,
		common.ErrABIDecode,
		common.ErrReadOnly,
		common.ErrDelegatecall,
		common.ErrOverflow,
		common.ErrBatchTooLarge,
		common.ErrOrobOraclePrice,
		common.ErrOrobOffsetRange,
		common.ErrClearingLengthMismatch,
		common.ErrOracleNoFeeds,
		common.ErrOracleNoConfidentFeeds,
		common.ErrOracleNotSigner,
		common.ErrOraclePrice,
		common.ErrOracleConfidence,
		common.ErrPofqLengthMismatch,
		common.ErrPofqDecayRange,
		common.ErrSchedulerJobNotFound,
		common.ErrSchedulerUnauthorized,
		common.ErrSchedulerHorizonExceeded,
		common.ErrSchedulerPastBlock,
		common.ErrSchedulerMaxJobs,
		common.ErrSchedulerDepositTooLow,
		common.ErrSchedulerInvalidGasLimit,
		common.ErrSchedulerInvalidTarget,
		common.ErrSchedulerJobInactive,
		common.ErrSchedulerEscrow,
		common.ErrSchedulerCallDataTooLarge,
		common.ErrStreamsInvalidPayee,
		common.ErrStreamsSelfPayment,
		common.ErrStreamsUnknownToken,
		common.ErrTEEUnknownFamily,
		common.ErrTEENoRoots,
		common.ErrTEEInvalidQuote,
		common.ErrTEEUntrustedChain,
		common.ErrTEEStale,
		common.ErrTEEDebugRefused,
		common.ErrTEEReportDataMismatch,
		common.ErrTEEUnauthorizedUpdate,
		common.ErrNotActive,
	} {
		t.Run(code.Error(), func(t *testing.T) {
			for _, detail := range []string{"", "agent error detail"} {
				text := revertText(t, pcommon.LXPRevert(code.Error(), detail))
				prefix, message, ok := strings.Cut(text, ": ")
				require.True(t, ok)
				require.Equal(t, code.Error(), prefix)
				require.NotEqual(t, "LXP_UNKNOWN", prefix)
				require.NotEmpty(t, message)
				if detail != "" {
					require.True(t, strings.HasSuffix(message, "; "+detail), text)
				}
			}
		})
	}
}

func TestAgentRunRevertsThroughLXPReason(t *testing.T) {
	t.Run("framework refusals keep their texts", TestFrameworkRefusalCodes)

	t.Run("inactive validation", func(t *testing.T) {
		ctx, upgrade := upgradeContext(t)
		p := common.NewPrecompile(guardedABI(t), nil, ethcommon.Address{}, "layerxagent", upgrade, "merge-plan")
		evm := evmContext(ctx)
		data, err := p.Run(evm, ethcommon.Address{}, ethcommon.Address{}, nil, nil, false, false, nil)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		require.Equal(t, "LXP_AGENT_NOT_ACTIVE: merge-plan not active at height 99", revertText(t, data))
		require.ErrorIs(t, state.GetDBImpl(evm.StateDB).GetPrecompileError(), common.ErrNotActive)
	})

	t.Run("coded executor error", func(t *testing.T) {
		ctx, upgrade := activeContext(t)
		p, err := eip712.NewPrecompile(upgrade, "merge-plan", 4, 4)
		require.NoError(t, err)
		input, err := p.Pack("domainSeparator", "over cap", "1", big.NewInt(1), ethcommon.Address{})
		require.NoError(t, err)
		evm := evmContext(ctx)
		data, err := p.Run(evm, ethcommon.Address{}, p.Address(), input, nil, true, false, nil)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		require.Equal(t, "LXP_AGENT_BATCH_TOO_LARGE: domain name or version exceeds its byte cap", revertText(t, data))
		require.ErrorIs(t, state.GetDBImpl(evm.StateDB).GetPrecompileError(), common.ErrBatchTooLarge)
	})

	t.Run("uncoded executor encoding error", func(t *testing.T) {
		ctx, upgrade := activeContext(t)
		p, err := eip712.NewPrecompile(upgrade, "merge-plan", 4, 4)
		require.NoError(t, err)
		input, err := p.Pack("hashTypedData", [32]byte{}, [32]byte{})
		require.NoError(t, err)
		data, err := p.Run(evmContext(ctx), ethcommon.Address{}, p.Address(), input, nil, true, false, nil)
		require.NoError(t, err)
		require.Len(t, data, 32)

		outputType, err := abi.NewType("uint256", "", nil)
		require.NoError(t, err)
		method := p.Methods["hashTypedData"]
		method.Outputs = abi.Arguments{{Type: outputType}}
		p.Methods["hashTypedData"] = method

		evm := evmContext(ctx)
		data, err = p.Run(evm, ethcommon.Address{}, p.Address(), input, nil, true, false, nil)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		executionErr := state.GetDBImpl(evm.StateDB).GetPrecompileError()
		require.Error(t, executionErr)
		var code common.ErrorCode
		require.False(t, errors.As(executionErr, &code))
		require.False(t, strings.HasPrefix(executionErr.Error(), "LXP_"))
		require.Contains(t, executionErr.Error(), "abi:")
		require.Equal(t, pcommon.LXPRevert("LXP_UNKNOWN", executionErr.Error()), data)
		require.True(t, strings.HasPrefix(revertText(t, data), "LXP_UNKNOWN: "))
	})
}

func TestAgentRevertErrorMapsPayCodes(t *testing.T) {
	for _, code := range []common.ErrorCode{common.ErrOverflow, common.ErrSchedulerCallDataTooLarge, common.ErrNotActive} {
		t.Run(code.Error(), func(t *testing.T) {
			for _, err := range []error{code, fmt.Errorf("agent operation: %w", code)} {
				require.True(t, common.RevertError(err) == err)
			}
		})
	}

	for _, row := range []struct {
		err  error
		code string
	}{
		{paytypes.ErrUnboundCaller, "LXP_PAY_UNBOUND_CALLER"},
		{paytypes.ErrFamilyDisabled, "LXP_MODULE_DISABLED"},
		{paytypes.ErrKernelGateMismatch, "LXP_PAY_KERNEL_GATE_MISMATCH"},
	} {
		t.Run(row.code, func(t *testing.T) {
			for _, original := range []error{row.err, fmt.Errorf("pay operation: %w", row.err)} {
				mapped := common.RevertError(original)
				require.EqualError(t, mapped, row.code+": "+original.Error())
				require.ErrorIs(t, mapped, row.err)
				require.True(t, errors.Unwrap(mapped) == original)
				require.Equal(t, mapped.Error(), revertText(t, pcommon.LXPReason(mapped)))
			}
			combined := errors.Join(common.ErrOverflow, row.err)
			require.True(t, common.RevertError(combined) == combined)
		})
	}

	require.Nil(t, common.RevertError(nil))
	for _, original := range []error{errors.New("unclassified failure"), fmt.Errorf("operation: %w", errors.New("unclassified failure"))} {
		require.True(t, common.RevertError(original) == original)
		require.Equal(t, pcommon.LXPRevert("LXP_UNKNOWN", original.Error()), pcommon.LXPReason(common.RevertError(original)))
	}
}
