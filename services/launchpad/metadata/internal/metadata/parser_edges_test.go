package metadata

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestMetadataDocumentParserBoundaries(t *testing.T) {
	for _, raw := range [][]byte{
		bytes.Repeat([]byte("x"), 16385),
		[]byte(`{"expectedRevision":"0"}{"expectedRevision":"1"}`),
		[]byte(`null`),
		[]byte(`{"expectedRevision":"0","tags":["a","b","c","d","e","f","g","h","i","j","k"]}`),
		[]byte(`{"expectedRevision":"0","tags":["art","media"]}`),
		[]byte(`{"expectedRevision":"0","logoAssetId":"not-a-uuid"}`),
		[]byte(`{"expectedRevision":"0","tags":["a","a"]}`),
		[]byte(`{"expectedRevision":"0","description":"\u0001"}`),
	} {
		patch, err := ParsePatch(raw)
		if string(raw) == `{"expectedRevision":"0","tags":["art","media"]}` {
			if err != nil || patch.Tags == nil || len(*patch.Tags) != 2 || (*patch.Tags)[1] != "media" {
				t.Fatalf("valid tags = %+v, %v", patch, err)
			}
		} else if err != ErrValidation {
			t.Fatalf("legacy patch = %+v, %v", patch, err)
		}
	}
	if revisionValid(strings.Repeat("9", 78)) || revisionValid("1x") || !revisionValid("1") {
		t.Fatal("revision bounds changed")
	}
	if _, err := text(string([]byte{0xff}), 0, 80); err != ErrValidation {
		t.Fatalf("invalid UTF-8 = %v", err)
	}
	for _, raw := range [][]byte{
		bytes.Repeat([]byte("x"), V2BodyLimit+1),
		[]byte(`{"expectedRevision":"0","tags":["a","a"]}`),
		[]byte(`{"expectedRevision":"0","socials":[{"label":"site","url":"https://example.com"},{"label":"copy","url":"https://example.com"}]}`),
		[]byte(`{"expectedRevision":"0","logoAssetId":"invalid"}`),
		[]byte(`{"expectedRevision":"0","socials":[{"label":"","url":"https://example.com"}]}`),
		[]byte(`{"expectedRevision":"0","socials":[{"label":"site","url":"https://example.com/` + strings.Repeat("x", 2049) + `"}]}`),
	} {
		if _, err := ParseV2Patch(raw); err != ErrValidation {
			t.Fatalf("V2 patch %q = %v", raw[:min(len(raw), 80)], err)
		}
	}
	deep := strings.Repeat("[", 65) + "0" + strings.Repeat("]", 65)
	if uniqueV2JSON(json.NewDecoder(strings.NewReader(deep)), 0) {
		t.Fatal("deeply nested JSON accepted")
	}
	if !uniqueV2JSON(json.NewDecoder(strings.NewReader(`{"a":[1,true,{"b":null}]}`)), 0) {
		t.Fatal("unique nested JSON rejected")
	}
	if uniqueV2JSON(json.NewDecoder(strings.NewReader(`{"a":1,"a":2}`)), 0) {
		t.Fatal("duplicate nested member accepted")
	}
	for _, raw := range [][]byte{
		bytes.Repeat([]byte("x"), 16385),
		[]byte(`[]`),
		[]byte(`{"mediaType":"image/png","bytes":1,"sha256":null}`),
		[]byte(`{"mediaType":"image/png","bytes":1}`),
		[]byte(`{"mediaType":"image/png","bytes":1,"sha256":"a","unknown":true}`),
	} {
		var body UploadCreateBody
		if err := decodeUpload(raw, map[string]bool{"mediaType": true, "bytes": true, "sha256": true}, &body); err != ErrValidation {
			t.Fatalf("upload body = %+v, %v", body, err)
		}
	}
}
