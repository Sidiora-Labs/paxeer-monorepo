package layerxprograms

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

type receiptEventCase struct {
	name      string
	signature string
	activity  uint32
	event     lxphost.ReceiptEvent
	programID [32]byte
	values    []interface{}
}

func receiptEventsEVM(t *testing.T) *vm.EVM {
	t.Helper()
	app := testkeeper.EVMTestApp
	ctx, _ := app.GetContextForDeliverTx(nil).CacheContext()
	return &vm.EVM{StateDB: state.NewDBImpl(ctx, &app.EvmKeeper, false)}
}

func assertReceiptEventCases(t *testing.T, cases []receiptEventCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decoded, err := decodeProgramsReceiptEvent(tc.activity, tc.event)
			require.NoError(t, err)
			require.Equal(t, tc.name, decoded.name)
			require.Equal(t, tc.values, decoded.values)
			definition, ok := programsABI.Events[tc.name]
			require.True(t, ok)
			require.Equal(t, crypto.Keccak256Hash([]byte(tc.signature)), definition.ID)
			evm := receiptEventsEVM(t)
			err = emitReceiptEvents(evm, tc.activity, tc.programID, lxphost.Receipt{Events: []lxphost.ReceiptEvent{tc.event}})
			if err != nil {
				t.Errorf("acceptance requires %s from the kernel receipt: %v", tc.name, err)
			}
			logs := state.GetDBImpl(evm.StateDB).GetAllLogs()
			if len(logs) != 1 {
				t.Errorf("want one %s log, got %d", tc.name, len(logs))
				return
			}
			require.Equal(t, common.HexToAddress("0x101D"), logs[0].Address)
			require.Equal(t, []common.Hash{definition.ID, common.Hash(tc.programID)}, logs[0].Topics)
			values, err := definition.Inputs.NonIndexed().Unpack(logs[0].Data)
			require.NoError(t, err)
			require.Len(t, values, len(tc.values))
			for i, value := range tc.values {
				require.Equal(t, value, values[i])
			}
			if !decoded.hasProgramID {
				otherID := sha256.Sum256([]byte("another program for " + tc.name))
				otherEVM := receiptEventsEVM(t)
				require.NoError(t, emitReceiptEvents(otherEVM, tc.activity, otherID, lxphost.Receipt{
					Events: []lxphost.ReceiptEvent{tc.event},
				}))
				otherLogs := state.GetDBImpl(otherEVM.StateDB).GetAllLogs()
				require.Len(t, otherLogs, 1)
				require.Equal(t, logs[0].Address, otherLogs[0].Address)
				require.Equal(t, []common.Hash{definition.ID, common.Hash(otherID)}, otherLogs[0].Topics)
				require.Equal(t, logs[0].Data, otherLogs[0].Data)
			}
		})
		t.Run(tc.name+"/malformed", func(t *testing.T) {
			for _, length := range []int{0, len(tc.event.Body) - 1, len(tc.event.Body) + 1} {
				evm := receiptEventsEVM(t)
				body := make([]byte, length)
				copy(body, tc.event.Body)
				err := emitReceiptEvents(evm, tc.activity, tc.programID, lxphost.Receipt{Events: []lxphost.ReceiptEvent{{Code: tc.event.Code, Body: body}}})
				require.ErrorIs(t, err, lxphost.Code(-1001))
				require.Contains(t, err.Error(), "body length")
				require.Empty(t, state.GetDBImpl(evm.StateDB).GetAllLogs())
			}
		})
	}
}

