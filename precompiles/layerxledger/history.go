package layerxledger

import (
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/src/modules/history"
)

// ListOwnedAccounts exposes the persisted registry for node-root history reads
// without creating another ledger or using the precompile's write authority.
func ListOwnedAccounts(ctx sdk.Context, did string, limit int) ([]string, error) {
	return history.OwnedAccounts(ctx, did, limit)
}
