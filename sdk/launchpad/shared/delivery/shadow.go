package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"

	"github.com/jackc/pgx/v5"
)

type ShadowRecord struct {
	EventID     string `json:"eventId"`
	PayloadHash string `json:"payloadHash"`
}
type ShadowDifference struct {
	Missing, Extra, Changed, Duplicates []string
	Digest                              string
}

func CompareShadow(expected, observed []ShadowRecord) (ShadowDifference, error) {
	var d ShadowDifference
	left := map[string]string{}
	right := map[string]string{}
	for index, records := range [][]ShadowRecord{expected, observed} {
		dest := left
		if index == 1 {
			dest = right
		}
		for _, record := range records {
			if !token.MatchString(record.EventID) || !archiveHash.MatchString(record.PayloadHash) {
				return d, ErrSchema
			}
			if _, exists := dest[record.EventID]; exists {
				d.Duplicates = append(d.Duplicates, record.EventID)
			}
			dest[record.EventID] = record.PayloadHash
		}
	}
	for id, hash := range left {
		actual, ok := right[id]
		if !ok {
			d.Missing = append(d.Missing, id)
		} else if actual != hash {
			d.Changed = append(d.Changed, id)
		}
	}
	for id := range right {
		if _, ok := left[id]; !ok {
			d.Extra = append(d.Extra, id)
		}
	}
	sort.Strings(d.Missing)
	sort.Strings(d.Extra)
	sort.Strings(d.Changed)
	sort.Strings(d.Duplicates)
	sorted := append([]ShadowRecord(nil), expected...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].EventID < sorted[j].EventID })
	raw, err := json.Marshal(sorted)
	if err != nil {
		return d, err
	}
	d.Digest = Hash(raw)
	if len(expected) == 0 || len(d.Missing)+len(d.Extra)+len(d.Changed)+len(d.Duplicates) > 0 {
		return d, ErrIntegrity
	}
	return d, nil
}
func (s *Store) RecordShadowTx(ctx context.Context, tx pgx.Tx, rangeDigest, path, subject string, e Envelope) error {
	if tx == nil || !archiveHash.MatchString(rangeDigest) || (path != "legacy" && path != "durable") {
		return ErrSchema
	}
	if err := e.Validate(s.policy, subject); err != nil {
		return err
	}
	if err := s.assertEpochTx(ctx, tx); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, "INSERT INTO "+s.table("delivery_shadow_observations")+"(range_digest,path,event_id,payload_hash,source_order,subject) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING", rangeDigest, path, e.EventID, e.PayloadHash, e.SourceOrder, subject)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var hash, oldSubject string
		var oldRaw []byte
		err = tx.QueryRow(ctx, "SELECT payload_hash,source_order,subject FROM "+s.table("delivery_shadow_observations")+" WHERE range_digest=$1 AND path=$2 AND event_id=$3", rangeDigest, path, e.EventID).Scan(&hash, &oldRaw, &oldSubject)
		if err != nil {
			return err
		}
		var oldOrder SourceOrder
		if json.Unmarshal(oldRaw, &oldOrder) != nil || hash != e.PayloadHash || oldOrder != e.SourceOrder || oldSubject != subject {
			return ErrIntegrity
		}
	}
	return s.assertEpochTx(ctx, tx)
}
func (s *Store) CompareShadowRange(ctx context.Context, manifestDigest string, subjects []string) (ShadowDifference, error) {
	return s.compareShadowRange(ctx, manifestDigest, subjects, "", "")
}

// CompareShadowGeneration binds evidence to the consumer cutover that may use it.
func (s *Store) CompareShadowGeneration(ctx context.Context, manifestDigest string, subjects []string, consumer, generation string) (ShadowDifference, error) {
	if !token.MatchString(consumer) || !token.MatchString(generation) {
		return ShadowDifference{}, ErrSchema
	}
	return s.compareShadowRange(ctx, manifestDigest, subjects, consumer, generation)
}

func (s *Store) shadowSubjects(subjects []string) ([]string, error) {
	if len(subjects) == 0 {
		return nil, ErrSchema
	}
	set := map[string]bool{}
	for _, subject := range subjects {
		if !s.policy.Subjects[subject] {
			return nil, ErrSchema
		}
		set[subject] = true
	}
	canonical := make([]string, 0, len(set))
	for subject := range set {
		canonical = append(canonical, subject)
	}
	sort.Strings(canonical)
	return canonical, nil
}

