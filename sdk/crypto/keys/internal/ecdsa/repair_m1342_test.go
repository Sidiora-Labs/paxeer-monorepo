package ecdsa_test

import (
	"crypto/elliptic"
	"math/big"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"

	ecdsa "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/internal/ecdsa"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256r1"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/address"
)

func repairM1342Key(t *testing.T, scalar *big.Int) *secp256r1.PrivKey {
	t.Helper()
	key, err := secp256r1.GenPrivKey()
	require.NoError(t, err)
	key.Secret.D = new(big.Int).Set(scalar)
	key.Secret.X, key.Secret.Y = elliptic.P256().ScalarBaseMult(key.Bytes())
	return key
}

func TestRepairM_1342_PrivateScalarProtobuf(t *testing.T) {
	order := elliptic.P256().Params().N
	for _, tc := range []struct {
		name   string
		scalar *big.Int
		valid  bool
	}{
		{"zero", new(big.Int), false},
		{"N", new(big.Int).Set(order), false},
		{"N_plus_one", new(big.Int).Add(order, big.NewInt(1)), false},
		{"one", big.NewInt(1), true},
		{"N_minus_one", new(big.Int).Sub(order, big.NewInt(1)), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			producer := repairM1342Key(t, tc.scalar)
			wire, err := proto.Marshal(producer)
			require.NoError(t, err)

			var decoded secp256r1.PrivKey
			err = proto.Unmarshal(wire, &decoded)
			if !tc.valid {
				require.Error(t, err)
				require.NotNil(t, decoded.Secret)
				require.Nil(t, decoded.Secret.D)
				require.Nil(t, decoded.Secret.X)
				require.Nil(t, decoded.Secret.Y)
				return
			}
			require.NoError(t, err)
			require.Equal(t, 0, tc.scalar.Cmp(decoded.Secret.D))
			require.Equal(t, producer.Bytes(), decoded.Bytes())
			require.True(t, decoded.Secret.Curve.IsOnCurve(decoded.Secret.X, decoded.Secret.Y))
			require.True(t, decoded.PubKey().Equals(producer.PubKey()))
			roundTrip, err := proto.Marshal(&decoded)
			require.NoError(t, err)
			require.Equal(t, wire, roundTrip)

			message := []byte("repair 22.33.1342 scalar import")
			signature, err := decoded.Sign(message)
			require.NoError(t, err)
			require.True(t, decoded.PubKey().VerifySignature(message, signature))
			require.True(t, producer.PubKey().VerifySignature(message, signature))
			require.False(t, decoded.PubKey().VerifySignature([]byte("different message"), signature))
		})
	}
}

func TestRepairM_1342_PrivateStatePreserved(t *testing.T) {
	producer := repairM1342Key(t, big.NewInt(2))
	wire, err := proto.Marshal(producer)
	require.NoError(t, err)
	var decoded secp256r1.PrivKey
	require.NoError(t, proto.Unmarshal(wire, &decoded))
	receiver := &decoded.Secret.PrivKey
	before := receiver.PrivateKey
	beforeBytes := receiver.Bytes()
	order := elliptic.P256().Params().N
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"zero", repairM1342Key(t, new(big.Int)).Bytes()},
		{"N", repairM1342Key(t, order).Bytes()},
		{"N_plus_one", repairM1342Key(t, new(big.Int).Add(order, big.NewInt(1))).Bytes()},
		{"short", beforeBytes[:31]},
		{"long", append(append([]byte{}, beforeBytes...), 0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Error(t, receiver.Unmarshal(tc.data, elliptic.P256(), 32))
			require.Equal(t, before, receiver.PrivateKey)
			require.Equal(t, beforeBytes, receiver.Bytes())
			message := []byte("previous private key remains usable")
			signature, err := decoded.Sign(message)
			require.NoError(t, err)
			require.True(t, producer.PubKey().VerifySignature(message, signature))
		})
	}
}

func TestRepairM_1342_AddressCache(t *testing.T) {
	var receiver ecdsa.PubKey
	var firstAddress []byte
	for _, scalar := range []int64{1, 2} {
		producer := repairM1342Key(t, big.NewInt(scalar))
		public := producer.PubKey().(*secp256r1.PubKey)
		wire, err := proto.Marshal(public)
		require.NoError(t, err)
		var decoded secp256r1.PubKey
		require.NoError(t, proto.Unmarshal(wire, &decoded))
		name := proto.MessageName(&decoded)
		require.NoError(t, receiver.Unmarshal(decoded.Bytes(), elliptic.P256(), 33))
		require.Equal(t, public.Bytes(), receiver.Bytes())
		currentAddress := receiver.Address(name)
		require.EqualValues(t, address.Hash(name, public.Bytes()), currentAddress)
		require.EqualValues(t, decoded.Address(), currentAddress)
		require.EqualValues(t, public.Address(), currentAddress)
		require.EqualValues(t, currentAddress, receiver.Address(name))
		if firstAddress == nil {
			firstAddress = append([]byte{}, currentAddress...)
		} else {
			require.NotEqual(t, firstAddress, []byte(currentAddress))
		}

		message := []byte("current public key verifies the current signer")
		signature, err := producer.Sign(message)
		require.NoError(t, err)
		require.True(t, receiver.VerifySignature(message, signature))

		invalidPoint := append([]byte{}, decoded.Bytes()...)
		invalidPoint[0] = 4
		for _, invalid := range [][]byte{decoded.Bytes()[:32], invalidPoint} {
			require.Error(t, receiver.Unmarshal(invalid, elliptic.P256(), 33))
			require.Equal(t, public.Bytes(), receiver.Bytes())
			require.EqualValues(t, currentAddress, receiver.Address(name))
			require.True(t, receiver.VerifySignature(message, signature))
		}
	}
}
