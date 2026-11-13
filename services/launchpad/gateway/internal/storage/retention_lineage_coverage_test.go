package storage

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	sharedstorage "github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
)

func TestConnectedLineageRetentionAndIdempotentBindings(t *testing.T) {
	a, ctx := connectedAuthority(t)
	hash, _ := seedConnectedAsset(t, a, ctx)
	variantKey := "approved/" + hash + "/512/" + strings.Repeat("c", 64) + ".png"
	until := time.Now().UTC().Add(31 * 24 * time.Hour).Truncate(time.Microsecond)
	objects := []LineageObject{
		{Hash: hash, Key: variantKey, Kind: variantKind, RetainUntil: until},
		{Hash: hash, Key: "private/owned-original", Kind: originalKind, RetainUntil: until},
		{Hash: hash, Key: "temporary/owned-preview", Kind: "temporary", RetainUntil: until},
		{Hash: hash, Key: "private/recordings/owned-recording", Kind: "recording", RetainUntil: until},
	}
	for _, object := range objects {
		if err := a.RegisterLineage(ctx, object); err != nil {
			t.Fatalf("register %s lineage: %v", object.Kind, err)
		}
		if err := a.RegisterLineage(ctx, object); err != nil {
			t.Fatalf("repeated %s lineage: %v", object.Kind, err)
		}
	}
	for _, tc := range []struct {
		name   string
		object LineageObject
	}{
		{"short original retention", LineageObject{Hash: hash, Key: "private/short-retention", Kind: originalKind, RetainUntil: time.Now().Add(24 * time.Hour)}},
		{"conflicting hold", LineageObject{Hash: hash, Key: variantKey, Kind: variantKind, RetainUntil: until, LegalHold: true}},
		{"unsafe object path", LineageObject{Hash: hash, Key: "temporary/../asset", Kind: "temporary", RetainUntil: until}},
		{"incorrect digest", LineageObject{Hash: strings.Repeat("f", 64), Key: variantKey, Kind: variantKind, RetainUntil: until}},
		{"unrecognized kind", LineageObject{Hash: hash, Key: "temporary/owned-preview", Kind: "other", RetainUntil: until}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := a.RegisterLineage(ctx, tc.object); !errors.Is(err, ErrConflict) {
				t.Fatalf("unbound lineage: %v", err)
			}
		})
	}
	missingHash := strings.Repeat("e", 64)
	if err := a.RegisterLineage(ctx, LineageObject{Hash: missingHash, Key: "temporary/absent", Kind: "temporary", RetainUntil: until}); !errors.Is(err, ErrRestricted) {
		t.Fatalf("unapproved asset lineage: %v", err)
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if err := a.RegisterLineage(canceled, LineageObject{Hash: hash, Key: "temporary/closed-context", Kind: "temporary", RetainUntil: until}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("closed database context: %v", err)
	}
	worker := DeleteWorker{Authority: a, Stores: map[string]*sharedstorage.Client{}}
	if worked, err := worker.RunOne(ctx); worked || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unbound deletion worker: worked=%t err=%v", worked, err)
	}
}
