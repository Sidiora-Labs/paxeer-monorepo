package types_test

import (
	"fmt"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"testing"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
)

// TestMsgCreateDenom tests if valid/invalid create denom messages are properly validated/invalidated
func TestMsgCreateDenom(t *testing.T) {
	// generate a private/public key pair and get the respective address
	pk1 := ed25519.GenerateSecretKey().Public()
	addr1 := sdk.AccAddress(pk1.Address())

	// make a proper createDenom message
	createMsg := func(after func(msg types.MsgCreateDenom) types.MsgCreateDenom) types.MsgCreateDenom {
		properMsg := *types.NewMsgCreateDenom(
			addr1.String(),
			"bitcoin",
		)

		return after(properMsg)
	}

	// validate createDenom message was created as intended
	msg := createMsg(func(msg types.MsgCreateDenom) types.MsgCreateDenom {
		return msg
	})
	require.Equal(t, msg.Route(), types.RouterKey)
	require.Equal(t, msg.Type(), "create_denom")
	signers := msg.GetSigners()
	require.Equal(t, len(signers), 1)
	require.Equal(t, signers[0].String(), addr1.String())

	tests := []struct {
		name       string
		msg        types.MsgCreateDenom
		expectPass bool
	}{
		{
			name: "proper msg",
			msg: createMsg(func(msg types.MsgCreateDenom) types.MsgCreateDenom {
				return msg
			}),
			expectPass: true,
		},
		{
			name: "empty sender",
			msg: createMsg(func(msg types.MsgCreateDenom) types.MsgCreateDenom {
				msg.Sender = ""
				return msg
			}),
			expectPass: false,
		},
		{
			name: "invalid subdenom",
			msg: createMsg(func(msg types.MsgCreateDenom) types.MsgCreateDenom {
				msg.Subdenom = "thissubdenomismuchtoolongasdkfjaasdfdsafsdlkfnmlksadmflksmdlfmlsakmfdsafasdfasdf"
				return msg
			}),
			expectPass: false,
		},
	}

	for _, test := range tests {
		if test.expectPass {
			require.NoError(t, test.msg.ValidateBasic(), "test: %v", test.name)
		} else {
			require.Error(t, test.msg.ValidateBasic(), "test: %v", test.name)
		}
	}
}

func TestMsgUpdateDenom(t *testing.T) {
	// generate a private/public key pair and get the respective address
	pk1 := ed25519.GenerateSecretKey().Public()
	addr1 := sdk.AccAddress(pk1.Address())

	// make a proper createDenom message
	updateMsg := func(after func(msg types.MsgUpdateDenom) types.MsgUpdateDenom) types.MsgUpdateDenom {
		properMsg := *types.NewMsgUpdateDenom(
			addr1.String(),
			fmt.Sprintf("factory/%s/bitcoin", addr1.String()),
			&banktypes.AllowList{Addresses: []string{addr1.String()}},
		)

		return after(properMsg)
	}

	// validate updateDenom message was created as intended
	msg := updateMsg(func(msg types.MsgUpdateDenom) types.MsgUpdateDenom {
		return msg
	})
	require.Equal(t, msg.Route(), types.RouterKey)
	require.Equal(t, "update_denom", msg.Type())
	signers := msg.GetSigners()
	require.Equal(t, len(signers), 1)
	require.Equal(t, signers[0].String(), addr1.String())

	tests := []struct {
		name       string
		msg        types.MsgUpdateDenom
		expectPass bool
	}{
		{
			name: "proper msg",
			msg: updateMsg(func(msg types.MsgUpdateDenom) types.MsgUpdateDenom {
				return msg
			}),
			expectPass: true,
		},
		{
			name: "empty sender",
			msg: updateMsg(func(msg types.MsgUpdateDenom) types.MsgUpdateDenom {
				msg.Sender = ""
				return msg
			}),
			expectPass: false,
		},
		{
			name: "invalid subdenom",
			msg: updateMsg(func(msg types.MsgUpdateDenom) types.MsgUpdateDenom {
				msg.Denom = "thissubdenomismuchtoolongasdkfjaasdfdsafsdlkfnmlksadmflksmdlfmlsakmfdsafasdfasdf"
				return msg
			}),
			expectPass: false,
		},
	}

	for _, test := range tests {
		if test.expectPass {
			require.NoError(t, test.msg.ValidateBasic(), "test: %v", test.name)
		} else {
			require.Error(t, test.msg.ValidateBasic(), "test: %v", test.name)
		}
	}
}

