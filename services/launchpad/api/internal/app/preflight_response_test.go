package app

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/capabilities"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestPreflightResponseClassifiesRequestAndSource(t *testing.T) {
	authority := contractAuthority(t)
	const trader = "0x00000000000000000000000000000000000000a1"
	const zero = "0x0000000000000000000000000000000000000000"
	body := func(revision string) []byte {
		return []byte(`{"family":"trading","expectedRevision":"` + revision + `","blockHash":"0x` + strings.Repeat("a", 64) + `","trader":"` + trader + `","recipient":"` + trader + `","pool":"` + zero + `","amountIn":"","isBuy":false,"creationPath":"","name":"","symbol":"","feeStrategy":0,"optical":"` + zero + `","nftId":""}`)
	}
	for _, tc := range []struct {
		name string
		raw []byte
		status int
		code string
		reports int
	}{
		{"invalid request", []byte(`{}`), 422, "FIELD_INVALID", 0},
		{"incomplete projection", body(strings.Repeat("0", 64)), 500, "INTERNAL_ERROR", 1},
		{"source unavailable", body(authority.engine.Revision().RevisionSHA256), 503, "DEPENDENCY_UNAVAILABLE", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/v2/preflight/trade", bytes.NewReader(tc.raw))
			response := httptest.NewRecorder()
			reports := 0
			servePreflight(response, request, authority.engine, capabilities.Trading, tc.raw, func(error) { reports++ })
			if response.Code != tc.status || reports != tc.reports {
				t.Fatalf("preflight response: %d %s, reports=%d", response.Code, response.Body.String(), reports)
			}
			if tc.status == 200 {
				if err := interfaces.ValidateJSON("ConnectedCapabilitiesPreflight", response.Body.Bytes()); err != nil || !strings.Contains(response.Body.String(), tc.code) {
					t.Fatalf("preflight document: %s, %v", response.Body.String(), err)
				}
			} else if !strings.Contains(response.Body.String(), `"code":"`+tc.code+`"`) {
				t.Fatalf("preflight error: %s", response.Body.String())
			}
		})
	}
}
