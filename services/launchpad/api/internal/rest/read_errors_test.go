package rest

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestReadFailureLogsCauseAndKeepsPublicEnvelope(t *testing.T) {
	var output bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	defer slog.SetDefault(original)
	for _, write := range []func(http.ResponseWriter, *http.Request, error){WriteReadError, writeComputedReadError, writeSnapshotError} {
		output.Reset()
		response := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v2/pools", nil)
		cause := fmt.Errorf("read committed projection: %w", errors.New("database connection closed"))
		write(response, req, cause)
		failure, err := v2.DecodeError(response.Body.Bytes())
		if err != nil || response.Code != 503 || failure.Code != "DEPENDENCY_UNAVAILABLE" || !strings.Contains(output.String(), cause.Error()) || !strings.Contains(output.String(), failure.RequestId) || strings.Contains(response.Body.String(), "database connection closed") {
			t.Fatalf("read failure: %d %s %v; diagnostic: %s", response.Code, response.Body, err, &output)
		}
	}
}
