package ledger

import (
	"bytes"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

//go:embed testdata/device_signing.json
var deviceSigningJSON []byte

type deviceSigningFixture struct {
	PublicKey string `json:"public_key"`
	Address   string `json:"address"`
	HRP       string `json:"hrp"`
	Message   string `json:"message"`
	DER       string `json:"der"`
	Raw       string `json:"raw"`
}

func loadDeviceSigningFixture(t *testing.T) deviceSigningFixture {
	t.Helper()
	var fixture deviceSigningFixture
	require.NoError(t, json.Unmarshal(deviceSigningJSON, &fixture))
	return fixture
}

func decodeDeviceSigningHex(t *testing.T, value string) []byte {
	t.Helper()
	b, err := hex.DecodeString(value)
	require.NoError(t, err)
	return b
}

// This peer replays committed driver responses at the SECP256K1 boundary.
// The production key creation, confirmation, signing and DER decoder run unchanged.
type deviceSigningExchange struct {
	operation string
	hrp       string
	message   []byte
	response  []byte
	address   string
}

type deviceSigningReplay struct {
	t         *testing.T
	exchanges []deviceSigningExchange
}

func (r *deviceSigningReplay) next(operation string, path []uint32, hrp string, message []byte) deviceSigningExchange {
	r.t.Helper()
	require.NotEmpty(r.t, r.exchanges, "unexpected device request: %s", operation)
	e := r.exchanges[0]
	r.exchanges = r.exchanges[1:]
	require.Equal(r.t, e.operation, operation)
	if operation != "close" {
		require.Equal(r.t, []uint32{44, 118, 0, 0, 0}, path)
	}
	require.Equal(r.t, e.hrp, hrp)
	require.Equal(r.t, e.message, message)
	return e
}

func (r *deviceSigningReplay) Close() error {
	r.next("close", nil, "", nil)
	return nil
}

func (r *deviceSigningReplay) GetPublicKeySECP256K1(path []uint32) ([]byte, error) {
	return bytes.Clone(r.next("public key", path, "", nil).response), nil
}

func (r *deviceSigningReplay) GetAddressPubKeySECP256K1(path []uint32, hrp string) ([]byte, string, error) {
	e := r.next("confirm address", path, hrp, nil)
	return bytes.Clone(e.response), e.address, nil
}

func (r *deviceSigningReplay) SignSECP256K1(path []uint32, message []byte, mode byte) ([]byte, error) {
	require.Zero(r.t, mode)
	return bytes.Clone(r.next("sign", path, "", message).response), nil
}

func connectDeviceSigningReplay(t *testing.T, exchanges ...deviceSigningExchange) {
	t.Helper()
	r := &deviceSigningReplay{t: t, exchanges: exchanges}
	previous := discoverLedger
	discoverLedger = func() (SECP256K1, error) { return r, nil }
	t.Cleanup(func() {
		discoverLedger = previous
		require.Empty(t, r.exchanges, "device transcript was not fully consumed")
	})
}

func deviceSigningPath() hd.BIP44Params {
	return *hd.NewFundraiserParams(0, 118, 0)
}

func TestLedgerCreateShowAddress(t *testing.T) {
	f := loadDeviceSigningFixture(t)
	require.NotEqual(t, sdk.GetConfig().GetBech32AccountAddrPrefix(), f.HRP)
	pub := decodeDeviceSigningHex(t, f.PublicKey)
	require.Len(t, pub, 33)
	confirm := deviceSigningExchange{operation: "confirm address", hrp: f.HRP, response: pub, address: f.Address}
	get := deviceSigningExchange{operation: "public key", response: pub}
	close := deviceSigningExchange{operation: "close"}
	connectDeviceSigningReplay(t, confirm, close, get, confirm, close)
	key, address, err := NewPrivKeySecp256k1(deviceSigningPath(), f.HRP)
	require.NoError(t, err)
	require.Equal(t, pub, key.PubKey().Bytes())
	require.Equal(t, f.Address, address)
	addressBytes, err := sdk.GetFromBech32(address, f.HRP)
	require.NoError(t, err)
	require.Equal(t, []byte(key.PubKey().Address()), addressBytes)
	require.NoError(t, ShowAddress(deviceSigningPath(), key.PubKey(), f.HRP))
}

func TestLedgerShowAddressRefusesMismatchedKey(t *testing.T) {
	f := loadDeviceSigningFixture(t)
	pub := decodeDeviceSigningHex(t, f.PublicKey)
	other := &secp256k1.PubKey{Key: decodeDeviceSigningHex(t, "0260d0487a3dfce9228eee2d0d83a40f6131f551526c8e52066fe7fe1e4a509666")}
	t.Run("before confirmation", func(t *testing.T) {
		connectDeviceSigningReplay(t, deviceSigningExchange{operation: "public key", response: pub}, deviceSigningExchange{operation: "close"})
		require.ErrorContains(t, ShowAddress(deviceSigningPath(), other, f.HRP), "does not match")
	})
	t.Run("after confirmation", func(t *testing.T) {
		connectDeviceSigningReplay(t,
			deviceSigningExchange{operation: "public key", response: pub},
			deviceSigningExchange{operation: "confirm address", hrp: f.HRP, response: other.Key, address: f.Address},
			deviceSigningExchange{operation: "close"})
		require.ErrorContains(t, ShowAddress(deviceSigningPath(), &secp256k1.PubKey{Key: pub}, f.HRP), "does not match")
	})
}

func TestLedgerDERSigning(t *testing.T) {
	f := loadDeviceSigningFixture(t)
	pub := decodeDeviceSigningHex(t, f.PublicKey)
	for _, api := range []string{"SignWithPath", "PrivKey.Sign"} {
		t.Run(api, func(t *testing.T) {
			connectDeviceSigningReplay(t,
				deviceSigningExchange{operation: "public key", response: pub},
				deviceSigningExchange{operation: "sign", message: []byte(f.Message), response: decodeDeviceSigningHex(t, f.DER)},
				deviceSigningExchange{operation: "close"})
			var sig []byte
			var err error
			key := &secp256k1.PubKey{Key: pub}
			if api == "SignWithPath" {
				var returned cryptotypes.PubKey
				sig, returned, err = SignWithPath(deviceSigningPath(), []byte(f.Message))
				require.NoError(t, err)
				require.Equal(t, pub, returned.Bytes())
				require.True(t, returned.VerifySignature([]byte(f.Message), sig))
			} else {
				private := PrivKeyLedgerSecp256k1{CachedPubKey: key, Path: deviceSigningPath()}
				sig, err = private.Sign([]byte(f.Message))
				require.NoError(t, err)
			}
			require.Equal(t, decodeDeviceSigningHex(t, f.Raw), sig)
			require.True(t, key.VerifySignature([]byte(f.Message), sig))
			require.False(t, key.VerifySignature([]byte(f.Message+"tampered"), sig))
			other, err := btcec.ParsePubKey(decodeDeviceSigningHex(t, "0260d0487a3dfce9228eee2d0d83a40f6131f551526c8e52066fe7fe1e4a509666"))
			require.NoError(t, err)
			require.False(t, (&secp256k1.PubKey{Key: other.SerializeCompressed()}).VerifySignature([]byte(f.Message), sig))
		})
	}
}

func checkLedgerDERConversion(t *testing.T) {
	f := loadDeviceSigningFixture(t)
	converted, err := convertDERtoBER(decodeDeviceSigningHex(t, f.DER))
	require.NoError(t, err)
	require.Equal(t, decodeDeviceSigningHex(t, f.Raw), converted)
	// DER sign padding is removed and short integers are left-padded to 32 bytes.
	converted, err = convertDERtoBER(decodeDeviceSigningHex(t, "3009020200800203010000"))
	require.NoError(t, err)
	expected := make([]byte, 64)
	expected[31], expected[61] = 0x80, 1
	require.Equal(t, expected, converted)
}

func checkLedgerMalformedDER(t *testing.T) {
	f := loadDeviceSigningFixture(t)
	der := decodeDeviceSigningHex(t, f.DER)
	bad := [][]byte{
		nil, {}, {0x30}, der[:len(der)-1], append(bytes.Clone(der), 0),
		decodeDeviceSigningHex(t, "3106020101020101"),                                                                 // wrong sequence
		decodeDeviceSigningHex(t, "3006020180020101"),                                                                 // negative R
		decodeDeviceSigningHex(t, "3006020101020180"),                                                                 // negative S
		decodeDeviceSigningHex(t, "300702020001020101"),                                                               // redundant padding
		decodeDeviceSigningHex(t, "3006020100020101"),                                                                 // zero R
		decodeDeviceSigningHex(t, "3006020101020100"),                                                                 // zero S
		decodeDeviceSigningHex(t, "3006020001020101"),                                                                 // invalid integer length
		decodeDeviceSigningHex(t, "3026022100fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141020101"), // R >= order
	}
	for i, input := range bad {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			result, err := convertDERtoBER(input)
			require.Error(t, err)
			require.Nil(t, result)
			for _, api := range []string{"SignWithPath", "PrivKey.Sign"} {
				t.Run(api, func(t *testing.T) {
					pub := decodeDeviceSigningHex(t, f.PublicKey)
					connectDeviceSigningReplay(t,
						deviceSigningExchange{operation: "public key", response: pub},
						deviceSigningExchange{operation: "sign", message: []byte(f.Message), response: input},
						deviceSigningExchange{operation: "close"})
					if api == "SignWithPath" {
						sig, key, err := SignWithPath(deviceSigningPath(), []byte(f.Message))
						require.ErrorContains(t, err, "failed to parse")
						require.Nil(t, sig)
						require.Nil(t, key)
					} else {
						key := PrivKeyLedgerSecp256k1{CachedPubKey: &secp256k1.PubKey{Key: pub}, Path: deviceSigningPath()}
						sig, err := key.Sign([]byte(f.Message))
						require.ErrorContains(t, err, "failed to parse")
						require.Nil(t, sig)
					}
				})
			}
		})
	}
}

func TestLedgerDERConversion(t *testing.T) { checkLedgerDERConversion(t) }
func TestLedgerMalformedDER(t *testing.T)  { checkLedgerMalformedDER(t) }

func TestLedgerRawCompatibility(t *testing.T) {
	f := loadDeviceSigningFixture(t)
	raw := decodeDeviceSigningHex(t, f.Raw)
	converted, err := convertDERtoBER(raw)
	require.NoError(t, err)
	require.Equal(t, raw, converted)
	connectDeviceSigningReplay(t,
		deviceSigningExchange{operation: "public key", response: decodeDeviceSigningHex(t, f.PublicKey)},
		deviceSigningExchange{operation: "sign", message: []byte(f.Message), response: raw},
		deviceSigningExchange{operation: "close"})
	sig, key, err := SignWithPath(deviceSigningPath(), []byte(f.Message))
	require.NoError(t, err)
	require.Equal(t, raw, sig)
	require.True(t, key.VerifySignature([]byte(f.Message), sig))
}
