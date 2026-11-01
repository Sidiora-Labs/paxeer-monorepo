package keeper_test

import (
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func agentGenesisJobs(h *harness) types.GenesisState {
	params := types.DefaultParams()
	params.MaxJobsPerCreator = 7
	params.MaxCallDataBytes = 128
	params.Paused = true
	return types.GenesisState{
		Params: params,
		Jobs: []types.Job{
			{Id: 256, Creator: h.creator.Hex(), Target: common.HexToAddress("0x1234").Hex(), CallData: []byte{0, 1, 0xff}, ExecuteAtBlock: dueAt, GasLimit: gasLimit, Deposit: "2000000000017", Active: true},
			{Id: 2, Creator: common.HexToAddress("0x5678").Hex(), Target: common.HexToAddress("0x4321").Hex(), CallData: []byte{0xde, 0xad}, ExecuteAtBlock: dueAt, GasLimit: gasLimit + 1, Deposit: "1000000000029", Active: true},
			{Id: 1, Creator: h.creator.Hex(), Target: common.HexToAddress("0x8765").Hex(), ExecuteAtBlock: later, GasLimit: gasLimit - 1, Deposit: "0", Active: true},
		},
		NextJobId: 300,
	}
}

func fundAgentGenesis(t *testing.T, h *harness, amount *big.Int) {
	t.Helper()
	coins, wei := state.SplitUhpxWeiAmount(amount)
	require.NoError(t, testkeeper.EVMTestApp.BankKeeper.SendCoinsAndWei(h.ctx, h.creatorAcc, h.keeper.ModuleAddress(), coins, wei))
}

func TestAgentGenesisRoundTrip(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		h := newHarness(t)
		gs := types.DefaultGenesis()
		require.Equal(t, gs, h.keeper.ExportGenesis(h.ctx))
		require.NoError(t, h.keeper.InitGenesis(h.ctx, *gs))
		require.Equal(t, gs, h.keeper.ExportGenesis(h.ctx))
		require.True(t, h.ctx.KVStore(h.key).Has(keeper.ParamsKey))
		require.True(t, h.ctx.KVStore(h.key).Has(keeper.NextJobIDKey))
	})

	for _, surplus := range []int64{0, 1} {
		name := "exact coverage"
		if surplus != 0 {
			name = "surplus coverage"
		}
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			gs := agentGenesisJobs(h)
			fundAgentGenesis(t, h, big.NewInt(3_000_000_000_046+surplus))
			balanceBefore := h.balance(h.ctx, h.keeper.ModuleAddress())
			require.NoError(t, h.keeper.InitGenesis(h.ctx, gs))
			require.Equal(t, gs.Params, h.keeper.GetParams(h.ctx))
			require.Equal(t, gs.NextJobId, h.keeper.NextJobID(h.ctx))
			for _, protoJob := range gs.Jobs {
				want, err := keeper.JobFromProto(protoJob)
				require.NoError(t, err)
				got, found := h.keeper.GetJob(h.ctx, protoJob.Id)
				require.True(t, found)
				require.Equal(t, want, got)
			}
			require.Equal(t, []uint64{1, 256}, h.keeper.Pending(h.ctx, h.creator))
			require.Equal(t, []uint64{2}, h.keeper.Pending(h.ctx, common.HexToAddress(gs.Jobs[1].Creator)))
			require.Empty(t, agentDueJobs(h, dueAt-1))
			due := agentDueJobs(h, dueAt)
			require.Len(t, due, 2)
			require.Equal(t, uint64(2), due[0].ID)
			require.Equal(t, uint64(256), due[1].ID)
			allDue := agentDueJobs(h, later)
			require.Len(t, allDue, 3)
			require.Equal(t, uint64(1), allDue[2].ID)
			want := &types.GenesisState{Params: gs.Params, Jobs: []types.Job{gs.Jobs[2], gs.Jobs[1], gs.Jobs[0]}, NextJobId: gs.NextJobId}
			exported := h.keeper.ExportGenesis(h.ctx)
			require.Equal(t, want, exported)
			require.NoError(t, exported.Validate())
			require.Equal(t, balanceBefore, h.balance(h.ctx, h.keeper.ModuleAddress()))

			restored := newHarness(t)
			fundAgentGenesis(t, restored, balanceBefore)
			require.NoError(t, restored.keeper.InitGenesis(restored.ctx, *exported))
			require.Equal(t, exported, restored.keeper.ExportGenesis(restored.ctx))
			require.Equal(t, []uint64{1, 256}, restored.keeper.Pending(restored.ctx, h.creator))
			require.Equal(t, allDue, agentDueJobs(restored, later))
			exported.Jobs[1].CallData[0] ^= 0xff
			require.Equal(t, want, h.keeper.ExportGenesis(h.ctx))
		})
	}
}

func TestAgentInitGenesisRefusesUnderfundedEscrow(t *testing.T) {
	for _, tc := range []struct {
		name    string
		balance int64
	}{
		{"empty escrow", 0},
		{"one wei short", 3_000_000_000_045},
		{"covers each job but not their sum", 2_000_000_000_017},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			gs := agentGenesisJobs(h)
			fundAgentGenesis(t, h, big.NewInt(tc.balance))
			before := h.keeper.ExportGenesis(h.ctx)
			balanceBefore := h.balance(h.ctx, h.keeper.ModuleAddress())
			require.ErrorIs(t, h.keeper.InitGenesis(h.ctx, gs), agentcommon.ErrSchedulerEscrow)
			require.Equal(t, before, h.keeper.ExportGenesis(h.ctx))
			require.False(t, h.ctx.KVStore(h.key).Has(keeper.ParamsKey))
			require.False(t, h.ctx.KVStore(h.key).Has(keeper.NextJobIDKey))
			require.Empty(t, h.keeper.Pending(h.ctx, h.creator))
			require.Empty(t, agentDueJobs(h, later))
			require.Equal(t, balanceBefore, h.balance(h.ctx, h.keeper.ModuleAddress()))
		})
	}

	for _, nextID := range []uint64{0, 255, 256} {
		t.Run(new(big.Int).SetUint64(nextID).String(), func(t *testing.T) {
			h := newHarness(t)
			gs := agentGenesisJobs(h)
			fundAgentGenesis(t, h, big.NewInt(3_000_000_000_046))
			gs.NextJobId = nextID
			before := h.keeper.ExportGenesis(h.ctx)
			require.Error(t, h.keeper.InitGenesis(h.ctx, gs))
			require.Equal(t, before, h.keeper.ExportGenesis(h.ctx))
			require.Empty(t, agentDueJobs(h, later))
		})
	}

	t.Run("sum exceeds sdk integer range", func(t *testing.T) {
		h := newHarness(t)
		gs := agentGenesisJobs(h)
		for i := range gs.Jobs {
			gs.Jobs[i].Deposit = "115792089237316195423570985008687907853269984665640564039457584007913129639935"
		}
		require.NoError(t, gs.Validate())
		require.ErrorIs(t, h.keeper.InitGenesis(h.ctx, gs), agentcommon.ErrSchedulerEscrow)
		require.Equal(t, types.DefaultGenesis(), h.keeper.ExportGenesis(h.ctx))
	})

	t.Run("invalid params", func(t *testing.T) {
		h := newHarness(t)
		gs := types.DefaultGenesis()
		gs.Params.MaxJobsPerCreator = 0
		require.Error(t, h.keeper.InitGenesis(h.ctx, *gs))
		require.Equal(t, types.DefaultGenesis(), h.keeper.ExportGenesis(h.ctx))
	})
}
