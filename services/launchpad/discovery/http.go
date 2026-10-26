package discovery

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
)

type Authenticate func(*http.Request) (string, error)
type HTTP struct {
	Feed         Feed
	Callouts     CalloutService
	Store        Store
	Authenticate Authenticate
	Intervals    IntervalAuthority
}

func (h HTTP) actor(r *http.Request) (string, error) {
	if h.Authenticate == nil {
		return "", ErrUnavailable
	}
	a, e := h.Authenticate(r)
	if e != nil || a == "" {
		return "", ErrForbidden
	}
	return a, nil
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return ErrInvalid
	}
	return nil
}
func reply(w http.ResponseWriter, r *http.Request, v any, e error) {
	if e != nil {
		status, code := errorStatus(e)
		writeError(w, r, status, code)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	_ = json.NewEncoder(w).Encode(v)
}
func (h HTTP) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v2/discovery/feed", func(w http.ResponseWriter, r *http.Request) {
		account := ""
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			var e error
			account, e = h.actor(r)
			if e != nil {
				writeError(w, r, http.StatusUnauthorized, "AUTH_REQUIRED")
				return
			}
		}
		order := r.URL.Query().Get("order")
		if order == "" {
			order = "chronological"
		}
		limit := 50
		if s := r.URL.Query().Get("limit"); s != "" {
			var e error
			limit, e = strconv.Atoi(s)
			if e != nil {
				writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST")
				return
			}
		}
		v, e := h.Feed.Page(r.Context(), account, order, r.URL.Query().Get("cursor"), limit, time.Now().UTC())
		reply(w, r, v, e)
	})
	mux.HandleFunc("GET /v2/discovery/feed/{id}", func(w http.ResponseWriter, r *http.Request) {
		account := ""
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			var e error
			account, e = h.actor(r)
			if e != nil {
				writeError(w, r, http.StatusUnauthorized, "AUTH_REQUIRED")
				return
			}
		}
		v, e := h.Feed.Detail(r.Context(), account, r.PathValue("id"), time.Now().UTC())
		reply(w, r, v, e)
	})
	mux.HandleFunc("POST /v2/discovery/callouts", func(w http.ResponseWriter, r *http.Request) {
		actor, e := h.actor(r)
		if e != nil {
			writeError(w, r, http.StatusUnauthorized, "AUTH_REQUIRED")
			return
		}
		var input struct{ ID, PoolAddress, Body, Direction, Disclosure, PositionDisclosure, SquadID string }
		if e = decode(w, r, &input); e != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		next := Callout{ID: input.ID, AuthorAccountID: actor, PoolAddress: input.PoolAddress, Body: input.Body, Direction: input.Direction, Disclosure: input.Disclosure, PositionDisclosure: input.PositionDisclosure, ModerationStatus: "approved", Revision: 1, CreatedAt: time.Now().UTC()}
		e = h.Callouts.Publish(r.Context(), actor, input.SquadID, next)
		reply(w, r, next, e)
	})
	mux.HandleFunc("PATCH /v2/discovery/callouts/{id}", func(w http.ResponseWriter, r *http.Request) {
		actor, e := h.actor(r)
		if e != nil {
			writeError(w, r, http.StatusUnauthorized, "AUTH_REQUIRED")
			return
		}
		var input struct {
			Body     string
			Expected uint64
			Delete   bool
		}
		if e = decode(w, r, &input); e != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		v, e := h.Callouts.Edit(r.Context(), actor, r.PathValue("id"), input.Body, input.Expected, input.Delete)
		reply(w, r, v, e)
	})
	mux.HandleFunc("PUT /v2/discovery/alerts/{id}", func(w http.ResponseWriter, r *http.Request) {
		actor, e := h.actor(r)
		if e != nil {
			writeError(w, r, http.StatusUnauthorized, "AUTH_REQUIRED")
			return
		}
		var input struct {
			Rule     AlertRule
			Expected uint64
		}
		if e = decode(w, r, &input); e != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		input.Rule.OwnerAccountID = actor
		input.Rule.ID = r.PathValue("id")
		e = h.Store.SaveRule(r.Context(), actor, input.Rule, input.Expected)
		reply(w, r, input.Rule, e)
	})
	mux.HandleFunc("POST /v2/squads", func(w http.ResponseWriter, r *http.Request) {
		actor, e := h.actor(r)
		if e != nil {
			writeError(w, r, http.StatusUnauthorized, "AUTH_REQUIRED")
			return
		}
		var input v2.SquadCreateBody
		if e = decodeV2Squad(w, r, "SquadCreateBody", &input); e != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		q := Squad{ID: uuid.NewString(), OwnerAccountID: actor, Name: input.Name, Description: input.Description, Visibility: input.Visibility, Policy: input.JoinPolicy, Revision: 1}
		if e = h.Store.CreateSquad(r.Context(), actor, q); e != nil {
			reply(w, r, nil, e)
			return
		}
		body, e := AdaptV2SquadCreate(squadRequestID(r), q, time.Now().UTC())
		if e == nil {
			var response v2.SquadCreateResponse
			e = json.Unmarshal(body, &response)
			response.Data.Squad.Description = q.Description
			response.Data.Squad.Visibility = q.Visibility
			if e == nil {
				body, e = marshalV2SquadEnvelope("SquadCreateResponse", response)
			}
		}
		replyV2Squad(w, r, http.StatusCreated, body, e)
	})
	mux.HandleFunc("GET /v2/squads/{squadId}", func(w http.ResponseWriter, r *http.Request) {
		actor, e := h.actor(r)
		if e != nil {
			writeError(w, r, http.StatusUnauthorized, "AUTH_REQUIRED")
			return
		}
		q, e := h.Store.Squad(r.Context(), r.PathValue("squadId"))
		if e != nil {
			reply(w, r, nil, e)
			return
		}
		members, e := h.squadMembers(r.Context(), q.ID)
		if e != nil {
			reply(w, r, nil, e)
			return
		}
		if q.Visibility == "private" {
			active := false
			for _, m := range members {
				if m.AccountID == actor && m.Status == "active" {
					active = true
				}
			}
			if !active {
				writeError(w, r, http.StatusNotFound, "NOT_FOUND")
				return
			}
		}
		body, e := AdaptV2SquadRead(squadRequestID(r), q, members, time.Now().UTC())
		if e == nil {
			var response v2.SquadReadResponse
			e = json.Unmarshal(body, &response)
			response.Data.Squad.Description = q.Description
			response.Data.Squad.Visibility = q.Visibility
			if e == nil {
				body, e = marshalV2SquadEnvelope("SquadReadResponse", response)
			}
		}
		replyV2Squad(w, r, http.StatusOK, body, e)
	})
	mux.HandleFunc("PUT /v2/squads/{squadId}/members/{accountId}", func(w http.ResponseWriter, r *http.Request) {
		actor, e := h.actor(r)
		if e != nil {
			writeError(w, r, http.StatusUnauthorized, "AUTH_REQUIRED")
			return
		}
		var input v2.SquadMembershipBody
		if e = decodeV2Squad(w, r, "SquadMembershipBody", &input); e != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		expected, e := strconv.ParseUint(string(input.ExpectedRevision), 10, 64)
		if e != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		id, account := r.PathValue("squadId"), r.PathValue("accountId")
		if _, e = uuid.Parse(id); e != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		if _, e = uuid.Parse(account); e != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		switch input.Action {
		case "invite":
			var existing Membership
			existing, e = h.squadMember(r.Context(), id, account)
			if errors.Is(e, sql.ErrNoRows) {
				e = nil
			}
			if e == nil && existing.Revision != expected {
				e = ErrConflict
			}
			if e == nil {
				e = h.Store.Invite(r.Context(), actor, id, account, uuid.NewString(), time.Now().UTC().Add(24*time.Hour))
			}
		case "request":
			if actor != account {
				e = ErrForbidden
			} else {
				_, e = h.Store.RequestJoin(r.Context(), actor, id, expected)
			}
		case "approve":
			_, e = h.Store.ApproveRequest(r.Context(), actor, id, account, expected, time.Now().UTC())
		case "remove":
			e = h.Store.SetMemberStatus(r.Context(), actor, id, account, "removed", expected)
		case "ban":
			e = h.Store.SetMemberStatus(r.Context(), actor, id, account, "banned", expected)
		case "promote":
			e = h.Store.SetMemberRole(r.Context(), actor, id, account, "moderator", expected)
		case "demote":
			e = h.Store.SetMemberRole(r.Context(), actor, id, account, "member", expected)
		default:
			e = ErrInvalid
		}
		if e != nil {
			reply(w, r, nil, e)
			return
		}
		m, e := h.squadMember(r.Context(), id, account)
		if e != nil {
			reply(w, r, nil, e)
			return
		}
		if m.Role == "" && m.JoinedAt.IsZero() {
			m.Role = "member"
		}
		body, e := AdaptV2SquadMembership(squadRequestID(r), m, time.Now().UTC())
		replyV2Squad(w, r, http.StatusOK, body, e)
	})
	mux.HandleFunc("POST /v2/discovery/squads", func(w http.ResponseWriter, r *http.Request) {
		actor, e := h.actor(r)
		if e != nil {
			writeError(w, r, http.StatusUnauthorized, "AUTH_REQUIRED")
			return
		}
		var input struct{ ID, Name, Policy string }
		if e = decode(w, r, &input); e != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		q := Squad{ID: input.ID, OwnerAccountID: actor, Name: input.Name, Policy: input.Policy, Revision: 1}
		e = h.Store.CreateSquad(r.Context(), actor, q)
		reply(w, r, q, e)
	})
	mux.HandleFunc("PATCH /v2/discovery/squads/{id}", func(w http.ResponseWriter, r *http.Request) {
		actor, e := h.actor(r)
		if e != nil {
			writeError(w, r, http.StatusUnauthorized, "AUTH_REQUIRED")
			return
		}
		var in struct {
			Squad    Squad
			Expected uint64
		}
		if e = decode(w, r, &in); e != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		in.Squad.ID = r.PathValue("id")
		e = h.Store.UpdateSquad(r.Context(), actor, in.Squad, in.Expected)
		reply(w, r, in.Squad, e)
	})
	mux.HandleFunc("PATCH /v2/discovery/squads/{id}/members/{account}", func(w http.ResponseWriter, r *http.Request) {
		actor, e := h.actor(r)
		if e != nil {
			writeError(w, r, http.StatusUnauthorized, "AUTH_REQUIRED")
			return
		}
		var in struct {
			Status   string
			Expected uint64
		}
		if e = decode(w, r, &in); e != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		e = h.Store.SetMemberStatus(r.Context(), actor, r.PathValue("id"), r.PathValue("account"), in.Status, in.Expected)
		reply(w, r, map[string]bool{"updated": e == nil}, e)
	})
	mux.HandleFunc("POST /v2/discovery/squads/{id}/invitations", func(w http.ResponseWriter, r *http.Request) {
		actor, e := h.actor(r)
		if e != nil {
			writeError(w, r, http.StatusUnauthorized, "AUTH_REQUIRED")
			return
		}
		var in struct {
			Account, Token string
			Expires        time.Time
		}
		if e = decode(w, r, &in); e != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		e = h.Store.Invite(r.Context(), actor, r.PathValue("id"), in.Account, in.Token, in.Expires)
		reply(w, r, map[string]bool{"invited": e == nil}, e)
	})
	mux.HandleFunc("POST /v2/discovery/squads/{id}/accept", func(w http.ResponseWriter, r *http.Request) {
		actor, e := h.actor(r)
		if e != nil {
			writeError(w, r, http.StatusUnauthorized, "AUTH_REQUIRED")
			return
		}
		var in struct {
			Token    string
			Expected uint64
		}
		if e = decode(w, r, &in); e != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		v, e := h.Store.AcceptInvitation(r.Context(), actor, r.PathValue("id"), in.Token, in.Expected, time.Now().UTC())
		reply(w, r, v, e)
	})
	mux.HandleFunc("POST /v2/discovery/squads/{id}/transfer", func(w http.ResponseWriter, r *http.Request) {
		actor, e := h.actor(r)
		if e != nil {
			writeError(w, r, http.StatusUnauthorized, "AUTH_REQUIRED")
			return
		}
		var in struct {
			Recipient string
			Expected  uint64
			Confirm   bool
		}
		if e = decode(w, r, &in); e != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		if in.Confirm {
			e = h.Store.ConfirmTransfer(r.Context(), actor, r.PathValue("id"))
		} else {
			e = h.Store.ProposeTransfer(r.Context(), actor, r.PathValue("id"), in.Recipient, in.Expected)
		}
		reply(w, r, map[string]bool{"accepted": e == nil}, e)
	})
	mux.HandleFunc("GET /v2/discovery/leaderboards", func(w http.ResponseWriter, r *http.Request) {
		start, e := time.Parse(time.RFC3339, r.URL.Query().Get("start"))
		if e != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		end, e := time.Parse(time.RFC3339, r.URL.Query().Get("end"))
		if e != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		v, e := RankIntervals(r.Context(), h.Intervals, start.UTC(), end.UTC())
		reply(w, r, v, e)
	})
	return mux
}

