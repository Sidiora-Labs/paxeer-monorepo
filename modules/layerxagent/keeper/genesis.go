package keeper

import (
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/prefix"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func (k Keeper) InitGenesis(ctx sdk.Context, gs types.GenesisState) error {
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
	if err := k.SetParams(ctx, gs.Params); err != nil {
		return err
	}
	for _, job := range jobs {
		k.SetJob(ctx, job)
	}
	k.setNextJobID(ctx, gs.NextJobId)
	return nil
}

func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
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
