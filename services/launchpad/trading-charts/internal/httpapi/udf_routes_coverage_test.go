package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

func TestUDFRegistrationsServeConfigurationAndTime(t *testing.T) {
	for _, tc := range []struct {
		name     string
		register func(chi.Router)
		prefix   string
	}{
		{"development", func(r chi.Router) { RegisterUDF(r, nil) }, ""},
		{"production", func(r chi.Router) { RegisterProductionUDF(r, SymbolsHandler{}, ProductionHistoryHandler{}) }, "/v2/udf"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := chi.NewRouter()
			tc.register(router)
			configResponse := httptest.NewRecorder()
			router.ServeHTTP(configResponse, httptest.NewRequest(http.MethodGet, tc.prefix+"/config", nil))
			var config map[string]any
			if err := json.Unmarshal(configResponse.Body.Bytes(), &config); err != nil || configResponse.Code != 200 || config["supports_time"] != true {
				t.Fatalf("UDF configuration: %d %v %v", configResponse.Code, config, err)
			}
			timeResponse := httptest.NewRecorder()
			before := time.Now().Unix()
			router.ServeHTTP(timeResponse, httptest.NewRequest(http.MethodGet, tc.prefix+"/time", nil))
			var observed int64
			if err := json.Unmarshal(timeResponse.Body.Bytes(), &observed); err != nil || timeResponse.Code != 200 || observed < before || observed > time.Now().Unix() {
				t.Fatalf("UDF time: %d %d %v", timeResponse.Code, observed, err)
			}
		})
	}
}

func TestFormattedHistoryNumbers(t *testing.T) {
	for _, tc := range []struct {
		name  string
		parse func(string) (float64, error)
		input string
		want  float64
		bad   bool
	}{
		{"price", parseFloatFormatted, "1250000000000000000", 1.25, false},
		{"price invalid", parseFloatFormatted, "not-a-number", 0, true},
		{"volume", parseFloatFormattedVolume, "1250000", 1.25, false},
		{"volume invalid", parseFloatFormattedVolume, "not-a-number", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.parse(tc.input)
			if tc.bad && err == nil || !tc.bad && (err != nil || got != tc.want) {
				t.Fatalf("formatted number: %g, %v", got, err)
			}
		})
	}
}
