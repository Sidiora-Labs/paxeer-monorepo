package layerxprograms_test

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"testing"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

// These recordings come from the live native sandboxExecute EVM path in
// TestRepairM_213_StorageEnvironmentResponse, encoded with the production ABI
// and transaction-result codecs alongside the host's kind-2 usage ledger.
//
//go:embed testdata/repair_m213_execute_result.hex
var repairM213ExecuteResultHex string

//go:embed testdata/repair_m213_usage_ledger.hex
var repairM213UsageLedgerHex string

func TestRepairM_213_StorageEnvironmentRecordedConsumer(t *testing.T) {
	result, err := hex.DecodeString(strings.TrimSpace(repairM213ExecuteResultHex))
	require.NoError(t, err)
	var messages sdk.TxMsgData
	require.NoError(t, messages.Unmarshal(result))
	require.Len(t, messages.Data, 1)
	var execution evmtypes.MsgEVMTransactionResponse
	require.NoError(t, execution.Unmarshal(messages.Data[0].Data))
	require.Empty(t, execution.VmError)
	outputs := lp.ProgramsABI.Methods["sandboxExecute"].Outputs
	frameValues, err := outputs.Unpack(execution.ReturnData)
	require.NoError(t, err)
	encoded, err := outputs.Pack(frameValues...)
	require.NoError(t, err)
	require.Equal(t, execution.ReturnData, encoded, "the recorded native return must match the production ABI encoder byte for byte")
	frame := frameValues[2].([]byte)
	require.True(t, bytes.HasPrefix(frame, []byte("LXP/programs/terminal-applied-legs/v1\x00")))
	ledger, err := hex.DecodeString(strings.TrimSpace(repairM213UsageLedgerHex))
	require.NoError(t, err)
	const ledgerDomain = "LayerX/programs/sandbox/usage-ledger/v2\x00"
	require.Equal(t, ledgerDomain, string(ledger[:len(ledgerDomain)]))
	offset := len(ledgerDomain)
	require.Equal(t, uint64(1), binary.BigEndian.Uint64(ledger[offset:offset+8]))
	offset += 8 + 16 + 32
	require.Equal(t, byte(1), ledger[offset])
	offset++
	length := int(binary.BigEndian.Uint32(ledger[offset : offset+4]))
	offset += 4
	require.Equal(t, offset+length+32, len(ledger))
	canonical := ledger[offset : offset+length]
	digest := sha256.Sum256(canonical)
	require.Equal(t, digest[:], ledger[offset+length:])
	root := [32]byte(canonical[len(canonical)-32:])
	decoded, err := lp.DecodeProgramExecutionResult(execution.ReturnData, 64, root)
	require.NoError(t, err)
	require.Zero(t, decoded.TerminalResult)
	require.Equal(t, uint8(1), decoded.TerminalKind)
	require.Empty(t, decoded.GuestResponse, "the recorded Wasm returns zero without response_write")
	require.Equal(t, ledger[offset:], decoded.SandboxUsageReceipt)
	const sandboxDomain = "LXP/programs/sandbox-settlement/v1\x00"
	require.True(t, bytes.HasPrefix(decoded.Execution, []byte(sandboxDomain)))
	receiptOffset := bytes.Index(frame, canonical)
	require.Positive(t, receiptOffset)
	for _, at := range []int{receiptOffset, receiptOffset + len(canonical) - 1, receiptOffset + len(canonical)} {
		bad := bytes.Clone(frame)
		bad[at] ^= 1
		output, err := lp.ProgramsABI.Methods["sandboxExecute"].Outputs.Pack(int32(0), uint8(1), bad)
		require.NoError(t, err)
		_, err = lp.DecodeProgramExecutionResult(output, 64, root)
		require.Error(t, err, "sandbox attachment mutation at %d", at)
	}
	const usageDomain = "LayerX/programs/sandbox/usage-receipt/v4\x00"
	kindOffset := len(usageDomain) + 32 + 8 + 8 + 7*32
	usageOffset := kindOffset + 1 + 32
	pricesOffset := usageOffset + 6*8 + 3*16 + 8*8 + 4
	for _, at := range []int{kindOffset, kindOffset + 1, usageOffset, usageOffset + 6*8, usageOffset + 6*8 + 16, pricesOffset, len(canonical) - 64} {
		bad := bytes.Clone(frame)
		bad[receiptOffset+at] ^= 1
		// The canonical attachment must agree with the included fee leg and
		// inner execution even without a redundant digest in the terminal frame.
		output, err := lp.ProgramsABI.Methods["sandboxExecute"].Outputs.Pack(int32(0), uint8(1), bad)
		require.NoError(t, err)
		_, err = lp.DecodeProgramExecutionResult(output, 64, root)
		require.Error(t, err, "sandbox accounting or identity mutation at %d", at)
	}
	// The former receipt-plus-digest terminal attachment is noncanonical after
	// the runtime has authenticated and removed the native transport digest.
	legacyFrame := bytes.Clone(frame)
	legacyFrame = append(legacyFrame[:receiptOffset+len(canonical)], append(bytes.Clone(digest[:]), legacyFrame[receiptOffset+len(canonical):]...)...)
	binary.BigEndian.PutUint32(legacyFrame[receiptOffset-4:receiptOffset], uint32(len(canonical)+32))
	executionLengthOffset := len("LXP/programs/terminal-applied-legs/v1\x00")
	binary.BigEndian.PutUint32(legacyFrame[executionLengthOffset:executionLengthOffset+4], uint32(len(decoded.Execution)+32))
	legacyOutput, err := outputs.Pack(int32(0), uint8(1), legacyFrame)
	require.NoError(t, err)
	_, err = lp.DecodeProgramExecutionResult(legacyOutput, 64, root)
	require.Error(t, err, "the terminal attachment must contain only canonical receipt bytes")
	wrongRoot := root
	wrongRoot[0] ^= 1
	_, err = lp.DecodeProgramExecutionResult(execution.ReturnData, 64, wrongRoot)
	require.Error(t, err)
}

func TestRepairM_213_StorageEnvironmentResponse(t *testing.T) {
	TestProgramsSandboxExecuteEmitsUsage(t)
	t.Run("zero asset refusal", TestProgramsSandboxExecuteZeroAssetReverts)
}

func TestRepairM_213_SimulationResponseOracle(t *testing.T) {
	TestProgramsSimulateCallMatchesCallOutput(t)
	TestProgramsSimulateCallLeavesStateUnchanged(t)
}

func TestRepairM_213_PublicProof(t *testing.T) {
	proofProgramsExecutionConsumer(t)
}

func TestRepairM_213_SimulationParity(t *testing.T) {
	TestProgramsSimulateMatchesCallForEveryFixtureEntrypoint(t)
}

func TestRepairM_213_Frozen(t *testing.T) {
	lp.RunRepairM213Frozen(t)
}

func TestRepairM_213_NativeAssetResolver(t *testing.T) {
	lp.RunRepairM213NativeResolver(t)
}

func TestRepairM_213_NativeWebFixture(t *testing.T) {
	lp.RunRepairM213NativeWeb(t)
}
