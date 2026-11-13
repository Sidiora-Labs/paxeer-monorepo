package auth

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestIdentityMeasurementRequestRecordsActualChallengeOutcomes(t *testing.T) {
	_, cfg := measurementFixtureConfig(t)
	unavailable := httptest.NewServer(http.NotFoundHandler())
	unavailableURL := unavailable.URL
	unavailable.Close()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	wallet := strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())
	file, err := os.Create(filepath.Join(t.TempDir(), "requests.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	recorder := &IdentityMeasurementRecorder{
		file: file, digest: sha256.New(),
		summary:       IdentityMeasurementSummary{PhaseWindows: map[string]*IdentityPhaseWindow{}},
		phaseRequests: map[string]int64{}, phaseSuccessful: map[string]int64{}, phaseFailures: map[string]int64{},
	}
	phase := IdentityMeasurementPhase{Name: "challenge"}
	for _, tc := range []struct {
		name, wallet, endpoint, want string
		expected                     int
	}{
		{"issued challenge", wallet, cfg.Clients[0].BaseURL, "", 201},
		{"unexpected status", wallet, cfg.Clients[0].BaseURL, "unexpected_status", 200},
		{"rejected wallet", "invalid", cfg.Clients[0].BaseURL, "unexpected_status", 201},
		{"unavailable transport", wallet, unavailableURL, "transport_or_response_unavailable", 201},
		{"invalid request authority", wallet, "://invalid", "request construction failed", 201},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := cfg.Clients[0]
			client.BaseURL = tc.endpoint
			body, headers, err := recorder.request(context.Background(), client, phase, "challenge", "/v2/auth/challenges", "", "", "", ChallengeRequest{Wallet: tc.wallet, Origin: client.Origin}, tc.expected, time.Now(), time.Second)
			if tc.want == "" {
				var response Envelope[struct {
					Challenge Challenge `json:"challenge"`
				}]
				if err != nil || json.Unmarshal(body, &response) != nil || response.Data.Challenge.Wallet != wallet || headers.Get("X-Request-Id") == "" {
					t.Fatalf("actual challenge: response=%s headers=%v err=%v", body, headers, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) || body != nil {
				t.Fatalf("challenge outcome: body=%s headers=%v err=%v, want %s", body, headers, err, tc.want)
			}
		})
	}
	if recorder.summary.Requests != 4 || recorder.summary.SuccessfulRequests != 1 || recorder.summary.UnexpectedFailures != 3 || recorder.phaseFailures[phase.Name] != 3 || recorder.failure != nil {
		t.Fatalf("recorded challenge outcomes: summary=%+v failures=%v write=%v", recorder.summary, recorder.phaseFailures, recorder.failure)
	}
}
