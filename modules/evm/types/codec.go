package types

//nolint:typecheck
import (
	"errors"
	fmt "fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/msgservice"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/gogo/protobuf/proto"
)

var (
	amino          = codec.NewLegacyAmino()
	ModuleCdc      = codec.NewAminoCodec(amino)
	txDataRegistry = codectypes.NewInterfaceRegistry()
)

func init() {
	registerTxDataInterfaces(txDataRegistry)
	RegisterCodec(amino)
	cryptocodec.RegisterCrypto(amino)
	amino.Seal()
}

func GetAmino() *codec.LegacyAmino {
	return amino
}

func RegisterCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgAssociate{}, "evm/MsgAssociate", nil)
	cdc.RegisterConcrete(&MsgEVMTransaction{}, "evm/MsgEVMTransaction", nil)
	cdc.RegisterConcrete(&MsgSend{}, "evm/MsgSend", nil)
	cdc.RegisterConcrete(&MsgRegisterPointer{}, "evm/MsgRegisterPointer", nil)
	cdc.RegisterConcrete(&MsgAssociateContractAddress{}, "evm/MsgAssociateContractAddress", nil)
	cdc.RegisterConcrete(&MsgClaim{}, "evm/MsgClaim", nil)
	cdc.RegisterConcrete(&MsgClaimSpecific{}, "evm/MsgClaimSpecific", nil)
	cdc.RegisterConcrete(&MsgBindERCNativePointer{}, "evm/MsgBindERCNativePointer", nil)
}

func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterImplementations((*govtypes.Content)(nil),
		&AddERCNativePointerProposal{},
		&AddERCCW20PointerProposal{},
		&AddERCCW721PointerProposal{},
		&AddERCCW1155PointerProposal{},
		&AddCWERC20PointerProposal{},
		&AddCWERC721PointerProposal{},
		&AddCWERC1155PointerProposal{},
		&AddERCNativePointerProposalV2{},
		&PointerBindingProposal{},
	)
	registry.RegisterImplementations(
		(*sdk.Msg)(nil),
		&MsgEVMTransaction{},
		&MsgSend{},
		&MsgRegisterPointer{},
		&MsgAssociateContractAddress{},
		&MsgClaim{},
		&MsgClaimSpecific{},
		&MsgAssociate{},
		&MsgBindERCNativePointer{},
	)
	registerTxDataInterfaces(registry)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}

func registerTxDataInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterInterface(
		"paxprotocol.paxchain.evm.TxData",
		(*ethtx.TxData)(nil),
		&ethtx.DynamicFeeTx{},
		&ethtx.AccessListTx{},
		&ethtx.LegacyTx{},
		&ethtx.BlobTx{},
		&ethtx.AssociateTx{},
		&ethtx.SetCodeTx{},
	)
}

func PackTxData(txData ethtx.TxData) (*codectypes.Any, error) {
	msg, ok := txData.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("cannot proto marshal %T", txData)
	}

	anyTxData, err := codectypes.NewAnyWithValue(msg)
	if err != nil {
		return nil, errors.New(err.Error())
	}

	return anyTxData, nil
}

func UnpackTxData(any *codectypes.Any) (ethtx.TxData, error) {
	if any == nil {
		return nil, errors.New("protobuf Any message cannot be nil")
	}

	// Resolve before consulting the cache: an Any's URL is authoritative.
	message, err := txDataRegistry.Resolve(any.TypeUrl)
	if err != nil {
		return nil, err
	}
	if _, ok := message.(ethtx.TxData); !ok {
		return nil, fmt.Errorf("type URL %q does not identify TxData", any.TypeUrl)
	}
	if cached := any.GetCachedValue(); cached != nil {
		txData, ok := cached.(ethtx.TxData)
		if !ok || "/"+proto.MessageName(txData) != any.TypeUrl {
			return nil, fmt.Errorf("cached TxData %T does not match type URL %q", cached, any.TypeUrl)
		}
		return txData, nil
	}
	var txData ethtx.TxData
	if err := txDataRegistry.UnpackAny(any, &txData); err != nil {
		return nil, err
	}
	return txData, nil
}
