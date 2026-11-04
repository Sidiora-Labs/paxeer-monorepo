package ethtx_test

import (
	"math/big"
	"strings"
	"testing"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/ante"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	txtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/gogo/protobuf/proto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func repairM1247Producer(t *testing.T, blob bool) (ethtx.EthereumTxData, *ethtypes.Transaction) {
	t.Helper()
	key, err := crypto.HexToECDSA(strings.Repeat("01", 32))
	require.NoError(t, err)
	to := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	var inner ethtypes.TxData
	if blob {
		inner = &ethtypes.BlobTx{
			ChainID: uint256.NewInt(125), GasTipCap: uint256.NewInt(2), GasFeeCap: uint256.NewInt(100),
			Gas: 100000, To: to, Value: uint256.NewInt(9), BlobFeeCap: uint256.NewInt(3),
			BlobHashes: []common.Hash{{1, 2}},
			Sidecar: &ethtypes.BlobTxSidecar{
				Blobs: []kzg4844.Blob{{}}, Commitments: []kzg4844.Commitment{{}}, Proofs: []kzg4844.Proof{{}},
			},
		}
	} else {
		auth, err := ethtypes.SignSetCode(key, ethtypes.SetCodeAuthorization{ChainID: *uint256.NewInt(125), Address: to})
		require.NoError(t, err)
		inner = &ethtypes.SetCodeTx{
			ChainID: uint256.NewInt(125), GasTipCap: uint256.NewInt(2), GasFeeCap: uint256.NewInt(100),
			Gas: 100000, To: to, Value: uint256.NewInt(9), AuthList: []ethtypes.SetCodeAuthorization{auth},
		}
	}
	tx, err := ethtypes.SignTx(ethtypes.NewTx(inner), ethtypes.NewPragueSigner(big.NewInt(125)), key)
	require.NoError(t, err)
	data, err := ethtx.NewTxDataFromTx(tx)
	require.NoError(t, err)
	return data, tx
}

func repairM1247Transport(t *testing.T, data ethtx.TxData) []*evmtypes.MsgEVMTransaction {
	t.Helper()
	msg, err := evmtypes.NewMsgEVMTransaction(data)
	require.NoError(t, err)
	wire, err := proto.Marshal(msg)
	require.NoError(t, err)
	var received evmtypes.MsgEVMTransaction
	require.NoError(t, proto.Unmarshal(wire, &received))
	require.Nil(t, received.Data.GetCachedValue())
	return []*evmtypes.MsgEVMTransaction{msg, &received}
}

func repairM1247Admission(t *testing.T, msg *evmtypes.MsgEVMTransaction) error {
	t.Helper()
	packed, err := codectypes.NewAnyWithValue(msg)
	require.NoError(t, err)
	tx := &txtypes.Tx{Body: &txtypes.TxBody{Messages: []*codectypes.Any{packed}}, AuthInfo: &txtypes.AuthInfo{Fee: &txtypes.Fee{}}}
	return ante.EvmStatelessChecks(sdk.Context{}, tx, big.NewInt(125))
}

func repairM1247Rejected(t *testing.T, data ethtx.TxData, want string) {
	t.Helper()
	require.NotPanics(t, func() { require.ErrorContains(t, data.Validate(), want) })
	for i, msg := range repairM1247Transport(t, data) {
		t.Run([]string{"cached", "protobuf"}[i], func(t *testing.T) {
			require.NotPanics(t, func() { require.ErrorContains(t, msg.ValidateBasic(), want) })
			require.NotPanics(t, func() { require.ErrorContains(t, repairM1247Admission(t, msg), want) })
		})
	}
}

func TestRepairM_1247_025_SetCodeDestination(t *testing.T) {
	for _, to := range []string{"", "0x12", "0x" + strings.Repeat("g", 40)} {
		t.Run(to, func(t *testing.T) {
			data, _ := repairM1247Producer(t, false)
			data.(*ethtx.SetCodeTx).To = to
			repairM1247Rejected(t, data, "invalid to address")
		})
	}
}

