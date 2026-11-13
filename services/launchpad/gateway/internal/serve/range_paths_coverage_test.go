package serve

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStreamedObjectRangesPreserveBoundedResponses(t *testing.T) {
	handler := New(Deps{})
	for _, tc := range []struct {
		name, method, contentType, rangeValue, ifRange, ifNoneMatch string
		body                                                        []byte
		size                                                        int64
		status                                                      int
		want                                                        string
	}{
		{"suffix", http.MethodGet, pngContentType, "bytes=-3", "", "", []byte("abcdef"), 6, http.StatusPartialContent, "def"},
		{"clamped end", http.MethodGet, pngContentType, "bytes=1-999", "", "", []byte("abcdef"), 6, http.StatusPartialContent, "bcdef"},
		{"unmatched validator", http.MethodGet, pngContentType, "bytes=1-2", `"other"`, "", []byte("abcdef"), 6, http.StatusOK, "abcdef"},
		{"unsupported media range", http.MethodGet, "application/octet-stream", "bytes=1-2", "", "", []byte("abcdef"), 6, http.StatusRequestedRangeNotSatisfiable, "range unavailable"},
		{"invalid range", http.MethodGet, pngContentType, "bytes=9-10", "", "", []byte("abcdef"), 6, http.StatusRequestedRangeNotSatisfiable, "invalid range"},
		{"short origin body", http.MethodGet, pngContentType, "bytes=3-4", "", "", []byte("ab"), 6, http.StatusBadGateway, `"statusCode":502`},
		{"conditional", http.MethodGet, pngContentType, "", "", `"owned"`, []byte("abcdef"), 6, http.StatusNotModified, ""},
		{"head range", http.MethodHead, pngContentType, "bytes=1-2", "", "", []byte("abcdef"), 6, http.StatusPartialContent, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, "/media/token/owned.png", nil)
			request.Header.Set("Range", tc.rangeValue)
			request.Header.Set("If-Range", tc.ifRange)
			request.Header.Set("If-None-Match", tc.ifNoneMatch)
			response := httptest.NewRecorder()
			handler.stream(response, request, tc.contentType, "owned", bytes.NewReader(tc.body), tc.size)
			if response.Code != tc.status || (tc.status < 400 && response.Body.String() != tc.want) || (tc.status >= 400 && !strings.Contains(response.Body.String(), tc.want)) {
				t.Fatalf("range response: status=%d body=%q, want status=%d body=%q", response.Code, response.Body.String(), tc.status, tc.want)
			}
		})
	}
}

func TestBoundedObjectRangeRejectsUnsafeIntervals(t *testing.T) {
	for _, tc := range []struct {
		value string
		size  int64
	}{
		{"bytes=0-1", 0},
		{strings.Repeat("x", 65), 10},
		{"items=0-1", 10},
		{"bytes=0-1,3-4", 10},
		{"bytes=0-1-2", 10},
		{"bytes=0-x", 10},
		{"bytes=-0", 10},
		{"bytes=-1048577", 10},
		{"bytes=10-", 10},
		{"bytes=2-1", 10},
		{"bytes=0-1048576", 1048577},
	} {
		if start, end, ok := boundedRange(tc.value, tc.size); ok {
			t.Fatalf("unbounded interval %q size=%d became %d-%d", tc.value, tc.size, start, end)
		}
	}
	if quoteETag(`W/"owned"`) != `W/"owned"` || quoteETag("owned") != `"owned"` {
		t.Fatal("object validators changed")
	}
}
