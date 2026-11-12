package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/internaltest"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	iface "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestPublicationAdmissionTracksCreatorIntentAndRevision(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.NewPostgres(t)
	const schema = "metadata_publication_admission"
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
	token := "0x" + strings.Repeat("a", 40)
	creator := "0x" + strings.Repeat("b", 40)
	poolAddress := "0x" + strings.Repeat("c", 40)
	identity := V2CanonicalIdentity{Token: iface.Token{Address: token}, PoolAddress: poolAddress, Creator: creator}
	principal := sharedauth.IdentityPrincipal{Wallet: creator, AccountID: uuid.NewString(), SessionID: uuid.NewString(), AuthEpoch: 1}
	service := PublicationService{Media: ConnectedMedia{Metadata: V2ContractHandlers{Store: store}}}
	metadata := json.RawMessage(`{"expectedRevision":"0","displayName":"Creator name","description":"Published description","tags":[],"socialLinks":[]}`)
	create := func(revision string) (PublicationBody, []byte, sharedauth.OperationRequest) {
		t.Helper()
		body := PublicationBody{DraftID: uuid.NewString(), OperationID: uuid.NewString(), CreationTxHash: "0x" + strings.Repeat("d", 64), Metadata: bytes.Replace(metadata, []byte(`"0"`), []byte(`"`+revision+`"`), 1), UploadRefs: []string{}}
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		path := "/v3/tokens/" + token + "/metadata-publications"
		now := time.Now().Unix()
		envelope := sharedauth.OperationEnvelope{Version: 2, ChainID: "125", Environment: "paxeer125-production", Actor: creator, Action: PublicationAction, Method: "POST", Path: path, BodyHash: publicationHash(raw), Nonce: strings.Repeat("a", 64), IssuedAt: now, ExpiresAt: now + 300}
		return body, raw, sharedauth.OperationRequest{Envelope: envelope, Body: raw, Actor: creator, Action: PublicationAction, Method: "POST", Path: path, RequestID: uuid.NewString()}
	}
	admit := func(body PublicationBody, raw []byte, req sharedauth.OperationRequest) ([]byte, error) {
		t.Helper()
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		response, err := service.admitInTx(ctx, tx, identity, principal, req, body, raw)
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		return response, nil
	}
	body, raw, req := create("0")
	changed := req
	changed.Actor = token
	if _, err := admit(body, raw, changed); !errors.Is(err, ErrForbidden) {
		t.Fatalf("changed admission actor = %v", err)
	}
	response, err := admit(body, raw, req)
	if err != nil {
		t.Fatal(err)
	}
	var status PublicationStatus
	if err := json.Unmarshal(response, &status); err != nil || status.Status != metadataPendingState || status.OperationID != body.OperationID || status.Creator != creator {
		t.Fatalf("admission status = %+v, %v", status, err)
	}
	replay, err := admit(body, raw, req)
	if err != nil || !bytes.Equal(replay, response) {
		t.Fatalf("durable admission replay = %s, %v", replay, err)
	}
	conflicting := body
	conflicting.OperationID = uuid.NewString()
	conflictRaw, err := json.Marshal(conflicting)
	if err != nil {
		t.Fatal(err)
	}
	conflictReq := req
	conflictReq.Body = conflictRaw
	conflictReq.Envelope.BodyHash = publicationHash(conflictRaw)
	if _, err := admit(conflicting, conflictRaw, conflictReq); !errors.Is(err, ErrConflict) {
		t.Fatalf("same draft with different operation = %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("metadata_publication_jobs_v3")+" SET state='authorization_required' WHERE operation=$1", body.OperationID); err != nil {
		t.Fatal(err)
	}
	renewed := req
	renewed.Envelope.Nonce = strings.Repeat("e", 64)
	if reply, err := admit(body, raw, renewed); err != nil || !bytes.Equal(reply, response) {
		t.Fatalf("renewed admission = %s, %v", reply, err)
	}
	var state, nonce string
	if err := pool.QueryRow(ctx, "SELECT state,grant_nonce FROM "+store.table("metadata_publication_jobs_v3")+" WHERE operation=$1", body.OperationID).Scan(&state, &nonce); err != nil || state != metadataPendingState || nonce != renewed.Envelope.Nonce {
		t.Fatalf("renewed grant = %q, %q, %v", state, nonce, err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO "+store.table("documents")+"(pool,blocked) VALUES($1,true)", poolAddress); err != nil {
		t.Fatal(err)
	}
	blocked, blockedRaw, blockedReq := create("0")
	if _, err := admit(blocked, blockedRaw, blockedReq); !errors.Is(err, ErrForbidden) {
		t.Fatalf("blocked pool admission = %v", err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM "+store.table("documents")+" WHERE pool=$1", poolAddress); err != nil {
		t.Fatal(err)
	}
	stale, staleRaw, staleReq := create("1")
	if _, err := admit(stale, staleRaw, staleReq); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale document admission = %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+store.table("metadata_publication_intents_v3")).Scan(&count); err != nil || count != 1 {
		t.Fatalf("durable intent count = %d, %v", count, err)
	}
	for _, table := range []string{"metadata_publication_intents_v3", "metadata_publication_jobs_v3", "pools", "documents"} {
		t.Run(table+" unavailable", func(t *testing.T) {
			if _, err := pool.Exec(ctx, "ALTER TABLE "+store.table(table)+" RENAME TO "+table+"_hidden"); err != nil {
				t.Fatal(err)
			}
			candidate, body, request := create("0")
			var databaseError *pgconn.PgError
			if _, err := admit(candidate, body, request); !errors.As(err, &databaseError) || databaseError.Code != "42P01" {
				t.Fatalf("unavailable %s = %v", table, err)
			}
			if _, err := pool.Exec(ctx, "ALTER TABLE "+store.table(table+"_hidden")+" RENAME TO "+table); err != nil {
				t.Fatal(err)
			}
		})
	}
}
