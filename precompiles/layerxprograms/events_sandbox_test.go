package layerxprograms

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func sandboxEventState(t *testing.T) (*vm.EVM, *state.DBImpl) {
	t.Helper()
	app := testkeeper.EVMTestApp
	ctx, _ := app.GetContextForDeliverTx(nil).CacheContext()
	db := state.NewDBImpl(ctx, &app.EvmKeeper, false)
	t.Cleanup(db.Cleanup)
	return &vm.EVM{StateDB: db}, db
}

func sandboxEventReceipt(t *testing.T, activityID [32]byte, effects ...codec.Effect) lxphost.Receipt {
	t.Helper()
	var encoded []byte
	u16 := func(value uint16) { encoded = binary.BigEndian.AppendUint16(encoded, value) }
	u32 := func(value uint32) { encoded = binary.BigEndian.AppendUint32(encoded, value) }
	u64 := func(value uint64) { encoded = binary.BigEndian.AppendUint64(encoded, value) }
	blob := func(value []byte) {
		u32(uint32(len(value)))
		encoded = append(encoded, value...)
	}
	zero := make([]byte, 32)
	u16(1)
	u16(0x5201)
	u16(1)
	blob(activityID[:])
	u64(7)
	blob(zero)
	blob(bytes.Repeat([]byte{0x44}, 32))
	blob(zero)
	u32(0)
	u32(uint32(len(effects)))
	for i, effect := range effects {
		u16(effect.ModuleID)
		u16(uint16(i))
		u16(effect.EventType)
		encoded = append(encoded, effect.Kind, 0)
		blob(effect.TransferSetRoot[:])
		blob(effect.Body)
	}
	encoded = append(encoded, make([]byte, 16)...)
	blob(zero)
	u16(9)
	u32(1)
	u32(1)
	encoded = append(encoded, 0)
	blob(zero)
	encoded = append(encoded, make([]byte, 16)...)
	blob(zero)
	encoded = append(encoded, make([]byte, 32)...)
	u64(0)
	blob(zero)
	encoded = append(encoded, make([]byte, 32)...)
	blob(zero)
	blob(zero)
	blob(zero)
	u64(10)
	encoded = append(encoded, 0)
	decoded, err := codec.DecodeReceipt(encoded)
	require.NoError(t, err)
	require.Len(t, decoded.Effects, len(effects))
	return lxphost.Receipt{
		ResultCode: lxphost.Code(decoded.ResultCode), GlobalSequence: decoded.GlobalSequence,
		Timestamp: decoded.Timestamp, PreviousStateRoot: decoded.PreviousStateRoot,
		ResultingStateRoot: decoded.ResultingStateRoot, Encoded: encoded,
	}
}

func sandboxKernelEvent(code uint16, body []byte) codec.Effect {
	return codec.Effect{ModuleID: 9, Kind: 3, EventType: code, Body: body}
}

func assertSandboxLog(t *testing.T, log *ethtypes.Log, signature string, topic common.Hash, index uint) {
	t.Helper()
	require.Equal(t, common.HexToAddress("0x101D"), log.Address)
	require.Equal(t, []common.Hash{common.HexToHash(signature), topic}, log.Topics)
	require.Equal(t, index, log.Index)
}

