package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

type Keeper struct {
	cdc          codec.BinaryCodec
	storeKey     sdk.StoreKey
	host         *lxphost.Host
	bankKeeper   types.BankKeeper
	evmKeeper    types.EVMKeeper
	authority    string
	issuerKeeper types.IssuerKeeper
}

func NewKeeper(cdc codec.BinaryCodec, storeKey sdk.StoreKey, host *lxphost.Host, bank types.BankKeeper, evm types.EVMKeeper, authority string) *Keeper {
	return &Keeper{cdc: cdc, storeKey: storeKey, host: host, bankKeeper: bank, evmKeeper: evm, authority: authority}
}

// NewKeeperWithIssuer installs issuer binding on the concrete keeper consumed by
// the module and precompile, including after the host is rebound on upgrade.
func NewKeeperWithIssuer(cdc codec.BinaryCodec, storeKey sdk.StoreKey, host *lxphost.Host, bank types.BankKeeper, evm types.EVMKeeper, authority string, issuer types.IssuerKeeper) *Keeper {
	k := NewKeeper(cdc, storeKey, host, bank, evm, authority)
	if issuer == nil {
		panic("layerxledger: nil issuer keeper")
	}
	k.issuerKeeper = issuer
	return k
}

func (k *Keeper) store(ctx sdk.Context) sdk.KVStore { return ctx.KVStore(k.storeKey) }

// GetParams reads the module store; absent params return defaults without a write.
func (k *Keeper) GetParams(ctx sdk.Context) types.Params {
	var params types.Params
	bz := k.store(ctx).Get(types.ParamsKey)
	if bz == nil {
		return types.DefaultParams()
	}
	k.cdc.MustUnmarshal(bz, &params)
	return params
}

func (k *Keeper) SetParams(ctx sdk.Context, params types.Params) error {
	if err := params.Validate(); err != nil {
		return err
	}
	k.store(ctx).Set(types.ParamsKey, k.cdc.MustMarshal(&params))
	return nil
}
