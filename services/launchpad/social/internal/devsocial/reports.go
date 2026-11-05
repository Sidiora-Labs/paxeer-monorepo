package devsocial

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

// Reports (IF_REPORT_CREATE) over chat.v2_reports. Filing a report returns a
// tracking id and never hides content by itself; the target must exist for
// messages and comments and be well-formed for profiles, tokens and streams.

var addressRE = regexp.MustCompile(`^0x[0-9a-f]{40}$`)

var reportReasons = map[string]bool{"spam": true, "abuse": true, "fraud": true, "copyright": true, "other": true}

func (rt *Runtime) createReport(w http.ResponseWriter, r *http.Request) {
	wallet, ok := actor(r)
	if !ok {
		storeError(w, r, errUnauthenticated)
		return
	}
	key, err := idempotencyKey(r)
	if err != nil {
		storeError(w, r, err)
		return
	}
	var body v2.ReportCreateBody
	if !decode(w, r, "ReportCreateBody", &body) {
		return
	}
	reason := strings.ToLower(strings.TrimSpace(body.Reason))
	if !reportReasons[reason] {
		reason = "other"
	}
	if !plainText(body.Reason) || (body.Evidence != "" && !plainText(body.Evidence)) {
		storeError(w, r, errInvalid)
		return
	}
	target := strings.TrimSpace(body.TargetId)
	if body.TargetType == "profile" || body.TargetType == "token" {
		target = strings.ToLower(target)
	}
	report, err := rt.insertReport(r.Context(), wallet, key, body.TargetType, target, reason, body.Reason, body.Evidence)
	if err != nil {
		storeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusCreated, "ReportCreateResponse", v2.ReportCreateData{Report: report}, nil)
}

func (rt *Runtime) reportTarget(ctx context.Context, q pgxQuerier, kind, id string) error {
	var exists bool
	var err error
	switch kind {
	case "message":
		err = q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM chat.chat_messages WHERE id=$1)`, id).Scan(&exists)
	case "comment":
		err = q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM chat.interaction_comments WHERE id=$1)`, id).Scan(&exists)
	case "profile", "token":
		exists = addressRE.MatchString(id)
		if !exists {
			return errInvalid
		}
	case "stream":
		_, e := uuid.Parse(id)
		if e != nil {
			return errInvalid
		}
		exists = true
	default:
		return errInvalid
	}
	if err != nil {
		return err
	}
	if !exists {
		return errNotFound
	}
	return nil
}

func (rt *Runtime) insertReport(ctx context.Context, wallet, key, kind, target, reason, reasonText, evidence string) (v2.Report, error) {
	hash := digest(kind, target, reasonText, evidence)
	tx, err := rt.pool.Begin(ctx)
	if err != nil {
		return v2.Report{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "report:"+wallet+":"+key); err != nil {
		return v2.Report{}, err
	}
	var out v2.Report
	var prior string
	err = tx.QueryRow(ctx, `SELECT report_id::text,reporter,target_type,target_id,reason,state,request_hash FROM chat.v2_reports WHERE reporter=$1 AND idempotency_key=$2`, wallet, key).Scan(&out.ReportId, &out.ReporterWallet, &out.TargetType, &out.TargetId, &out.Reason, &out.State, &prior)
	if err == nil {
		if prior != hash {
			return v2.Report{}, errIdempotency
		}
		if err = rt.intakeReport(ctx, tx, out, reasonText, evidence); err != nil {
			return v2.Report{}, err
		}
		return out, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return v2.Report{}, err
	}
	if err = rt.reportTarget(ctx, tx, kind, target); err != nil {
		return v2.Report{}, err
	}
	err = tx.QueryRow(ctx, `INSERT INTO chat.v2_reports(report_id,reporter,idempotency_key,request_hash,target_type,target_id,reason,reason_text,evidence) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
 RETURNING report_id::text,reporter,target_type,target_id,reason,state`, uuid.New(), wallet, key, hash, kind, target, reason, reasonText, evidence).Scan(&out.ReportId, &out.ReporterWallet, &out.TargetType, &out.TargetId, &out.Reason, &out.State)
	if err != nil {
		return v2.Report{}, err
	}
	if err = rt.intakeReport(ctx, tx, out, reasonText, evidence); err != nil {
		return v2.Report{}, err
	}
	return out, tx.Commit(ctx)
}

// intakeReport shares the report transaction: neither the frozen receipt nor
// its moderator case can commit alone. The intake event links each receipt to
// the canonical case even when several idempotency keys report the same target.
func (rt *Runtime) intakeReport(ctx context.Context, tx pgx.Tx, report v2.Report, reasonText, evidence string) error {
	id := uuid.NewSHA1(uuid.NameSpaceOID, []byte("frozen-report:"+report.ReportId)).String()
	var received bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM chat.safety_outbox WHERE id=$1)`, id).Scan(&received); err != nil {
		return err
	}
	if received {
		return nil
	}
	var pool, affected string
	var err error
	switch report.TargetType {
	case "message":
		err = tx.QueryRow(ctx, `SELECT CASE WHEN c.kind='pool' THEN substring(c.id from 6) ELSE '' END,m.sender FROM chat.chat_messages m JOIN chat.chat_conversations c ON c.id=m.conversation_id WHERE m.id=$1`, report.TargetId).Scan(&pool, &affected)
	case "comment":
		err = tx.QueryRow(ctx, `SELECT pool,author FROM chat.interaction_comments WHERE id=$1`, report.TargetId).Scan(&pool, &affected)
	case "profile":
		affected = report.TargetId
	case "token":
		query := `SELECT lower(pool_address),lower(creator) FROM indexer.pools WHERE lower(token_address)=$1 LIMIT 1`
		if rt.policy.Pools == CanonicalPools {
			query = `SELECT pool,creator FROM chat.social_canonical_pools() WHERE token=$1`
		}
		err = tx.QueryRow(ctx, query, report.TargetId).Scan(&pool, &affected)
	}
	if err != nil {
		return err
	}
	var caseID string
	var revision int64
	err = tx.QueryRow(ctx, `INSERT INTO chat.safety_cases(id,target_type,target_id,pool_id,affected_wallet,reporter,category,description) VALUES($1,$2,$3,$4,$5,$6,$7,$8)
 ON CONFLICT(reporter,target_type,target_id) DO UPDATE SET target_id=EXCLUDED.target_id RETURNING id::text,revision`, report.ReportId, report.TargetType, report.TargetId, pool, affected, report.ReporterWallet, reasonText, evidence).Scan(&caseID, &revision)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{"kind": "case_changed", "entityId": caseID, "revision": revision, "reportId": report.ReportId})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO chat.safety_outbox(id,kind,entity_id,revision,payload) VALUES($1,'case_changed',$2,$3,$4) ON CONFLICT(id) DO NOTHING`, id, caseID, revision, payload)
	return err
}
