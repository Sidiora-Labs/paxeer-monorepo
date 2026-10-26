package types

import (
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

const TypeMsgUpdateParams = "update_params"

var _ sdk.Msg = &MsgUpdateParams{}

func (m MsgUpdateParams) Route() string { return RouterKey }
func (m MsgUpdateParams) Type() string  { return TypeMsgUpdateParams }
func (m MsgUpdateParams) GetSigners() []sdk.AccAddress {
	decoded, _ := sdk.AccAddressFromBech32(m.Authority)
	return []sdk.AccAddress{decoded}
}
func (m MsgUpdateParams) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(&m))
}
func (m MsgUpdateParams) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid authority address (%s)", err)
	}
	return m.Params.Validate()
}
