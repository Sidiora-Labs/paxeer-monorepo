// Proves: common.ErrorCode, oracle.Aggregate, eip712.Eip712Precompile -> common.RevertError, common.Precompile.Run, pcommon.LXPReason/LXPRevert, state.DBImpl -> errors.Is/As and abi.UnpackRevert.
//
// The test is the first consumer of the errors anchor's exported Go and EVM
// surfaces. All 43 code producers cross the registry encoder; executable
// producers additionally cross the runtime encoder and the stored precompile
// error. Every producer runs twice, with fresh EVM state and no cached payload.
// The anchor has no persistent state to reopen. Malformed payloads are damaged
// copies of real producer output and are refused by the actual ABI decoder.
//
// Repairs precompiles/layerxagent/common/errors.go: RevertError used to prepend
// a pay code again on every mapping of an already mapped error. The proof
// requires repeated mapping to preserve both wire text and error identity.
// ErrClearingLengthMismatch has no emitting branch in
// precompiles/layerxagent/clearing/clearing.go: ComputeClearing accepts separate
// tuple slices, rather than parallel arrays. Its exported ErrorCode is the
// producer under this contract and must still reach the registry consumer.
package common_test

import (
	"bytes"
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
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/oracle"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func proofErrorsDecode(t *testing.T, data []byte) string {
	t.Helper()
	require.GreaterOrEqual(t, len(data), 4)
	require.Equal(t, []byte{0x08, 0xc3, 0x79, 0xa0}, data[:4])
	text, err := abi.UnpackRevert(data)
	require.NoError(t, err)
	return text
}

func proofErrorsRefuseMalformed(t *testing.T, data []byte) {
	t.Helper()
	badSelector := bytes.Clone(data)
	badSelector[0] ^= 0xff
	badOffset := bytes.Clone(data)
	for i := 4; i < 36; i++ {
		badOffset[i] = 0xff
	}
	for _, damaged := range [][]byte{data[:3], data[:36], badSelector, badOffset} {
		text, err := abi.UnpackRevert(damaged)
		require.Error(t, err)
		require.Empty(t, text)
	}
}

