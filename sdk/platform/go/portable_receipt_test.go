package layerx

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"
)

func TestPortableReceiptRoundTripsAtReceipted(t *testing.T) {
	count := 0
	for _, c := range sequencerReceiptCases(t) {
		if c.Vector != "V1" {
			continue
		}
		count++
		t.Run(c.ID, func(t *testing.T) {
			keys := sequencerFixtureKeys(t, c)
			p, err := ExportPortableReceipt(c.ReceiptJSON, sequencerFixtureBytes(t, c.RawTransaction), keys, nil)
			if err != nil {
				t.Fatal(err)
			}
			if p.Format != "layerx-receipt-proof-v2" || p.VerificationLevel != "receipted" || p.RawTransaction != c.RawTransaction || p.SignedReceipt != c.SignedReceipt || p.ReceiptDigest != c.ReceiptDigest || p.ReceiptKey != portableKey(keys.Pins[0]) {
				t.Fatal("export lost receipt evidence")
			}
			data, err := p.Export()
			if err != nil {
				t.Fatal(err)
			}
			verified, err := VerifyPortableReceipt(data, keys)
			if err != nil {
				t.Fatal(err)
			}
			if verified.Rung != "receipted" || verified.Level != "receipted" || verified.Status != 1 || verified.Revert != nil || !bytes.Equal(verified.CanonicalBytes, sequencerFixtureBytes(t, c.SignedReceipt)) || !bytes.Equal(verified.ReceiptDigest[:], sequencerFixtureBytes(t, c.ReceiptDigest)) {
				t.Fatal("portable verification lost authenticated receipt")
			}
			var restored PortableReceipt
			if err := json.Unmarshal(data, &restored); err != nil {
				t.Fatal(err)
			}
			reexported, err := restored.Export()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(data, reexported) {
				t.Fatal("portable receipt did not re-export byte-identically")
			}
			if _, err := restored.Verify(keys); err != nil {
				t.Fatal(err)
			}
			rebuilt, err := ExportPortableReceipt(restored.ReceiptJSON, sequencerFixtureBytes(t, restored.RawTransaction), keys, nil)
			if err != nil {
				t.Fatal(err)
			}
			rebuiltBytes, err := rebuilt.Export()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(data, rebuiltBytes) {
				t.Fatal("verified receipt did not export identically")
			}
		})
	}
	if count != 5 {
		t.Fatalf("executed %d V1 cases, want 5", count)
	}
	t.Run("committed", func(t *testing.T) {
		receipt, raw, keys, evidence, _ := repair141LoadRecorded(t)
		trust := portableRecordedTrust(t)
		signed, err := VerifySequencerReceipt(receipt, raw, keys)
		if err != nil {
			t.Fatal(err)
		}
		p, err := ExportPortableReceipt(receipt, raw, keys, &evidence, trust)
		if err != nil {
			t.Fatal(err)
		}
		data, err := p.Export()
		if err != nil {
			t.Fatal(err)
		}
		verified, err := VerifyPortableReceipt(data, keys, trust)
		if err != nil {
			t.Fatal(err)
		}
		if verified.Rung != "committed" || verified.Level != "committed" || verified.ReceiptDigest != signed.ReceiptDigest {
			t.Fatal("committed evidence was not verified")
		}
		var restored PortableReceipt
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatal(err)
		}
		again, err := restored.Export()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, again) {
			t.Fatal("committed bundle changed on re-export")
		}
		restored.Commit.Signatures[0].Signature[0] ^= 1
		if result, err := restored.Verify(keys, trust); !errors.Is(err, ErrVerification) || result.Rung != "" {
			t.Fatal("tampered commit was accepted")
		}
	})
}

func TestPortableReceiptRefusesDifferentWrappedTransaction(t *testing.T) {
	cases := sequencerReceiptCases(t)
	c := cases[0]
	keys := sequencerFixtureKeys(t, c)
	f := newCommittedFixture(t, c.ID, [4]int64{1, 1, 1, 1}, 0)
	evidence := PortableCommittedEvidence{BlockHeader: f.header, ReceiptInclusion: f.inclusion, Commit: f.commit, ValidatorSet: f.validators}
	p, err := ExportPortableReceipt(c.ReceiptJSON, sequencerFixtureBytes(t, c.RawTransaction), keys, &evidence, f.trust)
	if err != nil {
		t.Fatal(err)
	}
	different := sequencerFixtureBytes(t, cases[1].RawTransaction)
	if bytes.Equal(different, sequencerFixtureBytes(t, c.RawTransaction)) {
		t.Fatal("mutation must use a different signed EVM transaction")
	}
	f.inclusion.Transaction = committedFixtureWrappedTransaction(different)
	f.reanchor(t)
	p.BlockHeader, p.ReceiptInclusion, p.Commit = &f.header, &f.inclusion, &f.commit
	data, err := p.Export()
	if err != nil {
		t.Fatal(err)
	}
	result, err := VerifyPortableReceipt(data, keys)
	var verification *VerificationError
	if !errors.As(err, &verification) || verification.Check != ReceiptCheckRawTransaction || !errors.Is(err, ErrVerification) || result.Rung != "" || len(result.CanonicalBytes) != 0 {
		t.Fatalf("expected raw-transaction refusal, got %+v, %v", result, err)
	}
}

