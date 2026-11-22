package app

import (
	"reflect"
	"runtime"
	"testing"

	canonicalsearch "github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/search"
)

func TestSearchProjectionConsumerBindings(t *testing.T) {
	projection := &canonicalsearch.Projection{}
	for _, tc := range []struct {
		kind string
		want any
	}{
		{"creation", projection.ConsumeCanonicalCreation},
		{"account_profile", projection.ConsumeAccountProfile},
		{"social_policy", projection.ConsumeSocialPolicy},
		{"visibility", projection.Consume},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			got := runtime.FuncForPC(reflect.ValueOf(statsSearchProjectionConsumer(tc.kind, projection)).Pointer()).Name()
			want := runtime.FuncForPC(reflect.ValueOf(tc.want).Pointer()).Name()
			if got != want {
				t.Fatalf("search consumer binding: %s, want %s", got, want)
			}
		})
	}
}
