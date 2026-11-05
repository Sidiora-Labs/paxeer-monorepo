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

//go:embed testdata/repair_m1348.json
var repairM1348Evidence []byte

type repairM1348Fixture struct {
	PublicKey string `json:"public_key"`
	Address   string `json:"address"`
	HRP       string `json:"hrp"`
	Message   string `json:"message"`
	DER       string `json:"der"`
	Raw       string `json:"raw"`
}

func repairM1348Load(t *testing.T) repairM1348Fixture {
	t.Helper()
	var fixture repairM1348Fixture
	require.NoError(t, json.Unmarshal(repairM1348Evidence, &fixture))
	return fixture
}

func repairM1348Hex(t *testing.T, value string) []byte {
	t.Helper()
	b, err := hex.DecodeString(value)
	require.NoError(t, err)
	return b
}

// This peer replays committed driver responses at the SECP256K1 boundary.
// The production key creation, confirmation, signing and DER decoder run unchanged.
type repairM1348Exchange struct {
	operation string
	hrp       string
	message   []byte
	response  []byte
	address   string
}

type repairM1348Replay struct {
	t         *testing.T
	exchanges []repairM1348Exchange
}

func (r *repairM1348Replay) next(operation string, path []uint32, hrp string, message []byte) repairM1348Exchange {
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

func (r *repairM1348Replay) Close() error {
	r.next("close", nil, "", nil)
	return nil
}

func (r *repairM1348Replay) GetPublicKeySECP256K1(path []uint32) ([]byte, error) {
	return bytes.Clone(r.next("public key", path, "", nil).response), nil
}

func (r *repairM1348Replay) GetAddressPubKeySECP256K1(path []uint32, hrp string) ([]byte, string, error) {
	e := r.next("confirm address", path, hrp, nil)
	return bytes.Clone(e.response), e.address, nil
}

func (r *repairM1348Replay) SignSECP256K1(path []uint32, message []byte, mode byte) ([]byte, error) {
	require.Zero(r.t, mode)
	return bytes.Clone(r.next("sign", path, "", message).response), nil
}

func repairM1348Connect(t *testing.T, exchanges ...repairM1348Exchange) {
	t.Helper()
	r := &repairM1348Replay{t: t, exchanges: exchanges}
	previous := discoverLedger
	discoverLedger = func() (SECP256K1, error) { return r, nil }
	t.Cleanup(func() {
		discoverLedger = previous
		require.Empty(t, r.exchanges, "device transcript was not fully consumed")
	})
}

func repairM1348Path() hd.BIP44Params {
	return *hd.NewFundraiserParams(0, 118, 0)
}

func TestRepairM_1348_CreateShowAddress(t *testing.T) {
	f := repairM1348Load(t)
	require.NotEqual(t, sdk.GetConfig().GetBech32AccountAddrPrefix(), f.HRP)
	pub := repairM1348Hex(t, f.PublicKey)
	require.Len(t, pub, 33)
	confirm := repairM1348Exchange{operation: "confirm address", hrp: f.HRP, response: pub, address: f.Address}
	get := repairM1348Exchange{operation: "public key", response: pub}
	close := repairM1348Exchange{operation: "close"}
	repairM1348Connect(t, confirm, close, get, confirm, close)
	key, address, err := NewPrivKeySecp256k1(repairM1348Path(), f.HRP)
	require.NoError(t, err)
	require.Equal(t, pub, key.PubKey().Bytes())
	require.Equal(t, f.Address, address)
	addressBytes, err := sdk.GetFromBech32(address, f.HRP)
	require.NoError(t, err)
	require.Equal(t, []byte(key.PubKey().Address()), addressBytes)
	require.NoError(t, ShowAddress(repairM1348Path(), key.PubKey(), f.HRP))
}

func TestRepairM_1348_ShowAddressRefusesMismatchedKey(t *testing.T) {
	f := repairM1348Load(t)
	pub := repairM1348Hex(t, f.PublicKey)
	other := &secp256k1.PubKey{Key: repairM1348Hex(t, "0260d0487a3dfce9228eee2d0d83a40f6131f551526c8e52066fe7fe1e4a509666")}
	t.Run("before confirmation", func(t *testing.T) {
		repairM1348Connect(t, repairM1348Exchange{operation: "public key", response: pub}, repairM1348Exchange{operation: "close"})
		require.ErrorContains(t, ShowAddress(repairM1348Path(), other, f.HRP), "does not match")
	})
	t.Run("after confirmation", func(t *testing.T) {
		repairM1348Connect(t,
			repairM1348Exchange{operation: "public key", response: pub},
			repairM1348Exchange{operation: "confirm address", hrp: f.HRP, response: other.Key, address: f.Address},
			repairM1348Exchange{operation: "close"})
		require.ErrorContains(t, ShowAddress(repairM1348Path(), &secp256k1.PubKey{Key: pub}, f.HRP), "does not match")
	})
}

func TestRepairM_1348_DERSigning(t *testing.T) {
	f := repairM1348Load(t)
	pub := repairM1348Hex(t, f.PublicKey)
	for _, api := range []string{"SignWithPath", "PrivKey.Sign"} {
		t.Run(api, func(t *testing.T) {
			repairM1348Connect(t,
				repairM1348Exchange{operation: "public key", response: pub},
				repairM1348Exchange{operation: "sign", message: []byte(f.Message), response: repairM1348Hex(t, f.DER)},
				repairM1348Exchange{operation: "close"})
			var sig []byte
			var err error
			key := &secp256k1.PubKey{Key: pub}
			if api == "SignWithPath" {
				var returned cryptotypes.PubKey
				sig, returned, err = SignWithPath(repairM1348Path(), []byte(f.Message))
				require.NoError(t, err)
				require.Equal(t, pub, returned.Bytes())
				require.True(t, returned.VerifySignature([]byte(f.Message), sig))
			} else {
				private := PrivKeyLedgerSecp256k1{CachedPubKey: key, Path: repairM1348Path()}
				sig, err = private.Sign([]byte(f.Message))
				require.NoError(t, err)
			}
			require.Equal(t, repairM1348Hex(t, f.Raw), sig)
			require.True(t, key.VerifySignature([]byte(f.Message), sig))
			require.False(t, key.VerifySignature([]byte(f.Message+"tampered"), sig))
			other, err := btcec.ParsePubKey(repairM1348Hex(t, "0260d0487a3dfce9228eee2d0d83a40f6131f551526c8e52066fe7fe1e4a509666"))
			require.NoError(t, err)
			require.False(t, (&secp256k1.PubKey{Key: other.SerializeCompressed()}).VerifySignature([]byte(f.Message), sig))
		})
	}
}

func repairM1348DERConversion(t *testing.T) {
	f := repairM1348Load(t)
	converted, err := convertDERtoBER(repairM1348Hex(t, f.DER))
	require.NoError(t, err)
	require.Equal(t, repairM1348Hex(t, f.Raw), converted)
	// DER sign padding is removed and short integers are left-padded to 32 bytes.
	converted, err = convertDERtoBER(repairM1348Hex(t, "3009020200800203010000"))
	require.NoError(t, err)
	expected := make([]byte, 64)
	expected[31], expected[61] = 0x80, 1
	require.Equal(t, expected, converted)
}

func repairM1348MalformedDER(t *testing.T) {
	f := repairM1348Load(t)
	der := repairM1348Hex(t, f.DER)
	bad := [][]byte{
		nil, {}, {0x30}, der[:len(der)-1], append(bytes.Clone(der), 0),
		repairM1348Hex(t, "3106020101020101"),   // wrong sequence
		repairM1348Hex(t, "3006020180020101"),   // negative R
		repairM1348Hex(t, "3006020101020180"),   // negative S
		repairM1348Hex(t, "300702020001020101"), // redundant padding
		repairM1348Hex(t, "3006020100020101"),   // zero R
		repairM1348Hex(t, "3006020101020100"),   // zero S
		repairM1348Hex(t, "3006020001020101"),   // invalid integer length
		repairM1348Hex(t, "3026022100fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141020101"), // R >= order
	}
	for i, input := range bad {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			result, err := convertDERtoBER(input)
			require.Error(t, err)
			require.Nil(t, result)
			for _, api := range []string{"SignWithPath", "PrivKey.Sign"} {
				t.Run(api, func(t *testing.T) {
					pub := repairM1348Hex(t, f.PublicKey)
					repairM1348Connect(t,
						repairM1348Exchange{operation: "public key", response: pub},
						repairM1348Exchange{operation: "sign", message: []byte(f.Message), response: input},
						repairM1348Exchange{operation: "close"})
					if api == "SignWithPath" {
						sig, key, err := SignWithPath(repairM1348Path(), []byte(f.Message))
						require.ErrorContains(t, err, "failed to parse")
						require.Nil(t, sig)
						require.Nil(t, key)
					} else {
						key := PrivKeyLedgerSecp256k1{CachedPubKey: &secp256k1.PubKey{Key: pub}, Path: repairM1348Path()}
						sig, err := key.Sign([]byte(f.Message))
						require.ErrorContains(t, err, "failed to parse")
						require.Nil(t, sig)
					}
				})
			}
		})
	}
}

func TestRepairM_1348_DERConversion(t *testing.T) { repairM1348DERConversion(t) }
func TestRepairM_1348_MalformedDER(t *testing.T)  { repairM1348MalformedDER(t) }

func TestRepairM_1348_RawCompatibility(t *testing.T) {
	f := repairM1348Load(t)
	raw := repairM1348Hex(t, f.Raw)
	converted, err := convertDERtoBER(raw)
	require.NoError(t, err)
	require.Equal(t, raw, converted)
	repairM1348Connect(t,
		repairM1348Exchange{operation: "public key", response: repairM1348Hex(t, f.PublicKey)},
		repairM1348Exchange{operation: "sign", message: []byte(f.Message), response: raw},
		repairM1348Exchange{operation: "close"})
	sig, key, err := SignWithPath(repairM1348Path(), []byte(f.Message))
	require.NoError(t, err)
	require.Equal(t, raw, sig)
	require.True(t, key.VerifySignature([]byte(f.Message), sig))
}
