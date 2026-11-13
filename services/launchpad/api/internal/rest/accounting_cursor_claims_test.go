package rest

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

func TestAccountingCursorRejectsInvalidClaims(t *testing.T) {
	h := AccountingV2Handler{SigningKey: []byte(strings.Repeat("k", 32))}
	r := store.AccountingPageRequest{Pool: "0x" + strings.Repeat("1", 40), Limit: 10, From: 1000, To: 61000, SnapshotObservedAt: "2026-01-01T00:00:00.000Z", AfterOrder: &accounting.SourcePosition{Height: "1", TransactionIndex: "0", LogIndex: "0"}, AfterEventID: strings.Repeat("a", 64)}
	const snapshot = "00000000-0000-4000-8000-000000000001"
	for _, tc := range []struct {
		name   string
		change func(*store.AccountingPageRequest, *AccountingV2Handler, *string, *int64, *int64)
		want   error
	}{
		{"short key", func(_ *store.AccountingPageRequest, h *AccountingV2Handler, _ *string, _, _ *int64) {
			h.SigningKey = nil
		}, store.ErrAccountingNotReady},
		{"negative boundary", func(_ *store.AccountingPageRequest, _ *AccountingV2Handler, _ *string, b, _ *int64) { *b = -1 }, store.ErrAccountingNotReady},
		{"zero revision", func(_ *store.AccountingPageRequest, _ *AccountingV2Handler, _ *string, _, u *int64) { *u = 0 }, store.ErrAccountingNotReady},
		{"invalid snapshot", func(_ *store.AccountingPageRequest, _ *AccountingV2Handler, s *string, _, _ *int64) { *s = "invalid" }, store.ErrDataIntegrity},
		{"invalid clock", func(r *store.AccountingPageRequest, _ *AccountingV2Handler, _ *string, _, _ *int64) {
			r.SnapshotObservedAt = "invalid"
		}, store.ErrDataIntegrity},
		{"negative expiry", func(r *store.AccountingPageRequest, _ *AccountingV2Handler, _ *string, _, _ *int64) {
			r.CursorExpires = -1
		}, store.ErrDataIntegrity},
		{"missing source order", func(r *store.AccountingPageRequest, _ *AccountingV2Handler, _ *string, _, _ *int64) {
			r.AfterOrder = nil
		}, store.ErrDataIntegrity},
		{"invalid source identity", func(r *store.AccountingPageRequest, _ *AccountingV2Handler, _ *string, _, _ *int64) {
			r.AfterEventID = "invalid"
		}, store.ErrDataIntegrity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request, handler, id, boundary, upper := r, h, snapshot, int64(1), int64(1)
			tc.change(&request, &handler, &id, &boundary, &upper)
			if token, err := handler.sign(request, false, id, boundary, upper); token != "" || !errors.Is(err, tc.want) {
				t.Fatalf("invalid claims: token=%q err=%v", token, err)
			}
		})
	}
	token, err := h.sign(r, false, snapshot, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func([]byte)
	}{
		{"unrepresentable boundary", func(raw []byte) { binary.BigEndian.PutUint64(raw[35:43], math.MaxUint64) }},
		{"zero revision", func(raw []byte) { binary.BigEndian.PutUint64(raw[51:59], 0) }},
		{"expired", func(raw []byte) { binary.BigEndian.PutUint64(raw[43:51], 1) }},
		{"future expiry", func(raw []byte) { binary.BigEndian.PutUint64(raw[43:51], uint64(time.Now().Add(time.Hour).Unix())) }},
		{"future observation", func(raw []byte) {
			binary.BigEndian.PutUint64(raw[59:67], uint64(time.Now().Add(time.Hour).UnixMilli()))
		}},
		{"zero trade boundary", func(raw []byte) { binary.BigEndian.PutUint64(raw[35:43], 0) }},
		{"trade boundary beyond revision", func(raw []byte) { binary.BigEndian.PutUint64(raw[35:43], 3) }},
		{"unrepresentable height", func(raw []byte) { binary.BigEndian.PutUint64(raw[67:75], math.MaxUint64) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wire, err := base64.RawURLEncoding.DecodeString(token)
			if err != nil {
				t.Fatal(err)
			}
			raw := wire[:115]
			tc.change(raw)
			mac := hmac.New(sha256.New, h.SigningKey)
			mac.Write(raw)
			changed := base64.RawURLEncoding.EncodeToString(append(raw, mac.Sum(nil)...))
			if _, err := h.cursor(r, false, changed); !errors.Is(err, store.ErrCursorInvalid) {
				t.Fatalf("invalid cursor accepted: %v", err)
			}
		})
	}
	for _, invalid := range []AccountingV2Handler{{}, {SigningKey: h.SigningKey, PreviousSigningKeys: make([][]byte, 5)}, {SigningKey: h.SigningKey, PreviousSigningKeys: [][]byte{nil}}} {
		if _, err := invalid.cursor(r, false, token); !errors.Is(err, store.ErrCursorInvalid) {
			t.Fatalf("invalid key ring: %v", err)
		}
	}
	candle, err := h.sign(r, true, snapshot, 99, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.cursor(r, true, candle); !errors.Is(err, store.ErrCursorInvalid) {
		t.Fatalf("candle cursor outside range: %v", err)
	}
}

func TestAccountingEncoderRejectsInvalidContinuationMetadata(t *testing.T) {
	h := AccountingV2Handler{SigningKey: []byte(strings.Repeat("k", 32))}
	boundary := int64(1)
	for _, page := range []store.AccountingPage{
		{NextStart: &boundary, Watermark: json.RawMessage(`{`)},
		{NextStart: &boundary, Watermark: json.RawMessage(`{"observedAt":"2026-01-01T00:00:00.000Z"}`), Revision: "invalid"},
		{NextStart: &boundary, Watermark: json.RawMessage(`{"observedAt":"2026-01-01T00:00:00.000Z"}`), Revision: "1", SnapshotID: "invalid"},
		{Watermark: json.RawMessage(`{`)},
	} {
		if body, err := h.EncodeCachePage(store.AccountingPageRequest{}, true, page, "00000000-0000-4000-8000-000000000001"); body != nil || !errors.Is(err, store.ErrDataIntegrity) {
			t.Fatalf("invalid continuation: body=%s err=%v", body, err)
		}
	}
}
