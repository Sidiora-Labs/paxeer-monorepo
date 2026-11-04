package ethtx_test

import (
	"fmt"
	"math/big"
	"testing"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/types/ethtx"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/ethereum/go-ethereum/params"
	"github.com/gogo/protobuf/proto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func repairM1128Blob(t *testing.T) *ethtypes.Transaction {
	t.Helper()
	key, err := crypto.HexToECDSA("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	require.NoError(t, err)
	sidecar := &ethtypes.BlobTxSidecar{}
	for i := 0; i < 2; i++ {
		var blob kzg4844.Blob
		blob[len(blob)-1] = byte(i + 1)
		commitment, err := kzg4844.BlobToCommitment(&blob)
		require.NoError(t, err)
		proof, err := kzg4844.ComputeBlobProof(&blob, commitment)
		require.NoError(t, err)
		sidecar.Blobs = append(sidecar.Blobs, blob)
		sidecar.Commitments = append(sidecar.Commitments, commitment)
		sidecar.Proofs = append(sidecar.Proofs, proof)
	}
	signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.BlobTx{
		ChainID: uint256.NewInt(125), Nonce: 7,
		GasTipCap: uint256.NewInt(3), GasFeeCap: uint256.NewInt(100), Gas: 100000,
		To:    common.HexToAddress("0x1122334455667788990011223344556677889900"),
		Value: uint256.NewInt(31), Data: []byte{1, 2, 3},
		BlobFeeCap: uint256.NewInt(5), BlobHashes: sidecar.BlobHashes(), Sidecar: sidecar,
	}), ethtypes.NewCancunSigner(big.NewInt(125)), key)
	require.NoError(t, err)
	return signed
}

func repairM1128Transport(t *testing.T, data proto.Message, cache bool) *evmtypes.MsgEVMTransaction {
	t.Helper()
	produced, err := evmtypes.NewMsgEVMTransaction(data)
	require.NoError(t, err)
	wire, err := proto.Marshal(produced)
	require.NoError(t, err)
	var decoded evmtypes.MsgEVMTransaction
	require.NoError(t, proto.Unmarshal(wire, &decoded))
	require.Nil(t, decoded.Data.GetCachedValue())
	if cache {
		registry := codectypes.NewInterfaceRegistry()
		evmtypes.RegisterInterfaces(registry)
		require.NoError(t, decoded.UnpackInterfaces(registry))
		require.NotNil(t, decoded.Data.GetCachedValue())
	}
	return &decoded
}

func repairM1128InvalidMessage(t *testing.T, data ethtx.TxData, field string) {
	t.Helper()
	produced, err := evmtypes.NewMsgEVMTransaction(data)
	require.NoError(t, err)
	require.NotPanics(t, func() { require.ErrorContains(t, produced.ValidateBasic(), field) })
	for _, cache := range []bool{false, true} {
		decoded := repairM1128Transport(t, data, cache)
		require.NotPanics(t, func() { require.ErrorContains(t, decoded.ValidateBasic(), field) })
		unpacked, err := evmtypes.UnpackTxData(decoded.Data)
		require.NoError(t, err)
		require.NotPanics(t, func() { require.ErrorContains(t, unpacked.Validate(), field) })
	}
}

func repairM1128SameTransaction(t *testing.T, expected, actual *ethtypes.Transaction, signer ethtypes.Signer) {
	t.Helper()
	require.NotNil(t, actual)
	want, err := expected.MarshalBinary()
	require.NoError(t, err)
	got, err := actual.MarshalBinary()
	require.NoError(t, err)
	require.Equal(t, want, got)
	require.Equal(t, expected.Hash(), actual.Hash())
	wantSender, err := ethtypes.Sender(signer, expected)
	require.NoError(t, err)
	gotSender, err := ethtypes.Sender(signer, actual)
	require.NoError(t, err)
	require.Equal(t, wantSender, gotSender)
}

