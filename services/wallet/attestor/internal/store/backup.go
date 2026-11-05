package store

import (
	"bytes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.etcd.io/bbolt"
)

const (
	SnapshotVersion   byte = 3
	backupDomain           = "paxeer-attestor-backup-v3"
	maxSnapshotLength      = 1 << 30
	maxSnapshotNodeID      = 255
)

var (
	ErrSnapshotVersion = errors.New("store: unsupported snapshot version")
	ErrSnapshotAuth    = errors.New("store: snapshot failed authentication")
	ErrSnapshotFormat  = errors.New("store: malformed snapshot")
	ErrSnapshotNode    = errors.New("store: snapshot belongs to another node")
)

func snapshotHeader(nodeID string) ([]byte, error) {
	if nodeID == "" || len(nodeID) > maxSnapshotNodeID {
		return nil, fmt.Errorf("store: snapshot node id must be 1 to %d bytes", maxSnapshotNodeID)
	}
	h := make([]byte, 0, 3+len(nodeID))
	h = append(h, SnapshotVersion)
	h = binary.BigEndian.AppendUint16(h, uint16(len(nodeID)))
	return append(h, nodeID...), nil
}

func SnapshotNodeID(raw []byte) (string, error) {
	if len(raw) < 1 {
		return "", fmt.Errorf("%w: too short", ErrSnapshotFormat)
	}
	if raw[0] != SnapshotVersion {
		return "", fmt.Errorf("%w: %d", ErrSnapshotVersion, raw[0])
	}
	if len(raw) < 3 {
		return "", fmt.Errorf("%w: too short", ErrSnapshotFormat)
	}
	n := int(binary.BigEndian.Uint16(raw[1:3]))
	if n == 0 || n > maxSnapshotNodeID || len(raw) < 3+n+NonceSize {
		return "", fmt.Errorf("%w: bad node id header", ErrSnapshotFormat)
	}
	return string(raw[3 : 3+n]), nil
}

// Version 3 requires an explicit manifest, including empty buckets. A shares-only
// backup cannot safely be upgraded: missing usage and replay history is unknown.
type snapshotBody struct {
	Buckets []snapshotBucket `json:"buckets"`
}

type snapshotBucket struct {
	Name    string          `json:"name"`
	Entries []snapshotEntry `json:"entries"`
}

type snapshotEntry struct {
	Key   []byte `json:"key"`
	Value []byte `json:"value"`
}

func snapshotBuckets() [][]byte {
	return [][]byte{bucketShares, bucketStaged, bucketRecords, bucketReplay}
}

func (s *Store) Snapshot(w io.Writer, backupKey []byte, nodeID string) error {
	if len(backupKey) != KeySize {
		return fmt.Errorf("store: snapshot: backup key must be %d bytes", KeySize)
	}
	header, err := snapshotHeader(nodeID)
	if err != nil {
		return err
	}
	var body snapshotBody
	err = s.db.View(func(tx *bbolt.Tx) error {
		if err := s.validateSnapshotState(tx); err != nil {
			return err
		}
		// Reject new bucket types until their recovery semantics are defined.
		if err := tx.ForEach(func(name []byte, _ *bbolt.Bucket) error {
			for _, known := range snapshotBuckets() {
				if bytes.Equal(name, known) {
					return nil
				}
			}
			return fmt.Errorf("%w: unknown durable bucket %q", ErrSnapshotFormat, name)
		}); err != nil {
			return err
		}
		for _, name := range snapshotBuckets() {
			bucket := snapshotBucket{Name: string(name), Entries: []snapshotEntry{}}
			if b := tx.Bucket(name); b != nil {
				if err := b.ForEach(func(k, v []byte) error {
					if v == nil {
						return fmt.Errorf("%w: nested bucket", ErrSnapshotFormat)
					}
					bucket.Entries = append(bucket.Entries, snapshotEntry{Key: bytes.Clone(k), Value: bytes.Clone(v)})
					return nil
				}); err != nil {
					return err
				}
			}
			body.Buckets = append(body.Buckets, bucket)
		}
		return nil
	})
	if err != nil {
		return err
	}
	plain, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("store: snapshot: encode: %w", err)
	}
	defer zero(plain)
	aead, ad, err := backupAEAD(backupKey, header)
	if err != nil {
		return err
	}
	hl := len(header)
	if len(plain) > maxSnapshotLength-hl-NonceSize-aead.Overhead() {
		return fmt.Errorf("%w: too large", ErrSnapshotFormat)
	}
	out := make([]byte, hl+NonceSize, hl+NonceSize+len(plain)+aead.Overhead())
	copy(out, header)
	if _, err := rand.Read(out[hl:]); err != nil {
		return errors.New("store: snapshot: nonce generation failed")
	}
	out = aead.Seal(out, out[hl:hl+NonceSize], plain, ad)
	if n, err := w.Write(out); err != nil {
		return fmt.Errorf("store: snapshot: write: %w", err)
	} else if n != len(out) {
		return io.ErrShortWrite
	}
	return nil
}

