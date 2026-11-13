package image

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"testing"
)

func TestWorkerResponseValidation(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 12, 8))); err != nil {
		t.Fatal(err)
	}
	valid, err := ReencodeSupported(context.Background(), encoded.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	encode := func(response workerResponse) []byte {
		t.Helper()
		body, err := json.Marshal(response)
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	got, err := decodeWorkerResponse(encode(workerResponse{Raster: valid}))
	if err != nil || got.MIME != mimePNG || got.Width != 12 || got.Height != 8 || len(got.Variants) != 3 {
		t.Fatalf("validated raster = %+v, %v", got, err)
	}
	for _, tc := range []struct {
		name string
		body []byte
		want error
	}{
		{name: "invalid JSON", body: []byte(`{`), want: ErrInvalid},
		{name: "second document", body: append(encode(workerResponse{Raster: valid}), []byte(`{}`)...), want: ErrInvalid},
		{name: "unknown field", body: []byte(`{"Raster":{},"Failure":"","other":1}`), want: ErrInvalid},
		{name: "limit", body: encode(workerResponse{Failure: "limit"}), want: ErrLimit},
		{name: "unavailable", body: encode(workerResponse{Failure: "unavailable"}), want: ErrUnavailable},
		{name: "invalid", body: encode(workerResponse{Failure: "invalid"}), want: ErrInvalid},
		{name: "unknown failure", body: encode(workerResponse{Failure: "unknown"}), want: ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := decodeWorkerResponse(tc.body)
			if err != tc.want || len(result.Variants) != 0 {
				t.Fatalf("worker response = %+v, %v, want %v", result, err, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		name   string
		change func(*Raster)
	}{
		{name: "MIME", change: func(r *Raster) { r.MIME = "image/jpeg" }},
		{name: "width", change: func(r *Raster) { r.Width = 0 }},
		{name: "height", change: func(r *Raster) { r.Height = 0 }},
		{name: "large width", change: func(r *Raster) { r.Width = 8193 }},
		{name: "large height", change: func(r *Raster) { r.Height = 8193 }},
		{name: "large area", change: func(r *Raster) { r.Width, r.Height = 5000, 5000 }},
		{name: "missing variant", change: func(r *Raster) { delete(r.Variants, 512) }},
		{name: "empty variant", change: func(r *Raster) { r.Variants[512] = nil }},
		{name: "oversized variant", change: func(r *Raster) { r.Variants[512] = bytes.Repeat([]byte{1}, MaxBytes+1) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raster := valid
			raster.Variants = map[int][]byte{256: valid.Variants[256], 512: valid.Variants[512], 1024: valid.Variants[1024]}
			tc.change(&raster)
			result, err := decodeWorkerResponse(encode(workerResponse{Raster: raster}))
			if err != ErrInvalid || len(result.Variants) != 0 {
				t.Fatalf("%s response = %+v, %v", tc.name, result, err)
			}
		})
	}
}
