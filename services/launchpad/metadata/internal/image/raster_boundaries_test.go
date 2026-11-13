package image

import (
	"bytes"
	"context"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/png"
	"os"
	"testing"
)

func TestRasterRejectsMalformedContainerChunks(t *testing.T) {
	ctx := context.Background()
	webp := func(kind string, payload []byte) []byte {
		body := append([]byte("RIFF\x00\x00\x00\x00WEBP"), []byte(kind)...)
		size := make([]byte, 4)
		binary.LittleEndian.PutUint32(size, uint32(len(payload)))
		body = append(body, size...)
		body = append(body, payload...)
		if len(payload)%2 != 0 {
			body = append(body, 0)
		}
		binary.LittleEndian.PutUint32(body[4:8], uint32(len(body)-8))
		return body
	}
	for _, tc := range []struct {
		name string
		body []byte
	}{
		{"animated frame", webp("ANMF", nil)},
		{"animated header", webp("VP8X", append([]byte{2}, make([]byte, 9)...))},
		{"short extended header", webp("VP8X", make([]byte, 9))},
		{"truncated chunk", append([]byte("RIFF\x0d\x00\x00\x00WEBP"), []byte("VP8X\x00")...)},
		{"trailing chunk header", append([]byte("RIFF\x05\x00\x00\x00WEBP"), 'x')},
		{"missing odd chunk padding", []byte("RIFF\x0d\x00\x00\x00WEBPVP8X\x01\x00\x00\x00x")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ReencodeSupported(ctx, tc.body); err != ErrInvalid {
				t.Fatalf("malformed WebP = %v", err)
			}
		})
	}
	badLength := webp("VP8X", make([]byte, 10))
	binary.LittleEndian.PutUint32(badLength[4:8], 1)
	if _, err := ReencodeSupported(ctx, badLength); err != ErrInvalid {
		t.Fatalf("WebP length mismatch = %v", err)
	}
	oversizedChunk := webp("VP8X", make([]byte, 10))
	binary.LittleEndian.PutUint32(oversizedChunk[16:20], 20)
	if _, err := ReencodeSupported(ctx, oversizedChunk); err != ErrInvalid {
		t.Fatalf("WebP chunk length = %v", err)
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 3, 2))); err != nil {
		t.Fatal(err)
	}
	first := encoded.Bytes()
	actl := make([]byte, 16)
	binary.BigEndian.PutUint32(actl[:4], 4)
	copy(actl[4:8], "acTL")
	binary.BigEndian.PutUint32(actl[12:16], crc32.ChecksumIEEE(actl[4:12]))
	animated := append(append([]byte{}, first[:33]...), actl...)
	animated = append(animated, first[33:]...)
	if _, err := ReencodeSupported(ctx, animated); err != ErrInvalid {
		t.Fatalf("animated PNG = %v", err)
	}
	badChunk := append([]byte(nil), first...)
	binary.BigEndian.PutUint32(badChunk[33:37], uint32(len(first)))
	if _, err := ReencodeSupported(ctx, badChunk); err != ErrInvalid {
		t.Fatalf("PNG chunk length = %v", err)
	}
}

func TestRasterScalesTallImageAndBoundsInput(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 120, 600))); err != nil {
		t.Fatal(err)
	}
	result, err := Reencode(context.Background(), encoded.Bytes())
	if err != nil || result.Width != 120 || result.Height != 600 {
		t.Fatalf("tall raster = %+v, %v", result, err)
	}
	for size, wantWidth := range map[int]int{256: 51, 512: 102, 1024: 120} {
		variant, err := png.Decode(bytes.NewReader(result.Variants[size]))
		if err != nil || variant.Bounds().Dx() != wantWidth || variant.Bounds().Dy() != min(size, 600) {
			t.Fatalf("%d variant = %v, %v", size, variant, err)
		}
	}
	if _, err := Reencode(context.Background(), bytes.Repeat([]byte("x"), MaxBytes+1)); err != ErrLimit {
		t.Fatalf("oversized raster = %v", err)
	}
	closed, err := os.CreateTemp(t.TempDir(), "closed-input")
	if err != nil {
		t.Fatal(err)
	}
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadBounded(closed); err == nil {
		t.Fatal("closed image source accepted")
	}
}

func TestRasterScalesWideImage(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 600, 120))); err != nil {
		t.Fatal(err)
	}
	result, err := Reencode(context.Background(), encoded.Bytes())
	if err != nil || result.Width != 600 || result.Height != 120 {
		t.Fatalf("wide raster = %+v, %v", result, err)
	}
	for size, wantHeight := range map[int]int{256: 51, 512: 102, 1024: 120} {
		variant, err := png.Decode(bytes.NewReader(result.Variants[size]))
		if err != nil || variant.Bounds().Dx() != min(size, 600) || variant.Bounds().Dy() != wantHeight {
			t.Fatalf("%d variant = %v, %v", size, variant, err)
		}
	}
}
