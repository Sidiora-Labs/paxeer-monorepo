package account

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
	"github.com/google/uuid"
)

func TestRepair471_SealedExportBound(t *testing.T) {
	s := fixture(t)
	objects := internaltest.NewStore(t, "repair471-exports")
	ctx := context.Background()
	key := make([]byte, 32)
	_, err := rand.Read(key)
	must(t, err)
	runtime := &Runtime{Objects: objects, ExportKey: key, Metrics: &Metrics{}}
	worker := Worker{Store: s, Runtime: runtime, Owner: "repair471-worker", LeaseDuration: time.Minute}
	handler := (HTTP{Store: s, Runtime: runtime}).DownloadHandler()
	for i, tc := range []struct {
		name string
		size int64
	}{
		{"ordinary", 0},
		{"exact_limit", maxSealedExportBytes},
		{"limit_plus_one", maxSealedExportBytes + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := owner(t, s, 471+i)
			job, err := s.RequestExport(ctx, p)
			must(t, err)
			must(t, worker.RunOnce(ctx))
			var payload []byte
			var fence int64
			must(t, s.DB.QueryRow(ctx, `SELECT payload,fence FROM account_job WHERE job_id=$1 AND state='ready'`, job.ID).Scan(&payload, &fence))
			var metadata exportMetadata
			must(t, json.Unmarshal(payload, &metadata))
			token, err := Open(key, metadata.Authorization, []byte(job.ID.String()))
			must(t, err)
			snapshot, err := s.ExportSnapshot(ctx, p)
			must(t, err)
			plain := snapshot
			if tc.size != 0 {
				// JSON permits trailing whitespace. Keep the real snapshot and have
				// the production worker seal and publish the boundary-sized document.
				plain = append(append([]byte(nil), snapshot...), bytes.Repeat([]byte(" "), int(tc.size)-12-16-len(snapshot))...)
			}
			object, err := worker.writeExportObject(ctx, jobLease{Job: job, Fence: fence}, plain)
			must(t, err)
			if object != metadata.EncryptedObject {
				t.Fatal("worker published a different object", object)
			}
			info, err := objects.Head(ctx, object)
			must(t, err)
			if info.Size != int64(len(plain))+12+16 || tc.size != 0 && info.Size != tc.size {
				t.Fatal("sealed object did not exercise the requested size", info.Size)
			}
			requestID := uuid.New()
			download := func() *httptest.ResponseRecorder {
				r := httptest.NewRequest(http.MethodGet, "/v1/me/exports/"+job.ID.String()+"/download", nil)
				r.SetPathValue("jobId", job.ID.String())
				r.Header.Set("X-Export-Authorization", "Bearer "+string(token))
				r.Header.Set("X-Request-ID", requestID.String())
				r = r.WithContext(WithPrincipal(r.Context(), p))
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, r)
				return w
			}
			response := download()
			if tc.size > maxSealedExportBytes {
				// The ciphertext itself is authentic, so a successful unbounded
				// read would deliver it. Storage must reject it before delivery.
				obj, err := objects.Get(ctx, object)
				must(t, err)
				sealed, err := io.ReadAll(obj.Body)
				closeErr := obj.Body.Close()
				must(t, err)
				must(t, closeErr)
				opened, err := Open(key, sealed, []byte(job.ID.String()))
				must(t, err)
				if !bytes.Equal(opened, plain) {
					t.Fatal("oversize object was not authentic")
				}
				status, expected := PublicError(storage.ErrObjectTooLarge, requestID)
				var refusal WireError
				must(t, json.Unmarshal(response.Body.Bytes(), &refusal))
				if response.Code != status || !reflect.DeepEqual(refusal, expected) {
					t.Fatalf("refusal changed: %d %s", response.Code, response.Body.String())
				}
				if response.Header().Get("Content-Disposition") != "" {
					t.Fatal("refusal included attachment headers")
				}
				var consumed bool
				var leased bool
				must(t, s.DB.QueryRow(ctx, `SELECT consumed_at IS NOT NULL,lease_owner IS NOT NULL OR lease_until IS NOT NULL FROM account_job WHERE job_id=$1`, job.ID).Scan(&consumed, &leased))
				if consumed || leased {
					t.Fatal("oversize refusal consumed the credential or retained its claim", consumed, leased)
				}
				plain = snapshot
				_, err = worker.writeExportObject(ctx, jobLease{Job: job, Fence: fence}, plain)
				must(t, err)
				response = download()
			}
			if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), plain) {
				t.Fatalf("in-bound download failed: status=%d bytes=%d want=%d", response.Code, response.Body.Len(), len(plain))
			}
			if response.Header().Get("Cache-Control") != "private, no-store" || response.Header().Get("Content-Type") != "application/json" || response.Header().Get("Content-Disposition") != "attachment; filename=account-export.json" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal("download headers changed", response.Header())
			}
			replay := download()
			var refusal WireError
			must(t, json.Unmarshal(replay.Body.Bytes(), &refusal))
			if replay.Code != http.StatusForbidden || refusal.Code != "SCOPE_DENIED" {
				t.Fatal("successful export was reusable", replay.Code, refusal)
			}
			if _, err := s.ConsumeExport(ctx, p, job.ID, string(token)); !errors.Is(err, ErrDenied) {
				t.Fatal("successful delivery was not durably consumed", err)
			}
		})
	}
}
