package types

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
)

func TestPayMsgUpdateParamsValidateBasic(t *testing.T) {
	authority := authtypes.NewModuleAddress("gov")
	for flags := 0; flags < 32; flags++ {
		t.Run(fmt.Sprintf("flags_%02d", flags), func(t *testing.T) {
			params := Params{
				Paused:         flags&1 != 0,
				EscrowEnabled:  flags&2 != 0,
				BudgetEnabled:  flags&4 != 0,
				StreamEnabled:  flags&8 != 0,
				ServiceEnabled: flags&16 != 0,
			}
			msg := MsgUpdateParams{Authority: authority.String(), Params: params}
			if err := msg.ValidateBasic(); err != nil {
				t.Fatalf("governance authority with params %+v refused: %v", params, err)
			}
			if msg.Params != params {
				t.Fatal("ValidateBasic mutated params")
			}
			if msg.Route() != "layerxpay" || msg.Type() != "update_params" {
				t.Fatalf("route/type = %q/%q", msg.Route(), msg.Type())
			}
			signers := msg.GetSigners()
			if len(signers) != 1 || !signers[0].Equals(authority) {
				t.Fatalf("signers = %v, want [%s]", signers, authority)
			}
			signBytes := msg.GetSignBytes()
			if !bytes.Equal(signBytes, sdk.MustSortJSON(signBytes)) {
				t.Fatalf("sign bytes are not sorted JSON: %s", signBytes)
			}
			var document struct {
				Type  string          `json:"type"`
				Value MsgUpdateParams `json:"value"`
			}
			if err := json.Unmarshal(signBytes, &document); err != nil {
				t.Fatal(err)
			}
			if document.Type != "layerxpay/MsgUpdateParams" || document.Value != msg {
				t.Fatalf("sign document = %+v, want %+v", document, msg)
			}
			for _, bad := range []string{"", "bad", "not-an-address", authority.String() + "x"} {
				msg.Authority = bad
				if err := msg.ValidateBasic(); !errors.Is(err, sdkerrors.ErrInvalidAddress) {
					t.Fatalf("authority %q: error = %v, want ErrInvalidAddress", bad, err)
				}
			}
		})
	}
}

func TestPayCodecRegistersMsgUpdateParams(t *testing.T) {
	const typeURL = "/paxprotocol.paxchain.layerxpay.MsgUpdateParams"
	registry := cdctypes.NewInterfaceRegistry()
	RegisterInterfaces(registry)
	resolved, err := registry.Resolve(typeURL)
	if err != nil {
		t.Fatalf("resolve %s: %v", typeURL, err)
	}
	if _, ok := resolved.(*MsgUpdateParams); !ok {
		t.Fatalf("resolved type = %T, want *MsgUpdateParams", resolved)
	}
	cdc := codec.NewProtoCodec(registry)
	authority := authtypes.NewModuleAddress("gov").String()
	for flags := 0; flags < 32; flags++ {
		msg := MsgUpdateParams{Authority: authority, Params: Params{
			Paused: flags&1 != 0, EscrowEnabled: flags&2 != 0,
			BudgetEnabled: flags&4 != 0, StreamEnabled: flags&8 != 0,
			ServiceEnabled: flags&16 != 0,
		}}
		packed, err := cdctypes.NewAnyWithValue(&msg)
		if err != nil {
			t.Fatal(err)
		}
		if packed.TypeUrl != typeURL {
			t.Fatalf("type URL = %q, want %q", packed.TypeUrl, typeURL)
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
			t.Fatalf("protobuf round trip = %#v, want %#v", decoded, msg)
		}
		aminoJSON, err := ModuleCdc.MarshalAsJSON(&msg)
		if err != nil {
			t.Fatal(err)
		}
		var aminoDecoded MsgUpdateParams
		if err := ModuleCdc.UnmarshalAsJSON(aminoJSON, &aminoDecoded); err != nil {
			t.Fatal(err)
		}
		if aminoDecoded != msg {
			t.Fatalf("amino round trip = %+v, want %+v", aminoDecoded, msg)
		}
	}
	if _Msg_serviceDesc.ServiceName != "paxprotocol.paxchain.layerxpay.Msg" ||
		_Msg_serviceDesc.Metadata != "layerxpay/tx.proto" || len(_Msg_serviceDesc.Methods) != 1 ||
		_Msg_serviceDesc.Methods[0].MethodName != "UpdateParams" || _Msg_serviceDesc.Methods[0].Handler == nil {
		t.Fatalf("unexpected Msg service descriptor: %+v", _Msg_serviceDesc)
	}
	compressed := proto.FileDescriptor("layerxpay/tx.proto")
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	var file descriptor.FileDescriptorProto
	if err := proto.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	if file.GetPackage() != "paxprotocol.paxchain.layerxpay" || len(file.Service) != 1 ||
		file.Service[0].GetName() != "Msg" || len(file.Service[0].Method) != 1 {
		t.Fatalf("unexpected protobuf service: %s", &file)
	}
	method := file.Service[0].Method[0]
	if method.GetName() != "UpdateParams" || method.GetInputType() != ".paxprotocol.paxchain.layerxpay.MsgUpdateParams" ||
		method.GetOutputType() != ".paxprotocol.paxchain.layerxpay.MsgUpdateParamsResponse" {
		t.Fatalf("unexpected UpdateParams descriptor: %s", method)
	}
}
