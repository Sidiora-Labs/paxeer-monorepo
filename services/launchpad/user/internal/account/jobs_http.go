package account

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func identitySession(ctx context.Context) (uuid.UUID, error) {
	p, ok := ctx.Value(identityContextKey{}).(sharedauth.IdentityPrincipal)
	if !ok {
		return uuid.Nil, errAuthExpired
	}
	id, e := uuid.Parse(p.SessionID)
	if e != nil || id == uuid.Nil {
		return uuid.Nil, ErrDenied
	}
	return id, nil
}

type WireJob struct {
	JobID                 uuid.UUID         `json:"jobId"`
	AccountID             uuid.UUID         `json:"accountId"`
	Kind                  string            `json:"kind"`
	State                 string            `json:"state"`
	RequestedAt           string            `json:"requestedAt"`
	ExpiresAt             *string           `json:"expiresAt"`
	DomainResults         []any             `json:"domainResults"`
	RetainedReasons       []RetentionReason `json:"retainedReasons"`
	DownloadURL           *string           `json:"downloadUrl"`
	DownloadAuthorization *string           `json:"downloadAuthorization,omitempty"`
}

func wireJob(j Job) WireJob {
	state := "running"
	switch j.State {
	case "requested":
		state = "queued"
	case "ready", "completed":
		state = "completed"
	case "failed", "expired":
		state = "failed"
	case "blocked":
		state = "partial"
	}
	return WireJob{JobID: j.ID, AccountID: j.AccountID, Kind: j.Kind, State: state, RequestedAt: j.RequestedAt.UTC().Format("2006-01-02T15:04:05.000Z"), DomainResults: []any{}, RetainedReasons: []RetentionReason{}}
}
func (h HTTP) exportPost(w http.ResponseWriter, r *http.Request) {
	id := requestID(r)
	p, e := authenticated(r)
	if e != nil {
		WriteError(w, e, id)
		return
	}
	var v ExportBody
	if e = Decode(r.Body, &v); e == nil {
		e = v.Validate()
	}
	if e != nil {
		WriteError(w, e, id)
		return
	}
	if *v.IncludeMedia {
		WriteError(w, errors.New("FEATURE_UNAVAILABLE"), id)
		return
	}
	j, e := h.Store.RequestExport(r.Context(), p)
	if e != nil {
		WriteError(w, e, id)
		return
	}
	envelope, e := Response(id, map[string]any{"job": wireJob(j)}, "0", h.Store.now())
	if e != nil {
		WriteError(w, e, id)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(envelope)
}
func (h HTTP) jobGet(w http.ResponseWriter, r *http.Request, kind string) {
	id := requestID(r)
	p, e := authenticated(r)
	if e != nil {
		WriteError(w, e, id)
		return
	}
	jobID, e := uuid.Parse(r.PathValue("jobId"))
	if e != nil {
		WriteError(w, ErrInvalid, id)
		return
	}
	var j Job
	var payload []byte
	var expires *time.Time
	e = h.Store.DB.QueryRow(r.Context(), `SELECT job_id,account_id,kind,state,requested_at,expires_at,payload FROM account_job WHERE job_id=$1 AND account_id=$2 AND kind=$3`, jobID, p.AccountID, kind).Scan(&j.ID, &j.AccountID, &j.Kind, &j.State, &j.RequestedAt, &expires, &payload)
	if errors.Is(e, pgx.ErrNoRows) {
		e = ErrMissing
	}
	if e != nil {
		WriteError(w, e, id)
		return
	}
	v := wireJob(j)
	if expires != nil {
		at := expires.UTC().Format("2006-01-02T15:04:05.000Z")
		v.ExpiresAt = &at
	}
	if kind == "deletion" {
		projection := deletionProjection(j)
		v.RetainedReasons = projection.Job.RetainedReasons
		var progress struct {
			Acks []string `json:"acks"`
		}
		if e = json.Unmarshal(payload, &progress); e != nil {
			WriteError(w, e, id)
			return
		}
		acked := map[string]bool{}
		for _, domain := range progress.Acks {
			acked[domain] = true
		}
		for _, d := range projection.Job.DomainResults {
			if acked[d.Domain] {
				d.State = "completed"
			}
			v.DomainResults = append(v.DomainResults, d)
		}
	}
	if kind == "export" {
		h.respondExportStatus(w, r, j, expires, payload)
		return
	}
	h.respond(w, id, map[string]any{"job": v, "downloadUrl": v.DownloadURL}, "0", nil)
}
func (h HTTP) respondExportStatus(w http.ResponseWriter, r *http.Request, j Job, expires *time.Time, payload []byte) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	v, e := h.exportStatusProjection(j, expires, payload)
	if e != nil {
		WriteError(w, e, requestID(r))
		return
	}
	h.respond(w, requestID(r), map[string]any{"job": v, "downloadUrl": v.DownloadURL}, "0", nil)
}