func TestProgramsLifecycleEventsFromReceipt(t *testing.T) {
	programID := sha256.Sum256([]byte("lifecycle program"))
	oldHash := sha256.Sum256([]byte("old wasm"))
	newHash := sha256.Sum256([]byte("new wasm"))
	transferRoot := sha256.Sum256([]byte("transfer set"))
	cases := []receiptEventCase{
		{
			name: "ProgramDeployed", signature: "ProgramDeployed(bytes32,bytes32)", activity: 0x00090001,
			event: lxphost.ReceiptEvent{Code: 1, Body: newHash[:]}, programID: programID,
			values: []interface{}{newHash},
		},
		{
			name: "ProgramUpgraded", signature: "ProgramUpgraded(bytes32,bytes32,bytes32)", activity: 0x00090002,
			event: lxphost.ReceiptEvent{Code: 2, Body: bytes.Join([][]byte{oldHash[:], newHash[:]}, nil)}, programID: programID,
			values: []interface{}{oldHash, newHash},
		},
		{
			name: "RegistryRead", signature: "RegistryRead(bytes32)", activity: 0x00090004,
			event: lxphost.ReceiptEvent{Code: 4, Body: programID[:]}, programID: programID,
		},
		{
			name: "ProgramTransferred", signature: "ProgramTransferred(bytes32,bytes32)", activity: 0x00090005,
			event: lxphost.ReceiptEvent{Code: 5, Body: transferRoot[:]}, programID: programID,
			values: []interface{}{transferRoot},
		},
	}
	assertReceiptEventCases(t, cases)

	t.Run("receipt order", func(t *testing.T) {
		secondID := sha256.Sum256([]byte("second registry program"))
		evm := receiptEventsEVM(t)
		require.NoError(t, emitReceiptEvents(evm, 0x00090004, programID, lxphost.Receipt{Events: []lxphost.ReceiptEvent{
			{Code: 4, Body: secondID[:]}, {Code: 4, Body: programID[:]},
		}}))
		logs := state.GetDBImpl(evm.StateDB).GetAllLogs()
		require.Len(t, logs, 2)
		for i, id := range [][32]byte{secondID, programID} {
			require.Equal(t, common.HexToAddress("0x101D"), logs[i].Address)
			require.Equal(t, []common.Hash{programsABI.Events["RegistryRead"].ID, common.Hash(id)}, logs[i].Topics)
			require.Empty(t, logs[i].Data)
			require.Equal(t, uint(i), logs[i].Index)
		}
	})
	t.Run("validation before emission", func(t *testing.T) {
		evm := receiptEventsEVM(t)
		require.ErrorIs(t, emitReceiptEvents(evm, 0x00090004, programID, lxphost.Receipt{Events: []lxphost.ReceiptEvent{
			{Code: 4, Body: programID[:]}, {Code: 4, Body: programID[:31]},
		}}), lxphost.Code(-1001))
		require.Empty(t, state.GetDBImpl(evm.StateDB).GetAllLogs())
	})
	t.Run("invalid context and activity", func(t *testing.T) {
		require.NoError(t, emitReceiptEvents(nil, 0x00090004, programID, lxphost.Receipt{}))
		require.EqualError(t, emitReceiptEvents(nil, 0x00090004, programID, lxphost.Receipt{Events: []lxphost.ReceiptEvent{cases[2].event}}), "EVM is nil")
		require.EqualError(t, emitReceiptEvents(&vm.EVM{}, 0x00090004, programID, lxphost.Receipt{Events: []lxphost.ReceiptEvent{cases[2].event}}), "EVM StateDB is nil")
		evm := receiptEventsEVM(t)
		require.ErrorIs(t, emitReceiptEvents(evm, 0x00090001, programID, lxphost.Receipt{Events: []lxphost.ReceiptEvent{cases[2].event}}), lxphost.Code(-1001))
		require.ErrorIs(t, emitReceiptEvents(evm, 0x00090004, programID, lxphost.Receipt{Events: []lxphost.ReceiptEvent{{Code: 0xffff, Body: programID[:]}}}), lxphost.Code(-1001))
		require.Empty(t, state.GetDBImpl(evm.StateDB).GetAllLogs())
	})
}

