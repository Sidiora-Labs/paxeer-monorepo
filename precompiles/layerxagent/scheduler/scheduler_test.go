package scheduler_test

import (
	"math/big"
	"testing"

	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/scheduler"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

const (
	scheduledTopic   = "0xf946d531c7fc16d876afe80d23d130e85e58df4f16c73aec1c0deb0d21b94f34"
	cancelledTopic   = "0xa80c76c474b34cc7af71dec63d733b959fff08f4eb0789e288be5db6b608f942"
	rescheduledTopic = "0xb060fcde04def7313b0c93134d231aaaa8ac911995a00d5225e41b025cb1c5f1"
)

func TestSchedulerPrecompileSchedulePayable(t *testing.T) {
	for _, deposit := range []*big.Int{big.NewInt(200_000), big.NewInt(1_000_000_000_123)} {
		t.Run(deposit.String(), func(t *testing.T) {
			h := newSchedulerHarness(t)
			require.Equal(t, common.HexToAddress(scheduler.SchedulerAddress), h.precompile.Address())
			require.Equal(t, "scheduler", h.precompile.GetName())
			input := h.input("schedule", h.target, []byte{1}, uint64(schedulerHeight+10), schedulerGas)
			require.Equal(t, common.FromHex("0x849ba0a3"), input[:4])
			require.Equal(t, uint64(25000+30*len(input)), h.precompile.RequiredGas(input))
			before := h.balance(h.creator)
			id := h.write(deposit, "schedule", h.target, []byte{1}, uint64(schedulerHeight+10), schedulerGas)[0].(*big.Int)
			require.Zero(t, id.Cmp(big.NewInt(1)))
			require.Zero(t, new(big.Int).Sub(before, h.balance(h.creator)).Cmp(deposit))
			require.Zero(t, h.balance(h.precompile.Address()).Sign())
			require.Zero(t, h.escrow().Cmp(deposit))
			job, found := h.keeper.GetJob(h.stateDB.Ctx(), id.Uint64())
			require.True(t, found)
			require.Equal(t, h.creator, job.Creator)
			require.Equal(t, h.target, job.Target)
			require.Equal(t, []byte{1}, job.CallData)
			require.Equal(t, uint64(schedulerHeight+10), job.ExecuteAtBlock)
			require.Equal(t, schedulerGas, job.GasLimit)
			require.Zero(t, job.Deposit.BigInt().Cmp(deposit))
			require.True(t, job.Active)
			require.Len(t, h.stateDB.GetAllLogs(), 1)
			h.log("JobScheduled", scheduledTopic, id, &h.creator, h.target, uint64(schedulerHeight+10), schedulerGas, deposit)
		})
	}
	h := newSchedulerHarness(t)
	before := h.balance(h.creator)
	for _, tc := range []struct {
		target  common.Address
		at, gas uint64
		deposit *big.Int
		code    string
	}{
		{common.Address{}, uint64(schedulerHeight + 10), schedulerGas, big.NewInt(200_000), string(agentcommon.ErrSchedulerInvalidTarget)},
		{h.target, uint64(schedulerHeight), schedulerGas, big.NewInt(200_000), string(agentcommon.ErrSchedulerPastBlock)},
		{h.target, uint64(schedulerHeight + 10), 0, big.NewInt(200_000), string(agentcommon.ErrSchedulerInvalidGasLimit)},
		{h.target, uint64(schedulerHeight + 10), schedulerGas, big.NewInt(199_999), string(agentcommon.ErrSchedulerDepositTooLow)},
	} {
		ret, err := h.call(h.creator, tc.deposit, "schedule", tc.target, []byte{1}, tc.at, tc.gas)
		requireRevert(t, ret, err, tc.code)
		require.Zero(t, h.balance(h.creator).Cmp(before))
		require.Zero(t, h.balance(h.precompile.Address()).Sign())
		require.Zero(t, h.escrow().Sign())
		require.Equal(t, uint64(1), h.keeper.NextJobID(h.stateDB.Ctx()))
		require.Empty(t, h.stateDB.GetAllLogs())
	}
}

func TestSchedulerPrecompileCancelAndReschedule(t *testing.T) {
	h := newSchedulerHarness(t)
	before := h.balance(h.creator)
	id := h.schedule()
	for _, method := range []string{"cancel", "reschedule"} {
		args := []interface{}{id}
		if method == "reschedule" {
			args = append(args, uint64(schedulerHeight+20))
		}
		ret, err := h.call(h.other, new(big.Int), method, args...)
		requireRevert(t, ret, err, string(agentcommon.ErrSchedulerUnauthorized))
		high := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 64), id)
		args[0] = high
		ret, err = h.call(h.creator, new(big.Int), method, args...)
		requireRevert(t, ret, err, string(agentcommon.ErrABIDecode))
	}
	require.Len(t, h.stateDB.GetAllLogs(), 1)
	job, found := h.keeper.GetJob(h.stateDB.Ctx(), id.Uint64())
	require.True(t, found)
	require.Equal(t, uint64(schedulerHeight+10), job.ExecuteAtBlock)
	require.Equal(t, common.FromHex("0x08e6f770"), h.input("reschedule", id, uint64(schedulerHeight+20))[:4])
	h.write(new(big.Int), "reschedule", id, uint64(schedulerHeight+20))
	job, found = h.keeper.GetJob(h.stateDB.Ctx(), id.Uint64())
	require.True(t, found)
	require.Equal(t, uint64(schedulerHeight+20), job.ExecuteAtBlock)
	require.Len(t, h.stateDB.GetAllLogs(), 2)
	h.log("JobRescheduled", rescheduledTopic, id, nil, uint64(schedulerHeight+20))
	require.Equal(t, common.FromHex("0x40e58ee5"), h.input("cancel", id)[:4])
	h.write(new(big.Int), "cancel", id)
	require.Len(t, h.stateDB.GetAllLogs(), 3)
	h.log("JobCancelled", cancelledTopic, id, &h.creator, big.NewInt(200_000))
	_, found = h.keeper.GetJob(h.stateDB.Ctx(), id.Uint64())
	require.False(t, found)
	require.Empty(t, h.keeper.Pending(h.stateDB.Ctx(), h.creator))
	require.Zero(t, h.escrow().Sign())
	require.Zero(t, h.balance(h.creator).Cmp(before))
	ret, err := h.call(h.creator, new(big.Int), "cancel", id)
	requireRevert(t, ret, err, string(agentcommon.ErrSchedulerJobNotFound))
	require.Len(t, h.stateDB.GetAllLogs(), 3)
	require.Zero(t, h.balance(h.creator).Cmp(before))
}