// TestMsgMint tests if valid/invalid create denom messages are properly validated/invalidated
func TestMsgMint(t *testing.T) {
	// generate a private/public key pair and get the respective address
	pk1 := ed25519.GenerateSecretKey().Public()
	addr1 := sdk.AccAddress(pk1.Address())

	// make a proper mint message
	createMsg := func(after func(msg types.MsgMint) types.MsgMint) types.MsgMint {
		properMsg := *types.NewMsgMint(
			addr1.String(),
			sdk.NewCoin("bitcoin", sdk.NewInt(500000000)),
		)

		return after(properMsg)
	}

	// validate mint message was created as intended
	msg := createMsg(func(msg types.MsgMint) types.MsgMint {
		return msg
	})
	require.Equal(t, msg.Route(), types.RouterKey)
	require.Equal(t, msg.Type(), "mint")
	signers := msg.GetSigners()
	require.Equal(t, len(signers), 1)
	require.Equal(t, signers[0].String(), addr1.String())

	tests := []struct {
		name       string
		msg        types.MsgMint
		expectPass bool
	}{
		{
			name: "proper msg",
			msg: createMsg(func(msg types.MsgMint) types.MsgMint {
				return msg
			}),
			expectPass: true,
		},
		{
			name: "empty sender",
			msg: createMsg(func(msg types.MsgMint) types.MsgMint {
				msg.Sender = ""
				return msg
			}),
			expectPass: false,
		},
		{
			name: "zero amount",
			msg: createMsg(func(msg types.MsgMint) types.MsgMint {
				msg.Amount = sdk.NewCoin("bitcoin", sdk.ZeroInt())
				return msg
			}),
			expectPass: false,
		},
		{
			name: "negative amount",
			msg: createMsg(func(msg types.MsgMint) types.MsgMint {
				msg.Amount.Amount = sdk.NewInt(-10000000)
				return msg
			}),
			expectPass: false,
		},
	}

	for _, test := range tests {
		if test.expectPass {
			require.NoError(t, test.msg.ValidateBasic(), "test: %v", test.name)
		} else {
			require.Error(t, test.msg.ValidateBasic(), "test: %v", test.name)
		}
	}
}

// TestMsgBurn tests if valid/invalid create denom messages are properly validated/invalidated
func TestMsgBurn(t *testing.T) {
	pk := ed25519.GenerateSecretKey().Public()
	addr := sdk.AccAddress(pk.Address())
	newMsg := func() *types.MsgBurn {
		return types.NewMsgBurn(addr.String(), sdk.NewCoin("bitcoin", sdk.NewInt(500000000)))
	}
	baseMsg := newMsg()
	require.Equal(t, types.RouterKey, baseMsg.Route())
	require.Equal(t, "burn", baseMsg.Type())
	require.Equal(t, []sdk.AccAddress{addr}, baseMsg.GetSigners())

	for _, tc := range []struct {
		name    string
		mutate  func(*types.MsgBurn)
		wantErr error
	}{
		{name: "proper msg"},
		{
			name:    "empty sender",
			mutate:  func(msg *types.MsgBurn) { msg.Sender = "" },
			wantErr: sdkerrors.ErrInvalidAddress,
		},
		{
			name:    "zero amount",
			mutate:  func(msg *types.MsgBurn) { msg.Amount.Amount = sdk.ZeroInt() },
			wantErr: sdkerrors.ErrInvalidCoins,
		},
		{
			name:    "negative amount",
			mutate:  func(msg *types.MsgBurn) { msg.Amount.Amount = sdk.NewInt(-10000000) },
			wantErr: sdkerrors.ErrInvalidCoins,
		},
		{
			name:    "invalid coin denom",
			mutate:  func(msg *types.MsgBurn) { msg.Amount.Denom = "!" },
			wantErr: sdkerrors.ErrInvalidCoins,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			msg := newMsg()
			if tc.mutate != nil {
				tc.mutate(msg)
			}
			err := msg.ValidateBasic()
			if tc.wantErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tc.wantErr)
			}
			require.Equal(t, newMsg(), baseMsg, "a case must not mutate the baseline")
			require.NoError(t, baseMsg.ValidateBasic())
		})
	}
}

