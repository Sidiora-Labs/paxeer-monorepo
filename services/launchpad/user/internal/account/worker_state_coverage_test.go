package account

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
	"github.com/google/uuid"
)

func TestDeletionWorkerRequiresOwnersAndCompletesAcknowledgedJob(t *testing.T) {
	for _, tc := range []struct {
		name     string
		payload  any
		wantErr  error
		wantDone bool
	}{
		{"missing owners", map[string]any{}, ErrState, false},
		{"missing owner service", map[string]any{"requiredOwners": []string{"images"}, "deadline": time.Now().Add(-time.Minute)}, errors.New("DEPENDENCY_UNAVAILABLE"), false},
		{"acknowledged owner", map[string]any{"requiredOwners": []string{"images"}, "acks": []string{"images"}, "deadline": time.Now().Add(-time.Minute)}, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := fixture(t)
			p := owner(t, s, 9781)
			ctx := context.Background()
			job := Job{ID: uuid.New(), AccountID: p.AccountID, Kind: kindDeletion, State: stateProcessing, RequestedAt: time.Now().Add(-time.Minute)}
			raw, err := json.Marshal(tc.payload)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.DB.Exec(ctx, `INSERT INTO account_job(job_id,account_id,kind,state,requested_at,payload,lease_owner,lease_until,fence) VALUES($1,$2,'deletion','processing',$3,$4,'worker',clock_timestamp()+interval '1 minute',1)`, job.ID, p.AccountID, job.RequestedAt, raw); err != nil {
				t.Fatal(err)
			}
			metrics := &Metrics{}
			worker := Worker{Store: s, Runtime: &Runtime{Metrics: metrics, DeletionOwners: map[string]DeletionOwner{}}, Owner: "worker", LeaseDuration: time.Minute}
			err = worker.processDeletion(ctx, jobLease{Job: job, Fence: 1})
			if tc.wantErr == nil && err != nil || tc.wantErr != nil && (err == nil || !errors.Is(err, tc.wantErr) && err.Error() != tc.wantErr.Error()) {
				t.Fatalf("deletion process = %v, want %v", err, tc.wantErr)
			}
			var state string
			if err := s.DB.QueryRow(ctx, `SELECT state FROM account_job WHERE job_id=$1`, job.ID).Scan(&state); err != nil {
				t.Fatal(err)
			}
			if tc.wantDone && (state != "completed" || metrics.Snapshot()["deletion.success"].Count != 1) {
				t.Fatalf("acknowledged deletion = %s, %+v", state, metrics.Snapshot())
			}
			if !tc.wantDone && state != "processing" {
				t.Fatalf("rejected deletion changed state to %s", state)
			}
		})
	}
}

