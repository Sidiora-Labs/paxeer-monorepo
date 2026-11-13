package streams

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestHandlerPublicReadsAndOwnerOperation(t *testing.T) {
	f := repair57New(t)
	o, intent := f.create("handler-public-reads")
	get := func(path string, authorization string, want int) []byte {
		t.Helper()
		request, err := http.NewRequest(http.MethodGet, f.server.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		if authorization != "" {
			request.Header.Set("Authorization", "Bearer "+authorization)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != want {
			t.Fatalf("GET %s: status=%d body=%s", path, response.StatusCode, raw)
		}
		return raw
	}
	for _, path := range []string{
		"/v2/streams?limit=1",
		"/v2/streams?poolAddress=" + intent.Pool,
		"/v2/streams/" + o.ID,
		"/v2/streams/pool/" + intent.Pool,
	} {
		body := get(path, "", http.StatusOK)
		if !strings.Contains(string(body), o.ID) {
			t.Fatalf("public read %s omitted stream: %s", path, body)
		}
	}
	for _, path := range []string{
		"/v2/streams?limit=0", "/v2/streams?limit=01", "/v2/streams?limit=101",
		"/v2/streams?status=ended", "/v2/streams?unknown=1", "/v2/streams?limit=1&limit=2",
		"/v2/streams?poolAddress=invalid", "/v2/streams?cursor=",
	} {
		get(path, "", http.StatusBadRequest)
	}
	get("/v2/streams/does-not-exist", "", http.StatusNotFound)
	operationPath := "/v2/streams/operations/" + o.ID
	get(operationPath, "", http.StatusForbidden)
	operation := get(operationPath, f.session(f.key), http.StatusOK)
	var document struct {
		Data struct {
			Operation struct {
				OperationID string `json:"operationId"`
			} `json:"operation"`
		} `json:"data"`
	}
	if err := json.Unmarshal(operation, &document); err != nil || document.Data.Operation.OperationID != o.ID {
		t.Fatalf("owner operation document = %s, %v", operation, err)
	}
	if _, err := f.database.Exec(context.Background(), "INSERT INTO chat.safety_bans(wallet,scope) VALUES($1,'global')", f.wallet); err != nil {
		t.Fatal(err)
	}
	get("/v2/streams/"+o.ID, "", http.StatusOK)
}