func (h HTTP) exportStatusProjection(j Job, expires *time.Time, payload []byte) (WireJob, error) {
	v := wireJob(j)
	if expires != nil {
		at := expires.UTC().Format("2006-01-02T15:04:05.000Z")
		v.ExpiresAt = &at
	}
	if j.Kind != "export" || j.State != "ready" || expires == nil || !expires.After(h.Store.now()) || h.Runtime == nil {
		return v, nil
	}
	var metadata exportMetadata
	if e := json.Unmarshal(payload, &metadata); e != nil {
		return v, e
	}
	if len(metadata.Authorization) == 0 {
		return v, nil
	}
	raw, e := Open(h.Runtime.ExportKey, metadata.Authorization, []byte(j.ID.String()))
	if e != nil {
		return v, e
	}
	token := string(raw)
	if !validExportAuthorization(token) {
		return v, ErrDenied
	}
	base, e := url.Parse(strings.TrimRight(h.Runtime.DownloadBaseURL, "/"))
	if e != nil || SafeLink(h.Runtime.DownloadBaseURL) != nil || base.RawQuery != "" || base.Fragment != "" {
		return v, ErrInvalid
	}
	base.Path += "/v2/me/exports/" + j.ID.String() + "/download"
	base.RawPath = ""
	base.RawQuery = url.Values{"authorization": {token}}.Encode()
	downloadURL := base.String()
	v.DownloadAuthorization = &token
	v.DownloadURL = &downloadURL
	return v, nil
}

func (h HTTP) exportGet(w http.ResponseWriter, r *http.Request) { h.jobGet(w, r, "export") }
func (h HTTP) deletionGet(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Recovery-Receipt") != "" {
		p, e := h.recoveryPrincipal(r)
		if e != nil {
			WriteError(w, e, requestID(r))
			return
		}
		r = r.WithContext(WithPrincipal(r.Context(), p))
	}
	h.jobGet(w, r, "deletion")
}
func (h HTTP) deletionPost(w http.ResponseWriter, r *http.Request) {
	id := requestID(r)
	p, e := authenticated(r)
	if e != nil {
		WriteError(w, e, id)
		return
	}
	if h.Runtime == nil || h.Runtime.Authorizer == nil {
		WriteError(w, errors.New("DEPENDENCY_UNAVAILABLE"), id)
		return
	}
	body, e := io.ReadAll(io.LimitReader(r.Body, 16385))
	if e != nil || len(body) > 16384 {
		WriteError(w, ErrInvalid, id)
		return
	}
	identity, ok := r.Context().Value(identityContextKey{}).(sharedauth.IdentityPrincipal)
	if !ok {
		WriteError(w, errAuthExpired, id)
		return
	}
	session, e := identitySession(r.Context())
	if e != nil {
		WriteError(w, e, id)
		return
	}
	key, e := uuid.Parse(r.Header.Get("Idempotency-Key"))
	if e != nil || key == uuid.Nil {
		WriteError(w, ErrInvalid, id)
		return
	}
	a := h.Runtime.Authorizer
	req, e := a.AuthorizeRequest(r.Context(), r, identity, "account.delete", identity.Wallet, body)
	if e != nil {
		WriteError(w, ErrDenied, id)
		return
	}
	result, _, e := h.Store.RequestDeletionOperation(r.Context(), p, a.Ledger, req, session, key, a.Verifier.Verify)
	if e != nil {
		WriteError(w, e, id)
		return
	}
	var response Envelope[DeletionData]
	if e = json.Unmarshal(result, &response); e != nil {
		WriteError(w, e, id)
		return
	}
	receipt, e := h.recoveryReceipt(response.Data.Job)
	if e != nil {
		WriteError(w, e, id)
		return
	}
	w.Header().Set("X-Recovery-Receipt", receipt)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write(result)
}

// An explicit URL capability survives generated SDK decoding. Header clients
// retain the same authorization contract; ambiguous credentials are refused.
func exportDownloadAuthorization(r *http.Request) (string, error) {
	query, e := url.ParseQuery(r.URL.RawQuery)
	if e != nil {
		return "", ErrDenied
	}
	for name, values := range query {
		if name != "authorization" || len(values) != 1 || values[0] == "" {
			return "", ErrDenied
		}
	}
	headers := r.Header.Values("X-Export-Authorization")
	if len(headers) > 1 {
		return "", ErrDenied
	}
	header := strings.TrimPrefix(r.Header.Get("X-Export-Authorization"), "Bearer ")
	token := query.Get("authorization")
	if token != "" && header != "" && token != header {
		return "", ErrDenied
	}
	if token == "" {
		token = header
	}
	if !validExportAuthorization(token) {
		return "", ErrDenied
	}
	return token, nil
}

