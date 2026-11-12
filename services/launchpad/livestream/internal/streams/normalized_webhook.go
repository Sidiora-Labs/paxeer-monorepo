package streams

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strconv"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

const (
	NormalizedWebhookTimestampHeader = "X-Kindle-Webhook-Timestamp"
	NormalizedWebhookSignatureHeader = "X-Kindle-Webhook-Signature"
	normalizedWebhookDomain          = "livestream/IF_LIVE_WEBHOOK/v2"
)

// NormalizedProviderWebhook is emitted only after authenticating the exact
// Studio callback. Its signature binds the delivery timestamp and frozen body;
// occurredAt retains the provider's event time, including on delayed delivery.
type NormalizedProviderWebhook struct {
	Body      []byte
	Timestamp string
	Signature string
}

// DecodeStudioWebhook implements the Studio callback pinned by livestream/app
// at source revision 72187ec428cdd41c81ff75556d77a609b2990695.
func DecodeStudioWebhook(raw []byte) (ProviderEvent, error) {
	if _, err := sharedauth.BodyDigest(raw); err != nil {
		return ProviderEvent{}, ErrInvalid
	}
	var event struct {
		ID        string `json:"id"`
		WebhookID string `json:"webhookId"`
		CreatedAt int64  `json:"createdAt"`
		Timestamp int64  `json:"timestamp"`
		Event     string `json:"event"`
		Stream    struct {
			ID string `json:"id"`
		} `json:"stream"`
	}
	if json.Unmarshal(raw, &event) != nil || !identityPattern.MatchString(event.ID) || event.WebhookID == "" || !identityPattern.MatchString(event.Stream.ID) || event.CreatedAt < 1 || event.Timestamp < event.CreatedAt {
		return ProviderEvent{}, ErrInvalid
	}
	state := ""
	switch event.Event {
	case "stream.started":
		state = stateLive
	case "stream.idle":
		state = stateEnded
	default:
		return ProviderEvent{}, ErrInvalid
	}
	return ProviderEvent{EventID: event.ID, ProviderID: event.Stream.ID, State: state, ObservedAt: time.UnixMilli(event.CreatedAt).UTC()}, nil
}

func normalizedWebhookKey(secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(normalizedWebhookDomain))
	return string(mac.Sum(nil))
}

func NormalizeStudioWebhook(w http.ResponseWriter, r *http.Request, secret string, now time.Time) (NormalizedProviderWebhook, bool) {
	raw, ok := VerifyProviderWebhook(w, r, secret, now, true)
	if !ok {
		return NormalizedProviderWebhook{}, false
	}
	event, err := DecodeStudioWebhook(raw)
	if err != nil {
		writeRequestError(w, http.StatusBadRequest, ruleJSON, "Invalid Studio callback")
		return NormalizedProviderWebhook{}, false
	}
	body, err := marshalNormalizedWebhook(event)
	if err != nil {
		writeRequestError(w, http.StatusBadRequest, ruleJSON, "Unrepresentable Studio callback")
		return NormalizedProviderWebhook{}, false
	}
	timestamp := strconv.FormatInt(now.Unix(), 10)
	return NormalizedProviderWebhook{Body: body, Timestamp: timestamp, Signature: sharedauth.SignWebhook(normalizedWebhookKey(secret), timestamp, string(body))}, true
}

func marshalNormalizedWebhook(event ProviderEvent) ([]byte, error) {
	state := ""
	switch event.State {
	case stateLive:
		state = "active"
	case stateEnded:
		state = "inactive"
	default:
		return nil, ErrInvalid
	}
	if !identityPattern.MatchString(event.EventID) || !identityPattern.MatchString(event.ProviderID) || event.ObservedAt.IsZero() || !event.ObservedAt.Equal(event.ObservedAt.Truncate(time.Millisecond)) {
		return nil, ErrInvalid
	}
	raw, err := json.Marshal(v2.LiveWebhookBody{EventId: event.EventID, StreamId: event.ProviderID, Event: state, OccurredAt: event.ObservedAt.UTC().Format(v2LifecycleTimeLayout)})
	if err != nil {
		return nil, err
	}
	if err = v2.ValidateJSON("LiveWebhookBody", raw); err != nil {
		return nil, err
	}
	return raw, nil
}

type ProviderEventObserver interface {
	Observe(context.Context, ProviderEvent) (string, error)
}

// ProviderWebhookAdapter is the production Studio ingress and frozen consumer.
// The observer owns the transactional inbox; the adapter owns authentication,
// decoding and the declared response schema.
type ProviderWebhookAdapter struct {
	Observer       ProviderEventObserver
	Secret         string
	Clock          func() time.Time
	ObserveOutcome func(string)
}

func (a ProviderWebhookAdapter) Studio(w http.ResponseWriter, r *http.Request) {
	if a.Observer == nil || a.Clock == nil {
		new(Handlers).lifecycleError(w, ErrUnavailable)
		return
	}
	wire, ok := NormalizeStudioWebhook(w, r, a.Secret, a.Clock())
	if !ok {
		return
	}
	normalized := r.Clone(r.Context())
	normalized.Header = r.Header.Clone()
	normalized.Header.Set("Content-Type", jsonMediaType)
	normalized.Header.Set(NormalizedWebhookTimestampHeader, wire.Timestamp)
	normalized.Header.Set(NormalizedWebhookSignatureHeader, wire.Signature)
	normalized.Body = io.NopCloser(bytes.NewReader(wire.Body))
	normalized.ContentLength = int64(len(wire.Body))
	a.Normalized(w, normalized)
}

