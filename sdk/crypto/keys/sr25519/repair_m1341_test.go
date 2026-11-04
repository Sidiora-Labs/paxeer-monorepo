package sr25519_test

import (
	"bytes"
	"fmt"
	"log"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/sr25519"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	"github.com/gogo/protobuf/proto"
)

func TestRepairM_1341_StringRedactsSecret(t *testing.T) {
	for name, key := range map[string]*sr25519.PrivKey{
		"derived": sr25519.GenPrivKeyFromSecret([]byte("repair-m1341-public-test-fixture")),
		"random":  sr25519.GenPrivKey(),
	} {
		t.Run(name, func(t *testing.T) {
			before := append([]byte(nil), key.Bytes()...)
			if len(before) != sr25519.PrivKeySize || bytes.Equal(before, make([]byte, sr25519.PrivKeySize)) {
				t.Fatal("expected an initialized nonzero private key")
			}
			if key.String() != "PrivKeySr25519{<redacted>}" {
				t.Fatal("String did not return the redacted sr25519 label")
			}
			if !bytes.Equal(before, key.Bytes()) {
				t.Fatal("String changed the private key")
			}
			message := []byte("signing still works after redaction")
			signature, err := key.Sign(message)
			if err != nil {
				t.Fatal("initialized key failed to sign")
			}
			if !key.PubKey().VerifySignature(message, signature) {
				t.Fatal("signature failed verification after formatting")
			}
		})
	}
}

func TestRepairM_1341_FormattingRedactsSecret(t *testing.T) {
	key := sr25519.GenPrivKeyFromSecret([]byte("repair-m1341-formatting-fixture"))
	if len(key.Bytes()) != sr25519.PrivKeySize {
		t.Fatal("expected an initialized private key")
	}
	var sdkKey cryptotypes.PrivKey = key
	var protoKey proto.Message = key
	for name, value := range map[string]interface{}{
		"concrete": key,
		"sdk":      sdkKey,
		"protobuf": protoKey,
	} {
		t.Run(name, func(t *testing.T) {
			for _, format := range []string{"%s", "%v", "%+v", "%q", "%x", "%X"} {
				t.Run(format, func(t *testing.T) {
					want := fmt.Sprintf(format, "PrivKeySr25519{<redacted>}")
					if fmt.Sprintf(format, value) != want {
						t.Fatal("fmt did not emit only the redacted label")
					}
				})
			}
			var output bytes.Buffer
			log.New(&output, "", 0).Print(value)
			if output.String() != "PrivKeySr25519{<redacted>}\n" {
				t.Fatal("logger did not emit only the redacted label")
			}
		})
	}
}