func TestRepairM_1128_BlobMalformedMessage(t *testing.T) {
	signed := repairM1128Blob(t)
	cases := []struct {
		name   string
		field  string
		mutate func(*ethtx.BlobTx)
	}{
		{"missing_cap", "blob fee cap", func(tx *ethtx.BlobTx) { tx.BlobFeeCap = nil }},
		{"negative_cap", "blob fee cap", func(tx *ethtx.BlobTx) { cap := sdk.NewInt(-1); tx.BlobFeeCap = &cap }},
		{"missing_hashes", "blob hashes", func(tx *ethtx.BlobTx) { tx.BlobHashes = nil }},
		{"empty_hashes", "blob hashes", func(tx *ethtx.BlobTx) { tx.BlobHashes = [][]byte{} }},
		{"empty_sidecar", "counts", func(tx *ethtx.BlobTx) { tx.Sidecar = &ethtx.BlobTxSidecar{} }},
	}
	for _, field := range []string{"blobs", "commitments", "proofs"} {
		for _, count := range []int{0, 1, 3} {
			cases = append(cases, struct {
				name   string
				field  string
				mutate func(*ethtx.BlobTx)
			}{fmt.Sprintf("%s_count_%d", field, count), "counts", func(tx *ethtx.BlobTx) {
				items := make([][]byte, count)
				switch field {
				case "blobs":
					tx.Sidecar.Blobs = items
				case "commitments":
					tx.Sidecar.Commitments = items
				case "proofs":
					tx.Sidecar.Proofs = items
				}
			}})
		}
	}
	for index := 0; index < 2; index++ {
		for _, width := range []int{0, 1, 31, 33} {
			cases = append(cases, struct {
				name   string
				field  string
				mutate func(*ethtx.BlobTx)
			}{fmt.Sprintf("hash_%d_width_%d", index, width), "blob hash", func(tx *ethtx.BlobTx) {
				tx.BlobHashes[index] = make([]byte, width)
			}})
		}
		for _, version := range []byte{0, 2, 255} {
			cases = append(cases, struct {
				name   string
				field  string
				mutate func(*ethtx.BlobTx)
			}{fmt.Sprintf("hash_%d_version_%d", index, version), "version", func(tx *ethtx.BlobTx) {
				tx.BlobHashes[index][0] = version
			}})
		}
		for _, field := range []string{"blob", "commitment", "proof"} {
			width := len(kzg4844.Commitment{})
			if field == "blob" {
				width = len(kzg4844.Blob{})
			}
			for _, size := range []int{0, width - 1, width + 1} {
				cases = append(cases, struct {
					name   string
					field  string
					mutate func(*ethtx.BlobTx)
				}{fmt.Sprintf("%s_%d_width_%d", field, index, size), "length", func(tx *ethtx.BlobTx) {
					item := make([]byte, size)
					switch field {
					case "blob":
						tx.Sidecar.Blobs[index] = item
					case "commitment":
						tx.Sidecar.Commitments[index] = item
					case "proof":
						tx.Sidecar.Proofs[index] = item
					}
				}})
			}
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := ethtx.NewBlobTx(signed)
			require.NoError(t, err)
			tc.mutate(tx)
			repairM1128InvalidMessage(t, tx, tc.field)
		})
	}
}

func TestRepairM_1128_BlobUninitializedCap(t *testing.T) {
	tx, err := ethtx.NewBlobTx(repairM1128Blob(t))
	require.NoError(t, err)
	tx.BlobFeeCap = new(sdk.Int)
	require.NotPanics(t, func() { require.ErrorContains(t, tx.Validate(), "blob fee cap") })
	produced, err := evmtypes.NewMsgEVMTransaction(tx)
	require.NoError(t, err)
	// Marshaling sdk.Int initializes its pointer; restore the malformed cached input.
	tx.BlobFeeCap = new(sdk.Int)
	require.NotPanics(t, func() { require.ErrorContains(t, produced.ValidateBasic(), "blob fee cap") })
}

