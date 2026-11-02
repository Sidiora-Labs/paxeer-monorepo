package keeper_test

import (
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestRunDueStopsAtMaxBlockGas(t *testing.T) {
	for _, tc := range []struct {
		name     string
		blockGas uint64
		maxJobs  uint64
		heights  []uint64
		limits   []uint64
		first    []uint64
		next     []uint64
	}{
		{"reserved gas", 250_000, 64, []uint64{dueAt, dueAt, dueAt}, []uint64{gasLimit, gasLimit, gasLimit}, []uint64{1, 2}, []uint64{3}},
		{"exact gas bound", 200_000, 64, []uint64{dueAt, dueAt, dueAt}, []uint64{gasLimit, gasLimit, gasLimit}, []uint64{1, 2}, []uint64{3}},
		{"block then id order", 250_000, 64, []uint64{dueAt, dueAt - 1, dueAt - 1}, []uint64{gasLimit, gasLimit, gasLimit}, []uint64{2, 3}, []uint64{1}},
		{"stop before later smaller job", 250_000, 64, []uint64{dueAt, dueAt, dueAt, dueAt}, []uint64{gasLimit, gasLimit, gasLimit, 30_000}, []uint64{1, 2}, []uint64{3, 4}},
		{"job count bound", 400_000, 2, []uint64{dueAt, dueAt, dueAt}, []uint64{gasLimit, gasLimit, gasLimit}, []uint64{1, 2}, []uint64{3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			target := h.deploy(burner(0, []byte{byte(vm.STOP)}))
			params := keeper.DefaultParams()
			params.MaxBlockGas = tc.blockGas
			params.MaxJobGasLimit = gasLimit
			params.MaxJobsPerBlock = tc.maxJobs
			require.NoError(t, h.keeper.SetParams(h.ctx, params))
			jobs := make(map[uint64]keeper.Job)
			for i, at := range tc.heights {
				id, err := h.keeper.Schedule(h.ctx, h.creatorAcc, h.creator, target, nil, at, tc.limits[i], deposit)
				require.NoError(t, err)
				require.Equal(t, uint64(i+1), id)
				job, found := h.keeper.GetJob(h.ctx, id)
				require.True(t, found)
				jobs[id] = job
			}

			end := h.ctx.WithBlockHeight(int64(dueAt)).WithEventManager(sdk.NewEventManager())
			used, err := h.keeper.RunDue(end)
			require.NoError(t, err)
			require.Equal(t, uint64(len(tc.first))*intrinsic, used)
			require.Empty(t, failedEvents(t, end))
			var ran []uint64
			var reserved uint64
			for _, event := range executedEvents(t, end) {
				ran = append(ran, event.JobId)
				reserved += jobs[event.JobId].GasLimit
				_, found := h.keeper.GetJob(end, event.JobId)
				require.False(t, found)
			}
			require.Equal(t, tc.first, ran)
			require.LessOrEqual(t, reserved, params.MaxBlockGas)
			require.LessOrEqual(t, uint64(len(ran)), params.MaxJobsPerBlock)
			var stillDue []uint64
			h.keeper.IterateDue(end, dueAt, func(job keeper.Job) bool {
				stillDue = append(stillDue, job.ID)
				require.Equal(t, jobs[job.ID], job)
				return false
			})
			require.Equal(t, tc.next, stillDue)
			require.ElementsMatch(t, tc.next, h.keeper.Pending(end, h.creator))

			next := end.WithBlockHeight(int64(dueAt) + 1).WithEventManager(sdk.NewEventManager())
			used, err = h.keeper.RunDue(next)
			require.NoError(t, err)
			require.Equal(t, uint64(len(tc.next))*intrinsic, used)
			ran = nil
			for _, event := range executedEvents(t, next) {
				ran = append(ran, event.JobId)
			}
			require.Equal(t, tc.next, ran)
			require.Empty(t, failedEvents(t, next))
			require.Empty(t, h.keeper.Pending(next, h.creator))
			h.keeper.IterateDue(next, dueAt+1, func(job keeper.Job) bool {
				t.Fatalf("job %d is still due after execution", job.ID)
				return false
			})
			empty := next.WithBlockHeight(int64(dueAt) + 2).WithEventManager(sdk.NewEventManager())
			used, err = h.keeper.RunDue(empty)
			require.NoError(t, err)
			require.Zero(t, used)
			require.Empty(t, empty.EventManager().Events())
		})
	}
}

func TestRunDueReportsGasUsed(t *testing.T) {
	t.Run("successful jobs", func(t *testing.T) {
		h := newHarness(t)
		for _, used := range []uint64{30_000, 45_000} {
			target := h.deploy(burner(used-intrinsic, []byte{byte(vm.STOP)}))
			h.schedule(target, dueAt)
		}
		end := h.ctx.WithBlockHeight(int64(dueAt)).WithEventManager(sdk.NewEventManager())
		collector := authtypes.NewModuleAddress(authtypes.FeeCollectorName)
		before := h.balance(end, collector)
		used, err := h.keeper.RunDue(end)
		require.NoError(t, err)
		require.Equal(t, uint64(75_000), used)
		events := executedEvents(t, end)
		require.Len(t, events, 2)
		require.Equal(t, uint64(30_000), events[0].GasUsed)
		require.Equal(t, uint64(45_000), events[1].GasUsed)
		require.Equal(t, used, events[0].GasUsed+events[1].GasUsed)
		require.Empty(t, failedEvents(t, end))
		fee := new(big.Int).Sub(h.balance(end, collector), before)
		require.Equal(t, new(big.Int).Mul(new(big.Int).SetUint64(used), big.NewInt(baseFee)), fee)
		require.Empty(t, h.keeper.Pending(end, h.creator))
	})

	t.Run("failed jobs", func(t *testing.T) {
		h := newHarness(t)
		h.schedule(h.deploy(reverter(t, 30_000-intrinsic-42, "boom")), dueAt)
		h.schedule(h.deploy(burner(gasLimit, []byte{byte(vm.STOP)})), dueAt)
		end := h.ctx.WithBlockHeight(int64(dueAt)).WithEventManager(sdk.NewEventManager())
		collector := authtypes.NewModuleAddress(authtypes.FeeCollectorName)
		before := h.balance(end, collector)
		used, err := h.keeper.RunDue(end)
		require.NoError(t, err)
		require.Equal(t, uint64(30_000)+gasLimit, used)
		require.Empty(t, executedEvents(t, end))
		events := failedEvents(t, end)
		require.Len(t, events, 2)
		require.Equal(t, failed(1, h.creator, "boom"), events[0])
		require.Equal(t, failed(2, h.creator, "out of gas"), events[1])
		fee := new(big.Int).Sub(h.balance(end, collector), before)
		require.Equal(t, new(big.Int).Mul(new(big.Int).SetUint64(used), big.NewInt(baseFee)), fee)
		require.Empty(t, h.keeper.Pending(end, h.creator))
	})
}

func TestRunDuePausedRunsNothing(t *testing.T) {
	h := newHarness(t)
	target := h.deploy(burner(30_000-intrinsic, []byte{byte(vm.STOP)}))
	id := h.schedule(target, dueAt)
	future := h.schedule(target, later)
	job, found := h.keeper.GetJob(h.ctx, id)
	require.True(t, found)
	params := keeper.DefaultParams()
	params.Paused = true
	require.NoError(t, h.keeper.SetParams(h.ctx, params))
	end := h.ctx.WithBlockHeight(int64(dueAt)).WithEventManager(sdk.NewEventManager())
	collector := authtypes.NewModuleAddress(authtypes.FeeCollectorName)
	collectorBefore := h.balance(end, collector)
	creatorBefore := h.balance(end, h.creatorAcc)
	escrowBefore := h.balance(end, h.keeper.ModuleAddress())
	used, err := h.keeper.RunDue(end)
	require.NoError(t, err)
	require.Zero(t, used)
	require.NoError(t, h.keeper.EndBlocker(end))
	require.Empty(t, end.EventManager().Events())
	stored, found := h.keeper.GetJob(end, id)
	require.True(t, found)
	require.Equal(t, job, stored)
	require.Equal(t, []uint64{id, future}, h.keeper.Pending(end, h.creator))
	require.Equal(t, collectorBefore, h.balance(end, collector))
	require.Equal(t, creatorBefore, h.balance(end, h.creatorAcc))
	require.Equal(t, escrowBefore, h.balance(end, h.keeper.ModuleAddress()))
	var due []uint64
	h.keeper.IterateDue(end, dueAt, func(job keeper.Job) bool {
		due = append(due, job.ID)
		return false
	})
	require.Equal(t, []uint64{id}, due)

	params.Paused = false
	require.NoError(t, h.keeper.SetParams(end, params))
	next := end.WithBlockHeight(int64(dueAt) + 1).WithEventManager(sdk.NewEventManager())
	used, err = h.keeper.RunDue(next)
	require.NoError(t, err)
	require.Equal(t, uint64(30_000), used)
	require.Equal(t, []*types.EventJobExecuted{executed(id, h.creator, 30_000, 1_700_000)}, executedEvents(t, next))
	require.Empty(t, failedEvents(t, next))
	_, found = h.keeper.GetJob(next, id)
	require.False(t, found)
	require.Equal(t, []uint64{future}, h.keeper.Pending(next, h.creator))

	last := next.WithBlockHeight(int64(later)).WithEventManager(sdk.NewEventManager())
	require.NoError(t, h.keeper.EndBlocker(last))
	require.Equal(t, []*types.EventJobExecuted{executed(future, h.creator, 30_000, 1_700_000)}, executedEvents(t, last))
	require.Empty(t, h.keeper.Pending(last, h.creator))
}