func (a ProviderWebhookAdapter) Normalized(w http.ResponseWriter, r *http.Request) {
	fail := func(err error) { new(Handlers).lifecycleError(w, err) }
	if a.Observer == nil || a.Clock == nil || len(a.Secret) < 32 {
		fail(ErrUnavailable)
		return
	}
	now := a.Clock()
	event, ok := VerifyNormalizedProviderWebhook(w, r, a.Secret, now)
	if !ok {
		return
	}
	outcome, err := a.Observer.Observe(r.Context(), event)
	if err != nil {
		fail(err)
		return
	}
	if a.ObserveOutcome != nil {
		a.ObserveOutcome(outcome)
	}
	requestID, err := randomID()
	if err != nil {
		fail(err)
		return
	}
	response, err := AdaptV2LiveWebhook(requestID, now)
	if err != nil {
		fail(err)
		return
	}
	writeJSON(w, http.StatusOK, json.RawMessage(response))
}

func (h *Handlers) providerWebhookAdapter() ProviderWebhookAdapter {
	var observer ProviderEventObserver
	if h.lifecycle != nil {
		observer = h.lifecycle
	}
	return ProviderWebhookAdapter{Observer: observer, Secret: h.webhookSecret, Clock: h.clock, ObserveOutcome: h.observeOutcome}
}

func (h *Handlers) normalizedWebhook(w http.ResponseWriter, r *http.Request) {
	h.providerWebhookAdapter().Normalized(w, r)
}

// VerifyNormalizedProviderWebhook authenticates the transport before interpreting
// the frozen body. The durable handler and in-process consumers share this codec.
func VerifyNormalizedProviderWebhook(w http.ResponseWriter, r *http.Request, secret string, now time.Time) (ProviderEvent, bool) {
	if len(secret) < 32 {
		writeContractError(w, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Webhook signing unavailable", true, nil, nil)
		return ProviderEvent{}, false
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 16385))
	if err != nil {
		writeRequestError(w, http.StatusBadRequest, ruleBodyUnreadable, "Invalid webhook body")
		return ProviderEvent{}, false
	}
	if len(raw) > 16384 {
		writeRequestError(w, http.StatusRequestEntityTooLarge, ruleBodyTooLarge, "Webhook too large")
		return ProviderEvent{}, false
	}
	timestamp := r.Header.Get(NormalizedWebhookTimestampHeader)
	signature := r.Header.Get(NormalizedWebhookSignatureHeader)
	seconds, parseErr := strconv.ParseInt(timestamp, 10, 64)
	if parseErr != nil || seconds < 1 || strconv.FormatInt(seconds, 10) != timestamp || len(r.Header.Values(NormalizedWebhookTimestampHeader)) != 1 || len(r.Header.Values(NormalizedWebhookSignatureHeader)) != 1 || len(signature) != 71 || time.Unix(seconds, 0).Before(now.Add(-5*time.Minute)) || time.Unix(seconds, 0).After(now.Add(5*time.Minute)) || sharedauth.VerifyWebhook(normalizedWebhookKey(secret), timestamp, string(raw), signature, now, 5*time.Minute) != nil {
		writeSignatureError(w, "Invalid normalized provider signature")
		return ProviderEvent{}, false
	}
	mediaType, _, mediaErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaErr != nil || mediaType != jsonMediaType || r.Header.Get("Content-Encoding") != "" {
		writeRequestError(w, http.StatusUnsupportedMediaType, ruleMediaType, "JSON request required")
		return ProviderEvent{}, false
	}
	if _, err = sharedauth.BodyDigest(raw); err != nil || v2.ValidateJSON("LiveWebhookBody", raw) != nil {
		writeRequestError(w, http.StatusBadRequest, ruleJSON, "Invalid normalized webhook body")
		return ProviderEvent{}, false
	}
	var body v2.LiveWebhookBody
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil || decoder.Decode(new(any)) != io.EOF {
		writeRequestError(w, http.StatusBadRequest, ruleJSON, "Invalid normalized webhook body")
		return ProviderEvent{}, false
	}
	observedAt, err := time.Parse(v2LifecycleTimeLayout, body.OccurredAt)
	if err != nil {
		writeRequestError(w, http.StatusBadRequest, ruleJSON, "Invalid normalized webhook body")
		return ProviderEvent{}, false
	}
	state := stateLive
	if body.Event == "inactive" {
		state = stateEnded
	}
	return ProviderEvent{EventID: body.EventId, ProviderID: body.StreamId, State: state, ObservedAt: observedAt}, true
}

func AdaptV2LiveWebhook(requestID string, at time.Time) ([]byte, error) {
	if at.IsZero() {
		return nil, ErrInvalid
	}
	return marshalV2Lifecycle("LiveWebhookResponse", v2.LiveWebhookResponse{RequestId: requestID, Data: v2.LiveWebhookData{Accepted: true}, Quality: qualityComplete, GeneratedAt: at.UTC().Format(v2LifecycleTimeLayout)})
}
