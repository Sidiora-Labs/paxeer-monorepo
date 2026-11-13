package l17

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPortfolioCursorsRejectMalformedAuthenticatedFields(t *testing.T) {
	key := []byte(strings.Repeat("k", 32))
	now := time.Unix(10000, 0)
	original := Cursor{Wallet: wallet, Snapshot: uuid.NewString(), Revision: "1", Filter: filterAll, Sort: sortSnapshotOrdinalV1, After: -1, Limit: 50, Expires: now.Add(time.Hour).Unix()}
	for _, change := range []func(*Cursor){func(c *Cursor) { c.Wallet = "invalid" }, func(c *Cursor) { c.Limit = 0 }, func(c *Cursor) { c.After = -2 }, func(c *Cursor) { c.Revision = "0" }, func(c *Cursor) { c.Snapshot = "invalid" }} {
		bad := original
		change(&bad)
		if _, err := SignCursor(key, bad); !errors.Is(err, ErrInvalid) {
			t.Fatal(bad, err)
		}
		if _, err := SignPortfolioCursor(key, bad); !errors.Is(err, ErrInvalid) {
			t.Fatal(bad, err)
		}
	}
	if _, err := SignCursor(nil, original); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	for _, change := range []func(*Cursor){func(c *Cursor) { c.Filter = "other" }, func(c *Cursor) { c.Revision = "" }, func(c *Cursor) { c.After = 100001 }, func(c *Cursor) { c.Expires = 0 }} {
		bad := original
		change(&bad)
		if _, err := SignPortfolioCursor(key, bad); !errors.Is(err, ErrInvalid) {
			t.Fatal(bad, err)
		}
	}
	valid, err := SignCursor(key, original)
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"invalid", "!.YQ", "YQ.!", strings.Repeat("x", 2049)} {
		if _, err := VerifyCursor(key, token, wallet, filterAll, sortSnapshotOrdinalV1, now); !errors.Is(err, ErrInvalid) {
			t.Fatal(token, err)
		}
	}
	signed := func(raw []byte) string {
		m := hmac.New(sha256.New, key)
		m.Write(raw)
		return base64.RawURLEncoding.EncodeToString(raw) + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil))
	}
	for _, raw := range []string{"{", `{}`} {
		if _, err := VerifyCursor(key, signed([]byte(raw)), wallet, filterAll, sortSnapshotOrdinalV1, now); !errors.Is(err, ErrInvalid) {
			t.Fatal(raw, err)
		}
	}
	if got, err := VerifyCursor(key, valid, wallet, filterAll, sortSnapshotOrdinalV1, now); err != nil || got != original {
		t.Fatal(got, err)
	}
	compact, err := SignPortfolioCursor(key, original)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := VerifyPortfolioCursor(key, compact, wallet, now); err != nil || got != original {
		t.Fatal(got, err)
	}
	if _, err := VerifyPortfolioCursor(nil, compact, wallet, now); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	for _, token := range []string{strings.Repeat("x", 257), "!", compact + "="} {
		if _, err := VerifyPortfolioCursor(key, token, wallet, now); !errors.Is(err, ErrInvalid) {
			t.Fatal(token, err)
		}
	}
	raw, err := base64.RawURLEncoding.DecodeString(compact)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name         string
		change       func([]byte)
		authenticate bool
	}{
		{"signature", func(r []byte) { r[100] ^= 1 }, false},
		{"offset", func(r []byte) { binary.BigEndian.PutUint32(r[69:73], 100001) }, true},
		{"expiry", func(r []byte) { binary.BigEndian.PutUint64(r[74:82], 1<<63) }, true},
		{"revision", func(r []byte) { clear(r[37:69]) }, true},
		{"limit", func(r []byte) { r[73] = 0 }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := append([]byte(nil), raw...)
			tc.change(candidate)
			if tc.authenticate {
				m := hmac.New(sha256.New, key)
				m.Write([]byte(Algorithm))
				m.Write(candidate[:82])
				copy(candidate[82:], m.Sum(nil))
			}
			if _, err := VerifyPortfolioCursor(key, base64.RawURLEncoding.EncodeToString(candidate), wallet, now); !errors.Is(err, ErrInvalid) {
				t.Fatal(err)
			}
		})
	}
}

func TestSnapshotCommitValidatesWholeBatchBeforeWriting(t *testing.T) {
	ctx := context.Background()
	r := phase2Repository(t)
	positions := phase2Positions(1, "1")
	summary := Summary{Wallet: wallet, Watermark: positions[0].SourceWatermark}
	for _, tc := range []struct {
		name   string
		change func(*Summary, *[]Position, *string, *time.Time)
	}{
		{"snapshot", func(s *Summary, p *[]Position, source *string, expiry *time.Time) { s.SnapshotID = "invalid" }},
		{"source", func(s *Summary, p *[]Position, source *string, expiry *time.Time) { *source = "" }},
		{"revision", func(s *Summary, p *[]Position, source *string, expiry *time.Time) {
			s.Watermark.ProjectionRevision = "0"
		}},
		{"position", func(s *Summary, p *[]Position, source *string, expiry *time.Time) { (*p)[0].BalanceRaw = "bad" }},
		{"wallet", func(s *Summary, p *[]Position, source *string, expiry *time.Time) { s.Wallet = "invalid" }},
		{"expiry", func(s *Summary, p *[]Position, source *string, expiry *time.Time) { *expiry = time.Time{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := summary
			p := append([]Position(nil), positions...)
			source := "validated-batch"
			expiry := time.Now().Add(time.Hour)
			tc.change(&s, &p, &source, &expiry)
			tx, err := r.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer rollbackPortfolio(tx)
			if _, err = r.CommitSnapshotTx(ctx, tx, source, s, p, expiry); !errors.Is(err, ErrInvalid) {
				t.Fatal(err)
			}
			var count int
			if err = tx.QueryRow(ctx, `SELECT count(*) FROM portfolio_snapshots`).Scan(&count); err != nil || count != 0 {
				t.Fatal(count, err)
			}
		})
	}
	if _, err := r.CommitSnapshotTx(ctx, nil, "source", summary, positions, time.Now()); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := (Repository{}).CommitSnapshot(ctx, "source", summary, positions, time.Now()); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err := (Repository{}).Page(ctx, wallet, uuid.NewString(), -1, 1, time.Now()); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	for _, after := range []int{-2, 100001} {
		if _, err := r.Page(ctx, wallet, uuid.NewString(), after, 1, time.Now()); !errors.Is(err, ErrInvalid) {
			t.Fatal(err)
		}
	}
	if got, err := SafeMetadata(`<a x="b">`); err != nil || got != "&lt;a x=&#34;b&#34;&gt;" {
		t.Fatal(got, err)
	}
	if _, err := SafeMetadata(strings.Repeat("x", 257)); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}
