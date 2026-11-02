package types

import (
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

type AddressResolver interface {
	GetPaxAddressOrDefault(sdk.Context, common.Address) sdk.AccAddress
}
