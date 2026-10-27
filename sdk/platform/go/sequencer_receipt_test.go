package layerx

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
)

type sequencerReceiptCase struct {
	ID             string          `json:"id"`
	Vector         string          `json:"vector"`
	RawTransaction string          `json:"rawTransaction"`
	ReceiptJSON    json.RawMessage `json:"receiptJson"`
	SignedReceipt  string          `json:"signedReceipt"`
	ReceiptDigest  string          `json:"receiptDigest"`
	PrevSeqHash    string          `json:"prevSeqHash"`
	LXPCode        string          `json:"lxpCode"`
	PinnedKey      struct {
		KeyID       string `json:"keyId"`
		PublicKey   string `json:"publicKey"`
		FirstHeight uint64 `json:"firstHeight"`
		LastHeight  uint64 `json:"lastHeight"`
	} `json:"pinnedKey"`
	Expected struct {
		Rung      string `json:"rung"`
		RefusedAt string `json:"refusedAt"`
		Check     string `json:"check"`
	} `json:"expected"`
}

func sequencerReceiptCases(t *testing.T) []sequencerReceiptCase {
	t.Helper()
	data, err := os.ReadFile("../conformance/fixtures/sequencer-receipt-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []sequencerReceiptCase `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 11 {
		t.Fatalf("expected 11 conformance cases, got %d", len(fixture.Cases))
	}
	return fixture.Cases
}

