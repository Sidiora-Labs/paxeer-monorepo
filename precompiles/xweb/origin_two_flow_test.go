package xweb_test

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	xwebkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/xweb"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

var originTwoFeeAsset = types.Hash32{0x74, 0x01}

func (h *harness) RequestFromProgram() types.Observation {
	h.t.Helper()
	principal := h.account(stranger)
	coins := sdk.NewCoins(sdk.NewInt64Coin(types.ProgramFeeDenomUSDL, 75))
	bank := testkeeper.EVMTestApp.BankKeeper
	require.NoError(h.t, bank.MintCoins(h.ctx(), "evm", coins))
	require.NoError(h.t, bank.SendCoinsFromModuleToAccount(h.ctx(), "evm", principal, coins))
	before := bank.GetBalance(h.ctx(), principal, types.ProgramFeeDenomUSDL).Amount
	leg := xwebkeeper.ProgramLeg{From: principal, To: xwebkeeper.ProgramFeeAccount(originTwoFeeAsset),
		Asset: originTwoFeeAsset, Denom: types.ProgramFeeDenomUSDL, Amount: sdk.NewInt(75)}
	require.NoError(h.t, bank.SendCoins(h.ctx(), leg.From, leg.To, coins))
	require.True(h.t, before.SubRaw(75).Equal(bank.GetBalance(h.ctx(), principal, leg.Denom).Amount))
	call := &xwebkeeper.ProgramCall{ProgramID: types.Hash32{0x74, 0x02}, Principal: principal, Sequence: 23,
		Legs: []xwebkeeper.ProgramLeg{leg}}
	requestID := uint64(17)
	data := binary.BigEndian.AppendUint64(nil, requestID)
	data = append(data, types.KindFetch)
	data = binary.BigEndian.AppendUint32(data, uint32(len(payload)))
	data = append(data, payload...)
	require.NoError(h.t, h.keeper.RequestFromProgram(h.ctx(), call, []byte(xwebkeeper.ProgramRequestTopic), data))
	return types.Observation{Origin: types.OriginProgram, NetworkID: h.keeper.GetParams(h.ctx()).KernelNetworkId,
		ProgramID: call.ProgramID, RequestID: requestID, Kind: types.KindFetch, PayloadHash: types.Keccak(payload),
		ContentDigest: types.Hash32(digest), FullLength: fullLength, Response: bytes.Repeat([]byte{0x41, 0x00, 0xfe}, 800)}
}

func (h *harness) fulfilProgram(observation types.Observation) {
	h.t.Helper()
	set := h.keeper.GetAttestorSet(h.ctx())
	require.Equal(h.t, uint32(2), set.Threshold)
	raw := signProgramDispatchObservation(h.t, h, observation, int(set.Threshold))
	out, _, reason := h.call(stranger, xweb.FulfilProgramMethod, nil, supplied, raw)
	require.Empty(h.t, reason)
	require.Empty(h.t, out)
}

func (h *harness) getProgramResult(observation types.Observation) []interface{} {
	h.t.Helper()
	return h.view(xweb.GetProgramResultMethod, [32]byte(observation.ProgramID), observation.RequestID)
}

func (h *harness) getKernelNetworkId() uint32 {
	h.t.Helper()
	out := h.view(xweb.GetKernelNetworkIdMethod)
	require.Len(h.t, out, 1)
	return out[0].(uint32)
}

func TestOriginTwoRoundTripThroughThePrecompile(t *testing.T) {
	h := newProgramDispatchHarness(t)
	require.Equal(t, common.HexToAddress("0x1019"), precompile)
	observation := h.RequestFromProgram()
	record, found, err := h.keeper.GetProgramPendingRecord(h.ctx(), observation.ProgramID, observation.RequestID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, observation.Kind, record.Kind)
	require.Equal(t, observation.PayloadHash, record.PayloadHash)
	require.Equal(t, originTwoFeeAsset, record.FeeAsset)
	require.Equal(t, xwebkeeper.ProgramFeeAccountID(originTwoFeeAsset), record.FeeAccount)
	require.True(t, sdk.NewInt(75).Equal(record.Amount))
	require.Equal(t, uint64(23), record.Sequence)
	require.False(t, record.Fulfilled)
	require.Equal(t, uint64(1), h.keeper.ProgramPendingCount(h.ctx()))
	require.Zero(t, h.keeper.ProgramCommittedCount(h.ctx()))
	denom, found := h.keeper.ProgramFeeDenom(h.ctx(), record.FeeAsset)
	require.True(t, found)
	require.Equal(t, types.ProgramFeeDenomUSDL, denom)
	bank := testkeeper.EVMTestApp.BankKeeper
	feeAccount := xwebkeeper.ProgramFeeAccount(record.FeeAsset)
	feeBefore := bank.GetAllBalances(h.ctx(), feeAccount)
	require.True(t, sdk.NewInt(75).Equal(feeBefore.AmountOf(denom)))
	before := make([]sdk.Coins, len(h.attestors))
	for i, attestor := range h.attestors {
		before[i] = bank.GetAllBalances(h.ctx(), attestor.Payout)
	}

	h.fulfilProgram(observation)
	require.Equal(t, []interface{}{[32]byte(observation.ContentDigest), observation.FullLength,
		uint32(len(observation.Response)), observation.Response}, h.getProgramResult(observation))
	for i, share := range []int64{38, 37, 0} {
		want := before[i].Add(sdk.NewCoins(sdk.NewInt64Coin(denom, share))...)
		require.True(t, want.IsEqual(bank.GetAllBalances(h.ctx(), h.attestors[i].Payout)), "attestor %d", i)
	}
	require.True(t, feeBefore.Sub(sdk.NewCoins(sdk.NewInt64Coin(denom, 75))).IsEqual(bank.GetAllBalances(h.ctx(), feeAccount)))
	fulfilled, found, err := h.keeper.GetProgramPendingRecord(h.ctx(), observation.ProgramID, observation.RequestID)
	require.NoError(t, err)
	require.True(t, found)
	record.Fulfilled = true
	require.Equal(t, record, fulfilled)
	require.Zero(t, h.keeper.ProgramPendingCount(h.ctx()))
	require.Equal(t, uint64(1), h.keeper.ProgramCommittedCount(h.ctx()))
}

func TestGetKernelNetworkIdThroughTheEVM(t *testing.T) {
	h := newProgramDispatchHarness(t)
	require.Equal(t, uint32(1), kernelNetworkID)
	require.Equal(t, kernelNetworkID, h.keeper.GetParams(h.ctx()).KernelNetworkId)
	require.Equal(t, kernelNetworkID, h.getKernelNetworkId())
}

func TestFulfilProgramEmitsNoEVMLog(t *testing.T) {
	h := newProgramDispatchHarness(t)
	observation := h.RequestFromProgram()
	require.Empty(t, h.stateDB.GetAllLogs())
	h.fulfilProgram(observation)
	require.Empty(t, h.stateDB.GetAllLogs())
	events := h.ctx().EventManager().Events()
	require.NotEmpty(t, events)
	for _, event := range events {
		require.NotContains(t, strings.ToLower(event.Type), "xweb")
	}
	require.Equal(t, []interface{}{[32]byte(observation.ContentDigest), observation.FullLength,
		uint32(len(observation.Response)), observation.Response}, h.getProgramResult(observation))
	require.Empty(t, h.stateDB.GetAllLogs())
}
