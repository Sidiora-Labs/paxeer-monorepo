package oracle

import (
	"fmt"
	"math/big"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	exchangetypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxexchange/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	storekeys "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/ethereum/go-ethereum/common"
)

const (
	MergeMaxFeeds            = 32
	mergeMaxConfidence int64 = 1_000_000_000_000_000_000
)

// MergeMaxConfidence exposes the rehearsal cap for callers. Constructor policy
// uses the immutable value so mutation of this big.Int cannot alter activation.
var MergeMaxConfidence = big.NewInt(mergeMaxConfidence)

func init() {
	if err := precompiles.RegisterMergeConstructor(common.HexToAddress(OracleAddress), NewMergePrecompile); err != nil {
		panic(err)
	}
}

func NewMergePrecompile(keepers utils.Keepers, planName string) (precompiles.IPrecompile, error) {
	if keepers == nil {
		return nil, fmt.Errorf("%w: precompile keepers are unavailable", agentcommon.ErrNoContext)
	}
	evmKeeper, ok := keepers.EVMK().(*evmkeeper.Keeper)
	if !ok || evmKeeper == nil {
		return nil, fmt.Errorf("%w: evm keeper is unavailable", agentcommon.ErrNoContext)
	}
	hostKeepers, ok := keepers.(interface{ LayerXHost() *lxphost.Host })
	if !ok || hostKeepers.LayerXHost() == nil {
		return nil, fmt.Errorf("%w: committed oracle host is unavailable", agentcommon.ErrNoContext)
	}
	live, err := NewKeeper(hostKeepers.LayerXHost(), storekeys.LayerXHostStoreKey, exchangetypes.StoreKey)
	if err != nil {
		return nil, err
	}
	p, err := NewPrecompile(live, evmKeeper, evmKeeper.UpgradeKeeper(), planName, MergeMaxFeeds, big.NewInt(mergeMaxConfidence))
	if err != nil {
		return nil, err
	}
	return p, nil
}
