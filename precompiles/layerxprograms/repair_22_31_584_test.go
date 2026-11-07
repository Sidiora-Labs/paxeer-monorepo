package layerxprograms_test

import (
	"crypto/sha256"
	"testing"

	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestRepair22_31_584(t *testing.T) {
	t.Run("distinct fixture request", func(t *testing.T) {
		h, args, request := gasSandboxExecuteFixture(t)
		decoded := decodeSandboxLeaseRequest(t, request)
		program := sha256.Sum256([]byte(t.Name()))
		recorded := decodeSandboxLeaseRequest(t, recordedSandboxLeaseRequest(t))
		require.NotEqual(t, recorded.Program, program)
		require.Equal(t, program, decoded.Program)
		record, err := h.host.ReadProgram(h.stateDB.Ctx(), program)
		require.NoError(t, err)
		require.True(t, record.Active)
		require.Equal(t, record.CodeHash, decoded.CodeHash)
		fee, err := h.host.ReadFeeSchedule(h.stateDB.Ctx())
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(fee), 97)
		require.Equal(t, [32]byte(fee[65:97]), decoded.Asset)
		require.Equal(t, args[4].([32]byte), decoded.Asset)
		require.Equal(t, sha256.Sum256([]byte(t.Name()+"/lease")), decoded.LeaseID)
		require.Equal(t, sha256.Sum256([]byte(t.Name()+"/request-activity")), decoded.ActivityID)
		require.Equal(t, sha256.Sum256([]byte(t.Name()+"/request-receipt")), decoded.ReceiptDigest)
	})
	t.Run("recording request codec round trip", TestProgramsSandboxExecuteEmitsUsage)
	t.Run("committed sandboxExecute gas", func(t *testing.T) {
		h, input := gasWriteFixture(t, "sandboxExecute")
		row := lp.GasTable["sandboxExecute"]
		require.True(t, row.Write)
		require.Equal(t, uint64(50000), row.Surcharge)
		required := expectedGas(d2Table["sandboxExecute"], len(input[4:]))
		require.Equal(t, required, row.Gas(input[4:]))
		require.Equal(t, required, h.precompile.RequiredGas(input))
		const supplied uint64 = 20000000
		require.Less(t, required, supplied)
		output, remaining, err := h.evm.Call(h.caller, h.precompile.Address(), input, supplied, uint256.NewInt(0))
		require.NoError(t, err, "sandboxExecute returned %x", output)
		require.NoError(t, h.stateDB.GetPrecompileError())
		require.Equal(t, required, supplied-remaining)
		values, err := lp.ProgramsABI.Unpack("sandboxExecute", output)
		require.NoError(t, err)
		require.Equal(t, int32(0), values[0])
		require.Equal(t, uint8(1), values[1])
	})
}