// TestMsgChangeAdmin checks transfer and renunciation validation independently.
func TestMsgChangeAdmin(t *testing.T) {
	addr1 := sdk.AccAddress(ed25519.GenerateSecretKey().Public().Address())
	addr2 := sdk.AccAddress(ed25519.GenerateSecretKey().Public().Address())
	denom, err := types.GetTokenDenom(addr1.String(), "bitcoin")
	require.NoError(t, err)
	newMsg := func() *types.MsgChangeAdmin {
		return types.NewMsgChangeAdmin(addr1.String(), denom, addr2.String())
	}
	baseMsg := newMsg()
	require.Equal(t, types.RouterKey, baseMsg.Route())
	require.Equal(t, "change_admin", baseMsg.Type())
	require.Equal(t, []sdk.AccAddress{addr1}, baseMsg.GetSigners())

	for _, tc := range []struct {
		name    string
		mutate  func(*types.MsgChangeAdmin)
		wantErr error
	}{
		{name: "proper msg"},
		{
			name:    "empty sender",
			mutate:  func(msg *types.MsgChangeAdmin) { msg.Sender = "" },
			wantErr: sdkerrors.ErrInvalidAddress,
		},
		{
			name:   "empty newAdmin renounces authority",
			mutate: func(msg *types.MsgChangeAdmin) { msg.NewAdmin = "" },
		},
		{
			name:    "invalid nonempty newAdmin",
			mutate:  func(msg *types.MsgChangeAdmin) { msg.NewAdmin = "invalid-admin" },
			wantErr: sdkerrors.ErrInvalidAddress,
		},
		{
			name:    "invalid denom",
			mutate:  func(msg *types.MsgChangeAdmin) { msg.Denom = "bitcoin" },
			wantErr: types.ErrInvalidDenom,
		},
		{
			name:    "invalid denom during renunciation",
			mutate:  func(msg *types.MsgChangeAdmin) { msg.Denom = "bitcoin"; msg.NewAdmin = "" },
			wantErr: types.ErrInvalidDenom,
		},
		{
			name:    "empty sender during renunciation",
			mutate:  func(msg *types.MsgChangeAdmin) { msg.Sender = ""; msg.NewAdmin = "" },
			wantErr: sdkerrors.ErrInvalidAddress,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			msg := newMsg()
			if tc.mutate != nil {
				tc.mutate(msg)
			}
			err := msg.ValidateBasic()
			if tc.wantErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tc.wantErr)
			}
			require.Equal(t, newMsg(), baseMsg, "a case must not mutate the baseline")
			require.NoError(t, baseMsg.ValidateBasic())
		})
	}
}

// TestMsgSetDenomPaused tests if valid/invalid set denom paused messages are properly validated/invalidated
func TestMsgSetDenomPaused(t *testing.T) {
	pk1 := ed25519.GenerateSecretKey().Public()
	addr1 := sdk.AccAddress(pk1.Address())
	denom := fmt.Sprintf("factory/%s/usdl", addr1.String())

	msg := types.NewMsgSetDenomPaused(addr1.String(), denom, true)
	require.Equal(t, types.RouterKey, msg.Route())
	require.Equal(t, "set_denom_paused", msg.Type())
	require.Equal(t, []sdk.AccAddress{addr1}, msg.GetSigners())
	require.Equal(t,
		fmt.Sprintf(`{"type":"tokenfactory/MsgSetDenomPaused","value":{"denom":"%s","paused":true,"sender":"%s"}}`, denom, addr1.String()),
		string(msg.GetSignBytes()),
	)

	tests := []struct {
		name    string
		msg     *types.MsgSetDenomPaused
		wantErr error
	}{
		{
			name: "proper pause msg",
			msg:  types.NewMsgSetDenomPaused(addr1.String(), denom, true),
		},
		{
			name: "proper unpause msg",
			msg:  types.NewMsgSetDenomPaused(addr1.String(), denom, false),
		},
		{
			name:    "empty sender",
			msg:     types.NewMsgSetDenomPaused("", denom, true),
			wantErr: sdkerrors.ErrInvalidAddress,
		},
		{
			name:    "denom outside the factory namespace",
			msg:     types.NewMsgSetDenomPaused(addr1.String(), "upax", true),
			wantErr: types.ErrInvalidDenom,
		},
		{
			name:    "denom with a wrong namespace prefix",
			msg:     types.NewMsgSetDenomPaused(addr1.String(), fmt.Sprintf("bank/%s/usdl", addr1.String()), true),
			wantErr: types.ErrInvalidDenom,
		},
	}

	for _, test := range tests {
		err := test.msg.ValidateBasic()
		if test.wantErr == nil {
			require.NoError(t, err, "test: %v", test.name)
		} else {
			require.ErrorIs(t, err, test.wantErr, "test: %v", test.name)
		}
	}
}
