package account

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
)

func (h HTTP) watchV2Get(w http.ResponseWriter, r *http.Request) {
	id := requestID(r)
	items, revision, next, err := h.watchPage(r)
	if err != nil {
		WriteError(w, err, id)
		return
	}
	envelope, err := Response(id, items, revision, h.Store.now())
	if err != nil {
		WriteError(w, err, id)
		return
	}
	envelope.NextCursor = next
	raw, err := AdaptV2Watchlist(envelope)
	writeV2AccountResponse(w, id, raw, err)
}

func (h HTTP) watchV2Put(w http.ResponseWriter, r *http.Request) {
	id := requestID(r)
	entry, revision, err := h.putWatch(r)
	if err != nil {
		WriteError(w, err, id)
		return
	}
	envelope, err := Response(id, entry, revision, h.Store.now())
	if err != nil {
		WriteError(w, err, id)
		return
	}
	raw, err := AdaptV2WatchPut(envelope)
	writeV2AccountResponse(w, id, raw, err)
}

func (h HTTP) watchV2Delete(w http.ResponseWriter, r *http.Request) {
	h.watchDelete(w, r)
}

func (h HTTP) profileV2Patch(w http.ResponseWriter, r *http.Request) {
	id := requestID(r)
	profile, err := h.patchProfile(r)
	if err != nil {
		if errors.Is(err, ErrConflict) {
			_, wire := PublicError(err, id)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "private, no-store")
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(wire)
			return
		}
		WriteError(w, err, id)
		return
	}
	envelope, err := Response(id, profile, profile.Revision, h.Store.now())
	if err != nil {
		WriteError(w, err, id)
		return
	}
	raw, err := AdaptV2ProfilePatch(envelope)
	writeV2AccountResponse(w, id, raw, err)
}

func writeV2AccountResponse(w http.ResponseWriter, id uuid.UUID, raw []byte, err error) {
	if err != nil {
		WriteError(w, err, id)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}