func TestProgramsWindDownAndAccountEventsFromReceipt(t *testing.T) {
	programID := sha256.Sum256([]byte("wind-down program"))
	exitProgram := sha256.Sum256([]byte("exit program"))
	account := sha256.Sum256([]byte("program account"))
	asset := sha256.Sum256([]byte("asset"))
	destination := sha256.Sum256([]byte("destination account"))
	principal := sha256.Sum256([]byte("principal"))
	accountRoot := sha256.Sum256([]byte("account registry"))
	transferRoot := sha256.Sum256([]byte("exit transfer set"))
	boundedRoot := sha256.Sum256([]byte("bounded exit transfer set"))
	seed := bytes.Repeat([]byte{0x7a}, 128)
	seedDigest := sha256.Sum256(seed)
	const sequence uint64 = 0xfedcba9876543210
	const deadline uint64 = 0x8123456789abcdef
	const accountCount uint16 = 0x1234
	const liveCount uint16 = 0x0210
	registered := bytes.Join([][]byte{[]byte("LXPA1"), programID[:], account[:], asset[:]}, nil)
	registered = binary.BigEndian.AppendUint16(registered, uint16(len(seed)))
	registered = append(registered, seedDigest[:]...)
	registered = binary.BigEndian.AppendUint64(registered, sequence)
	transition := func(prior, target byte) []byte {
		body := bytes.Join([][]byte{{1, prior, target}, principal[:], exitProgram[:]}, nil)
		body = binary.BigEndian.AppendUint64(body, deadline)
		body = binary.BigEndian.AppendUint64(body, sequence)
		body = binary.BigEndian.AppendUint16(body, accountCount)
		body = binary.BigEndian.AppendUint16(body, liveCount)
		return append(body, accountRoot[:]...)
	}
	transitionValues := func(prior, target uint8) []interface{} {
		return []interface{}{prior, target, principal, exitProgram, deadline, sequence, accountCount, liveCount, accountRoot}
	}
	const transitionSignature = "(bytes32,uint8,uint8,bytes32,bytes32,uint64,uint64,uint16,uint16,bytes32)"
	cases := []receiptEventCase{
		{
			name: "ProgramAccountRegistered", signature: "ProgramAccountRegistered(bytes32,bytes32,bytes32,bytes32,uint16,uint64)", activity: 0x00090006,
			event: lxphost.ReceiptEvent{Code: 8, Body: registered}, programID: programID,
			values: []interface{}{account, asset, seedDigest, uint16(len(seed)), sequence},
		},
		{
			name: "ExitRouteSet", signature: "ExitRouteSet(bytes32,bytes32,bytes32,bytes32)", activity: 0x00090007,
			event: lxphost.ReceiptEvent{Code: 9, Body: bytes.Join([][]byte{programID[:], account[:], asset[:], destination[:]}, nil)}, programID: programID,
			values: []interface{}{account, asset, destination},
		},
		{
			name: "ProgramDeprecated", signature: "ProgramDeprecated" + transitionSignature, activity: 0x00090007,
			event: lxphost.ReceiptEvent{Code: 10, Body: transition(1, 2)}, programID: programID,
			values: transitionValues(1, 2),
		},
		{
			name: "ProgramTombstoned", signature: "ProgramTombstoned" + transitionSignature, activity: 0x00090007,
			event: lxphost.ReceiptEvent{Code: 11, Body: transition(2, 3)}, programID: programID,
			values: transitionValues(2, 3),
		},
		{
			name: "ProgramValueExited", signature: "ProgramValueExited(bytes32,bytes32,bytes32)", activity: 0x00090007,
			event: lxphost.ReceiptEvent{Code: 12, Body: bytes.Join([][]byte{programID[:], account[:], transferRoot[:]}, nil)}, programID: programID,
			values: []interface{}{account, transferRoot},
		},
		{
			name: "ProgramValueExited", signature: "ProgramValueExited(bytes32,bytes32,bytes32)", activity: 0x00090007,
			event: lxphost.ReceiptEvent{Code: 12, Body: bytes.Join([][]byte{programID[:], account[:], boundedRoot[:]}, nil)}, programID: programID,
			values: []interface{}{account, boundedRoot},
		},
	}
	assertReceiptEventCases(t, cases)

	t.Run("receipt order", func(t *testing.T) {
		evm := receiptEventsEVM(t)
		require.NoError(t, emitReceiptEvents(evm, 0x00090007, programID, lxphost.Receipt{Events: []lxphost.ReceiptEvent{
			cases[4].event, cases[2].event, cases[1].event, cases[3].event, cases[5].event,
		}}))
		logs := state.GetDBImpl(evm.StateDB).GetAllLogs()
		require.Len(t, logs, 5)
		for i, tc := range []receiptEventCase{cases[4], cases[2], cases[1], cases[3], cases[5]} {
			require.Equal(t, common.HexToAddress("0x101D"), logs[i].Address)
			require.Equal(t, []common.Hash{programsABI.Events[tc.name].ID, common.Hash(programID)}, logs[i].Topics)
			values, err := programsABI.Events[tc.name].Inputs.NonIndexed().Unpack(logs[i].Data)
			require.NoError(t, err)
			require.Equal(t, tc.values, values)
			require.Equal(t, uint(i), logs[i].Index)
		}
	})
	t.Run("invalid record metadata", func(t *testing.T) {
		for _, tc := range []receiptEventCase{cases[0], cases[2], cases[3]} {
			body := append([]byte(nil), tc.event.Body...)
			body[0] ^= 0xff
			evm := receiptEventsEVM(t)
			require.ErrorIs(t, emitReceiptEvents(evm, tc.activity, tc.programID, lxphost.Receipt{Events: []lxphost.ReceiptEvent{{Code: tc.event.Code, Body: body}}}), lxphost.Code(-1001))
			require.Empty(t, state.GetDBImpl(evm.StateDB).GetAllLogs())
		}
	})
}
