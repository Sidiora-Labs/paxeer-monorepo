package types

import (
	"fmt"
	"strings"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

func DefaultGenesis() *GenesisState {
	return &GenesisState{Params: DefaultParams(), NextJobId: 1}
}

func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return err
	}
	if gs.NextJobId == 0 {
		return fmt.Errorf("layerxagent genesis: next job id must be positive")
	}
	ids := make(map[uint64]struct{}, len(gs.Jobs))
	for _, job := range gs.Jobs {
		if job.Id == 0 || job.Id >= gs.NextJobId {
			return fmt.Errorf("layerxagent genesis: job id %d must be positive and below next job id %d", job.Id, gs.NextJobId)
		}
		if _, exists := ids[job.Id]; exists {
			return fmt.Errorf("layerxagent genesis: duplicate job id %d", job.Id)
		}
		ids[job.Id] = struct{}{}
		if !strings.HasPrefix(job.Creator, "0x") || !common.IsHexAddress(job.Creator) {
			return fmt.Errorf("layerxagent genesis: job %d creator must be a 0x hex address", job.Id)
		}
		if !strings.HasPrefix(job.Target, "0x") || !common.IsHexAddress(job.Target) {
			return fmt.Errorf("layerxagent genesis: job %d target must be a 0x hex address", job.Id)
		}
		if common.HexToAddress(job.Target) == (common.Address{}) {
			return fmt.Errorf("layerxagent genesis: job %d target must be nonzero", job.Id)
		}
		if !job.Active {
			return fmt.Errorf("layerxagent genesis: job %d must be active", job.Id)
		}
		deposit, ok := sdk.NewIntFromString(job.Deposit)
		if !ok || deposit.IsNegative() {
			return fmt.Errorf("layerxagent genesis: job %d deposit must be a nonnegative integer", job.Id)
		}
	}
	return nil
}
