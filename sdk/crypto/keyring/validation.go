package keyring

import (
	"fmt"
	"reflect"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/multisig"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/sr25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	"github.com/btcsuite/btcd/btcec/v2"
)

func isNilInterface(v interface{}) bool {
	return v == nil || (reflect.ValueOf(v).Kind() == reflect.Ptr && reflect.ValueOf(v).IsNil())
}

func publicKeyAlgorithm(pub types.PubKey) (hd.PubKeyType, error) {
	if isNilInterface(pub) {
		return "", fmt.Errorf("missing public key")
	}
	switch pk := pub.(type) {
	case *secp256k1.PubKey:
		if _, err := btcec.ParsePubKey(pk.Bytes()); err != nil {
			return "", fmt.Errorf("invalid secp256k1 public key")
		}
		return hd.Secp256k1Type, nil
	case *sr25519.PubKey:
		if len(pk.Bytes()) != 32 {
			return "", fmt.Errorf("invalid sr25519 public key size")
		}
		return hd.Sr25519Type, nil
	case *ed25519.PubKey:
		if len(pk.Bytes()) != ed25519.PubKeySize {
			return "", fmt.Errorf("invalid ed25519 public key size")
		}
		return hd.Ed25519Type, nil
	default:
		return "", fmt.Errorf("%w: public key type %q", ErrUnsupportedSigningAlgo, pub.Type())
	}
}

func validateInfo(info Info) error {
	if isNilInterface(info) {
		return fmt.Errorf("missing key information")
	}
	pub := info.GetPubKey()
	if isNilInterface(pub) {
		return fmt.Errorf("missing public key")
	}
	if info.GetType() == TypeMulti {
		pk, ok := pub.(*multisig.LegacyAminoPubKey)
		if !ok || pk == nil {
			return fmt.Errorf("invalid multisig public key")
		}
		for _, child := range pk.GetPubKeys() {
			if _, err := publicKeyAlgorithm(child); err != nil {
				return err
			}
		}
		return nil
	}
	actual, err := publicKeyAlgorithm(pub)
	if err != nil {
		return err
	}
	if info.GetType() == TypeOffline && info.GetAlgo() != actual {
		return fmt.Errorf("public key algorithm does not match stored metadata")
	}
	return nil
}