func (s *Store) compareShadowRange(ctx context.Context, manifestDigest string, subjects []string, consumer, generation string) (ShadowDifference, error) {
	var empty ShadowDifference
	if !archiveHash.MatchString(manifestDigest) {
		return empty, ErrSchema
	}
	subjects, err := s.shadowSubjects(subjects)
	if err != nil {
		return empty, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return empty, err
	}
	defer rollbackBounded(tx)
	diff, count, err := s.compareShadowTx(ctx, tx, manifestDigest, subjects)
	if err != nil {
		return diff, err
	}
	if err = s.assertEpochTx(ctx, tx); err != nil {
		return diff, err
	}
	_, err = tx.Exec(ctx, "INSERT INTO "+s.table("delivery_shadow_comparisons")+"(manifest_digest,event_digest,event_count,subjects,consumer_id,generation) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(manifest_digest,consumer_id,generation,subjects) DO UPDATE SET event_digest=EXCLUDED.event_digest,event_count=EXCLUDED.event_count,compared_at=clock_timestamp()", manifestDigest, diff.Digest, count, subjects, consumer, generation)
	if err != nil {
		return diff, err
	}
	return diff, tx.Commit(ctx)
}
func (s *Store) compareShadowTx(ctx context.Context, tx pgx.Tx, manifestDigest string, subjects []string) (ShadowDifference, int, error) {
	var empty ShadowDifference
	var raw []byte
	if err := tx.QueryRow(ctx, "SELECT manifest FROM "+s.table("delivery_archive_manifests")+" WHERE manifest_digest=$1", manifestDigest).Scan(&raw); err != nil {
		return empty, 0, err
	}
	var m ArchiveManifest
	if json.Unmarshal(raw, &m) != nil || ValidateArchiveManifest(m) != nil {
		return empty, 0, ErrIntegrity
	}
	sets := map[string][]ShadowRecord{"legacy": {}, "durable": {}}
	rows, err := tx.Query(ctx, "SELECT path,event_id,payload_hash,source_order FROM "+s.table("delivery_shadow_observations")+" WHERE range_digest=$1 AND subject=ANY($2) ORDER BY path,event_id", manifestDigest, subjects)
	if err != nil {
		return empty, 0, err
	}
	for rows.Next() {
		var path string
		var record ShadowRecord
		var positionRaw []byte
		if err = rows.Scan(&path, &record.EventID, &record.PayloadHash, &positionRaw); err != nil {
			rows.Close()
			return empty, 0, err
		}
		var order SourceOrder
		if json.Unmarshal(positionRaw, &order) != nil || order.Compare(SourceOrder{Height: strconv.FormatInt(m.FromHeight, 10), TransactionIndex: "0", LogIndex: "0"}) < 0 || order.Compare(SourceOrder{Height: strconv.FormatInt(m.ToHeight+1, 10), TransactionIndex: "0", LogIndex: "0"}) >= 0 {
			rows.Close()
			return empty, 0, ErrIntegrity
		}
		sets[path] = append(sets[path], record)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return empty, 0, err
	}
	diff, err := CompareShadow(sets["legacy"], sets["durable"])
	if err != nil {
		return diff, 0, err
	}
	var expected []ShadowRecord
	rows, err = tx.Query(ctx, "SELECT event_id,payload_hash FROM "+s.table("delivery_outbox")+" WHERE (convert_from(envelope,'UTF8')::jsonb->'source_order'->>'height')::bigint BETWEEN $1 AND $2 AND subject=ANY($3) ORDER BY event_id", m.FromHeight, m.ToHeight, subjects)
	if err != nil {
		return diff, 0, err
	}
	for rows.Next() {
		var record ShadowRecord
		if err = rows.Scan(&record.EventID, &record.PayloadHash); err != nil {
			rows.Close()
			return diff, 0, err
		}
		expected = append(expected, record)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return diff, 0, err
	}
	if _, err = CompareShadow(expected, sets["durable"]); err != nil {
		return diff, 0, err
	}
	return diff, len(expected), nil
}

func (s *Store) BindGenerationPathTx(ctx context.Context, tx pgx.Tx, consumer, generation, path string) error {
	if tx == nil || !token.MatchString(consumer) || !token.MatchString(generation) || (path != "legacy" && path != "durable") {
		return ErrSchema
	}
	if err := s.assertEpochTx(ctx, tx); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, "INSERT INTO "+s.table("delivery_generation_paths")+"(consumer_id,generation,authoritative_path) VALUES($1,$2,$3) ON CONFLICT DO NOTHING", consumer, generation, path)
	if err != nil {
		return err
	}
	var actual string
	if err = tx.QueryRow(ctx, "SELECT authoritative_path FROM "+s.table("delivery_generation_paths")+" WHERE consumer_id=$1 AND generation=$2", consumer, generation).Scan(&actual); err != nil {
		return err
	}
	if actual != path {
		return errors.New("delivery: generation already bound to another authoritative path")
	}
	return s.assertEpochTx(ctx, tx)
}
