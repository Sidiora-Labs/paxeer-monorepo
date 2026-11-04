package user

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
)

const operationEnvironment = "launchpad-user-legacy"
const maxJSONBody = 1 << 20
const multipartOverhead = 16 << 10

// uploadIntent binds the complete wire body and its parser interpretation to the
// shared JSON operation digest, including multipart boundaries and MIME headers.
func uploadIntent(contentType string, body []byte) []byte {
	sum := sha256.Sum256(body)
	data, _ := json.Marshal(struct {
		ContentType string `json:"content_type"`
		SHA256      string `json:"sha256"`
	}{contentType, hex.EncodeToString(sum[:])})
	return data
}

// operationHeaders authenticates bounded consent before upload body parsing.
func (h *Handlers) operationHeaders(w http.ResponseWriter, r *http.Request, addr, action string) (sharedauth.OperationEnvelope, bool) {
	var e sharedauth.OperationEnvelope
	encoded, signatures := r.Header.Values(sharedauth.OperationEnvelopeHeader), r.Header.Values(sharedauth.OperationSignatureHeader)
	if len(encoded) != 1 || len(signatures) != 1 || encoded[0] == "" || signatures[0] == "" {
		sharedhttp.WriteError(w, http.StatusBadRequest, "Bad Request", "Missing signed operation")
		return e, false
	}
	deny := func() (sharedauth.OperationEnvelope, bool) {
		sharedhttp.WriteError(w, http.StatusForbidden, "Forbidden", "Invalid signed operation")
		return e, false
	}
	if len(encoded[0]) > 8192 || len(signatures[0]) != 132 {
		return deny()
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(encoded[0])
	if err != nil {
		return deny()
	}
	// The shared digest parser also rejects duplicate keys and trailing JSON.
	if _, err = sharedauth.BodyDigest(raw); err != nil {
		return deny()
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&e) != nil {
		return deny()
	}
	statement, err := e.Statement()
	now := h.clock().Unix()
	if err != nil || e.Environment != operationEnvironment || e.Actor != addr || e.Action != action || e.Method != r.Method || e.Path != r.URL.Path || r.URL.RawQuery != "" || r.URL.RawPath != "" || e.IssuedAt > now+30 || e.ExpiresAt <= now {
		return deny()
	}
	if _, err = sharedauth.VerifyEOA(addr, statement, signatures[0], nil); err != nil {
		return deny()
	}
	return e, true
}

func (h *Handlers) consumeOperation(w http.ResponseWriter, r *http.Request, e sharedauth.OperationEnvelope, body []byte) bool {
	now := h.clock()
	if err := e.Validate(now, operationEnvironment, e.Actor, e.Action, r.Method, r.URL.Path, body); err != nil {
		sharedhttp.WriteError(w, http.StatusForbidden, "Forbidden", "Operation content mismatch")
		return false
	}
	if h.redis == nil {
		sharedhttp.WriteError(w, http.StatusServiceUnavailable, "Service Unavailable", "Replay protection unavailable")
		return false
	}
	// Retain beyond the complete validity window, including allowed clock skew.
	// Never release a consumed nonce on mutation failure: a new consent is required.
	claimed, err := h.redis.SetNX(r.Context(), "user:operation:125:"+e.Actor+":"+e.Nonce, "used", time.Duration(e.ExpiresAt-now.Unix()+60)*time.Second).Result()
	if err != nil {
		h.logErr("claim operation nonce", err)
		sharedhttp.WriteError(w, http.StatusServiceUnavailable, "Service Unavailable", "Replay protection unavailable")
		return false
	}
	if !claimed {
		sharedhttp.WriteError(w, http.StatusForbidden, "Forbidden", "Operation already used")
		return false
	}
	return true
}

func readBoundedBody(w http.ResponseWriter, r *http.Request, limit int64) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		sharedhttp.WriteRequestBodyError(w, http.StatusRequestEntityTooLarge, sharedhttp.RuleMaxBytes, "request body exceeds budget or could not be read")
		return nil, false
	}
	return body, true
}
