package layerx_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	sdk "github.com/Sidiora-Labs/LayerX-Network/sdk/platform/go"
)

// These entry points keep the conformance suite on the published Go verifier API.
func VerifySequencerReceipt(receiptJSON, rawTransaction []byte, keys sdk.ReceiptKeys) (sdk.SequencerReceiptVerification, error) {
	return sdk.VerifySequencerReceipt(receiptJSON, rawTransaction, keys)
}

func VerifyCommittedReceipt(receipt sdk.SequencerReceiptVerification, header sdk.CommittedHeader, inclusion sdk.ReceiptInclusion, commit sdk.CometCommit, validators []sdk.CometValidator) (sdk.Committed, error) {
	return sdk.VerifyCommittedReceipt(receipt, header, inclusion, commit, validators)
}

type conformanceReceiptCase struct {
	ID             string          `json:"id"`
	Vector         string          `json:"vector"`
	RawTransaction string          `json:"rawTransaction"`
	ReceiptJSON    json.RawMessage `json:"receiptJson"`
	SignedReceipt  string          `json:"signedReceipt"`
	ReceiptDigest  string          `json:"receiptDigest"`
	PrevSeqHash    string          `json:"prevSeqHash"`
	PinnedKey      struct {
		KeyID       string `json:"keyId"`
		PublicKey   string `json:"publicKey"`
		FirstHeight uint64 `json:"firstHeight"`
		LastHeight  uint64 `json:"lastHeight"`
	} `json:"pinnedKey"`
	LXPCode  string `json:"lxpCode"`
	Expected struct {
		Rung      string `json:"rung"`
		RefusedAt string `json:"refusedAt"`
		Check     string `json:"check"`
	} `json:"expected"`
}

type conformanceCommittedCase struct {
	conformanceReceiptCase
	BlockHeader      sdk.CommittedHeader  `json:"blockHeader"`
	ReceiptInclusion sdk.ReceiptInclusion `json:"receiptInclusion"`
	Commit           sdk.CometCommit      `json:"commit"`
	ValidatorSet     []sdk.CometValidator `json:"validatorSet"`
}

type conformanceCommittedVector struct {
	Name     string                   `json:"name"`
	Positive conformanceCommittedCase `json:"positive"`
	Refusals []struct {
		ID            string           `json:"id"`
		NextCommit    sdk.CometCommit  `json:"nextCommit"`
		ExpectedCheck sdk.ReceiptCheck `json:"expectedCheck"`
	} `json:"refusals"`
}

func readConformanceVector(t *testing.T, name string, value any) {
	t.Helper()
	data, err := os.ReadFile("../conformance/fixtures/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatal(err)
	}
}

func conformanceHex(t *testing.T, value string, size int) []byte {
	t.Helper()
	if !strings.HasPrefix(value, "0x") {
		t.Fatalf("missing hex prefix: %q", value)
	}
	data, err := hex.DecodeString(value[2:])
	if err != nil {
		t.Fatal(err)
	}
	if size > 0 && len(data) != size {
		t.Fatalf("want %d bytes, got %d", size, len(data))
	}
	return data
}

func conformanceKeys(t *testing.T, c conformanceReceiptCase) sdk.ReceiptKeys {
	t.Helper()
	var key sdk.ReceiptKey
	copy(key.KeyID[:], conformanceHex(t, c.PinnedKey.KeyID, 32))
	copy(key.PublicKey[:], conformanceHex(t, c.PinnedKey.PublicKey, 32))
	key.FirstHeight, key.LastHeight = c.PinnedKey.FirstHeight, c.PinnedKey.LastHeight
	keys := sdk.ReceiptKeys{Pins: []sdk.ReceiptKey{key}}
	copy(keys.PreviousSequenceHash[:], conformanceHex(t, c.PrevSeqHash, 32))
	return keys
}

func assertConformanceReceipt(t *testing.T, c conformanceReceiptCase, result sdk.SequencerReceiptVerification, level string) {
	t.Helper()
	if result.Rung != level || result.Level != level {
		t.Fatalf("want rung and level %s, got %s and %s", level, result.Rung, result.Level)
	}
	if !bytes.Equal(result.CanonicalBytes, conformanceHex(t, c.SignedReceipt, 0)) ||
		!bytes.Equal(result.ReceiptDigest[:], conformanceHex(t, c.ReceiptDigest, 32)) {
		t.Fatal("verified signed receipt or digest differs from vector")
	}
}

func conformanceFailureCheck(err error) sdk.ReceiptCheck {
	var failure *sdk.VerificationError
	if errors.As(err, &failure) {
		return failure.Check
	}
	return ""
}

