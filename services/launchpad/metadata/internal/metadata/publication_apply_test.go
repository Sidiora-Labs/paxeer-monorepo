package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/internaltest"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestPublicationWorkerCommitsAndReplaysStoredIntent(t *testing.T) {
	ctx := context.Background()
	pool, dsn := internaltest.NewPostgresURL(t)
	const schema = "metadata_publication_apply"
	const canonical = "metadata_publication_canonical"
	const stream = "paxeer125-canonical"
	for _, name := range []string{schema, canonical} {
		if _, err := pool.Exec(ctx, "CREATE SCHEMA "+name); err != nil {
			t.Fatal(err)
		}
	}
	key := repair1007Key(t)
	creation := repair1007Creator(loadCreationFixture(t), key)
	seedCanonical(t, pool, canonical, stream, creation)
	store, err := NewProductionStore(pool, schema, nil)
	if err != nil {
		t.Fatal(err)
	}
	ledgerDDL := strings.ReplaceAll(sharedauth.LedgerDDL, "CREATE TABLE operation_ledger", "CREATE TABLE "+store.table("operation_ledger"))
	ledgerDDL = strings.ReplaceAll(ledgerDDL, "CREATE TABLE security_audit_outbox", "CREATE TABLE "+store.table("security_audit_outbox"))
	if _, err := pool.Exec(ctx, store.SchemaDDL()+store.V2SchemaDDL()+store.ConnectedSchemaDDL()+store.PublicationSchemaDDL()+ledgerDDL); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO "+store.table("pools")+"(pool,creator,token) VALUES($1,$2,$3)", creation.Pool, creation.Creator, creation.Token); err != nil {
		t.Fatal(err)
	}
	objects := internaltest.NewMinIO(t, "metadata-publication-apply")
	redisURL := internaltest.NewRedisURL(t)
	gateway := internaltest.NewGateway(t, dsn, redisURL, objects, "https://app.sidiora.test")
	wallet, bearer := gateway.SignInWithKey(t, key)
	if wallet != creation.Creator {
		t.Fatalf("creator identity = %q, want %q", wallet, creation.Creator)
	}
	authority := sharedauth.HTTPIdentityAuthority{Endpoint: gateway.URL, ServiceToken: gateway.ServiceToken, AllowPlainHTTP: true}
	authRequest := httptest.NewRequest(http.MethodGet, "/v3/metadata-publication-authority", nil)
	authRequest.Header.Set("Authorization", "Bearer "+bearer)
	authRequest.Header.Set("Origin", "https://app.sidiora.test")
	principal, err := authority.AuthenticateRequest(ctx, authRequest)
	if err != nil || principal.Wallet != wallet {
		t.Fatalf("creator principal = %+v, %v", principal, err)
	}
	service := PublicationService{
		Media:      ConnectedMedia{Metadata: V2ContractHandlers{Store: store, Canonical: PostgresCanonicalSource{Schema: canonical, CanonicalStreamID: stream}}, Identity: authority},
		Operations: sharedauth.OperationAuthorizer{Ledger: sharedauth.Ledger{Pool: pool, Environment: "paxeer125-production", AuditKey: []byte(strings.Repeat("a", 32))}, Verifier: ReportedCommittedProofVerifier{}},
		Identities: SessionCheckIdentities{Authority: sharedauth.HTTPStreamAuthority(authority)},
	}
	operation, draft, requestID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	metadata := []byte(`{"expectedRevision":"0","displayName":"Published name","displaySymbol":"PUB","description":"A public description","tags":["art"],"socialLinks":[{"label":"Site","url":"https://example.com"}]}`)
	body, err := json.Marshal(PublicationBody{DraftID: draft, OperationID: operation, CreationTxHash: creation.TxHash, Metadata: metadata, UploadRefs: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	path := "/v3/tokens/" + creation.Token + "/metadata-publications"
	router := chi.NewRouter()
	service.RegisterRoutes(router)
	unavailableRouter := chi.NewRouter()
	(PublicationService{}).RegisterRoutes(unavailableRouter)
	for _, route := range []string{"/v3/metadata-publication-authority", path + "/" + operation} {
		response := httptest.NewRecorder()
		unavailableRouter.ServeHTTP(response, httptest.NewRequest(http.MethodGet, route, nil))
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("unavailable publication route %s = %d %s", route, response.Code, response.Body.String())
		}
	}
	unavailableAdmission := httptest.NewRecorder()
	unavailableRouter.ServeHTTP(unavailableAdmission, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body)))
	if unavailableAdmission.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable admission = %d %s", unavailableAdmission.Code, unavailableAdmission.Body.String())
	}
	request := func(method, route, contentType, session, idempotency, requestID string, payload []byte) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, route, bytes.NewReader(payload))
		if contentType != "" {
			r.Header.Set("Content-Type", contentType)
		}
		if session != "" {
			r.Header.Set("Authorization", "Bearer "+session)
			r.Header.Set("Origin", "https://app.sidiora.test")
		}
		if idempotency != "" {
			r.Header.Set("Idempotency-Key", idempotency)
		}
		if requestID != "" {
			r.Header.Set("X-Request-ID", requestID)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, r)
		return response
	}
	if response := request(http.MethodGet, "/v3/metadata-publication-authority", "", bearer, "", "", nil); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"environment":"paxeer125-production"`) {
		t.Fatalf("publication authority = %d %s", response.Code, response.Body.String())
	}
	if response := request(http.MethodGet, "/v3/metadata-publication-authority", "", "", "", "", nil); response.Code != http.StatusForbidden {
		t.Fatalf("unauthenticated authority = %d %s", response.Code, response.Body.String())
	}
	for _, tc := range []struct {
		name, contentType, session, idempotency, requestID string
		payload                                            []byte
		want                                               int
	}{
		{"unsupported content type", "text/plain", bearer, operation, requestID, body, http.StatusUnsupportedMediaType},
		{"oversized body", "application/json", bearer, operation, requestID, bytes.Repeat([]byte("x"), PublicationBodyLimit+1), http.StatusUnprocessableEntity},
		{"invalid body", "application/json", bearer, operation, requestID, []byte(`{}`), http.StatusUnprocessableEntity},
		{"missing operation key", "application/json", bearer, "", requestID, body, http.StatusUnprocessableEntity},
		{"missing request id", "application/json", bearer, operation, "", body, http.StatusUnprocessableEntity},
		{"unauthenticated creator", "application/json", "", operation, requestID, body, http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := request(http.MethodPost, path, tc.contentType, tc.session, tc.idempotency, tc.requestID, tc.payload)
			if response.Code != tc.want {
				t.Fatalf("publication admission = %d %s, want %d", response.Code, response.Body.String(), tc.want)
			}
		})
	}
	if response := request(http.MethodGet, path+"/invalid", "", bearer, "", "", nil); response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid publication status = %d %s", response.Code, response.Body.String())
	}
	if response := request(http.MethodGet, path+"/"+uuid.NewString(), "", bearer, "", "", nil); response.Code != http.StatusNotFound {
		t.Fatalf("unknown publication status = %d %s", response.Code, response.Body.String())
	}
	issued := time.Now().Unix()
	envelope := sharedauth.OperationEnvelope{Version: 2, ChainID: "125", Environment: "paxeer125-production", Actor: wallet, Action: PublicationAction, Method: http.MethodPost, Path: path, BodyHash: publicationHash(body), Nonce: strings.Repeat("a", 64), IssuedAt: issued, ExpiresAt: issued + 300}
	statement, err := envelope.Statement()
	if err != nil {
		t.Fatal(err)
	}
	envelopeBytes, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	envelopeHash := publicationHash([]byte(statement))
	admission := []byte(`{"status":"metadata_pending"}`)
	_, err = pool.Exec(ctx, "INSERT INTO "+store.table("operation_ledger")+"(actor,action,nonce,envelope_digest,body_digest,request_id,response,response_digest) VALUES($1,$2,$3,$4,$5,$6,$7,$8)", wallet, PublicationAction, envelope.Nonce, envelopeHash, publicationHash(body), requestID, admission, publicationHash(admission))
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, "INSERT INTO "+store.table("security_audit_outbox")+"(request_id,actor_pseudonym,action,result,policy_version,request_digest) VALUES($1,$2,$3,'accepted','metadata-publication-v3',$4)", requestID, publicationHash([]byte(wallet)), PublicationAction, envelopeHash)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, "INSERT INTO "+store.table("metadata_publication_intents_v3")+`(operation,draft_id,token,pool,creator,creation_tx_hash,content_hash,body_hash,body,metadata,upload_refs,envelope,envelope_digest,nonce,request_id,account_id,session_id,auth_epoch,admission_response,accepted_at,expires_at)
		SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'[]',$11,$12,$13,$14,$15,$16,$17,$18,accepted.at,accepted.at+interval '30 minutes' FROM (SELECT clock_timestamp() AS at) accepted`, operation, draft, creation.Token, creation.Pool, wallet, creation.TxHash, publicationHash(metadata), publicationHash(body), body, metadata, envelopeBytes, envelopeHash, envelope.Nonce, requestID, principal.AccountID, principal.SessionID, principal.AuthEpoch, admission)
	if err != nil {
		t.Fatal(err)
	}
	owner := uuid.NewString()
	_, err = pool.Exec(ctx, "INSERT INTO "+store.table("metadata_publication_jobs_v3")+"(operation,state,attempt_count,fence,lease_owner,lease_until,grant_nonce,next_at,updated_at) VALUES($1,'publishing',1,1,$2,clock_timestamp()+interval '1 minute',$3,clock_timestamp(),clock_timestamp())", operation, owner, envelope.Nonce)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, "INSERT INTO "+store.table("metadata_publication_grants_v3")+"(operation,nonce,envelope,envelope_digest,body_hash,request_id,account_id,session_id,auth_epoch,response) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)", operation, envelope.Nonce, envelopeBytes, envelopeHash, publicationHash(body), requestID, principal.AccountID, principal.SessionID, principal.AuthEpoch, admission)
	if err != nil {
		t.Fatal(err)
	}
	lease := publicationLease{Operation: operation, Owner: owner, Fence: 1, Attempt: 1, Pool: creation.Pool}
	if err := service.applyPublication(ctx, lease); err != nil {
		t.Fatalf("publish stored creator intent: %v", err)
	}
	var state, revision string
	if err := pool.QueryRow(ctx, "SELECT state,revision FROM "+store.table("metadata_publication_jobs_v3")+" WHERE operation=$1", operation).Scan(&state, &revision); err != nil || state != "published" || revision != "1" {
		t.Fatalf("published job = %q, %q, %v", state, revision, err)
	}
	if response := request(http.MethodGet, path+"/"+operation, "", bearer, "", "", nil); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"published"`) {
		t.Fatalf("published status = %d %s", response.Code, response.Body.String())
	}
	if response := request(http.MethodGet, path+"/"+operation, "", "", "", "", nil); response.Code != http.StatusForbidden {
		t.Fatalf("unauthenticated publication status = %d %s", response.Code, response.Body.String())
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("metadata_publication_intents_v3")+" SET admission_response=$2 WHERE operation=$1", operation, []byte(`{`)); err != nil {
		t.Fatal(err)
	}
	if response := request(http.MethodGet, path+"/"+operation, "", bearer, "", "", nil); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("unrepresentable publication status = %d %s", response.Code, response.Body.String())
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("metadata_publication_intents_v3")+" SET admission_response=$2 WHERE operation=$1", operation, admission); err != nil {
		t.Fatal(err)
	}
	var published []byte
	if err := pool.QueryRow(ctx, "SELECT document FROM "+store.table("v2_documents")+" WHERE pool=$1", creation.Pool).Scan(&published); err != nil || !strings.Contains(string(published), `"symbol": "PUB"`) || !strings.Contains(string(published), `"url": "https://example.com"`) {
		t.Fatalf("published metadata = %s, %v", published, err)
	}
	var events int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+store.table("v2_outbox")+" WHERE payload->>'publicationOperation'=$1", operation).Scan(&events); err != nil || events != 1 {
		t.Fatalf("publication events = %d, %v", events, err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("metadata_publication_jobs_v3")+" SET state='publishing',lease_owner=$2,lease_until=clock_timestamp()+interval '1 minute' WHERE operation=$1", operation, owner); err != nil {
		t.Fatal(err)
	}
	if err := service.applyPublication(ctx, lease); err != nil {
		t.Fatalf("replay stored creator intent: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+store.table("v2_outbox")+" WHERE payload->>'publicationOperation'=$1", operation).Scan(&events); err != nil || events != 1 {
		t.Fatalf("replay events = %d, %v", events, err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("metadata_publication_jobs_v3")+" SET state='publishing',lease_owner=$2,lease_until=clock_timestamp()+interval '1 minute' WHERE operation=$1", operation, owner); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, table, column    string
		key, changed, restored any
		want                   error
	}{
		{"invalid stored body", store.table("metadata_publication_intents_v3"), "body", operation, []byte(`{}`), body, ErrValidation},
		{"invalid envelope encoding", store.table("metadata_publication_grants_v3"), "envelope", operation, []byte(`{`), envelopeBytes, ErrUnrepresentable},
		{"invalid envelope statement", store.table("metadata_publication_grants_v3"), "envelope", operation, []byte(`{}`), envelopeBytes, ErrUnrepresentable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			query := "UPDATE " + tc.table + " SET " + tc.column + "=$2 WHERE operation=$1"
			if _, err := pool.Exec(ctx, query, tc.key, tc.changed); err != nil {
				t.Fatal(err)
			}
			if err := service.applyPublication(ctx, lease); !errors.Is(err, tc.want) {
				t.Fatalf("stored publication input = %v, want %v", err, tc.want)
			}
			if _, err := pool.Exec(ctx, query, tc.key, tc.restored); err != nil {
				t.Fatal(err)
			}
		})
	}
	var originalResult []byte
	if err := pool.QueryRow(ctx, "SELECT result FROM "+store.table("v2_operations")+" WHERE operation=$1", operation).Scan(&originalResult); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("v2_operations")+" SET result='\"invalid\"'::jsonb WHERE operation=$1", operation); err != nil {
		t.Fatal(err)
	}
	if err := service.applyPublication(ctx, lease); !errors.Is(err, ErrUnrepresentable) {
		t.Fatalf("unrepresentable stored result = %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("v2_operations")+" SET result=$2::jsonb WHERE operation=$1", operation, string(originalResult)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("metadata_publication_jobs_v3")+" SET lease_until=clock_timestamp()-interval '1 second' WHERE operation=$1", operation); err != nil {
		t.Fatal(err)
	}
	if err := service.applyPublication(ctx, lease); err == nil || err.Error() != "publication_fence_lost" {
		t.Fatalf("expired publication lease = %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("metadata_publication_jobs_v3")+" SET lease_until=clock_timestamp()+interval '1 minute' WHERE operation=$1", operation); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("metadata_publication_jobs_v3")+" SET state='publishing',lease_owner=$2,lease_until=clock_timestamp()+interval '1 minute' WHERE operation=$1", operation, owner); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, change, restore string
	}{
		{"changed content hash", "UPDATE " + store.table("metadata_publication_intents_v3") + " SET content_hash=repeat('0',64) WHERE operation=$1", "UPDATE " + store.table("metadata_publication_intents_v3") + " SET content_hash=$2 WHERE operation=$1"},
		{"changed ledger digest", "UPDATE " + store.table("operation_ledger") + " SET envelope_digest=repeat('0',64) WHERE actor=$1", "UPDATE " + store.table("operation_ledger") + " SET envelope_digest=$2 WHERE actor=$1"},
		{"unaccepted audit", "UPDATE " + store.table("security_audit_outbox") + " SET result='rejected' WHERE request_id=$1", "UPDATE " + store.table("security_audit_outbox") + " SET result='accepted' WHERE request_id=$1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := operation
			var restored any = publicationHash(metadata)
			if tc.name == "changed ledger digest" {
				key, restored = wallet, envelopeHash
			}
			if tc.name == "unaccepted audit" {
				key = requestID
			}
			if _, err := pool.Exec(ctx, tc.change, key); err != nil {
				t.Fatal(err)
			}
			if err := service.applyPublication(ctx, lease); !errors.Is(err, ErrForbidden) {
				t.Fatalf("changed durable proof = %v", err)
			}
			if tc.name == "unaccepted audit" {
				_, err = pool.Exec(ctx, tc.restore, key)
			} else {
				_, err = pool.Exec(ctx, tc.restore, key, restored)
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("pools")+" SET creator=$2 WHERE token=$1", creation.Token, "0x"+strings.Repeat("b", 40)); err != nil {
		t.Fatal(err)
	}
	if err := service.applyPublication(ctx, lease); !errors.Is(err, ErrForbidden) {
		t.Fatalf("changed stored creator = %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("pools")+" SET creator=$2 WHERE token=$1", creation.Token, wallet); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO "+store.table("documents")+"(pool,blocked) VALUES($1,true)", creation.Pool); err != nil {
		t.Fatal(err)
	}
	if err := service.applyPublication(ctx, lease); !errors.Is(err, ErrForbidden) {
		t.Fatalf("blocked creator document = %v", err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM "+store.table("documents")+" WHERE pool=$1", creation.Pool); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"operation_ledger", "security_audit_outbox", "pools", "documents", "v2_operations"} {
		t.Run(table+" unavailable", func(t *testing.T) {
			if _, err := pool.Exec(ctx, "ALTER TABLE "+store.table(table)+" RENAME TO "+table+"_hidden"); err != nil {
				t.Fatal(err)
			}
			var databaseError *pgconn.PgError
			if err := service.applyPublication(ctx, lease); !errors.As(err, &databaseError) || databaseError.Code != "42P01" {
				t.Fatalf("unavailable %s = %v", table, err)
			}
			if _, err := pool.Exec(ctx, "ALTER TABLE "+store.table(table+"_hidden")+" RENAME TO "+table); err != nil {
				t.Fatal(err)
			}
		})
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("metadata_publication_grants_v3")+" SET session_id=$2 WHERE operation=$1", operation, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if err := service.applyPublication(ctx, lease); !errors.Is(err, ErrForbidden) {
		t.Fatalf("changed creator session = %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("metadata_publication_grants_v3")+" SET session_id=$2 WHERE operation=$1", operation, principal.SessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("v2_operations")+" SET body_hash=$2 WHERE operation=$1", operation, strings.Repeat("0", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("metadata_publication_jobs_v3")+" SET state='publishing',lease_owner=$2,lease_until=clock_timestamp()+interval '1 minute' WHERE operation=$1", operation, owner); err != nil {
		t.Fatal(err)
	}
	if err := service.applyPublication(ctx, lease); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed operation record = %v", err)
	}
	service.Media.Identity = authority
	service.Media.Moderation = DevelopmentVisibility{}
	service.Operations.Ledger.Pool = pool
	service.Operations.Verifier = ReportedCommittedProofVerifier{}
	if processed, err := service.RunPublicationBatch(ctx, 0); processed != 0 || !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid publication batch limit = %d, %v", processed, err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("metadata_publication_intents_v3")+" SET body='{}' WHERE operation=$1", operation); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("metadata_publication_jobs_v3")+" SET state='metadata_pending',lease_owner=NULL,lease_until=NULL,next_at=clock_timestamp() WHERE operation=$1", operation); err != nil {
		t.Fatal(err)
	}
	if processed, err := service.RunPublicationBatch(ctx, 1); processed != 1 || err != nil {
		t.Fatalf("invalid stored publication batch = %d, %v", processed, err)
	}
	var finalState, finalCode string
	if err := pool.QueryRow(ctx, "SELECT state,last_error_code FROM "+store.table("metadata_publication_jobs_v3")+" WHERE operation=$1", operation).Scan(&finalState, &finalCode); err != nil || finalState != "rejected" || finalCode != "invalid_intent" {
		t.Fatalf("rejected stored intent = %q, %q, %v", finalState, finalCode, err)
	}
	if processed, err := service.RunPublicationBatch(ctx, 1); processed != 0 || err != nil {
		t.Fatalf("empty publication batch = %d, %v", processed, err)
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE "+store.table("metadata_publication_jobs_v3")+" RENAME TO metadata_publication_jobs_hidden"); err != nil {
		t.Fatal(err)
	}
	var databaseError *pgconn.PgError
	if processed, err := service.RunPublicationBatch(ctx, 1); processed != 0 || !errors.As(err, &databaseError) || databaseError.Code != "42P01" {
		t.Fatalf("unavailable publication queue = %d, %v", processed, err)
	}
}
