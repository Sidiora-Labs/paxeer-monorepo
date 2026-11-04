package eddsa

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/tss/dealershare"
	"github.com/getamis/alice/crypto/birkhoffinterpolation"
	pt "github.com/getamis/alice/crypto/ecpointgrouplaw"
	"github.com/getamis/alice/crypto/elliptic"
)

func repairM128Bundles(t *testing.T) []dealer.ShareBundle {
	t.Helper()
	secret, err := dealer.Ed25519ScalarFromSeed(bytes.Repeat([]byte{0x28}, ed25519.SeedSize))
	if err != nil {
		t.Fatal(err)
	}
	bundles, _, err := dealer.Split(dealer.Ed25519, secret, participantIDs())
	if err != nil {
		t.Fatal(err)
	}
	return bundles
}

func TestRepairM_128_MalformedImportedBundle(t *testing.T) {
	bundle := repairM128Bundles(t)[0]
	cases := []struct {
		name    string
		corrupt func(*dealer.ShareBundle)
	}{
		{"nil_public_key", func(b *dealer.ShareBundle) { b.PublicKey = nil }},
		{"nil_share", func(b *dealer.ShareBundle) { b.Share = nil }},
		{"nil_bks", func(b *dealer.ShareBundle) { b.Bks = nil }},
		{"nil_points", func(b *dealer.ShareBundle) { b.PartialPublicKeys = nil }},
		{"wrong_public_curve", func(b *dealer.ShareBundle) { b.PublicKey = pt.NewBase(elliptic.Secp256k1()) }},
		{"identity_public_key", func(b *dealer.ShareBundle) { b.PublicKey = pt.NewIdentity(elliptic.Ed25519()) }},
		{"empty_public_key", func(b *dealer.ShareBundle) { b.PublicKey = new(pt.ECPoint) }},
		{"mismatched_maps", func(b *dealer.ShareBundle) {
			b.PartialPublicKeys["unknown"] = b.PartialPublicKeys[b.ParticipantID]
			delete(b.PartialPublicKeys, b.ParticipantID)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := bundle.Clone()
			tc.corrupt(&b)
			got, err := NewKeyShare(b.ParticipantID, b.Threshold, b.PublicKey, b.Share, b.Bks, b.PartialPublicKeys)
			if got != nil || !errors.Is(err, ErrInconsistent) {
				t.Fatalf("malformed bundle returned share=%v error=%v; want nil, ErrInconsistent", got, err)
			}
		})
	}
	for _, pid := range bundle.ParticipantIDs() {
		for _, tc := range []struct {
			name    string
			corrupt func(*dealer.ShareBundle, string)
		}{
			{"nil_bk", func(b *dealer.ShareBundle, id string) { b.Bks[id] = nil }},
			{"nil_x", func(b *dealer.ShareBundle, id string) { b.Bks[id] = birkhoffinterpolation.NewBkParameter(nil, 0) }},
			{"nil_point", func(b *dealer.ShareBundle, id string) { b.PartialPublicKeys[id] = nil }},
			{"missing_bk", func(b *dealer.ShareBundle, id string) { delete(b.Bks, id) }},
			{"missing_point", func(b *dealer.ShareBundle, id string) { delete(b.PartialPublicKeys, id) }},
			{"wrong_curve", func(b *dealer.ShareBundle, id string) { b.PartialPublicKeys[id] = pt.NewBase(elliptic.Secp256k1()) }},
			{"identity_point", func(b *dealer.ShareBundle, id string) { b.PartialPublicKeys[id] = pt.NewIdentity(elliptic.Ed25519()) }},
			{"empty_point", func(b *dealer.ShareBundle, id string) { b.PartialPublicKeys[id] = new(pt.ECPoint) }},
		} {
			t.Run(pid+"/"+tc.name, func(t *testing.T) {
				b := bundle.Clone()
				tc.corrupt(&b, pid)
				got, err := NewKeyShare(b.ParticipantID, b.Threshold, b.PublicKey, b.Share, b.Bks, b.PartialPublicKeys)
				if got != nil || !errors.Is(err, ErrInconsistent) {
					t.Fatalf("malformed bundle returned share=%v error=%v; want nil, ErrInconsistent", got, err)
				}
			})
		}
	}
}

