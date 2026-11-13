package streams

import (
	"bytes"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCreatorCredentialRouteRequiresCurrentStateAndEncryptedKey(t *testing.T) {
	f := repair57New(t)
	requested, _ := f.create("credential-pending")
	ready, _ := f.create("credential-authorized")
	ready, err := f.store.BindProvider(t.Context(), ready.ID, "credential_provider")
	if err != nil {
		t.Fatal(err)
	}
	call := func(id string, headers map[string]string, want int) {
		t.Helper()
		path := "/v2/streams/" + id + "/ingest-key"
		request, err := http.NewRequest(http.MethodPost, f.server.URL+path, strings.NewReader(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		for key, value := range headers {
			request.Header.Set(key, value)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil || response.StatusCode != want || !bytes.Contains(raw, []byte(`"code"`)) {
			t.Fatalf("credential decision = %d %s, %v", response.StatusCode, raw, err)
		}
	}
	signed := func(id string) map[string]string {
		path := "/v2/streams/" + id + "/ingest-key"
		return f.signed(path, "stream.credential", []byte(`{}`), f.key)
	}
	call(requested.ID, signed(requested.ID), http.StatusConflict)
	call(ready.ID, signed(ready.ID), http.StatusServiceUnavailable)
	f.handlers.ingestKeys = &EncryptedCredentials{Store: f.store, Transit: &VaultTransit{Key: "credential_key"}, IngestEndpoint: "rtmps://rtmp.livepeer.studio/live"}
	call(ready.ID, signed(ready.ID), http.StatusServiceUnavailable)
	for _, change := range []func(map[string]string){
		func(h map[string]string) { h["X-Kindle-Operation"] = strings.Repeat("x", 4097) },
		func(h map[string]string) { h["X-Kindle-Operation"] = "%%%" },
		func(h map[string]string) {
			h["X-Kindle-Operation"] = base64.RawURLEncoding.EncodeToString([]byte(`[]`))
		},
		func(h map[string]string) { h["X-Kindle-Signature"] = "invalid" },
	} {
		headers := signed(ready.ID)
		change(headers)
		call(ready.ID, headers, http.StatusForbidden)
	}
}
