package account

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
)

func TestLegacyProjectionValidatesProfileFieldsAndImages(t *testing.T) {
	const wallet = "0x1111111111111111111111111111111111111111"
	base := `{"display_name":" Ada ","bio":"builder","website":"https://example.org","twitter":"https://x.com/ada","telegram":null,"discord":""}`
	profile, err := legacyProfileProjection(context.Background(), wallet, json.RawMessage(base), nil)
	if err != nil || profile.Wallet != wallet || profile.DisplayName != "Ada" || profile.Bio != "builder" || profile.Website == nil || *profile.Website != "https://example.org" || profile.Socials["twitter"] != "https://x.com/ada" || len(profile.Socials) != 1 {
		t.Fatalf("legacy profile = %+v, %v", profile, err)
	}
	assetID := uuid.NewString()
	for _, tc := range []struct {
		name   string
		input  string
		images []json.RawMessage
		want   error
	}{
		{"invalid JSON", `{`, nil, nil},
		{"invalid display name", `{"display_name":1,"bio":"builder"}`, nil, nil},
		{"invalid website", `{"display_name":"Ada","bio":"builder","website":"javascript:alert(1)","twitter":null,"telegram":null,"discord":null}`, nil, ErrInvalid},
		{"invalid social value", `{"display_name":"Ada","bio":"builder","website":null,"twitter":123,"telegram":null,"discord":null}`, nil, nil},
		{"invalid social link", `{"display_name":"Ada","bio":"builder","website":null,"twitter":"http://example.org","telegram":null,"discord":null}`, nil, ErrInvalid},
		{"malformed image", base, []json.RawMessage{json.RawMessage(`{`)}, nil},
		{"invalid image ID", base, []json.RawMessage{json.RawMessage(`{"id":"bad","image_type":"avatar"}`)}, ErrInvalid},
		{"missing asset authority", base, []json.RawMessage{json.RawMessage(`{"id":"` + assetID + `","image_type":"avatar"}`)}, ErrState},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := legacyProfileProjection(context.Background(), wallet, json.RawMessage(tc.input), tc.images)
			if err == nil || (tc.want != nil && !errors.Is(err, tc.want)) {
				t.Fatalf("projection error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestLegacyRangeQuarantinesConflictingStableOwner(t *testing.T) {
	s := connectedLegacyFixture(t)
	ctx := context.Background()
	p := owner(t, s, 9661)
	legacyWallet := fmt.Sprintf("0x%040x", 9662)
	seedConnectedIdentity(t, s, p, uuid.New(), legacyWallet)
	if _, err := s.DB.Exec(ctx, `INSERT INTO users.user_profiles(wallet_address,display_name,bio,created_at,updated_at) VALUES($1,'Legacy owner','profile',1,1)`, legacyWallet); err != nil {
		t.Fatal(err)
	}
	for _, rangeID := range []string{"conflicting-owner", "conflicting-owner-replay"} {
		counts, err := s.BackfillLegacyRange(ctx, BackfillRange{ID: rangeID, Limit: 10})
		if err != nil || counts.Scanned != 1 || counts.Quarantined != 1 || counts.Linked != 0 {
			t.Fatalf("stable owner reconciliation = %+v, %v", counts, err)
		}
	}
	var reason string
	if err := s.DB.QueryRow(ctx, `SELECT reason FROM account_backfill WHERE source_key=$1 AND state='quarantined'`, "legacy:"+legacyWallet).Scan(&reason); err != nil || reason == "" {
		t.Fatalf("quarantined legacy source = %q, %v", reason, err)
	}
	stable, err := s.Public(ctx, fmt.Sprintf("0x%040x", 9661))
	if err != nil || stable.Wallet != fmt.Sprintf("0x%040x", 9661) {
		t.Fatalf("stable owner profile = %+v, %v", stable, err)
	}
}

func TestLegacyRangeRequiresItsDurableSourceTables(t *testing.T) {
	for _, table := range []string{"account_backfill_range", "users.user_profiles", "users.user_images", "users.watchlists", "identity_wallet_links", "account_state", "account_backfill", "account_watch", "account_outbox"} {
		t.Run(table, func(t *testing.T) {
			s := connectedLegacyFixture(t)
			ctx := context.Background()
			p := owner(t, s, 9693)
			wallet := fmt.Sprintf("0x%040x", 9693)
			seedConnectedIdentity(t, s, p, uuid.New(), wallet)
			if _, err := s.DB.Exec(ctx, `INSERT INTO users.user_profiles(wallet_address,display_name,bio,created_at,updated_at) VALUES($1,'Legacy owner','profile',1,1)`, wallet); err != nil {
				t.Fatal(err)
			}
			if _, err := s.DB.Exec(ctx, "DROP TABLE "+table+" CASCADE"); err != nil {
				t.Fatal(err)
			}
			counts, err := s.BackfillLegacyRange(ctx, BackfillRange{ID: "source-schema", Limit: 10})
			if err == nil || counts.Linked != 0 || counts.Verified != 0 {
				t.Fatalf("backfill without %s = %+v, %v", table, counts, err)
			}
		})
	}
}

func TestStableReadCutoverRequiresBackfillAndSwitchState(t *testing.T) {
	for _, table := range []string{"account_backfill", "account_read_switch"} {
		t.Run(table, func(t *testing.T) {
			s := connectedLegacyFixture(t)
			ctx := context.Background()
			if _, err := s.DB.Exec(ctx, "DROP TABLE "+table+" CASCADE"); err != nil {
				t.Fatal(err)
			}
			if err := s.SetStableAccountReads(ctx, true); err == nil {
				t.Fatalf("cutover without %s accepted", table)
			}
		})
	}
}

func TestLegacyRangeNormalizesAfterWallet(t *testing.T) {
	s := connectedLegacyFixture(t)
	counts, err := s.BackfillLegacyRange(context.Background(), BackfillRange{ID: "normalized-after", After: "0x00000000000000000000000000000000000000AB", Limit: 10})
	if err != nil || counts.Scanned != 0 || counts.LastWallet != "" {
		t.Fatalf("normalized range cursor = %+v, %v", counts, err)
	}
}

func TestLegacyReconciliationPreservesReviewedPublicState(t *testing.T) {
	base := Profile{PublicProfile: PublicProfile{Wallet: "0x1111111111111111111111111111111111111111", DisplayName: "Ada"}, Socials: map[string]string{"twitter": "https://x.com/ada"}}
	same := base
	same.Revision = "9"
	if !sameLegacyProfile(base, same) {
		t.Fatal("revision changed public comparison")
	}
	changed := base
	changed.Socials = map[string]string{"twitter": "https://x.com/other"}
	if sameLegacyProfile(base, changed) {
		t.Fatal("social change accepted as equivalent")
	}
	if result, err := reconcileLegacyProfile(base, changed, 4, false); !errors.Is(err, ErrConflict) || result.DisplayName != "Ada" {
		t.Fatalf("existing profile reconciliation = %+v, %v", result, err)
	}
	foreign := base
	foreign.Wallet = "0x2222222222222222222222222222222222222222"
	if _, err := reconcileLegacyProfile(base, foreign, 4, false); !errors.Is(err, ErrConflict) {
		t.Fatalf("foreign profile reconciliation = %v", err)
	}
	if result, err := reconcileLegacyProfile(base, same, 4, true); err != nil || result.DisplayName != "Ada" {
		t.Fatalf("equivalent profile reconciliation = %+v, %v", result, err)
	}
	blank := Profile{PublicProfile: PublicProfile{Wallet: base.Wallet}}
	if result, err := reconcileLegacyProfile(blank, base, 4, false); err != nil || result.Revision != "4" || result.DisplayName != "Ada" {
		t.Fatalf("blank profile reconciliation = %+v, %v", result, err)
	}
	if _, err := reconcileLegacyProfile(blank, base, 4, true); !errors.Is(err, ErrConflict) {
		t.Fatalf("edited profile reconciliation = %v", err)
	}
}
