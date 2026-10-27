package xweb_test

import (
	"bytes"
	"encoding/binary"
	"math/big"
	"testing"

	xwebkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/keeper"
	xwebtestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/testutil"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/xweb"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func newProgramDispatchHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	testkeeper.EVMTestApp.UpgradeKeeper.SetDone(h.ctx(), xwebkeeper.ProgramOriginPlan)
	require.True(t, h.keeper.ProgramOriginActive(h.ctx()))
	return h
}

func programDispatchObservation(t *testing.T, h *harness, resp []byte, signatures int) (types.Observation, []byte) {
	t.Helper()
	programID, asset := types.Hash32{0x71, 0x32}, types.Hash32{0x19}
	id := uint64(17)
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
	data := binary.BigEndian.AppendUint64(nil, id)
	data = append(data, types.KindSearch)
	data = binary.BigEndian.AppendUint32(data, uint32(len(payload)))
	data = append(data, payload...)
	require.NoError(t, h.keeper.RequestFromProgram(h.ctx(), call, []byte(xwebkeeper.ProgramRequestTopic), data))
	observation := types.Observation{Origin: types.OriginProgram,
		NetworkID: h.keeper.GetParams(h.ctx()).KernelNetworkId, ProgramID: programID, RequestID: id,
		Kind: types.KindSearch, PayloadHash: types.Keccak(payload), ContentDigest: types.Keccak(resp),
		FullLength: uint32(len(resp) + 19), Response: resp}
	raw := signProgramDispatchObservation(t, h, observation, signatures)
	return observation, raw
}

func signProgramDispatchObservation(t *testing.T, h *harness, observation types.Observation, signatures int) []byte {
	t.Helper()
	attestation := types.Attestation{Origin: observation.Origin,
		NetworkID: new(big.Int).SetUint64(uint64(observation.NetworkID)), Requester: observation.ProgramID,
		RequestID: observation.RequestID, Kind: observation.Kind, PayloadHash: observation.PayloadHash,
		ContentDigest: observation.ContentDigest, ResponseHash: types.Keccak(observation.Response), FullLength: observation.FullLength}
	observation.Signatures = xwebtestutil.Sign(types.Digest(attestation), h.attestors[:signatures]...)
	raw, err := types.EncodeObservation(observation)
	require.NoError(t, err)
	return raw
}

func requireProgramDispatchPending(t *testing.T, h *harness, observation types.Observation) {
	t.Helper()
	record, found, err := h.keeper.GetProgramPendingRecord(h.ctx(), observation.ProgramID, observation.RequestID)
	require.NoError(t, err)
	require.True(t, found)
	require.False(t, record.Fulfilled)
	require.Equal(t, uint64(1), h.keeper.ProgramPendingCount(h.ctx()))
	require.Zero(t, h.keeper.ProgramCommittedCount(h.ctx()))
	_, err = h.keeper.GetProgramResult(h.ctx(), observation.ProgramID, observation.RequestID)
	require.EqualError(t, err, "LXP_UNKNOWN_FIELD")
}

func TestFulfilProgramThroughTheEVMCommitsTheAnswer(t *testing.T) {
	h := newProgramDispatchHarness(t)
	observation, raw := programDispatchObservation(t, h, response, 2)
	bank := testkeeper.EVMTestApp.BankKeeper
	before := make([]sdk.Int, len(h.attestors))
	for i, attestor := range h.attestors {
		before[i] = bank.GetBalance(h.ctx(), attestor.Payout, types.ProgramFeeDenomPAX).Amount
	}
	ret, _, err := h.evm.Call(stranger, precompile, h.input(xweb.FulfilProgramMethod, raw), supplied, uint256.NewInt(0))
	require.NoError(t, err)
	require.Empty(t, ret)
	result, err := h.keeper.GetProgramResult(h.ctx(), observation.ProgramID, observation.RequestID)
	require.NoError(t, err)
	require.Equal(t, observation.ContentDigest, result.ContentDigest)
	require.Equal(t, observation.FullLength, result.FullLength)
	require.Equal(t, uint32(len(response)), result.ResponseLength)
	require.Equal(t, response, result.Response)
	record, found, err := h.keeper.GetProgramPendingRecord(h.ctx(), observation.ProgramID, observation.RequestID)
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, record.Fulfilled)
	require.Zero(t, h.keeper.ProgramPendingCount(h.ctx()))
	require.Equal(t, uint64(1), h.keeper.ProgramCommittedCount(h.ctx()))
	for i, share := range []int64{38, 37, 0} {
		balance := bank.GetBalance(h.ctx(), h.attestors[i].Payout, types.ProgramFeeDenomPAX).Amount
		require.True(t, balance.Equal(before[i].AddRaw(share)))
	}
	_, _, reason := h.call(stranger, xweb.FulfilProgramMethod, nil, supplied, raw)
	require.Equal(t, "LXP_SEQUENCE_REUSED", reason)
	require.Equal(t, uint64(1), h.keeper.ProgramCommittedCount(h.ctx()))
}

