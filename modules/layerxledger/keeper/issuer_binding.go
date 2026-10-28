package keeper

import (
	"fmt"
	"math/big"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	tokenfactorytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
)

type IssuerBindingKeeper struct {
	*Keeper
	issuerKeeper types.IssuerKeeper
}

func NewIssuerBindingKeeper(ledger *Keeper, issuer types.IssuerKeeper) *IssuerBindingKeeper {
	return &IssuerBindingKeeper{Keeper: ledger, issuerKeeper: issuer}
}

type tokenfactoryIssuerKeeper struct {
	tokenfactorytypes.IssuerKeeper
}

func NewTokenfactoryIssuerKeeper(issuer tokenfactorytypes.IssuerKeeper) types.IssuerKeeper {
	return tokenfactoryIssuerKeeper{IssuerKeeper: issuer}
}

func (k tokenfactoryIssuerKeeper) SetDenomPausedByIssuer(ctx sdk.Context, denom string, paused bool) error {
	issuerDid, err := k.IssuerDid(ctx, denom)
	if err != nil {
		return err
	}
	return k.IssuerKeeper.SetDenomPausedByIssuer(ctx, denom, issuerDid, paused)
}

func issuerDenomKey(assetID [32]byte) []byte {
	return append([]byte{0x02}, assetID[:]...)
}

func (k *IssuerBindingKeeper) bindIssuerDenom(ctx sdk.Context, assetID [32]byte, issuerDid string) error {
	key := issuerDenomKey(assetID)
	if k.store(ctx).Has(key) {
		return types.ErrAssetAlreadyRegistered
	}
	subdenom := fmt.Sprintf("%043s", new(big.Int).SetBytes(assetID[:]).Text(62))
	denom, err := k.issuerKeeper.CreateDenom(ctx, authtypes.NewModuleAddress(types.ModuleName).String(), subdenom)
	if err != nil {
		return err
	}
	if denom == "" {
		return fmt.Errorf("layerxledger: issuer returned an empty denom")
	}
	if err := k.issuerKeeper.SetIssuerDid(ctx, denom, issuerDid); err != nil {
		return err
	}
	k.store(ctx).Set(key, []byte(denom))
	return nil
}

func (k *IssuerBindingKeeper) mirrorPause(ctx sdk.Context, assetID [32]byte, paused bool) error {
	denom := k.store(ctx).Get(issuerDenomKey(assetID))
	if len(denom) == 0 {
		return fmt.Errorf("layerxledger: asset %x has no issuer denom binding", assetID)
	}
	return k.issuerKeeper.SetDenomPausedByIssuer(ctx, string(denom), paused)
}

func (k *IssuerBindingKeeper) RegisterAsset(ctx sdk.Context, actor Actor, args types.RegisterArgs, idempotencyKey [32]byte) (Result, error) {
	cache, write := ctx.CacheContext()
	result, err := k.Keeper.RegisterAsset(cache, actor, args, idempotencyKey)
	if err != nil {
		return result, err
	}
	if err := k.bindIssuerDenom(cache, args.AssetID, string(actor.DID)); err != nil {
		return result, err
	}
	write()
	return result, nil
}

func (k *IssuerBindingKeeper) PauseAsset(ctx sdk.Context, actor Actor, assetID, idempotencyKey [32]byte) (Result, error) {
	cache, write := ctx.CacheContext()
	result, err := k.Keeper.PauseAsset(cache, actor, assetID, idempotencyKey)
	if err != nil {
		return result, err
	}
	if err := k.mirrorPause(cache, assetID, true); err != nil {
		return result, err
	}
	write()
	return result, nil
}

func (k *IssuerBindingKeeper) UnpauseAsset(ctx sdk.Context, actor Actor, assetID, idempotencyKey [32]byte) (Result, error) {
	cache, write := ctx.CacheContext()
	result, err := k.Keeper.UnpauseAsset(cache, actor, assetID, idempotencyKey)
	if err != nil {
		return result, err
	}
	if err := k.mirrorPause(cache, assetID, false); err != nil {
		return result, err
	}
	write()
	return result, nil
}
