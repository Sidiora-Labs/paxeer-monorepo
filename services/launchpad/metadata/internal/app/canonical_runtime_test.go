package app

import (
	"context"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/config"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
)

func TestProductionStartupRequiresCanonicalStream(t *testing.T) {
	for _, stream := range []string{"", " \t\n"} {
		cfg := config.Config{NodeEnv: "production", Ingress: config.IngressEdge, CanonicalStreamID: stream}
		owned := false
		instance, err := newRuntime(context.Background(), cfg, nil, nil, nil, nil, nil, nil, map[string]sharedhttp.Check{}, &owned)
		if instance != nil || owned || err == nil || !strings.Contains(err.Error(), "metadata_canonical_stream_required") || !strings.Contains(err.Error(), "CANONICAL_STREAM_ID") {
			t.Fatalf("canonical stream configuration = %v, owned=%v", err, owned)
		}
	}
}
