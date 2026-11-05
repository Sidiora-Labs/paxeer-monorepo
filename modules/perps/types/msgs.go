package types

import (
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

var (
	_ sdk.Msg = &MsgUpdateParams{}
	_ sdk.Msg = &MsgSetMarket{}
	_ sdk.Msg = &MsgSetPaused{}
)

func (m MsgUpdateParams) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return ErrInvalidParams.Wrapf("authority: %v", err)
	}
	return m.Params.Validate()
}

func (m MsgSetMarket) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return ErrInvalidParams.Wrapf("authority: %v", err)
	}
	return m.Market.Validate()
}

func (m MsgSetPaused) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return ErrInvalidParams.Wrapf("authority: %v", err)
	}
	return nil
}

func (m MsgUpdateParams) GetSigners() []sdk.AccAddress {
	return authoritySigners(m.Authority)
}

func (m MsgSetMarket) GetSigners() []sdk.AccAddress {
	return authoritySigners(m.Authority)
}

func (m MsgSetPaused) GetSigners() []sdk.AccAddress {
	return authoritySigners(m.Authority)
}

func authoritySigners(authority string) []sdk.AccAddress {
	a, err := sdk.AccAddressFromBech32(authority)
	if err != nil {
		return []sdk.AccAddress{}
	}
	return []sdk.AccAddress{a}
}
