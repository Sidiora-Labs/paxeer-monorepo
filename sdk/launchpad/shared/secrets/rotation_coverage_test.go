package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestEnvironmentVersionRotationAppliesCurrentValue(t *testing.T) {
	const registry = "ROTATION_COVERAGE_REGISTRY"
	const reference = "ROTATION_COVERAGE_VALUE"
	ctx := context.Background()
	provider, err := NewChecked(Options{Provider: "env"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (EnvironmentVersions{Registry: "invalid-name", Provider: provider}).Load(ctx); err == nil {
		t.Fatal("invalid version registry name admitted")
	}
	if _, err := (EnvironmentVersions{Registry: registry, Provider: provider}).Load(ctx); err == nil {
		t.Fatal("missing version registry admitted")
	}
	record := VersionRecord{ID: "audit", Version: "v1", Audience: "launchpad", Activate: time.Now().Add(-time.Hour), Retire: time.Now().Add(time.Hour), Reference: reference}
	encode := func(records []VersionRecord) string {
		t.Helper()
		wire, err := json.Marshal(records)
		if err != nil {
			t.Fatal(err)
		}
		return string(wire)
	}
	t.Setenv(registry, encode([]VersionRecord{record}))
	t.Setenv(reference, strings.Repeat("a", 32))
	source := EnvironmentVersions{Registry: registry, Provider: provider}
	values, err := source.Load(ctx)
	if err != nil || len(values) != 1 || values[0].Record.ID != "audit" || string(values[0].Value) != strings.Repeat("a", 32) {
		t.Fatalf("loaded versioned value = %+v, %v", values, err)
	}
	if _, err := NewRotation(ctx, nil, time.Second, func([]VersionedValue) error { return nil }); err == nil {
		t.Fatal("rotation without a source admitted")
	}
	var applied []byte
	var stopRun context.CancelFunc
	rotation, err := NewRotation(ctx, source, 5*time.Millisecond, func(next []VersionedValue) error {
		if len(next) != 1 || next[0].Record.ID != "audit" {
			return errors.New("version identity changed")
		}
		applied = append([]byte(nil), next[0].Value...)
		if string(applied) == strings.Repeat("c", 32) && stopRun != nil {
			stopRun()
		}
		return nil
	})
	if err != nil || string(applied) != strings.Repeat("a", 32) || rotation.Ready() != nil {
		t.Fatalf("initial rotation = %q, %v", applied, err)
	}
	t.Setenv(reference, strings.Repeat("b", 32))
	if err := rotation.Refresh(ctx); err != nil || string(applied) != strings.Repeat("b", 32) || rotation.Ready() != nil {
		t.Fatalf("refreshed value = %q, %v", applied, err)
	}
	t.Setenv(reference, "")
	if err := rotation.Refresh(ctx); err == nil || rotation.Ready() == nil {
		t.Fatalf("unavailable current value retained readiness: %v", err)
	}
	t.Setenv(reference, strings.Repeat("c", 32))
	runCtx, cancel := context.WithTimeout(ctx, time.Second)
	stopRun = cancel
	defer cancel()
	if err := rotation.Run(runCtx); !errors.Is(err, context.Canceled) || string(applied) != strings.Repeat("c", 32) || rotation.Ready() != nil {
		t.Fatalf("scheduled refresh = %q, %v", applied, err)
	}
	for _, invalid := range [][]VersionRecord{{record, record}, {{ID: "audit", Version: "v2", Audience: "launchpad", Activate: record.Retire, Retire: record.Activate, Reference: reference}}} {
		t.Setenv(registry, encode(invalid))
		if _, err := source.Load(ctx); err == nil {
			t.Fatal("invalid version inventory admitted")
		}
	}
	t.Setenv(registry, `[{"id":"audit","id":"changed"}]`)
	if _, err := source.Load(ctx); err == nil {
		t.Fatal("duplicate version field admitted")
	}
}
