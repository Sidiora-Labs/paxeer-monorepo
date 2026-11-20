package httpapi

import (
	"context"
	"encoding/binary"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProductionSymbolsInvalidBeforeAuthority(t *testing.T) {
	for _, query := range []string{"symbol=bad", "symbol=0x1111111111111111111111111111111111111111&chainId=1", "symbol=0x1111111111111111111111111111111111111111&symbol=0x1111111111111111111111111111111111111111"} {
		response := httptest.NewRecorder()
		SymbolsHandler{}.ServeHTTP(response, httptest.NewRequest("GET", "/v2/udf/symbols?"+query, nil))
		if response.Code != 422 {
			t.Fatalf("invalid request reached authority: %d", response.Code)
		}
	}
}
func TestProductionMetadataABIText(t *testing.T) {
	raw := make([]byte, 96)
	binary.BigEndian.PutUint64(raw[24:32], 32)
	binary.BigEndian.PutUint64(raw[56:64], 4)
	copy(raw[64:], "USDL")
	value, err := decodeMetadataString(raw)
	if err != nil || value != "USDL" {
		t.Fatalf("actual ABI string: %q %v", value, err)
	}
	raw[64] = '<'
	if value, err = decodeMetadataString(raw); err != nil || value != "<SDL" || boundedMetadataText(value, 16, udfPool) != "SDL" {
		t.Fatalf("metadata display text: %q %v", value, err)
	}
	binary.BigEndian.PutUint64(raw[56:64], 1024)
	if _, err = decodeMetadataString(raw); err == nil {
		t.Fatal("ABI overflow accepted")
	}
}

func TestProductionSymbolsUnavailableAndCanceledMetadata(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		request := httptest.NewRequest("GET", "/v2/udf/symbols?symbol="+udfPool, nil)
		if canceled {
			ctx, cancel := context.WithCancel(request.Context())
			cancel()
			request = request.WithContext(ctx)
		}
		response := httptest.NewRecorder()
		SymbolsHandler{}.ServeHTTP(response, request)
		if response.Code != 503 || strings.Contains(response.Body.String(), "pricescale") || !strings.Contains(response.Body.String(), `"retryable":true`) {
			t.Fatal(response.Code, response.Body.String())
		}
	}
}