func assertConformanceBlockTransaction(t *testing.T, c conformanceReceiptCase, inclusion sdk.ReceiptInclusion) {
	t.Helper()
	data, err := os.ReadFile("../../../daemon/committed-fixture/vectors/vector-v4-committed.json")
	if err != nil {
		t.Fatal(err)
	}
	var source struct {
		Settlement struct {
			RawTransaction   string `json:"raw_transaction"`
			TransactionProof struct {
				Data []byte `json:"data"`
			} `json:"transaction_proof"`
		} `json:"settlement"`
	}
	if err := json.Unmarshal(data, &source); err != nil {
		t.Fatal(err)
	}
	if c.RawTransaction != source.Settlement.RawTransaction ||
		len(source.Settlement.TransactionProof.Data) == 0 ||
		!bytes.Equal(inclusion.Transaction, source.Settlement.TransactionProof.Data) {
		t.Fatal("committed inclusion must use the recorded block transaction and retain the raw EVM transaction separately")
	}
	if bytes.Equal(inclusion.Transaction, conformanceHex(t, c.RawTransaction, 0)) {
		t.Fatal("recorded Cosmos block transaction must differ from its wrapped EVM transaction")
	}
}

func TestVectorFilesV1ToV3MatchSequencerReceiptFixture(t *testing.T) {
	var source struct {
		Cases []json.RawMessage `json:"cases"`
	}
	readConformanceVector(t, "sequencer-receipt-v1", &source)
	if len(source.Cases) != 11 {
		t.Fatalf("want 11 source cases, got %d", len(source.Cases))
	}
	refusals := map[string]struct {
		check sdk.ReceiptCheck
		rung  string
	}{
		"V3-key-not-pinned":           {sdk.ReceiptCheckKeyID, "R3"},
		"V3-height-outside-key-range": {sdk.ReceiptCheckKeyHeight, "R3"},
		"V3-activity-id-not-raw-hash": {sdk.ReceiptCheckRawTransaction, "R4"},
		"V3-seq-hash-mismatch":        {sdk.ReceiptCheckSequenceHash, "R6"},
		"V3-tampered-log":             {sdk.ReceiptCheckLogs, "R5"},
		"V3-non-canonical-bytes":      {sdk.ReceiptCheckCanonicalEncoding, "R2"},
	}
	for _, group := range []struct {
		name, section string
		count         int
	}{
		{"vector-v1-receipted-positive", "V1", 4},
		{"vector-v2-receipted-revert", "V2", 1},
		{"vector-v3-receipt-refusals", "V3", 6},
	} {
		t.Run(group.section, func(t *testing.T) {
			want := make(map[string]any)
			for _, raw := range source.Cases {
				var c conformanceReceiptCase
				if err := json.Unmarshal(raw, &c); err != nil {
					t.Fatal(err)
				}
				if c.Vector == group.section {
					var document any
					if err := json.Unmarshal(raw, &document); err != nil {
						t.Fatal(err)
					}
					if _, duplicate := want[c.ID]; duplicate {
						t.Fatalf("duplicate source case %s", c.ID)
					}
					want[c.ID] = document
				}
			}
			var vector struct {
				Name  string            `json:"name"`
				Cases []json.RawMessage `json:"cases"`
			}
			readConformanceVector(t, group.name, &vector)
			if vector.Name != group.name || len(vector.Cases) != group.count || len(want) != group.count {
				t.Fatalf("wrong vector name or case count: %s, %d published, %d source", vector.Name, len(vector.Cases), len(want))
			}
			seen := make(map[string]bool)
			for _, raw := range vector.Cases {
				var c conformanceReceiptCase
				if err := json.Unmarshal(raw, &c); err != nil {
					t.Fatal(err)
				}
				t.Run(c.ID, func(t *testing.T) {
					var document any
					if err := json.Unmarshal(raw, &document); err != nil {
						t.Fatal(err)
					}
					if seen[c.ID] || !reflect.DeepEqual(document, want[c.ID]) {
						t.Fatal("duplicate case or case differs from sequencer source")
					}
					seen[c.ID] = true
					result, err := VerifySequencerReceipt(c.ReceiptJSON, conformanceHex(t, c.RawTransaction, 0), conformanceKeys(t, c))
					if group.section == "V3" {
						want, ok := refusals[c.ID]
						if !ok || c.Expected.RefusedAt != want.rung || c.Expected.Check == "" {
							t.Fatalf("missing or incorrect refusal expectation for %s", c.ID)
						}
						var refusal *sdk.VerificationError
						if !errors.Is(err, sdk.ErrVerification) || !errors.As(err, &refusal) || refusal.Check != want.check {
							t.Fatalf("want refusal %s, got %v", want.check, err)
						}
						if result.Rung != "" || len(result.CanonicalBytes) != 0 {
							t.Fatal("refusal returned verified evidence")
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					assertConformanceReceipt(t, c, result, c.Expected.Rung)
					if group.section == "V1" {
						if result.Status != 1 || result.Revert != nil {
							t.Fatal("positive receipt was not successful")
						}
					} else if result.Status != 0 || result.Revert == nil || result.Revert.Code != c.LXPCode ||
						result.Revert.Reason != "LXP_INSUFFICIENT_BALANCE: insufficient balance" || !bytes.Equal(result.Revert.ReturnData, result.ReturnData) {
						t.Fatalf("revert lost its authenticated reason: %+v", result.Revert)
					}
				})
			}
		})
	}
}

func TestVectorV4CommittedVerifiesAndRefuses(t *testing.T) {
	var vector conformanceCommittedVector
	readConformanceVector(t, "vector-v4-committed", &vector)
	if vector.Name != "vector-v4-committed" || len(vector.Refusals) != 1 || vector.Refusals[0].ID != "insufficient-power" {
		t.Fatal("missing V4 identity or insufficient-power refusal")
	}
	c := vector.Positive
	if c.Expected.Rung != "committed" {
		t.Fatal("V4 positive must require the committed rung")
	}
	if c.Commit.Height != c.BlockHeader.Height || c.ReceiptInclusion.NextHeader.Height != c.BlockHeader.Height+1 || c.ReceiptInclusion.NextCommit.Height != c.ReceiptInclusion.NextHeader.Height {
		t.Fatal("V4 must supply separate execution and successor commits at their header heights")
	}
	assertConformanceBlockTransaction(t, c.conformanceReceiptCase, c.ReceiptInclusion)
	receipt, err := VerifySequencerReceipt(c.ReceiptJSON, conformanceHex(t, c.RawTransaction, 0), conformanceKeys(t, c.conformanceReceiptCase))
	if err != nil {
		t.Fatalf("V4 signed receipt: %v", err)
	}
	assertConformanceReceipt(t, c.conformanceReceiptCase, receipt, "receipted")
	t.Run("positive", func(t *testing.T) {
		result, err := VerifyCommittedReceipt(receipt, c.BlockHeader, c.ReceiptInclusion, c.Commit, c.ValidatorSet)
		if err != nil {
			t.Fatalf("recorded V4 positive must verify at committed (header height %d, supplied commit height %d, check %s): %v", c.BlockHeader.Height, c.Commit.Height, conformanceFailureCheck(err), err)
		}
		assertConformanceReceipt(t, c.conformanceReceiptCase, result.SequencerReceiptVerification, c.Expected.Rung)
		if result.Height != uint64(c.BlockHeader.Height) || result.BlockHash != c.BlockHeader.Hash() || result.VotingPower <= 0 || result.Status != 1 {
			t.Fatal("committed result lost its block identity, voting power or successful status")
		}
	})
	for _, refusal := range vector.Refusals {
		t.Run(refusal.ID, func(t *testing.T) {
			commit := refusal.NextCommit
			validators := c.ReceiptInclusion.NextValidators
			nextCommit := c.ReceiptInclusion.NextCommit
			if commit.Height != nextCommit.Height || commit.Round != nextCommit.Round || !reflect.DeepEqual(commit.BlockID, nextCommit.BlockID) || len(commit.Signatures) != len(validators) {
				t.Fatal("insufficient-power refusal changed the commit identity")
			}
			var total, signed int64
			for i, validator := range validators {
				if validator.VotingPower <= 0 {
					t.Fatal("nonpositive validator power")
				}
				total += validator.VotingPower
				sig := commit.Signatures[i]
				if sig.BlockIDFlag == sdk.CometBlock {
					if !reflect.DeepEqual(sig, nextCommit.Signatures[i]) {
						t.Fatal("refusal introduced a different signature")
					}
					signed += validator.VotingPower
				} else if sig.BlockIDFlag != sdk.CometAbsent || len(sig.Signature) != 0 || len(sig.ValidatorAddress) != 0 || !sig.Timestamp.IsZero() {
					t.Fatal("refusal must retain a vote or mark it absent")
				}
			}
			if total == 0 || signed*3 > total*2 {
				t.Fatal("refusal is not at or below two thirds power")
			}
			inclusion := c.ReceiptInclusion
			inclusion.NextCommit = commit
			result, err := VerifyCommittedReceipt(receipt, c.BlockHeader, inclusion, c.Commit, c.ValidatorSet)
			var verification *sdk.VerificationError
			if !errors.Is(err, sdk.ErrVerification) || !errors.As(err, &verification) || verification.Check != refusal.ExpectedCheck || result.Rung != "" || len(result.CanonicalBytes) != 0 {
				t.Fatalf("want %s refusal without verified evidence, got %+v, %v", refusal.ExpectedCheck, result, err)
			}
		})
	}
}

func TestVectorV5PortableRoundTrips(t *testing.T) {
	var vector struct {
		Name    string            `json:"name"`
		Bundles []json.RawMessage `json:"bundles"`
	}
	readConformanceVector(t, "vector-v5-portable", &vector)
	if vector.Name != "vector-v5-portable" || len(vector.Bundles) != 2 {
		t.Fatal("V5 must contain exactly one receipted and one committed bundle")
	}
	var v1 struct {
		Cases []conformanceReceiptCase `json:"cases"`
	}
	readConformanceVector(t, "vector-v1-receipted-positive", &v1)
	var v4 conformanceCommittedVector
	readConformanceVector(t, "vector-v4-committed", &v4)
	if len(v1.Cases) != 4 {
		t.Fatal("V1 source cases missing")
	}
	for i, level := range []string{"receipted", "committed"} {
		t.Run(level, func(t *testing.T) {
			c := v1.Cases[0]
			var evidence *sdk.PortableCommittedEvidence
			if level == "committed" {
				c = v4.Positive.conformanceReceiptCase
				evidence = &sdk.PortableCommittedEvidence{BlockHeader: v4.Positive.BlockHeader, ReceiptInclusion: v4.Positive.ReceiptInclusion, Commit: v4.Positive.Commit, ValidatorSet: v4.Positive.ValidatorSet}
			}
			keys := conformanceKeys(t, c)
			data := vector.Bundles[i]
			var bundle sdk.PortableReceipt
			if err := json.Unmarshal(data, &bundle); err != nil {
				t.Fatal(err)
			}
			if bundle.VerificationLevel != level || bundle.Format != sdk.PortableReceiptFormat || bundle.RawTransaction != c.RawTransaction || bundle.SignedReceipt != c.SignedReceipt || bundle.ReceiptDigest != c.ReceiptDigest {
				t.Fatal("portable bundle differs from source evidence or stated level")
			}
			reexported, err := bundle.Export()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(data, reexported) {
				t.Fatal("published portable bundle did not re-export byte-identically")
			}
			if level == "committed" {
				t.Run("committed-evidence", func(t *testing.T) {
					assertConformanceBlockTransaction(t, c, *bundle.ReceiptInclusion)
					receipt, err := VerifySequencerReceipt(c.ReceiptJSON, conformanceHex(t, c.RawTransaction, 0), keys)
					if err != nil {
						t.Fatalf("V5 signed receipt: %v", err)
					}
					if !reflect.DeepEqual(bundle.BlockHeader, &evidence.BlockHeader) ||
						!reflect.DeepEqual(bundle.ReceiptInclusion, &evidence.ReceiptInclusion) ||
						!reflect.DeepEqual(bundle.Commit, &evidence.Commit) ||
						!reflect.DeepEqual(bundle.ValidatorSet, evidence.ValidatorSet) {
						t.Fatal("portable committed evidence differs from V4 execution and successor evidence")
					}
					committed, err := VerifyCommittedReceipt(receipt, *bundle.BlockHeader, *bundle.ReceiptInclusion, *bundle.Commit, bundle.ValidatorSet)
					if err != nil {
						t.Fatalf("V5 committed evidence (check %s): %v", conformanceFailureCheck(err), err)
					}
					assertConformanceReceipt(t, c, committed.SequencerReceiptVerification, level)
				})
			}
			result, err := sdk.VerifyPortableReceipt(data, keys)
			if err != nil {
				t.Fatalf("portable %s verification (check %s): %v", level, conformanceFailureCheck(err), err)
			}
			assertConformanceReceipt(t, c, result, level)
			rebuilt, err := sdk.ExportPortableReceipt(c.ReceiptJSON, conformanceHex(t, c.RawTransaction, 0), keys, evidence)
			if err != nil {
				t.Fatal(err)
			}
			rebuiltBytes, err := rebuilt.Export()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(data, rebuiltBytes) {
				t.Fatal("exporting the original evidence changed the portable bundle")
			}
		})
	}
}
