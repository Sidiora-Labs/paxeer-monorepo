package apijson_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	sdk "chainflow-data-gateway"
	"chainflow-data-gateway/internal/apijson"
	"chainflow-data-gateway/option"
)

func TestRepairM_1307_CompleteSyntax(t *testing.T) {
	inputs := []struct {
		name string
		body string
	}{
		{"empty", ""},
		{"whitespace", " \n\t"},
		{"malformed", `{"status" "ok"}`},
		{"unquoted_key", `{status:"ok"}`},
		{"trailing_comma", `{"status":"ok",}`},
		{"invalid_unknown_field", `{"status":"ok","future":invalid}`},
		{"invalid_escape", `{"status":"ok","future":"\q"}`},
		{"invalid_number", `{"status":"ok","future":01}`},
		{"truncated_object", `{"status":"ok"`},
		{"truncated_array", `{"status":"ok","future":[1,2`},
		{"truncated_string", `{"status":"ok","future":"unfinished`},
		{"trailing_garbage", `{"status":"ok"}garbage`},
		{"second_document", `{"status":"ok"} {"status":"bad"}`},
	}
	for name, decode := range map[string]func([]byte, any) error{
		"Unmarshal":     apijson.Unmarshal,
		"UnmarshalRoot": apijson.UnmarshalRoot,
	} {
		t.Run(name, func(t *testing.T) {
			for _, input := range inputs {
				t.Run(input.name, func(t *testing.T) {
					model := sdk.FeedHealth{Status: "existing", Symbol: "eth"}
					before := model
					if err := decode([]byte(input.body), &model); err == nil {
						t.Fatal("invalid JSON was accepted")
					}
					if !reflect.DeepEqual(model, before) {
						t.Fatal("invalid JSON mutated the destination before validation")
					}
				})
			}
		})
	}
}

func TestRepairM_1307_ObjectRoot(t *testing.T) {
	for name, decode := range map[string]func([]byte, any) error{
		"Unmarshal":     apijson.Unmarshal,
		"UnmarshalRoot": apijson.UnmarshalRoot,
	} {
		t.Run(name, func(t *testing.T) {
			for _, body := range []string{`"ok"`, `42`, `true`, `false`, `null`, `[]`, `[{"status":"ok"}]`} {
				t.Run(body, func(t *testing.T) {
					model := sdk.FeedHealth{Status: "existing", Symbol: "eth"}
					before := model
					if err := decode([]byte(body), &model); err == nil || !strings.Contains(err.Error(), "expected an object") {
						t.Fatalf("incompatible root must return an object error, got %v", err)
					}
					if !reflect.DeepEqual(model, before) {
						t.Fatal("incompatible root mutated the model or its JSON metadata")
					}
					var pointer *sdk.FeedHealth
					if err := decode([]byte(body), &pointer); err == nil {
						t.Fatal("incompatible root was accepted through a pointer")
					}
					if pointer != nil {
						t.Fatal("failed decoding published a model pointer")
					}
					object := map[string]string{"existing": "value"}
					if err := decode([]byte(body), &object); err == nil {
						t.Fatal("incompatible root was accepted as a map")
					}
					if !reflect.DeepEqual(object, map[string]string{"existing": "value"}) {
						t.Fatal("incompatible root mutated the map")
					}
				})
			}
		})
	}
}

func TestRepairM_1307_RequestRejectsMalformedAndTruncatedResponses(t *testing.T) {
	for _, input := range []struct {
		name string
		body string
	}{
		{"malformed", `{"status" "ok"}`},
		{"malformed_unknown_field", `{"status":"ok","future":invalid}`},
		{"truncated_object", `{"status":"ok"`},
		{"truncated_string", `{"status":"unfinished`},
		{"truncated_array", `{"future":[1,2`},
	} {
		t.Run(input.name, func(t *testing.T) {
			service := repairM1307HealthServer(t, input.body)
			if _, err := service.ListHealth(context.Background(), "btc"); err == nil {
				t.Fatal("invalid HTTP JSON response returned a successful model")
			}
		})
	}
}

