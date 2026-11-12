package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/internaltest"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestPublicationQueueClaimsAndChecksStoredIntent(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.NewPostgres(t)
	const schema = "metadata_publication_queue"
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	store, err := NewProductionStore(pool, schema, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, store.SchemaDDL()+store.V2SchemaDDL()+store.ConnectedSchemaDDL()+store.PublicationSchemaDDL()); err != nil {
		t.Fatal(err)
	}
	poolAddress := "0x" + strings.Repeat("a", 40)
	if _, err := pool.Exec(ctx, "INSERT INTO "+store.table("pools")+"(pool,creator,token) VALUES($1,$1,$1)", poolAddress); err != nil {
		t.Fatal(err)
	}
	service := PublicationService{
		Media:      ConnectedMedia{Metadata: V2ContractHandlers{Store: store}},
		Operations: sharedauth.OperationAuthorizer{Ledger: sharedauth.Ledger{Environment: "paxeer125-production"}},
	}
	metadata := []byte(`{"expectedRevision":"0","displayName":"Queue name"}`)
	creation := "0x" + strings.Repeat("b", 64)
	insert := func(operation string, raw, envelope []byte, expires time.Time) {
		t.Helper()
		_, err := pool.Exec(ctx, "INSERT INTO "+store.table("metadata_publication_intents_v3")+`(operation,draft_id,token,pool,creator,creation_tx_hash,content_hash,body_hash,body,metadata,upload_refs,envelope,envelope_digest,nonce,request_id,account_id,session_id,auth_epoch,admission_response,accepted_at,expires_at)
			VALUES($1,$2,$3,$3,$3,$4,$5,$6,$7,$8,'[]',$9,$10,'nonce',$2,$2,$2,1,'{}',$11::timestamptz,$11::timestamptz+interval '30 minutes')`, operation, uuid.NewString(), poolAddress, creation, publicationHash(metadata), publicationHash(raw), raw, metadata, envelope, publicationHash(envelope), expires.Add(-30*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		_, err = pool.Exec(ctx, "INSERT INTO "+store.table("metadata_publication_jobs_v3")+"(operation,state,grant_nonce,next_at,updated_at) VALUES($1,'metadata_pending','nonce',clock_timestamp(),clock_timestamp())", operation)
		if err != nil {
			t.Fatal(err)
		}
		_, err = pool.Exec(ctx, "INSERT INTO "+store.table("metadata_publication_grants_v3")+"(operation,nonce,envelope,envelope_digest,body_hash,request_id,account_id,session_id,auth_epoch,response) VALUES($1,'nonce',$2,$3,$4,$5,$5,$5,1,'{}')", operation, envelope, publicationHash(envelope), publicationHash(raw), uuid.NewString())
		if err != nil {
			t.Fatal(err)
		}
	}
	expired := uuid.NewString()
	insert(expired, []byte(`{}`), []byte(`{}`), time.Now().Add(-time.Minute))
	if _, err := service.claimPublication(ctx); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expired intent claimed: %v", err)
	}
	var expiredState, expiredCode string
	if err := pool.QueryRow(ctx, "SELECT state,last_error_code FROM "+store.table("metadata_publication_jobs_v3")+" WHERE operation=$1", expired).Scan(&expiredState, &expiredCode); err != nil || expiredState != "rejected" || expiredCode != "retry_exhausted" {
		t.Fatalf("expired intent state = %q, %q, %v", expiredState, expiredCode, err)
	}
	for _, tc := range []struct {
		name     string
		raw      []byte
		envelope []byte
		want     error
	}{
		{name: "invalid body", raw: []byte(`{}`), envelope: []byte(`{}`), want: ErrValidation},
		{name: "invalid envelope", envelope: []byte(`{`), want: ErrUnrepresentable},
		{name: "invalid statement", envelope: []byte(`{}`), want: ErrUnrepresentable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			operation := uuid.NewString()
			raw := tc.raw
			if raw == nil {
				var err error
				raw, err = json.Marshal(PublicationBody{DraftID: uuid.NewString(), OperationID: operation, CreationTxHash: creation, Metadata: metadata, UploadRefs: []string{}})
				if err != nil {
					t.Fatal(err)
				}
			}
			insert(operation, raw, tc.envelope, time.Now().Add(20*time.Minute))
			lease, err := service.claimPublication(ctx)
			if err != nil || lease.Operation != operation || lease.Attempt != 1 || lease.Fence != 1 {
				t.Fatalf("claim = %+v, %v", lease, err)
			}
			wrong := lease
			wrong.Fence++
			if err := service.applyPublication(ctx, wrong); err == nil || err.Error() != "publication_fence_lost" {
				t.Fatalf("wrong fence = %v", err)
			}
			if err := service.applyPublication(ctx, lease); !errors.Is(err, tc.want) {
				t.Fatalf("stored intent = %v, want %v", err, tc.want)
			}
			if err := service.failPublication(ctx, lease, tc.want); err != nil {
				t.Fatal(err)
			}
			var state string
			if err := pool.QueryRow(ctx, "SELECT state FROM "+store.table("metadata_publication_jobs_v3")+" WHERE operation=$1", operation).Scan(&state); err != nil || state != "rejected" {
				t.Fatalf("failed intent state = %q, %v", state, err)
			}
		})
	}
	operation := uuid.NewString()
	insert(operation, []byte(`{}`), []byte(`{}`), time.Now().Add(20*time.Minute))
	lease, err := service.claimPublication(ctx)
	if err != nil || lease.Operation != operation {
		t.Fatalf("failure classification lease = %+v, %v", lease, err)
	}
	for _, tc := range []struct {
		name  string
		cause error
		count int
		state string
		code  string
	}{
		{"creator authorization", ErrForbidden, 1, "authorization_required", "creator_authorization_required"},
		{"stale revision", ErrConflict, 1, "rejected", "stale_revision"},
		{"asset pending", ErrMissing, 1, "metadata_pending", "asset_pending"},
		{"dependency unavailable", errors.New("storage unavailable"), 1, "metadata_pending", "dependency_unavailable"},
		{"retry exhausted", errors.New("storage unavailable"), 5, "rejected", "retry_exhausted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := pool.Exec(ctx, "UPDATE "+store.table("metadata_publication_jobs_v3")+" SET state='publishing',lease_owner=$2,lease_until=clock_timestamp()+interval '1 minute' WHERE operation=$1", operation, lease.Owner); err != nil {
				t.Fatal(err)
			}
			candidate := lease
			candidate.Attempt = tc.count
			if err := service.failPublication(ctx, candidate, tc.cause); err != nil {
				t.Fatal(err)
			}
			var state, code string
			if err := pool.QueryRow(ctx, "SELECT state,last_error_code FROM "+store.table("metadata_publication_jobs_v3")+" WHERE operation=$1", operation).Scan(&state, &code); err != nil || state != tc.state || code != tc.code {
				t.Fatalf("classified failure = %q, %q, %v; want %q, %q", state, code, err, tc.state, tc.code)
			}
		})
	}
}