func TestFulfilProgramRefusesAStaticCall(t *testing.T) {
	h := newProgramDispatchHarness(t)
	observation, raw := programDispatchObservation(t, h, response, 2)
	input := h.input(xweb.FulfilProgramMethod, raw)
	ret, left, err := h.evm.StaticCall(stranger, precompile, input, supplied)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, err := abi.UnpackRevert(ret)
	require.NoError(t, err)
	require.Equal(t, "LXP_XWEB_STATICCALL: cannot call xweb fulfilProgram from staticcall", reason)
	require.Equal(t, supplied-(30000+16*uint64(len(input)-4)+8000*2), left)
	require.True(t, h.precompile.GetExecutor().(interface{ IsTransaction(string) bool }).IsTransaction(xweb.FulfilProgramMethod))
	requireProgramDispatchPending(t, h, observation)
}

func TestFulfilProgramChargesTheObservationGas(t *testing.T) {
	for _, tc := range []struct {
		name       string
		signatures int
		modify     func(types.Observation) types.Observation
		malformed  bool
		charged    uint64
		reason     string
	}{
		{name: "two_signers", signatures: 2, charged: 2},
		{name: "three_signers", signatures: 3, charged: 3},
		{name: "below_threshold", signatures: 1, charged: 1, reason: "LXP_ATTESTATION_THRESHOLD"},
		{name: "wrong_network", signatures: 2, charged: 2, reason: "LXP_WRONG_NETWORK",
			modify: func(o types.Observation) types.Observation { o.NetworkID++; return o }},
		{name: "unknown_request", signatures: 2, charged: 2, reason: "LXP_UNKNOWN_FIELD",
			modify: func(o types.Observation) types.Observation { o.RequestID++; return o }},
		{name: "malformed_observation", signatures: 2, malformed: true, charged: 16, reason: "LXP_NON_CANONICAL"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newProgramDispatchHarness(t)
			observation, raw := programDispatchObservation(t, h, response, tc.signatures)
			if tc.modify != nil {
				raw = signProgramDispatchObservation(t, h, tc.modify(observation), tc.signatures)
			}
			if tc.malformed {
				raw = raw[:types.ObservationHeaderBytes]
			}
			input := h.input(xweb.FulfilProgramMethod, raw)
			want := uint64(30000) + 16*uint64(len(input)-4) + 8000*tc.charged
			require.Equal(t, want, h.precompile.RequiredGas(input))
			_, left, err := h.evm.Call(stranger, precompile, input, want-1, uint256.NewInt(0))
			require.ErrorIs(t, err, vm.ErrOutOfGas)
			require.Zero(t, left)
			requireProgramDispatchPending(t, h, observation)
			out, left, reason := h.call(stranger, xweb.FulfilProgramMethod, nil, supplied, raw)
			require.Equal(t, tc.reason, reason)
			require.Equal(t, supplied-want, left)
			if tc.reason == "" {
				require.Empty(t, out)
				require.Equal(t, uint64(1), h.keeper.ProgramCommittedCount(h.ctx()))
			} else {
				requireProgramDispatchPending(t, h, observation)
			}
		})
	}
}

func TestGetProgramResultThroughTheEVMAfterTheMerge(t *testing.T) {
	h := newProgramDispatchHarness(t)
	resp := bytes.Repeat([]byte{0x41, 0x00, 0xfe}, 800)
	observation, raw := programDispatchObservation(t, h, resp, 2)
	viewUnknown := func(programID [32]byte, id uint64) {
		input := h.input(xweb.GetProgramResultMethod, programID, id)
		ret, left, err := h.evm.StaticCall(stranger, precompile, input, supplied)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		reason, err := abi.UnpackRevert(ret)
		require.NoError(t, err)
		require.Equal(t, "LXP_UNKNOWN_FIELD", reason)
		require.Equal(t, supplied-(3000+16*uint64(len(input)-4)), left)
	}
	viewUnknown([32]byte(observation.ProgramID), observation.RequestID)
	network := h.view(xweb.GetKernelNetworkIdMethod)
	require.Equal(t, []interface{}{h.keeper.GetParams(h.ctx()).KernelNetworkId}, network)
	_, _, reason := h.call(stranger, xweb.FulfilProgramMethod, nil, supplied, raw)
	require.Empty(t, reason)
	out := h.view(xweb.GetProgramResultMethod, [32]byte(observation.ProgramID), observation.RequestID)
	require.Equal(t, []interface{}{[32]byte(observation.ContentDigest), observation.FullLength, uint32(len(resp)), resp}, out)
	viewUnknown([32]byte(observation.ProgramID), observation.RequestID+1)
	viewUnknown([32]byte{0x72}, observation.RequestID)
	require.Equal(t, common.HexToAddress("0x1019"), precompile)
}
