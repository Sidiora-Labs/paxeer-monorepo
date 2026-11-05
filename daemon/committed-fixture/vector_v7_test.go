package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestVectorV7NamedPayeeVerifies(t *testing.T) {
	data, err := os.ReadFile("vectors/" + vectorV7Name + ".json")
	require.NoError(t, err)
	var vector v7Vector
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	require.NoError(t, decoder.Decode(&vector))
	require.Equal(t, vectorV7Name, vector.Name)
	require.NotEmpty(t, vector.Provenance)
	require.NotNil(t, vectors[vectorV7Name], "the executable must register the real V7 recorder")
	assertV4CommittedVerifies(t, &vector.v4Vector)

	p := vector.Parties
	for _, did := range []string{p.PayerDID, p.PayeeDID} {
		require.True(t, strings.HasPrefix(did, evmtypes.LayerXDidPrefix))
		key, err := hex.DecodeString(strings.TrimPrefix(did, evmtypes.LayerXDidPrefix))
		require.NoError(t, err)
		require.Len(t, key, 32)
		require.Equal(t, evmtypes.LayerXDid([32]byte(key)), did, "DIDs must use the production canonical form")
	}
	require.NotEqual(t, p.PayerDID, p.PayeeDID)
	require.NotEqual(t, common.Hash{}, p.Asset)
	mainName := "agent:" + p.PayeeDID + ":main"
	assetName := fmt.Sprintf("agent:%s:asset:%x", p.PayeeDID, p.Asset[:])
	require.Equal(t, mainName, p.PayeeMainAccount.Name)
	require.Equal(t, assetName, p.PayeeAssetAccount.Name)
	mainID, err := codec.DeriveAccountID([]byte(mainName))
	require.NoError(t, err)
	require.Equal(t, common.Hash(mainID), p.PayeeMainAccount.Identifier)
	// Independently reconstruct the native domain hash, including the four-byte
	// big-endian name length, rather than trusting the recorder's account helper.
	material := append([]byte("LX:ACCOUNT:v1"), binary.BigEndian.AppendUint32(nil, uint32(len(assetName)))...)
	material = append(material, []byte(assetName)...)
	require.Equal(t, common.Hash(sha256.Sum256(material)), p.PayeeAssetAccount.Identifier)
	assetID, err := codec.DeriveAccountID([]byte(assetName))
	require.NoError(t, err)
	require.Equal(t, common.Hash(assetID), p.PayeeAssetAccount.Identifier)
	require.NotEqual(t, p.PayeeMainAccount.Identifier, p.PayeeAssetAccount.Identifier)
	payerID, err := codec.DeriveAccountID([]byte(fmt.Sprintf("agent:%s:asset:%x", p.PayerDID, p.Asset[:])))
	require.NoError(t, err)

	var tx ethtypes.Transaction
	require.NoError(t, tx.UnmarshalBinary(vector.Settlement.RawTransaction))
	require.Equal(t, uint8(ethtypes.DynamicFeeTxType), tx.Type())
	require.Equal(t, big.NewInt(125), tx.ChainId())
	ledgerABI := layerxledger.GetABI()
	method := ledgerABI.Methods["payExact"]
	require.Len(t, tx.Data(), 4+6*32)
	require.Equal(t, method.ID, tx.Data()[:4])
	inputs, err := method.Inputs.Unpack(tx.Data()[4:])
	require.NoError(t, err)
	require.Equal(t, payerID, inputs[0].([32]byte), "payer provenance must name the debited asset account")
	require.Equal(t, p.PayeeAssetAccount.Identifier, common.Hash(inputs[1].([32]byte)))
	require.Equal(t, p.Asset, common.Hash(inputs[2].([32]byte)))
	require.Equal(t, "3", p.Amount)
	require.Equal(t, p.Amount, inputs[3].(*big.Int).String())
	require.NotEqual(t, common.Hash{}, p.Reference)
	require.Equal(t, p.Reference, common.Hash(inputs[5].([32]byte)))
	var payments int
	event := ledgerABI.Events["PaymentExact"]
	for _, log := range vector.Settlement.Receipt.Logs {
		if log.Address != common.HexToAddress(layerxledger.LedgerAddress) || len(log.Topics) == 0 || log.Topics[0] != event.ID {
			continue
		}
		payments++
		require.Len(t, log.Topics, 4)
		require.Equal(t, p.Reference, log.Topics[1])
		require.Equal(t, common.Hash(payerID), log.Topics[2])
		require.Equal(t, p.PayeeAssetAccount.Identifier, log.Topics[3], "authenticated destination must be the named payee asset account")
		values, err := event.Inputs.NonIndexed().Unpack(log.Data)
		require.NoError(t, err)
		require.Len(t, values, 2)
		require.Equal(t, p.Asset, common.Hash(values[0].([32]byte)))
		require.Equal(t, p.Amount, values[1].(*big.Int).String())
	}
	require.Equal(t, 1, payments)
	converted, err := os.ReadFile("../../sdk/platform/conformance/fixtures/" + vectorV7Name + ".json")
	require.NoError(t, err)
	require.Equal(t, data, converted, "conformance conversion must preserve every byte, including parties")
}
