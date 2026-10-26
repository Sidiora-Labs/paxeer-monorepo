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

func transferTestID(value byte) [32]byte {
	var id [32]byte
	for i := range id {
		id[i] = value
	}
	return id
}

func transferTestHex(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func transferTestGrant(t *testing.T) []byte {
	t.Helper()
	return transferTestHex(t,
		strings.Repeat("11", 32)+strings.Repeat("22", 32)+
			strings.Repeat("33", 32)+strings.Repeat("44", 32)+
			"0102030405060708090a0b0c0d0e0f10"+
			"1112131415161718191a1b1c1d1e1f20"+
			"01"+"2122232425262728"+"3132333435363738"+
			strings.Repeat("55", 32)+"01"+strings.Repeat("66", 32)+
			"4142434445464748"+strings.Repeat("77", 32)+strings.Repeat("88", 64))
}

func TestEncodeSendPayload(t *testing.T) {
	args := SendArgs{
		From: transferTestID(0x11), To: transferTestID(0x22), Asset: transferTestID(0x33),
		Amount: sdk.NewInt(5), Sequence: 1, IdempotencyKey: transferTestID(0x44),
		ExpiresAt: 0x0102030405060708, ContextHash: transferTestID(0x55),
		PublicKey: transferTestID(0x66), NetworkID: 0x11223344, ProtocolVersion: 0x0102,
	}
	payload, err := EncodeSendPayload(args)
	if err != nil {
		t.Fatal(err)
	}
	want := transferTestHex(t, "5301000a"+
		strings.Repeat("11", 32)+strings.Repeat("22", 32)+strings.Repeat("33", 32)+
		"00000000000000000000000000000005"+"0000000000000001"+
		strings.Repeat("44", 32)+"0102030405060708"+strings.Repeat("55", 32)+"00"+
		"01"+strings.Repeat("11", 32)+strings.Repeat("66", 32)+strings.Repeat("00", 64)+
		strings.Repeat("55", 32)+"11223344"+"0102")
	if len(payload) != 364 || !bytes.Equal(payload, want) {
		t.Fatalf("send wire = %x, want %x (364 bytes)", payload, want)
	}
	if err := ValidateSendEnvelope(payload); err != nil {
		t.Fatalf("encoded send refused: %v", err)
	}
	nineConditions := append([]byte(nil), payload[:197]...)
	nineConditions[196] = 9
	nineConditions = append(nineConditions, make([]byte, 9*9)...)
	nineConditions = append(nineConditions, payload[197:]...)
	if err := ValidateSendEnvelope(nineConditions); !errors.Is(err, ErrMalformedSend) {
		t.Fatalf("nine conditions: %v, want ErrMalformedSend", err)
	}

	maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
	args.Amount = sdk.NewIntFromBigInt(maximum)
	args.Sequence = ^uint64(0)
	args.ExpiresAt = ^uint64(0)
	payload, err = EncodeSendPayload(args)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(payload[100:124], bytes.Repeat([]byte{0xff}, 24)) ||
		!bytes.Equal(payload[156:164], bytes.Repeat([]byte{0xff}, 8)) {
		t.Fatalf("maximum amount/sequence/expiry encoded incorrectly: %x", payload)
	}
	for _, tc := range []struct {
		name   string
		amount sdk.Int
		want   error
	}{
		{"nil", sdk.Int{}, ErrNonCanonical},
		{"zero", sdk.ZeroInt(), ErrZeroAmount},
		{"negative", sdk.NewInt(-1), ErrNonCanonical},
		{"above u128", sdk.NewIntFromBigInt(new(big.Int).Lsh(big.NewInt(1), 128)), ErrNonCanonical},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args.Amount = tc.amount
			got, err := EncodeSendPayload(args)
			if !errors.Is(err, tc.want) || got != nil {
				t.Fatalf("payload = %x, err = %v, want nil / %v", got, err, tc.want)
			}
		})
	}
}

