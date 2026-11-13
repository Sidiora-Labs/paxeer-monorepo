package image_test

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/jpeg"
	"image/png"
	"testing"

	metadataimage "github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/image"
)

func TestSniffRasterContent(t *testing.T) {
	var pngBody bytes.Buffer
	if err := png.Encode(&pngBody, image.NewNRGBA(image.Rect(0, 0, 4, 3))); err != nil {
		t.Fatal(err)
	}
	var jpegBody bytes.Buffer
	if err := jpeg.Encode(&jpegBody, image.NewNRGBA(image.Rect(0, 0, 5, 2)), nil); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, declared string
		body           []byte
		mime           string
		width, height  int
	}{
		{"png", "image/png", pngBody.Bytes(), "image/png", 4, 3},
		{"jpeg", "image/jpeg", jpegBody.Bytes(), "image/jpeg", 5, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := metadataimage.Sniff(tc.declared, tc.body)
			if err != nil || got.MIME != tc.mime || got.Width != tc.width || got.Height != tc.height {
				t.Fatalf("Sniff = %+v, %v", got, err)
			}
		})
	}
	if _, err := metadataimage.Sniff("image/jpeg", pngBody.Bytes()); err != metadataimage.ErrMimeMismatch {
		t.Fatalf("declared MIME mismatch = %v", err)
	}
	if _, err := metadataimage.Sniff("image/png", []byte("<svg/>")); err != metadataimage.ErrInvalid {
		t.Fatalf("active content = %v", err)
	}
	if _, err := metadataimage.Sniff("image/png", append([]byte(nil), pngBody.Bytes()[:24]...)); err != metadataimage.ErrInvalid {
		t.Fatalf("truncated PNG = %v", err)
	}
	tooWide := append([]byte(nil), pngBody.Bytes()...)
	binary.BigEndian.PutUint32(tooWide[16:20], metadataimage.MaxDimension+1)
	binary.BigEndian.PutUint32(tooWide[29:33], crc32.ChecksumIEEE(tooWide[12:29]))
	if _, err := metadataimage.Sniff("image/png", tooWide); err != metadataimage.ErrLimit {
		t.Fatalf("oversized dimensions = %v", err)
	}
}
