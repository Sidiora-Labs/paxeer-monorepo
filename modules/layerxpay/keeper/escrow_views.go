package keeper

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
)

var ErrEscrowCorruptState = errors.New("LXP_NON_CANONICAL: corrupt escrow state")

func decodeEscrowHold(data []byte, id [32]byte) (types.EscrowRecord, bool, error) {
	if data == nil {
		return types.EscrowRecord{}, false, nil
	}
	record, err := types.DecodeEscrowRecord(data)
	if err != nil {
		return types.EscrowRecord{}, false, fmt.Errorf("%w: hold: %v", ErrEscrowCorruptState, err)
	}
	if record.EscrowID != id {
		return types.EscrowRecord{}, false, fmt.Errorf("%w: hold identifier", ErrEscrowCorruptState)
	}
	return record, true, nil
}

func decodeEscrowResult(data []byte) (types.EscrowResult, bool, error) {
	if data == nil {
		return types.EscrowResult{}, false, nil
	}
	result, err := types.DecodeEscrowResult(data)
	if err != nil {
		return types.EscrowResult{}, false, fmt.Errorf("%w: result: %v", ErrEscrowCorruptState, err)
	}
	return result, true, nil
}

// GetEscrow distinguishes absence from corruption without panicking.
func (k Keeper) GetEscrow(ctx sdk.Context, id [32]byte) (types.EscrowRecord, bool, error) {
	key := types.KernelKVKey(types.EscrowModuleID, append([]byte("hold:"), id[:]...))
	return decodeEscrowHold(ctx.KVStore(k.hostKey).Get(key), id)
}

func (k Keeper) EscrowRemaining(ctx sdk.Context, id [32]byte) (codec.U128, bool, error) {
	record, found, err := k.GetEscrow(ctx, id)
	return record.LockedAmount, found, err
}

func (k Keeper) GetEscrowResult(ctx sdk.Context, idempotencyKey [32]byte) (types.EscrowResult, bool, error) {
	key := types.KernelKVKey(types.EscrowModuleID, append([]byte("result:"), idempotencyKey[:]...))
	result, found, err := decodeEscrowResult(ctx.KVStore(k.hostKey).Get(key))
	if err != nil || !found {
		return result, found, err
	}
	hold, linked, err := k.GetEscrow(ctx, result.EscrowID)
	if err != nil {
		return types.EscrowResult{}, false, err
	}
	if !linked || hold.AssetID != result.AssetID {
		return types.EscrowResult{}, false, fmt.Errorf("%w: result hold link", ErrEscrowCorruptState)
	}
	return result, true, nil
}

// Validate only owned escrow keys, using the same wire decoders as reads. A
// result is keyed by idempotency key, not escrow id; its body links to a hold.
func validateEscrowRecords(records []kv.Pair) error {
	holds := make(map[[32]byte]types.EscrowRecord)
	var results []types.EscrowResult
	for _, pair := range records {
		if !bytes.HasPrefix(pair.Key, types.EscrowKVPrefix) {
			continue
		}
		key := pair.Key[len(types.EscrowKVPrefix):]
		switch {
		case bytes.HasPrefix(key, []byte("hold:")):
			if len(key) != 5+32 || pair.Value == nil {
				return fmt.Errorf("%w: hold key or value", ErrEscrowCorruptState)
			}
			id := [32]byte(key[5:])
			record, _, err := decodeEscrowHold(pair.Value, id)
			if err != nil {
				return err
			}
			holds[id] = record
		case bytes.HasPrefix(key, []byte("result:")):
			if len(key) != 7+32 || [32]byte(key[7:]) == ([32]byte{}) || pair.Value == nil {
				return fmt.Errorf("%w: result key or value", ErrEscrowCorruptState)
			}
			result, _, err := decodeEscrowResult(pair.Value)
			if err != nil {
				return err
			}
			results = append(results, result)
		default:
			return fmt.Errorf("%w: unknown escrow key", ErrEscrowCorruptState)
		}
	}
	for _, result := range results {
		hold, found := holds[result.EscrowID]
		if !found || hold.AssetID != result.AssetID {
			return fmt.Errorf("%w: result hold link", ErrEscrowCorruptState)
		}
	}
	return nil
}