func TestSchedulerPrecompileViews(t *testing.T) {
	h := newSchedulerHarness(t)
	one, two := h.schedule(), h.schedule()
	require.Equal(t, common.FromHex("0xbf22c457"), h.input("getJob", one)[:4])
	require.Equal(t, common.FromHex("0x5eebea20"), h.input("pending", h.creator)[:4])
	job := h.view("getJob", one)
	require.Len(t, job, 8)
	require.Zero(t, job[0].(*big.Int).Cmp(one))
	require.Equal(t, h.creator, job[1])
	require.Equal(t, h.target, job[2])
	require.Equal(t, []byte{1}, job[3])
	require.Equal(t, uint64(schedulerHeight+10), job[4])
	require.Equal(t, schedulerGas, job[5])
	require.Zero(t, job[6].(*big.Int).Cmp(big.NewInt(200_000)))
	require.Equal(t, true, job[7])
	for _, absent := range []*big.Int{big.NewInt(9), new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 64), one)} {
		zero := h.view("getJob", absent)
		require.Len(t, zero, 8)
		require.Zero(t, zero[0].(*big.Int).Sign())
		require.Equal(t, common.Address{}, zero[1])
		require.Equal(t, common.Address{}, zero[2])
		require.Empty(t, zero[3])
		require.Equal(t, uint64(0), zero[4])
		require.Equal(t, uint64(0), zero[5])
		require.Zero(t, zero[6].(*big.Int).Sign())
		require.Equal(t, false, zero[7])
	}
	ids := h.view("pending", h.creator)[0].([]*big.Int)
	require.Len(t, ids, 2)
	require.Zero(t, ids[0].Cmp(one))
	require.Zero(t, ids[1].Cmp(two))
	require.Empty(t, h.view("pending", h.other)[0])
	require.Equal(t, []uint64{1, 2}, h.keeper.Pending(h.stateDB.Ctx(), h.creator))
	h.write(new(big.Int), "cancel", one)
	ids = h.view("pending", h.creator)[0].([]*big.Int)
	require.Len(t, ids, 1)
	require.Zero(t, ids[0].Cmp(two))
}

