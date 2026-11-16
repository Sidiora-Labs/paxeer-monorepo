package search

import (
	"context"
)

// Ready reports whether the active search generation has been validated.
func (s Store) Ready(ctx context.Context) error {
	if s.Pool == nil {
		return ErrUnavailable
	}
	var active bool
	if err := s.Pool.QueryRow(ctx, "SELECT g.state='active' AND g.validated_at IS NOT NULL FROM stats_search.active_v2 a JOIN stats_search.generations_v2 g USING(generation) WHERE a.singleton").Scan(&active); err != nil {
		return err
	}
	if !active {
		return ErrUnavailable
	}
	return nil
}
