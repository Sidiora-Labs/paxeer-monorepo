package config

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	sharedconfig "github.com/Sidiora-Technologies/KindleLaunch/shared/config"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
)

func TestSecretsCheckContext(t *testing.T) {
	features := ChartRuntimeFeatures{SchemaVersion: 2, EndpointSetVersion: "main", DeploymentVersion: deployment.Current().DeploymentVersion, AnchorHeight: 1, AnchorHash: "0x" + strings.Repeat("a", 64), CanonicalAPIOrigin: "http://localhost", ReplayGatewayOrigin: "http://localhost", CanonicalSeriesModel: "exact-v1", DisplayPrecision: 6, MaxHistoryOrigins: 2, RollupBatch: 4, RollupIntervalMillis: 1000}
	raw, err := json.Marshal(features)
	if err != nil {
		t.Fatal(err)
	}
	base := Config{BaseEnv: sharedconfig.BaseEnv{NodeEnv: nodeEnvironmentProduction, SecretsProvider: "env", SecretsRefreshSeconds: 1}, FeaturesConfigDocument: base64.StdEncoding.EncodeToString(raw), FeaturesConfigSHA256: fmt.Sprintf("%x", sha256.Sum256(raw)), NATSCredentials: "configured"}
	for _, key := range []string{"DATABASE_URL", "REDIS_URL", "RPC_URL", "WEBHOOK_HMAC_SECRET", "NATS_URL", "NATS_CREDENTIALS"} {
		t.Setenv(key, key+"-initial")
	}
	newCheck := func() (func(context.Context) error, context.CancelFunc) {
		cfg := base
		check, stop, err := cfg.ResolveProductionSecrets(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		return check, stop
	}
	check, stop := newCheck()
	defer stop()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := check(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled check = %v", err)
	}
	deadline, expire := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer expire()
	if err := check(deadline); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expired check = %v", err)
	}
	if err := check(context.Background()); err != nil {
		t.Fatalf("live check after ended contexts = %v", err)
	}
	t.Setenv("DATABASE_URL", "DATABASE_URL-rotated")
	if err := check(context.Background()); err == nil || err.Error() != "required_secret_rotated_restart_required: DATABASE_URL" {
		t.Fatalf("rotation check = %v", err)
	}
	canceled, cancel = context.WithCancel(context.Background())
	cancel()
	if err := check(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("ended rotation check = %v", err)
	}
	t.Setenv("DATABASE_URL", "DATABASE_URL-initial")
	if err := check(context.Background()); err == nil || err.Error() != "required_secret_rotated_restart_required: DATABASE_URL" {
		t.Fatalf("retained rotation check = %v", err)
	}
	missing, missingStop := newCheck()
	defer missingStop()
	t.Setenv("REDIS_URL", "")
	if err := missing(context.Background()); err == nil || err.Error() != "required_secret_unavailable: REDIS_URL" {
		t.Fatalf("missing check = %v", err)
	}
	deadline, expire = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer expire()
	if err := missing(deadline); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("ended missing check = %v", err)
	}
	t.Setenv("REDIS_URL", "REDIS_URL-initial")
	if err := missing(context.Background()); err == nil || err.Error() != "required_secret_unavailable: REDIS_URL" {
		t.Fatalf("retained missing check = %v", err)
	}
}
