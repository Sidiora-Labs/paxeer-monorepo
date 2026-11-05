package types

import (
	"bytes"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"
	"testing"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func TestEncodeRegisterPayload(t *testing.T) {
	assetID := assetPayloadTestID(0)
	salt := assetPayloadTestID(32)
	capBytes := []byte{0x80, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	base := RegisterArgs{
		AssetID: assetID, Salt: salt, Symbol: "PAX", Name: "Paxéer", Decimals: 18,
		SupplyCap:  sdk.NewIntFromBigInt(new(big.Int).SetBytes(capBytes)),
		IssuerKind: AssetIssuerPaxeerCustody, CustodyReference: []byte{0, 0xff, 0x7f},
	}
	t.Run("wire order and validator", func(t *testing.T) {
		got, err := EncodeRegisterPayload(base)
		if err != nil {
			t.Fatal(err)
		}
		assertAssetPayload(t, got, 100,
			"0001"+
				"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"+
				"202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f"+
				"0350415807506178c3a9657212"+
				"800102030405060708090a0b0c0d0e0f020300ff7f")
		if err := ValidateRegisterPayload(got); err != nil {
			t.Fatalf("encoder output refused: %v", err)
		}
		got[len(got)-1] = 0
		if !bytes.Equal(base.CustodyReference, []byte{0, 0xff, 0x7f}) {
			t.Fatal("encoding aliases the custody reference")
		}
	})
	maxU128 := sdk.NewIntFromBigInt(new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1)))
	valid := []struct {
		name   string
		args   RegisterArgs
		length int
	}{
		{"native minimum", RegisterArgs{Symbol: "A", Name: "B", SupplyCap: sdk.ZeroInt(), IssuerKind: AssetIssuerNative}, 89},
		{"maximum bounds", RegisterArgs{AssetID: assetID, Salt: salt, Symbol: strings.Repeat("S", 16), Name: strings.Repeat("é", 16), Decimals: 38, SupplyCap: maxU128, IssuerKind: AssetIssuerPaxeerCustody, CustodyReference: bytes.Repeat([]byte{0xff}, 128)}, 263},
		{"custody empty", RegisterArgs{Symbol: "A", Name: "B", SupplyCap: sdk.NewInt(1), IssuerKind: AssetIssuerPaxeerCustody}, 89},
		{"ASCII controls", RegisterArgs{Symbol: "\x00\x7f", Name: "B", SupplyCap: sdk.ZeroInt(), IssuerKind: AssetIssuerNative}, 90},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			got, err := EncodeRegisterPayload(tc.args)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tc.length {
				t.Fatalf("length = %d, want %d", len(got), tc.length)
			}
			if err := ValidateRegisterPayload(got); err != nil {
				t.Fatalf("encoder output refused: %v", err)
			}
			capOffset := 69 + len(tc.args.Symbol) + len(tc.args.Name)
			if tc.args.SupplyCap.IsZero() && !bytes.Equal(got[capOffset:capOffset+16], make([]byte, 16)) {
				t.Fatal("zero supply cap encoded as a nonzero integer")
			}
			if gotCap := new(big.Int).SetBytes(got[capOffset : capOffset+16]); gotCap.Cmp(tc.args.SupplyCap.BigInt()) != 0 {
				t.Fatalf("supply cap = %s, want %s", gotCap, tc.args.SupplyCap)
			}
		})
	}
	invalid := []struct {
		name   string
		change func(*RegisterArgs)
	}{
		{"empty symbol", func(a *RegisterArgs) { a.Symbol = "" }},
		{"symbol too long", func(a *RegisterArgs) { a.Symbol = strings.Repeat("A", 17) }},
		{"symbol length wrap", func(a *RegisterArgs) { a.Symbol = strings.Repeat("A", 257) }},
		{"symbol non ASCII", func(a *RegisterArgs) { a.Symbol = "é" }},
		{"empty name", func(a *RegisterArgs) { a.Name = "" }},
		{"name too long", func(a *RegisterArgs) { a.Name = strings.Repeat("n", 33) }},
		{"name byte bound", func(a *RegisterArgs) { a.Name = strings.Repeat("é", 17) }},
		{"name invalid UTF8", func(a *RegisterArgs) { a.Name = string([]byte{0xc0, 0x80}) }},
		{"name surrogate", func(a *RegisterArgs) { a.Name = string([]byte{0xed, 0xa0, 0x80}) }},
		{"decimals too large", func(a *RegisterArgs) { a.Decimals = 39 }},
		{"issuer zero", func(a *RegisterArgs) { a.IssuerKind = 0 }},
		{"issuer unknown", func(a *RegisterArgs) { a.IssuerKind = 3 }},
		{"native custody", func(a *RegisterArgs) { a.IssuerKind = AssetIssuerNative }},
		{"custody too long", func(a *RegisterArgs) { a.CustodyReference = make([]byte, 129) }},
		{"custody length wrap", func(a *RegisterArgs) { a.CustodyReference = make([]byte, 256) }},
		{"cap nil", func(a *RegisterArgs) { a.SupplyCap = sdk.Int{} }},
		{"cap negative", func(a *RegisterArgs) { a.SupplyCap = sdk.NewInt(-1) }},
		{"cap overflow", func(a *RegisterArgs) { a.SupplyCap = sdk.NewIntFromBigInt(new(big.Int).Lsh(big.NewInt(1), 128)) }},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			args := base
			tc.change(&args)
			got, err := EncodeRegisterPayload(args)
			if !errors.Is(err, ErrNonCanonical) || got != nil {
				t.Fatalf("payload = %x, err = %v, want nil, ErrNonCanonical", got, err)
			}
		})
	}
}