func TestRepairM_1247_026_BlobDimensions(t *testing.T) {
	type blobCase struct {
		name   string
		mutate func(*ethtx.BlobTx)
		want   string
	}
	cases := []blobCase{
		{"no hashes", func(tx *ethtx.BlobTx) { tx.BlobHashes = nil }, "blob hashes cannot be empty"},
		{"no destination", func(tx *ethtx.BlobTx) { tx.To = "" }, "invalid to address"},
	}
	for _, length := range []int{0, 31, 33} {
		length := length
		cases = append(cases, blobCase{
			name:   "hash length " + big.NewInt(int64(length)).String(),
			mutate: func(tx *ethtx.BlobTx) { tx.BlobHashes[0] = make([]byte, length) }, want: "blob hash 0",
		})
	}
	for _, field := range []string{"blobs", "commitments", "proofs"} {
		field := field
		for _, change := range []int{-1, 1} {
			change := change
			cases = append(cases, blobCase{
				name: field + " length delta " + big.NewInt(int64(change)).String(),
				mutate: func(tx *ethtx.BlobTx) {
					var list *[][]byte
					switch field {
					case "blobs":
						list = &tx.Sidecar.Blobs
					case "commitments":
						list = &tx.Sidecar.Commitments
					case "proofs":
						list = &tx.Sidecar.Proofs
					}
					(*list)[0] = make([]byte, len((*list)[0])+change)
				}, want: "must be",
			})
		}
		for _, count := range []int{0, 2} {
			count := count
			cases = append(cases, blobCase{
				name: field + " count " + big.NewInt(int64(count)).String(),
				mutate: func(tx *ethtx.BlobTx) {
					switch field {
					case "blobs":
						tx.Sidecar.Blobs = make([][]byte, count)
					case "commitments":
						tx.Sidecar.Commitments = make([][]byte, count)
					case "proofs":
						tx.Sidecar.Proofs = make([][]byte, count)
					}
				}, want: "list lengths must match",
			})
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, _ := repairM1247Producer(t, true)
			tc.mutate(data.(*ethtx.BlobTx))
			repairM1247Rejected(t, data, tc.want)
		})
	}
}

func repairM1247NumericFields(data ethtx.EthereumTxData) map[string]**sdk.Int {
	if tx, ok := data.(*ethtx.BlobTx); ok {
		return map[string]**sdk.Int{"chain ID": &tx.ChainID, "gas tip cap": &tx.GasTipCap, "gas fee cap": &tx.GasFeeCap, "amount": &tx.Amount, "blob fee cap": &tx.BlobFeeCap}
	}
	tx := data.(*ethtx.SetCodeTx)
	return map[string]**sdk.Int{"chain ID": &tx.ChainID, "gas tip cap": &tx.GasTipCap, "gas fee cap": &tx.GasFeeCap, "amount": &tx.Amount, "auth list chain id": &tx.AuthList[0].ChainID}
}

func TestRepairM_1247_025_026_MandatoryUint256(t *testing.T) {
	for _, blob := range []bool{false, true} {
		data, _ := repairM1247Producer(t, blob)
		for name := range repairM1247NumericFields(data) {
			for _, invalid := range []string{"missing", "negative", "uninitialized", "overflow"} {
				t.Run(proto.MessageName(data)+"/"+name+"/"+invalid, func(t *testing.T) {
					data, _ := repairM1247Producer(t, blob)
					field := repairM1247NumericFields(data)[name]
					switch invalid {
					case "missing":
						*field = nil
						repairM1247Rejected(t, data, name)
					case "negative":
						value := sdk.NewInt(-1)
						*field = &value
						repairM1247Rejected(t, data, name)
					case "uninitialized":
						// Protobuf normalizes an uninitialized Int to zero; exercise the cached input before marshaling.
						msg, err := evmtypes.NewMsgEVMTransaction(data)
						require.NoError(t, err)
						*field = &sdk.Int{}
						require.ErrorContains(t, data.Validate(), name)
						require.ErrorContains(t, msg.ValidateBasic(), name)
						require.ErrorContains(t, repairM1247Admission(t, msg), name)
					case "overflow":
						integer := big.NewInt(1)
						value := sdk.NewIntFromBigInt(integer)
						integer.Lsh(integer, 256)
						*field = &value
						require.ErrorContains(t, data.Validate(), name)
						for _, msg := range repairM1247Transport(t, data) {
							require.Error(t, msg.ValidateBasic())
							require.Error(t, repairM1247Admission(t, msg))
						}
					}
				})
			}
		}
	}
}

func TestRepairM_1247_025_026_ValidRoundTrip(t *testing.T) {
	for _, blob := range []bool{false, true} {
		data, original := repairM1247Producer(t, blob)
		t.Run(proto.MessageName(data), func(t *testing.T) {
			for _, msg := range repairM1247Transport(t, data) {
				require.NoError(t, msg.ValidateBasic())
				converted, _ := msg.AsTransaction()
				require.Equal(t, original.Hash(), converted.Hash())
				require.Equal(t, original.To(), converted.To())
				if blob {
					require.Equal(t, original.BlobTxSidecar(), converted.BlobTxSidecar())
					require.ErrorIs(t, repairM1247Admission(t, msg), sdkerrors.ErrUnsupportedTxType)
				} else {
					require.Equal(t, original.SetCodeAuthorizations(), converted.SetCodeAuthorizations())
					require.NoError(t, repairM1247Admission(t, msg))
				}
			}
		})
	}
	data, _ := repairM1247Producer(t, true)
	data.(*ethtx.BlobTx).Sidecar = nil
	for _, msg := range repairM1247Transport(t, data) {
		require.NoError(t, msg.ValidateBasic())
		require.ErrorIs(t, repairM1247Admission(t, msg), sdkerrors.ErrUnsupportedTxType)
	}
}
