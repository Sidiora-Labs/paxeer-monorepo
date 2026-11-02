package social

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/interactions"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/voice"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/migrations"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type v2VoicePostgresPools struct{ db *pgxpool.Pool }

func (p v2VoicePostgresPools) Exists(ctx context.Context, pool string) (bool, error) {
	var exists bool
	err := p.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM indexer_canonical_creations_v1 WHERE pool=$1)`, pool).Scan(&exists)
	return exists, err
}

func TestPostgresV2VoiceCreateAndRead(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("L38_POSTGRES_DSN")
	if dsn == "" {
		dsn = "postgres://kindle_fixture:kindle_fixture_password@127.0.0.1:25432/kindle_fixture?sslmode=disable"
	}
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	schema := "voice_v2_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	ident := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+ident); err != nil {
		t.Fatalf("create PostgreSQL voice fixture (L38_POSTGRES_DSN): %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+ident+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	cfg.MaxConns = 4
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	ddl, err := migrations.FS.ReadFile("00003_safety.sql")
	if err != nil {
		t.Fatal(err)
	}
	up := strings.SplitN(string(ddl), "-- +goose Down", 2)[0]
	if _, err = db.Exec(ctx, strings.ReplaceAll(up, "chat.", ident+".")); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `
CREATE TABLE identity_wallet_links_v2(account_id uuid NOT NULL,wallet text NOT NULL,chain_id bigint NOT NULL,state text NOT NULL);
CREATE TABLE account_state_v2(account_id uuid PRIMARY KEY,wallet text NOT NULL,deleted boolean NOT NULL);
CREATE TABLE indexer_canonical_creations_v1(pool text PRIMARY KEY,token text NOT NULL,creator text NOT NULL);`); err != nil {
		t.Fatal(err)
	}
	guardStart := strings.Index(interactions.RuntimeAuthoritySQL, "CREATE FUNCTION chat.social_account_for_wallet(")
	if guardStart < 0 {
		t.Fatal("production account guard is absent")
	}
	guard := strings.SplitN(interactions.RuntimeAuthoritySQL[guardStart:], ";", 2)[0] + ";"
	guard = strings.NewReplacer("chat.", ident+".", "users.", ident+".").Replace(guard)
	if _, err = db.Exec(ctx, guard); err != nil {
		t.Fatal(err)
	}
	const hostWallet = "0x1111111111111111111111111111111111111111"
	const guestWallet = "0x2222222222222222222222222222222222222222"
	const poolID = "0x3333333333333333333333333333333333333333"
	hostID := uuid.NewString()
	if _, err = db.Exec(ctx, `INSERT INTO account_state_v2 VALUES($1,$2,false)`, hostID, hostWallet); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO identity_wallet_links_v2 VALUES($1,$2,125,'verified')`, hostID, hostWallet); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO indexer_canonical_creations_v1 VALUES($1,$2,$3)`, poolID, guestWallet, hostWallet); err != nil {
		t.Fatal(err)
	}
	store := &safety.Store{DB: db}
	control := &voice.Control{DB: db, Policy: store, Pools: v2VoicePostgresPools{db: db}}
	router := chi.NewRouter()
	RegisterSafetyRoutes(router, store, control)
	host := safety.Principal{Wallet: hostWallet, SessionID: uuid.NewString(), Expires: time.Now().Add(time.Hour)}
	guest := safety.Principal{Wallet: guestWallet, SessionID: uuid.NewString(), Expires: time.Now().Add(time.Hour)}
	requestID := uuid.NewString()
	request := func(routes http.Handler, principal *safety.Principal, method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("X-Request-ID", requestID)
		req.Header.Set("Content-Type", "application/json")
		if principal != nil {
			req = req.WithContext(safety.WithPrincipal(req.Context(), *principal))
		}
		rec := httptest.NewRecorder()
		routes.ServeHTTP(rec, req)
		return rec
	}
	assertError := func(rec *httptest.ResponseRecorder, status int, code string) {
		t.Helper()
		if rec.Code != status {
			t.Fatalf("status = %d; want %d; body %s", rec.Code, status, rec.Body.String())
		}
		if err := v2.ValidateJSON("Error", rec.Body.Bytes()); err != nil {
			t.Fatalf("frozen Error: %v; body %s", err, rec.Body.String())
		}
		var got v2.Error
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Code != code || got.Retryable != (status == 503) || got.RequestId != requestID || rec.Header().Get("X-Request-ID") != requestID {
			t.Fatalf("error = %+v; headers %v", got, rec.Header())
		}
	}
	for _, visibility := range []string{"public", "private"} {
		t.Run(visibility, func(t *testing.T) {
			body := `{"scopeType":"pool","scopeId":"` + poolID + `","title":"Community room","visibility":"` + visibility + `"}`
			created := request(router, &host, http.MethodPost, "/v2/voice-rooms", body)
			if created.Code != http.StatusCreated {
				t.Fatalf("create status = %d; body %s", created.Code, created.Body.String())
			}
			if err := v2.ValidateJSON("VoiceCreateResponse", created.Body.Bytes()); err != nil {
				t.Fatalf("frozen VoiceCreateResponse: %v", err)
			}
			var create v2.VoiceCreateResponse
			if err := json.Unmarshal(created.Body.Bytes(), &create); err != nil {
				t.Fatal(err)
			}
			room := create.Data.Room
			if _, err := uuid.Parse(room.RoomId); err != nil {
				t.Fatal(err)
			}
			if room.HostAccountId != hostID || room.ScopeType != "pool" || room.ScopeId != poolID || room.Visibility != visibility || room.State != "scheduled" || room.Revision != "1" {
				t.Fatalf("room = %+v", room)
			}
			if create.Data.Capabilities != (v2.VoiceCapabilities{}) || create.RequestId != requestID || create.Revision == nil || *create.Revision != "1" || create.Quality != "complete" {
				t.Fatalf("create = %+v", create)
			}
			if created.Header().Get("X-Request-ID") != requestID || created.Header().Get("Cache-Control") != "private, no-store" || created.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("create headers = %v", created.Header())
			}
			persisted, err := control.Read(ctx, room.RoomId)
			if err != nil || persisted.HostWallet != hostWallet || persisted.PoolID != poolID || persisted.State != "draft" || persisted.Revision != 1 {
				t.Fatalf("persisted = %+v; error %v", persisted, err)
			}
			var metadata v2.VoiceCreateBody
			var raw []byte
			if err := db.QueryRow(ctx, `SELECT payload FROM safety_outbox WHERE kind='voice_room_created_v2' AND entity_id=$1`, room.RoomId).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(raw, &metadata); err != nil || metadata.Title != "Community room" || metadata.Visibility != visibility {
				t.Fatalf("metadata = %+v; error %v", metadata, err)
			}
			restarted := chi.NewRouter()
			RegisterSafetyRoutes(restarted, store, &voice.Control{DB: db, Policy: store, Pools: v2VoicePostgresPools{db: db}})
			assertRead := func(principal *safety.Principal, actions []string) {
				t.Helper()
				read := request(restarted, principal, http.MethodGet, "/v2/voice-rooms/"+room.RoomId, "")
				if read.Code != http.StatusOK {
					t.Fatalf("read status = %d; body %s", read.Code, read.Body.String())
				}
				if err := v2.ValidateJSON("VoiceReadResponse", read.Body.Bytes()); err != nil {
					t.Fatalf("frozen VoiceReadResponse: %v", err)
				}
				var got v2.VoiceReadResponse
				if err := json.Unmarshal(read.Body.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if got.Data.Room != room || !reflect.DeepEqual(got.Data.AllowedActions, actions) || got.RequestId != requestID || got.Revision == nil || *got.Revision != "1" {
					t.Fatalf("read = %+v", got)
				}
				for _, secret := range []string{hostWallet, persisted.StartKey, "providerSession", "participants"} {
					if strings.Contains(read.Body.String(), secret) {
						t.Fatalf("read exposed internal field %q", secret)
					}
				}
			}
			assertRead(&host, []string{})
			if visibility == "public" {
				assertRead(&guest, []string{})
			} else {
				assertError(request(restarted, &guest, http.MethodGet, "/v2/voice-rooms/"+room.RoomId, ""), 403, "SCOPE_DENIED")
			}
			if _, err := db.Exec(ctx, `INSERT INTO voice_participants(room_id,wallet,role) VALUES($1,$2,'listener')`, room.RoomId, guestWallet); err != nil {
				t.Fatal(err)
			}
			assertRead(&guest, []string{"leave"})
			if _, err := db.Exec(ctx, `UPDATE voice_participants SET revoked=true WHERE room_id=$1 AND wallet=$2`, room.RoomId, guestWallet); err != nil {
				t.Fatal(err)
			}
			if visibility == "private" {
				assertError(request(restarted, &guest, http.MethodGet, "/v2/voice-rooms/"+room.RoomId, ""), 403, "SCOPE_DENIED")
			} else {
				assertRead(&guest, []string{})
			}
			if _, err := db.Exec(ctx, `INSERT INTO safety_bans(id,wallet,scope,resource,actor) VALUES($1,$2,'pool',$3,$4)`, uuid.NewString(), guestWallet, poolID, hostWallet); err != nil {
				t.Fatal(err)
			}
			assertError(request(restarted, &guest, http.MethodGet, "/v2/voice-rooms/"+room.RoomId, ""), 403, "SCOPE_DENIED")
			if _, err := db.Exec(ctx, `UPDATE safety_bans SET revoked_at=now() WHERE wallet=$1`, guestWallet); err != nil {
				t.Fatal(err)
			}
		})
	}
	valid := `{"scopeType":"pool","scopeId":"` + poolID + `","title":"Room","visibility":"public"}`
	var before int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM voice_rooms`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	assertError(request(router, &host, http.MethodPost, "/v2/voice-rooms", strings.Replace(valid, `"pool"`, `"squad"`, 1)), 503, "DEPENDENCY_UNAVAILABLE")
	for _, body := range []string{"", "{}", "null", valid + " {}", strings.Replace(valid, `"public"`, `"unknown"`, 1), strings.Replace(valid, `"title":"Room"`, `"title":"Room","extra":true`, 1)} {
		assertError(request(router, &host, http.MethodPost, "/v2/voice-rooms", body), 400, "INVALID_REQUEST")
	}
	assertError(request(router, &host, http.MethodPost, "/v2/voice-rooms", strings.Replace(valid, poolID, "unknown-pool", 1)), 400, "INVALID_REQUEST")
	assertError(request(router, nil, http.MethodPost, "/v2/voice-rooms", valid), 403, "SCOPE_DENIED")
	assertError(request(router, nil, http.MethodGet, "/v2/voice-rooms/"+uuid.NewString(), ""), 403, "SCOPE_DENIED")
	assertError(request(router, &host, http.MethodGet, "/v2/voice-rooms/not-a-uuid", ""), 400, "INVALID_REQUEST")
	assertError(request(router, &host, http.MethodGet, "/v2/voice-rooms/"+uuid.NewString(), ""), 404, "NOT_FOUND")
	var after int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM voice_rooms`).Scan(&after); err != nil || after != before {
		t.Fatalf("rejected creates changed room count: before %d; after %d; error %v", before, after, err)
	}
	unavailable := chi.NewRouter()
	RegisterSafetyRoutes(unavailable, store, nil)
	assertError(request(unavailable, &host, http.MethodPost, "/v2/voice-rooms", valid), 503, "DEPENDENCY_UNAVAILABLE")
	assertError(request(unavailable, &host, http.MethodGet, "/v2/voice-rooms/"+uuid.NewString(), ""), 503, "DEPENDENCY_UNAVAILABLE")
}
