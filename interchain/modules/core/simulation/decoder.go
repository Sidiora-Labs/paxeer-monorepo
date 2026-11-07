package simulation

import (
	"fmt"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"

	clientsim "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/simulation"
	clienttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/types"
	connectionsim "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/03-connection/simulation"
	connectiontypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/03-connection/types"
	channelsim "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/04-channel/simulation"
	channeltypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/04-channel/types"
	host "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/24-host"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/keeper"
)

// NewDecodeStore returns a decoder function closure that unmarshals the KVPair's
// Value to the corresponding ibc type.
func NewDecodeStore(k keeper.Keeper) func(kvA, kvB kv.Pair) string {
	return func(kvA, kvB kv.Pair) string {
		var allocation string
		switch string(kvA.Key) {
		case clienttypes.KeyNextClientSequence:
			allocation = "NextClientSequence"
		case connectiontypes.KeyNextConnectionSequence:
			allocation = "NextConnectionSequence"
		case channeltypes.KeyNextChannelSequence:
			allocation = "NextChannelSequence"
		}
		if allocation != "" {
			if len(kvA.Value) != 8 || len(kvB.Value) != 8 {
				panic(fmt.Sprintf("invalid %s allocation value: %s", host.ModuleName, string(kvA.Key)))
			}
			return fmt.Sprintf("%s A: %d\n%s B: %d", allocation, sdk.BigEndianToUint64(kvA.Value), allocation, sdk.BigEndianToUint64(kvB.Value))
		}

		if res, found := clientsim.NewDecodeStore(k.ClientKeeper, kvA, kvB); found {
			return res
		}

		if res, found := connectionsim.NewDecodeStore(k.Codec(), kvA, kvB); found {
			return res
		}

		if res, found := channelsim.NewDecodeStore(k.Codec(), kvA, kvB); found {
			return res
		}

		panic(fmt.Sprintf("invalid %s key prefix: %s", host.ModuleName, string(kvA.Key)))
	}
}
