package keeper_test

import (
	"bytes"
	"math/big"
	"reflect"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	layerxgovtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func agentDueJobs(h *harness, at uint64) []keeper.Job {
	var jobs []keeper.Job
	h.keeper.IterateDue(h.ctx, at, func(job keeper.Job) bool {
		jobs = append(jobs, job)
		return false
	})
	return jobs
}

func TestAgentScheduleCallDataCap(t *testing.T) {
	for _, tc := range []struct {
		name       string
		size       int
		zeroTarget bool
		gas        uint64
		wantErr    error
	}{
		{name: "empty", size: 0, gas: gasLimit},
		{name: "below cap", size: 3, gas: gasLimit},
		{name: "at cap", size: 4, gas: gasLimit},
		{name: "above cap", size: 5, gas: gasLimit, wantErr: agentcommon.ErrSchedulerCallDataTooLarge},
		{name: "cap precedes gas check", size: 5, gas: 0, wantErr: agentcommon.ErrSchedulerCallDataTooLarge},
		{name: "target precedes cap", size: 5, zeroTarget: true, gas: gasLimit, wantErr: agentcommon.ErrSchedulerInvalidTarget},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			params := keeper.DefaultParams()
			params.MaxCallDataBytes = 4
			require.NoError(t, h.keeper.SetParams(h.ctx, params))
			target := h.deploy([]byte{byte(vm.STOP)})
			if tc.zeroTarget {
				target = common.Address{}
			}
			callData := bytes.Repeat([]byte{0xab}, tc.size)
			creatorBefore := h.balance(h.ctx, h.creatorAcc)
			moduleBefore := h.balance(h.ctx, h.keeper.ModuleAddress())
			nextID := h.keeper.NextJobID(h.ctx)
			id, err := h.keeper.Schedule(h.ctx, h.creatorAcc, h.creator, target, callData, dueAt, tc.gas, deposit)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				require.Zero(t, id)
				require.Equal(t, creatorBefore, h.balance(h.ctx, h.creatorAcc))
				require.Equal(t, moduleBefore, h.balance(h.ctx, h.keeper.ModuleAddress()))
				require.Equal(t, nextID, h.keeper.NextJobID(h.ctx))
				require.Empty(t, h.keeper.Pending(h.ctx, h.creator))
				require.Empty(t, agentDueJobs(h, dueAt))
				_, found := h.keeper.GetJob(h.ctx, nextID)
				require.False(t, found)
				return
			}
			require.NoError(t, err)
			require.Equal(t, nextID, id)
			job, found := h.keeper.GetJob(h.ctx, id)
			require.True(t, found)
			require.True(t, bytes.Equal(callData, job.CallData))
			require.Equal(t, new(big.Int).Sub(creatorBefore, deposit), h.balance(h.ctx, h.creatorAcc))
			require.Equal(t, new(big.Int).Add(moduleBefore, deposit), h.balance(h.ctx, h.keeper.ModuleAddress()))
		})
	}
}

