package cache

import (
	"errors"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
)

func TestAccountingBindingsCutoverMovesBothRoutesTogether(t *testing.T) {
	runtime := repairM992Runtime(t, repairM992Config(t, 0))
	b := runtime.bindings
	for _, tc := range []struct {
		name, expected, next, version string
		model                         *store.AccountingReadModel
	}{
		{"missing model", "g1", "g2", "v3", nil},
		{"unchanged generation", "g1", "g1", "v3", &store.AccountingReadModel{}},
		{"missing version", "g1", "g2", "", &store.AccountingReadModel{}},
		{"unexpected generation", "other", "g2", "v3", &store.AccountingReadModel{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := b.Cutover(tc.expected, tc.next, tc.version, tc.model); !errors.Is(err, ErrProjectionChanged) {
				t.Fatalf("invalid cutover: %v", err)
			}
			for _, route := range []string{literalAccountingCandles, literalAccountingTrades} {
				policy, err := runtime.adapter.policy(route)
				if err != nil || policy.Generation != "g1" {
					t.Fatalf("%s changed after invalid cutover: %+v, %v", route, policy, err)
				}
			}
		})
	}
	nextModel := &store.AccountingReadModel{}
	if err := b.Cutover("g1", "g2", "v3", nextModel); err != nil {
		t.Fatal(err)
	}
	if b.config.Generation != "g2" || b.config.ProjectionVersion != "v3" || b.config.Model != nextModel {
		t.Fatalf("binding cutover: %+v", b.config)
	}
	for _, route := range []string{literalAccountingCandles, literalAccountingTrades} {
		policy, err := runtime.adapter.policy(route)
		if err != nil || policy.Generation != "g2" {
			t.Fatalf("%s generation after cutover: %+v, %v", route, policy, err)
		}
	}
	if err := b.Cutover("g1", "g3", "v4", nextModel); !errors.Is(err, ErrProjectionChanged) {
		t.Fatalf("old generation reused: %v", err)
	}
}
