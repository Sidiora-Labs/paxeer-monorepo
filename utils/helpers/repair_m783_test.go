package helpers

import (
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestRepairM_783_AddressInvalidKey(t *testing.T) {
	privateKey, err := crypto.HexToECDSA("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	require.NoError(t, err)
	valid := crypto.FromECDSAPub(&privateKey.PublicKey)
	outOfField := make([]byte, 65)
	outOfField[0] = 4
	for i := 1; i < len(outOfField); i++ {
		outOfField[i] = 0xff
	}
	for _, tc := range []struct {
		name string
		key  []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"prefix only", []byte{4}},
		{"off curve zero point", append([]byte{4}, make([]byte, 64)...)},
		{"coordinates outside field", outOfField},
		{"truncated point", valid[:64]},
		{"trailing byte", append(append([]byte(nil), valid...), 0)},
		{"wrong prefix", []byte{1, 2, 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NotPanics(t, func() {
				evmAddr, err := PubkeyToEVMAddress(tc.key)
				require.ErrorIs(t, err, ErrInvalidPubkey)
				require.Equal(t, common.Address{}, evmAddr)

				paxPubkey, err := PubkeyBytesToPaxPubKey(tc.key)
				require.ErrorIs(t, err, ErrInvalidPubkey)
				require.Empty(t, paxPubkey.Key)

				evmAddr, paxAddr, pubkey, err := GetAddressesFromPubkeyBytes(tc.key)
				require.ErrorIs(t, err, ErrInvalidPubkey)
				require.Equal(t, common.Address{}, evmAddr)
				require.Empty(t, paxAddr)
				require.Nil(t, pubkey)
			})
		})
	}

	expectedPubkey := crypto.CompressPubkey(&privateKey.PublicKey)
	expectedEVM := crypto.PubkeyToAddress(privateKey.PublicKey)
	expectedPax := sdk.AccAddress((&secp256k1.PubKey{Key: expectedPubkey}).Address())
	for _, encoding := range [][]byte{valid, expectedPubkey} {
		paxPubkey, err := PubkeyBytesToPaxPubKey(encoding)
		require.NoError(t, err)
		require.Equal(t, expectedPubkey, paxPubkey.Bytes())
		require.Equal(t, expectedPax, sdk.AccAddress(paxPubkey.Address()))
	}
	_, err = PubkeyToEVMAddress(expectedPubkey)
	require.ErrorIs(t, err, ErrInvalidPubkey)
	_, _, _, err = GetAddressesFromPubkeyBytes(expectedPubkey)
	require.ErrorIs(t, err, ErrInvalidPubkey)

	hash := crypto.Keccak256Hash([]byte("repair-m783-address-conversion"))
	signature, err := crypto.Sign(hash.Bytes(), privateKey)
	require.NoError(t, err)
	evmAddr, paxAddr, pubkey, err := GetAddresses(
		big.NewInt(int64(signature[64])+27),
		new(big.Int).SetBytes(signature[:32]),
		new(big.Int).SetBytes(signature[32:64]), hash,
	)
	require.NoError(t, err)
	require.Equal(t, expectedEVM, evmAddr)
	require.Equal(t, expectedPax, paxAddr)
	require.Equal(t, expectedPubkey, pubkey.Bytes())
}

func TestRepairM_783_AddressTestChain(t *testing.T) {
	TestRecoverAddressesFromTx_Mainnet125(t)
}

func TestRepairM_783_AddressTestInvalidKey(t *testing.T) {
	TestEdgeCases(t)
	TestAddressConversions(t)
	TestKeyConversionConsistency(t)
}
