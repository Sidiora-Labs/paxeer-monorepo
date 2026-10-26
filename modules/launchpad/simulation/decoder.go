package simulation

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
)

func NewDecodeStore() func(kvA, kvB kv.Pair) string {
	return func(kvA, kvB kv.Pair) string {
		return fmt.Sprintf("%s\n%s", decodeStoreValue(kvA), decodeStoreValue(kvB))
	}
}

func decodeStoreValue(pair kv.Pair) string {
	if len(pair.Key) == 0 {
		panic("launchpad: empty store key")
	}
	switch pair.Key[0] {
	case types.ParamsKey[0]:
		return fmt.Sprintf("Params: %+v", decodeJSON[types.Params](pair.Value))
	case types.MarketCountKey[0]:
		if len(pair.Value) != 8 {
			panic(fmt.Sprintf("launchpad: invalid market count length %d", len(pair.Value)))
		}
		return fmt.Sprintf("MarketCount: %d", binary.BigEndian.Uint64(pair.Value))
	case types.MarketPrefix[0]:
		return fmt.Sprintf("Market: %+v", decodeJSON[types.Market](pair.Value))
	case types.SurfacePrefix[0]:
		return fmt.Sprintf("Surface: %+v", decodeJSON[keeper.Surface](pair.Value))
	default:
		return fmt.Sprintf("launchpad prefix 0x%02x: %X", pair.Key[0], pair.Value)
	}
}

func decodeJSON[T any](value []byte) T {
	var decoded T
	if err := json.Unmarshal(value, &decoded); err != nil {
		panic(fmt.Errorf("launchpad: decode %T: %w", decoded, err))
	}
	return decoded
}
