package keeper_test

import (
	"math/big"
	"testing"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestRunDueWorstCaseWithinMaxBlockGas(t *testing.T) {
	h := newHarness(t)
	params := h.keeper.GetParams(h.ctx)
	require.Equal(t, uint64(64), params.MaxJobsPerBlock)
	require.Equal(t, uint64(10_000_000), params.MaxJobGasLimit)
	require.Equal(t, uint64(30_000_000), params.MaxBlockGas)

	target := h.deploy(burner(params.MaxJobGasLimit-intrinsic, []byte{byte(vm.STOP)}))
	jobDeposit := new(big.Int).SetUint64(params.MaxJobGasLimit)
	jobDeposit.Mul(jobDeposit, big.NewInt(baseFee))
	jobDeposit.Mul(jobDeposit, new(big.Int).SetUint64(params.MinDepositFactor))
	wantIDs := make([]uint64, params.MaxJobsPerBlock)
	for i := range wantIDs {
		id, err := h.keeper.Schedule(h.ctx, h.creatorAcc, h.creator, target, nil,
			dueAt, params.MaxJobGasLimit, jobDeposit)
		require.NoError(t, err)
		wantIDs[i] = uint64(i) + 1
		require.Equal(t, wantIDs[i], id)
		job, found := h.keeper.GetJob(h.ctx, id)
		require.True(t, found)
		require.Equal(t, params.MaxJobGasLimit, job.GasLimit)
		require.Equal(t, dueAt, job.ExecuteAtBlock)
	}
	require.Equal(t, wantIDs, h.keeper.Pending(h.ctx, h.creator))

	jobsPerHeight := params.MaxBlockGas / params.MaxJobGasLimit
	heights := (params.MaxJobsPerBlock + jobsPerHeight - 1) / jobsPerHeight
	var reports []uint64
	var executedIDs []uint64
	for offset := uint64(0); offset < heights; offset++ {
		end := h.ctx.WithBlockHeight(int64(dueAt + offset)).WithEventManager(sdk.NewEventManager())
		gasUsed, err := h.keeper.RunDue(end)
		require.NoError(t, err)
		reports = append(reports, gasUsed)
		require.LessOrEqual(t, gasUsed, params.MaxBlockGas, "height %d", end.BlockHeight())
		require.Empty(t, failedEvents(t, end))

		events := executedEvents(t, end)
		wantCount := min(jobsPerHeight, params.MaxJobsPerBlock-uint64(len(executedIDs)))
		require.Len(t, events, int(wantCount), "height %d", end.BlockHeight())
		var eventGas uint64
		for _, event := range events {
			require.Equal(t, params.MaxJobGasLimit, event.GasUsed, "job %d", event.JobId)
			executedIDs = append(executedIDs, event.JobId)
			eventGas += event.GasUsed
			_, found := h.keeper.GetJob(end, event.JobId)
			require.False(t, found, "executed job %d remains stored", event.JobId)
		}
		require.Equal(t, eventGas, gasUsed)
		require.Equal(t, wantCount*params.MaxJobGasLimit, gasUsed)
		require.Equal(t, wantIDs[len(executedIDs):], h.keeper.Pending(end, h.creator))
		for _, id := range wantIDs[len(executedIDs):] {
			job, found := h.keeper.GetJob(end, id)
			require.True(t, found, "deferred job %d was lost", id)
			require.Equal(t, dueAt, job.ExecuteAtBlock)
			require.Equal(t, params.MaxJobGasLimit, job.GasLimit)
		}
	}
	require.Equal(t, wantIDs, executedIDs)
	var totalGas uint64
	for _, report := range reports {
		totalGas += report
	}
	require.Equal(t, params.MaxJobsPerBlock*params.MaxJobGasLimit, totalGas)

	empty := h.ctx.WithBlockHeight(int64(dueAt + heights)).WithEventManager(sdk.NewEventManager())
	gasUsed, err := h.keeper.RunDue(empty)
	require.NoError(t, err)
	require.Zero(t, gasUsed)
	require.Empty(t, executedEvents(t, empty))
	require.Empty(t, failedEvents(t, empty))
}
