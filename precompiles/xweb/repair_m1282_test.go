package xweb_test

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	xwebkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/xweb"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func repairM1282SolidityEvents(t *testing.T, raw []byte) map[string]abi.Event {
	t.Helper()
	source := regexp.MustCompile(`(?s)/\*.*?\*/|//[^\n]*`).ReplaceAllString(string(raw), "")
	events := make(map[string]abi.Event)
	declarations := regexp.MustCompile(`(?s)\bevent\s+(\w+)\s*\(([^)]*)\)\s*;`).FindAllStringSubmatch(source, -1)
	require.Equal(t, strings.Count(source, "event "), len(declarations))
	for _, declaration := range declarations {
		name := declaration[1]
		_, duplicate := events[name]
		require.False(t, duplicate, name)
		var args abi.Arguments
		for _, field := range strings.Split(declaration[2], ",") {
			parts := strings.Fields(field)
			require.True(t, len(parts) == 2 || len(parts) == 3, field)
			indexed := len(parts) == 3
			if indexed {
				require.Equal(t, "indexed", parts[1])
			}
			typ, err := abi.NewType(parts[0], parts[0], nil)
			require.NoError(t, err)
			args = append(args, abi.Argument{Name: parts[len(parts)-1], Type: typ, Indexed: indexed})
		}
		events[name] = abi.NewEvent(name, name, false, args)
	}
	return events
}

func TestRepairM_1282_SolidityProgramEventFields(t *testing.T) {
	local, err := os.ReadFile("XWeb.sol")
	require.NoError(t, err)
	raw, err := os.ReadFile("abi.json")
	require.NoError(t, err)
	parsed, err := abi.JSON(strings.NewReader(string(raw)))
	require.NoError(t, err)
	declared := repairM1282SolidityEvents(t, local)
	event := parsed.Events["XWebProgramRequested"]
	require.Equal(t, event.Inputs, declared[event.Name].Inputs)
	require.Equal(t, event.ID, declared[event.Name].ID)
	require.Equal(t, []string{"programId", "requestId", "kind", "payload", "sequence"},
		[]string{event.Inputs[0].Name, event.Inputs[1].Name, event.Inputs[2].Name, event.Inputs[3].Name, event.Inputs[4].Name})
	require.Equal(t, uint8(1), types.KindFetch)
	require.Equal(t, uint8(2), types.KindSearch)
	require.Equal(t, uint8(2), types.OriginProgram)
	TestProgramRequestEmitsXWebProgramRequested(t)
}

func TestRepairM_1282_EventParityIncludesNames(t *testing.T) {
	// Retain the existing topic assertions and require both published interfaces
	// to match the complete named ABI, including indexed positions.
	TestEventTopics(t)
	TestABIMatchesTheSolidityInterface(t)
}

func TestRepairM_1282_ABIProgramEventContext(t *testing.T) {
	repairM1282ProgramLogs(t)
}

func TestRepairM_1282_ProgramEventRealLogs(t *testing.T) {
	TestProgramRequestEmitsXWebProgramRequested(t)
	repairM1282ProgramLogs(t)
}

