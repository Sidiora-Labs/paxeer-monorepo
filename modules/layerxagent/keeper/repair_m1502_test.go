package keeper_test

import (
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov"
	layerxgovtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func repairM1502Proposal(t *testing.T, h *harness, params types.Params) error {
	t.Helper()
	proposal, err := layerxgovtypes.NewLayerXProposal("Scheduler gas limits", "Preserve pending scheduler jobs",
		&types.MsgUpdateParams{Authority: layerxgovtypes.GovernanceAuthority(), Params: params})
	require.NoError(t, err)
	registry := cdctypes.NewInterfaceRegistry()
	govtypes.RegisterInterfaces(registry)
	layerxgovtypes.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	encoded, err := cdc.MarshalInterfaceJSON(proposal)
	require.NoError(t, err)
	var decoded govtypes.Content
	require.NoError(t, cdc.UnmarshalInterfaceJSON(encoded, &decoded))
	require.NoError(t, decoded.ValidateBasic())
	router := baseapp.NewMsgServiceRouter()
	router.SetInterfaceRegistry(registry)
	types.RegisterMsgServer(router, keeper.NewMsgServerImpl(h.keeper, layerxgovtypes.GovernanceAuthority()))
	return layerxgov.NewProposalHandler(router)(h.ctx, decoded)
}

func TestRepairM_1502_GovernanceReductionRestart(t *testing.T) {
	for _, tc := range []struct {
		name       string
		blockLimit uint64
		largeAt    uint64
	}{
		{"job cap", 120_000, dueAt},
		{"block and job caps", 50_000, dueAt},
		{"future job", 50_000, later},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := repair145Open(t)
			h := d.h
			params := keeper.DefaultParams()
			params.MaxJobGasLimit = gasLimit
			params.MaxBlockGas = 120_000
			params.MaxJobsPerBlock = 1
			require.NoError(t, repairM1502Proposal(t, h, params))
			target := h.deploy([]byte{byte(vm.STOP)})
			collector := authtypes.NewModuleAddress(authtypes.FeeCollectorName)
			creatorBefore := h.balance(h.ctx, h.creatorAcc)
			collectorBefore := h.balance(h.ctx, collector)
			escrowBefore := h.balance(h.ctx, h.keeper.ModuleAddress())
			large, err := h.keeper.Schedule(h.ctx, h.creatorAcc, h.creator, target, nil, tc.largeAt, gasLimit, deposit)
			require.NoError(t, err)
			small, err := h.keeper.Schedule(h.ctx, h.creatorAcc, h.creator, target, nil, dueAt, 30_000, deposit)
			require.NoError(t, err)
			require.Less(t, large, small)
			before := h.keeper.ExportRecoveryGenesis(h.ctx)
			escrowed := h.balance(h.ctx, h.keeper.ModuleAddress())
			creatorEscrowed := h.balance(h.ctx, h.creatorAcc)
			require.Equal(t, new(big.Int).Add(escrowBefore, new(big.Int).Mul(deposit, big.NewInt(2))), escrowed)

			proposed := params
			proposed.MaxJobGasLimit = 50_000
			proposed.MaxBlockGas = tc.blockLimit
			proposed.Paused = true
			err = repairM1502Proposal(t, h, proposed)
			require.ErrorIs(t, err, agentcommon.ErrSchedulerInvalidGasLimit)
			require.ErrorContains(t, err, "job 1 gas limit 100000")
			require.Equal(t, before, h.keeper.ExportRecoveryGenesis(h.ctx))
			require.Equal(t, creatorEscrowed, h.balance(h.ctx, h.creatorAcc))
			require.Equal(t, escrowed, h.balance(h.ctx, h.keeper.ModuleAddress()))
			require.Equal(t, collectorBefore, h.balance(h.ctx, collector))
			d.restart(h.ctx)
			require.Equal(t, before, h.keeper.ExportRecoveryGenesis(h.ctx))
			require.Equal(t, params, h.keeper.GetParams(h.ctx))
			require.Equal(t, []uint64{large, small}, h.keeper.Pending(h.ctx, h.creator))

			order := []uint64{large, small}
			heights := []uint64{dueAt, dueAt + 1}
			if tc.largeAt == later {
				order = []uint64{small, large}
				heights = []uint64{dueAt, later}
			}
			for i, id := range order {
				h.ctx = h.ctx.WithBlockHeight(int64(heights[i])).WithEventManager(sdk.NewEventManager())
				used, err := h.keeper.RunDue(h.ctx)
				require.NoError(t, err)
				require.Equal(t, intrinsic, used)
				require.Empty(t, failedEvents(t, h.ctx))
				require.Equal(t, []*types.EventJobExecuted{executed(id, h.creator, intrinsic, deposit.Int64()-int64(intrinsic)*baseFee)}, executedEvents(t, h.ctx))
				_, found := h.keeper.GetJob(h.ctx, id)
				require.False(t, found)
				remaining := new(big.Int).Mul(deposit, big.NewInt(int64(1-i)))
				fee := new(big.Int).Mul(big.NewInt(int64(i+1)*int64(intrinsic)), big.NewInt(baseFee))
				require.Equal(t, new(big.Int).Add(escrowBefore, remaining), h.balance(h.ctx, h.keeper.ModuleAddress()))
				require.Equal(t, new(big.Int).Add(collectorBefore, fee), h.balance(h.ctx, collector))
				require.Equal(t, new(big.Int).Sub(new(big.Int).Sub(creatorBefore, remaining), fee), h.balance(h.ctx, h.creatorAcc))
				require.Empty(t, h.keeper.ExportRecoveryGenesis(h.ctx).Settlements)

				if id == large {
					proposed.Paused = false
					require.NoError(t, repairM1502Proposal(t, h, proposed))
					require.Equal(t, proposed, h.keeper.GetParams(h.ctx))
				}
				snapshot := h.keeper.ExportRecoveryGenesis(h.ctx)
				d.restart(h.ctx)
				require.Equal(t, snapshot, h.keeper.ExportRecoveryGenesis(h.ctx))
			}
			require.Empty(t, h.keeper.Pending(h.ctx, h.creator))
			require.Empty(t, proofAgentDueIDs(h, later+1))
			h.ctx = h.ctx.WithBlockHeight(int64(later + 1)).WithEventManager(sdk.NewEventManager())
			creatorAfter := h.balance(h.ctx, h.creatorAcc)
			collectorAfter := h.balance(h.ctx, collector)
			used, err := h.keeper.RunDue(h.ctx)
			require.NoError(t, err)
			require.Zero(t, used)
			require.Empty(t, executedEvents(t, h.ctx))
			require.Empty(t, failedEvents(t, h.ctx))
			require.Equal(t, creatorAfter, h.balance(h.ctx, h.creatorAcc))
			require.Equal(t, collectorAfter, h.balance(h.ctx, collector))
			require.Equal(t, escrowBefore, h.balance(h.ctx, h.keeper.ModuleAddress()))
		})
	}
}

