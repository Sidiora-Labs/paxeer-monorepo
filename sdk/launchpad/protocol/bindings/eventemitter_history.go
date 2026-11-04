package bindings

import (
	_ "embed"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
)

//go:embed abi/eventemitter_v1.json
var historicalEventEmitterABI string

// HistoricalEventEmitterMetaData describes the retained emitter before its v2 upgrade.
var HistoricalEventEmitterMetaData = &bind.MetaData{ABI: historicalEventEmitterABI}

// EventEmitterMetadataAtHeight selects the retained contract ABI using the
// deployment's actual v2 activation height. Native 0x1017 logs always use
// NativeLaunchpadMetaData, independently of retained-contract upgrades.
func EventEmitterMetadataAtHeight(height, v2ActivationHeight uint64) *bind.MetaData {
	if height < v2ActivationHeight {
		return HistoricalEventEmitterMetaData
	}
	return EventEmitterMetaData
}
