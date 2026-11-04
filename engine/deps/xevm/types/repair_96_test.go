package types_test

import (
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/types/ethtx"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/gogo/protobuf/proto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

type repair96Transaction struct {
	name string
	data ethtx.TxData
	eth  *ethtypes.Transaction
}

func repair96Transactions(t *testing.T) []repair96Transaction {
	t.Helper()
	chainID := big.NewInt(125)
	to := common.HexToAddress("0x1122334455667788990011223344556677889900")
	accesses := ethtypes.AccessList{{Address: to, StorageKeys: []common.Hash{
		common.HexToHash("0x1234"), common.HexToHash("0x5678"),
	}}}
	data := []byte{0x01, 0x02, 0x03, 0xff}
	authorityKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	auth, err := ethtypes.SignSetCode(authorityKey, ethtypes.SetCodeAuthorization{
		ChainID: *uint256.NewInt(125), Address: to, Nonce: 23,
	})
	require.NoError(t, err)
	secondAuth, err := ethtypes.SignSetCode(authorityKey, ethtypes.SetCodeAuthorization{
		ChainID: *uint256.NewInt(125), Address: crypto.PubkeyToAddress(authorityKey.PublicKey), Nonce: 24,
	})
	require.NoError(t, err)
	var blob kzg4844.Blob
	blob[31] = 1
	commitment, err := kzg4844.BlobToCommitment(&blob)
	require.NoError(t, err)
	proof, err := kzg4844.ComputeBlobProof(&blob, commitment)
	require.NoError(t, err)
	sidecar := &ethtypes.BlobTxSidecar{
		Blobs: []kzg4844.Blob{blob}, Commitments: []kzg4844.Commitment{commitment}, Proofs: []kzg4844.Proof{proof},
	}
	inputs := []struct {
		name string
		data ethtypes.TxData
	}{
		{"LegacyTx", &ethtypes.LegacyTx{
			Nonce: 7, GasPrice: big.NewInt(100), Gas: 100000, To: &to, Value: big.NewInt(31), Data: data,
		}},
		{"AccessListTx", &ethtypes.AccessListTx{
			ChainID: chainID, Nonce: 8, GasPrice: big.NewInt(101), Gas: 100001, To: &to,
			Value: big.NewInt(32), Data: data, AccessList: accesses,
		}},
		{"DynamicFeeTx", &ethtypes.DynamicFeeTx{
			ChainID: chainID, Nonce: 9, GasTipCap: big.NewInt(3), GasFeeCap: big.NewInt(102), Gas: 100002,
			To: &to, Value: big.NewInt(33), Data: data, AccessList: accesses,
		}},
		{"BlobTx", &ethtypes.BlobTx{
			ChainID: uint256.NewInt(125), Nonce: 10, GasTipCap: uint256.NewInt(4), GasFeeCap: uint256.NewInt(103),
			Gas: 100003, To: to, Value: uint256.NewInt(34), Data: data, AccessList: accesses,
			BlobFeeCap: uint256.NewInt(5), BlobHashes: sidecar.BlobHashes(), Sidecar: sidecar,
		}},
		{"SetCodeTx", &ethtypes.SetCodeTx{
			ChainID: uint256.NewInt(125), Nonce: 11, GasTipCap: uint256.NewInt(6), GasFeeCap: uint256.NewInt(104),
			Gas: 100004, To: to, Value: uint256.NewInt(35), Data: data, AccessList: accesses,
			AuthList: []ethtypes.SetCodeAuthorization{auth, secondAuth},
		}},
	}
	transactions := make([]repair96Transaction, 0, 6)
	for _, input := range inputs {
		var signed *ethtypes.Transaction
		// Use a nonzero recovery parity so every signature field is populated on the wire.
		for {
			key, err := crypto.GenerateKey()
			require.NoError(t, err)
			signed, err = ethtypes.SignTx(ethtypes.NewTx(input.data), ethtypes.LatestSignerForChainID(chainID), key)
			require.NoError(t, err)
			v, r, s := signed.RawSignatureValues()
			require.Positive(t, r.Sign())
			require.Positive(t, s.Sign())
			if v.Sign() > 0 {
				break
			}
		}
		txData, err := ethtx.NewTxDataFromTx(signed)
		require.NoError(t, err)
		transactions = append(transactions, repair96Transaction{input.name, txData, signed})
	}
	associate, err := ethtx.NewAssociateTx(transactions[0].eth, "repair 96 association")
	require.NoError(t, err)
	return append(transactions, repair96Transaction{"AssociateTx", associate, nil})
}

func repair96Transport(t *testing.T, packed *codectypes.Any) *codectypes.Any {
	t.Helper()
	wire, err := proto.Marshal(packed)
	require.NoError(t, err)
	var transported codectypes.Any
	require.NoError(t, proto.Unmarshal(wire, &transported))
	require.Nil(t, transported.GetCachedValue())
	require.Equal(t, packed.TypeUrl, transported.TypeUrl)
	require.Equal(t, packed.Value, transported.Value)
	return &transported
}

func TestRepair_96_RegisteredTransactionRoundTrip(t *testing.T) {
	registry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)
	registered := registry.ListImplementations("paxprotocol.paxchain.gigaevm.TxData")
	transactions := repair96Transactions(t)
	require.Len(t, registered, len(transactions))
	for _, transaction := range transactions {
		t.Run(transaction.name, func(t *testing.T) {
			packed, err := types.PackTxData(transaction.data)
			require.NoError(t, err)
			require.Contains(t, registered, packed.TypeUrl)
			cached, err := types.UnpackTxData(packed)
			require.NoError(t, err)
			require.Same(t, transaction.data, cached)
			transported := repair96Transport(t, packed)
			decoded, err := types.UnpackTxData(transported)
			require.NoError(t, err)
			require.IsType(t, transaction.data, decoded)
			require.Equal(t, transaction.data, decoded)
			msg := &types.MsgEVMTransaction{Data: transported}
			ethTx, consumed := msg.AsTransaction()
			require.Equal(t, transaction.data, consumed)
			if transaction.eth == nil {
				require.Nil(t, ethTx)
				associate, ok := msg.GetAssociateTx()
				require.True(t, ok)
				require.Equal(t, transaction.data, associate)
			} else {
				require.NotNil(t, ethTx)
				require.Equal(t, transaction.eth.Hash(), ethTx.Hash())
				require.Equal(t, transaction.eth.SetCodeAuthorizations(), ethTx.SetCodeAuthorizations())
			}
			require.Nil(t, transported.GetCachedValue())
			require.NoError(t, msg.UnpackInterfaces(registry))
			require.NotNil(t, transported.GetCachedValue())
			decoded, err = types.UnpackTxData(transported)
			require.NoError(t, err)
			require.IsType(t, transaction.data, decoded)
			require.Equal(t, transaction.data, decoded)
		})
	}
}