func TestRepairM_1502_CompatibleReduction(t *testing.T) {
	h := newHarness(t)
	target := h.deploy([]byte{byte(vm.STOP)})
	id := h.schedule(target, later)
	job, found := h.keeper.GetJob(h.ctx, id)
	require.True(t, found)
	escrow := h.balance(h.ctx, h.keeper.ModuleAddress())
	params := h.keeper.GetParams(h.ctx)
	params.MaxJobGasLimit = job.GasLimit
	params.MaxBlockGas = job.GasLimit
	require.NoError(t, repairM1502Proposal(t, h, params))
	require.Equal(t, params, h.keeper.GetParams(h.ctx))
	stored, found := h.keeper.GetJob(h.ctx, id)
	require.True(t, found)
	require.Equal(t, job, stored)
	require.Equal(t, escrow, h.balance(h.ctx, h.keeper.ModuleAddress()))
	params.MaxJobGasLimit--
	params.MaxBlockGas--
	require.ErrorIs(t, h.keeper.SetParams(h.ctx, params), agentcommon.ErrSchedulerInvalidGasLimit)
	refund, err := h.keeper.Cancel(h.ctx, h.creator, id)
	require.NoError(t, err)
	require.Equal(t, deposit, refund.BigInt())
	require.NoError(t, repairM1502Proposal(t, h, params))
	require.Equal(t, params, h.keeper.GetParams(h.ctx))
	require.Zero(t, h.balance(h.ctx, h.keeper.ModuleAddress()).Sign())
}