func TestProgramsSandboxUsageReceiptLog(t *testing.T) {
	activityID := [32]byte{0x19, 0x56, 0x01}
	const sequence uint64 = 0xfedcba9876543210
	usage := make([]byte, 237)
	for i := range usage {
		usage[i] = byte(i)
	}
	frames := make([][]byte, 2)
	var effects []codec.Effect
	for i, chunk := range [][]byte{usage[:204], usage[204:]} {
		frame := make([]byte, 52+len(chunk))
		copy(frame, "LXUR")
		copy(frame[4:36], activityID[:])
		binary.BigEndian.PutUint64(frame[36:44], sequence)
		binary.BigEndian.PutUint16(frame[44:46], uint16(i))
		binary.BigEndian.PutUint16(frame[46:48], 2)
		binary.BigEndian.PutUint32(frame[48:52], uint32(len(usage)))
		copy(frame[52:], chunk)
		frames[i] = frame
		effects = append(effects, sandboxKernelEvent(0x0909, frame))
	}
	evm, db := sandboxEventState(t)
	receipt := sandboxEventReceipt(t, activityID, effects...)
	require.NoError(t, emitSandboxEvents(evm, receipt))
	logs := db.GetAllLogs()
	require.Len(t, logs, 2)
	var reconstructed []byte
	for i, log := range logs {
		assertSandboxLog(t, log, "0x5c8386df0628270b0285d3f16695d6147ce4a66e048cbef0d572dae8ac4d2ba7", common.Hash(activityID), uint(i))
		values, err := programsABI.Events["SandboxUsageReceipt"].Inputs.NonIndexed().Unpack(log.Data)
		require.NoError(t, err)
		require.Equal(t, []interface{}{sequence, uint16(i), uint16(2), uint32(len(usage)), frames[i][52:]}, values)
		require.Equal(t, uint64(160), binary.BigEndian.Uint64(log.Data[152:160]))
		reconstructed = append(reconstructed, values[4].([]byte)...)
	}
	require.Equal(t, usage, reconstructed)
	require.Equal(t, frames[0], effects[0].Body)

	for _, tc := range []struct {
		name string
		edit func([]byte)
	}{
		{"magic", func(b []byte) { b[0] = 'X' }},
		{"activity", func(b []byte) { b[4] ^= 1 }},
		{"index", func(b []byte) { binary.BigEndian.PutUint16(b[44:46], 2) }},
		{"count", func(b []byte) { binary.BigEndian.PutUint16(b[46:48], 0) }},
		{"length", func(b []byte) { binary.BigEndian.PutUint32(b[48:52], 4097) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := bytes.Clone(frames[1])
			tc.edit(bad)
			evm, db := sandboxEventState(t)
			receipt := sandboxEventReceipt(t, activityID, effects[0], sandboxKernelEvent(0x0909, bad))
			require.Error(t, emitSandboxEvents(evm, receipt))
			require.Empty(t, db.GetAllLogs())
		})
	}
	for _, body := range [][]byte{nil, frames[0][:51], frames[0][:255]} {
		evm, db := sandboxEventState(t)
		require.Error(t, emitSandboxEvents(evm, sandboxEventReceipt(t, activityID, sandboxKernelEvent(0x0909, body))))
		require.Empty(t, db.GetAllLogs())
	}
	require.ErrorContains(t, emitSandboxEvents(nil, receipt), "EVM is nil")
	require.ErrorContains(t, emitSandboxEvents(&vm.EVM{}, receipt), "StateDB is nil")
	require.Error(t, emitSandboxEvents(evm, lxphost.Receipt{Encoded: receipt.Encoded[:20]}))
}

