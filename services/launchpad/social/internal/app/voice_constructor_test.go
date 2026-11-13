package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/voice"
	"github.com/jackc/pgx/v5"
)

func TestQualifiedVoiceConstructorBindsProviderAndPolicy(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.NewPostgres(t)
	runtime := &socialConnectedRuntime{pool: pool, safety: &safety.Store{DB: pool}}
	path := filepath.Join(t.TempDir(), "provider.log")
	if err := os.WriteFile(path, []byte("provider qualification complete\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	logBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	endpoint, apiKey, apiSecret := "https://"+strings.Join([]string{"fixture", "livekit", "cloud"}, "."), "fixture", strings.Repeat("s", 32)
	proof := voiceQualification{Version: 1, Provider: "livekit", Endpoint: endpoint, ExpiresAt: time.Now().Add(time.Hour)}
	proof.Gates = make(map[string]struct {
		Command   string `json:"command"`
		ExitCode  int    `json:"exitCode"`
		LogFile   string `json:"logFile"`
		LogSHA256 string `json:"logSHA256"`
	})
	for _, name := range []string{"provider", "ejection", "acl", "retention"} {
		proof.Gates[name] = struct {
			Command   string `json:"command"`
			ExitCode  int    `json:"exitCode"`
			LogFile   string `json:"logFile"`
			LogSHA256 string `json:"logSHA256"`
		}{Command: "qualified", LogFile: path, LogSHA256: fmt.Sprintf("%x", sha256.Sum256(logBytes))}
	}
	binding := config.VoiceRuntimeBinding{Endpoint: proof.Endpoint, APIKeyReference: "VOICE_API_KEY", APISecretReference: "VOICE_API_SECRET", PolicyVersion: "voice-v1"}
	secret := func(name string) (string, error) {
		switch name {
		case binding.APIKeyReference:
			return apiKey, nil
		case binding.APISecretReference:
			return apiSecret, nil
		default:
			return "", errors.New("unknown provider secret")
		}
	}
	writeProof := func(value voiceQualification) config.VoiceRuntimeBinding {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(t.TempDir(), "qualification.json")
		if err := os.WriteFile(file, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		candidate := binding
		candidate.QualificationFile = file
		candidate.QualificationSHA256 = fmt.Sprintf("%x", sha256.Sum256(raw))
		return candidate
	}
	qualified := writeProof(proof)
	control, err := runtime.newVoice(ctx, qualified, secret, pgx.Identifier{"chat", "social_pool_exists"})
	if err != nil || control == nil || control.Provider == nil || !control.Gates.Ready() || control.SafetyPolicyVersion != "voice-v1" {
		t.Fatalf("qualified voice control = %+v: %v", control, err)
	}
	for _, tc := range []struct {
		name  string
		proof voiceQualification
	}{
		{name: "expired provider", proof: func() voiceQualification { v := proof; v.ExpiresAt = time.Now().Add(-time.Minute); return v }()},
		{name: "different provider", proof: func() voiceQualification { v := proof; v.Provider = "other"; return v }()},
		{name: "missing gate", proof: func() voiceQualification {
			v := proof
			v.Gates = make(map[string]struct {
				Command   string `json:"command"`
				ExitCode  int    `json:"exitCode"`
				LogFile   string `json:"logFile"`
				LogSHA256 string `json:"logSHA256"`
			}, len(proof.Gates))
			for k, gate := range proof.Gates {
				if k != "acl" {
					v.Gates[k] = gate
				}
			}
			return v
		}()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := runtime.newVoice(ctx, writeProof(tc.proof), secret, pgx.Identifier{"chat", "social_pool_exists"}); got != nil || !errors.Is(err, voice.ErrUnavailable) {
				t.Fatalf("voice qualification = %+v: %v", got, err)
			}
		})
	}
	badDigest := qualified
	badDigest.QualificationSHA256 = fmt.Sprintf("%064x", 0)
	if got, err := runtime.newVoice(ctx, badDigest, secret, nil); got != nil || !errors.Is(err, voice.ErrUnavailable) {
		t.Fatalf("qualification digest = %+v: %v", got, err)
	}
	if got, err := runtime.newVoice(ctx, qualified, func(string) (string, error) { return "", errors.New("secret unavailable") }, nil); got != nil || err == nil || err.Error() != "secret unavailable" {
		t.Fatalf("provider secret = %+v: %v", got, err)
	}
	recording := qualified
	recording.RecordingEnabled = true
	recording.RecordingAccessKeyReference = binding.APIKeyReference
	recording.RecordingSecretReference = binding.APISecretReference
	recording.RecordingEndpoint = "http:"
	if got, err := runtime.newVoice(ctx, recording, secret, pgx.Identifier{"chat", "social_pool_exists"}); got != nil || !errors.Is(err, voice.ErrUnavailable) {
		t.Fatalf("recording endpoint binding = %+v: %v", got, err)
	}
}
