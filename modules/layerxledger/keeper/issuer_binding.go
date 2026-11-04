package keeper

import (
	"fmt"
	"math/big"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	tokenfactorytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
)

type IssuerBindingKeeper struct {
	*Keeper
}

func NewIssuerBindingKeeper(ledger *Keeper, issuer types.IssuerKeeper) *IssuerBindingKeeper {
	if issuer == nil {
		panic("layerxledger: nil issuer keeper")
	}
	configured := *ledger
	configured.issuerKeeper = issuer
	return &IssuerBindingKeeper{Keeper: &configured}
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

func (k *Keeper) bindIssuerDenom(ctx sdk.Context, assetID [32]byte, issuerDid string) error {
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

func (k *Keeper) mirrorPause(ctx sdk.Context, assetID [32]byte, paused bool) error {
	denom := k.store(ctx).Get(issuerDenomKey(assetID))
	if len(denom) == 0 {
		return fmt.Errorf("layerxledger: asset %x has no issuer denom binding", assetID)
	}
	return k.issuerKeeper.SetDenomPausedByIssuer(ctx, string(denom), paused)
}

// rebuildIssuerBindings derives the owned mapping from tokenfactory genesis and
// validates it against the already imported kernel asset and issuer state.
func (k *Keeper) rebuildIssuerBindings(ctx sdk.Context) error {
	if k.issuerKeeper == nil {
		return nil
	}
	source, ok := k.issuerKeeper.(interface {
		issuerBindings(sdk.Context) ([]issuerBinding, error)
	})
	if !ok {
		return fmt.Errorf("layerxledger: issuer keeper cannot rebuild bindings")
	}
	bindings, err := source.issuerBindings(ctx)
	if err != nil {
		return err
	}
	for _, binding := range bindings {
		asset, err := k.GetAsset(ctx, binding.asset)
		if err != nil {
			return err
		}
		issuerID, err := lxphost.DIDID([]byte(binding.issuer))
		if err != nil {
			return err
		}
		if asset.IssuerDID != issuerID || asset.Paused != binding.paused {
			return fmt.Errorf("layerxledger: inconsistent issuer binding for %x", binding.asset)
		}
	}
	for _, binding := range bindings {
		k.store(ctx).Set(issuerDenomKey(binding.asset), []byte(binding.denom))
	}
	return nil
}

type issuerBinding struct {
	asset         [32]byte
	denom, issuer string
	paused        bool
}

func (k tokenfactoryIssuerKeeper) issuerBindings(ctx sdk.Context) ([]issuerBinding, error) {
	source, ok := k.IssuerKeeper.(interface {
		GetAllDenomsFromCreator(sdk.Context, string) []string
	})
	if !ok {
		return nil, fmt.Errorf("layerxledger: tokenfactory cannot enumerate issuer denoms")
	}
	creator := authtypes.NewModuleAddress(types.ModuleName).String()
	var bindings []issuerBinding
	for _, denom := range source.GetAllDenomsFromCreator(ctx, creator) {
		owner, subdenom, err := tokenfactorytypes.DeconstructDenom(denom)
		if err != nil {
			return nil, err
		}
		value, ok := new(big.Int).SetString(subdenom, 62)
		if !ok || value.Sign() < 0 || value.BitLen() > 256 || owner != creator ||
			fmt.Sprintf("%043s", value.Text(62)) != subdenom {
			return nil, fmt.Errorf("layerxledger: noncanonical issuer denom %q", denom)
		}
		binding := issuerBinding{denom: denom, paused: k.IsDenomPaused(ctx, denom)}
		value.FillBytes(binding.asset[:])
		binding.issuer, err = k.IssuerDid(ctx, denom)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, binding)
	}
	return bindings, nil
}
