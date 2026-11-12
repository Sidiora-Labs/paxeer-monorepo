package streams

import (
	"bytes"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestConnectedMutationsRejectAmbiguousBodiesAndUnboundSignatures(t *testing.T) {
	f := repair57New(t)
	o, intent := f.create("mutation-validation-setup")
	call := func(method, path, contentType string, body []byte, headers map[string]string, want int) {
		t.Helper()
		r, err := http.NewRequest(method, f.server.URL+path, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if contentType != "" {
			r.Header.Set("Content-Type", contentType)
		}
		for name, value := range headers {
			r.Header.Set(name, value)
		}
		response, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil || response.StatusCode != want {
			t.Fatalf("%s %s = %d %s, %v", method, path, response.StatusCode, raw, err)
		}
		if want >= 400 && !bytes.Contains(raw, []byte(`"code"`)) {
			t.Fatalf("contract error missing code: %s", raw)
		}
	}
	path := "/v2/streams"
	valid := []byte(`{"poolAddress":"` + intent.Pool + `","title":"New stream"}`)
	call(http.MethodPost, path, "text/plain", valid, nil, http.StatusUnsupportedMediaType)
	call(http.MethodPost, path, "application/json", []byte(strings.Repeat("x", 16385)), nil, http.StatusRequestEntityTooLarge)
	call(http.MethodPost, path, "application/json", []byte{'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'}, nil, http.StatusBadRequest)
	call(http.MethodPost, path, "application/json", []byte(`{"poolAddress":"`+intent.Pool+`","title":"New stream","title":"Other"}`), nil, http.StatusBadRequest)
	call(http.MethodPost, path, "application/json", valid, map[string]string{"Authorization": "Bearer " + f.session(f.key)}, http.StatusForbidden)
	call(http.MethodPost, path, "application/json", valid, map[string]string{"Authorization": "Bearer " + f.session(f.key), "X-Kindle-Operation": "%%%"}, http.StatusForbidden)
	call(http.MethodPost, "/v2/streams/"+o.ID+"/go-live", "application/json", []byte(`{"revision":"01"}`), nil, http.StatusUnprocessableEntity)
	call(http.MethodPatch, "/v2/streams/"+o.ID, "application/json", []byte(`{"title":"Changed","revision":"01"}`), nil, http.StatusUnprocessableEntity)
	call(http.MethodPost, "/v2/streams/"+o.ID+"/ingest-key", "application/json", []byte(`{}`), map[string]string{"Authorization": "Bearer " + f.session(f.key)}, http.StatusForbidden)
}

func TestConnectedCreateRefusesNonEnvelopeAndChangedActorProof(t *testing.T) {
	f := repair57New(t)
	_, intent := f.create("create-proof-setup")
	path := "/v2/streams"
	body := []byte(`{"poolAddress":"` + intent.Pool + `","title":"Proof bound stream"}`)
	for _, raw := range [][]byte{[]byte(`[]`), []byte(`null`), []byte(`{"unexpected":true}`), []byte(`{} {}`)} {
		headers := f.signed(path, "stream.create", body, f.key)
		headers["X-Kindle-Operation"] = base64.RawURLEncoding.EncodeToString(raw)
		f.exchange(http.MethodPost, path, body, headers, http.StatusForbidden)
	}
	headers := f.signed(path, "stream.create", body, f.key)
	headers["X-Kindle-Signature"] = strings.Repeat("x", 8195)
	f.exchange(http.MethodPost, path, body, headers, http.StatusForbidden)
	headers = f.signed(path, "stream.create", body, f.key)
	headers["X-Kindle-Signature"] = "invalid"
	f.exchange(http.MethodPost, path, body, headers, http.StatusForbidden)
	var count int
	if err := f.database.QueryRow(t.Context(), "SELECT count(*) FROM livestream.stream_operations WHERE title=$1", "Proof bound stream").Scan(&count); err != nil || count != 0 {
		t.Fatalf("unverified create side effects = %d, %v", count, err)
	}
}
