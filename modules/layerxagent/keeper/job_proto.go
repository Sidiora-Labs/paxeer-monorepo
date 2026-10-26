package keeper

import (
	"fmt"
	"strings"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

func JobToProto(job Job) types.Job {
	return types.Job{
		Id:             job.ID,
		Creator:        job.Creator.Hex(),
		Target:         job.Target.Hex(),
		CallData:       append([]byte(nil), job.CallData...),
		ExecuteAtBlock: job.ExecuteAtBlock,
		GasLimit:       job.GasLimit,
		Deposit:        job.Deposit.String(),
		Active:         job.Active,
	}
}

func JobFromProto(job types.Job) (Job, error) {
	if !strings.HasPrefix(job.Creator, "0x") || !common.IsHexAddress(job.Creator) {
		return Job{}, fmt.Errorf("layerxagent job %d: creator must be a 0x hex address", job.Id)
	}
	if !strings.HasPrefix(job.Target, "0x") || !common.IsHexAddress(job.Target) {
		return Job{}, fmt.Errorf("layerxagent job %d: target must be a 0x hex address", job.Id)
	}
	if job.Deposit == "" || strings.IndexFunc(job.Deposit, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return Job{}, fmt.Errorf("layerxagent job %d: deposit must be a nonnegative integer", job.Id)
	}
	decimal := strings.TrimLeft(job.Deposit, "0")
	if decimal == "" {
		decimal = "0"
	}
	deposit, ok := sdk.NewIntFromString(decimal)
	if !ok || deposit.IsNegative() {
		return Job{}, fmt.Errorf("layerxagent job %d: deposit must be a nonnegative integer", job.Id)
	}
	return Job{
		ID:             job.Id,
		Creator:        common.HexToAddress(job.Creator),
		Target:         common.HexToAddress(job.Target),
		CallData:       append([]byte(nil), job.CallData...),
		ExecuteAtBlock: job.ExecuteAtBlock,
		GasLimit:       job.GasLimit,
		Deposit:        deposit,
		Active:         job.Active,
	}, nil
}
