package social

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/voice"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
)

type v2VoiceEnvelope[T any] struct {
	RequestID   string              `json:"requestId"`
	Data        T                   `json:"data"`
	Watermark   *v2.Watermark       `json:"watermark"`
	Revision    *v2.PositiveUInt256 `json:"revision"`
	Quality     string              `json:"quality"`
	NextCursor  *string             `json:"nextCursor"`
	GeneratedAt string              `json:"generatedAt"`
}

func marshalV2Voice[T any](name, requestID string, data T, revision int64, at time.Time) ([]byte, error) {
	if revision < 0 {
		return nil, fmt.Errorf("%s: negative revision", name)
	}
	var envelopeRevision *v2.PositiveUInt256
	if revision > 0 {
		value := v2.PositiveUInt256(strconv.FormatInt(revision, 10))
		envelopeRevision = &value
	}
	raw, err := json.Marshal(v2VoiceEnvelope[T]{
		RequestID: requestID, Data: data, Revision: envelopeRevision,
		Quality: "complete", GeneratedAt: at.UTC().Format("2006-01-02T15:04:05.000Z"),
	})
	if err != nil {
		return nil, err
	}
	if err = v2.ValidateJSON(name, raw); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return raw, nil
}

func adaptV2VoiceRoom(room voice.Room, hostAccountID, visibility string) v2.VoiceRoom {
	return v2.VoiceRoom{
		RoomId: room.ID, HostAccountId: hostAccountID,
		ScopeType: "pool", ScopeId: room.PoolID, Visibility: visibility,
		State: room.APIState(), Revision: v2.UInt256(strconv.FormatInt(room.Revision, 10)),
	}
}

func AdaptV2VoiceCreate(requestID string, room voice.Room, hostAccountID, visibility string, capabilities v2.VoiceCapabilities, at time.Time) ([]byte, error) {
	return marshalV2Voice("VoiceCreateResponse", requestID, v2.VoiceCreateData{
		Room: adaptV2VoiceRoom(room, hostAccountID, visibility), Capabilities: capabilities,
	}, room.Revision, at)
}

func AdaptV2VoiceRead(requestID string, room voice.Room, hostAccountID, visibility string, allowedActions []string, at time.Time) ([]byte, error) {
	if allowedActions == nil {
		allowedActions = []string{}
	}
	return marshalV2Voice("VoiceReadResponse", requestID, v2.VoiceReadData{
		Room: adaptV2VoiceRoom(room, hostAccountID, visibility), AllowedActions: allowedActions,
	}, room.Revision, at)
}

func AdaptV2VoiceEnd(requestID string, room voice.Room, hostAccountID, visibility string, at time.Time) ([]byte, error) {
	return marshalV2Voice("VoiceEndResponse", requestID, v2.VoiceEndData{
		Room: adaptV2VoiceRoom(room, hostAccountID, visibility),
	}, room.Revision, at)
}

func AdaptV2VoiceLeave(requestID, grantState string, revision int64, at time.Time) ([]byte, error) {
	return marshalV2Voice("VoiceLeaveResponse", requestID, v2.VoiceLeaveData{
		GrantState: grantState, Revision: v2.UInt256(strconv.FormatInt(revision, 10)),
	}, revision, at)
}

func AdaptV2VoiceJoinTicket(requestID, ticket, providerEndpoint string, expiresAt, at time.Time) ([]byte, error) {
	return marshalV2Voice("VoiceJoinTicketResponse", requestID, v2.VoiceJoinTicketData{
		Ticket: ticket, ProviderEndpoint: providerEndpoint,
		ExpiresAt: expiresAt.UTC().Format("2006-01-02T15:04:05.000Z"),
	}, 0, at)
}

func writeV2VoiceError(w http.ResponseWriter, r *http.Request, e error) {
	status := safety.HTTPStatus(e)
	code := "DEPENDENCY_UNAVAILABLE"
	switch status {
	case http.StatusBadRequest:
		code = "INVALID_REQUEST"
	case http.StatusUnauthorized:
		code = "AUTH_REQUIRED"
	case http.StatusForbidden:
		code = "SCOPE_DENIED"
	case http.StatusNotFound:
		code = "NOT_FOUND"
	case http.StatusConflict:
		code = "CONFLICT"
	case http.StatusServiceUnavailable:
		code = "DEPENDENCY_UNAVAILABLE"
	}
	requestID := r.Header.Get("X-Request-ID")
	if _, err := uuid.Parse(requestID); err != nil || len(requestID) != 36 {
		requestID = uuid.NewString()
	}
	w.Header().Set("X-Request-ID", requestID)
	w.Header().Set("Cache-Control", "private, no-store")
	sharedhttp.WriteErrorEnvelope(w, status, code, http.StatusText(status), status == http.StatusServiceUnavailable, 0, nil, nil)
}

func decodeV2Voice(w http.ResponseWriter, r *http.Request, schema string, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
	d.DisallowUnknownFields()
	var raw json.RawMessage
	if d.Decode(&raw) != nil || d.Decode(&struct{}{}) != io.EOF || v2.ValidateJSON(schema, raw) != nil {
		writeV2VoiceError(w, r, safety.ErrInvalid)
		return false
	}
	if json.Unmarshal(raw, v) != nil {
		writeV2VoiceError(w, r, safety.ErrInvalid)
		return false
	}
	return true
}