func validExportAuthorization(token string) bool {
	if len(token) != 64 {
		return false
	}
	for _, c := range token {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func (h HTTP) DownloadHandler() http.Handler {
	return h.downloadHandler(h.Store.exportDownloads())
}

func (h HTTP) downloadHandler(downloads exportDownloads) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		p, e := authenticated(r)
		if e != nil {
			WriteError(w, e, requestID(r))
			return
		}
		job, e := uuid.Parse(r.PathValue("jobId"))
		token, authorizationErr := exportDownloadAuthorization(r)
		if e != nil || authorizationErr != nil || len(token) != 64 || h.Runtime == nil || h.Runtime.Objects == nil {
			WriteError(w, ErrDenied, requestID(r))
			return
		}
		claim, e := downloads.claim(r.Context(), p, job, token)
		if e != nil {
			WriteError(w, e, requestID(r))
			return
		}
		// Cleanup has its own bounded context so a cancelled request can release
		// its claim. A crash or failed cleanup is recovered by lease expiry.
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = downloads.release(ctx, claim)
		}()
		var v exportMetadata
		if e = json.Unmarshal(claim.Payload, &v); e != nil {
			WriteError(w, e, requestID(r))
			return
		}
		sealed, _, e := h.Runtime.Objects.GetBytes(r.Context(), v.EncryptedObject)
		if e != nil {
			WriteError(w, e, requestID(r))
			return
		}
		plain, e := Open(h.Runtime.ExportKey, sealed, []byte(job.String()))
		if e != nil {
			WriteError(w, e, requestID(r))
			return
		}
		// Delivery commits here, after retrieval and authentication of the full
		// object and before the first response byte. A failed/partial socket
		// write remains consumed: it must never permit a second delivery.
		if e = r.Context().Err(); e == nil {
			e = downloads.consume(r.Context(), claim)
		}
		if e != nil {
			WriteError(w, e, requestID(r))
			return
		}
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=account-export.json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write(plain)
	})
}

func (h HTTP) recoveryReceipt(j WireAccountJob) (string, error) {
	if h.Runtime == nil || len(h.Runtime.EndpointKey) != 32 {
		return "", errors.New("DEPENDENCY_UNAVAILABLE")
	}
	job, e := uuid.Parse(j.JobID)
	if e != nil {
		return "", e
	}
	account, e := uuid.Parse(j.AccountID)
	if e != nil {
		return "", e
	}
	raw := make([]byte, 41)
	raw[0] = 1
	copy(raw[1:17], job[:])
	copy(raw[17:33], account[:])
	binary.BigEndian.PutUint64(raw[33:], uint64(h.Store.now().Add(30*24*time.Hour).Unix()))
	m := hmac.New(sha256.New, h.Runtime.EndpointKey)
	m.Write([]byte("account-deletion-recovery-v1"))
	m.Write(raw)
	return base64.RawURLEncoding.EncodeToString(append(raw, m.Sum(nil)...)), nil
}
func (h HTTP) recoveryPrincipal(r *http.Request) (Principal, error) {
	if h.Runtime == nil || len(h.Runtime.EndpointKey) != 32 {
		return Principal{}, ErrDenied
	}
	raw, e := base64.RawURLEncoding.Strict().DecodeString(r.Header.Get("X-Recovery-Receipt"))
	if e != nil || len(raw) != 73 || raw[0] != 1 {
		return Principal{}, ErrDenied
	}
	m := hmac.New(sha256.New, h.Runtime.EndpointKey)
	m.Write([]byte("account-deletion-recovery-v1"))
	m.Write(raw[:41])
	if !hmac.Equal(raw[41:], m.Sum(nil)) || uint64(h.Store.now().Unix()) >= binary.BigEndian.Uint64(raw[33:41]) {
		return Principal{}, ErrDenied
	}
	job, e := uuid.Parse(r.PathValue("jobId"))
	if e != nil || !bytes.Equal(job[:], raw[1:17]) {
		return Principal{}, ErrDenied
	}
	account, e := uuid.FromBytes(raw[17:33])
	if e != nil || account == uuid.Nil {
		return Principal{}, ErrDenied
	}
	return Principal{AccountID: account}, nil
}
