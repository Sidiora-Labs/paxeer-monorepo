package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/db/sqlcdb"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	"github.com/jackc/pgx/v5"
)

// AuditedDeadLetters provides the security policy's inspect/retention operations
// without requiring the indexer module to import a sibling module. DB is the
// durable operator journal/quarantine database, never the compatibility queue.
type AuditedDeadLetters struct {
	DB                                                      sqlcdb.DBTX
	Verifier                                                auth.ServiceAssertionVerifier
	DeliverySchema, SecuritySchema, Audience, PolicyVersion string
}

type deadLetterMetadata struct {
	Consumer     string    `json:"consumer"`
	Generation   string    `json:"generation"`
	EnvelopeHash string    `json:"envelopeHash"`
	FailureClass string    `json:"failureClass"`
	Created      time.Time `json:"created"`
}

type deadLetterCursor struct {
	Consumer     string `json:"consumer"`
	Generation   string `json:"generation"`
	EnvelopeHash string `json:"envelopeHash"`
}

var operationalSchema = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
var operationalDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)

func (d AuditedDeadLetters) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /status/dead-letter", func(w http.ResponseWriter, r *http.Request) { d.serve(w, r, "inspect") })
	mux.HandleFunc("POST /status/dead-letter/drain", func(w http.ResponseWriter, r *http.Request) { d.serve(w, r, "retention-delete") })
	return mux
}

func (d AuditedDeadLetters) serve(w http.ResponseWriter, r *http.Request, scope string) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if d.DB == nil || !operationalSchema.MatchString(d.DeliverySchema) || !operationalSchema.MatchString(d.SecuritySchema) || d.Audience == "" || d.Verifier.Audience != d.Audience || d.PolicyVersion == "" {
		http.Error(w, "dead-letter policy unavailable", http.StatusServiceUnavailable)
		return
	}
	principal, verifyErr := d.Verifier.Authenticate(r, nil)
	allowed := false
	actor := ""
	if verifyErr == nil {
		var audience string
		var issued, expires, mfa time.Time
		var scopes []string
		err := d.DB.QueryRow(ctx, "SELECT actor,audience,issued_at,expires_at,mfa_expires_at,scopes FROM "+pgx.Identifier{d.SecuritySchema, "operator_sessions"}.Sanitize()+" WHERE session_id=$1 AND wallet=$2 AND NOT revoked AND expires_at>clock_timestamp()", principal.SessionID, principal.Wallet).Scan(&actor, &audience, &issued, &expires, &mfa, &scopes)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "authorization unavailable", http.StatusServiceUnavailable)
			return
		}
		now := time.Now()
		allowed = err == nil && actor != "" && len(actor) <= 160 && audience == d.Audience && !issued.After(now) && expires.After(now) && expires.After(issued) && expires.Sub(issued) <= 15*time.Minute && mfa.After(now) && mfa.Sub(now) <= 5*time.Minute && len(scopes) <= 32
		hasScope := false
		for _, granted := range scopes {
			if granted == scope {
				hasScope = true
			}
		}
		allowed = allowed && hasScope
	}
	requestID := r.Header.Get("X-Request-ID")
	if len(requestID) > 128 {
		requestID = ""
	}
	if len(actor) > 160 {
		actor = ""
	}
	_, err := d.DB.Exec(ctx, "INSERT INTO "+pgx.Identifier{d.SecuritySchema, "authorization_decisions"}.Sanitize()+"(actor,scope,request_id,policy_version,allowed) VALUES($1,$2,$3,$4,$5)", actor, scope, requestID, d.PolicyVersion, allowed)
	if err != nil {
		http.Error(w, "audit unavailable", http.StatusServiceUnavailable)
		return
	}
	if verifyErr != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if scope == "retention-delete" {
		http.Error(w, "separate retention approval required", http.StatusForbidden)
		return
	}
	d.inspect(ctx, w, r)
}

func (d AuditedDeadLetters) inspect(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			http.Error(w, "invalid pagination", http.StatusBadRequest)
			return
		}
		limit = n
	}
	cursor := deadLetterCursor{}
	after := r.URL.Query().Get("after")
	if limit < 1 || limit > 100 || len(after) > 2048 {
		http.Error(w, "invalid pagination", http.StatusBadRequest)
		return
	}
	if after != "" {
		wire, err := base64.RawURLEncoding.Strict().DecodeString(after)
		decoder := json.NewDecoder(bytes.NewReader(wire))
		decoder.DisallowUnknownFields()
		if err != nil || decoder.Decode(&cursor) != nil || decoder.Decode(new(any)) != io.EOF || !operationalDigest.MatchString(cursor.EnvelopeHash) || len(cursor.Consumer) > 128 || len(cursor.Generation) > 128 {
			http.Error(w, "invalid pagination", http.StatusBadRequest)
			return
		}
	}
	rows, err := d.DB.Query(ctx, "SELECT consumer_id,projection_generation,envelope_hash,failure_class,created_at FROM "+pgx.Identifier{d.DeliverySchema, "delivery_quarantine"}.Sanitize()+" WHERE (envelope_hash,consumer_id,projection_generation)>($1,$2,$3) ORDER BY envelope_hash,consumer_id,projection_generation LIMIT $4", cursor.EnvelopeHash, cursor.Consumer, cursor.Generation, limit)
	if err != nil {
		http.Error(w, "inspection unavailable", http.StatusServiceUnavailable)
		return
	}
	defer rows.Close()
	entries := []deadLetterMetadata{}
	for rows.Next() {
		var entry deadLetterMetadata
		if rows.Scan(&entry.Consumer, &entry.Generation, &entry.EnvelopeHash, &entry.FailureClass, &entry.Created) != nil || !operationalDigest.MatchString(entry.EnvelopeHash) || len(entry.Consumer) > 128 || len(entry.Generation) > 128 {
			http.Error(w, "inspection unavailable", http.StatusServiceUnavailable)
			return
		}
		switch entry.FailureClass {
		case "invalid-envelope", "unsupported-version", "projection-conflict", "schema-conflict", "handler-error", "retry-exhausted", "authority-unavailable":
		default:
			entry.FailureClass = "unclassified"
		}
		entries = append(entries, entry)
	}
	if rows.Err() != nil {
		http.Error(w, "inspection unavailable", http.StatusServiceUnavailable)
		return
	}
	next := ""
	if len(entries) == limit {
		last := entries[len(entries)-1]
		wire, _ := json.Marshal(deadLetterCursor{Consumer: last.Consumer, Generation: last.Generation, EnvelopeHash: last.EnvelopeHash})
		next = base64.RawURLEncoding.EncodeToString(wire)
	}
	sharedhttp.WriteJSON(w, http.StatusOK, map[string]any{"entries": entries, "nextCursor": next})
}
