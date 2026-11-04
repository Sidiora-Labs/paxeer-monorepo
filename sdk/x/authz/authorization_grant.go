package authz

import (
	"time"

	proto "github.com/gogo/protobuf/proto"

	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

// NewGrant returns a grant whose expiration is strictly after the chain block time.
func NewGrant(blockTime time.Time, a Authorization, expiration time.Time) (Grant, error) {
	if !expiration.After(blockTime) {
		return Grant{}, ErrInvalidExpirationTime.Wrapf("expiration must be after the current block time (%s), got %s", blockTime.Format(time.RFC3339Nano), expiration.Format(time.RFC3339Nano))
	}
	if a == nil {
		return Grant{}, sdkerrors.ErrInvalidType.Wrap("authorization is required")
	}
	if err := a.ValidateBasic(); err != nil {
		return Grant{}, err
	}
	g := Grant{
		Expiration: expiration,
	}
	msg, ok := a.(proto.Message)
	if !ok {
		return Grant{}, sdkerrors.Wrapf(sdkerrors.ErrPackAny, "cannot proto marshal %T", a)
	}

	any, err := cdctypes.NewAnyWithValue(msg)
	if err != nil {
		return Grant{}, err
	}
	g.Authorization = any

	return g, nil
}

var (
	_ cdctypes.UnpackInterfacesMessage = &Grant{}
)

// UnpackInterfaces implements UnpackInterfacesMessage.UnpackInterfaces
func (g Grant) UnpackInterfaces(unpacker cdctypes.AnyUnpacker) error {
	var authorization Authorization
	return unpacker.UnpackAny(g.Authorization, &authorization)
}

// GetAuthorization returns the cached value from the Grant.Authorization if present.
func (g Grant) GetAuthorization() Authorization {
	if g.Authorization == nil {
		return nil
	}
	a, ok := g.Authorization.GetCachedValue().(Authorization)
	if !ok {
		return nil
	}
	return a
}

func (g Grant) ValidateBasic() error {
	if g.Authorization == nil {
		return sdkerrors.ErrInvalidType.Wrap("authorization is required")
	}
	av := g.Authorization.GetCachedValue()
	a, ok := av.(Authorization)
	if !ok {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidType, "expected %T, got %T", (Authorization)(nil), av)
	}
	return a.ValidateBasic()
}