func Restore(r io.Reader, backupKey []byte, nodeID string, dataDir string, nodeKey []byte) (*Store, error) {
	if len(backupKey) != KeySize {
		return nil, fmt.Errorf("store: restore: backup key must be %d bytes", KeySize)
	}
	if len(nodeKey) != KeySize {
		return nil, ErrKeySize
	}
	raw, err := io.ReadAll(io.LimitReader(r, maxSnapshotLength+1))
	if err != nil {
		return nil, fmt.Errorf("store: restore: read: %w", err)
	}
	if len(raw) > maxSnapshotLength {
		return nil, fmt.Errorf("%w: too large", ErrSnapshotFormat)
	}
	owner, err := SnapshotNodeID(raw)
	if err != nil {
		return nil, err
	}
	if owner != nodeID {
		return nil, fmt.Errorf("%w: snapshot names node %q, this node is %q", ErrSnapshotNode, owner, nodeID)
	}
	header, err := snapshotHeader(nodeID)
	if err != nil {
		return nil, err
	}
	hl := len(header)
	aead, ad, err := backupAEAD(backupKey, header)
	if err != nil {
		return nil, err
	}
	plain, err := aead.Open(nil, raw[hl:hl+NonceSize], raw[hl+NonceSize:], ad)
	if err != nil {
		return nil, ErrSnapshotAuth
	}
	defer zero(plain)
	var body snapshotBody
	decoder := json.NewDecoder(bytes.NewReader(plain))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		return nil, fmt.Errorf("%w: undecodable body", ErrSnapshotFormat)
	}

	if decoder.Decode(new(any)) != io.EOF || len(body.Buckets) != len(snapshotBuckets()) {
		return nil, fmt.Errorf("%w: incomplete bucket manifest or trailing data", ErrSnapshotFormat)
	}
	seen := make(map[string]bool)
	for _, bucket := range body.Buckets {
		known := false
		for _, name := range snapshotBuckets() {
			known = known || bucket.Name == string(name)
		}
		if !known || seen[bucket.Name] || bucket.Entries == nil {
			return nil, fmt.Errorf("%w: invalid bucket manifest", ErrSnapshotFormat)
		}
		seen[bucket.Name] = true
	}

	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("store: restore: create data directory: %w", err)
	}
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return nil, fmt.Errorf("store: restore: read data directory: %w", err)
	}
	if len(entries) != 0 {
		return nil, ErrDataDirInUse
	}
	path := filepath.Join(dataDir, FileName)
	s, err := openFile(path, nodeKey)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*Store, error) {
		s.Close()
		os.Remove(path)
		return nil, err
	}
	err = s.db.Update(func(tx *bbolt.Tx) error {
		for _, bucket := range body.Buckets {
			b, err := tx.CreateBucketIfNotExists([]byte(bucket.Name))
			if err != nil {
				return err
			}
			for _, entry := range bucket.Entries {
				if len(entry.Key) == 0 || entry.Value == nil || b.Get(entry.Key) != nil {
					return fmt.Errorf("%w: invalid or duplicate bucket entry", ErrSnapshotFormat)
				}
				if err := b.Put(entry.Key, entry.Value); err != nil {
					return err
				}
			}
		}
		// Validate the entire candidate in the same transaction as the writes.
		// No caller can obtain a partially restored, sign-capable store.
		return s.validateSnapshotState(tx)
	})
	if err != nil {
		return fail(err)
	}
	return s, nil
}

