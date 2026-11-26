package discovery

import (
	"context"
	"encoding/json"
	"time"
)

func (s SourceRef) Equal(other SourceRef) bool {
	return sameSource(s, other)
}

func (s Store) RenewSources(ctx context.Context, documents []json.RawMessage, observed time.Time) error {
	if s.DB == nil {
		return ErrUnavailable
	}
	if observed.IsZero() || observed.After(time.Now()) {
		return ErrInvalid
	}
	for offset := 0; offset < len(documents); offset += 500 {
		batch, err := json.Marshal(documents[offset:min(offset+500, len(documents))])
		if err != nil {
			return err
		}
		_, err = s.DB.ExecContext(ctx, `UPDATE narrative_discovery.sources AS s SET observed_at=$2 FROM jsonb_array_elements($1::jsonb) AS renewal(document) WHERE s.id=renewal.document->>'ID' AND s.document=renewal.document`, batch, observed)
		if err != nil {
			return err
		}
	}
	return ctx.Err()
}
