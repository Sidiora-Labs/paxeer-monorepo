package app

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/config"
	sharedsecrets "github.com/Sidiora-Technologies/KindleLaunch/shared/secrets"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func connectedControlRolePools(t *testing.T) (*pgxpool.Pool, *pgxpool.Pool, *pgxpool.Pool) {
	dsn := os.Getenv("L41_POSTGRES_URL")
	if dsn == "" {
		t.Fatal("gateway database fixture is required")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	for _, role := range []string{"identity_runtime", "control_runtime"} {
		password := uuid.NewString()
		if _, err := admin.Exec(ctx, "CREATE ROLE "+role+" LOGIN PASSWORD '"+password+"'"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := admin.Exec(context.Background(), "DROP OWNED BY "+role); err != nil {
				t.Error(err)
			}
			if _, err := admin.Exec(context.Background(), "DROP ROLE "+role); err != nil {
				t.Error(err)
			}
		})
		endpoint, err := url.Parse(dsn)
		if err != nil {
			t.Fatal(err)
		}
		endpoint.User = url.UserPassword(role, password)
		if role == "control_runtime" {
			t.Setenv("GATEWAY_TEST_CONTROL_ROLE_DSN", endpoint.String())
		} else {
			t.Setenv("GATEWAY_TEST_IDENTITY_ROLE_DSN", endpoint.String())
		}
	}
	for _, statement := range []string{
		"CREATE SCHEMA pnl",
		"CREATE TABLE pnl.authority_credentials(credential_id uuid PRIMARY KEY)",
		"CREATE VIEW pnl.authority_credentials_v2 AS SELECT credential_id FROM pnl.authority_credentials",
		"CREATE VIEW pnl.identity_control_wallets_v2 AS SELECT NULL::uuid AS account_id, NULL::text AS wallet, 125::integer AS chain_id, 'verified'::text AS state WHERE false",
		"GRANT USAGE ON SCHEMA pnl TO identity_runtime,control_runtime",
		"GRANT SELECT ON pnl.authority_credentials_v2,pnl.identity_control_wallets_v2 TO identity_runtime",
	} {
		if _, err := admin.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA pnl CASCADE"); err != nil {
			t.Error(err)
		}
	})
	identityPool, err := pgxpool.New(ctx, os.Getenv("GATEWAY_TEST_IDENTITY_ROLE_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(identityPool.Close)
	controlPool, err := pgxpool.New(ctx, os.Getenv("GATEWAY_TEST_CONTROL_ROLE_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(controlPool.Close)
	return admin, identityPool, controlPool
}

func TestConnectedControlChecksDistinctDatabaseRoles(t *testing.T) {
	ctx := context.Background()
	_, identityPool, controlPool := connectedControlRolePools(t)
	provider, err := sharedsecrets.NewChecked(sharedsecrets.Options{Provider: "env"})
	if err != nil {
		t.Fatal(err)
	}
	runtime := &gatewayConnectedRuntime{binding: gatewayRuntimeBinding{Control: gatewayControlBinding{Schema: "pnl", AuthoritySchema: "pnl"}, Pools: config.RuntimePoolSplit{ControlCredentialReference: "GATEWAY_TEST_CONTROL_ROLE_DSN"}}}
	secret := func(reference string) (string, error) { return provider.GetRequired(ctx, reference) }
	t.Setenv("GATEWAY_TEST_REVOCATION_TOKEN", uuid.NewString()+uuid.NewString())
	revocationEndpoint := httptest.NewTLSServer(http.NotFoundHandler())
	defer revocationEndpoint.Close()
	for _, tc := range []struct {
		name, endpoint, reference string
	}{
		{"missing revocation credential", revocationEndpoint.URL, "GATEWAY_TEST_ABSENT_REVOCATION_TOKEN"},
		{"invalid revocation authority", "://invalid", "GATEWAY_TEST_REVOCATION_TOKEN"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := &gatewayConnectedRuntime{binding: runtime.binding}
			candidate.binding.Control.RevocationEndpoint = tc.endpoint
			candidate.binding.Control.RevocationTokenReference = tc.reference
			if err := candidate.configureControl(ctx, config.Config{}, identityPool, controlPool, nil, &tls.Config{MinVersion: tls.VersionTLS13}, secret); err == nil || candidate.control != nil {
				t.Fatalf("revocation authority: runtime=%v err=%v", candidate.control, err)
			}
		})
	}
	if err := runtime.configureControl(ctx, config.Config{}, identityPool, controlPool, nil, nil, secret); err != nil {
		t.Fatalf("distinct control roles: %v", err)
	}
	if runtime.control == nil || runtime.controlHandler == nil {
		t.Fatal("control authority handler is unavailable")
	}
	if err := runtime.control.Ready(ctx); err != nil {
		t.Fatalf("control readiness: %v", err)
	}
	public := httptest.NewRecorder()
	runtime.controlHandler.ServeHTTP(public, httptest.NewRequest(http.MethodGet, "/v2/control/caps/owned", nil))
	if public.Code != http.StatusOK || !strings.Contains(public.Body.String(), `"capability":"owned"`) {
		t.Fatalf("public control capability: status=%d body=%s", public.Code, public.Body.String())
	}
	for _, tc := range []struct {
		name          string
		authorization []string
		status        int
		code          string
	}{
		{"ambiguous authorization", []string{"Bearer first", "Bearer second"}, http.StatusBadRequest, "FIELD_INVALID"},
		{"unrecognized credential", []string{"Bearer invalid"}, http.StatusUnauthorized, "AUTH_REQUIRED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/v2/control/actions", strings.NewReader(`{}`))
			request.Header["Authorization"] = tc.authorization
			response := httptest.NewRecorder()
			runtime.controlHandler.ServeHTTP(response, request)
			if response.Code != tc.status || !strings.Contains(response.Body.String(), tc.code) {
				t.Fatalf("control authorization: status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
	if err := runtime.ready(ctx); err == nil || err.Error() != "identity broker unavailable" {
		t.Fatalf("broker readiness: %v", err)
	}
}