func repairM1282ProgramLogs(t *testing.T) {
	t.Helper()
	h := newProgramDispatchHarness(t)
	bank := testkeeper.EVMTestApp.BankKeeper
	id := h.request(recorder, callbackGas)
	require.Equal(t, uint8(types.StatusPending), h.requestView(id).Status)
	event := h.precompile.GetABI().Events["XWebProgramRequested"]
	schemas := []abi.Event{event}
	for _, path := range []string{"XWeb.sol", "../../contracts/src/precompiles/IXWeb.sol"} {
		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		declared := repairM1282SolidityEvents(t, raw)
		schema, found := declared[event.Name]
		require.True(t, found, path)
		require.Equal(t, event.Inputs, schema.Inputs, path)
		require.Equal(t, event.ID, schema.ID, path)
		schemas = append(schemas, schema)
	}
	for i, kind := range []uint8{types.KindFetch, types.KindSearch} {
		programID := types.Hash32{0x82, byte(i + 1)}
		asset := types.Hash32{0x61, byte(i + 1)}
		sequence := []uint64{0x0102030405060708, 0xfedcba9876543210}[i]
		input := [][]byte{[]byte("https://paxeer.app/fetch?raw=\x00\xff"), []byte("program search \x00\xfe")}[i]
		principal := h.account(stranger)
		coins := sdk.NewCoins(sdk.NewInt64Coin(types.ProgramFeeDenomPAX, 75))
		require.NoError(t, bank.MintCoins(h.ctx(), "evm", coins))
		require.NoError(t, bank.SendCoinsFromModuleToAccount(h.ctx(), "evm", principal, coins))
		snapshot := h.stateDB.Snapshot()
		leg := xwebkeeper.ProgramLeg{From: principal, To: xwebkeeper.ProgramFeeAccount(asset), Asset: asset, Denom: types.ProgramFeeDenomPAX, Amount: sdk.NewInt(75)}
		require.NoError(t, bank.SendCoins(h.ctx(), leg.From, leg.To, coins))
		call := xwebkeeper.ProgramCall{ProgramID: programID, Principal: principal, Sequence: sequence, Legs: []xwebkeeper.ProgramLeg{leg}}
		original := call
		data := binary.BigEndian.AppendUint64(nil, id)
		data = append(data, kind)
		data = binary.BigEndian.AppendUint32(data, uint32(len(input)))
		data = append(data, input...)
		before := len(h.stateDB.GetAllLogs())
		require.ErrorContains(t, h.precompile.RequestFromProgram(h.evm, &call, []byte(xwebkeeper.ProgramRequestTopic), data, true), "LXP_XWEB_STATICCALL")
		require.Equal(t, original, call)
		require.Len(t, h.stateDB.GetAllLogs(), before)
		require.NoError(t, h.precompile.RequestFromProgram(h.evm, &call, []byte(xwebkeeper.ProgramRequestTopic), data, false))
		require.Len(t, h.stateDB.GetAllLogs(), before+1)
		h.stateDB.RevertToSnapshot(snapshot)
		require.Len(t, h.stateDB.GetAllLogs(), before)
		_, found, err := h.keeper.GetProgramPendingRecord(h.ctx(), programID, id)
		require.NoError(t, err)
		require.False(t, found, "parent rollback must remove admission")
		require.True(t, bank.GetAllBalances(h.ctx(), leg.To).IsZero())
		require.NoError(t, bank.SendCoins(h.ctx(), leg.From, leg.To, coins))
		call = original
		require.NoError(t, h.precompile.RequestFromProgram(h.evm, &call, []byte(xwebkeeper.ProgramRequestTopic), data, false))
		record, found, err := h.keeper.GetProgramPendingRecord(h.ctx(), programID, id)
		require.NoError(t, err)
		require.True(t, found)
		require.False(t, record.Fulfilled)
		require.Equal(t, sequence, record.Sequence)
		require.Equal(t, kind, record.Kind)
		require.Equal(t, types.Keccak(input), record.PayloadHash)
		emitted := h.logs("XWebProgramRequested")[i]
		require.Equal(t, []common.Hash{event.ID, common.Hash(programID), topicOf(id)}, emitted.Topics)
		raw, err := json.Marshal(emitted)
		require.NoError(t, err)
		var transported ethtypes.Log
		require.NoError(t, json.Unmarshal(raw, &transported))
		require.Equal(t, emitted.Data, transported.Data)
		require.Equal(t, emitted.Topics, transported.Topics)
		for _, schema := range schemas {
			decoded := make(map[string]interface{})
			require.NoError(t, schema.Inputs.NonIndexed().UnpackIntoMap(decoded, transported.Data))
			require.Equal(t, map[string]interface{}{"kind": kind, "payload": input, "sequence": sequence}, decoded)
		}
		decoded, err := event.Inputs.NonIndexed().Unpack(transported.Data)
		require.NoError(t, err)
		answer := append([]byte("answer\x00\xff"), kind)
		observation := types.Observation{Origin: types.OriginProgram, NetworkID: kernelNetworkID,
			ProgramID: types.Hash32(transported.Topics[1]), RequestID: binary.BigEndian.Uint64(transported.Topics[2][24:]),
			Kind: decoded[0].(uint8), PayloadHash: types.Keccak(decoded[1].([]byte)),
			ContentDigest: types.Keccak(answer), FullLength: uint32(len(answer)), Response: answer}
		signed := signProgramDispatchObservation(t, h, observation, 2)
		beforePayout := bank.GetBalance(h.ctx(), h.attestors[0].Payout, leg.Denom).Amount
		_, _, reason := h.call(stranger, xweb.FulfilProgramMethod, nil, supplied, signed)
		require.Empty(t, reason)
		require.Equal(t, []interface{}{[32]byte(observation.ContentDigest), uint32(len(answer)), uint32(len(answer)), answer}, h.view(xweb.GetProgramResultMethod, [32]byte(programID), id))
		requireAmount(t, beforePayout.AddRaw(38), bank.GetBalance(h.ctx(), h.attestors[0].Payout, leg.Denom).Amount)
		_, _, reason = h.call(stranger, xweb.FulfilProgramMethod, nil, supplied, signed)
		require.Equal(t, "LXP_SEQUENCE_REUSED", reason)
		requireAmount(t, beforePayout.AddRaw(38), bank.GetBalance(h.ctx(), h.attestors[0].Payout, leg.Denom).Amount)
		fresh := original
		require.ErrorContains(t, h.precompile.RequestFromProgram(h.evm, &fresh, []byte(xwebkeeper.ProgramRequestTopic), data, false), "LXP_SEQUENCE_REUSED")
		record, found, err = h.keeper.GetProgramPendingRecord(h.ctx(), programID, id)
		require.NoError(t, err)
		require.True(t, found)
		require.True(t, record.Fulfilled)
		require.Equal(t, sequence, record.Sequence)
		require.Equal(t, uint8(types.StatusPending), h.requestView(id).Status, "program fulfilment must not consume same-id EVM request")
	}
	require.Len(t, h.logs("XWebProgramRequested"), 2)
	require.Len(t, h.logs("XWebRequested"), 1)
	require.Zero(t, h.keeper.ProgramPendingCount(h.ctx()))
	require.Equal(t, uint64(2), h.keeper.ProgramCommittedCount(h.ctx()))
	_, _, reason := h.call(stranger, xweb.FulfilMethod, nil, supplied, id, response, digest, fullLength,
		h.signatures(id, response, digest, fullLength))
	require.Empty(t, reason)
	require.Equal(t, uint8(types.StatusFulfilled), h.requestView(id).Status)
}
