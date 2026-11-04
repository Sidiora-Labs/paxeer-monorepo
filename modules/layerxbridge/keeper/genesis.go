package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge/types"
	tokenfactorytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

// InitGenesis loads a validated genesis. Bridged denoms are tokenfactory
// state and come from the tokenfactory genesis; only their mapping is kept
// here.
func (k Keeper) InitGenesis(ctx sdk.Context, genesis types.GenesisState) {
	if err := genesis.Validate(); err != nil {
		panic(err)
	}
	if err := k.SetParams(ctx, genesis.Params); err != nil {
		panic(err)
	}
	k.setPaused(ctx, genesis.Paused)
	for _, chain := range genesis.Chains {
		k.setChain(ctx, chain)
	}
	k.setAttestorSet(ctx, genesis.Attestors)
	for _, asset := range genesis.Assets {
		if types.IsSidioraPair(asset.ChainID, asset.Asset) {
			// Tokenfactory genesis creates default metadata after bank genesis.
			// Restore the bridge-owned canonical metadata using its imported admin.
			if _, err := k.tokenFactoryMsg.SetDenomMetadata(sdk.WrapSDKContext(ctx), &tokenfactorytypes.MsgSetDenomMetadata{
				Sender: k.ModuleAddress().String(), Metadata: SidioraMetadata(),
			}); err != nil {
				panic(err)
			}
		}
		k.setAsset(ctx, asset)
	}
	for _, c := range genesis.Caps {
		k.setCap(ctx, c)
	}
	for _, entry := range genesis.InFlight {
		k.setInFlight(ctx, entry.Denom, entry.Amount)
	}
	for _, nullifier := range genesis.Nullifiers {
		k.setNullifier(ctx, nullifier)
	}
	for _, nonce := range genesis.OutboundNonces {
		k.setOutboundNonce(ctx, nonce.ChainID, nonce.Nonce)
	}
}

func (k Keeper) ExportGenesis(ctx sdk.Context) types.GenesisState {
	return types.GenesisState{
		Params:         k.GetParams(ctx),
		Paused:         k.IsPaused(ctx),
		Chains:         k.GetChains(ctx),
		Attestors:      k.GetAttestorSet(ctx),
		Assets:         k.GetAssets(ctx),
		Caps:           k.GetCaps(ctx),
		InFlight:       k.GetInFlight(ctx),
		Nullifiers:     k.GetNullifiers(ctx),
		OutboundNonces: k.GetOutboundNonces(ctx),
	}
}
