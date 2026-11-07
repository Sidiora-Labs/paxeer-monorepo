//go:build cgo && !gofuzz

package secp256k1_test

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	native "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1/internal/secp256k1"
	btcSecp256k1 "github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/require"
)

func TestRepairM_1343_Libsecp256k1ScalarBoundaries(t *testing.T) {
	msg := []byte("canonical imported secp256k1 key")
	hash := sha256.Sum256(msg)
	for _, tc := range repairM1343ScalarCases() {
		t.Run(tc.name, func(t *testing.T) {
			var key secp256k1.PrivKey
			importErr := key.UnmarshalAmino(tc.key)
			rsv, nativeErr := native.Sign(hash[:], tc.key)
			if !tc.valid {
				require.Error(t, importErr)
				require.ErrorIs(t, nativeErr, native.ErrInvalidKey)
				require.Nil(t, rsv)
				return
			}
			require.NoError(t, importErr)
			require.NoError(t, nativeErr)
			require.Len(t, rsv, 65)
			sdkSig, err := key.Sign(msg)
			require.NoError(t, err)
			require.Len(t, sdkSig, 64)
			pub := key.PubKey()
			require.True(t, pub.VerifySignature(msg, rsv[:64]))
			require.True(t, native.VerifySignature(pub.Bytes(), hash[:], sdkSig))
			parsedPub, err := btcSecp256k1.ParsePubKey(pub.Bytes())
			require.NoError(t, err)
			r := new(big.Int).SetBytes(rsv[:32])
			s := new(big.Int).SetBytes(rsv[32:64])
			require.True(t, ecdsa.Verify(parsedPub.ToECDSA(), hash[:], r, s))
		})
	}
}