func TestRepairM_128_ImportedBundleRecovery(t *testing.T) {
	shares := make(map[string]*KeyShare)
	dir := t.TempDir()
	for _, b := range repairM128Bundles(t) {
		k, err := NewKeyShare(b.ParticipantID, b.Threshold, b.PublicKey, b.Share, b.Bks, b.PartialPublicKeys)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := k.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, b.ParticipantID+".json")
		if err := os.WriteFile(path, encoded, 0600); err != nil {
			t.Fatal(err)
		}
		k = nil
		saved, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		reopened, err := Load(saved)
		if err != nil {
			t.Fatal(err)
		}
		if reopened.ID() != b.ParticipantID || reopened.Threshold() != b.Threshold {
			t.Fatal("reopened share lost its participant or threshold")
		}
		publicKey, scalar, bks, points := reopened.Material()
		bk := bks[b.ParticipantID]
		bks[b.ParticipantID] = birkhoffinterpolation.NewBkParameter(nil, bk.GetRank())
		if got, err := NewKeyShare(b.ParticipantID, b.Threshold, publicKey, scalar, bks, points); got != nil || !errors.Is(err, ErrInconsistent) {
			t.Fatalf("malformed recovered bundle returned share=%v error=%v", got, err)
		}
		bks[b.ParticipantID] = bk
		recovered, err := NewKeyShare(b.ParticipantID, b.Threshold, publicKey, scalar, bks, points)
		if err != nil {
			t.Fatalf("valid re-import after malformed bundle: %v", err)
		}
		again, err := recovered.Marshal()
		if err != nil || !bytes.Equal(encoded, again) {
			t.Fatalf("recovery changed the saved share: %v", err)
		}
		shares[b.ParticipantID] = recovered
	}
	message := []byte("repair 22.33.128 recovered bundle signing")
	signature := runSign(t, shares, participantIDs()[:Threshold], message)
	publicKey := shares[participantIDs()[0]].PublicKeyBytes()
	if !ed25519.Verify(ed25519.PublicKey(publicKey[:]), message, signature[:]) {
		t.Fatal("recovered dealer shares produced an invalid signature")
	}
}

func TestRepairM_128_CopyIsolationAndMalformedEncoding(t *testing.T) {
	b := repairM128Bundles(t)[0]
	k, err := NewKeyShare(b.ParticipantID, b.Threshold, b.PublicKey, b.Share, b.Bks, b.PartialPublicKeys)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := k.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	b.Share.SetInt64(0)
	for id, bk := range b.Bks {
		bk.GetX().SetInt64(0)
		delete(b.Bks, id)
		delete(b.PartialPublicKeys, id)
	}
	_, scalar, bks, points := k.Material()
	scalar.SetInt64(0)
	for id, bk := range bks {
		bk.GetX().SetInt64(0)
		points[id] = nil
	}
	again, err := k.Marshal()
	if err != nil || !bytes.Equal(encoded, again) {
		t.Fatalf("import or exported material aliases the key share: %v", err)
	}
	for _, field := range []string{"x", "public_share"} {
		t.Run(field, func(t *testing.T) {
			var record keyShareV1
			if err := json.Unmarshal(encoded, &record); err != nil {
				t.Fatal(err)
			}
			if field == "x" {
				record.Parties[len(record.Parties)-1].X = nil
			} else {
				record.Parties[len(record.Parties)-1].PublicShare = nil
			}
			malformed, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			if got, err := Load(malformed); got != nil || !errors.Is(err, ErrEncoding) {
				t.Fatalf("malformed saved %s returned share=%v error=%v", field, got, err)
			}
			if got, err := Load(encoded); err != nil || got.PublicKeyBytes() != k.PublicKeyBytes() {
				t.Fatalf("valid load after malformed saved share: %v", err)
			}
		})
	}
}
