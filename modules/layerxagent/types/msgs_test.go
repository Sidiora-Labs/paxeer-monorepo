package types

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

func TestAgentMsgUpdateParamsValidateBasic(t *testing.T) {
	account := sdk.AccAddress(bytes.Repeat([]byte{0x11}, 20))
	msg := MsgUpdateParams{Authority: account.String(), Params: DefaultParams()}
	msg.Params.Paused = true
	if err := msg.ValidateBasic(); err != nil {
		t.Fatalf("valid message refused: %v", err)
	}
	if msg.Route() != "layerxagent" || msg.Type() != "update_params" {
		t.Fatalf("route/type = %q/%q", msg.Route(), msg.Type())
	}
	signers := msg.GetSigners()
	if len(signers) != 1 || !signers[0].Equals(account) {
		t.Fatalf("signers = %v, want [%s]", signers, account)
	}
	for _, authority := range []string{"", "not-an-address", "0x1111111111111111111111111111111111111111", account.String() + "x"} {
		bad := msg
		bad.Authority = authority
		if err := bad.ValidateBasic(); !errors.Is(err, sdkerrors.ErrInvalidAddress) {
			t.Fatalf("authority %q: err = %v, want ErrInvalidAddress", authority, err)
		}
	}
	for _, params := range []Params{{}, func() Params { p := DefaultParams(); p.MaxCallDataBytes = 0; return p }(),
		func() Params { p := DefaultParams(); p.MaxJobGasLimit = p.MaxBlockGas + 1; return p }()} {
		bad := msg
		bad.Params = params
		want := params.Validate()
		if want == nil {
			t.Fatal("invalid params case unexpectedly validates")
		}
		if err := bad.ValidateBasic(); err == nil || err.Error() != want.Error() || errors.Is(err, sdkerrors.ErrInvalidAddress) {
			t.Fatalf("invalid params: err = %v, want %v", err, want)
		}
	}
	signBytes := msg.GetSignBytes()
	if !bytes.Equal(sdk.MustSortJSON(signBytes), signBytes) {
		t.Fatalf("sign bytes are not sorted JSON: %s", signBytes)
	}
	var document struct {
		Type  string `json:"type"`
		Value struct {
			Authority string          `json:"authority"`
			Params    json.RawMessage `json:"params"`
		} `json:"value"`
	}
	if err := json.Unmarshal(signBytes, &document); err != nil {
		t.Fatalf("decode sign bytes: %v", err)
	}
	var signedParams Params
	if err := ModuleCdc.UnmarshalAsJSON(document.Value.Params, &signedParams); err != nil {
		t.Fatalf("decode signed params: %v", err)
	}
	if document.Type != "layerxagent/MsgUpdateParams" || document.Value.Authority != msg.Authority || signedParams != msg.Params {
		t.Fatalf("sign document = %+v", document)
	}
	registry := cdctypes.NewInterfaceRegistry()
	sdk.RegisterInterfaces(registry)
	RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	packed, err := cdctypes.NewAnyWithValue(&msg)
	if err != nil {
		t.Fatalf("pack message: %v", err)
	}
	if packed.TypeUrl != "/paxprotocol.paxchain.layerxagent.MsgUpdateParams" {
		t.Fatalf("type URL = %q", packed.TypeUrl)
	}
	encoded, err := cdc.MarshalInterface(&msg)
	if err != nil {
		t.Fatalf("marshal interface: %v", err)
	}
	var decoded sdk.Msg
	if err := cdc.UnmarshalInterface(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal interface: %v", err)
	}
	if got, ok := decoded.(*MsgUpdateParams); !ok || *got != msg {
		t.Fatalf("round trip = %#v, want %#v", decoded, msg)
	}
}
