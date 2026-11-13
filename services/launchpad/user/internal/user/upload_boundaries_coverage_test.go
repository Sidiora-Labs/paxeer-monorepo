package user_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
)

func TestImageUploadValidatesMultipartBoundaries(t *testing.T) {
	h := newHarness(t)
	for _, tc := range []struct {
		name  string
		parts []uploadFile
		want  int
	}{
		{"empty form", nil, http.StatusBadRequest},
		{"wrong part name", []uploadFile{{field: "other", name: "image.png", contentType: "image/png", data: pngFile("avatar").data}}, http.StatusBadRequest},
		{"missing filename", []uploadFile{{field: "avatar", contentType: "image/png", data: pngFile("avatar").data}}, http.StatusBadRequest},
		{"second image", []uploadFile{*pngFile("avatar"), *pngFile("avatar")}, http.StatusBadRequest},
		{"unsupported media type", []uploadFile{{field: "avatar", name: "image.gif", contentType: "image/gif", data: []byte("GIF89a")}}, http.StatusUnsupportedMediaType},
		{"unsafe vector", []uploadFile{{field: "avatar", name: "image.svg", contentType: "image/svg+xml", data: []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)}}, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			for _, file := range tc.parts {
				header := textproto.MIMEHeader{}
				header.Set("Content-Disposition", `form-data; name="`+file.field+`"; filename="`+file.name+`"`)
				header.Set("Content-Type", file.contentType)
				part, err := writer.CreatePart(header)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := part.Write(file.data); err != nil {
					t.Fatal(err)
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			path := "/users/" + h.wallet + "/avatar"
			req, err := http.NewRequest(http.MethodPost, h.baseURL+path, bytes.NewReader(body.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			contentType := writer.FormDataContentType()
			req.Header.Set("Content-Type", contentType)
			sum := sha256.Sum256(body.Bytes())
			intent, err := json.Marshal(struct {
				ContentType string `json:"content_type"`
				SHA256      string `json:"sha256"`
			}{contentType, hex.EncodeToString(sum[:])})
			if err != nil {
				t.Fatal(err)
			}
			h.authorize(t, req, "image.avatar.update", intent, h.sign(t, "sign-in"), "sign-in")
			response, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			raw, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != tc.want {
				t.Fatalf("multipart %s = %d %s, want %d", tc.name, response.StatusCode, raw, tc.want)
			}
		})
	}
}

func TestPublicImageReadDistinguishesMissingObjectAndUnavailableStores(t *testing.T) {
	h := newHarness(t)
	wallet := "0x5555555555555555555555555555555555555555"
	_, err := h.pool.Exec(context.Background(), `INSERT INTO users.user_images(id,wallet_address,image_type,storage_key,mime_type,size_bytes,uploaded_at) VALUES($1,$2,'avatar',$3,'image/png',8,1760000000)`, wallet+"-avatar", wallet, "avatars/missing.png")
	if err != nil {
		t.Fatal(err)
	}
	path := "/users/" + wallet + "/avatar"
	response, raw := h.do(t, http.MethodGet, path, nil)
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("missing image object = %d %s", response.StatusCode, raw)
	}
	unavailable, err := storage.New(context.Background(), storage.Config{Endpoint: "http://127.0.0.1:9", Region: "us-east-1", AccessKeyID: "test", SecretAccessKey: "test", Bucket: "user-assets"})
	if err != nil {
		t.Fatal(err)
	}
	deps := h.deps
	deps.Store = unavailable
	otherURL := serve(t, deps)
	response, err = http.Get(otherURL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err = io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusInternalServerError {
		t.Fatalf("unavailable image object = %d %s, %v", response.StatusCode, raw, err)
	}
	h.pool.Close()
	for _, path := range []string{
		"/users/0x6666666666666666666666666666666666666666",
		"/users/" + wallet + "/avatar",
		"/users/" + wallet + "/watchlist",
	} {
		response, raw := h.do(t, http.MethodGet, path, nil)
		if response.StatusCode != http.StatusInternalServerError {
			t.Fatalf("unavailable database for %s = %d %s", path, response.StatusCode, raw)
		}
	}
}

func TestUserRoutesRejectInvalidWalletAcrossMethods(t *testing.T) {
	h := newHarness(t)
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/users/invalid"},
		{http.MethodPost, "/users/invalid/avatar"},
		{http.MethodGet, "/users/invalid/avatar"},
		{http.MethodGet, "/users/invalid/watchlist"},
		{http.MethodPut, "/users/invalid/watchlist/0x1111111111111111111111111111111111111111"},
		{http.MethodDelete, "/users/invalid/watchlist/0x1111111111111111111111111111111111111111"},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			response, raw := h.do(t, tc.method, tc.path, nil)
			if response.StatusCode != http.StatusBadRequest {
				t.Fatalf("invalid wallet route = %d %s", response.StatusCode, raw)
			}
		})
	}
}

func TestPublicProfileRespondsWhenCacheIsUnavailable(t *testing.T) {
	h := newHarness(t)
	if err := h.deps.Redis.Close(); err != nil {
		t.Fatal(err)
	}
	response, raw := h.do(t, http.MethodGet, "/users/"+h.wallet, nil)
	if response.StatusCode != http.StatusOK || !bytes.Contains(raw, []byte(strings.ToLower(h.wallet))) {
		t.Fatalf("uncached public profile = %d %s", response.StatusCode, raw)
	}
}
