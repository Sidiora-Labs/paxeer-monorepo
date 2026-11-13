package metadata

import (
	"context"
	"strings"
	"testing"

	raster "github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/image"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/internaltest"
	"github.com/google/uuid"
)

func TestPublicationRetryClassifiesDurableJobs(t *testing.T) {
	ctx := context.Background()
	pool, _ := internaltest.NewPostgresURL(t)
	const schema = "metadata_publication_retry"
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
	address := "0x" + strings.Repeat("a", 40)
	if _, err := pool.Exec(ctx, "INSERT INTO "+store.table("pools")+"(pool,creator,token) VALUES($1,$1,$1)", address); err != nil {
		t.Fatal(err)
	}
	service := PublicationService{Media: ConnectedMedia{Metadata: V2ContractHandlers{Store: store}}}
	for _, tc := range []struct {
		name, state, code string
		cause             error
		attempt           int
	}{
		{"asset pending", metadataPendingState, "asset_pending", ErrMissing, 1},
		{"authorization", "authorization_required", "creator_authorization_required", ErrForbidden, 1},
		{"revision", metadataStateRejectedWire, "stale_revision", ErrConflict, 1},
		{"invalid intent", metadataStateRejectedWire, "invalid_intent", ErrValidation, 1},
		{"dependency", metadataPendingState, "dependency_unavailable", raster.ErrUnavailable, 1},
		{"retry exhaustion", metadataStateRejectedWire, "retry_exhausted", ErrMissing, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			operation, owner := uuid.NewString(), uuid.NewString()
			_, err := pool.Exec(ctx, "INSERT INTO "+store.table("metadata_publication_intents_v3")+`(operation,draft_id,token,pool,creator,creation_tx_hash,content_hash,body_hash,body,metadata,upload_refs,envelope,envelope_digest,nonce,request_id,account_id,session_id,auth_epoch,admission_response,accepted_at,expires_at)
				SELECT $1,$2,$3,$3,$3,$4,$5,$5,'{}','{}','[]','{}',$5,'nonce',$2,$2,$2,1,'{}',accepted.at,accepted.at+interval '30 minutes' FROM (SELECT clock_timestamp() AS at) accepted`, operation, uuid.NewString(), address, "0x"+strings.Repeat("b", 64), strings.Repeat("c", 64))
			if err != nil {
				t.Fatal(err)
			}
			_, err = pool.Exec(ctx, "INSERT INTO "+store.table("metadata_publication_jobs_v3")+"(operation,state,attempt_count,fence,lease_owner,lease_until,grant_nonce,next_at,updated_at) VALUES($1,'publishing',$2,1,$3,clock_timestamp()+interval '1 minute','nonce',clock_timestamp(),clock_timestamp())", operation, tc.attempt, owner)
			if err != nil {
				t.Fatal(err)
			}
			if err := service.failPublication(ctx, publicationLease{Operation: operation, Owner: owner, Fence: 1, Attempt: tc.attempt}, tc.cause); err != nil {
				t.Fatal(err)
			}
			var state, code string
			if err := pool.QueryRow(ctx, "SELECT state,last_error_code FROM "+store.table("metadata_publication_jobs_v3")+" WHERE operation=$1", operation).Scan(&state, &code); err != nil || state != tc.state || code != tc.code {
				t.Fatalf("retry state %q code %q, want %q %q: %v", state, code, tc.state, tc.code, err)
			}
		})
	}
}
