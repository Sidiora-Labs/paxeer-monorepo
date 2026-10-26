package spot

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/ethereum/go-ethereum/common"
)

// NewServedSet wraps every spot rail for dynamic gas dispatch.
func NewServedSet(keepers utils.Keepers) (map[common.Address]*Precompile, error) {
	set, err := NewPrecompileSet(keepers)
	if err != nil {
		return nil, err
	}
	served := make(map[common.Address]*Precompile, len(set))
	for address, base := range set {
		current, err := newPrecompile(base)
		if err != nil {
			return nil, err
		}
		served[address] = current
	}
	return served, nil
}

// GetVersioned keys every address of NewServedSet to latestUpgrade and to
// v6.12.0, the plan that brings the set online.
func GetVersioned(latestUpgrade string, keepers utils.Keepers) (map[common.Address]utils.VersionedPrecompiles, error) {
	set, err := NewServedSet(keepers)
	if err != nil {
		return nil, err
	}
	versioned := make(map[common.Address]utils.VersionedPrecompiles, len(set))
	for address, current := range set {
		versioned[address] = utils.VersionedPrecompiles{
			latestUpgrade: current,
			"v6.12.0":     current,
		}
	}
	return versioned, nil
}
