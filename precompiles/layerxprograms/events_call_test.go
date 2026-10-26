package layerxprograms

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"testing"

	ibctransferkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/keeper"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func callEventState(t *testing.T) (*vm.EVM, *state.DBImpl) {
	t.Helper()
	key := sdk.NewKVStoreKey("programs_event_evm")
	transient := sdk.NewTransientStoreKey("programs_event_transient")
	ctx := testutil.DefaultContext(key, transient).WithBlockHeight(41)
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	subspace := func(name string) paramtypes.Subspace {
		return paramtypes.NewSubspace(cdc, codec.NewLegacyAmino(), key, transient, name)
	}
	accounts := authkeeper.NewAccountKeeper(cdc, key, subspace("programs_event_auth"), authtypes.ProtoBaseAccount,
		map[string][]string{authtypes.FeeCollectorName: nil})
	keeper := evmkeeper.NewKeeper(key, transient, subspace("programs_event_evm"), nil, nil, &accounts, nil,
		ibctransferkeeper.Keeper{}, nil, nil, nil)
	db := state.NewDBImpl(ctx, keeper, false)
	t.Cleanup(db.Cleanup)
	return &vm.EVM{StateDB: db}, db
}

func callEnvelopePrefix(header string, program [32]byte, frame [8]byte, depth byte) []byte {
	body := append([]byte(header), program[:]...)
	principal, activity := [32]byte{0x22}, [32]byte{0x33}
	body = append(body, principal[:]...)
	body = append(body, activity[:]...)
	body = append(body, frame[:]...)
	return append(body, depth)
}

func callGuestEnvelope(program [32]byte, frame [8]byte, depth byte, index uint32, topic, data []byte) lxphost.ReceiptEvent {
	body := callEnvelopePrefix("LXGE\x01\x01", program, frame, depth)
	body = binary.BigEndian.AppendUint32(body, index)
	body = binary.BigEndian.AppendUint16(body, uint16(len(topic)))
	topicDigest := sha256.Sum256(append([]byte("LayerX/programs/event-topic/v1"), topic...))
	dataDigest := sha256.Sum256(append([]byte("LayerX/programs/event-data/v1"), data...))
	body = append(body, topicDigest[:]...)
	body = binary.BigEndian.AppendUint32(body, uint32(len(data)))
	body = append(body, dataDigest[:]...)
	return lxphost.ReceiptEvent{Code: 6, Body: body}
}

func callOutcomeReceipt(legacy bool) lxphost.Receipt {
	header := "LXMO\x02\x02"
	if legacy {
		header = "LXCO\x01\x02"
	}
	body := callEnvelopePrefix(header, [32]byte{0x11}, [8]byte{}, 0)
	body = binary.BigEndian.AppendUint16(body, 3)
	body = binary.BigEndian.AppendUint16(body, 5)
	body = binary.BigEndian.AppendUint32(body, 0x12345678)
	if !legacy {
		body = binary.BigEndian.AppendUint32(body, 1)
	}
	body = binary.BigEndian.AppendUint32(body, 0)
	for _, root := range [][32]byte{{0x44}, {0x55}, {0x66}, {0x77}} {
		body = append(body, root[:]...)
	}
	return lxphost.Receipt{
		Events: []lxphost.ReceiptEvent{{Code: 7, Body: body}},
		ProgramOutcome: &lxphost.ProgramOutcome{
			Status: 0, Kind: 1, FeeScheduleVersion: 0x12345678, MeteringScheduleVersion: 1,
		},
	}
}

