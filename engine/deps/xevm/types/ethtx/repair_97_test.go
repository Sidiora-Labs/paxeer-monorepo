package ethtx_test

import (
	"math/big"
	"strings"
	"testing"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/types/ethtx"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gogo/protobuf/proto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func repair97Transport(t *testing.T, destination string, cache bool) (*evmtypes.MsgEVMTransaction, *ethtypes.Transaction) {
	t.Helper()
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	delegate := common.HexToAddress("0x1122334455667788990011223344556677889900")
	auth, err := ethtypes.SignSetCode(key, ethtypes.SetCodeAuthorization{
		ChainID: *uint256.NewInt(125), Address: delegate, Nonce: 23,
	})
	require.NoError(t, err)
	signer := ethtypes.NewPragueSigner(big.NewInt(125))
	signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.SetCodeTx{
		ChainID: uint256.NewInt(125), Nonce: 7,
		GasTipCap: uint256.NewInt(3), GasFeeCap: uint256.NewInt(100), Gas: 100000,
		To: common.HexToAddress(destination), Value: uint256.NewInt(31), Data: []byte{1, 2, 3},
		AccessList: ethtypes.AccessList{{Address: delegate, StorageKeys: []common.Hash{common.HexToHash("0x1234")}}},
		AuthList:   []ethtypes.SetCodeAuthorization{auth},
	}), signer, key)
	require.NoError(t, err)
	txData, err := ethtx.NewSetCodeTx(signed)
	require.NoError(t, err)
	// Change only the destination to exercise invalid protobuf input as well as valid address spellings.
	txData.To = destination
	produced, err := evmtypes.NewMsgEVMTransaction(txData)
	require.NoError(t, err)
	wire, err := proto.Marshal(produced)
	require.NoError(t, err)
	var decoded evmtypes.MsgEVMTransaction
	require.NoError(t, proto.Unmarshal(wire, &decoded))
	require.NotNil(t, decoded.Data)
	require.Nil(t, decoded.Data.GetCachedValue())
	if cache {
		registry := codectypes.NewInterfaceRegistry()
		evmtypes.RegisterInterfaces(registry)
		require.NoError(t, decoded.UnpackInterfaces(registry))
		require.NotNil(t, decoded.Data.GetCachedValue())
	}
	unpacked, err := evmtypes.UnpackTxData(decoded.Data)
	require.NoError(t, err)
	setCode, ok := unpacked.(*ethtx.SetCodeTx)
	require.True(t, ok)
	require.Equal(t, destination, setCode.To)
	require.Equal(t, signed.SetCodeAuthorizations(), setCode.GetAuthList())
	require.Len(t, setCode.AuthList, 1)
	require.NotEmpty(t, setCode.AuthList[0].R)
	require.NotEmpty(t, setCode.AuthList[0].S)
	authority, err := auth.Authority()
	require.NoError(t, err)
	require.Equal(t, crypto.PubkeyToAddress(key.PublicKey), authority)
	return &decoded, signed
}

func TestRepair_97_AbsentDestination(t *testing.T) {
	for _, cache := range []bool{false, true} {
		msg, _ := repair97Transport(t, "", cache)
		txData, err := evmtypes.UnpackTxData(msg.Data)
		require.NoError(t, err)
		require.EqualError(t, txData.Validate(), "invalid to address")
		require.EqualError(t, msg.ValidateBasic(), "invalid to address")
	}
}

func TestRepair_97_MalformedDestination(t *testing.T) {
	for _, destination := range []string{
		"0x", "0x1234", "not-an-address",
		"0x1122334455667788990011223344556677889900ff",
		"0x112233445566778899001122334455667788990g",
		" 0x1122334455667788990011223344556677889900",
	} {
		t.Run(destination, func(t *testing.T) {
			for _, cache := range []bool{false, true} {
				msg, _ := repair97Transport(t, destination, cache)
				txData, err := evmtypes.UnpackTxData(msg.Data)
				require.NoError(t, err)
				require.EqualError(t, txData.Validate(), "invalid to address")
				require.EqualError(t, msg.ValidateBasic(), "invalid to address")
			}
		})
	}
}

func TestRepair_97_ValidDestinationRoundTrip(t *testing.T) {
	address := common.HexToAddress("0xabcdef1234567890abcdef1234567890abcdef12")
	for _, destination := range []string{
		address.Hex(), strings.ToLower(address.Hex()), strings.ToUpper(address.Hex()),
		address.Hex()[2:], (common.Address{}).Hex(),
	} {
		t.Run(destination, func(t *testing.T) {
			for _, cache := range []bool{false, true} {
				msg, signed := repair97Transport(t, destination, cache)
				require.NoError(t, msg.ValidateBasic())
				var converted *ethtypes.Transaction
				var consumed ethtx.TxData
				require.NotPanics(t, func() { converted, consumed = msg.AsTransaction() })
				require.NotNil(t, converted)
				require.IsType(t, &ethtx.SetCodeTx{}, consumed)
				require.Equal(t, uint8(ethtypes.SetCodeTxType), converted.Type())
				require.NotNil(t, converted.To())
				require.Equal(t, common.HexToAddress(destination), *converted.To())
				require.Equal(t, signed.SetCodeAuthorizations(), converted.SetCodeAuthorizations())
				expectedWire, err := signed.MarshalBinary()
				require.NoError(t, err)
				actualWire, err := converted.MarshalBinary()
				require.NoError(t, err)
				require.Equal(t, expectedWire, actualWire)
				require.Equal(t, signed.Hash(), converted.Hash())
				signer := ethtypes.NewPragueSigner(signed.ChainId())
				expectedSender, err := ethtypes.Sender(signer, signed)
				require.NoError(t, err)
				actualSender, err := ethtypes.Sender(signer, converted)
				require.NoError(t, err)
				require.Equal(t, expectedSender, actualSender)
			}
		})
	}
}
