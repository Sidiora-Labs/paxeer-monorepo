package storage

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestConnectedStorageOperationsRespectCanceledRequest(t *testing.T) {
	base, ctx := connectedAuthority(t)
	hash, key := seedConnectedAsset(t, base, ctx)
	a, err := NewConnectedAuthority(base.Pool, base.Schema, base.MetadataSchema, base.SafetySchema)
	if err != nil {
		t.Fatal(err)
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	for _, tc := range []struct {
		name string
		run  func(context.Context) error
	}{
		{"download authority", func(c context.Context) error { return a.Allowed(c, key) }},
		{"restricted delivery", func(c context.Context) error {
			_, err := a.Restrict(c, hash, 2, blockedState)
			return err
		}},
		{"operation read", func(c context.Context) error {
			_, err := a.Operation(c, uuid.NewString())
			return err
		}},
		{"signed download authority", func(c context.Context) error { return a.AuthorizeDownload(c, key, time.Minute) }},
		{"tombstone replay", func(c context.Context) error { return a.ReplayTombstones(c) }},
		{"approved lineage import", func(c context.Context) error { return a.ImportApprovedLineage(c, "asset") }},
		{"restriction intake", func(c context.Context) error {
			_, err := a.IntakeRestrictions(c, nil)
			return err
		}},
		{"approved batch import", func(c context.Context) error {
			_, err := a.ImportApprovedBatch(c)
			return err
		}},
		{"deleted asset intake", func(c context.Context) error {
			_, err := a.IntakeDeletedAssets(c)
			return err
		}},
		{"purge backlog age", func(c context.Context) error {
			_, err := a.PurgeBacklogAge(c)
			return err
		}},
		{"media backlog ages", func(c context.Context) error {
			_, _, err := a.MediaBacklogAges(c)
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(canceled); err == nil {
				t.Fatal("canceled storage operation succeeded")
			}
		})
	}
}
