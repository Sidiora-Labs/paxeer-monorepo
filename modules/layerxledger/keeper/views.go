package keeper

import (
	"errors"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func (k *Keeper) GetAsset(ctx sdk.Context, assetID [32]byte) (lxphost.AssetRecord, error) {
	asset, err := k.host.ReadAsset(ctx, assetID)
	return asset, ledgerReadError(err)
}

func (k *Keeper) BalanceOf(ctx sdk.Context, accountID, assetID [32]byte) (sdk.Int, error) {
	balance, err := k.host.ReadBalance(ctx, accountID, assetID)
	return balance, ledgerReadError(err)
}

func (k *Keeper) TotalUnits(ctx sdk.Context, assetID [32]byte) (sdk.Int, error) {
	total, err := k.host.ReadTotalUnits(ctx, assetID)
	return total, ledgerReadError(err)
}

func (k *Keeper) AccountOwner(ctx sdk.Context, accountID [32]byte) ([32]byte, error) {
	owner, err := k.host.ReadAccountOwner(ctx, accountID)
	return owner, ledgerReadError(err)
}

func (k *Keeper) GetPayerGrant(ctx sdk.Context, grantID [32]byte) (lxphost.PayerGrantState, error) {
	grant, err := k.host.ReadPayerGrant(ctx, grantID)
	return grant, ledgerReadError(err)
}

func (k *Keeper) NextSequence(ctx sdk.Context, didKey, assetID [32]byte) (uint64, error) {
	name, accountID, err := lxphost.AssetAccountID(didKey, assetID)
	if err != nil {
		return 0, ledgerReadError(err)
	}
	account, err := k.host.ReadAccount(ctx, name, accountID)
	if err != nil {
		return 0, ledgerReadError(err)
	}
	return account.NextSequence, nil
}

func ledgerReadError(err error) error {
	var code lxphost.Code
	if errors.As(err, &code) {
		return FromKernel(code)
	}
	return err
}
