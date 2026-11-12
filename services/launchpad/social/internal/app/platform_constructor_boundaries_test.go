package app

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/config"
)

func TestPlatformConstructionRejectsInvalidRuntimeBindings(t *testing.T) {
	h := newPlatformHarness(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, tc := range []struct {
		name   string
		change func(*testing.T, *config.Config)
		want   string
	}{
		{"identity endpoint", func(_ *testing.T, c *config.Config) { c.IdentityURL = "http://invalid" }, "identity"},
		{"identity certificate", func(_ *testing.T, c *config.Config) { c.IdentityMTLSClientCertFile = "/missing/social-identity.pem" }, "identity"},
		{"private certificate", func(_ *testing.T, c *config.Config) { c.PrivateTLSCertFile = "/missing/social-private.pem" }, "tls"},
		{"cursor authority", func(t *testing.T, _ *config.Config) { t.Setenv("SOCIAL_CURSOR_KEY", "invalid") }, "cursor"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := platformConfig(h, h.db)
			tc.change(t, &cfg)
			app, err := New(context.Background(), cfg, logger)
			if app != nil {
				app.Close()
				t.Fatal("invalid runtime binding constructed a service")
			}
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), tc.want) {
				t.Fatalf("runtime binding error = %v; want %s", err, tc.want)
			}
		})
	}
}
