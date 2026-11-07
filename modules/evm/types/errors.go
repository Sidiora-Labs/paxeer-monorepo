package types

import (
	"fmt"
	"strings"

	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

var (
	ErrFeeTokenRateStale       = sdkerrors.Register(ModuleName, 100, "fee-token rate is stale")
	ErrFeeTokenRateUnavailable = sdkerrors.Register(ModuleName, 101, "fee-token rate is unavailable")
	ErrFeeTokenRateInvalid     = sdkerrors.Register(ModuleName, 102, "fee-token rate is invalid")
	ErrFeeTokenRateSpread      = sdkerrors.Register(ModuleName, 103, "fee-token rate update exceeds max_fee_token_spread")
)

type AssociationMissingErr struct {
	Address string
}

func NewAssociationMissingErr(address string) AssociationMissingErr {
	return AssociationMissingErr{Address: address}
}

func (e AssociationMissingErr) Error() string {
	return fmt.Sprintf("address %s is not linked", e.Address)
}

func (e AssociationMissingErr) AddressType() string {
	if strings.HasPrefix(e.Address, "0x") {
		return "evm"
	}
	return "pax"
}