func TestRepairM_1307_RequestRejectsScalarResponses(t *testing.T) {
	for _, body := range []string{`"ok"`, `42`, `true`, `false`, `[]`, `[{"status":"ok"}]`} {
		t.Run(body, func(t *testing.T) {
			service := repairM1307HealthServer(t, body)
			if _, err := service.ListHealth(context.Background(), "btc"); err == nil || !strings.Contains(err.Error(), "apijson: expected an object") {
				t.Fatalf("request must propagate the production decoder's root error, got %v", err)
			}
		})
	}
}

func TestRepairM_1307_RequestValidatesCompleteErrorBody(t *testing.T) {
	for _, body := range []string{
		`{"error":invalid}`,
		`{"error":"unfinished"`,
		`{"error":"unavailable"} trailing`,
		`{"error":"unavailable"} {}`,
		`"unavailable"`, `42`, `true`, `null`, `[]`,
		`{"error":"unavailable","future":{"retry":true}}`,
	} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/api/btc/health" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			service := sdk.NewPriceFeedService(option.WithBaseURL(server.URL), option.WithHTTPClient(server.Client()), option.WithMaxRetries(0))
			model, err := service.ListHealth(context.Background(), "btc")
			if err == nil || model != nil {
				t.Fatal("HTTP error response returned a successful model")
			}
			var apiError *sdk.Error
			if body == `{"error":"unavailable","future":{"retry":true}}` {
				if !errors.As(err, &apiError) || apiError.StatusCode != http.StatusBadRequest || apiError.JSON.RawJSON() != body || apiError.JSON.ExtraFields["future"].Raw() != `{"retry":true}` {
					t.Fatalf("valid API error lost its status, raw body or unknown fields: %v", err)
				}
				return
			}
			if errors.As(err, &apiError) || !strings.Contains(err.Error(), "apijson:") {
				t.Fatalf("invalid error body must fail in the production decoder, got %v", err)
			}
		})
	}
}

func TestRepairM_1307_RequestRetainsUnknownFields(t *testing.T) {
	for _, status := range []sdk.FeedHealthStatus{sdk.FeedHealthStatusOk, "warming"} {
		t.Run(string(status), func(t *testing.T) {
			extra := map[string]any{
				"future":   map[string]any{"sequence": float64(7), "flags": []any{true, nil, "ready"}},
				"optional": nil,
			}
			payload := map[string]any{
				"status": status, "symbol": "btc", "backfill": nil, "lastCandle": nil,
				"future": extra["future"], "optional": extra["optional"],
			}
			requests := make(chan string, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests <- r.Method + " " + r.URL.RequestURI()
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(payload); err != nil {
					t.Errorf("encode response: %v", err)
				}
			}))
			defer server.Close()
			service := sdk.NewPriceFeedService(option.WithBaseURL(server.URL), option.WithHTTPClient(server.Client()), option.WithMaxRetries(0))
			model, err := service.ListHealth(context.Background(), "btc")
			if err != nil {
				t.Fatalf("valid response with unknown fields: %v", err)
			}
			if model == nil || model.Status != status || model.Symbol != "btc" {
				t.Fatal("consumer did not receive the produced health fields")
			}
			if request := <-requests; request != "GET /api/btc/health" {
				t.Fatalf("unexpected request: %s", request)
			}
			if model.JSON.Status.IsMissing() || model.JSON.Status.IsInvalid() || model.JSON.Symbol.IsInvalid() {
				t.Fatal("valid fields or a future enum value lost their metadata")
			}
			if !model.JSON.Backfill.IsNull() || model.JSON.Backfill.IsMissing() || !model.JSON.LastCandle.IsNull() || model.JSON.LastCandle.IsMissing() {
				t.Fatal("nullable fields lost their explicit null metadata")
			}
			if len(model.JSON.ExtraFields) != len(extra) {
				t.Fatalf("unknown fields were not retained: %v", model.JSON.ExtraFields)
			}
			for key, want := range extra {
				field, ok := model.JSON.ExtraFields[key]
				if !ok || field.IsMissing() {
					t.Fatalf("missing unknown field %s", key)
				}
				var got any
				if err := json.Unmarshal([]byte(field.Raw()), &got); err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("unknown field %s changed: got %v, error %v", key, got, err)
				}
			}
			var raw map[string]json.RawMessage
			if err := json.Unmarshal([]byte(model.JSON.RawJSON()), &raw); err != nil || len(raw) != len(payload) {
				t.Fatalf("raw object was not retained: %v", err)
			}
		})
	}
}