func TestAgentPausedRefusesScheduleAllowsCancel(t *testing.T) {
	h := newHarness(t)
	target := h.deploy([]byte{byte(vm.STOP)})
	creatorBefore := h.balance(h.ctx, h.creatorAcc)
	moduleBefore := h.balance(h.ctx, h.keeper.ModuleAddress())
	id := h.schedule(target, dueAt)
	require.NoError(t, h.keeper.Reschedule(h.ctx, h.creator, id, later))
	jobBefore, found := h.keeper.GetJob(h.ctx, id)
	require.True(t, found)
	params := h.keeper.GetParams(h.ctx)
	params.Paused = true
	require.NoError(t, h.keeper.SetParams(h.ctx, params))
	creatorEscrowed := h.balance(h.ctx, h.creatorAcc)
	moduleEscrowed := h.balance(h.ctx, h.keeper.ModuleAddress())
	nextID := h.keeper.NextJobID(h.ctx)

	newID, err := h.keeper.Schedule(h.ctx, h.creatorAcc, h.creator, target, nil, dueAt, gasLimit, deposit)
	require.ErrorIs(t, err, layerxgovtypes.ErrPausedScope)
	require.ErrorContains(t, err, types.ModuleName+".schedule")
	require.Zero(t, newID)
	err = h.keeper.Reschedule(h.ctx, h.creator, id, dueAt)
	require.ErrorIs(t, err, layerxgovtypes.ErrPausedScope)
	require.ErrorContains(t, err, types.ModuleName+".reschedule")
	require.ErrorIs(t, h.keeper.Reschedule(h.ctx, h.creator, nextID, dueAt), layerxgovtypes.ErrPausedScope)
	jobAfter, found := h.keeper.GetJob(h.ctx, id)
	require.True(t, found)
	require.Equal(t, jobBefore, jobAfter)
	require.Equal(t, []uint64{id}, h.keeper.Pending(h.ctx, h.creator))
	require.Empty(t, agentDueJobs(h, dueAt))
	require.Equal(t, []keeper.Job{jobBefore}, agentDueJobs(h, later))
	require.Equal(t, nextID, h.keeper.NextJobID(h.ctx))
	require.Equal(t, creatorEscrowed, h.balance(h.ctx, h.creatorAcc))
	require.Equal(t, moduleEscrowed, h.balance(h.ctx, h.keeper.ModuleAddress()))

	refund, err := h.keeper.Cancel(h.ctx, h.creator, id)
	require.NoError(t, err)
	require.Equal(t, deposit, refund.BigInt())
	require.Equal(t, creatorBefore, h.balance(h.ctx, h.creatorAcc))
	require.Equal(t, moduleBefore, h.balance(h.ctx, h.keeper.ModuleAddress()))
	_, found = h.keeper.GetJob(h.ctx, id)
	require.False(t, found)
	require.Empty(t, h.keeper.Pending(h.ctx, h.creator))
	require.Empty(t, agentDueJobs(h, later))
	require.True(t, h.keeper.GetParams(h.ctx).Paused)
}

func TestAgentParamsAliasTypesParams(t *testing.T) {
	h := newHarness(t)
	var keeperParams keeper.Params = types.DefaultParams()
	var sharedParams types.Params = keeperParams
	require.Equal(t, reflect.TypeOf(sharedParams), reflect.TypeOf(keeperParams))
	require.Equal(t, types.ModuleName, keeper.ModuleName)
	require.Equal(t, types.StoreKey, keeper.StoreKey)
	require.Equal(t, types.DefaultParams(), keeper.DefaultParams())
	require.Equal(t, keeperParams, h.keeper.GetParams(h.ctx))

	h.ctx.KVStore(h.key).Set(keeper.ParamsKey, []byte(`{
		"max_jobs_per_creator": 7,
		"max_schedule_horizon": 123,
		"min_deposit_factor": 3,
		"max_jobs_per_block": 5,
		"max_job_gas_limit": 200000,
		"max_reason_bytes": 17
	}`))
	want := types.DefaultParams()
	want.MaxJobsPerCreator = 7
	want.MaxScheduleHorizon = 123
	want.MinDepositFactor = 3
	want.MaxJobsPerBlock = 5
	want.MaxJobGasLimit = 200_000
	want.MaxReasonBytes = 17
	got := h.keeper.GetParams(h.ctx)
	require.Equal(t, want, got)
	require.Equal(t, types.DefaultMaxCallDataBytes, got.MaxCallDataBytes)
	require.Equal(t, types.DefaultMaxBlockGas, got.MaxBlockGas)
	require.False(t, got.Paused)
	require.NoError(t, got.Validate())

	want.MaxCallDataBytes = 32
	want.MaxBlockGas = 400_000
	want.Paused = true
	require.NoError(t, h.keeper.SetParams(h.ctx, want))
	require.Equal(t, want, h.keeper.GetParams(h.ctx))
	invalid := want
	invalid.MaxCallDataBytes = 0
	require.Error(t, h.keeper.SetParams(h.ctx, invalid))
	invalid = want
	invalid.MaxBlockGas = invalid.MaxJobGasLimit - 1
	require.Error(t, h.keeper.SetParams(h.ctx, invalid))
	require.Equal(t, want, h.keeper.GetParams(h.ctx))
}