func backupAEAD(backupKey []byte, header []byte) (cipher.AEAD, []byte, error) {
	ad := append([]byte(nil), header...)
	ad = append(ad, backupDomain...)
	key, err := hkdf.Key(sha256.New, backupKey, nil, backupDomain, KeySize)
	if err != nil {
		return nil, nil, errors.New("store: derive backup key failed")
	}
	defer zero(key)
	aead, err := newGCM(key)
	if err != nil {
		return nil, nil, errors.New("store: backup cipher setup failed")
	}
	return aead, ad, nil
}

// validateSnapshotState authenticates the inner node-key envelopes as well as
// checking the lifecycle links. The outer backup envelope alone cannot establish
// that staged shares and decisions belong to the restored committed keys.
func (s *Store) validateSnapshotState(tx *bbolt.Tx) error {
	invalid := func(why string) error { return fmt.Errorf("%w: %s", ErrSnapshotFormat, why) }
	shares := make(map[string]ShareRecord)
	stages := make(map[string]ShareRecord)
	for _, bucket := range []struct {
		name    []byte
		records map[string]ShareRecord
	}{{bucketShares, shares}, {bucketStaged, stages}} {
		b := tx.Bucket(bucket.name)
		if b == nil {
			return invalid("missing share bucket")
		}
		if err := b.ForEach(func(k, v []byte) error {
			if v == nil {
				return invalid("nested share bucket")
			}
			rec, err := decodeRecord(string(k), v)
			if err != nil {
				return err
			}
			if err := validate(rec); err != nil {
				return err
			}
			plain, err := open(s.nodeKey, rec)
			if err != nil {
				return err
			}
			defer zero(plain)
			if len(plain) == 0 {
				return invalid("empty share")
			}
			bucket.records[string(k)] = rec
			return nil
		}); err != nil {
			return err
		}
	}
	for key, next := range stages {
		current, ok := shares[key]
		if !ok || current.Epoch == ^uint64(0) || next.Epoch != current.Epoch+1 || next.Curve != current.Curve || !bytes.Equal(next.PublicKey, current.PublicKey) {
			return invalid("staged share does not follow its committed key")
		}
	}
	replacements := make(map[string][]ShareReplacement)
	imports := make(map[string]ImportReceipt)
	refreshes := make(map[string]CeremonyRefresh)
	// Verification belongs to the server; only its durable ceremony binding is
	// interpreted here. All other record kinds remain opaque but authenticated.
	type verificationBinding struct {
		CeremonyID      string `json:"ceremony_id"`
		ImportSessionID string `json:"import_session_id"`
		SessionID       string `json:"session_id"`
		Epoch           uint64 `json:"epoch"`
	}
	verifications := make(map[string]verificationBinding)
	if err := tx.Bucket(bucketRecords).ForEach(func(k, v []byte) error {
		i := bytes.IndexByte(k, 0)
		if i <= 0 || i == len(k)-1 || v == nil {
			return invalid("invalid record key")
		}
		kind, id := RecordKind(k[:i]), string(k[i+1:])
		plain, err := s.openRecord(kind, id, v)
		if err != nil {
			return err
		}
		defer zero(plain)
		switch kind {
		case RecordReplacement:
			var history []ShareReplacement
			if json.Unmarshal(plain, &history) != nil || len(history) == 0 {
				return invalid("undecodable or empty replacement history")
			}
			replacements[id] = history
		case RecordCeremony:
			var receipt ImportReceipt
			if json.Unmarshal(plain, &receipt) != nil {
				return invalid("undecodable import receipt")
			}
			imports[id] = receipt
		case RecordRefresh:
			var state CeremonyRefresh
			if json.Unmarshal(plain, &state) != nil {
				return invalid("undecodable refresh decision")
			}
			refreshes[id] = state
		case RecordVerification:
			var state verificationBinding
			if json.Unmarshal(plain, &state) != nil {
				return invalid("undecodable verification decision")
			}
			verifications[id] = state
		}
		return nil
	}); err != nil {
		return err
	}
	origins := make(map[string]ShareRecord)
	for key, history := range replacements {
		held, ok := shares[key]
		if !ok {
			return invalid("replacement history has no committed key")
		}
		origin, err := s.replacementOrigin(key, held, history)
		if err != nil {
			return err
		}
		origins[key] = origin
	}
	for key, receipt := range imports {
		identity := receipt.Identity
		held, ok := shares[key]
		if origin, replaced := origins[key]; replaced {
			held = origin
		}
		if !ok || identity.KeyID != key || identity.CeremonyID == "" || identity.SessionID == "" || identity.Epoch != 0 || identity.Threshold != 3 || len(identity.Participants) != 5 || identity.Curve != held.Curve || !bytes.Equal(identity.PublicKey, held.PublicKey) || !equalStrings(identity.Participants, held.Participants) {
			return invalid("import receipt differs from its committed key")
		}
		if held.Epoch == identity.Epoch {
			plain, err := open(s.nodeKey, held)
			if err != nil {
				return err
			}
			digest := sha256.Sum256(plain)
			zero(plain)
			if digest != identity.MaterialDigest {
				return invalid("import receipt material digest differs")
			}
		}
	}
	for key, state := range refreshes {
		held, ok := shares[key]
		if !equalStrings(state.Participants, held.Participants) {
			for _, change := range replacements[key] {
				if equalStrings(state.Participants, change.Previous.Participants) {
					held = change.Previous
				}
			}
		}
		if !ok || state.KeyID != key || state.CeremonyID == "" || state.SessionID == "" || state.BaseEpoch == ^uint64(0) || len(state.Participants) != 5 || state.Curve != held.Curve || !bytes.Equal(state.PublicKey, held.PublicKey) || !equalStrings(state.Participants, held.Participants) {
			return invalid("refresh identity differs from its committed key")
		}
		switch state.State {
		case "prepared", "running", "staged", "complete":
		default:
			return invalid("unknown refresh state")
		}
		if state.Decision != "" && state.Decision != "commit" && state.Decision != "abort" {
			return invalid("unknown refresh decision")
		}
		receipt, imported := imports[key]
		if state.ExistingKey {
			if imported || state.ImportSessionID != "" {
				return invalid("existing-key refresh has an import identity")
			}
		} else {
			if !imported || receipt.AuditSequence == 0 || state.CeremonyID != receipt.Identity.CeremonyID || state.ImportSessionID != receipt.Identity.SessionID {
				return invalid("refresh has no matching audited import")
			}
		}
		next, staged := stages[key]
		if staged && (next.Epoch != state.BaseEpoch+1 || !equalStrings(next.Participants, state.Participants)) {
			return invalid("refresh stage differs from decision")
		}
		if held.Epoch == state.BaseEpoch {
			if state.State == "complete" || (state.Decision == "commit" && !staged) {
				return invalid("refresh decision has no recoverable stage")
			}
		} else if held.Epoch == state.BaseEpoch+1 {
			if state.Decision != "commit" || staged {
				return invalid("advanced key has no matching commit decision")
			}
		} else {
			return invalid("refresh epoch differs from committed key")
		}
		if state.State == "complete" && (state.Decision != "commit" || state.AuditSequence == 0) {
			return invalid("completed refresh has no audited commit")
		}
		// A stage can be persisted immediately before its state record is advanced.
		// A staged record with no stage is recoverable only after an abort.
		if state.State == "staged" && !staged && held.Epoch == state.BaseEpoch && state.Decision != "abort" {
			return invalid("staged refresh lost its share")
		}
	}
	for key, state := range verifications {
		held, ok := shares[key]
		receipt, imported := imports[key]
		if !ok || !imported || receipt.AuditSequence == 0 || state.CeremonyID != receipt.Identity.CeremonyID || state.ImportSessionID != receipt.Identity.SessionID || state.SessionID == "" || state.Epoch <= receipt.Identity.Epoch || state.Epoch > held.Epoch {
			return invalid("verification has no matching refreshed import")
		}
	}
	if b := tx.Bucket(bucketReplay); b != nil {
		if err := b.ForEach(func(k, v []byte) error {
			token := bytes.HasPrefix(k, prefixToken) && len(k) == len(prefixToken)+64
			nonce := bytes.HasPrefix(k, prefixNonce) && len(k) == len(prefixNonce)+48
			if (!token && !nonce) || len(v) != 8 {
				return invalid("malformed replay entry")
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}
