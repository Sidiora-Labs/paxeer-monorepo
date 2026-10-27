package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func (k Keeper) ReceiptKeyPins(ctx sdk.Context) ([]verify.SequencerAuthorization, error) {
	keys, err := k.ReceiptKeys(ctx)
	if err != nil {
		return nil, err
	}
	pins := make([]verify.SequencerAuthorization, len(keys))
	for i, key := range keys {
		pins[i] = verify.SequencerAuthorization{
			SequencerID:      key.KeyId,
			PublicKey:        key.PublicKey,
			FirstBatchNumber: key.FirstHeight,
			LastBatchNumber:  key.LastHeight,
		}
	}
	return pins, nil
}