func TestPortableReceiptRefusesVersionOneFormat(t *testing.T) {
	c := sequencerReceiptCases(t)[0]
	keys := sequencerFixtureKeys(t, c)
	p, err := ExportPortableReceipt(c.ReceiptJSON, sequencerFixtureBytes(t, c.RawTransaction), keys, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := p.Export()
	if err != nil {
		t.Fatal(err)
	}
	refuse := func(t *testing.T, data []byte, pins ReceiptKeys) {
		t.Helper()
		result, err := VerifyPortableReceipt(data, pins)
		if !errors.Is(err, ErrVerification) || result.Rung != "" || len(result.CanonicalBytes) != 0 {
			t.Fatalf("expected verification refusal, got %+v, %v", result, err)
		}
	}
	mutate := func(t *testing.T, change func(map[string]json.RawMessage)) []byte {
		t.Helper()
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err != nil {
			t.Fatal(err)
		}
		change(fields)
		out, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	t.Run("v1", func(t *testing.T) {
		refuse(t, mutate(t, func(fields map[string]json.RawMessage) {
			fields["format"] = json.RawMessage(`"layerx-receipt-proof-v1"`)
		}), keys)
	})
	for _, field := range []string{"batchId", "asset", "previousStateRoot", "resultingStateRoot"} {
		for _, location := range []string{"bundle", "receiptKey", "receiptJson"} {
			t.Run(field+"/"+location, func(t *testing.T) {
				refuse(t, mutate(t, func(fields map[string]json.RawMessage) {
					if location == "bundle" {
						fields[field] = json.RawMessage(`null`)
						return
					}
					var nested map[string]json.RawMessage
					if err := json.Unmarshal(fields[location], &nested); err != nil {
						t.Fatal(err)
					}
					nested[field] = json.RawMessage(`null`)
					encoded, err := json.Marshal(nested)
					if err != nil {
						t.Fatal(err)
					}
					fields[location] = encoded
				}), keys)
			})
		}
	}
	for _, mutation := range []string{"transaction", "signature", "digest", "key", "key-range", "receipt-view", "committed-without-evidence", "orphan-commit", "missing-transaction", "unknown-field"} {
		t.Run(mutation, func(t *testing.T) {
			refuse(t, mutate(t, func(fields map[string]json.RawMessage) {
				switch mutation {
				case "transaction":
					fields["rawTransaction"] = json.RawMessage(`"0x01"`)
				case "signature":
					var view sequencerReceiptJSON
					if err := json.Unmarshal(fields["receiptJson"], &view); err != nil {
						t.Fatal(err)
					}
					signed := sequencerFixtureBytes(t, c.SignedReceipt)
					signed[len(signed)-1] ^= 1
					view.Receipt = portableHex(signed)
					fields["signedReceipt"], err = json.Marshal(view.Receipt)
					if err != nil {
						t.Fatal(err)
					}
					fields["receiptJson"], err = json.Marshal(view)
					if err != nil {
						t.Fatal(err)
					}
				case "digest":
					fields["receiptDigest"], err = json.Marshal(portableHex(make([]byte, 32)))
				case "key", "key-range":
					key := p.ReceiptKey
					if mutation == "key" {
						key.PublicKey = portableHex(make([]byte, 32))
					} else {
						key.LastHeight = "0xffff"
					}
					fields["receiptKey"], err = json.Marshal(key)
				case "receipt-view":
					var view sequencerReceiptJSON
					if err := json.Unmarshal(fields["receiptJson"], &view); err != nil {
						t.Fatal(err)
					}
					view.GasUsed = "0x1"
					fields["receiptJson"], err = json.Marshal(view)
				case "committed-without-evidence":
					fields["verificationLevel"] = json.RawMessage(`"committed"`)
				case "orphan-commit":
					fields["commit"] = json.RawMessage(`null`)
				case "missing-transaction":
					delete(fields, "rawTransaction")
				case "unknown-field":
					fields["unexpected"] = json.RawMessage(`true`)
				}
				if err != nil {
					t.Fatal(err)
				}
			}), keys)
		})
	}
	t.Run("unpinned", func(t *testing.T) { refuse(t, data, ReceiptKeys{PreviousSequenceHash: keys.PreviousSequenceHash}) })
	t.Run("predecessor", func(t *testing.T) { changed := keys; changed.PreviousSequenceHash[0] ^= 1; refuse(t, data, changed) })
	t.Run("duplicate-field", func(t *testing.T) {
		refuse(t, append([]byte(`{"format":"layerx-receipt-proof-v2",`), data[1:]...), keys)
	})
	t.Run("trailing-document", func(t *testing.T) { refuse(t, append(append([]byte(nil), data...), []byte(` {}`)...), keys) })
}

// This checkpoint is an out-of-band fixture pin. Never obtain it from the
// portable bundle or compute the trusted hash from its untrusted header.
func portableRecordedTrust(t *testing.T) CommittedTrust {
	t.Helper()
	data, err := os.ReadFile("testdata/repair_m1384/checkpoint.json")
	if err != nil {
		t.Fatal(err)
	}
	var checkpoint struct {
		ChainID   string `json:"chain_id"`
		Height    int64  `json:"height"`
		BlockHash string `json:"block_hash"`
	}
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		t.Fatal(err)
	}
	hash, err := hex.DecodeString(checkpoint.BlockHash)
	if err != nil || len(hash) != 32 {
		t.Fatal("invalid independent checkpoint")
	}
	trust := CommittedTrust{ChainID: checkpoint.ChainID, Height: checkpoint.Height}
	copy(trust.BlockHash[:], hash)
	return trust
}

func TestPortableReceiptRequiresExternalNetworkAnchor(t *testing.T) {
	receipt, raw, keys, evidence, _ := repair141LoadRecorded(t)
	trust := portableRecordedTrust(t)
	bundle, err := ExportPortableReceipt(receipt, raw, keys, &evidence, trust)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := bundle.Export()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := VerifyPortableReceipt(encoded, keys, trust); err != nil || got.Level != "committed" {
		t.Fatalf("independently pinned producer fixture: %+v %v", got, err)
	}
	for _, anchors := range [][]CommittedTrust{nil, {{ChainID: "attacker-network", Height: trust.Height, BlockHash: trust.BlockHash}}, {{ChainID: trust.ChainID, Height: trust.Height + 1, BlockHash: trust.BlockHash}}} {
		if got, err := VerifyPortableReceipt(encoded, keys, anchors...); !errors.Is(err, ErrVerification) || got.Level != "" {
			t.Fatalf("untrusted committed bundle: %+v %v", got, err)
		}
	}
	// Keep the genuine signed transaction receipt and inclusion proofs, replacing
	// every consensus key, header and commit coherently with an attacker's chain.
	var attackerKeys []ed25519.PrivateKey
	var attackers []CometValidator
	for n := range evidence.ValidatorSet {
		key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{byte(0x80 + n)}, 32))
		attackerKeys = append(attackerKeys, key)
		var public [32]byte
		copy(public[:], key.Public().(ed25519.PublicKey))
		attackers = append(attackers, CometValidator{PublicKey: public, VotingPower: evidence.ValidatorSet[n].VotingPower})
	}
	hash := committedValidatorsHash(attackers)
	evidence.BlockHeader.ValidatorsHash, evidence.BlockHeader.NextValidatorsHash = hash[:], hash[:]
	evidence.ValidatorSet = attackers
	evidence.Commit = committedFixtureCommit(evidence.BlockHeader, attackerKeys, len(attackerKeys))
	evidence.ReceiptInclusion.NextHeader.ValidatorsHash, evidence.ReceiptInclusion.NextHeader.NextValidatorsHash = hash[:], hash[:]
	evidence.ReceiptInclusion.NextHeader.LastBlockID = evidence.Commit.BlockID
	evidence.ReceiptInclusion.NextValidators = attackers
	evidence.ReceiptInclusion.NextCommit = committedFixtureCommit(evidence.ReceiptInclusion.NextHeader, attackerKeys, len(attackerKeys))
	// The unauthenticated transport coordinates can be replaced too.
	var view sequencerReceiptJSON
	if err := json.Unmarshal(receipt, &view); err != nil {
		t.Fatal(err)
	}
	blockHash := evidence.BlockHeader.Hash()
	changedHash := portableHex(blockHash[:])
	view.BlockHash = &changedHash
	changedReceipt, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	forgedTrust := CommittedTrust{ChainID: evidence.BlockHeader.ChainID, Height: evidence.BlockHeader.Height, BlockHash: blockHash}
	forged, err := ExportPortableReceipt(changedReceipt, raw, keys, &evidence, forgedTrust)
	if err != nil {
		t.Fatalf("attack must be internally consistent: %v", err)
	}
	data, err := forged.Export()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := VerifyPortableReceipt(data, keys, forgedTrust); err != nil || got.Level != "committed" {
		t.Fatalf("replacement chain consistency control: %+v %v", got, err)
	}
	if got, err := VerifyPortableReceipt(data, keys, trust); !errors.Is(err, ErrVerification) || got.Rung != "" || got.Level != "" {
		t.Fatalf("replacement validator chain established network finality: %+v %v", got, err)
	}
	if _, err := ExportPortableReceipt(changedReceipt, raw, keys, &evidence, trust); !errors.Is(err, ErrVerification) {
		t.Fatalf("export accepted replacement validator history: %v", err)
	}
}

