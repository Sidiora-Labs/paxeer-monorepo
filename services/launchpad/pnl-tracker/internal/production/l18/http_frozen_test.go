package l18

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	iface "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestPrepareClaimRejectsRequestsOutsideFrozenRewardClaim(t *testing.T) {
	const wallet = "0x1111111111111111111111111111111111111111"
	const key = "11111111-1111-4111-8111-111111111111"
	const body = `{"rewardIds":["creator:1"],"recipient":"` + wallet + `"}`
	for _, tc := range []struct {
		name   string
		body   string
		key    string
		status int
		code   string
	}{
		{
			name:   "missing Idempotency-Key",
			body:   body,
			status: http.StatusUnprocessableEntity,
			code:   "INVALID_REQUEST",
		},
		{
			name:   "malformed JSON",
			body:   `{"rewardIds":`,
			key:    key,
			status: http.StatusBadRequest,
			code:   "MALFORMED_JSON",
		},
		{
			name:   "extra body field",
			body:   `{"rewardIds":["creator:1"],"recipient":"` + wallet + `","extra":true}`,
			key:    key,
			status: http.StatusUnprocessableEntity,
			code:   "INVALID_REQUEST",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := HTTP{
				Service: &JourneyService{},
				Authenticate: func(*http.Request) (Actor, error) {
					return Actor{Wallet: wallet}, nil
				},
			}
			req := httptest.NewRequest(http.MethodPost, "/v2/me/rewards:prepare-claim", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			if tc.key != "" {
				req.Header.Set("Idempotency-Key", tc.key)
			}
			response := httptest.NewRecorder()
			h.PrepareClaim(response, req)

			if response.Code != tc.status {
				t.Errorf("status = %d, want %d; body = %s", response.Code, tc.status, response.Body.String())
			}
			if err := iface.ValidateJSON("Error", response.Body.Bytes()); err != nil {
				t.Fatalf("response violates frozen Error schema: %v; body = %s", err, response.Body.String())
			}
			var envelope struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.Code != tc.code {
				t.Errorf("code = %q, want %q", envelope.Code, tc.code)
			}
		})
	}
}
