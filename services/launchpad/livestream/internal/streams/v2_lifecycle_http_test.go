package streams

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresV2StreamStartEndAliasesUseExpectedRevision(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	config := s.pool.Config()
	config.ConnConfig.RuntimeParams["search_path"] = s.schema + ",pg_catalog"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	s.pool = pool
	if _, err = pool.Exec(ctx, sharedauth.LedgerDDL); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS chat;
CREATE TABLE IF NOT EXISTS chat.safety_bans(wallet text NOT NULL,revoked_at timestamptz,expires_at timestamptz,scope text NOT NULL,resource text);
CREATE OR REPLACE VIEW chat.safety_bans_v1 AS SELECT wallet,scope,resource,expires_at,revoked_at FROM chat.safety_bans`); err != nil {
		t.Fatal(err)
	}
	endpoint := os.Getenv("L34_ANVIL_URL")
	if endpoint == "" {
		t.Fatal("L34_ANVIL_URL is required for the real chain-125 operation verifier")
	}
	client, err := ethclient.DialContext(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	verifier := &sharedauth.WalletOperationVerifier{Backend: sharedauth.NewReportedHashBackend(client.Client())}
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	wallet := strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())
	h := New(Deps{
		Lifecycle:         s,
		OperationLedger:   &sharedauth.Ledger{Pool: pool, Environment: "test", AuditKey: []byte(strings.Repeat("a", 32))},
		OperationVerifier: verifier,
		Restrictions:      &RestrictionReader{Pool: pool},
		Authorize: func(ctx context.Context, r *http.Request, action string, o Operation) (Authorization, error) {
			raw, err := base64.RawURLEncoding.DecodeString(r.Header.Get("X-Kindle-Operation"))
			if err != nil {
				return Authorization{}, ErrDenied
			}
			var envelope sharedauth.OperationEnvelope
			if err = json.Unmarshal(raw, &envelope); err != nil {
				return Authorization{}, ErrDenied
			}
			statement, err := envelope.Statement()
			if err != nil {
				return Authorization{}, ErrDenied
			}
			if envelope.Action != action || envelope.Actor != o.Wallet {
				return Authorization{}, ErrDenied
			}
			if err = verifier.Verify(ctx, envelope.Actor, statement, r.Header.Get("X-Kindle-Signature")); err != nil {
				return Authorization{}, err
			}
			return Authorization{Wallet: envelope.Actor, Pool: o.Pool, Allowed: true, ExpiresAt: time.Unix(envelope.ExpiresAt, 0)}, nil
		},
	})
	router := chi.NewRouter()
	h.RegisterRoutes(router)
	post := func(target, body string, signed bool, envelope sharedauth.OperationEnvelope, signature string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if signed {
			raw, err := json.Marshal(envelope)
			if err != nil {
				t.Fatal(err)
			}
			r.Header.Set("X-Kindle-Operation", base64.RawURLEncoding.EncodeToString(raw))
			r.Header.Set("X-Kindle-Signature", signature)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	sign := func(target, action, body string) (sharedauth.OperationEnvelope, string) {
		digest, err := sharedauth.BodyDigest([]byte(body))
		if err != nil {
			t.Fatal(err)
		}
		nonce := make([]byte, 32)
		if _, err = rand.Read(nonce); err != nil {
			t.Fatal(err)
		}
		now := time.Now().Unix()
		envelope := sharedauth.OperationEnvelope{Version: 1, ChainID: "125", Environment: "test", Actor: wallet, Action: "stream." + action, Method: http.MethodPost, Path: target, BodyHash: digest, Nonce: fmt.Sprintf("%x", nonce), IssuedAt: now, ExpiresAt: now + 300}
		statement, err := envelope.Statement()
		if err != nil {
			t.Fatal(err)
		}
		signature, err := crypto.Sign(accounts.TextHash([]byte(statement)), key)
		if err != nil {
			t.Fatal(err)
		}
		return envelope, hexutil.Encode(signature)
	}
	checkError := func(w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("status = %d, want %d: %s", w.Code, status, w.Body.String())
		}
		if err := v2.ValidateJSON("Error", w.Body.Bytes()); err != nil {
			t.Fatal(err, w.Body.String())
		}
	}
	for _, frozen := range []bool{true, false} {
		t.Run(fmt.Sprintf("frozen_%t", frozen), func(t *testing.T) {
			poolAddress := randomAddress(t)
			o, err := s.Create(ctx, CreateIntent{Wallet: wallet, Pool: poolAddress, Title: "Lifecycle HTTP stream", IdempotencyKey: fmt.Sprintf("http-%t", frozen)}, CreatorDecision{Wallet: wallet, Pool: poolAddress, Allowed: true, ExpiresAt: time.Now().Add(time.Minute)}, true)
			if err != nil {
				t.Fatal(err)
			}
			providerID := "http_" + o.ID
			o, err = s.BindProvider(ctx, o.ID, providerID)
			if err != nil {
				t.Fatal(err)
			}
			for _, action := range []string{"start", "end"} {
				target := "/v2/streams/" + o.ID + "/" + action
				field := "revision"
				if action == "start" {
					target = "/v2/streams/" + o.ID + "/go-live"
				}
				if frozen {
					target, field = "/v2/streams/"+o.ID+":"+action, "expectedRevision"
					for _, invalid := range []string{`{"revision":"2"}`, `{}`, `{"expectedRevision":null}`, `{"expectedRevision":"02"}`, `{"expectedRevision":"2","revision":"2"}`} {
						checkError(post(target, invalid, false, sharedauth.OperationEnvelope{}, ""), http.StatusBadRequest)
					}
				}
				stale := fmt.Sprintf(`{%q:%q}`, field, strconv.FormatInt(o.Revision-1, 10))
				envelope, signature := sign(target, action, stale)
				checkError(post(target, stale, true, envelope, signature), http.StatusConflict)
				body := fmt.Sprintf(`{%q:%q}`, field, strconv.FormatInt(o.Revision, 10))
				checkError(post(target, body, false, sharedauth.OperationEnvelope{}, ""), http.StatusForbidden)
				envelope, signature = sign(target, action, body)
				response := post(target, body, true, envelope, signature)
				if response.Code != http.StatusAccepted || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Content-Type") != "application/json" {
					t.Fatal(response.Code, response.Header(), response.Body.String())
				}
				var operation V2Operation
				if frozen {
					schema := "StreamStartResponse"
					if action == "end" {
						schema = "StreamEndResponse"
					}
					if err = v2.ValidateJSON(schema, response.Body.Bytes()); err != nil {
						t.Fatal(err, response.Body.String())
					}
					var result struct {
						RequestID string `json:"requestId"`
						Revision  string `json:"revision"`
						Data      struct {
							Operation V2Operation `json:"operation"`
						} `json:"data"`
					}
					if err = json.Unmarshal(response.Body.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
					operation = result.Data.Operation
					if !frozenUUID.MatchString(result.RequestID) || result.Revision != operation.Revision {
						t.Fatal("invalid frozen response", response.Body.String())
					}
				} else {
					var result struct {
						Operation V2Operation `json:"operation"`
					}
					if err = json.Unmarshal(response.Body.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
					operation = result.Operation
				}
				wantState := "READY"
				if action == "end" {
					wantState = "ENDING"
				}
				if operation.StreamID != o.ID || operation.OperationID != o.ID || operation.Status != strings.ToLower(wantState) || operation.Revision != strconv.FormatInt(o.Revision+1, 10) {
					t.Fatal("unexpected lifecycle response", operation)
				}
				replay := post(target, body, true, envelope, signature)
				if replay.Code != http.StatusAccepted || replay.Body.String() != response.Body.String() {
					t.Fatal("signed replay changed response", replay.Code, replay.Body.String())
				}
				current, err := s.Get(ctx, o.ID)
				if err != nil || current.Revision != o.Revision+1 || current.State != wantState || current.Intent != action {
					t.Fatal("transition or replay changed state incorrectly", current, err)
				}
				var count int
				if err = pool.QueryRow(ctx, "SELECT count(*) FROM "+s.schema+".stream_outbox WHERE operation_id=$1 AND kind=$2", o.ID, action).Scan(&count); err != nil || count != 1 {
					t.Fatal("expected exactly one durable lifecycle action", count, err)
				}
				o = current
				if action == "start" {
					if outcome, err := s.Observe(ctx, ProviderEvent{EventID: "live_" + o.ID, ProviderID: providerID, State: "LIVE", ObservedAt: time.Now().UTC().Truncate(time.Microsecond)}); err != nil || outcome != "applied" {
						t.Fatal(outcome, err)
					}
					o, err = s.Get(ctx, o.ID)
					if err != nil || o.State != "LIVE" {
						t.Fatal(o, err)
					}
				} else {
					if outcome, err := s.Observe(ctx, ProviderEvent{EventID: "ended_" + o.ID, ProviderID: providerID, State: "ENDED", ObservedAt: time.Now().UTC().Truncate(time.Microsecond)}); err != nil || outcome != "applied" {
						t.Fatal(outcome, err)
					}
					ended, err := s.Get(ctx, o.ID)
					if err != nil || ended.State != "ENDED" {
						t.Fatal(ended, err)
					}
				}
			}
		})
	}
}

func TestV2StreamHeartbeatAliasRejectsConnectedBody(t *testing.T) {
	const streamID = "11111111-1111-4111-8111-111111111111"
	const sessionID = "22222222-2222-4222-8222-222222222222"
	target := "/v2/streams/" + streamID + "/viewers:heartbeat"
	router := chi.NewRouter()
	New(Deps{}).RegisterRoutes(router)
	for _, body := range []string{
		`{"sessionId":"` + sessionID + `","token":"connected"}`,
		`{"viewerToken":"token","playbackSessionId":"` + sessionID + `","token":"connected"}`,
		`{}`, `null`, `{"viewerToken":"token"}`, `{"playbackSessionId":"` + sessionID + `"}`,
		`{"viewerToken":"","playbackSessionId":"` + sessionID + `"}`,
		`{"viewerToken":null,"playbackSessionId":"` + sessionID + `"}`,
		`{"viewerToken":"token","playbackSessionId":"invalid"}`,
		`{"ViewerToken":"token","playbackSessionId":"` + sessionID + `"}`,
		`{"viewerToken":"one","viewerToken":"two","playbackSessionId":"` + sessionID + `"}`,
		`{"viewerToken":"` + strings.Repeat("t", 4097) + `","playbackSessionId":"` + sessionID + `"}`,
	} {
		t.Run(body, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest {
				t.Fatal("invalid frozen body admitted", w.Code, w.Body.String())
			}
			if err := v2.ValidateJSON("Error", w.Body.Bytes()); err != nil {
				t.Fatal(err, w.Body.String())
			}
		})
	}
	for _, token := range []string{"valid-shape", strings.Repeat("t", 4096)} {
		r := httptest.NewRequest(http.MethodPost, target, strings.NewReader(`{"viewerToken":"`+token+`","playbackSessionId":"`+sessionID+`"}`))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != http.StatusServiceUnavailable {
			t.Fatal("valid frozen body did not reach dependency check", w.Code, w.Body.String())
		}
		if err := v2.ValidateJSON("Error", w.Body.Bytes()); err != nil {
			t.Fatal(err, w.Body.String())
		}
	}
	readOnly := chi.NewRouter()
	New(Deps{BroadcastUnprovisioned: true}).RegisterRoutes(readOnly)
	for _, action := range []string{"start", "end"} {
		w := httptest.NewRecorder()
		readOnly.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v2/streams/"+streamID+":"+action, strings.NewReader(`{"expectedRevision":"1"}`)))
		var body v2.Error
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || w.Code != http.StatusConflict || body.Code != BroadcastNotProvisionedCode {
			t.Fatal("frozen mutation bypassed provisioned guard", w.Code, w.Body.String(), err)
		}
	}
}
