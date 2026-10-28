package layerxprograms_test

import (
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func tombstoneSubmit(t *testing.T, h *harness, ctx sdk.Context, methodName string, activityType uint32, payload []byte) (lxphost.Receipt, error) {
	t.Helper()
	method := lp.ProgramsABI.Methods[methodName]
	return lp.Submit(ctx, h.executor, lp.NewCallContext(h.caller, &method), activityType, payload, [16]byte{})
}

func tombstoneProgram(t *testing.T) (*harness, [32]byte) {
	t.Helper()
	h := newHarness(t)
	ctx := h.stateDB.Ctx()
	authority, _, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	metering := binary.BigEndian.AppendUint32([]byte("LXMR1"), 1)
	for _, coefficient := range []uint64{1, 1, 1, 1, 1, 8, 8, 64, 8} {
		metering = binary.BigEndian.AppendUint64(metering, coefficient)
	}
	metering = binary.BigEndian.AppendUint64(metering, 1)
	metering = append(metering, 1)
	metering = append(metering, authority.Principal[:]...)
	moduleKey := func(name []byte) []byte { return append([]byte{1, 0, 9}, name...) }
	store := ctx.KVStore(h.key)
	store.Set(moduleKey([]byte("progmet/active/v1")), metering)
	store.Set(moduleKey(binary.BigEndian.AppendUint32([]byte("progmet/history/v1/"), 1)), metering)
	wasm := []byte{
		0, 97, 115, 109, 1, 0, 0, 0, 1, 12, 2, 96, 2, 127, 127, 1, 127, 96, 1, 127, 1, 127,
		3, 3, 2, 0, 1, 5, 3, 1, 0, 1, 7, 41, 3,
		11, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'c', 'a', 'l', 'l', 0, 0,
		14, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'r', 'e', 's', 'e', 'r', 'v', 'e', 0, 1,
		6, 'm', 'e', 'm', 'o', 'r', 'y', 2, 0, 10, 11, 2, 4, 0, 65, 0, 11, 4, 0, 65, 0, 11,
	}
	programID := sha256.Sum256([]byte("tombstone program"))
	payload, err := lp.EncodeDeployPayload(programID, 2, 1, authority.Principal, sha256.Sum256(wasm), nil, wasm)
	require.NoError(t, err)
	receipt, err := tombstoneSubmit(t, h, ctx, "deploy", 0x00090001, payload)
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	record, err := h.host.ReadProgram(ctx, programID)
	require.NoError(t, err)
	require.Equal(t, uint16(2), record.ABIVersion)
	require.True(t, record.Active)
	return h, programID
}

func TestProgramsTombstoneAfterDeadline(t *testing.T) {
	for _, tc := range []struct {
		name     string
		deadline uint64
		advance  int
	}{
		{name: "after deadline", deadline: 5, advance: 3},
		{name: "kernel does not require deadline", deadline: 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, programID := tombstoneProgram(t)
			ctx := h.stateDB.Ctx()
			payload, err := lp.EncodeDeprecatePayload(programID, programID, tc.deadline)
			require.NoError(t, err)
			deprecated, err := tombstoneSubmit(t, h, ctx, "deprecate", 0x00090007, payload)
			require.NoError(t, err)
			require.Len(t, deprecated.Events, 1)
			require.Equal(t, uint16(10), deprecated.Events[0].Code)
			for i := 0; i < tc.advance; i++ {
				read, err := lp.EncodeRegistryReadPayload(programID, nil)
				require.NoError(t, err)
				_, err = tombstoneSubmit(t, h, ctx, "registryRead", 0x00090004, read)
				require.NoError(t, err)
			}
			payload, err = lp.EncodeTombstonePayload(programID)
			require.NoError(t, err)
			require.Len(t, payload, 33)
			require.Equal(t, programID[:], payload[:32])
			require.Equal(t, byte(3), payload[32])
			reference, _ := ctx.CacheContext()
			expected, err := tombstoneSubmit(t, h, reference, "tombstone", 0x00090007, payload)
			require.NoError(t, err)
			require.Zero(t, expected.ResultCode)
			require.Len(t, expected.Events, 1)
			require.Equal(t, uint16(11), expected.Events[0].Code)
			if tc.advance > 0 {
				require.Greater(t, expected.GlobalSequence, tc.deadline)
			} else {
				require.Less(t, expected.GlobalSequence, tc.deadline)
			}
			input, err := lp.ProgramsABI.Pack("tombstone", programID)
			require.NoError(t, err)
			output, _, err := h.evm.Call(h.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
			require.NoError(t, err)
			require.Empty(t, output)
			require.Equal(t, h.dump(reference), h.dump(h.stateDB.Ctx()))
			windDown, err := h.host.ReadWindDown(h.stateDB.Ctx(), programID)
			require.NoError(t, err)
			require.Equal(t, uint8(3), windDown.Status)
			require.Equal(t, tc.deadline, windDown.Deadline)
			require.Equal(t, expected.GlobalSequence, windDown.EffectiveSequence)
			record, err := h.host.ReadProgram(h.stateDB.Ctx(), programID)
			require.NoError(t, err)
			require.False(t, record.Active)
			logs := h.stateDB.GetAllLogs()
			require.Len(t, logs, 1)
			definition := lp.ProgramsABI.Events["ProgramTombstoned"]
			require.Equal(t, lp.Address, logs[0].Address)
			require.Equal(t, []common.Hash{definition.ID, common.Hash(programID)}, logs[0].Topics)
			decoded, err := lp.DecodeProgramsReceiptEvent(0x00090007, expected.Events[0])
			require.NoError(t, err)
			values, err := definition.Inputs.NonIndexed().Unpack(logs[0].Data)
			require.NoError(t, err)
			require.Equal(t, decoded.Values(), values)
			require.Equal(t, uint8(2), values[0])
			require.Equal(t, uint8(3), values[1])
			require.Equal(t, programID, values[3])
			require.Equal(t, tc.deadline, values[4])
			require.Equal(t, windDown.EffectiveSequence, values[5])
		})
	}
}

func TestProgramsTombstoneNotDeprecatedReverts(t *testing.T) {
	h, programID := tombstoneProgram(t)
	ctx := h.stateDB.Ctx()
	before := h.dump(ctx)
	payload, err := lp.EncodeTombstonePayload(programID)
	require.NoError(t, err)
	reference, _ := ctx.CacheContext()
	receipt, err := tombstoneSubmit(t, h, reference, "tombstone", 0x00090007, payload)
	require.Error(t, err)
	require.Contains(t, err.Error(), "LXP_PROGRAM_REFUSED")
	require.ErrorIs(t, err, receipt.ResultCode)
	require.NotZero(t, receipt.ResultCode)
	require.Empty(t, receipt.Events)
	input, err := lp.ProgramsABI.Pack("tombstone", programID)
	require.NoError(t, err)
	output, _, err := h.evm.Call(h.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, err := abi.UnpackRevert(output)
	require.NoError(t, err)
	require.Equal(t, lp.KernelError(receipt.ResultCode, "tombstone").Error(), reason)
	require.ErrorIs(t, h.stateDB.GetPrecompileError(), receipt.ResultCode)
	require.Empty(t, h.stateDB.GetAllLogs())
	require.Equal(t, before, h.dump(h.stateDB.Ctx()))
	_, err = h.host.ReadWindDown(h.stateDB.Ctx(), programID)
	require.ErrorIs(t, err, lxphost.Code(-7))
	record, err := h.host.ReadProgram(h.stateDB.Ctx(), programID)
	require.NoError(t, err)
	require.True(t, record.Active)
}