func TestEncodeReceivePayload(t *testing.T) {
	grant := transferTestGrant(t)
	args := ReceiveArgs{
		From: transferTestID(0x11), To: transferTestID(0x22), Asset: transferTestID(0x33),
		Amount: sdk.NewInt(7), GrantID: transferTestID(0x11), ReceiverSequence: 3,
		IdempotencyKey: transferTestID(0x44), ContextHash: transferTestID(0x55),
		PublicKey: transferTestID(0x66), NetworkID: 0x11223344, ProtocolVersion: 0x0102,
		PayerGrant: grant,
	}
	payload, err := EncodeReceivePayload(args)
	if err != nil {
		t.Fatal(err)
	}
	want := transferTestHex(t, "5201000a"+
		strings.Repeat("11", 32)+strings.Repeat("22", 32)+strings.Repeat("33", 32)+
		"00000000000000000000000000000007"+strings.Repeat("11", 32)+"0000000000000003"+
		strings.Repeat("44", 32)+strings.Repeat("55", 32)+
		"01"+strings.Repeat("22", 32)+strings.Repeat("66", 32)+strings.Repeat("00", 64)+
		strings.Repeat("55", 32)+"11223344"+"0102")
	want = append(want, grant...)
	if len(payload) != 733 || !bytes.Equal(payload, want) {
		t.Fatalf("receive wire = %x, want %x (733 bytes)", payload, want)
	}
	if !bytes.Equal(payload[100:116], want[100:116]) ||
		!bytes.Equal(payload[116:148], args.GrantID[:]) ||
		!bytes.Equal(payload[188:220], args.ContextHash[:]) ||
		!bytes.Equal(payload[285:349], make([]byte, 64)) {
		t.Fatal("receive amount, grant id, context hash or unsigned authorization has wrong offsets")
	}
	grantBefore := append([]byte(nil), grant...)
	payload[387] ^= 0xff
	if !bytes.Equal(grant, grantBefore) {
		t.Fatal("encoded payload aliases the payer grant")
	}
	args.Amount = sdk.NewIntFromBigInt(new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1)))
	args.ReceiverSequence = 0x0102030405060708
	payload, err = EncodeReceivePayload(args)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(payload[100:116], bytes.Repeat([]byte{0xff}, 16)) ||
		!bytes.Equal(payload[148:156], []byte{1, 2, 3, 4, 5, 6, 7, 8}) {
		t.Fatal("receive u128 amount or receiver sequence is not big-endian")
	}
	for _, tc := range []struct {
		name   string
		amount sdk.Int
		want   error
	}{
		{"nil", sdk.Int{}, ErrNonCanonical},
		{"zero", sdk.ZeroInt(), ErrZeroAmount},
		{"negative", sdk.NewInt(-1), ErrNonCanonical},
		{"above u128", sdk.NewIntFromBigInt(new(big.Int).Lsh(big.NewInt(1), 128)), ErrNonCanonical},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args.Amount = tc.amount
			got, err := EncodeReceivePayload(args)
			if !errors.Is(err, tc.want) || got != nil {
				t.Fatalf("payload = %x, err = %v, want nil / %v", got, err, tc.want)
			}
		})
	}
	args.Amount = sdk.NewInt(7)
	badReference := append([]byte(nil), grant...)
	badReference[209] = 2
	badRecurring := append([]byte(nil), grant...)
	badRecurring[160] = 2
	for _, bad := range [][]byte{nil, grant[:345], append(append([]byte(nil), grant...), 0), badReference, badRecurring} {
		args.PayerGrant = bad
		got, err := EncodeReceivePayload(args)
		if !errors.Is(err, ErrNonCanonical) || got != nil {
			t.Fatalf("invalid receive grant: payload = %x, err = %v", got, err)
		}
	}
}

func TestValidateIssueGrantPayload(t *testing.T) {
	grant := transferTestGrant(t)
	if len(grant) != 346 {
		t.Fatalf("grant wire length = %d, want 346", len(grant))
	}
	before := append([]byte(nil), grant...)
	if err := ValidateIssueGrantPayload(grant); err != nil {
		t.Fatalf("canonical grant refused: %v", err)
	}
	if !bytes.Equal(grant, before) {
		t.Fatal("validation mutated the grant")
	}
	for recurring := byte(0); recurring <= 1; recurring++ {
		for reference := byte(0); reference <= 1; reference++ {
			candidate := append([]byte(nil), grant...)
			candidate[160], candidate[209] = recurring, reference
			if err := ValidateIssueGrantPayload(candidate); err != nil {
				t.Fatalf("recurring/reference %d/%d refused: %v", recurring, reference, err)
			}
		}
	}
	for length := 0; length < len(grant); length++ {
		if err := ValidateIssueGrantPayload(grant[:length]); !errors.Is(err, ErrNonCanonical) {
			t.Fatalf("length %d: err = %v, want ErrNonCanonical", length, err)
		}
	}
	for _, candidate := range [][]byte{nil, append(append([]byte(nil), grant...), 0), make([]byte, 692)} {
		if err := ValidateIssueGrantPayload(candidate); !errors.Is(err, ErrNonCanonical) {
			t.Fatalf("length %d: err = %v, want ErrNonCanonical", len(candidate), err)
		}
	}
	for _, offset := range []int{160, 209} {
		for value := 2; value <= 255; value++ {
			candidate := append([]byte(nil), grant...)
			candidate[offset] = byte(value)
			if err := ValidateIssueGrantPayload(candidate); !errors.Is(err, ErrNonCanonical) {
				t.Fatalf("byte %d = %d: err = %v, want ErrNonCanonical", offset, value, err)
			}
		}
	}
}
