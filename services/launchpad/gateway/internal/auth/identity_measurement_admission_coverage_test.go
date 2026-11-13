package auth

import (
	"context"
	"testing"
	"time"
)

func TestIdentityMeasurementRecordsBoundedGeneratorAdmission(t *testing.T) {
	_, cfg := measurementFixtureConfig(t)
	cfg.Phases = []IdentityMeasurementPhase{{Name: "admission", SignInRefreshRPS: 3000, DynamicBudgetRPS: 20000, WriteBudgetRPS: 6000, Duration: time.Second}}
	cfg.Workers = 1
	cfg.QueueCapacity = 1
	cfg.MaxAcknowledgements = 1500
	cfg.RequestTimeout = 2 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	summary, err := RunIdentityMeasurement(ctx, cfg)
	window := summary.PhaseWindows["admission"]
	if err == nil || window == nil || window.OfferedJourneys != 1500 || window.EnqueuedJourneys >= window.OfferedJourneys || summary.GeneratorRejected == 0 || summary.GeneratorQueueCapacity != 1 || summary.FinalAcceptance {
		t.Fatalf("bounded admission: window=%+v rejected=%d capacity=%d accepted=%t err=%v", window, summary.GeneratorRejected, summary.GeneratorQueueCapacity, summary.FinalAcceptance, err)
	}
}
