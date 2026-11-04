package secp256k1_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	btcSecp256k1 "github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/require"
)

type repairM1343ScalarCase struct {
	name  string
	key   []byte
	valid bool
}

func repairM1343ScalarCases() []repairM1343ScalarCase {
	n := btcSecp256k1.S256().N
	encode := func(v *big.Int) []byte {
		return v.FillBytes(make([]byte, secp256k1.PrivKeySize))
	}
	return []repairM1343ScalarCase{
		{"zero", make([]byte, 32), false},
		{"N", encode(n), false},
		{"N-plus-one", encode(new(big.Int).Add(n, big.NewInt(1))), false},
		{"maximum-256-bit", bytes.Repeat([]byte{0xff}, 32), false},
		{"one", encode(big.NewInt(1)), true},
		{"two", encode(big.NewInt(2)), true},
		{"N-minus-one", encode(new(big.Int).Sub(n, big.NewInt(1))), true},
		{"empty", []byte{}, false},
		{"short", bytes.Repeat([]byte{1}, 31), false},
		{"long", bytes.Repeat([]byte{1}, 33), false},
	}
}

func TestRepairM_1343_PrivateScalar(t *testing.T) {
	cdc := codec.NewLegacyAmino()
	msg := []byte("canonical imported secp256k1 key")
	hash := sha256.Sum256(msg)
	for _, tc := range repairM1343ScalarCases() {
		t.Run(tc.name, func(t *testing.T) {
			for _, decode := range []struct {
				name string
				fn   func(*secp256k1.PrivKey, []byte) error
			}{
				{"amino-callback", (*secp256k1.PrivKey).UnmarshalAmino},
				{"amino-json-callback", (*secp256k1.PrivKey).UnmarshalAminoJSON},
			} {
				t.Run(decode.name, func(t *testing.T) {
					key := secp256k1.GenPrivKeyFromSecret([]byte("existing key"))
					previous := bytes.Clone(key.Key)
					input := bytes.Clone(tc.key)
					err := decode.fn(key, input)
					if !tc.valid {
						require.Error(t, err)
						require.Equal(t, previous, key.Key, "invalid import must not replace the existing key")
						return
					}
					require.NoError(t, err)
					require.Equal(t, tc.key, key.Bytes(), "keep canonical leading zero bytes")
					clear(input)
					require.Equal(t, tc.key, key.Bytes(), "the decoder must own its validated bytes")
				})
			}

			producer := &secp256k1.PrivKey{Key: bytes.Clone(tc.key)}
			for _, transport := range []struct {
				name      string
				marshal   func(interface{}) ([]byte, error)
				unmarshal func([]byte, interface{}) error
			}{
				{"amino-binary", cdc.Marshal, cdc.Unmarshal},
				{"amino-json", cdc.MarshalAsJSON, cdc.UnmarshalAsJSON},
			} {
				t.Run(transport.name, func(t *testing.T) {
					wire, err := transport.marshal(producer)
					require.NoError(t, err)
					var consumer secp256k1.PrivKey
					err = transport.unmarshal(wire, &consumer)
					if !tc.valid {
						require.Error(t, err)
						return
					}
					require.NoError(t, err)
					require.Equal(t, tc.key, consumer.Bytes())
					sig, err := consumer.Sign(msg)
					require.NoError(t, err)
					require.Len(t, sig, 64)
					_, independentPub := btcSecp256k1.PrivKeyFromBytes(tc.key)
					require.Equal(t, independentPub.SerializeCompressed(), consumer.PubKey().Bytes())
					require.True(t, ecdsa.Verify(independentPub.ToECDSA(), hash[:],
						new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])))
					require.True(t, consumer.PubKey().VerifySignature(msg, sig))
				})
			}

			sig, err := producer.Sign(msg)
			if tc.valid {
				require.NoError(t, err)
				require.True(t, producer.PubKey().VerifySignature(msg, sig))
			} else {
				require.Error(t, err, "directly constructed or protobuf-decoded keys must not be reduced modulo N")
				require.Nil(t, sig)
			}
		})
	}
}

func TestRepairM_1343_NilPrivateScalar(t *testing.T) {
	key := secp256k1.GenPrivKeyFromSecret([]byte("existing key"))
	previous := bytes.Clone(key.Bytes())
	require.Error(t, key.UnmarshalAmino(nil))
	require.Equal(t, previous, key.Bytes())
	require.Error(t, key.UnmarshalAminoJSON(nil))
	require.Equal(t, previous, key.Bytes())
	sig, err := (&secp256k1.PrivKey{}).Sign([]byte("nil private scalar"))
	require.Error(t, err)
	require.Nil(t, sig)
}

func TestRepairM_1343_HighS(t *testing.T) {
	for _, tc := range repairM1343ScalarCases() {
		if !tc.valid {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			var key secp256k1.PrivKey
			require.NoError(t, key.UnmarshalAmino(tc.key))
			msg := []byte("high-S malleability refusal")
			hash := sha256.Sum256(msg)
			sig, err := key.Sign(msg)
			require.NoError(t, err)
			require.Len(t, sig, 64)
			pub := key.PubKey()
			require.True(t, pub.VerifySignature(msg, sig))
			require.False(t, pub.VerifySignature([]byte("different message"), sig))
			parsedPub, err := btcSecp256k1.ParsePubKey(pub.Bytes())
			require.NoError(t, err)
			r := new(big.Int).SetBytes(sig[:32])
			s := new(big.Int).SetBytes(sig[32:])
			require.True(t, ecdsa.Verify(parsedPub.ToECDSA(), hash[:], r, s))
			n := btcSecp256k1.S256().N
			highS := new(big.Int).Sub(n, s)
			require.Greater(t, highS.Cmp(new(big.Int).Rsh(new(big.Int).Set(n), 1)), 0)
			malleated := bytes.Clone(sig)
			highS.FillBytes(malleated[32:])
			require.Equal(t, sig[:32], malleated[:32])
			require.True(t, ecdsa.Verify(parsedPub.ToECDSA(), hash[:], r, highS))
			require.False(t, pub.VerifySignature(msg, malleated), "refuse a mathematically valid high-S signature")
		})
	}
}

func TestRepairM_1343_IndependentVerifier(t *testing.T) {
	TestSignAndValidateSecp256k1(t)
}