func decodeV2Squad(w http.ResponseWriter, r *http.Request, schema string, value any) error {
	var body json.RawMessage
	if err := decode(w, r, &body); err != nil {
		return err
	}
	if err := v2.ValidateJSON(schema, body); err != nil {
		return ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	return d.Decode(value)
}

func squadRequestID(r *http.Request) string {
	id := r.Header.Get("X-Request-Id")
	if _, err := uuid.Parse(id); err != nil {
		id = uuid.NewString()
	}
	return id
}

func replyV2Squad(w http.ResponseWriter, r *http.Request, status int, body []byte, err error) {
	if err != nil {
		reply(w, r, nil, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func (h HTTP) squadMembers(ctx context.Context, id string) ([]Membership, error) {
	if h.Store.DB == nil {
		return nil, ErrUnavailable
	}
	rows, err := h.Store.DB.QueryContext(ctx, `SELECT document FROM narrative_discovery.members WHERE squad_id=$1 ORDER BY account_id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := []Membership{}
	for rows.Next() {
		var body []byte
		var m Membership
		if err = rows.Scan(&body); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(body, &m); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (h HTTP) squadMember(ctx context.Context, id, account string) (Membership, error) {
	var m Membership
	if h.Store.DB == nil {
		return m, ErrUnavailable
	}
	var body []byte
	err := h.Store.DB.QueryRowContext(ctx, `SELECT document FROM narrative_discovery.members WHERE squad_id=$1 AND account_id=$2`, id, account).Scan(&body)
	if err == nil {
		err = json.Unmarshal(body, &m)
	}
	return m, err
}
