package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMountedVersionsRejectInsecureAndUnboundedSources(t *testing.T) {
	ctx := context.Background()
	record := VersionRecord{ID: "signing", Version: "v1", Audience: "launchpad", Activate: time.Now().Add(-time.Hour), Retire: time.Now().Add(time.Hour), Reference: "signing-key"}
	for _, mode := range []string{"configuration", "cancelled", "missing registry", "public registry", "oversize registry", "malformed registry", "invalid version", "invalid reference", "missing value", "short value", "oversize value"} {
		t.Run(mode, func(t *testing.T) {
			directory := t.TempDir()
			source := MountedVersions{Directory: directory, RegistryName: "versions.json"}
			candidate := record
			callCtx := ctx
			switch mode {
			case "configuration":
				source.Directory = "relative"
			case "cancelled":
				bounded, stop := context.WithCancel(ctx)
				stop()
				callCtx = bounded
			case "invalid version":
				candidate.Audience = ""
			case "invalid reference":
				candidate.Reference = "../outside"
			}
			wire, err := json.Marshal([]VersionRecord{candidate})
			if err != nil {
				t.Fatal(err)
			}
			if mode == "oversize registry" {
				wire = []byte(strings.Repeat(" ", 65537))
			}
			if mode == "malformed registry" {
				wire = []byte("{")
			}
			path := filepath.Join(directory, "versions.json")
			if err := os.WriteFile(path, wire, 0600); err != nil {
				t.Fatal(err)
			}
			if mode == "missing registry" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "public registry" {
				if err := os.Chmod(path, 0644); err != nil {
					t.Fatal(err)
				}
			}
			value := strings.Repeat("a", 32)
			if mode == "short value" {
				value = "short"
			}
			if mode == "oversize value" {
				value = strings.Repeat("a", 4097)
			}
			if mode != "missing value" {
				if err := os.WriteFile(filepath.Join(directory, "signing-key"), []byte(value), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := source.Load(callCtx); err == nil {
				t.Fatal("unbound mounted source admitted")
			}
		})
	}
	const name = "SOURCE_BOUNDARY_VALUE"
	t.Setenv(name, strings.Repeat("a", 32))
	provider := newCached(EnvProvider{}, time.Hour, nil)
	for i := 0; i < 1024; i++ {
		provider.cache[fmt.Sprint(i)] = cacheEntry{value: "value", expiresAt: time.Now().Add(time.Hour)}
	}
	value, ok, err := provider.Get(ctx, name)
	if err != nil || !ok || value != strings.Repeat("a", 32) || len(provider.cache) != 1024 {
		t.Fatal("bounded secret cache changed value")
	}
	for key, entry := range provider.cache {
		entry.expiresAt = time.Now().Add(-time.Second)
		provider.cache[key] = entry
	}
	if _, ok, err := provider.Get(ctx, name); err != nil || !ok || len(provider.cache) != 1 {
		t.Fatal("expired secret cache entries retained")
	}
	if _, _, err := provider.Get(ctx, ""); err == nil {
		t.Fatal("empty cached reference admitted")
	}
	cancelled, stop := context.WithCancel(ctx)
	stop()
	if _, _, err := (EnvProvider{}).Get(cancelled, name); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := (EnvProvider{}).GetRequired(ctx, ""); err == nil {
		t.Fatal("empty secret reference admitted")
	}
	if _, err := LoadProduction(ctx, ProviderEnv, time.Second, []string{"invalid-name"}); err == nil {
		t.Fatal("invalid production reference admitted")
	}
	if _, err := LoadProduction(ctx, ProviderEnv, time.Second, []string{name, name}); err == nil {
		t.Fatal("duplicate production reference admitted")
	}
	if _, err := LoadProduction(ctx, ProviderEnv, time.Second, []string{"SOURCE_BOUNDARY_ABSENT"}); err == nil {
		t.Fatal("missing production reference admitted")
	}
	var startup *Startup
	if err := startup.Ready(ctx); err == nil {
		t.Fatal("uninitialized secret inventory ready")
	}
	const registry = "SOURCE_BOUNDARY_REGISTRY"
	record.Reference = name
	wire, err := json.Marshal([]VersionRecord{record})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(registry, string(wire))
	source := EnvironmentVersions{Registry: registry, Provider: provider}
	if _, err := source.Load(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	denied := errors.New("rotation application rejected")
	if _, err := NewRotation(ctx, source, time.Second, func([]VersionedValue) error { return denied }); !errors.Is(err, denied) {
		t.Fatalf("rotation application: %v", err)
	}
	for _, wire := range []string{`null`, `[null]`, `[{}] {}`, `[{"id":"a","id":"b"}]`} {
		var records []VersionRecord
		if err := decodeVersionRecords([]byte(wire), &records); err == nil {
			t.Fatalf("ambiguous version inventory admitted %s", wire)
		}
	}
}
