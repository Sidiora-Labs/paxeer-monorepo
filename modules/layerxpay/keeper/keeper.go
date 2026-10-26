package keeper

import (
	"fmt"

	custodytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

type EVMBinding interface {
	GetLayerXDid(ctx sdk.Context, address common.Address) ([32]byte, bool)
}

type CustodyReader interface {
	GetAssetByPointer(ctx sdk.Context, pointer common.Address) (custodytypes.AssetMapping, bool)
	GetAssetByDenom(ctx sdk.Context, denom string) (custodytypes.AssetMapping, bool)
}

type Keeper struct {
	cdc       codec.BinaryCodec
	storeKey  sdk.StoreKey
	hostKey   sdk.StoreKey
	host      *lxphost.Host
	evm       EVMBinding
	custody   CustodyReader
	authority string
}

func NewKeeper(cdc codec.BinaryCodec, storeKey, hostKey sdk.StoreKey, host *lxphost.Host, evm EVMBinding, custody CustodyReader, authority string) (Keeper, error) {
	if storeKey == nil {
		return Keeper{}, fmt.Errorf("layerxpay: missing module store key")
	}
	if hostKey == nil {
		return Keeper{}, fmt.Errorf("layerxpay: missing shared host store key")
	}
	if host == nil {
		return Keeper{}, fmt.Errorf("layerxpay: missing shared host")
	}
	if authority == "" {
		return Keeper{}, fmt.Errorf("layerxpay: missing authority")
	}
	return Keeper{cdc: cdc, storeKey: storeKey, hostKey: hostKey, host: host, evm: evm, custody: custody, authority: authority}, nil
}

func (k Keeper) Host() *lxphost.Host { return k.host }

func (k Keeper) Authority() string { return k.authority }