func TestProofErrorsRegistryTransport(t *testing.T) {
	// The literal tokens are the consumer's frozen inventory, independent of
	// the producer's Error method and the registry's own lookup table.
	for _, row := range []struct {
		code common.ErrorCode
		wire string
	}{
		{common.ErrInputTooShort, "LXP_PRECOMPILE_INPUT_TOO_SHORT"},
		{common.ErrUnknownMethod, "LXP_PRECOMPILE_UNKNOWN_METHOD"},
		{common.ErrArgCount, "LXP_PRECOMPILE_ARG_COUNT"},
		{common.ErrNonPayable, "LXP_PRECOMPILE_NON_PAYABLE"},
		{common.ErrNoContext, "LXP_PRECOMPILE_NO_CONTEXT"},
		{common.ErrABIDecode, "LXP_AGENT_ABI_DECODE"},
		{common.ErrReadOnly, "LXP_AGENT_READ_ONLY"},
		{common.ErrDelegatecall, "LXP_AGENT_DELEGATECALL"},
		{common.ErrOverflow, "LXP_AGENT_OVERFLOW"},
		{common.ErrBatchTooLarge, "LXP_AGENT_BATCH_TOO_LARGE"},
		{common.ErrOrobOraclePrice, "LXP_AGENT_OROB_ORACLE_PRICE"},
		{common.ErrOrobOffsetRange, "LXP_AGENT_OROB_OFFSET_RANGE"},
		{common.ErrClearingLengthMismatch, "LXP_AGENT_CLEARING_LENGTH_MISMATCH"},
		{common.ErrOracleNoFeeds, "LXP_AGENT_ORACLE_NO_FEEDS"},
		{common.ErrOracleNoConfidentFeeds, "LXP_AGENT_ORACLE_NO_CONFIDENT_FEEDS"},
		{common.ErrOracleNotSigner, "LXP_AGENT_ORACLE_NOT_SIGNER"},
		{common.ErrOraclePrice, "LXP_AGENT_ORACLE_PRICE"},
		{common.ErrOracleConfidence, "LXP_AGENT_ORACLE_CONFIDENCE"},
		{common.ErrPofqLengthMismatch, "LXP_AGENT_POFQ_LENGTH_MISMATCH"},
		{common.ErrPofqDecayRange, "LXP_AGENT_POFQ_DECAY_RANGE"},
		{common.ErrSchedulerJobNotFound, "LXP_AGENT_SCHEDULER_JOB_NOT_FOUND"},
		{common.ErrSchedulerUnauthorized, "LXP_AGENT_SCHEDULER_UNAUTHORIZED"},
		{common.ErrSchedulerHorizonExceeded, "LXP_AGENT_SCHEDULER_HORIZON_EXCEEDED"},
		{common.ErrSchedulerPastBlock, "LXP_AGENT_SCHEDULER_PAST_BLOCK"},
		{common.ErrSchedulerMaxJobs, "LXP_AGENT_SCHEDULER_MAX_JOBS"},
		{common.ErrSchedulerDepositTooLow, "LXP_AGENT_SCHEDULER_DEPOSIT_TOO_LOW"},
		{common.ErrSchedulerInvalidGasLimit, "LXP_AGENT_SCHEDULER_INVALID_GAS_LIMIT"},
		{common.ErrSchedulerInvalidTarget, "LXP_AGENT_SCHEDULER_INVALID_TARGET"},
		{common.ErrSchedulerJobInactive, "LXP_AGENT_SCHEDULER_JOB_INACTIVE"},
		{common.ErrSchedulerEscrow, "LXP_AGENT_SCHEDULER_ESCROW"},
		{common.ErrSchedulerCallDataTooLarge, "LXP_AGENT_SCHEDULER_CALLDATA_TOO_LARGE"},
		{common.ErrStreamsInvalidPayee, "LXP_AGENT_STREAMS_INVALID_PAYEE"},
		{common.ErrStreamsSelfPayment, "LXP_AGENT_STREAMS_SELF_PAYMENT"},
		{common.ErrStreamsUnknownToken, "LXP_AGENT_STREAMS_UNKNOWN_TOKEN"},
		{common.ErrTEEUnknownFamily, "LXP_AGENT_TEE_UNKNOWN_FAMILY"},
		{common.ErrTEENoRoots, "LXP_AGENT_TEE_NO_ROOTS"},
		{common.ErrTEEInvalidQuote, "LXP_AGENT_TEE_INVALID_QUOTE"},
		{common.ErrTEEUntrustedChain, "LXP_AGENT_TEE_UNTRUSTED_CHAIN"},
		{common.ErrTEEStale, "LXP_AGENT_TEE_STALE"},
		{common.ErrTEEDebugRefused, "LXP_AGENT_TEE_DEBUG_REFUSED"},
		{common.ErrTEEReportDataMismatch, "LXP_AGENT_TEE_REPORT_DATA_MISMATCH"},
		{common.ErrTEEUnauthorizedUpdate, "LXP_AGENT_TEE_UNAUTHORIZED_UPDATE"},
		{common.ErrNotActive, "LXP_AGENT_NOT_ACTIVE"},
	} {
		t.Run(row.wire, func(t *testing.T) {
			for attempt := 0; attempt < 2; attempt++ {
				produced := common.RevertError(row.code)
				require.ErrorIs(t, produced, row.code)
				var observed common.ErrorCode
				require.ErrorAs(t, produced, &observed)
				require.Equal(t, row.wire, observed.Error())
				// A different detail on each crossing proves fresh encoding.
				detail := fmt.Sprintf("crossing %d", attempt)
				data := pcommon.LXPRevert(produced.Error(), detail)
				text := proofErrorsDecode(t, data)
				code, message, ok := strings.Cut(text, ": ")
				require.True(t, ok)
				require.Equal(t, row.wire, code)
				require.NotEmpty(t, strings.TrimSuffix(message, "; "+detail))
				require.True(t, strings.HasSuffix(message, "; "+detail), text)
				proofErrorsRefuseMalformed(t, data)
			}
		})
	}
}

func TestProofErrorsProducerRefusals(t *testing.T) {
	ctx, upgrade := activeContext(t)
	for _, row := range []struct {
		name    string
		produce func() error
		code    common.ErrorCode
		text    string
	}{
		{"delegatecall", func() error { return common.GuardWrite(ctx, false, true) }, common.ErrDelegatecall, "LXP_AGENT_DELEGATECALL: cannot delegatecall an agent function"},
		{"inactive", func() error {
			return common.HeightGate(common.WithHeightGate(ctx.WithBlockHeight(99), upgrade, "merge-plan"))
		}, common.ErrNotActive, "LXP_AGENT_NOT_ACTIVE: merge-plan not active at height 99"},
		{"confidence", func() error {
			_, _, err := oracle.Aggregate([]oracle.Feed{{Price: big.NewInt(10)}}, 4)
			return err
		}, common.ErrOracleConfidence, "LXP_AGENT_ORACLE_CONFIDENCE: invalid confidence"},
		{"no confident feeds", func() error {
			_, _, err := oracle.Aggregate([]oracle.Feed{{Price: big.NewInt(10), Confidence: new(big.Int)}}, 4)
			return err
		}, common.ErrOracleNoConfidentFeeds, "LXP_AGENT_ORACLE_NO_CONFIDENT_FEEDS: no feeds with non-zero confidence"},
	} {
		t.Run(row.name, func(t *testing.T) {
			for attempt := 0; attempt < 2; attempt++ {
				produced := row.produce()
				require.ErrorIs(t, produced, row.code)
				mapped := common.RevertError(produced)
				require.True(t, produced == mapped)
				evm := evmContext(ctx)
				pcommon.HandlePrecompileError(mapped, evm, "proof_errors")
				require.ErrorIs(t, state.GetDBImpl(evm.StateDB).GetPrecompileError(), row.code)
				data := pcommon.LXPReason(mapped)
				require.Equal(t, row.text, proofErrorsDecode(t, data))
				proofErrorsRefuseMalformed(t, data)
			}
		})
	}
	for attempt := 0; attempt < 2; attempt++ {
		price, confidence, err := oracle.Aggregate([]oracle.Feed{{Price: big.NewInt(10 + int64(attempt)), Confidence: big.NewInt(2)}}, 4)
		require.NoError(t, err)
		require.Nil(t, common.RevertError(err))
		require.Equal(t, big.NewInt(10+int64(attempt)), price)
		require.Equal(t, big.NewInt(2), confidence)
		require.NoError(t, common.GuardWrite(ctx, false, false))
		require.NoError(t, common.HeightGate(common.WithHeightGate(ctx, upgrade, "merge-plan")))
	}
}

