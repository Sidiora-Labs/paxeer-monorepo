package simulation

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"

	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/types"
	host "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/24-host"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/exported"
	ibctmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/light-clients/07-tendermint/types"
)

var _ ClientUnmarshaler = (*keeper.Keeper)(nil)

// ClientUnmarshaler defines an interface for unmarshaling ICS02 interfaces.
type ClientUnmarshaler interface {
	MustUnmarshalClientState([]byte) exported.ClientState
	MustUnmarshalConsensusState([]byte) exported.ConsensusState
}

// NewDecodeStore returns a decoder function closure that unmarshals the KVPair's
// Value to the corresponding client type.
func NewDecodeStore(cdc ClientUnmarshaler, kvA, kvB kv.Pair) (string, bool) {
	// Split only the textual client prefix: iteration heights contain arbitrary bytes,
	// including slashes, after the third component.
	path := bytes.SplitN(kvA.Key, []byte("/"), 3)
	if len(path) != 3 || !bytes.Equal(path[0], host.KeyClientStorePrefix) || len(path[1]) == 0 {
		return "", false
	}
	key := path[2]
	if bytes.Equal(key, host.ClientStateKey()) {
		clientStateA := cdc.MustUnmarshalClientState(kvA.Value)
		clientStateB := cdc.MustUnmarshalClientState(kvB.Value)
		return fmt.Sprintf("ClientState A: %v\nClientState B: %v", clientStateA, clientStateB), true
	}

	if bytes.HasPrefix(key, []byte(ibctmtypes.KeyIterateConsensusStatePrefix)) &&
		len(key) == len(ibctmtypes.KeyIterateConsensusStatePrefix)+16 {
		height := ibctmtypes.GetHeightFromIterationKey(key)
		return fmt.Sprintf("IterationKey (%s) A: %s\nIterationKey (%s) B: %s", height, kvA.Value, height, kvB.Value), true
	}

	leaf := bytes.Split(key, []byte("/"))
	if (len(leaf) != 2 && len(leaf) != 3) || string(leaf[0]) != host.KeyConsensusStatePrefix {
		return "", false
	}
	height, err := types.ParseHeight(string(leaf[1]))
	if err != nil || height.String() != string(leaf[1]) {
		return "", false
	}
	if len(leaf) == 2 {
		consensusStateA := cdc.MustUnmarshalConsensusState(kvA.Value)
		consensusStateB := cdc.MustUnmarshalConsensusState(kvB.Value)
		return fmt.Sprintf("ConsensusState A: %v\nConsensusState B: %v", consensusStateA, consensusStateB), true
	}

	switch "/" + string(leaf[2]) {
	case string(ibctmtypes.KeyProcessedTime):
		if len(kvA.Value) != 8 || len(kvB.Value) != 8 {
			panic("invalid processed time: expected big-endian uint64")
		}
		return fmt.Sprintf("ProcessedTime A: %d\nProcessedTime B: %d", binary.BigEndian.Uint64(kvA.Value), binary.BigEndian.Uint64(kvB.Value)), true
	case string(ibctmtypes.KeyProcessedHeight):
		processedHeightA := types.MustParseHeight(string(kvA.Value))
		processedHeightB := types.MustParseHeight(string(kvB.Value))
		return fmt.Sprintf("ProcessedHeight A: %s\nProcessedHeight B: %s", processedHeightA, processedHeightB), true
	default:
		return "", false
	}
}