func sequencerFixtureBytes(t *testing.T, value string) []byte {
	t.Helper()
	if !strings.HasPrefix(value, "0x") {
		t.Fatalf("fixture is not hex: %q", value)
	}
	data, err := hex.DecodeString(value[2:])
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func sequencerFixtureKeys(t *testing.T, c sequencerReceiptCase) ReceiptKeys {
	t.Helper()
	array := func(value string) [32]byte {
		data := sequencerFixtureBytes(t, value)
		if len(data) != 32 {
			t.Fatalf("expected 32 bytes, got %d", len(data))
		}
		var out [32]byte
		copy(out[:], data)
		return out
	}
	return ReceiptKeys{Pins: []ReceiptKey{{KeyID: array(c.PinnedKey.KeyID), PublicKey: array(c.PinnedKey.PublicKey), FirstHeight: c.PinnedKey.FirstHeight, LastHeight: c.PinnedKey.LastHeight}}, PreviousSequenceHash: array(c.PrevSeqHash)}
}

func TestSequencerReceiptV1CasesVerifyReceipted(t *testing.T) {
	count := 0
	for _, c := range sequencerReceiptCases(t) {
		if c.Vector != "V1" {
			continue
		}
		count++
		t.Run(c.ID, func(t *testing.T) {
			keys := sequencerFixtureKeys(t, c)
			raw := sequencerFixtureBytes(t, c.RawTransaction)
			verified, err := VerifySequencerReceipt(c.ReceiptJSON, raw, keys)
			if err != nil {
				t.Fatal(err)
			}
			if verified.Rung != "receipted" || verified.Level != c.Expected.Rung || verified.Status != 1 || verified.Revert != nil {
				t.Fatalf("unexpected rung/status/revert: %+v", verified)
			}
			if !bytes.Equal(verified.CanonicalBytes, sequencerFixtureBytes(t, c.SignedReceipt)) || !bytes.Equal(verified.ReceiptDigest[:], sequencerFixtureBytes(t, c.ReceiptDigest)) {
				t.Fatal("receipt bytes or signed digest differ from fixture")
			}
			if verified.Receipt.ContextHash != keys.Pins[0].KeyID || verified.Receipt.ActivityID != receiptKeccak256(raw) {
				t.Fatal("receipt does not bind the pinned key and raw transaction")
			}
			var view sequencerReceiptJSON
			if err := json.Unmarshal(c.ReceiptJSON, &view); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(verified.ReturnData, sequencerFixtureBytes(t, view.ReturnData)) || len(verified.Logs) != len(view.Logs) {
				t.Fatal("verified outputs differ from JSON")
			}
			height, err := strconv.ParseUint(view.Height[2:], 16, 64)
			if err != nil {
				t.Fatal(err)
			}
			keys.Pins[0].FirstHeight = height
			keys.Pins[0].LastHeight = height
			standby := keys.Pins[0]
			primary := standby
			primary.KeyID[0] ^= 1
			primary.PublicKey[0] ^= 1
			keys.Pins = []ReceiptKey{primary, standby}
			if _, err := VerifySequencerReceipt(c.ReceiptJSON, raw, keys); err != nil {
				t.Fatalf("standby pin at inclusive height bounds: %v", err)
			}
		})
	}
	if count != 4 {
		t.Fatalf("executed %d V1 cases, expected 4", count)
	}
}

func TestSequencerReceiptV2RevertDecodesReason(t *testing.T) {
	count := 0
	for _, c := range sequencerReceiptCases(t) {
		if c.Vector != "V2" {
			continue
		}
		count++
		t.Run(c.ID, func(t *testing.T) {
			verified, err := VerifySequencerReceipt(c.ReceiptJSON, sequencerFixtureBytes(t, c.RawTransaction), sequencerFixtureKeys(t, c))
			if err != nil {
				t.Fatalf("receipted revert is not a verification failure: %v", err)
			}
			if verified.Rung != "receipted" || verified.Level != c.Expected.Rung || verified.Status != 0 || verified.Receipt.ResultCode != 1 || verified.Revert == nil {
				t.Fatalf("unexpected revert result: %+v", verified)
			}
			if verified.Revert.Code != c.LXPCode || !strings.HasPrefix(verified.Revert.Reason, c.LXPCode+":") || verified.Revert.Reason != "LXP_INSUFFICIENT_BALANCE: insufficient balance" {
				t.Fatalf("decoded reason: %+v", verified.Revert)
			}
			var typed *ReceiptRevert
			if !errors.As(verified.Revert, &typed) || errors.Is(verified.Revert, ErrVerification) || typed.Error() != typed.Reason {
				t.Fatal("revert did not retain its distinct error type")
			}
			if !bytes.Equal(typed.ReturnData, verified.ReturnData) || !bytes.Equal(verified.ReceiptDigest[:], sequencerFixtureBytes(t, c.ReceiptDigest)) {
				t.Fatal("revert lost authenticated data")
			}
		})
	}
	if count != 1 {
		t.Fatalf("executed %d V2 cases, expected 1", count)
	}
}

func TestSequencerReceiptV3RefusalsNameTheirCheck(t *testing.T) {
	expected := map[string]ReceiptCheck{
		"V3-key-not-pinned":           ReceiptCheckKeyID,
		"V3-height-outside-key-range": ReceiptCheckKeyHeight,
		"V3-activity-id-not-raw-hash": ReceiptCheckRawTransaction,
		"V3-seq-hash-mismatch":        ReceiptCheckSequenceHash,
		"V3-tampered-log":             ReceiptCheckLogs,
		"V3-non-canonical-bytes":      ReceiptCheckCanonicalEncoding,
	}
	count := 0
	cases := sequencerReceiptCases(t)
	assertRefusal := func(t *testing.T, view, raw []byte, keys ReceiptKeys, want ReceiptCheck) {
		t.Helper()
		verified, err := VerifySequencerReceipt(view, raw, keys)
		var refusal *VerificationError
		if !errors.As(err, &refusal) || !errors.Is(err, ErrVerification) || refusal.Check != want {
			t.Fatalf("want refusal %s, got %v (%+v)", want, err, refusal)
		}
		if verified.Rung != "" || len(verified.CanonicalBytes) != 0 {
			t.Fatal("refusal returned a verified receipt")
		}
	}
	for _, c := range cases {
		if c.Vector != "V3" {
			continue
		}
		count++
		t.Run(c.ID, func(t *testing.T) {
			want, ok := expected[c.ID]
			if !ok {
				t.Fatalf("missing refusal expectation for %s: %s", c.ID, c.Expected.Check)
			}
			rung := map[ReceiptCheck]string{ReceiptCheckKeyID: "R3", ReceiptCheckKeyHeight: "R3", ReceiptCheckRawTransaction: "R4", ReceiptCheckSequenceHash: "R6", ReceiptCheckLogs: "R5", ReceiptCheckCanonicalEncoding: "R2"}
			if c.Expected.RefusedAt != rung[want] || c.Expected.Check == "" {
				t.Fatalf("fixture expectation differs: %+v", c.Expected)
			}
			assertRefusal(t, c.ReceiptJSON, sequencerFixtureBytes(t, c.RawTransaction), sequencerFixtureKeys(t, c), want)
		})
	}
	if count != len(expected) {
		t.Fatalf("executed %d V3 cases, expected %d", count, len(expected))
	}
	c := cases[0]
	raw := sequencerFixtureBytes(t, c.RawTransaction)
	for _, mutation := range []struct {
		name, field, value string
		check              ReceiptCheck
	}{
		{"status", "status", "0x0", ReceiptCheckFields},
		{"nonce", "nonce", "0x1", ReceiptCheckFields},
		{"gas", "gasUsed", "0x5209", ReceiptCheckFields},
		{"return-data", "returnData", "0x01", ReceiptCheckReturnData},
		{"vm-error", "vmError", "untrusted reason", ReceiptCheckFields},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			var v map[string]json.RawMessage
			if err := json.Unmarshal(c.ReceiptJSON, &v); err != nil {
				t.Fatal(err)
			}
			v[mutation.field], _ = json.Marshal(mutation.value)
			data, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			assertRefusal(t, data, raw, sequencerFixtureKeys(t, c), mutation.check)
		})
	}
	t.Run("signature", func(t *testing.T) {
		var v sequencerReceiptJSON
		if err := json.Unmarshal(c.ReceiptJSON, &v); err != nil {
			t.Fatal(err)
		}
		signed := sequencerFixtureBytes(t, v.Receipt)
		signed[len(signed)-1] ^= 1
		v.Receipt = "0x" + hex.EncodeToString(signed)
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		assertRefusal(t, data, raw, sequencerFixtureKeys(t, c), ReceiptCheckSignature)
	})
	t.Run("trusted-predecessor", func(t *testing.T) {
		keys := sequencerFixtureKeys(t, c)
		keys.PreviousSequenceHash[0] ^= 1
		assertRefusal(t, c.ReceiptJSON, raw, keys, ReceiptCheckSequenceHash)
	})
}
