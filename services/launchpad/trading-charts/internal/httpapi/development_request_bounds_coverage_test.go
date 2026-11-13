package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDevelopmentHistoryBoundsRequestsBeforeSourceAccess(t *testing.T) {
	base := "symbol=" + devPool + "&resolution=1&from=0&to=120"
	for _, tc := range []struct {
		name, query, code string
		status            int
	}{
		{"oversized", base + "&padding=" + strings.Repeat("a", 16385), "FIELD_INVALID", http.StatusUnprocessableEntity},
		{"malformed escape", base + "&mode=%", "FIELD_INVALID", http.StatusUnprocessableEntity},
		{"unsupported mode", base + "&mode=unknown", "FIELD_INVALID", http.StatusUnprocessableEntity},
		{"duplicate mode", base + "&mode=price&mode=price", "FIELD_INVALID", http.StatusUnprocessableEntity},
		{"unbound cursor", base + "&cursor=next", "FIELD_INVALID", http.StatusUnprocessableEntity},
		{"source unavailable", base, "DEPENDENCY_UNAVAILABLE", http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			(DevelopmentHistoryHandler{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v2/udf/history?"+tc.query, nil))
			var body struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if recorder.Code != tc.status || body.Code != tc.code {
				t.Fatalf("history response: status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
