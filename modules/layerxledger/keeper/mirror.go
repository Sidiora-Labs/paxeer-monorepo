package keeper

import (
	"math/big"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func MirrorDenom(assetID [32]byte) string { return types.MirrorDenom(assetID) }

func BankAddress(accountID [32]byte) sdk.AccAddress { return lxphost.BankAddress(accountID) }

func IssuanceAccountID(assetID [32]byte) (string, [32]byte, error) {
	return lxphost.IssuanceAccountID(assetID)
}

func (k *Keeper) Mirror(ctx sdk.Context, activityType uint32, receipt lxphost.Receipt) error {
	if receipt.ResultCode != 0 {
		return FromKernel(receipt.ResultCode)
	}
	switch activityType {
	case 0x00010001:
		var assetID [32]byte
		found := false
		for _, event := range receipt.Events {
			if event.Code != 1 {
				continue
			}
			if found || len(event.Body) != len(assetID) {
				return types.ErrNonCanonical
			}
			copy(assetID[:], event.Body)
			found = true
		}
		if !found {
			return types.ErrNonCanonical
		}
		asset, err := k.GetAsset(ctx, assetID)
		if err != nil {
			return err
		}
		cap := asset.SupplyCap
		if cap.IsZero() {
			cap = sdk.NewIntFromBigInt(new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1)))
		}
		_, issuance, err := IssuanceAccountID(assetID)
		if err != nil {
			return ledgerReadError(err)
		}
		coins := sdk.NewCoins(sdk.NewCoin(MirrorDenom(assetID), cap))
		if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, coins); err != nil {
			return types.ErrFatalSupplyMismatch.Wrapf("mint mirror supply: %v", err)
		}
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, BankAddress(issuance), coins); err != nil {
			return types.ErrFatalSupplyMismatch.Wrapf("fund issuance account: %v", err)
		}
	case 0x00010005, 0x00010006, 0x0001000a, 0x0001000b:
	default:
		return nil
	}
	for _, leg := range receipt.Transfers {
		if leg.Amount.IsNil() || !leg.Amount.IsPositive() || leg.Amount.BigInt().BitLen() > 128 {
			return types.ErrNonCanonical
		}
		coins := sdk.NewCoins(sdk.NewCoin(MirrorDenom(leg.Asset), leg.Amount))
		if err := k.bankKeeper.SendCoins(ctx, BankAddress(leg.From), BankAddress(leg.To), coins); err != nil {
			return types.ErrFatalSupplyMismatch.Wrapf("mirror transfer: %v", err)
		}
	}
	return nil
}
