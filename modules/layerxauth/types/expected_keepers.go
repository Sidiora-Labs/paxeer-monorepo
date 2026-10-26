package types

import (
	"time"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz"
	"github.com/ethereum/go-ethereum/common"
)

type AuthzKeeper interface {
	SaveGrant(ctx sdk.Context, grantee, granter sdk.AccAddress, authorization authz.Authorization, expiration time.Time) error
	DeleteGrant(ctx sdk.Context, grantee, granter sdk.AccAddress, msgType string) error
	GetCleanAuthorization(ctx sdk.Context, grantee, granter sdk.AccAddress, msgType string) (authz.Authorization, time.Time)
}

type EVMKeeper interface {
	GetLayerXDid(ctx sdk.Context, evmAddress common.Address) ([32]byte, bool)
	GetEVMAddress(ctx sdk.Context, paxAddress sdk.AccAddress) (common.Address, bool)
}