func TestRepairM_1128_BlobRoundTrip(t *testing.T) {
	signed := repairM1128Blob(t)
	for _, sidecar := range []bool{false, true} {
		for _, cache := range []bool{false, true} {
			t.Run(fmt.Sprintf("sidecar_%t_cache_%t", sidecar, cache), func(t *testing.T) {
				expected := signed
				if !sidecar {
					expected = signed.WithoutBlobTxSidecar()
				}
				tx, err := ethtx.NewBlobTx(expected)
				require.NoError(t, err)
				msg := repairM1128Transport(t, tx, cache)
				require.NoError(t, msg.ValidateBasic())
				var converted *ethtypes.Transaction
				require.NotPanics(t, func() { converted, _ = msg.AsTransaction() })
				repairM1128SameTransaction(t, expected, converted, ethtypes.NewCancunSigner(big.NewInt(125)))
				require.Equal(t, expected.BlobHashes(), converted.BlobHashes())
				require.Equal(t, expected.BlobTxSidecar(), converted.BlobTxSidecar())
				require.Equal(t, uint64(2*params.BlobTxBlobGasPerBlob), tx.BlobGas())
				require.Equal(t, big.NewInt(100000*100+2*params.BlobTxBlobGasPerBlob*5), tx.Fee())
				if sidecar {
					sc := converted.BlobTxSidecar()
					require.NoError(t, sc.ValidateBlobCommitmentHashes(converted.BlobHashes()))
					for i := range sc.Blobs {
						require.NoError(t, kzg4844.VerifyBlobProof(&sc.Blobs[i], sc.Commitments[i], sc.Proofs[i]))
					}
				}
			})
		}
	}
	tx, err := ethtx.NewBlobTx(signed)
	require.NoError(t, err)
	zero := sdk.ZeroInt()
	tx.BlobFeeCap = &zero
	msg := repairM1128Transport(t, tx, true)
	require.NoError(t, msg.ValidateBasic())
	converted, _ := msg.AsTransaction()
	require.Zero(t, converted.BlobGasFeeCap().Sign())
	require.Equal(t, big.NewInt(100000*100), tx.Fee())
}

func repairM1128SetCode(t *testing.T, domain *uint256.Int) *ethtypes.Transaction {
	t.Helper()
	key, err := crypto.HexToECDSA("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	require.NoError(t, err)
	delegate := common.HexToAddress("0x1122334455667788990011223344556677889900")
	auth, err := ethtypes.SignSetCode(key, ethtypes.SetCodeAuthorization{
		ChainID: *domain, Address: delegate, Nonce: 23,
	})
	require.NoError(t, err)
	signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.SetCodeTx{
		ChainID: uint256.NewInt(125), Nonce: 7,
		GasTipCap: uint256.NewInt(3), GasFeeCap: uint256.NewInt(100), Gas: 100000,
		To: delegate, Value: uint256.NewInt(31), Data: []byte{1, 2, 3},
		AccessList: ethtypes.AccessList{{Address: delegate, StorageKeys: []common.Hash{common.HexToHash("0x1234")}}},
		AuthList:   []ethtypes.SetCodeAuthorization{auth, auth},
	}), ethtypes.NewPragueSigner(big.NewInt(125)), key)
	require.NoError(t, err)
	return signed
}

func TestRepairM_1128_AuthDomains(t *testing.T) {
	max := new(uint256.Int).SetAllOne()
	for _, domain := range []*uint256.Int{uint256.NewInt(0), uint256.NewInt(125), max} {
		t.Run(domain.ToBig().String(), func(t *testing.T) {
			signed := repairM1128SetCode(t, domain)
			for _, cache := range []bool{false, true} {
				tx, err := ethtx.NewSetCodeTx(signed)
				require.NoError(t, err)
				msg := repairM1128Transport(t, tx, cache)
				require.NoError(t, msg.ValidateBasic())
				data, err := evmtypes.UnpackTxData(msg.Data)
				require.NoError(t, err)
				setCode := data.(*ethtx.SetCodeTx)
				checked, err := setCode.AuthList.ToEthAuthListChecked()
				require.NoError(t, err)
				require.Equal(t, signed.SetCodeAuthorizations(), *checked)
				var converted *ethtypes.Transaction
				require.NotPanics(t, func() { converted, _ = msg.AsTransaction() })
				repairM1128SameTransaction(t, signed, converted, ethtypes.NewPragueSigner(big.NewInt(125)))
				for i, auth := range converted.SetCodeAuthorizations() {
					require.Equal(t, domain, &auth.ChainID)
					want := signed.SetCodeAuthorizations()[i]
					wantAuthority, err := want.Authority()
					require.NoError(t, err)
					gotAuthority, err := auth.Authority()
					require.NoError(t, err)
					require.Equal(t, wantAuthority, gotAuthority)
				}
			}
		})
	}
}

