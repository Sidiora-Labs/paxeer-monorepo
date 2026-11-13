package account

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestStoreOperationsReportUnavailableDatabase(t *testing.T) {
	s := fixture(t)
	p := owner(t, s, 9881)
	ctx := context.Background()
	const wallet = "0x0000000000000000000000000000000000002699"
	s.DB.Close()
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"create account", func() error { return s.Create(ctx, uuid.New(), wallet) }},
		{"patch profile", func() error {
			return s.PatchProfile(ctx, p, p.AccountID, ProfilePatch{ExpectedRevision: "0", DisplayName: str("Ada")})
		}},
		{"read public profile", func() error { _, err := s.Public(ctx, wallet); return err }},
		{"read presentation", func() error { _, err := s.PublicPresentation(ctx, wallet); return err }},
		{"read selected profile", func() error { _, err := s.PublicFromReadSwitch(ctx, wallet); return err }},
		{"patch preferences", func() error { return s.PatchPreferences(ctx, p, p.AccountID, PreferencesPatch{ExpectedRevision: "0"}) }},
		{"patch extended preferences", func() error {
			return s.PatchExtendedPreferences(ctx, p, ExtendedPreferencesPatch{PreferencesPatch: PreferencesPatch{ExpectedRevision: "0"}})
		}},
		{"patch extended profile", func() error {
			return s.PatchExtendedProfile(ctx, p, ExtendedProfilePatch{ProfilePatch: ProfilePatch{ExpectedRevision: "0", DisplayName: str("Ada")}})
		}},
		{"read preferences", func() error { _, err := s.ReadPreferences(ctx, p, p.AccountID); return err }},
		{"read devices", func() error { _, err := s.Devices(ctx, p); return err }},
		{"delete watch", func() error { return s.DeleteWatch(ctx, p, p.AccountID, wallet) }},
		{"list watches", func() error { _, err := s.Watches(ctx, p, p.AccountID, nil, 10); return err }},
		{"endpoint eligibility", func() error {
			_, err := s.EndpointEligible(ctx, p.AccountID, uuid.New(), "trade", "push", 1)
			return err
		}},
		{"consent eligibility", func() error { _, err := s.Eligible(ctx, p.AccountID, "trade", "push", 1); return err }},
		{"account statistics", func() error { _, err := s.Stats(ctx); return err }},
		{"export snapshot", func() error { _, err := s.ExportSnapshot(ctx, p); return err }},
		{"request export", func() error { _, err := s.RequestExport(ctx, p); return err }},
		{"ready export", func() error { _, err := s.ReadyExport(ctx, uuid.New(), "account-exports/object"); return err }},
		{"acknowledge deletion", func() error { return s.AckDeletion(ctx, uuid.New(), "images") }},
		{"run account worker", func() error {
			return (Worker{Store: s, Runtime: &Runtime{}, Owner: "worker", LeaseDuration: time.Minute}).RunOnce(ctx)
		}},
		{"stable read selection", func() error { return s.SetStableAccountReads(ctx, true) }},
		{"legacy range", func() error {
			_, err := s.BackfillLegacyRange(ctx, BackfillRange{ID: uuid.NewString(), Limit: 10})
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); err == nil || !strings.Contains(err.Error(), "closed pool") {
				t.Fatalf("unavailable store result = %v", err)
			}
		})
	}
}
