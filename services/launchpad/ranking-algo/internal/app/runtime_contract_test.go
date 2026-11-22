package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	sharedprocess "github.com/Sidiora-Technologies/KindleLaunch/shared/process"
	"github.com/google/uuid"
)

func TestNativeRuntimeRejectsInvalidMaterial(t *testing.T) {
	f := publicationRecoveryNewFixture(t)
	var binding config.RankingRuntimeBinding
	if err := config.ReadBoundDocument(f.cfg.RankingRuntimeFile, f.cfg.RankingRuntimeSHA256, &binding); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*testing.T, *config.Config, *config.RankingRuntimeBinding)
	}{
		{"secret provider", func(_ *testing.T, c *config.Config, _ *config.RankingRuntimeBinding) { c.SecretsProvider = "unknown" }},
		{"CA encoding", func(t *testing.T, _ *config.Config, b *config.RankingRuntimeBinding) {
			b.CAFile = filepath.Join(t.TempDir(), "ca.pem")
			b.CASHA256 = writeBound(t, b.CAFile, []byte("invalid certificate"))
		}},
		{"empty key reference", func(_ *testing.T, _ *config.Config, b *config.RankingRuntimeBinding) { b.ClientKeyReference = "" }},
		{"client key encoding", func(t *testing.T, _ *config.Config, b *config.RankingRuntimeBinding) {
			t.Setenv(b.ClientKeyReference, "invalid key")
		}},
		{"server key encoding", func(t *testing.T, _ *config.Config, b *config.RankingRuntimeBinding) {
			t.Setenv(b.ServerKeyReference, "invalid key")
		}},
		{"cursor encoding", func(t *testing.T, _ *config.Config, b *config.RankingRuntimeBinding) {
			t.Setenv(b.CursorKeyReference, "invalid encoding!")
		}},
		{"cursor upper bound", func(t *testing.T, _ *config.Config, b *config.RankingRuntimeBinding) {
			t.Setenv(b.CursorKeyReference, base64.RawURLEncoding.EncodeToString(make([]byte, 65)))
		}},
		{"cursor lower bound", func(t *testing.T, _ *config.Config, b *config.RankingRuntimeBinding) {
			t.Setenv(b.CursorKeyReference, base64.RawURLEncoding.EncodeToString(make([]byte, 31)))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, changed := f.cfg, binding
			tc.change(t, &cfg, &changed)
			raw, err := json.Marshal(changed)
			if err != nil {
				t.Fatal(err)
			}
			cfg.RankingRuntimeFile = filepath.Join(t.TempDir(), "runtime.json")
			cfg.RankingRuntimeSHA256 = writeBound(t, cfg.RankingRuntimeFile, raw)
			instance, err := newNativeApp(context.Background(), cfg, quietLogger(), f.app.pool, f.app.redis, sharedprocess.NewTracker(), nil, map[string]sharedhttp.Check{}, nil)
			if err == nil {
				instance.Close()
				t.Fatal("invalid runtime material accepted")
			}
			if instance != nil {
				t.Fatal("refused runtime returned an application")
			}
		})
	}
}

func TestRefreshDiagnosticsTrackRefusalAndRecovery(t *testing.T) {
	f := publicationRecoveryNewFixture(t)
	runtime := f.app.native
	var output bytes.Buffer
	runtime.logger = slog.New(slog.NewJSONHandler(&output, nil))
	ctx := context.Background()
	f.authority.Store(false)
	_, err := runtime.coordinator.Refresh(ctx, uuid.NewString(), f.cutoff, false)
	if !errors.Is(err, ranker.ErrUnavailable) {
		t.Fatalf("refresh without authority: %v", err)
	}
	runtime.reportRefresh(false, err)
	runtime.reportRefresh(false, err)
	f.authority.Store(true)
	if _, err = runtime.coordinator.Refresh(ctx, uuid.NewString(), f.cutoff, false); err != nil {
		t.Fatalf("refresh after authority recovery: %v", err)
	}
	runtime.reportRefresh(false, nil)
	runtime.reportRefresh(false, nil)
	if _, err = f.app.pool.Exec(ctx, `ALTER TABLE ranking.snapshots ADD CONSTRAINT new_category_available CHECK(category<>'new') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	_, err = runtime.coordinator.Refresh(ctx, uuid.NewString(), f.cutoff, true)
	step, class, ok := ranker.RefreshRefusalDetails(err)
	if !ok || step != "completion" || class != "sink" {
		t.Fatalf("completion refusal = %s %s %v: %v", step, class, ok, err)
	}
	runtime.reportRefresh(true, err)
	runtime.reportRefresh(true, err)
	if _, err = f.app.pool.Exec(ctx, `ALTER TABLE ranking.snapshots DROP CONSTRAINT new_category_available`); err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.coordinator.Refresh(ctx, uuid.NewString(), f.cutoff, true); err != nil {
		t.Fatalf("refresh after completion recovery: %v", err)
	}
	runtime.reportRefresh(true, nil)
	if strings.Count(output.String(), `"msg":"ranking refresh refused"`) != 2 || strings.Count(output.String(), `"msg":"ranking refresh recovered"`) != 2 {
		t.Fatalf("refresh transitions: %s", output.String())
	}
	for _, fields := range []string{`"run":"all","step":"unknown","class":"unavailable"`, `"run":"new","step":"completion","class":"sink"`} {
		if !strings.Contains(output.String(), fields) {
			t.Fatalf("missing refusal fields %s: %s", fields, output.String())
		}
	}
}

func TestNativeReadinessRefusesRotatedSecretsAndCanceledReads(t *testing.T) {
	f := publicationRecoveryNewFixture(t)
	runtime := f.app.native
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runtime.ready(ctx); err == nil || !strings.HasPrefix(err.Error(), "ranking secret unavailable or rotated: ") {
		t.Fatalf("canceled readiness: %v", err)
	}
	var binding config.RankingRuntimeBinding
	if err := config.ReadBoundDocument(f.cfg.RankingRuntimeFile, f.cfg.RankingRuntimeSHA256, &binding); err != nil {
		t.Fatal(err)
	}
	t.Setenv(binding.CursorKeyReference, "rotated")
	if err := runtime.ready(context.Background()); err == nil || err.Error() != "ranking secret unavailable or rotated: "+binding.CursorKeyReference {
		t.Fatalf("rotated secret readiness: %v", err)
	}
}

func TestVisibilityTransportFailuresAreRefused(t *testing.T) {
	t.Run("invalid endpoint", func(t *testing.T) {
		reader := &rankingVisibilityClient{endpoint: ":invalid", client: http.DefaultClient, now: time.Now}
		if _, err := reader.ReadRankingVisibility(context.Background()); err == nil {
			t.Fatal("invalid visibility endpoint accepted")
		}
	})
	t.Run("incomplete response", func(t *testing.T) {
		peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Length", "100")
			_, _ = w.Write([]byte("{"))
		}))
		defer peer.Close()
		reader := &rankingVisibilityClient{endpoint: peer.URL, client: peer.Client(), now: time.Now}
		if _, err := reader.ReadRankingVisibility(context.Background()); !errors.Is(err, ranker.ErrUnavailable) {
			t.Fatalf("incomplete visibility response: %v", err)
		}
	})
}
