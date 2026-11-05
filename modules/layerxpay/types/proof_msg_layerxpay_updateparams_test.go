// Proves: MsgUpdateParams -> ModuleCdc / RegisterCodec / RegisterInterfaces -> sdk.Msg.ValidateBasic and MsgUpdateParams.GetSigners.
//
// The consumer is this external-package test, using the exported type boundary
// named by the contract. All producers run in process; real Amino and protobuf
// codecs carry bytes to independently registered consumers. There is no durable
// state: each result is decoded twice into fresh objects, including after the
// previous object is mutated. Truncated producer output must fail at decoding.
//
// Repairs modules/layerxpay/types/msgs.go: MsgUpdateParams.GetSigners discarded
// address decoding errors and returned an empty signer. It now panics on invalid
// authority instead of presenting an invalid signer to the signing consumer.
package types_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
)

func TestProofMsgLayerxpayUpdateparams(t *testing.T) {
	newAmino := func() codec.Codec {
		amino := codec.NewLegacyAmino()
		paytypes.RegisterCodec(amino)
		sdk.RegisterLegacyAminoCodec(amino)
		amino.Seal()
		return codec.NewAminoCodec(amino)
	}
	newProto := func() codec.Codec {
		registry := cdctypes.NewInterfaceRegistry()
		sdk.RegisterInterfaces(registry)
		paytypes.RegisterInterfaces(registry)
		return codec.NewProtoCodec(registry)
	}
	authority := authtypes.NewModuleAddress("gov")
	for _, transport := range []struct {
		name        string
		producer    codec.Codec
		newConsumer func() codec.Codec
	}{
		{"ModuleCdc", paytypes.ModuleCdc, newAmino},
		{"RegisterCodec", newAmino(), newAmino},
		{"RegisterInterfaces", newProto(), newProto},
	} {
		for _, jsonTransport := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/json_%t", transport.name, jsonTransport), func(t *testing.T) {
				encode := transport.producer.MarshalInterface
				if jsonTransport {
					encode = transport.producer.MarshalInterfaceJSON
				}
				decode := func(wire []byte) (sdk.Msg, error) {
					consumer := transport.newConsumer()
					var observed sdk.Msg
					var err error
					if jsonTransport {
						err = consumer.UnmarshalInterfaceJSON(wire, &observed)
					} else {
						err = consumer.UnmarshalInterface(wire, &observed)
					}
					return observed, err
				}

				for flags := 0; flags < 32; flags++ {
					t.Run(fmt.Sprintf("flags_%02d", flags), func(t *testing.T) {
						producer := &paytypes.MsgUpdateParams{
							Authority: authority.String(),
							Params: paytypes.Params{
								Paused: flags&1 != 0, EscrowEnabled: flags&2 != 0,
								BudgetEnabled: flags&4 != 0, StreamEnabled: flags&8 != 0,
								ServiceEnabled: flags&16 != 0,
							},
						}
						if err := producer.ValidateBasic(); err != nil {
							t.Fatalf("modules/layerxpay/types/msgs.go producer refused: %v", err)
						}
						want := *producer
						wire, err := encode(producer)
						if err != nil {
							t.Fatalf("modules/layerxpay/types/codec.go encode: %v", err)
						}
						originalWire := bytes.Clone(wire)
						// Only the serialized bytes may carry the producer's result.
						producer.Authority = "invalid-after-encoding"
						var previous *paytypes.MsgUpdateParams
						for round := 0; round < 2; round++ {
							observed, err := decode(wire)
							if err != nil {
								t.Fatalf("round %d decode: %v", round, err)
							}
							got, ok := observed.(*paytypes.MsgUpdateParams)
							if !ok || got == nil {
								t.Fatalf("round %d consumer type = %T, want *MsgUpdateParams", round, observed)
							}
							if got == producer || got == previous || *got != want {
								t.Fatalf("round %d consumer reused an object or changed fields: %+v, want %+v", round, got, want)
							}
							if err := observed.ValidateBasic(); err != nil {
								t.Fatalf("round %d consumer validation: %v", round, err)
							}
							signers := observed.GetSigners()
							if len(signers) != 1 || !signers[0].Equals(authority) {
								t.Fatalf("round %d consumer signers = %v, want [%s]", round, signers, authority)
							}
							if got.Route() != "layerxpay" || got.Type() != "update_params" {
								t.Fatalf("consumer route/type = %s/%s", got.Route(), got.Type())
							}
							signBytes := got.GetSignBytes()
							if !bytes.Equal(signBytes, sdk.MustSortJSON(signBytes)) {
								t.Fatalf("consumer sign bytes are not sorted: %s", signBytes)
							}
							var document struct {
								Type  string                   `json:"type"`
								Value paytypes.MsgUpdateParams `json:"value"`
							}
							if err := json.Unmarshal(signBytes, &document); err != nil {
								t.Fatalf("consumer sign document JSON: %v", err)
							}
							if document.Type != "layerxpay/MsgUpdateParams" || document.Value != want {
								t.Fatalf("consumer sign document = %+v, want layerxpay/MsgUpdateParams with %+v", document, want)
							}
							var signed sdk.Msg
							if err := newAmino().UnmarshalInterfaceJSON(signBytes, &signed); err != nil {
								t.Fatalf("consumer sign document decode: %v", err)
							}
							if signedMsg, ok := signed.(*paytypes.MsgUpdateParams); !ok || signedMsg == nil || *signedMsg != want {
								t.Fatalf("consumer sign document = %#v, want %+v", signed, want)
							}
							previous = got
							got.Authority = "invalid-after-consumption"
							got.Params.Paused = !got.Params.Paused
							if !bytes.Equal(wire, originalWire) {
								t.Fatal("consumer mutated the transport bytes")
							}
						}
						if len(wire) < 2 {
							t.Fatalf("producer emitted unexpectedly short output: %x", wire)
						}
						if _, err := decode(wire[:len(wire)-1]); err == nil {
							t.Fatal("consumer decoder accepted truncated producer output")
						}
					})
				}

				for _, bad := range []string{"", "bad", authority.String() + "x"} {
					t.Run("invalid_authority_"+bad, func(t *testing.T) {
						producer := &paytypes.MsgUpdateParams{Authority: bad, Params: paytypes.DefaultParams()}
						wire, err := encode(producer)
						if err != nil {
							t.Fatalf("encode syntactically serializable invalid authority: %v", err)
						}
						for round := 0; round < 2; round++ {
							observed, err := decode(wire)
							if err != nil {
								t.Fatalf("decode syntactically serializable invalid authority: %v", err)
							}
							if err := observed.ValidateBasic(); !errors.Is(err, sdkerrors.ErrInvalidAddress) {
								t.Fatalf("consumer validation = %v, want ErrInvalidAddress", err)
							}
							func() {
								defer func() {
									if recover() == nil {
										t.Fatal("GetSigners accepted an invalid authority instead of panicking")
									}
								}()
								observed.GetSigners()
							}()
							if _, err := decode(wire[:len(wire)-1]); err == nil {
								t.Fatal("consumer decoder accepted truncated invalid-authority producer output")
							}
						}
					})
				}
			})
		}
	}
}
