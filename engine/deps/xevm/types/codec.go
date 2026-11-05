package types

//nolint:typecheck
import (
	"errors"
	fmt "fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/msgservice"

	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/types/ethtx"
	"github.com/gogo/protobuf/proto"
)

var (
	amino     = codec.NewLegacyAmino()
	ModuleCdc = codec.NewAminoCodec(amino)
)

func init() {
	RegisterCodec(amino)
	cryptocodec.RegisterCrypto(amino)
	amino.Seal()
}

func GetAmino() *codec.LegacyAmino {
	return amino
}

func RegisterCodec(cdc *codec.LegacyAmino) {}

func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterInterface(
		"paxprotocol.paxchain.gigaevm.TxData",
		(*ethtx.TxData)(nil),
		&ethtx.DynamicFeeTx{},
		&ethtx.AccessListTx{},
		&ethtx.LegacyTx{},
		&ethtx.BlobTx{},
		&ethtx.AssociateTx{},
		&ethtx.SetCodeTx{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
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

	var txData ethtx.TxData
	switch any.TypeUrl {
	case "/" + proto.MessageName(&ethtx.LegacyTx{}):
		txData = &ethtx.LegacyTx{}
	case "/" + proto.MessageName(&ethtx.AccessListTx{}):
		txData = &ethtx.AccessListTx{}
	case "/" + proto.MessageName(&ethtx.DynamicFeeTx{}):
		txData = &ethtx.DynamicFeeTx{}
	case "/" + proto.MessageName(&ethtx.BlobTx{}):
		txData = &ethtx.BlobTx{}
	case "/" + proto.MessageName(&ethtx.AssociateTx{}):
		txData = &ethtx.AssociateTx{}
	case "/" + proto.MessageName(&ethtx.SetCodeTx{}):
		txData = &ethtx.SetCodeTx{}
	default:
		return nil, fmt.Errorf("cannot unpack TxData with unregistered type URL %q", any.TypeUrl)
	}

	if cached := any.GetCachedValue(); cached != nil {
		cachedTxData, ok := cached.(ethtx.TxData)
		if !ok || "/"+proto.MessageName(cachedTxData) != any.TypeUrl {
			return nil, fmt.Errorf("TxData type URL %q does not match cached value %T", any.TypeUrl, cached)
		}
		return cachedTxData, nil
	}

	if err := proto.Unmarshal(any.Value, txData); err != nil {
		return nil, fmt.Errorf("cannot unpack TxData with type URL %q: %w", any.TypeUrl, err)
	}

	return txData, nil
}
