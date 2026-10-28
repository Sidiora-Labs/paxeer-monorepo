package xweb_test

import (
	"encoding/binary"
	"testing"

	xwebkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestProgramRequestEmitsXWebProgramRequested(t *testing.T) {
	h := newProgramDispatchHarness(t)
	event, found := h.precompile.GetABI().Events["XWebProgramRequested"]
	require.True(t, found)
	require.Equal(t, "XWebProgramRequested(bytes32,uint64,uint8,bytes,uint64)", event.Sig)
	require.False(t, event.Anonymous)
	require.Len(t, event.Inputs, 5)
	for i, name := range []string{"programId", "requestId", "origin", "input", "deadline"} {
		require.Equal(t, name, event.Inputs[i].Name)
		require.Equal(t, i < 2, event.Inputs[i].Indexed)
	}
	topic := crypto.Keccak256Hash([]byte("XWebProgramRequested(bytes32,uint64,uint8,bytes,uint64)"))
	require.Equal(t, topic, event.ID)

	evmID := h.request(recorder, callbackGas)
	require.Empty(t, h.logs("XWebProgramRequested"))
	require.Len(t, h.logs("XWebRequested"), 1)
	require.Equal(t, evmID, h.requestView(evmID).Id)

	programID := types.Hash32{0x94, 0x03, 0x7f}
	asset := types.Hash32{0x61, 0x22}
	principal := h.account(stranger)
	amount := sdk.NewInt(75)
	coins := sdk.NewCoins(sdk.NewCoin(types.ProgramFeeDenomPAX, amount))
	bank := testkeeper.EVMTestApp.BankKeeper
	require.NoError(t, bank.MintCoins(h.ctx(), "evm", coins))
	require.NoError(t, bank.SendCoinsFromModuleToAccount(h.ctx(), "evm", principal, coins))
	leg := xwebkeeper.ProgramLeg{From: principal, To: xwebkeeper.ProgramFeeAccount(asset),
		Asset: asset, Denom: types.ProgramFeeDenomPAX, Amount: amount}
	require.NoError(t, bank.SendCoins(h.ctx(), leg.From, leg.To, coins))
	call := &xwebkeeper.ProgramCall{ProgramID: programID, Principal: principal, Sequence: 23,
		Legs: []xwebkeeper.ProgramLeg{leg}}
	id := uint64(0x123456789abcdef0)
	input := []byte("https://paxeer.app/program?raw=\x00\xff")
	data := binary.BigEndian.AppendUint64(nil, id)
	data = append(data, types.KindFetch)
	data = binary.BigEndian.AppendUint32(data, uint32(len(input)))
	data = append(data, input...)
	requestTopic := []byte(xwebkeeper.ProgramRequestTopic)
	before := len(h.stateDB.GetAllLogs())

	require.ErrorContains(t, h.precompile.RequestFromProgram(h.evm, call, requestTopic, data, true), "LXP_XWEB_STATICCALL")
	require.Equal(t, before, len(h.stateDB.GetAllLogs()))
	require.Zero(t, h.keeper.ProgramPendingCount(h.ctx()))
	require.NoError(t, h.precompile.RequestFromProgram(h.evm, call, []byte("OTHER_TOPIC"), nil, false))
	require.Equal(t, before, len(h.stateDB.GetAllLogs()))
	require.Error(t, h.precompile.RequestFromProgram(h.evm, call, requestTopic, data[:12], false))
	require.Equal(t, before, len(h.stateDB.GetAllLogs()))
	require.Zero(t, h.keeper.ProgramPendingCount(h.ctx()))

	require.NoError(t, h.precompile.RequestFromProgram(h.evm, call, requestTopic, data, false))
	record, found, err := h.keeper.GetProgramPendingRecord(h.ctx(), programID, id)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, types.KindFetch, record.Kind)
	require.Equal(t, types.Keccak(input), record.PayloadHash)
	require.Equal(t, uint64(1), h.keeper.ProgramPendingCount(h.ctx()))
	logs := h.logs("XWebProgramRequested")
	require.Len(t, logs, 1)
	require.Equal(t, precompile, logs[0].Address)
	require.Equal(t, []common.Hash{topic, common.Hash(programID), topicOf(id)}, logs[0].Topics)
	decoded, err := event.Inputs.NonIndexed().Unpack(logs[0].Data)
	require.NoError(t, err)
	require.Equal(t, []interface{}{types.OriginProgram, input, uint64(0)}, decoded)
	allLogs := h.stateDB.GetAllLogs()
	require.Len(t, allLogs, before+2)
	require.Equal(t, h.precompile.GetABI().Events["XWebRequested"].ID, allLogs[before].Topics[0])
	require.Equal(t, topicOf(id), allLogs[before].Topics[1])
	require.Equal(t, topic, allLogs[before+1].Topics[0])

	require.ErrorContains(t, h.precompile.RequestFromProgram(h.evm, call, requestTopic, data, false), "LXP_DUPLICATE_ENTRY")
	require.Len(t, h.stateDB.GetAllLogs(), before+2)
	h.request(recorder, callbackGas)
	require.Len(t, h.logs("XWebProgramRequested"), 1)
	require.Len(t, h.logs("XWebRequested"), 3)
}
