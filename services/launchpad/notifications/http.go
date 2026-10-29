package notifications

import (
	"crypto/subtle"
	"encoding/json"
	"github.com/Sidiora-Technologies/KindleLaunch/core/discovery"
	"github.com/google/uuid"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Authenticate func(*http.Request) (string, error)
type HTTP struct {
	Store             Store
	Authenticate      Authenticate
	Discovery         discovery.Store
	Recipient         RecipientConsumer
	InternalAuthorize func(*http.Request, string) error
}

func (h HTTP) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v2/me/notifications", h.listV2Notifications)
	mux.HandleFunc("PUT /v2/me/notifications/{notificationId}/read", h.readV2Notification)
	mux.HandleFunc("GET /v2/discovery/notifications", func(w http.ResponseWriter, r *http.Request) {
		account, e := h.actor(r)
		if e != nil {
			writeError(w, r, http.StatusUnauthorized, "AUTH_REQUIRED")
			return
		}
		items, e := h.Store.List(r.Context(), account, 50)
		respond(w, items, e)
	})
	mux.HandleFunc("PATCH /v2/discovery/notifications/{id}", func(w http.ResponseWriter, r *http.Request) {
		account, e := h.actor(r)
		if e != nil {
			writeError(w, r, http.StatusUnauthorized, "AUTH_REQUIRED")
			return
		}
		var input struct {
			Read bool `json:"read"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
		decoder.DisallowUnknownFields()
		if e = decoder.Decode(&input); e != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		var extra any
		if e = decoder.Decode(&extra); e != io.EOF {
			writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		e = h.Store.SetRead(r.Context(), account, r.PathValue("id"), input.Read)
		respond(w, map[string]bool{"read": input.Read}, e)
	})
	mux.HandleFunc("DELETE /v2/discovery/notifications/{id}", func(w http.ResponseWriter, r *http.Request) {
		account, e := h.actor(r)
		if e != nil {
			writeError(w, r, http.StatusUnauthorized, "AUTH_REQUIRED")
			return
		}
		e = h.Store.DeleteItem(r.Context(), account, r.PathValue("id"))
		respond(w, map[string]bool{"deleted": e == nil}, e)
	})
	return mux
}

func notificationListQuery(raw string) (bool, string, int, error) {
	query, err := url.ParseQuery(raw)
	if err != nil {
		return false, "", 0, ErrInvalid
	}
	unreadOnly, cursor, limit := false, "", 50
	for key, values := range query {
		if len(values) != 1 {
			return false, "", 0, ErrInvalid
		}
		value := values[0]
		switch key {
		case "unreadOnly":
			if value != "true" && value != "false" {
				return false, "", 0, ErrInvalid
			}
			unreadOnly = value == "true"
		case "cursor":
			if len(value) < 1 || len(value) > 256 || !utf8.ValidString(value) {
				return false, "", 0, ErrInvalid
			}
			cursor = value
		case "limit":
			limit, err = strconv.Atoi(value)
			if err != nil || limit < 1 || limit > 100 {
				return false, "", 0, ErrInvalid
			}
		default:
			return false, "", 0, ErrInvalid
		}
	}
	return unreadOnly, cursor, limit, nil
}

func notificationRequestID(r *http.Request) string {
	id, err := uuid.Parse(r.Header.Get("X-Request-Id"))
	if err != nil {
		id = uuid.New()
	}
	return id.String()
}

func writeV2Notification(w http.ResponseWriter, r *http.Request, raw []byte, err error) {
	if err != nil {
		status, code := errorStatus(err)
		writeError(w, r, status, code)
		return
	}
	respond(w, json.RawMessage(raw), nil)
}

var notificationWalletPattern = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)

func (h HTTP) listV2Notifications(w http.ResponseWriter, r *http.Request) {
	account, err := h.actor(r)
	if err != nil {
		writeError(w, r, http.StatusUnauthorized, "AUTH_REQUIRED")
		return
	}
	wallet := r.Header.Get("X-Actor-Wallet")
	if !notificationWalletPattern.MatchString(wallet) {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	unreadOnly, cursor, limit, err := notificationListQuery(r.URL.RawQuery)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	if h.Store.DB == nil {
		writeV2Notification(w, r, nil, ErrUnavailable)
		return
	}
	page, err := h.Store.ListPage(r.Context(), account, unreadOnly, cursor, limit)
	if err != nil {
		writeV2Notification(w, r, nil, err)
		return
	}
	wire := V2NotificationsPage{Items: make([]V2NotificationItem, 0, len(page.Items))}
	if page.NextCursor != "" {
		wire.NextCursor = &page.NextCursor
	}
	for _, item := range page.Items {
		wire.Items = append(wire.Items, V2NotificationItem{
			InboxItem: item.InboxItem, V2ID: item.V2ID, Wallet: wallet,
			ReadAt: item.ReadAt, DeliveryState: item.DeliveryState,
		})
	}
	raw, err := AdaptV2Notifications(notificationRequestID(r), wire, time.Now().UTC())
	writeV2Notification(w, r, raw, err)
}

func (h HTTP) readV2Notification(w http.ResponseWriter, r *http.Request) {
	account, err := h.actor(r)
	if err != nil {
		writeError(w, r, http.StatusUnauthorized, "AUTH_REQUIRED")
		return
	}
	if !notificationWalletPattern.MatchString(r.Header.Get("X-Actor-Wallet")) {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	value := r.PathValue("notificationId")
	id, err := uuid.Parse(value)
	if err != nil || len(value) != 36 {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	if h.Store.DB == nil {
		writeV2Notification(w, r, nil, ErrUnavailable)
		return
	}
	if err = h.Store.SetReadV2(r.Context(), account, id.String(), true); err != nil {
		writeV2Notification(w, r, nil, err)
		return
	}
	var readAt time.Time
	err = h.Store.DB.QueryRowContext(r.Context(), `SELECT read_at FROM discovery_notifications.inbox WHERE account_id=$1 AND v2_id=$2::uuid`, account, id.String()).Scan(&readAt)
	if err != nil {
		writeV2Notification(w, r, nil, err)
		return
	}
	raw, err := AdaptV2NotificationRead(notificationRequestID(r), readAt, time.Now().UTC())
	writeV2Notification(w, r, raw, err)
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
func respond(w http.ResponseWriter, v any, e error) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", "application/json")
	if e != nil {
		status, code := errorStatus(e)
		writeError(w, nil, status, code)
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

type ChatRequest struct {
	EventID, AccountID, ResourceID string
	CreatedAt                      time.Time
}
type ChatResponse struct {
	NotificationID string
	Created        bool
}
type InternalPolicy struct {
	Credentials                          *Credentials
	TokenRef, DeletePeerDNS, ChatPeerDNS string
}

func (p InternalPolicy) Authorize(r *http.Request, account string) error {
	if account == "" || r.TLS == nil || len(r.TLS.VerifiedChains) == 0 || p.Credentials == nil {
		return ErrForbidden
	}
	peer := ""
	if r.Method == "DELETE" {
		peer = p.DeletePeerDNS
	} else if r.Method == "POST" && r.URL.Path == "/internal/v2/notifications/chat" {
		peer = p.ChatPeerDNS
	}
	if peer == "" || len(r.TLS.PeerCertificates) == 0 || r.TLS.PeerCertificates[0].VerifyHostname(peer) != nil {
		return ErrForbidden
	}
	token, e := p.Credentials.Get(p.TokenRef)
	if e != nil {
		return ErrUnavailable
	}
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") || subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(header, "Bearer ")), []byte(token)) != 1 {
		return ErrForbidden
	}
	return nil
}
func (h HTTP) InternalHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /internal/v2/accounts/{accountId}/notification-data", func(w http.ResponseWriter, r *http.Request) {
		account := r.PathValue("accountId")
		deletion := r.URL.Query().Get("deletionId")
		uuid := regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
		if !uuid.MatchString(account) || !uuid.MatchString(deletion) {
			writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		if h.InternalAuthorize == nil {
			writeError(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE")
			return
		}
		if e := h.InternalAuthorize(r, account); e != nil {
			respond(w, nil, e)
			return
		}
		if h.Discovery.DB == nil {
			writeError(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE")
			return
		}
		if e := h.Discovery.DeleteAccount(r.Context(), account); e != nil {
			respond(w, nil, e)
			return
		}
		e := h.Store.DeleteAccount(r.Context(), account)
		respond(w, map[string]bool{"deleted": e == nil}, e)
	})
	mux.HandleFunc("POST /internal/v2/notifications/chat", func(w http.ResponseWriter, r *http.Request) {
		var in ChatRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if e := decoder.Decode(&in); e != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		var extra any
		if e := decoder.Decode(&extra); e != io.EOF {
			writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		if h.InternalAuthorize == nil {
			writeError(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE")
			return
		}
		if e := h.InternalAuthorize(r, in.AccountID); e != nil {
			respond(w, nil, e)
			return
		}
		n, created, e := h.Recipient.EnqueueChat(r.Context(), in.EventID, in.AccountID, in.ResourceID, in.CreatedAt)
		respond(w, ChatResponse{NotificationID: n.ID, Created: created}, e)
	})
	return mux
}
