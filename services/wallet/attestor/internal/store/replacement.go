package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"sort"

	"go.etcd.io/bbolt"
)

const RecordReplacement RecordKind = "share-replacement"

// ShareReplacement retains the authenticated shares on both sides of a roster
// change. Import receipts describe the original material, not its replacement.
type ShareReplacement struct {
	Previous         ShareRecord `json:"previous"`
	Next             ShareRecord `json:"next"`
	Quorum           []string    `json:"quorum"`
	NewParticipantID string      `json:"new_participant_id"`
}

func (s *Store) ReplaceShare(rec ShareRecord, share []byte, quorum []string, newcomer string) error {
	meta := normalize(rec)
	if len(share) == 0 {
		return ErrInvalid
	}
	roster := append(append([]string(nil), quorum...), newcomer)
	sort.Strings(roster)
	if newcomer == "" || len(quorum) < 3 || !equalStrings(meta.Participants, roster) {
		return ErrInvalid
	}
	for i, member := range roster {
		if member == "" || (i > 0 && roster[i-1] == member) {
			return ErrInvalid
		}
	}
	if err := validate(meta); err != nil {
		return err
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		if tx.Bucket(bucketStaged).Get([]byte(meta.KeyID)) != nil {
			return fmt.Errorf("%w: replacement during staged refresh", ErrInvalid)
		}
		var history []ShareReplacement
		records := tx.Bucket(bucketRecords)
		key := recordKey(RecordReplacement, meta.KeyID)
		if raw := records.Get(key); raw != nil {
			plain, err := s.openRecord(RecordReplacement, meta.KeyID, raw)
			if err != nil {
				return err
			}
			err = json.Unmarshal(plain, &history)
			zero(plain)
			if err != nil {
				return err
			}
		}
		raw := tx.Bucket(bucketShares).Get([]byte(meta.KeyID))
		if raw == nil {
			if len(history) != 0 {
				return ErrInvalid
			}
			return s.putShare(tx, meta, share)
		}
		previous, err := decodeRecord(meta.KeyID, raw)
		if err != nil {
			return err
		}
		if meta.Epoch != previous.Epoch {
			return ErrStageEpoch
		}
		if _, err := s.replacementOrigin(meta.KeyID, previous, history); err != nil {
			return err
		}
		if err := s.putShare(tx, meta, share); err != nil {
			return err
		}
		next, err := decodeRecord(meta.KeyID, tx.Bucket(bucketShares).Get([]byte(meta.KeyID)))
		if err != nil {
			return err
		}
		history = append(history, ShareReplacement{Previous: previous, Next: next, Quorum: append([]string(nil), quorum...), NewParticipantID: newcomer})
		if _, err := s.replacementOrigin(meta.KeyID, next, history); err != nil {
			return err
		}
		plain, err := json.Marshal(history)
		if err != nil {
			return err
		}
		defer zero(plain)
		protected, err := s.sealRecord(RecordReplacement, meta.KeyID, plain)
		if err != nil {
			return err
		}
		return records.Put(key, protected)
	})
}

// The chain permits a refreshed successor only with the same public identity and
// roster. At the same epoch the exact authenticated share must still match.
func replacementSuccessor(previous, next ShareRecord) bool {
	return previous.KeyID == next.KeyID && previous.Curve == next.Curve && bytes.Equal(previous.PublicKey, next.PublicKey) &&
		equalStrings(previous.Participants, next.Participants) && previous.Epoch <= next.Epoch &&
		(previous.Epoch != next.Epoch || bytes.Equal(previous.Ciphertext, next.Ciphertext))
}

func (s *Store) replacementOrigin(key string, held ShareRecord, history []ShareReplacement) (ShareRecord, error) {
	if len(history) == 0 {
		return held, nil
	}
	invalid := func() (ShareRecord, error) {
		return ShareRecord{}, fmt.Errorf("%w: invalid share replacement lineage", ErrSnapshotFormat)
	}
	for i, change := range history {
		for _, rec := range []ShareRecord{change.Previous, change.Next} {
			if rec.KeyID != key || validate(rec) != nil {
				return invalid()
			}
			plain, err := open(s.nodeKey, rec)
			if err != nil {
				return ShareRecord{}, err
			}
			n := len(plain)
			zero(plain)
			if n == 0 {
				return invalid()
			}
		}
		before, after := change.Previous, change.Next
		quorum := append([]string(nil), change.Quorum...)
		sort.Strings(quorum)
		if len(quorum) < 3 || change.NewParticipantID == "" || slices.Contains(before.Participants, change.NewParticipantID) ||
			before.Epoch != after.Epoch || before.Curve != after.Curve || !bytes.Equal(before.PublicKey, after.PublicKey) {
			return invalid()
		}
		for j, member := range quorum {
			if !slices.Contains(before.Participants, member) || (j > 0 && quorum[j-1] == member) {
				return invalid()
			}
		}
		roster := append(quorum, change.NewParticipantID)
		sort.Strings(roster)
		if !equalStrings(roster, after.Participants) || (i > 0 && !replacementSuccessor(history[i-1].Next, before)) {
			return invalid()
		}
	}
	if !replacementSuccessor(history[len(history)-1].Next, held) {
		return invalid()
	}
	return history[0].Previous, nil
}