func TestPortableReceiptAuthenticatesValidatorRotation(t *testing.T) {
	c := sequencerReceiptCases(t)[0]
	keys := sequencerFixtureKeys(t, c)
	f := newCommittedFixture(t, c.ID, [4]int64{1, 1, 1, 1}, 0)
	var oldKeys []ed25519.PrivateKey
	var oldValidators []CometValidator
	for n := 0; n < 4; n++ {
		key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{byte(0x50 + n)}, 32))
		oldKeys = append(oldKeys, key)
		var public [32]byte
		copy(public[:], key.Public().(ed25519.PublicKey))
		oldValidators = append(oldValidators, CometValidator{PublicKey: public, VotingPower: 1})
	}
	previous := f.header
	previous.Height--
	previous.Time = previous.Time.Add(-time.Second)
	oldHash := committedValidatorsHash(oldValidators)
	previous.ValidatorsHash = oldHash[:]
	previousCommit := committedFixtureCommit(previous, oldKeys, 3)
	// Establish the independently selected checkpoint before constructing the
	// target proof. A validator change must follow its NextValidatorsHash.
	trust := CommittedTrust{ChainID: previous.ChainID, Height: previous.Height, BlockHash: previous.Hash()}
	f.header.LastBlockID = previousCommit.BlockID
	f.commit = committedFixtureCommit(f.header, f.keys, 3)
	f.inclusion.NextHeader.LastBlockID = f.commit.BlockID
	f.inclusion.NextCommit = committedFixtureCommit(f.inclusion.NextHeader, f.keys, 3)
	f.inclusion.ValidatorHistory = []CommittedLightBlock{{Header: previous, Commit: previousCommit, Validators: oldValidators}}
	evidence := PortableCommittedEvidence{BlockHeader: f.header, ReceiptInclusion: f.inclusion, Commit: f.commit, ValidatorSet: f.validators}
	bundle, err := ExportPortableReceipt(c.ReceiptJSON, sequencerFixtureBytes(t, c.RawTransaction), keys, &evidence, trust)
	if err != nil {
		t.Fatal(err)
	}
	data, err := bundle.Export()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := VerifyPortableReceipt(data, keys, trust); err != nil || got.Level != "committed" {
		t.Fatalf("valid validator rotation: %+v %v", got, err)
	}
	for _, mutation := range []string{"missing-history", "old-signature", "new-signature", "transition", "network"} {
		t.Run(mutation, func(t *testing.T) {
			var changed PortableReceipt
			if err := json.Unmarshal(data, &changed); err != nil {
				t.Fatal(err)
			}
			history := changed.ReceiptInclusion.ValidatorHistory
			switch mutation {
			case "missing-history":
				changed.ReceiptInclusion.ValidatorHistory = nil
			case "old-signature":
				history[0].Commit.Signatures[0].Signature[0] ^= 1
			case "new-signature":
				changed.Commit.Signatures[0].Signature[0] ^= 1
			case "transition":
				history[0].Header.NextValidatorsHash = bytes.Repeat([]byte{0x99}, 32)
				history[0].Commit = committedFixtureCommit(history[0].Header, oldKeys, 3)
			case "network":
				history[0].Header.ChainID = "private-network"
				history[0].Commit = committedFixtureCommit(history[0].Header, oldKeys, 3)
			}
			encoded, err := changed.Export()
			if err != nil {
				t.Fatal(err)
			}
			if got, err := VerifyPortableReceipt(encoded, keys, trust); !errors.Is(err, ErrVerification) || got.Level != "" {
				t.Fatalf("untrusted rotation accepted: %+v %v", got, err)
			}
		})
	}
}