func TestRepair_96_RejectUnknownAndMismatchedURLs(t *testing.T) {
	decoded, err := types.UnpackTxData(nil)
	require.Error(t, err)
	require.Nil(t, decoded)
	foreignCache, err := codectypes.NewAnyWithValue(&types.MsgEVMTransaction{})
	require.NoError(t, err)
	foreignCache.TypeUrl = "/" + proto.MessageName(&ethtx.LegacyTx{})
	decoded, err = types.UnpackTxData(foreignCache)
	require.Error(t, err)
	require.Nil(t, decoded)
	for _, transaction := range repair96Transactions(t) {
		t.Run(transaction.name, func(t *testing.T) {
			packed, err := types.PackTxData(transaction.data)
			require.NoError(t, err)
			correctURL := packed.TypeUrl
			if transaction.name == "LegacyTx" {
				mismatched := repair96Transport(t, packed)
				mismatched.TypeUrl = "/" + proto.MessageName(&ethtx.SetCodeTx{})
				decoded, err := types.UnpackTxData(mismatched)
				require.Error(t, err)
				require.Nil(t, decoded)
			}
			for _, url := range []string{"", "/unknown.TxData", "/" + proto.MessageName(&types.MsgEVMTransaction{}), "type.googleapis.com" + correctURL} {
				packed.TypeUrl = url
				for _, any := range []*codectypes.Any{packed, repair96Transport(t, packed)} {
					decoded, err := types.UnpackTxData(any)
					require.Error(t, err, "URL %q must be rejected", url)
					require.Nil(t, decoded)
				}
			}
			for _, other := range []ethtx.TxData{
				&ethtx.LegacyTx{}, &ethtx.AccessListTx{}, &ethtx.DynamicFeeTx{},
				&ethtx.BlobTx{}, &ethtx.SetCodeTx{}, &ethtx.AssociateTx{},
			} {
				packed.TypeUrl = "/" + proto.MessageName(other)
				if packed.TypeUrl == correctURL {
					continue
				}
				decoded, err := types.UnpackTxData(packed)
				require.Error(t, err, "cached type must match URL %q", packed.TypeUrl)
				require.Nil(t, decoded)
			}
		})
	}
}

func TestRepair_96_RejectMalformedPayload(t *testing.T) {
	for _, transaction := range repair96Transactions(t) {
		t.Run(transaction.name, func(t *testing.T) {
			packed, err := types.PackTxData(transaction.data)
			require.NoError(t, err)
			transported := repair96Transport(t, packed)
			transported.Value = append(transported.Value, 0x80)
			decoded, err := types.UnpackTxData(transported)
			require.Error(t, err)
			require.ErrorContains(t, err, transported.TypeUrl)
			require.Nil(t, decoded)
		})
	}
}
