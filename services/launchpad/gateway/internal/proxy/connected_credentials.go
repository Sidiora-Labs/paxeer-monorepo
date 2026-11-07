package proxy

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/google/uuid"
)

func deletionStatusRoute(route *ConnectedRoute) bool {
	return route.Method == http.MethodGet && (route.Template == "/v1/me/deletion-requests/{jobId}" || route.Template == "/v2/me/deletion-requests/{jobId}")
}

func deletionCreateRoute(route *ConnectedRoute) bool {
	return route.Method == http.MethodPost && (route.Template == "/v1/me/deletion-requests" || route.Template == "/v2/me/deletion-requests")
}

func validExportAuthorization(value string) bool {
	token := strings.TrimPrefix(value, "Bearer ")
	if len(token) != 64 {
		return false
	}
	raw, err := hex.DecodeString(token)
	return err == nil && hex.EncodeToString(raw) == token
}

// The authority verifies the MAC. The edge validates the bounded wire format
// and job binding before admitting a request without a normal session.
func validRecoveryHeader(header http.Header, jobID string) bool {
	values := header.Values("X-Recovery-Receipt")
	if len(values) != 1 || len(values[0]) != 98 {
		return false
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(values[0])
	if err != nil || len(raw) != 73 || raw[0] != 1 {
		return false
	}
	job, err := uuid.Parse(jobID)
	if err != nil || job == uuid.Nil || !bytes.Equal(job[:], raw[1:17]) {
		return false
	}
	account, err := uuid.FromBytes(raw[17:33])
	return err == nil && account != uuid.Nil && binary.BigEndian.Uint64(raw[33:41]) > uint64(time.Now().Unix())
}

func recoveryAccount(receipt string) string {
	raw, err := base64.RawURLEncoding.Strict().DecodeString(receipt)
	if err != nil || len(raw) != 73 {
		return ""
	}
	account, err := uuid.FromBytes(raw[17:33])
	if err != nil {
		return ""
	}
	return account.String()
}

var privateIngestKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// Only the authenticated owner ingest-key route may return this closed shape.
// Unknown and duplicate fields are refused, including nested secret fields.
func validIngestResponse(raw []byte, streamID string, now time.Time) bool {
	if len(raw) > 4096 {
		return false
	}
	if _, err := sharedauth.BodyDigest(raw); err != nil {
		return false
	}
	var response struct {
		StreamID  string `json:"streamId"`
		IngestKey struct {
			StreamKey       string `json:"streamKey"`
			IngestEndpoint  string `json:"ingestEndpoint"`
			VisibilityUntil string `json:"visibilityUntil"`
		} `json:"ingestKey"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&response) != nil || decoder.Decode(new(any)) != io.EOF || response.StreamID != streamID || !privateIngestKeyPattern.MatchString(response.IngestKey.StreamKey) {
		return false
	}
	id, err := uuid.Parse(response.StreamID)
	if err != nil || id == uuid.Nil || id.String() != response.StreamID {
		return false
	}
	endpoint, err := url.Parse(response.IngestKey.IngestEndpoint)
	if err != nil || len(response.IngestKey.IngestEndpoint) > 2048 || (endpoint.Scheme != "https" && endpoint.Scheme != "rtmps") || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.ForceQuery || endpoint.Fragment != "" || strings.ContainsAny(response.IngestKey.IngestEndpoint, "\r\n\\") {
		return false
	}
	expires, err := time.Parse(time.RFC3339, response.IngestKey.VisibilityUntil)
	return err == nil && expires.After(now) && !expires.After(now.Add(2*time.Minute+5*time.Second))
}