func TestProgramsSandboxDestroyedAndRetiredLogs(t *testing.T) {
	activityID := [32]byte{0x19, 0x56, 0x02}
	leaseID := [32]byte{0x87, 0x65, 0x43, 0x21}
	terminal := bytes.Repeat([]byte{0xab}, 352)
	copy(terminal, "LXSD1")
	copy(terminal[5:37], leaseID[:])
	copy(terminal[37:69], activityID[:])
	var destruction [][]byte
	for i, chunk := range [][]byte{terminal[:248], terminal[248:]} {
		frame := append([]byte{'L', 'X', 'D', 'T', byte(i), 2, 1, 0}, chunk...)
		destruction = append(destruction, frame)
	}
	header := make([]byte, 226)
	copy(header, []byte{'L', 'X', 'R', 'T', 0, 4, 1})
	copy(header[7:39], activityID[:])
	header[199] = 3
	retirement := [][]byte{header}
	for i := byte(1); i <= 3; i++ {
		frame := bytes.Repeat([]byte{i}, 117)
		copy(frame, []byte{'L', 'X', 'R', 'T', i, 4, 1, i, 1})
		retirement = append(retirement, frame)
	}
	var effects []codec.Effect
	for _, frame := range destruction {
		effects = append(effects, sandboxKernelEvent(0x090A, frame))
	}
	for _, frame := range retirement {
		effects = append(effects, sandboxKernelEvent(0x090B, frame))
	}
	evm, db := sandboxEventState(t)
	require.Empty(t, db.GetAllLogs())
	require.NoError(t, emitSandboxEvents(evm, sandboxEventReceipt(t, activityID)))
	require.Empty(t, db.GetAllLogs())
	require.NoError(t, emitSandboxEvents(evm, sandboxEventReceipt(t, activityID, effects...)))
	logs := db.GetAllLogs()
	require.Len(t, logs, 6)
	var reconstructed []byte
	for i, log := range logs {
		name := "SandboxDestroyed"
		frame := effects[i].Body
		if i < 2 {
			assertSandboxLog(t, log, "0x88612a15e0cf0a2b8bfa1e89a7ad1a7284828c20680de0bb004210052506dc10", common.Hash(leaseID), uint(i))
			reconstructed = append(reconstructed, frame[8:]...)
		} else {
			name = "StorageRetired"
			assertSandboxLog(t, log, "0xb8fb07925225ab6c059c761581d51adeb4b7017ea3b0523efded11df61bed65b", common.Hash(activityID), uint(i))
		}
		values, err := programsABI.Events[name].Inputs.NonIndexed().Unpack(log.Data)
		require.NoError(t, err)
		require.Equal(t, []interface{}{frame[4], frame}, values)
		require.Equal(t, uint64(64), binary.BigEndian.Uint64(log.Data[56:64]))
	}
	require.Equal(t, terminal, reconstructed)

	t.Run("unrelated effects do not emit", func(t *testing.T) {
		evm, db := sandboxEventState(t)
		otherModule := sandboxKernelEvent(0x090A, nil)
		otherModule.ModuleID = 8
		stateEffect := sandboxKernelEvent(0x090B, nil)
		stateEffect.Kind = 1
		require.NoError(t, emitSandboxEvents(evm, sandboxEventReceipt(t, activityID, otherModule, stateEffect, sandboxKernelEvent(0x0908, nil))))
		require.Empty(t, db.GetAllLogs())
	})
	for _, tc := range []struct {
		name    string
		effects []codec.Effect
	}{
		{"orphan destruction", effects[1:2]},
		{"incomplete destruction", effects[:1]},
		{"duplicate destruction header", []codec.Effect{effects[0], effects[0]}},
		{"orphan retirement", effects[3:4]},
		{"incomplete retirement", effects[2:4]},
		{"out of order retirement", []codec.Effect{effects[2], effects[4], effects[3], effects[5]}},
		{"truncated destruction", []codec.Effect{sandboxKernelEvent(0x090A, destruction[0][:255])}},
		{"truncated retirement", []codec.Effect{sandboxKernelEvent(0x090B, header[:225])}},
		{"short destruction", []codec.Effect{sandboxKernelEvent(0x090A, []byte("LXDT"))}},
		{"short retirement", []codec.Effect{sandboxKernelEvent(0x090B, []byte("LXRT"))}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			evm, db := sandboxEventState(t)
			require.Error(t, emitSandboxEvents(evm, sandboxEventReceipt(t, activityID, tc.effects...)))
			require.Empty(t, db.GetAllLogs())
		})
	}
	for _, i := range []int{0, 2} {
		bad := bytes.Clone(effects[i].Body)
		bad[6] = 2
		evm, db := sandboxEventState(t)
		require.Error(t, emitSandboxEvents(evm, sandboxEventReceipt(t, activityID, sandboxKernelEvent(effects[i].EventType, bad))))
		require.Empty(t, db.GetAllLogs())
	}
}
