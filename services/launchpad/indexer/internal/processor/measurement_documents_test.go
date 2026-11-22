package processor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func TestMeasurementTopologyRequiresBoundedDistinctIdentities(t *testing.T) {
	spec := nativeAccountingMeasurementSpec(t)
	raw, err := os.ReadFile(spec.TopologyPath)
	if err != nil {
		t.Fatal(err)
	}
	var original IngestTopologyEvidence
	if err := json.Unmarshal(raw, &original); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"syntax", "unknown field", "trailing", "missing authority", "duplicate zone", "empty node", "oversize", "missing", "directory"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "topology.json")
			value := original
			value.Zones = append([]string(nil), original.Zones...)
			value.NodeIDs = append([]string(nil), original.NodeIDs...)
			switch name {
			case "missing authority":
				value.ApprovedAuthorityID = ""
			case "duplicate zone":
				value.Zones[1] = value.Zones[0]
			case "empty node":
				value.NodeIDs[1] = ""
			}
			body, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			switch name {
			case "syntax":
				body = []byte(`{`)
			case "unknown field":
				body = []byte(`{"unknown":true}`)
			case "trailing":
				body = append(body, []byte(`{}`)...)
			case "oversize":
				body = []byte(strings.Repeat(" ", 1<<20+1))
			}
			if name == "directory" {
				path = t.TempDir()
			} else if name != "missing" {
				if err := os.WriteFile(path, body, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, _, err := readTopology(path, delivery.Hash(body)); err == nil {
				t.Fatal("invalid topology admitted")
			}
		})
	}
}

func TestMeasurementCandidateFilesRequireBoundedRegularContent(t *testing.T) {
	for _, name := range []string{"missing", "directory", "oversize", "content"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "candidate.go")
			switch name {
			case "directory":
				path = t.TempDir()
			case "oversize":
				f, err := os.Create(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := f.Truncate(64<<20 + 1); err != nil {
					t.Fatal(err)
				}
				if err := f.Close(); err != nil {
					t.Fatal(err)
				}
			case "content":
				if err := os.WriteFile(path, []byte("package candidate\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := validateCandidateFiles(map[string]string{path: strings.Repeat("0", 64)}, strings.Repeat("0", 64)); err == nil {
				t.Fatal("unbound candidate file admitted")
			}
		})
	}
	if err := validateCandidateFiles(nil, ""); err == nil {
		t.Fatal("empty candidate inventory admitted")
	}
	for _, body := range []string{`{`, `{"Unknown":true}`, `{} {}`} {
		path := filepath.Join(t.TempDir(), "checkpoint.json")
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadRestoreCheckpoint(path, delivery.Hash([]byte(body))); err == nil {
			t.Fatal("malformed restore checkpoint admitted")
		}
	}
	if _, err := loadRestoreCheckpoint(filepath.Join(t.TempDir(), "absent"), strings.Repeat("0", 64)); err == nil {
		t.Fatal("missing checkpoint admitted")
	}
	var h measurementHistogram
	if h.p99() != -1 {
		t.Fatal("empty histogram reports a latency")
	}
	h.add(-2*time.Millisecond, 1)
	h.add(time.Second, 0)
	if h.p99() != 0 || h.count != 1 {
		t.Fatalf("negative latency normalization: %d/%d", h.p99(), h.count)
	}
}