func TestRepairM_1128_AuthMalformedMessage(t *testing.T) {
	signed := repairM1128SetCode(t, uint256.NewInt(125))
	max := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	for _, domain := range []*big.Int{big.NewInt(-1), big.NewInt(-125), new(big.Int).Neg(max)} {
		t.Run(domain.String(), func(t *testing.T) {
			tx, err := ethtx.NewSetCodeTx(signed)
			require.NoError(t, err)
			chainID := sdk.NewIntFromBigInt(domain)
			tx.AuthList[1].ChainID = &chainID
			repairM1128InvalidMessage(t, tx, "auth list chain id")
			var converted *[]ethtypes.SetCodeAuthorization
			require.NotPanics(t, func() { converted, err = tx.AuthList.ToEthAuthListChecked() })
			require.ErrorContains(t, err, "unsigned 256-bit")
			require.Nil(t, converted)
		})
	}
	t.Run("nil", func(t *testing.T) {
		tx, err := ethtx.NewSetCodeTx(signed)
		require.NoError(t, err)
		tx.AuthList[1].ChainID = nil
		repairM1128InvalidMessage(t, tx, "auth list chain id")
		converted, err := tx.AuthList.ToEthAuthListChecked()
		require.Error(t, err)
		require.Nil(t, converted)
	})
	t.Run("uninitialized", func(t *testing.T) {
		tx, err := ethtx.NewSetCodeTx(signed)
		require.NoError(t, err)
		tx.AuthList[1].ChainID = new(sdk.Int)
		require.NotPanics(t, func() { require.ErrorContains(t, tx.Validate(), "auth list chain id") })
		msg, err := evmtypes.NewMsgEVMTransaction(tx)
		require.NoError(t, err)
		tx.AuthList[1].ChainID = new(sdk.Int)
		require.NotPanics(t, func() { require.ErrorContains(t, msg.ValidateBasic(), "auth list chain id") })
		converted, err := tx.AuthList.ToEthAuthListChecked()
		require.Error(t, err)
		require.Nil(t, converted)
	})
}

func TestRepairM_1128_AuthOversizedDomain(t *testing.T) {
	tx, err := ethtx.NewSetCodeTx(repairM1128SetCode(t, uint256.NewInt(125)))
	require.NoError(t, err)
	// sdk.Int retains the supplied big.Int. Its producer can mutate it after construction.
	backing := big.NewInt(125)
	chainID := sdk.NewIntFromBigInt(backing)
	tx.AuthList[1].ChainID = &chainID
	backing.Lsh(big.NewInt(1), 256)
	require.NotPanics(t, func() { require.ErrorContains(t, tx.Validate(), "unsigned 256-bit") })
	converted, err := tx.AuthList.ToEthAuthListChecked()
	require.ErrorContains(t, err, "unsigned 256-bit")
	require.Nil(t, converted)
	msg, err := evmtypes.NewMsgEVMTransaction(tx)
	require.NoError(t, err)
	require.NotPanics(t, func() { require.ErrorContains(t, msg.ValidateBasic(), "auth list chain id") })
	decoded := repairM1128Transport(t, tx, false)
	require.NotPanics(t, func() { require.ErrorContains(t, decoded.ValidateBasic(), "integer out of range") })
	registry := codectypes.NewInterfaceRegistry()
	evmtypes.RegisterInterfaces(registry)
	require.ErrorContains(t, decoded.UnpackInterfaces(registry), "integer out of range")
}
