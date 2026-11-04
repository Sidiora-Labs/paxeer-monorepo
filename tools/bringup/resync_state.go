package bringup

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type ResyncRecord struct {
	Version                              int
	Source, Destination, Original, Phase string
}
type ResyncState struct{ Path string }

func OpenResyncState(path string) (*ResyncState, error) {
	path, e := filepath.Abs(path)
	if e != nil {
		return nil, e
	}
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return nil, e
	}
	return &ResyncState{path}, nil
}
func (s *ResyncState) Close() { s.Path = "" }
func (s *ResyncState) Pending() (*ResyncRecord, error) {
	if s.Path == "" {
		return nil, errors.New("resync journal closed")
	}
	b, e := os.ReadFile(s.Path)
	if os.IsNotExist(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	var r ResyncRecord
	if json.Unmarshal(b, &r) != nil || r.Version != 1 || r.Source == "" || r.Destination == "" || r.Source == r.Destination || !resyncOriginal(r.Original) || (r.Phase != "prepared" && r.Phase != "copying" && r.Phase != "restoring") {
		return nil, errors.New("invalid resync recovery journal")
	}
	return &r, nil
}
func resyncOriginal(state string) bool {
	return state == "active" || state == "inactive" || state == "failed"
}
func (s *ResyncState) write(r *ResyncRecord) error {
	b, e := json.Marshal(r)
	if e != nil {
		return e
	}
	return edgeAtomic(s.Path, b, 0600)
}
func (s *ResyncState) Prepare(source, destination, original string) error {
	pending, e := s.Pending()
	if e != nil {
		return e
	}
	if pending != nil {
		return errors.New("recover the retained source before another destination")
	}
	if source == "" || destination == "" || strings.ContainsAny(source+destination, " \n\r\t") || strings.HasPrefix(source, "-") || strings.HasPrefix(destination, "-") || source == destination || !resyncOriginal(original) {
		return errors.New("invalid source identity or unstable original unit state")
	}
	return s.write(&ResyncRecord{1, source, destination, original, "prepared"})
}
func (s *ResyncState) Phase(phase string) error {
	r, e := s.Pending()
	if e != nil {
		return e
	}
	if r == nil {
		return errors.New("resync has no pending source")
	}
	if !(r.Phase == "prepared" && phase == "copying") && !(phase == "restoring" && (r.Phase == "prepared" || r.Phase == "copying" || r.Phase == "restoring")) {
		return errors.New("invalid resync phase transition")
	}
	r.Phase = phase
	return s.write(r)
}

// RecoveryAction never emits a start for a source originally stopped by
// its operator. The shell obtains actual state over the same service transport.
func (s *ResyncState) RecoveryAction(actual string) (string, error) {
	r, e := s.Pending()
	if e != nil {
		return "", e
	}
	if r == nil {
		return "", errors.New("no retained source")
	}
	if !resyncOriginal(actual) && actual != "activating" && actual != "deactivating" {
		return "", errors.New("source unit state unavailable")
	}
	if r.Original == "active" {
		if actual == "active" {
			return "none", nil
		}
		return "start", nil
	}
	if actual != r.Original {
		return "", errors.New("operator-stopped source state changed")
	}
	return "none", nil
}

func (s *ResyncState) Restored(actual string) error {
	r, e := s.Pending()
	if e != nil {
		return e
	}
	if r == nil {
		return errors.New("no source restoration pending")
	}
	if r.Phase != "restoring" || (r.Original == "active" && actual != "active") || (r.Original != "active" && actual != r.Original) {
		return errors.New("source original unit state has not been restored")
	}
	if e = os.Remove(s.Path); e != nil {
		return e
	}
	return edgeSyncDir(filepath.Dir(s.Path))
}
