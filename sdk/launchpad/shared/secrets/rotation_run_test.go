package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type observedRotationSource struct {
	source     EnvironmentVersions
	loads      int
	beforeLoad func(context.Context)
}

func (s *observedRotationSource) Load(ctx context.Context) ([]VersionedValue, error) {
	s.loads++
	if s.beforeLoad != nil {
		s.beforeLoad(ctx)
	}
	return s.source.Load(ctx)
}

func rotationRunEnvironment(t *testing.T) *observedRotationSource {
	t.Helper()
	record := VersionRecord{
		ID: "audit", Version: "v1", Audience: "launchpad",
		Activate: time.Now().Add(-time.Hour), Retire: time.Now().Add(time.Hour),
		Reference: "ROTATION_RUN_VALUE",
	}
	wire, err := json.Marshal([]VersionRecord{record})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ROTATION_RUN_REGISTRY", string(wire))
	t.Setenv(record.Reference, strings.Repeat("a", 32))
	return &observedRotationSource{source: EnvironmentVersions{
		Registry: "ROTATION_RUN_REGISTRY", Provider: EnvProvider{},
	}}
}

func TestRotationRunCanceledFromApplyStaysReady(t *testing.T) {
	source := rotationRunEnvironment(t)
	for iteration := 0; iteration < 200; iteration++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		applies := 0
		rotation, err := NewRotation(ctx, source, time.Millisecond, func(values []VersionedValue) error {
			if len(values) != 1 || string(values[0].Value) != strings.Repeat("a", 32) {
				return errors.New("unexpected rotation value")
			}
			applies++
			if applies == 2 {
				cancel()
			}
			return nil
		})
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		loads := source.loads
		err = rotation.Run(ctx)
		cancel()
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("iteration %d: Run = %v", iteration, err)
		}
		if applies != 2 || source.loads != loads+1 {
			t.Fatalf("iteration %d: applies = %d, scheduled loads = %d", iteration, applies, source.loads-loads)
		}
		if err := rotation.Ready(); err != nil {
			t.Fatalf("iteration %d: Ready = %v", iteration, err)
		}
	}
}

func TestRotationRunCanceledBeforeFirstTickDoesNotLoad(t *testing.T) {
	source := rotationRunEnvironment(t)
	applies := 0
	rotation, err := NewRotation(context.Background(), source, time.Nanosecond, func([]VersionedValue) error {
		applies++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for iteration := 0; iteration < 200; iteration++ {
		if err := rotation.Run(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("Run = %v", err)
		}
	}
	if source.loads != 1 || applies != 1 {
		t.Fatalf("loads = %d, applies = %d", source.loads, applies)
	}
	if err := rotation.Ready(); err != nil {
		t.Fatalf("Ready = %v", err)
	}
}

func TestRotationRunRefreshCanceledDuringLoadPreservesReadiness(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		name := "ready"
		if unavailable {
			name = "unavailable"
		}
		t.Run(name, func(t *testing.T) {
			source := rotationRunEnvironment(t)
			applies := 0
			rotation, err := NewRotation(context.Background(), source, time.Millisecond, func([]VersionedValue) error {
				applies++
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if unavailable {
				t.Setenv("ROTATION_RUN_VALUE", "")
				if err := rotation.Refresh(context.Background()); err == nil {
					t.Fatal("missing source value admitted")
				}
			}
			refreshed := rotation.refreshed
			if err := rotation.Ready(); (err != nil) != unavailable {
				t.Fatalf("initial Ready = %v, unavailable = %t", err, unavailable)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			reading := make(chan struct{})
			source.beforeLoad = func(loadCtx context.Context) {
				close(reading)
				<-loadCtx.Done()
			}
			done := make(chan error, 1)
			go func() { done <- rotation.Refresh(ctx) }()
			select {
			case <-reading:
				cancel()
			case <-ctx.Done():
				t.Fatal("source read did not start")
			}
			if err := <-done; !errors.Is(err, context.Canceled) {
				t.Fatalf("Refresh = %v", err)
			}
			if !rotation.refreshed.Equal(refreshed) || rotation.unavailable != unavailable || applies != 1 {
				t.Fatalf("refreshed = %v, unavailable = %t, applies = %d", rotation.refreshed, rotation.unavailable, applies)
			}
			if err := rotation.Ready(); (err != nil) != unavailable {
				t.Fatalf("Ready = %v, unavailable = %t", err, unavailable)
			}
		})
	}
}

func TestRotationRunRefreshSourceErrorMakesUnavailable(t *testing.T) {
	source := rotationRunEnvironment(t)
	ctx := context.Background()
	applies := 0
	rotation, err := NewRotation(ctx, source, time.Millisecond, func([]VersionedValue) error {
		applies++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	refreshed := rotation.refreshed
	t.Setenv("ROTATION_RUN_VALUE", "")
	if err := rotation.Refresh(ctx); err == nil || ctx.Err() != nil {
		t.Fatalf("Refresh = %v, caller context = %v", err, ctx.Err())
	}
	if !rotation.refreshed.Equal(refreshed) || !rotation.unavailable || applies != 1 {
		t.Fatalf("refreshed = %v, unavailable = %t, applies = %d", rotation.refreshed, rotation.unavailable, applies)
	}
	if err := rotation.Ready(); err == nil {
		t.Fatal("source error retained readiness")
	}
}

func TestRotationRunRefreshCanceledDuringApplyPreservesReadiness(t *testing.T) {
	source := rotationRunEnvironment(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	applies := 0
	rotation, err := NewRotation(ctx, source, time.Millisecond, func(values []VersionedValue) error {
		if len(values) != 1 || string(values[0].Value) != strings.Repeat("a", 32) {
			return errors.New("unexpected rotation value")
		}
		applies++
		if applies == 2 {
			cancel()
			return ctx.Err()
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	refreshed := rotation.refreshed
	if err := rotation.Refresh(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Refresh = %v", err)
	}
	if !rotation.refreshed.Equal(refreshed) || rotation.unavailable || applies != 2 {
		t.Fatalf("refreshed = %v, unavailable = %t, applies = %d", rotation.refreshed, rotation.unavailable, applies)
	}
	if err := rotation.Ready(); err != nil {
		t.Fatalf("Ready = %v", err)
	}
}