func TestProofErrorsPrecompileConsumer(t *testing.T) {
	ctx, upgrade := activeContext(t)
	p, err := eip712.NewPrecompile(upgrade, "merge-plan", 4, 4)
	require.NoError(t, err)
	for attempt := 0; attempt < 2; attempt++ {
		domain, hash := [32]byte{byte(attempt + 1)}, [32]byte{0x81}
		input, err := p.Pack(eip712.HashTypedDataMethod, domain, hash)
		require.NoError(t, err)
		evm := evmContext(ctx)
		data, err := p.Run(evm, ethcommon.Address{}, p.Address(), input, nil, true, false, nil)
		require.NoError(t, err)
		require.NoError(t, state.GetDBImpl(evm.StateDB).GetPrecompileError())
		outputs, err := p.Methods[eip712.HashTypedDataMethod].Outputs.Unpack(data)
		require.NoError(t, err)
		want := crypto.Keccak256Hash([]byte{0x19, 0x01}, domain[:], hash[:])
		require.Equal(t, []interface{}{[32]byte(want)}, outputs)

		overCap, err := p.Pack(eip712.DomainSeparatorMethod, "over cap", "1", big.NewInt(1), ethcommon.Address{})
		require.NoError(t, err)
		for _, row := range []struct {
			ctx   sdk.Context
			input []byte
			code  common.ErrorCode
			text  string
		}{
			{ctx, overCap, common.ErrBatchTooLarge, "LXP_AGENT_BATCH_TOO_LARGE: domain name or version exceeds its byte cap"},
			{ctx, input[:4], common.ErrABIDecode, "LXP_AGENT_ABI_DECODE: "},
			{ctx.WithBlockHeight(99), input, common.ErrNotActive, "LXP_AGENT_NOT_ACTIVE: merge-plan not active at height 99"},
		} {
			evm := evmContext(row.ctx)
			data, err := p.Run(evm, ethcommon.Address{}, p.Address(), row.input, nil, true, false, nil)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			stored := state.GetDBImpl(evm.StateDB).GetPrecompileError()
			require.ErrorIs(t, stored, row.code)
			require.NotErrorIs(t, stored, common.ErrUnknownMethod)
			var code common.ErrorCode
			require.ErrorAs(t, stored, &code)
			require.Equal(t, row.code, code)
			text := proofErrorsDecode(t, data)
			if row.code == common.ErrABIDecode {
				require.True(t, strings.HasPrefix(text, row.text), text)
				require.NotEmpty(t, strings.TrimPrefix(text, row.text))
			} else {
				require.Equal(t, row.text, text)
			}
			require.Equal(t, data, pcommon.LXPReason(common.RevertError(stored)))
			proofErrorsRefuseMalformed(t, data)
		}
	}
}

func TestProofErrorsPayMapping(t *testing.T) {
	for _, row := range []struct {
		cause error
		wire  string
	}{
		{paytypes.ErrUnboundCaller, "LXP_PAY_UNBOUND_CALLER"},
		{paytypes.ErrFamilyDisabled, "LXP_MODULE_DISABLED"},
		{paytypes.ErrKernelGateMismatch, "LXP_PAY_KERNEL_GATE_MISMATCH"},
	} {
		t.Run(row.wire, func(t *testing.T) {
			for attempt := 0; attempt < 2; attempt++ {
				produced := fmt.Errorf("pay crossing %d: %w", attempt, row.cause)
				mapped := common.RevertError(produced)
				require.ErrorIs(t, mapped, row.cause)
				require.True(t, errors.Unwrap(mapped) == produced)
				require.EqualError(t, mapped, row.wire+": "+produced.Error())
				require.True(t, common.RevertError(mapped) == mapped)
				data := pcommon.LXPReason(mapped)
				require.Equal(t, mapped.Error(), proofErrorsDecode(t, data))
				proofErrorsRefuseMalformed(t, data)
			}
		})
	}
	unknown := errors.New("unclassified producer refusal")
	require.True(t, common.RevertError(unknown) == unknown)
	data := pcommon.LXPReason(common.RevertError(unknown))
	require.Equal(t, "LXP_UNKNOWN: unknown; unclassified producer refusal", proofErrorsDecode(t, data))
}
