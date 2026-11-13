package l17

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/google/uuid"
)

func TestSelectedShareDatabaseCancellation(t *testing.T) {
	ctx := context.Background()
	for _, action := range []string{"preview", "publish", "replay", "owner", "revoke", "public", "expire", "render"} {
		t.Run(action, func(t *testing.T) {
			portfolioCancellationMatrix(t, func(t *testing.T, r Repository) func() error {
				s, p, summary := sharingV3RepositoryFixture(t, r)
				selection := ShareSelectionV3{SnapshotID: summary.SnapshotID, Fields: []string{"knownRealizedSubtotalUsdlRaw"}, ExpiresAt: time.Now().Add(time.Hour)}
				var preview SharePreviewV3
				var err error
				if action != "preview" {
					preview, err = s.Preview(ctx, p, selection)
					if err != nil {
						t.Fatal(err)
					}
				}
				if action == "replay" || action == "public" || action == "render" {
					if _, err = s.Publish(ctx, p, SharePublishV3{ID: preview.ID, Intent: preview.Intent}, "selected-share"); err != nil {
						t.Fatal(err)
					}
				}
				worker := ShareWorkerV3{Repository: r, Owner: "selected-share-worker", FontDir: filepath.Join("..", "..", "ogrender", "fonts")}
				if action == "public" {
					if err = worker.RunOnce(ctx); err != nil {
						t.Fatal(err)
					}
				}
				return func() error {
					switch action {
					case "preview":
						result, err := s.Preview(ctx, p, selection)
						if err == nil && (result.ID == "" || result.Share.Values.KnownRealizedSubtotalUsdlRaw == nil) {
							t.Fatal(result)
						}
						return err
					case "publish", "replay":
						result, err := s.Publish(ctx, p, SharePublishV3{ID: preview.ID, Intent: preview.Intent}, "selected-share")
						if err == nil && result.State != "pending" {
							t.Fatal(result)
						}
						return err
					case "owner":
						result, err := s.Owner(ctx, p, preview.ID)
						if err == nil && result.State != "draft" {
							t.Fatal(result)
						}
						return err
					case "revoke":
						result, err := s.Revoke(ctx, p, preview.ID)
						if err == nil && result.State != "revoked" {
							t.Fatal(result)
						}
						return err
					case "public":
						result, image, err := s.Public(ctx, preview.ID)
						if err == nil && (result.ID != preview.ID || len(image) == 0) {
							t.Fatal(result, len(image))
						}
						return err
					case "expire":
						return worker.ExpireOnce(ctx)
					default:
						return worker.RunOnce(ctx)
					}
				}
			})
		})
	}
}

func TestSelectedShareValidatesOwnerExpiryAndPublicationState(t *testing.T) {
	ctx := context.Background()
	s, p, summary := sharingV3Fixture(t)
	now := time.Now().UTC()
	s.clock = func() time.Time { return now }
	selection := ShareSelectionV3{SnapshotID: summary.SnapshotID, Fields: []string{"knownRealizedSubtotalUsdlRaw"}, ExpiresAt: now.Add(5 * time.Minute)}
	for _, tc := range []struct {
		name      string
		principal auth.IdentityPrincipal
		selection ShareSelectionV3
	}{
		{"principal", auth.IdentityPrincipal{}, selection},
		{"selection", p, ShareSelectionV3{}},
		{"source", p, func() ShareSelectionV3 { x := selection; x.SnapshotID = uuid.NewString(); return x }()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.Preview(ctx, tc.principal, tc.selection); err == nil {
				t.Fatal("invalid preview accepted")
			}
		})
	}
	preview, err := s.Preview(ctx, p, selection)
	if err != nil || !preview.PreviewUntil.Equal(preview.Share.ExpiresAt) {
		t.Fatal(preview, err)
	}
	for _, action := range []string{"publish", "owner", "revoke"} {
		t.Run("invalid owner "+action, func(t *testing.T) {
			var err error
			switch action {
			case "publish":
				_, err = s.Publish(ctx, auth.IdentityPrincipal{}, SharePublishV3{ID: preview.ID, Intent: preview.Intent}, "publish")
			case "owner":
				_, err = s.Owner(ctx, auth.IdentityPrincipal{}, preview.ID)
			case "revoke":
				_, err = s.Revoke(ctx, auth.IdentityPrincipal{}, preview.ID)
			}
			if err == nil {
				t.Fatal("invalid owner accepted")
			}
		})
	}
	for _, id := range []string{"bad", uuid.NewString()} {
		if _, err = s.Owner(ctx, p, id); !errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
		if _, err = s.Revoke(ctx, p, id); !errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
		if _, _, err = s.Public(ctx, id); !errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
		if _, err = s.Publish(ctx, p, SharePublishV3{ID: id, Intent: preview.Intent}, "publish"); err == nil {
			t.Fatal("unavailable share published")
		}
	}
	for _, key := range []string{"", strings.Repeat("k", 129), "invalid\nkey"} {
		if _, err = s.Publish(ctx, p, SharePublishV3{ID: preview.ID, Intent: preview.Intent}, key); !errors.Is(err, ErrInvalid) {
			t.Fatal(err)
		}
	}
	if _, err = s.Publish(ctx, p, SharePublishV3{ID: preview.ID, Intent: "short"}, "publish"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	now = now.Add(6 * time.Minute)
	if _, err = s.Publish(ctx, p, SharePublishV3{ID: preview.ID, Intent: preview.Intent}, "publish"); !errors.Is(err, ErrGone) {
		t.Fatal(err)
	}
	owner, err := s.Owner(ctx, p, preview.ID)
	if err != nil || owner.State != statusExpired {
		t.Fatal(owner, err)
	}
	now = now.Add(-6 * time.Minute)
	for i := 1; i < 5; i++ {
		if _, err = s.Preview(ctx, p, selection); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.Preview(ctx, p, selection); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	if err = (ShareWorkerV3{Repository: s.repository, Owner: "invalid worker"}).RunOnce(ctx); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if err = (ShareWorkerV3{Repository: s.repository, Owner: "valid-worker"}).RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ release, contract string }{{strings.Repeat("z", 64), strings.Repeat("b", 64)}, {strings.Repeat("a", 64), strings.Repeat("z", 64)}} {
		if _, err = NewSharesV3(s.repository, s.authority, s.key, s.realm, tc.release, tc.contract, s.clock); !errors.Is(err, ErrInvalid) {
			t.Fatal(err)
		}
	}
	if _, err = NewSharesV3(Repository{}, s.authority, s.key, s.realm, s.release, s.contract, s.clock); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err = shareDigestV3([]byte(`{`)); err == nil {
		t.Fatal("invalid share digest accepted")
	}
}