func TestDeletionWorkerPersistsImageOwnerAcknowledgement(t *testing.T) {
	s := connectedLegacyFixture(t)
	p := owner(t, s, 9785)
	ctx := context.Background()
	objects := internaltest.NewStore(t, "user-worker-deletion")
	job := uuid.New()
	payload, err := json.Marshal(map[string]any{"requiredOwners": []string{"images"}, "deadline": time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(ctx, `UPDATE account_state SET deleted=true WHERE account_id=$1`, p.AccountID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(ctx, `INSERT INTO account_job(job_id,account_id,kind,state,requested_at,payload) VALUES($1,$2,'deletion','requested',clock_timestamp(),$3)`, job, p.AccountID, payload); err != nil {
		t.Fatal(err)
	}
	metrics := &Metrics{}
	worker := Worker{Store: s, Runtime: &Runtime{Metrics: metrics, DeletionOwners: map[string]DeletionOwner{"images": ImagePurger{Store: s, Objects: objects, Exports: objects}}}, Owner: "image-worker", LeaseDuration: time.Minute}
	if err := worker.RunOnce(ctx); err != nil {
		t.Fatalf("image deletion worker = %v", err)
	}
	var state string
	var result []byte
	if err := s.DB.QueryRow(ctx, `SELECT state,payload FROM account_job WHERE job_id=$1`, job).Scan(&state, &result); err != nil {
		t.Fatal(err)
	}
	var progress struct {
		Acks []string `json:"acks"`
	}
	if err := json.Unmarshal(result, &progress); err != nil || state != "completed" || len(progress.Acks) != 1 || progress.Acks[0] != "images" || metrics.Snapshot()["deletion.success"].Count != 1 {
		t.Fatalf("image owner acknowledgement = %q %+v, %v", state, progress, err)
	}
}

func TestExportWorkerRemovesObjectAfterLostLease(t *testing.T) {
	s := fixture(t)
	p := owner(t, s, 9786)
	ctx := context.Background()
	objects := internaltest.NewStore(t, "user-lost-export")
	job, err := s.RequestExport(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	worker := Worker{Store: s, Runtime: &Runtime{Objects: objects, ExportKey: []byte(strings.Repeat("k", 32)), Metrics: &Metrics{}}, Owner: "export-worker", LeaseDuration: time.Minute}
	lease := jobLease{Job: job, Fence: 99}
	if err := worker.prepareExport(ctx, lease); !errors.Is(err, delivery.ErrLease) {
		t.Fatalf("lost export lease = %v", err)
	}
	object := "account-exports/" + job.ID.String() + "/99"
	if _, err := objects.Head(ctx, object); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("uncommitted export object = %v", err)
	}
	var state string
	if err := s.DB.QueryRow(ctx, `SELECT state FROM account_job WHERE job_id=$1`, job.ID).Scan(&state); err != nil || state != "requested" {
		t.Fatalf("export job after lost lease = %q, %v", state, err)
	}
}

func TestImagePurgerRejectsUncommittedDeletion(t *testing.T) {
	s := fixture(t)
	p := owner(t, s, 9782)
	objects := internaltest.NewStore(t, "user-account-purge")
	purger := ImagePurger{Store: s, Objects: objects, Exports: objects}
	if err := purger.DeleteAccount(context.Background(), p.AccountID, uuid.New()); !errors.Is(err, ErrState) {
		t.Fatalf("live account image purge = %v", err)
	}
	if err := purger.DeleteAccount(context.Background(), uuid.Nil, uuid.New()); err == nil || err.Error() != "DEPENDENCY_UNAVAILABLE" {
		t.Fatalf("missing account image purge = %v", err)
	}
	if _, err := s.DB.Exec(context.Background(), `UPDATE account_state SET deleted=true WHERE account_id=$1`, p.AccountID); err != nil {
		t.Fatal(err)
	}
	purger.Exports = nil
	if err := purger.DeleteAccount(context.Background(), p.AccountID, uuid.New()); err == nil || err.Error() != "DEPENDENCY_UNAVAILABLE" {
		t.Fatalf("missing export object store = %v", err)
	}
}

func TestExportCleanupRefusesInvalidObjectMetadata(t *testing.T) {
	s := connectedLegacyFixture(t)
	p := owner(t, s, 9787)
	ctx := context.Background()
	objects := internaltest.NewStore(t, "user-invalid-export")
	job := uuid.New()
	if _, err := s.DB.Exec(ctx, `INSERT INTO account_job(job_id,account_id,kind,state,requested_at,payload) VALUES($1,$2,'export','ready',clock_timestamp(),jsonb_build_object('encryptedObject',42,'retainUntil',$3::timestamptz))`, job, p.AccountID, s.now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	worker := Worker{Store: s, Runtime: &Runtime{Objects: objects}}
	if err := worker.PurgeExpiredExports(ctx, 10); err == nil {
		t.Fatal("invalid export metadata expired as a valid object")
	}
	if _, err := s.DB.Exec(ctx, `UPDATE account_state SET deleted=true WHERE account_id=$1`, p.AccountID); err != nil {
		t.Fatal(err)
	}
	purger := ImagePurger{Store: s, Objects: objects, Exports: objects}
	if err := purger.DeleteAccount(ctx, p.AccountID, uuid.New()); err == nil {
		t.Fatal("invalid export metadata allowed account purge")
	}
	var state string
	if err := s.DB.QueryRow(ctx, `SELECT state FROM account_job WHERE job_id=$1`, job).Scan(&state); err != nil || state != "ready" {
		t.Fatalf("invalid export job = %q, %v", state, err)
	}
}

func TestImagePurgerDeletesLegacyObjectsAndExport(t *testing.T) {
	s := connectedLegacyFixture(t)
	p := owner(t, s, 9783)
	ctx := context.Background()
	wallet := fmt.Sprintf("0x%040x", 9783)
	objects := internaltest.NewStore(t, "user-account-purge")
	const imageKey = "avatars/account-delete.png"
	const exportKey = "account-exports/account-delete"
	for key, body := range map[string]string{imageKey: "image data", exportKey: "encrypted export"} {
		if err := objects.Put(ctx, key, strings.NewReader(body), int64(len(body)), "application/octet-stream"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.DB.Exec(ctx, `INSERT INTO users.user_profiles(wallet_address,display_name,created_at,updated_at) VALUES($1,'Ada',1,1)`, wallet); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(ctx, `INSERT INTO users.user_images(id,wallet_address,image_type,storage_key,mime_type,size_bytes,uploaded_at) VALUES($1,$2,'avatar',$3,'image/png',10,1)`, wallet+"-avatar", wallet, imageKey); err != nil {
		t.Fatal(err)
	}
	jobID := uuid.New()
	payload, err := json.Marshal(exportMetadata{EncryptedObject: exportKey, RetainUntil: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(ctx, `INSERT INTO account_job(job_id,account_id,kind,state,requested_at,payload) VALUES($1,$2,'export','ready',clock_timestamp(),$3)`, jobID, p.AccountID, payload); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(ctx, `UPDATE account_state SET deleted=true WHERE account_id=$1`, p.AccountID); err != nil {
		t.Fatal(err)
	}
	purger := ImagePurger{Store: s, Objects: objects, Exports: objects}
	if err := purger.DeleteAccount(ctx, p.AccountID, uuid.New()); err != nil {
		t.Fatalf("account image purge = %v", err)
	}
	var profiles, images int
	if err := s.DB.QueryRow(ctx, `SELECT count(*) FROM users.user_profiles WHERE lower(wallet_address)=$1`, wallet).Scan(&profiles); err != nil || profiles != 0 {
		t.Fatalf("legacy profiles = %d, %v", profiles, err)
	}
	if err := s.DB.QueryRow(ctx, `SELECT count(*) FROM users.user_images WHERE lower(wallet_address)=$1`, wallet).Scan(&images); err != nil || images != 0 {
		t.Fatalf("legacy images = %d, %v", images, err)
	}
	var state string
	if err := s.DB.QueryRow(ctx, `SELECT state FROM account_job WHERE job_id=$1`, jobID).Scan(&state); err != nil || state != "expired" {
		t.Fatalf("export after purge = %q, %v", state, err)
	}
	for _, key := range []string{imageKey, exportKey} {
		if _, err := objects.Head(ctx, key); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("deleted object %s = %v", key, err)
		}
	}
}

func TestWorkerExpiresRetainedExportObject(t *testing.T) {
	s := fixture(t)
	p := owner(t, s, 9784)
	objects := internaltest.NewStore(t, "user-expired-export")
	ctx := context.Background()
	const key = "account-exports/expired-object"
	if err := objects.Put(ctx, key, strings.NewReader("encrypted snapshot"), int64(len("encrypted snapshot")), "application/octet-stream"); err != nil {
		t.Fatal(err)
	}
	jobID := uuid.New()
	payload, err := json.Marshal(exportMetadata{EncryptedObject: key, RetainUntil: s.now().Add(-time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(ctx, `INSERT INTO account_job(job_id,account_id,kind,state,requested_at,payload,expires_at) VALUES($1,$2,'export','ready',clock_timestamp(),$3,clock_timestamp()-interval '1 minute')`, jobID, p.AccountID, payload); err != nil {
		t.Fatal(err)
	}
	worker := Worker{Store: s, Runtime: &Runtime{Objects: objects}}
	if err := worker.PurgeExpiredExports(ctx, 16); err != nil {
		t.Fatal(err)
	}
	var state string
	var cleared bool
	if err := s.DB.QueryRow(ctx, `SELECT state,payload='{}'::jsonb FROM account_job WHERE job_id=$1`, jobID).Scan(&state, &cleared); err != nil || state != "expired" || !cleared {
		t.Fatalf("expired export = %q, cleared=%t, %v", state, cleared, err)
	}
	if _, err := objects.Head(ctx, key); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("retained export object = %v", err)
	}
}
