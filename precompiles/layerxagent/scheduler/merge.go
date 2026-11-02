package scheduler

import (
	"fmt"
	"strings"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	agentkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

type LayerXAgentKeepers interface {
	LayerXAgentK() *agentkeeper.Keeper
}

const jobOutcomeABI = `[
{"type":"event","name":"JobExecuted","anonymous":false,"inputs":[{"name":"jobId","type":"uint256","indexed":true},{"name":"creator","type":"address","indexed":true},{"name":"gasUsed","type":"uint64","indexed":false},{"name":"refund","type":"uint256","indexed":false}]},
{"type":"event","name":"JobFailed","anonymous":false,"inputs":[{"name":"jobId","type":"uint256","indexed":true},{"name":"creator","type":"address","indexed":true},{"name":"reason","type":"string","indexed":false}]}
]`

func init() {
	if err := precompiles.RegisterMergeConstructor(common.HexToAddress(SchedulerAddress), NewMergePrecompile); err != nil {
		panic(err)
	}
}

func NewMergePrecompile(keepers utils.Keepers, planName string) (precompiles.IPrecompile, error) {
	if keepers == nil {
		return nil, fmt.Errorf("%w: precompile keepers are unavailable", agentcommon.ErrNoContext)
	}
	wired, ok := keepers.(LayerXAgentKeepers)
	if !ok {
		return nil, fmt.Errorf("%w: scheduler keeper is unavailable", agentcommon.ErrNoContext)
	}
	keeper := wired.LayerXAgentK()
	if keeper == nil {
		return nil, fmt.Errorf("%w: scheduler keeper is unavailable", agentcommon.ErrNoContext)
	}
	evm, ok := keepers.EVMK().(*evmkeeper.Keeper)
	if !ok || evm == nil {
		return nil, fmt.Errorf("%w: evm keeper is unavailable", agentcommon.ErrNoContext)
	}
	p, err := NewPrecompile(*keeper, evm, evm.UpgradeKeeper(), planName)
	if err != nil {
		return nil, err
	}
	outcomes, err := abi.JSON(strings.NewReader(jobOutcomeABI))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", agentcommon.ErrABIDecode, err)
	}
	for name, event := range outcomes.Events {
		p.Events[name] = event
	}
	return p, nil
}
