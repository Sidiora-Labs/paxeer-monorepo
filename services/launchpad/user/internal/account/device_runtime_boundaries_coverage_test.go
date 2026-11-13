package account

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/google/uuid"
)

func TestDeviceRegistrationRequiresRuntimeSessionAndProof(t *testing.T) {
	s := fixture(t)
	p := owner(t, s, 9701)
	device := uuid.New()
	key := bytes.Repeat([]byte{7}, 32)
	runtime := &Runtime{EndpointKey: key, EndpointFingerprintKey: key, DeviceVerifier: CompactDeviceProofVerifier(ed25519.PublicKey(key))}
	for _, tc := range []struct {
		name     string
		runtime  *Runtime
		identity *sharedauth.IdentityPrincipal
		want     int
	}{
		{"missing runtime", nil, nil, http.StatusServiceUnavailable},
		{"missing session", runtime, nil, http.StatusUnauthorized},
		{"invalid proof", runtime, &sharedauth.IdentityPrincipal{SessionID: uuid.NewString()}, http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPut, "/v2/me/devices/"+device.String(), strings.NewReader(`{"platform":"web","provider":"webpush","token":"token","deviceProof":"invalid"}`))
			r.SetPathValue("deviceId", device.String())
			r = r.WithContext(WithPrincipal(r.Context(), p))
			if tc.identity != nil {
				r = r.WithContext(context.WithValue(r.Context(), identityContextKey{}, *tc.identity))
			}
			w := httptest.NewRecorder()
			(HTTP{Store: s, Runtime: tc.runtime}).devicePut(w, r)
			if w.Code != tc.want {
				t.Fatalf("device registration = %d %s, want %d", w.Code, w.Body.String(), tc.want)
			}
		})
	}
}
