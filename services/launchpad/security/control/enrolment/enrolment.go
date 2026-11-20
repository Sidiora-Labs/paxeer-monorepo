// Package enrolment provisions operator factors and short-lived control sessions.
package enrolment

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // RFC 6238 specifies HMAC-SHA-1 for this authenticator.
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalid     = errors.New("invalid operator input")
	ErrDenied      = errors.New("operator authentication denied")
	ErrEnrolled    = errors.New("operator already enrolled")
	ErrUnavailable = errors.New("operator authority unavailable")
	uuidPattern    = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	scopePattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,159}$`)
)

const (
	factorBytes     = 20
	stepSeconds     = 30
	sessionLifetime = 5 * time.Minute
)

type Store struct {
	pool *pgxpool.Pool
	key  [32]byte
}

func New(pool *pgxpool.Pool, key []byte) (*Store, error) {
	if pool == nil || len(key) != 32 {
		return nil, ErrInvalid
	}
	s := &Store{pool: pool}
	copy(s.key[:], key)
	return s, nil
}

// Code returns the six-digit RFC 6238 authenticator for a thirty-second step.
func Code(secret []byte, at time.Time) (string, error) {
	seconds := at.Unix()
	if len(secret) != factorBytes || seconds < 0 {
		return "", ErrInvalid
	}
	return codeForStep(secret, uint64(seconds/stepSeconds)), nil
}

func codeForStep(secret []byte, step uint64) string {
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], step)
	mac := hmac.New(sha1.New, secret)
	_, _ = mac.Write(counter[:])
	digest := mac.Sum(nil)
	offset := digest[len(digest)-1] & 15
	value := binary.BigEndian.Uint32(digest[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", value%1000000)
}

func factorCipher(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, ErrInvalid
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrUnavailable
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrUnavailable
	}
	return aead, nil
}

func seal(secret, key []byte, actor string) ([]byte, error) {
	aead, err := factorCipher(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, ErrUnavailable
	}
	return aead.Seal(nonce, nonce, secret, []byte(actor)), nil
}

func unseal(sealed, key []byte, actor string) ([]byte, error) {
	aead, err := factorCipher(key)
	if err != nil {
		return nil, err
	}
	if len(sealed) != aead.NonceSize()+factorBytes+aead.Overhead() {
		return nil, ErrDenied
	}
	secret, err := aead.Open(nil, sealed[:aead.NonceSize()], sealed[aead.NonceSize():], []byte(actor))
	if err != nil {
		return nil, ErrDenied
	}
	return secret, nil
}

// Enrol returns a new factor once; only a revoked enrolment can be replaced.
func (s *Store) Enrol(ctx context.Context, actor, wallet, role string, scopes []string) ([]byte, error) {
	if !uuidPattern.MatchString(actor) || !utf8.ValidString(wallet) || wallet == "" || strings.TrimSpace(wallet) != wallet || strings.ContainsAny(wallet, "\x00\r\n") || len(scopes) < 1 || len(scopes) > 32 {
		return nil, ErrInvalid
	}
	switch role {
	case "infrastructure", "finance", "moderator", "support":
	default:
		return nil, ErrInvalid
	}
	for _, scope := range scopes {
		if !scopePattern.MatchString(scope) {
			return nil, ErrInvalid
		}
	}
	actor = strings.ToLower(actor)
	secret := make([]byte, factorBytes)
	if _, err := rand.Read(secret); err != nil {
		return nil, ErrUnavailable
	}
	sealed, err := seal(secret, s.key[:], actor)
	if err != nil {
		clear(secret)
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	result, err := s.pool.Exec(ctx, `INSERT INTO security.operator_enrolments(actor,wallet,role,scopes,factor)
 VALUES($1,$2,$3,$4,$5) ON CONFLICT(actor) DO UPDATE SET wallet=$2,role=$3,scopes=$4,factor=$5,last_step=0,enrolled_at=clock_timestamp(),revoked_at=NULL
 WHERE security.operator_enrolments.revoked_at IS NOT NULL`, actor, wallet, role, scopes, sealed)
	if err != nil {
		clear(secret)
		return nil, ErrUnavailable
	}
	if result.RowsAffected() != 1 {
		clear(secret)
		return nil, ErrEnrolled
	}
	return secret, nil
}

func (s *Store) transact(ctx context.Context, fn func(context.Context, pgx.Tx) error) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ErrUnavailable
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), time.Second)
		defer done()
		_ = tx.Rollback(cleanup)
	}()
	if _, err := tx.Exec(ctx, "SET LOCAL lock_timeout='2s'; SET LOCAL statement_timeout='4s'"); err != nil {
		return ErrUnavailable
	}
	if err := fn(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return ErrUnavailable
	}
	return nil
}

func newID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", ErrUnavailable
	}
	raw[6] = raw[6]&15 | 64
	raw[8] = raw[8]&63 | 128
	encoded := hex.EncodeToString(raw[:])
	return encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:], nil
}

// Open consumes one database-clock step and atomically creates its authority.
func (s *Store) Open(ctx context.Context, actor, code string) (credential, sessionID string, err error) {
	if !uuidPattern.MatchString(actor) || len(code) != 6 {
		return "", "", ErrDenied
	}
	for _, digit := range code {
		if digit < '0' || digit > '9' {
			return "", "", ErrDenied
		}
	}
	actor = strings.ToLower(actor)
	err = s.transact(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var wallet, role string
		var scopes []string
		var sealed []byte
		var lastStep int64
		var revokedAt *time.Time
		err := tx.QueryRow(ctx, `SELECT wallet,role,scopes,factor,last_step,revoked_at FROM security.operator_enrolments WHERE actor=$1 FOR UPDATE`, actor).Scan(&wallet, &role, &scopes, &sealed, &lastStep, &revokedAt)
		if errors.Is(err, pgx.ErrNoRows) || revokedAt != nil {
			return ErrDenied
		}
		if err != nil {
			return ErrUnavailable
		}
		secret, err := unseal(sealed, s.key[:], actor)
		if err != nil {
			return err
		}
		defer clear(secret)
		var now time.Time
		if err := tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
			return ErrUnavailable
		}
		seconds := now.Unix()
		if seconds < stepSeconds {
			return ErrUnavailable
		}
		step := seconds / stepSeconds
		current := codeForStep(secret, uint64(step))
		previous := codeForStep(secret, uint64(step-1))
		currentMatch := subtle.ConstantTimeCompare([]byte(code), []byte(current))
		previousMatch := subtle.ConstantTimeCompare([]byte(code), []byte(previous))
		if currentMatch != 1 {
			if previousMatch != 1 {
				return ErrDenied
			}
			step--
		}
		if step <= lastStep {
			return ErrDenied
		}
		if _, err := tx.Exec(ctx, "UPDATE security.operator_enrolments SET last_step=$2 WHERE actor=$1", actor, step); err != nil {
			return ErrUnavailable
		}
		sessionID, err = newID()
		if err != nil {
			return err
		}
		credentialID, err := newID()
		if err != nil {
			return err
		}
		var token [32]byte
		if _, err := rand.Read(token[:]); err != nil {
			return ErrUnavailable
		}
		credential = hex.EncodeToString(token[:])
		hash := sha256.Sum256([]byte(credential))
		expires := now.Add(sessionLifetime)
		if _, err := tx.Exec(ctx, `INSERT INTO security.operator_sessions(session_id,wallet,actor,audience,issued_at,expires_at,mfa_expires_at,scopes,revoked)
 VALUES($1,$2,$3,'control',$4,$5,$5,$6,false)`, sessionID, wallet, actor, now, expires, scopes); err != nil {
			return ErrUnavailable
		}
		if _, err := tx.Exec(ctx, `INSERT INTO pnl.authority_credentials(credential_id,actor_id,role,scopes,token_hash,session_id,expires_at)
 VALUES($1,$2,$3,$4,$5,$6,$7)`, credentialID, actor, role, scopes, hash[:], sessionID, expires); err != nil {
			return ErrUnavailable
		}
		return nil
	})
	if err != nil {
		return "", "", err
	}
	return credential, sessionID, nil
}

func (s *Store) Close(ctx context.Context, sessionID string) error {
	if !uuidPattern.MatchString(sessionID) {
		return ErrInvalid
	}
	sessionID = strings.ToLower(sessionID)
	return s.transact(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "UPDATE security.operator_sessions SET revoked=true WHERE session_id=$1 AND NOT revoked", sessionID); err != nil {
			return ErrUnavailable
		}
		if _, err := tx.Exec(ctx, "UPDATE pnl.authority_credentials SET revoked_at=clock_timestamp() WHERE session_id=$1 AND revoked_at IS NULL", sessionID); err != nil {
			return ErrUnavailable
		}
		return nil
	})
}

func (s *Store) Revoke(ctx context.Context, actor string) error {
	if !uuidPattern.MatchString(actor) {
		return ErrInvalid
	}
	actor = strings.ToLower(actor)
	return s.transact(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "UPDATE security.operator_enrolments SET revoked_at=clock_timestamp() WHERE actor=$1 AND revoked_at IS NULL", actor); err != nil {
			return ErrUnavailable
		}
		if _, err := tx.Exec(ctx, "UPDATE security.operator_sessions SET revoked=true WHERE actor=$1 AND NOT revoked", actor); err != nil {
			return ErrUnavailable
		}
		if _, err := tx.Exec(ctx, "UPDATE pnl.authority_credentials SET revoked_at=clock_timestamp() WHERE actor_id=$1 AND revoked_at IS NULL", actor); err != nil {
			return ErrUnavailable
		}
		return nil
	})
}
