package upload

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

const testAddr = "0x1234567890abcdef1234567890abcdef12345678"

// metaUpstream is a fake media/metadata that records the forwarded request.
type metaUpstream struct {
	path   string
	fields map[string]string
	files  map[string]int // field -> byte length received
	called bool
}

func newMetaUpstream(t *testing.T, rec *metaUpstream) *httptest.Server {
	t.Helper()
	rec.fields = map[string]string{}
	rec.files = map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.called = true
		rec.path = r.URL.Path
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		for k, v := range r.MultipartForm.Value {
			rec.fields[k] = v[0]
		}
		for k, fhs := range r.MultipartForm.File {
			f, _ := fhs[0].Open()
			b, _ := io.ReadAll(f)
			_ = f.Close()
			rec.files[k] = len(b)
		}
		sharedhttp.WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// buildUpload builds a multipart body with the given text fields and image file.
func buildUpload(t *testing.T, fields map[string]string, fileField, filename, mime string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	if fileField != "" {
		hdr := map[string][]string{
			"Content-Disposition": {`form-data; name="` + fileField + `"; filename="` + filename + `"`},
			"Content-Type":        {mime},
		}
		part, err := mw.CreatePart(hdr)
		if err != nil {
			t.Fatalf("create part: %v", err)
		}
		_, _ = part.Write(data)
	}
	_ = mw.Close()
	return &buf, mw.FormDataContentType()
}

func serve(t *testing.T, h *Handler, addr string, body *bytes.Buffer, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPost, "/upload/token/"+addr, body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func cleanScan(context.Context, []byte) sharedhttp.ScanResult {
	return sharedhttp.ScanResult{Clean: true}
}

func TestUpload_ForwardsValidUpload(t *testing.T) {
	var up metaUpstream
	srv := newMetaUpstream(t, &up)
	h := New(Deps{MetadataBaseURL: srv.URL, MaxBytes: 6 << 20, Scan: cleanScan})

	png := []byte("\x89PNG\r\n\x1a\nfake-png-bytes")
	body, ct := buildUpload(t, map[string]string{
		"wallet":    testAddr,
		"signature": "0xsig",
		"message":   "sign me",
		"metadata":  `{"name":"Tok"}`,
	}, "logo", "logo.png", "image/png", png)

	rec := serve(t, h, testAddr, body, ct)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !up.called || up.path != "/metadata/"+testAddr {
		t.Fatalf("upstream path = %q called=%v", up.path, up.called)
	}
	if up.fields["wallet"] != testAddr || up.fields["metadata"] != `{"name":"Tok"}` {
		t.Errorf("forwarded fields = %v", up.fields)
	}
	if up.files["logo"] != len(png) {
		t.Errorf("forwarded logo bytes = %d, want %d", up.files["logo"], len(png))
	}
}

func TestUpload_RejectsBadMime(t *testing.T) {
	var up metaUpstream
	srv := newMetaUpstream(t, &up)
	h := New(Deps{MetadataBaseURL: srv.URL, MaxBytes: 6 << 20, Scan: cleanScan})

	body, ct := buildUpload(t, map[string]string{"wallet": testAddr}, "logo", "a.gif", "image/gif", []byte("GIF89a"))
	rec := serve(t, h, testAddr, body, ct)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if up.called {
		t.Error("upstream should not be called on rejected upload")
	}
}

func TestUpload_RejectsUnsafeSVG(t *testing.T) {
	srv := newMetaUpstream(t, &metaUpstream{})
	h := New(Deps{MetadataBaseURL: srv.URL, MaxBytes: 6 << 20, Scan: cleanScan})

	svg := []byte(`<svg onload="evil()"></svg>`)
	body, ct := buildUpload(t, map[string]string{"wallet": testAddr}, "logo", "a.svg", "image/svg+xml", svg)
	rec := serve(t, h, testAddr, body, ct)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestUpload_RejectsInfectedFile(t *testing.T) {
	srv := newMetaUpstream(t, &metaUpstream{})
	infected := func(context.Context, []byte) sharedhttp.ScanResult {
		return sharedhttp.ScanResult{Clean: false, Verdict: sharedhttp.ScanInfected, Reason: "Eicar-Test-Signature"}
	}
	h := New(Deps{MetadataBaseURL: srv.URL, MaxBytes: 6 << 20, Scan: infected})

	body, ct := buildUpload(t, map[string]string{"wallet": testAddr}, "logo", "a.png", "image/png", []byte("x"))
	rec := serve(t, h, testAddr, body, ct)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "virus scanner") {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestUpload_RejectsInvalidAddr(t *testing.T) {
	srv := newMetaUpstream(t, &metaUpstream{})
	h := New(Deps{MetadataBaseURL: srv.URL, MaxBytes: 6 << 20, Scan: cleanScan})
	body, ct := buildUpload(t, map[string]string{"wallet": testAddr}, "", "", "", nil)
	rec := serve(t, h, "not-an-address", body, ct)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestUpload_BadGatewayWhenMetadataDown(t *testing.T) {
	srv := newMetaUpstream(t, &metaUpstream{})
	addr := srv.URL
	srv.Close() // upstream unreachable
	h := New(Deps{MetadataBaseURL: addr, MaxBytes: 6 << 20, Scan: cleanScan})

	body, ct := buildUpload(t, map[string]string{"wallet": testAddr}, "logo", "a.png", "image/png", []byte("\x89PNG"))
	rec := serve(t, h, testAddr, body, ct)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
}

func TestUpload_RejectsOversize(t *testing.T) {
	srv := newMetaUpstream(t, &metaUpstream{})
	h := New(Deps{MetadataBaseURL: srv.URL, MaxBytes: 64, Scan: cleanScan}) // 64-byte cap

	big := bytes.Repeat([]byte("A"), 4096)
	body, ct := buildUpload(t, map[string]string{"wallet": testAddr}, "logo", "a.png", "image/png", big)
	rec := serve(t, h, testAddr, body, ct)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
	if err := v2.ValidateJSON("Error", rec.Body.Bytes()); err != nil {
		t.Fatalf("413 body violates the v2 Error schema: %v (%s)", err, rec.Body.String())
	}
	var e v2.Error
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	if e.Code != "FIELD_INVALID" || e.Retryable || len(e.FieldErrors) != 1 || e.FieldErrors[0].Rule != sharedhttp.RuleMaxBytes {
		t.Fatalf("413 envelope = %+v", e)
	}
}

func TestUploadRejectsInvalidMultipartFields(t *testing.T) {
	textFields := map[string]string{}
	for i := 0; i < 17; i++ {
		textFields[strconv.Itoa(i)] = "value"
	}
	for _, tc := range []struct {
		name, field, filename, mime, message string
		fields                               map[string]string
	}{
		{"unknown image", "avatar", "avatar.png", "image/png", "unexpected or duplicate image field", nil},
		{"too many values", "", "", "", "too many fields", textFields},
		{"long text name", "", "", "", "invalid text field", map[string]string{strings.Repeat("n", 65): "value"}},
		{"long text value", "", "", "", "invalid text field", map[string]string{"description": strings.Repeat("v", 8193)}},
		{"long filename", "logo", strings.Repeat("n", 129) + ".png", "image/png", "invalid image filename", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var upstream metaUpstream
			server := newMetaUpstream(t, &upstream)
			h := New(Deps{MetadataBaseURL: server.URL, MaxBytes: 6 << 20, Scan: cleanScan})
			body, contentType := buildUpload(t, tc.fields, tc.field, tc.filename, tc.mime, []byte("image bytes"))
			response := serve(t, h, testAddr, body, contentType)
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), tc.message) || upstream.called {
				t.Fatalf("multipart response=%d body=%s forwarded=%v", response.Code, response.Body.String(), upstream.called)
			}
		})
	}
}

func TestUploadForwardBoundsMetadataResponse(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   int
	}{
		{"redirect", http.StatusFound, "redirect", http.StatusBadGateway},
		{"oversized response", http.StatusOK, strings.Repeat("x", (1<<20)+1), http.StatusBadGateway},
		{"accepted response", http.StatusAccepted, `{"state":"queued"}`, http.StatusAccepted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/metadata/"+testAddr {
					t.Errorf("forwarded path=%s", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			t.Cleanup(server.Close)
			h := New(Deps{MetadataBaseURL: server.URL, MaxBytes: 6 << 20, Scan: cleanScan})
			body, contentType := buildUpload(t, map[string]string{"wallet": testAddr}, "", "", "", nil)
			response := serve(t, h, testAddr, body, contentType)
			if response.Code != tc.want || response.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("forwarded response=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
			}
			if tc.want == http.StatusAccepted && response.Body.String() != tc.body {
				t.Fatalf("accepted response body=%s", response.Body.String())
			}
		})
	}
}
