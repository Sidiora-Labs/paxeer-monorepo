package keeper

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/big"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/prefix"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
)

// InitGenesis validates the decoded module state and its existing bank escrow
// before writing parameters, jobs and indexes. It does not mint deposits.
func (k Keeper) InitGenesis(ctx sdk.Context, gs types.GenesisState) error {
	if len(k.settlementIDs(ctx, 1)) != 0 {
		return fmt.Errorf("layerxagent genesis: outstanding settlements require InitRecoveryGenesis")
	}
	return k.initGenesis(ctx, gs, nil)
}

// RecoveryGenesis is the complete keeper snapshot until the owning module adds
// settlement fields to its protobuf genesis schema and wires this lifecycle.
// Legacy export refuses to discard obligations rather than silently losing them.
type RecoveryGenesis struct {
	Version     uint32             `json:"version"`
	Scheduler   types.GenesisState `json:"scheduler"`
	Settlements []Settlement       `json:"settlements"`
}

func (k Keeper) InitRecoveryGenesis(ctx sdk.Context, gs RecoveryGenesis) error {
	if gs.Version != 1 {
		return fmt.Errorf("layerxagent recovery genesis: unsupported version %d", gs.Version)
	}
	return k.initGenesis(ctx, gs.Scheduler, gs.Settlements)
}

func (k Keeper) initGenesis(ctx sdk.Context, gs types.GenesisState, settlements []Settlement) error {
	if err := gs.Validate(); err != nil {
		return err
	}
	jobs := make([]Job, len(gs.Jobs))
	total := new(big.Int)
	for i, protoJob := range gs.Jobs {
		job, err := JobFromProto(protoJob)
		if err != nil {
			return err
		}
		jobs[i] = job
		total.Add(total, job.Deposit.BigInt())
	}
	ids := make(map[uint64]bool, len(jobs)+len(settlements))
	for _, job := range jobs {
		ids[job.ID] = true
	}
	for _, obligation := range settlements {
		if obligation.JobID == 0 || obligation.JobID >= gs.NextJobId || ids[obligation.JobID] {
			return fmt.Errorf("layerxagent genesis: invalid or duplicate settlement id %d", obligation.JobID)
		}
		ids[obligation.JobID] = true
		if obligation.Fee.IsNil() || obligation.Refund.IsNil() || obligation.Fee.IsNegative() || obligation.Refund.IsNegative() ||
			new(big.Int).Add(obligation.Fee.BigInt(), obligation.Refund.BigInt()).BitLen() > 256 {
			return fmt.Errorf("layerxagent genesis: invalid settlement amounts for job %d", obligation.JobID)
		}
		if obligation.FeePaid && obligation.RefundPaid {
			return fmt.Errorf("layerxagent genesis: settlement %d has no outstanding legs", obligation.JobID)
		}
		if err := sdk.VerifyAddressFormat(obligation.RefundRecipient); err != nil {
			return fmt.Errorf("layerxagent genesis: settlement %d refund recipient: %w", obligation.JobID, err)
		}
		if !obligation.FeeRecipient.Equals(authtypes.NewModuleAddress(authtypes.FeeCollectorName)) {
			return fmt.Errorf("layerxagent genesis: settlement %d fee recipient is not the fee collector", obligation.JobID)
		}
		if !obligation.FeePaid {
			total.Add(total, obligation.Fee.BigInt())
		}
		if !obligation.RefundPaid {
			total.Add(total, obligation.Refund.BigInt())
		}
	}
	if k.evm == nil {
		return fmt.Errorf("%w: layerxagent genesis requires an EVM bank keeper", agentcommon.ErrSchedulerEscrow)
	}
	bank := k.evm.BankKeeper()
	address := k.ModuleAddress()
	coins := bank.GetBalance(ctx, address, k.evm.GetBaseDenom(ctx)).Amount
	balance := new(big.Int).Mul(coins.BigInt(), state.SdkUhpxToSweiMultiplier.BigInt())
	balance.Add(balance, bank.GetWeiBalance(ctx, address).BigInt())
	if total.Cmp(balance) > 0 {
		return fmt.Errorf("%w: layerxagent genesis deposits %s exceed module balance %s", agentcommon.ErrSchedulerEscrow, total, balance)
	}
	cached, write := ctx.CacheContext()
	if err := k.SetParams(cached, gs.Params); err != nil {
		return err
	}
	for _, job := range jobs {
		k.SetJob(cached, job)
	}
	for _, obligation := range settlements {
		k.setSettlement(cached, obligation)
	}
	k.setNextJobID(cached, gs.NextJobId)
	write()
	return nil
}

// ExportGenesis produces detached protobuf jobs in ID order, including the
// parameters and next ID needed to restore the scheduler after a restart.
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	if len(k.settlementIDs(ctx, 1)) != 0 {
		panic("layerxagent genesis: outstanding settlements require ExportRecoveryGenesis")
	}
	return k.exportRunnableGenesis(ctx)
}

func (k Keeper) ExportRecoveryGenesis(ctx sdk.Context) *RecoveryGenesis {
	gs := &RecoveryGenesis{Version: 1, Scheduler: *k.exportRunnableGenesis(ctx)}
	for _, id := range k.settlementIDs(ctx, math.MaxUint64) {
		obligation, found := k.GetSettlement(ctx, id)
		if !found {
			panic(fmt.Errorf("layerxagent genesis: missing settlement %d", id))
		}
		gs.Settlements = append(gs.Settlements, obligation)
	}
	return gs
}

func (k Keeper) exportRunnableGenesis(ctx sdk.Context) *types.GenesisState {
	gs := &types.GenesisState{Params: k.GetParams(ctx), NextJobId: k.NextJobID(ctx)}
	iterator := prefix.NewStore(k.store(ctx), JobKeyPrefix).Iterator(nil, nil)
	defer func() { _ = iterator.Close() }()
	for ; iterator.Valid(); iterator.Next() {
		id := binary.BigEndian.Uint64(iterator.Key())
		job, found := k.GetJob(ctx, id)
		if !found {
			panic(fmt.Errorf("layerxagent genesis: missing job %d", id))
		}
		gs.Jobs = append(gs.Jobs, JobToProto(job))
	}
	return gs
}