func TestSchedulerPrecompileWriteGuards(t *testing.T) {
	h := newSchedulerHarness(t)
	id := h.schedule()
	before := h.balance(h.creator)
	for _, tc := range []struct {
		name string
		args []interface{}
	}{
		{"schedule", []interface{}{h.target, []byte{1}, uint64(schedulerHeight + 10), schedulerGas}},
		{"cancel", []interface{}{id}},
		{"reschedule", []interface{}{id, uint64(schedulerHeight + 20)}},
	} {
		input := h.input(tc.name, tc.args...)
		ret, err := h.precompile.Run(h.evm, h.creator, h.precompile.Address(), input, new(big.Int), false, true, nil)
		requireRevert(t, ret, err, string(agentcommon.ErrDelegatecall))
		ret, _, err = h.evm.StaticCall(h.creator, h.precompile.Address(), input, callGas)
		requireRevert(t, ret, err, string(agentcommon.ErrReadOnly))
		if tc.name != "schedule" {
			ret, err = h.call(h.creator, big.NewInt(1), tc.name, tc.args...)
			requireRevert(t, ret, err, string(agentcommon.ErrNonPayable))
		}
	}
	require.Len(t, h.stateDB.GetAllLogs(), 1)
	require.Zero(t, h.balance(h.creator).Cmp(before))
	params := h.keeper.GetParams(h.stateDB.Ctx())
	params.Paused = true
	require.NoError(t, h.keeper.SetParams(h.stateDB.Ctx(), params))
	ret, err := h.call(h.creator, big.NewInt(200_000), "schedule", h.target, []byte{1}, uint64(schedulerHeight+10), schedulerGas)
	requireRevert(t, ret, err, "LXP_PAUSED_SCOPE")
	ret, err = h.call(h.creator, new(big.Int), "reschedule", id, uint64(schedulerHeight+20))
	requireRevert(t, ret, err, "LXP_PAUSED_SCOPE")
	require.Len(t, h.stateDB.GetAllLogs(), 1)
	require.Zero(t, h.balance(h.creator).Cmp(before))
	require.Zero(t, h.balance(h.precompile.Address()).Sign())
	require.Zero(t, h.escrow().Cmp(big.NewInt(200_000)))
	require.Equal(t, true, h.view("getJob", id)[7])
	require.Len(t, h.view("pending", h.creator)[0], 1)
	h.write(new(big.Int), "cancel", id)
	require.Zero(t, h.escrow().Sign())
	require.Empty(t, h.view("pending", h.creator)[0])
	ctx := h.stateDB.Ctx().WithBlockHeight(schedulerHeight - 1)
	h.stateDB.WithCtx(ctx)
	ret, err = h.precompile.Run(h.evm, h.creator, h.precompile.Address(), h.input("getJob", id), new(big.Int), true, false, nil)
	requireRevert(t, ret, err, string(agentcommon.ErrNotActive))
}
