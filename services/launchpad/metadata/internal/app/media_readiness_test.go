package app

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	raster "github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/image"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/metadata"
)

func TestMediaReadinessStartupReasons(t *testing.T) {
	for _, tc := range []struct {
		name string
		part string
		step string
		want string
	}{
		{name: "raster", part: metadata.MediaPartRaster, want: "metadata_media_raster_unavailable"},
		{name: "scanner", part: metadata.MediaPartScanner, want: "metadata_media_scanner_unavailable"},
		{name: "objects", part: metadata.MediaPartObjects, step: metadata.MediaObjectsStepApprovedBucket, want: "metadata_media_objects_unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cause := errors.New("dependency unavailable")
			err := mediaStartupError(&metadata.MediaReadinessError{Part: tc.part, Step: tc.step, Err: cause})
			if !errors.Is(err, cause) || !containsErrorCode(err, tc.want) {
				t.Fatalf("startup error = %v, want %s", err, tc.want)
			}
		})
	}
	if got := mediaStartupError(raster.ErrUnavailable); got.Error() != "metadata_media_dependencies_unavailable" {
		t.Fatalf("unclassified media error = %v", got)
	}
}

func containsErrorCode(err error, want string) bool {
	return len(err.Error()) >= len(want) && (err.Error() == want || len(err.Error()) > len(want) && err.Error()[:len(want)] == want)
}

func TestMediaReadinessLogsDependencyCause(t *testing.T) {
	var output bytes.Buffer
	cause := &metadata.MediaReadinessError{Part: metadata.MediaPartScanner, Err: context.DeadlineExceeded}
	check := mediaReadinessCheck(slog.New(slog.NewJSONHandler(&output, nil)), "media_scanner", func(context.Context) error { return cause })
	if err := check(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("readiness cause = %v", err)
	}
	for _, field := range []string{`"check":"media_scanner"`, `"part":"scanner"`, `"cause":"deadline_exceeded"`} {
		if !strings.Contains(output.String(), field) {
			t.Fatalf("readiness diagnostic lacks %s: %s", field, output.String())
		}
	}
}
