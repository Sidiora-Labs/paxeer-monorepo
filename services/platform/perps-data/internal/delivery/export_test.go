package delivery

import (
	"context"
	"time"
)

// RelayClaim is the claim of Relay, for tests that stop a replica between
// its claim and its record.
func (s *Store) RelayClaim(ctx context.Context, owner string, limit int, duration time.Duration) ([]Lease, error) {
	return s.claim(ctx, owner, limit, duration, true)
}

// ClaimStatement exposes the production statement for database plan checks.
func (s *Store) ClaimStatement(relay bool) string { return s.claimSQL(relay) }

// HeadAgeStatement exposes the production statement for database plan checks.
func (s *Store) HeadAgeStatement() string { return s.headAgeSQL() }

// PrunePublishedStatement exposes the production statement for database plan checks.
func (s *Store) PrunePublishedStatement() string { return s.prunePublishedSQL() }