func TestEncodeFixedPayloads(t *testing.T) {
	assetID := assetPayloadTestID(0)
	accountID := assetPayloadTestID(32)
	const assetHex = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	const accountHex = "202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f"
	for _, tc := range []struct {
		name   string
		encode func([32]byte) []byte
	}{
		{"pause", EncodePausePayload},
		{"open account", EncodeOpenAccountPayload},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.encode(assetID)
			assertAssetPayload(t, got, 34, "0001"+assetHex)
			if err := ValidatePausePayload(got); err != nil {
				t.Fatal(err)
			}
			got[2] ^= 0xff
			assertAssetPayload(t, tc.encode(assetID), 34, "0001"+assetHex)
			assertAssetPayload(t, tc.encode([32]byte{}), 34, "0001"+strings.Repeat("00", 32))
		})
	}
	for _, tc := range []struct {
		name      string
		amountHex string
	}{
		{"one", "00000000000000000000000000000001"},
		{"high word", "00000000000000010000000000000000"},
		{"varied bytes", "800102030405060708090a0b0c0d0e0f"},
		{"maximum", "ffffffffffffffffffffffffffffffff"},
	} {
		t.Run("supply "+tc.name, func(t *testing.T) {
			integer, ok := new(big.Int).SetString(tc.amountHex, 16)
			if !ok {
				t.Fatal("invalid amount fixture")
			}
			amount := sdk.NewIntFromBigInt(integer)
			got, err := EncodeSupplyPayload(assetID, accountID, amount)
			if err != nil {
				t.Fatal(err)
			}
			assertAssetPayload(t, got, 82, "0001"+assetHex+accountHex+tc.amountHex)
			if amount.BigInt().Cmp(integer) != 0 {
				t.Fatal("encoder changed the amount")
			}
		})
	}
	for _, tc := range []struct {
		name   string
		amount sdk.Int
		want   error
	}{
		{"zero", sdk.ZeroInt(), ErrZeroAmount},
		{"nil", sdk.Int{}, ErrNonCanonical},
		{"negative", sdk.NewInt(-1), ErrNonCanonical},
		{"above u128", sdk.NewIntFromBigInt(new(big.Int).Lsh(big.NewInt(1), 128)), ErrNonCanonical},
		{"maximum sdk integer", sdk.NewIntFromBigInt(new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))), ErrNonCanonical},
	} {
		t.Run("refuse supply "+tc.name, func(t *testing.T) {
			got, err := EncodeSupplyPayload(assetID, accountID, tc.amount)
			if !errors.Is(err, tc.want) || got != nil {
				t.Fatalf("payload = %x, err = %v, want nil, %v", got, err, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		name     string
		sequence uint64
		want     string
	}{
		{"zero", 0, "0000000000000000"},
		{"one", 1, "0000000000000001"},
		{"varied bytes", 0x8001020304050607, "8001020304050607"},
		{"maximum", ^uint64(0), "ffffffffffffffff"},
	} {
		t.Run("revoke "+tc.name, func(t *testing.T) {
			assertAssetPayload(t, EncodeRevokeGrantPayload(accountID, tc.sequence), 42, "0001"+accountHex+tc.want)
		})
	}
}

func assetPayloadTestID(start byte) [32]byte {
	var id [32]byte
	for i := range id {
		id[i] = start + byte(i)
	}
	return id
}

func assertAssetPayload(t *testing.T, got []byte, length int, wantHex string) {
	t.Helper()
	want, err := hex.DecodeString(wantHex)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != length || len(want) != length || !bytes.Equal(got, want) {
		t.Fatalf("payload (%d bytes) = %x, want (%d bytes) %x; required length %d", len(got), got, len(want), want, length)
	}
}