func TestProgramsGuestEventLogs(t *testing.T) {
	evm, db := callEventState(t)
	receipt := callOutcomeReceipt(false)
	topics := [][]byte{[]byte("first"), {0, 0xff, 0x80}}
	data := [][]byte{{1, 2, 0, 0xff}, {}}
	programs := [][32]byte{{0x11}, {0x99}}
	frames := [][8]byte{{}, {2, 3}}
	for i := range programs {
		depth := byte(0)
		if i == 1 {
			depth = 2
		}
		receipt.Events = append(receipt.Events, callGuestEnvelope(programs[i], frames[i], depth, uint32(i), topics[i], data[i]))
	}
	receipt.Events = append(receipt.Events, lxphost.ReceiptEvent{Code: 4, Body: []byte("unrelated")})
	require.NoError(t, emitCallEvents(evm, receipt))
	logs := db.GetAllLogs()
	require.Len(t, logs, 3)
	for i := range programs {
		log := logs[i]
		require.Equal(t, common.HexToAddress("0x101D"), log.Address)
		require.Equal(t, []common.Hash{
			common.HexToHash("0x23a84d3a8c8e7d232703d9715c64dba47a94a94c87c46fe61532e5a609e761d1"),
			common.Hash(programs[i]),
		}, log.Topics)
		require.Equal(t, uint(i), log.Index)
		values, err := programsABI.Events["ProgramGuestEvent"].Inputs.NonIndexed().Unpack(log.Data)
		require.NoError(t, err)
		topicDigest := sha256.Sum256(append([]byte("LayerX/programs/event-topic/v1"), topics[i]...))
		dataDigest := sha256.Sum256(append([]byte("LayerX/programs/event-data/v1"), data[i]...))
		depth := uint8(0)
		if i == 1 {
			depth = 2
		}
		require.Equal(t, []interface{}{
			[32]byte{0x22}, [32]byte{0x33}, binary.BigEndian.Uint64(frames[i][:]), depth,
			uint32(i), uint16(len(topics[i])), topicDigest, uint32(len(data[i])), dataDigest,
		}, values)
	}
	require.Equal(t, programsABI.Events["ProgramCallOutcome"].ID, logs[2].Topics[0])

	for _, mutate := range []struct {
		name  string
		apply func([]byte) []byte
	}{
		{"truncated", func(b []byte) []byte { return b[:184] }},
		{"trailing", func(b []byte) []byte { return append(b, 0) }},
		{"version", func(b []byte) []byte { b[4] = 2; return b }},
		{"depth", func(b []byte) []byte { b[110] = 9; return b }},
		{"frame", func(b []byte) []byte { b[102] = 1; return b }},
		{"topic bound", func(b []byte) []byte { binary.BigEndian.PutUint16(b[115:117], 65); return b }},
		{"data bound", func(b []byte) []byte { binary.BigEndian.PutUint32(b[149:153], 65537); return b }},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			evm, db := callEventState(t)
			r := callOutcomeReceipt(false)
			guest := callGuestEnvelope([32]byte{0x11}, [8]byte{}, 0, 0, nil, nil)
			guest.Body = mutate.apply(guest.Body)
			r.Events = append(r.Events, guest)
			require.Error(t, emitCallEvents(evm, r))
			require.Empty(t, db.GetAllLogs())
		})
	}
}

func TestProgramsCallOutcomeLog(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		name := "metered"
		if legacy {
			name = "legacy"
		}
		t.Run(name, func(t *testing.T) {
			evm, db := callEventState(t)
			require.NoError(t, emitCallEvents(evm, callOutcomeReceipt(legacy)))
			logs := db.GetAllLogs()
			require.Len(t, logs, 1)
			require.Equal(t, common.HexToAddress("0x101D"), logs[0].Address)
			require.Equal(t, []common.Hash{
				common.HexToHash("0xcfe5297f03a2951065e463e9e0c2e3686bc0204f727eeec1a52357b5dd4a27a0"),
				common.Hash{0x11},
			}, logs[0].Topics)
			values, err := programsABI.Events["ProgramCallOutcome"].Inputs.NonIndexed().Unpack(logs[0].Data)
			require.NoError(t, err)
			require.Equal(t, []interface{}{
				[32]byte{0x22}, [32]byte{0x33}, uint64(0), uint8(0), uint16(3), uint16(5),
				uint32(0x12345678), uint32(1), int32(0), [32]byte{0x44}, [32]byte{0x55}, [32]byte{0x66}, [32]byte{0x77},
			}, values)
		})
	}
	for _, tc := range []struct {
		name   string
		mutate func(*lxphost.Receipt)
	}{
		{"missing program outcome", func(r *lxphost.Receipt) { r.ProgramOutcome = nil }},
		{"missing envelope", func(r *lxphost.Receipt) { r.Events = nil }},
		{"duplicate envelope", func(r *lxphost.Receipt) { r.Events = append(r.Events, r.Events[0]) }},
		{"truncated", func(r *lxphost.Receipt) { r.Events[0].Body = r.Events[0].Body[:254] }},
		{"unknown version", func(r *lxphost.Receipt) { r.Events[0].Body[4] = 3 }},
		{"fee overflow", func(r *lxphost.Receipt) { r.ProgramOutcome.FeeScheduleVersion = math.MaxUint32 + 1 }},
		{"metering overflow", func(r *lxphost.Receipt) { r.ProgramOutcome.MeteringScheduleVersion = math.MaxUint32 + 1 }},
		{"fee mismatch", func(r *lxphost.Receipt) { r.ProgramOutcome.FeeScheduleVersion++ }},
		{"metering mismatch", func(r *lxphost.Receipt) { r.ProgramOutcome.MeteringScheduleVersion++ }},
		{"status mismatch", func(r *lxphost.Receipt) { r.ProgramOutcome.Status = -1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			evm, db := callEventState(t)
			r := callOutcomeReceipt(false)
			tc.mutate(&r)
			require.Error(t, emitCallEvents(evm, r))
			require.Empty(t, db.GetAllLogs())
		})
	}
	require.ErrorContains(t, emitCallEvents(nil, callOutcomeReceipt(false)), "EVM is nil")
	require.ErrorContains(t, emitCallEvents(&vm.EVM{}, callOutcomeReceipt(false)), "StateDB is nil")
}
