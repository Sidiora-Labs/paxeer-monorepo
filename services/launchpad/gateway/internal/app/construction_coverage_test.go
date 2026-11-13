package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/internaltest"
	sharedsecrets "github.com/Sidiora-Technologies/KindleLaunch/shared/secrets"
)

func TestGatewayConstructionValidatesBoundDependencies(t *testing.T) {
	_, redisURL := internaltest.NewRedis(t)
	minio := internaltest.NewMinIO(t, "gateway-construction")
	base := config.Config{
		RedisURL: redisURL, JWTSecret: "gateway-construction-signing-key", JWTTTLSeconds: 3600,
		NonceTTLSeconds: 300, AppDomain: "kindlelaunch", SocialHTTPURL: "https://localhost:4443",
		SocialWSURL: "wss://localhost:4443", MetadataUploadURL: "https://localhost:4443",
		UpstreamTimeoutSeconds: 5, S3Endpoint: minio.Endpoint, S3AccessKeyID: minio.AccessKeyID,
		S3SecretAccessKey: minio.SecretAccessKey, S3Region: "us-east-1", TokenBucket: "gateway-construction",
		ObjectCacheTTLSeconds: 30, ObjectCacheMaxBytes: 1024, MaxUploadBytes: 6 << 20,
		RateLimitMax: 100, RateLimitWindowSeconds: 60,
	}
	app, err := New(context.Background(), base, nil)
	if err != nil || app == nil || app.Router == nil {
		t.Fatalf("gateway construction: app=%v err=%v", app, err)
	}
	request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	response := httptest.NewRecorder()
	app.Router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("gateway live route: status=%d body=%s", response.Code, response.Body.String())
	}
	app.Close()
	user := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer user.Close()
	withUser := base
	withUser.UserHTTPURL = user.URL
	userGateway, err := New(context.Background(), withUser, nil)
	if err != nil || userGateway == nil {
		t.Fatalf("development user route construction: app=%v err=%v", userGateway, err)
	}
	defer userGateway.Close()
	response = httptest.NewRecorder()
	userGateway.Router.ServeHTTP(response, httptest.NewRequest(http.MethodPatch, "/v3/profiles/0x123", strings.NewReader(`{}`)))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated profile write: status=%d body=%s", response.Code, response.Body.String())
	}
	for _, tc := range []struct {
		name string
		edit func(*config.Config)
	}{
		{"redis authority", func(c *config.Config) { c.RedisURL = "not-a-redis-url" }},
		{"social HTTP authority", func(c *config.Config) { c.SocialHTTPURL = "://invalid" }},
		{"social socket authority", func(c *config.Config) { c.SocialWSURL = "://invalid" }},
		{"user authority", func(c *config.Config) { c.UserHTTPURL = "://invalid" }},
		{"object endpoint", func(c *config.Config) { c.S3Endpoint = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := base
			tc.edit(&candidate)
			constructed, err := New(context.Background(), candidate, nil)
			if constructed != nil || err == nil {
				if constructed != nil {
					constructed.Close()
				}
				t.Fatalf("invalid dependency admitted: app=%v err=%v", constructed, err)
			}
		})
	}
}

func TestConnectedControlRejectsUnboundCredentials(t *testing.T) {
	t.Setenv("GATEWAY_TEST_CONTROL_CREDENTIAL", "bound-control-credential")
	provider, err := sharedsecrets.NewChecked(sharedsecrets.Options{Provider: "env"})
	if err != nil {
		t.Fatal(err)
	}
	secret := func(reference string) (string, error) { return provider.GetRequired(context.Background(), reference) }
	for _, tc := range []struct {
		name    string
		control gatewayControlBinding
		want    string
	}{
		{"schema", gatewayControlBinding{Schema: "other", AuthoritySchema: "pnl"}, "exact control authority schemas required"},
		{"control credential", gatewayControlBinding{Schema: "pnl", AuthoritySchema: "pnl"}, ""},
		{"incident credential", gatewayControlBinding{Schema: "pnl", AuthoritySchema: "pnl", IncidentCredentialReference: "GATEWAY_TEST_INCIDENT"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			credentialReference := "GATEWAY_TEST_CONTROL_CREDENTIAL"
			if tc.name == "control credential" {
				credentialReference = "GATEWAY_TEST_MISSING_CONTROL_CREDENTIAL"
			}
			runtime := &gatewayConnectedRuntime{binding: gatewayRuntimeBinding{Control: tc.control, Pools: config.RuntimePoolSplit{ControlCredentialReference: credentialReference}}}
			err := runtime.configureControl(context.Background(), config.Config{}, nil, nil, nil, nil, secret)
			if err == nil || (tc.want != "" && !strings.Contains(err.Error(), tc.want)) || runtime.control != nil {
				t.Fatalf("control credential binding: err=%v control=%v", err, runtime.control)
			}
		})
	}
}