func TestRepairM_1307_RequestUnionRequiresObject(t *testing.T) {
	for _, body := range []string{`[]`, `[{"error":"No price data available"}]`, `{"error":"No price data available"}`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/api/btc/price" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			service := sdk.NewPriceFeedService(option.WithBaseURL(server.URL), option.WithHTTPClient(server.Client()), option.WithMaxRetries(0))
			model, err := service.List(context.Background(), "btc")
			if strings.HasPrefix(body, "[") {
				if err == nil {
					t.Fatal("array root became an empty successful union variant")
				}
				return
			}
			if err != nil || model == nil || model.Error != "No price data available" {
				t.Fatalf("valid error object union failed: model %v, error %v", model, err)
			}
			if _, ok := model.AsUnion().(sdk.ErrorModel); !ok {
				t.Fatalf("wrong union variant: %T", model.AsUnion())
			}
		})
	}
}

func TestRepairM_1307_CompatibleRootKinds(t *testing.T) {
	var text string
	if err := apijson.Unmarshal([]byte(`"ok"`), &text); err != nil || text != "ok" {
		t.Fatalf("string root: %q, %v", text, err)
	}
	var number int64
	if err := apijson.Unmarshal([]byte(`42`), &number); err != nil || number != 42 {
		t.Fatalf("number root: %d, %v", number, err)
	}
	var flag bool
	if err := apijson.Unmarshal([]byte(`true`), &flag); err != nil || !flag {
		t.Fatalf("boolean root: %t, %v", flag, err)
	}
	var timestamp time.Time
	if err := apijson.UnmarshalRoot([]byte(`"2026-01-02T03:04:05Z"`), &timestamp); err != nil || !timestamp.Equal(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)) {
		t.Fatalf("time root: %v, %v", timestamp, err)
	}
	var inline struct {
		Value string `json:",inline"`
	}
	if err := apijson.UnmarshalRoot([]byte(`"ok"`), &inline); err != nil || inline.Value != "ok" {
		t.Fatalf("inline scalar root: %q, %v", inline.Value, err)
	}
	var models []sdk.FeedHealth
	if err := apijson.Unmarshal([]byte(`[{"status":"ok","symbol":"btc"}]`), &models); err != nil || len(models) != 1 || models[0].Status != "ok" || models[0].Symbol != "btc" {
		t.Fatalf("array of objects: %v, %v", models, err)
	}
	var object map[string]sdk.FeedHealth
	if err := apijson.Unmarshal([]byte(`{"feed":{"status":"ok","symbol":"btc"}}`), &object); err != nil || object["feed"].Symbol != "btc" {
		t.Fatalf("map of objects: %v, %v", object, err)
	}
	model := sdk.FeedHealth{Status: "existing"}
	if err := apijson.UnmarshalRoot([]byte(" \n {\"status\":\"ok\",\"symbol\":\"btc\"} \t\r\n"), &model); err != nil || model.Status != "ok" || model.Symbol != "btc" {
		t.Fatalf("whitespace around object: %v, %v", model, err)
	}
	if err := apijson.UnmarshalRoot([]byte(`{}`), &model); err != nil {
		t.Fatalf("valid empty object must retain existing missing-field policy: %v", err)
	}
	var pointer *sdk.FeedHealth
	if err := apijson.Unmarshal([]byte(`{"status":"ok","symbol":"btc"}`), &pointer); err != nil || pointer == nil || pointer.Status != "ok" || pointer.Symbol != "btc" {
		t.Fatalf("valid object must publish the decoded model pointer: %v, %v", pointer, err)
	}
}

func repairM1307HealthServer(t *testing.T, body string) *sdk.PriceFeedService {
	t.Helper()
	requests := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.Method + " " + r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(func() {
		server.Close()
		select {
		case request := <-requests:
			if request != "GET /api/btc/health" {
				t.Errorf("unexpected request: %s", request)
			}
		default:
			t.Error("SDK did not reach the loopback response producer")
		}
	})
	return sdk.NewPriceFeedService(option.WithBaseURL(server.URL), option.WithHTTPClient(server.Client()), option.WithMaxRetries(0))
}
