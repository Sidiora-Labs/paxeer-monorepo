package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMeasurementDocumentsAndFilesHaveFiniteBounds(t *testing.T) {
	for _, wire := range []string{"", `{"a":1,"a":2}`, `{"a":{"b":1,"b":2}}`, `[1,]`, `{"a":}`, `{"a":[`, `"unterminated`, strings.Repeat("[", 18) + "0" + strings.Repeat("]", 18), `{} {}`, "[" + strings.Repeat("0,", 10000) + "0]"} {
		if err := strictObject([]byte(wire)); err == nil {
			t.Fatalf("ambiguous measurement document admitted: %.32s", wire)
		}
	}
	if err := strictObject([]byte(`{"array":[1,true,null,{"b":2}]}`)); err != nil {
		t.Fatal(err)
	}
	root := repairM951Candidate(t)
	output := filepath.Join(t.TempDir(), "index.json")
	repairM951Issue(t, writeSourceIndex("", output, nil), "candidate_source_index_invalid")
	repairM951Issue(t, writeSourceIndex(root, "", nil), "candidate_source_index_invalid")
	repairM951Issue(t, writeSourceIndex(root, output, make([]string, 10001)), "candidate_source_index_invalid")
	oversized := filepath.Join(root, "large.bin")
	file, err := os.Create(oversized)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate((64 << 20) + 1); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	repairM951Issue(t, writeSourceIndex(root, output, []string{"large.bin"}), "candidate_source_changed")
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("oversized input produced index: %v", err)
	}
	path := filepath.Join(t.TempDir(), "input.json")
	wire := []byte(`{"bounded":true}`)
	if err := os.WriteFile(path, wire, 0600); err != nil {
		t.Fatal(err)
	}
	_, err = readPinned(path, repairM951Digest(wire), 1)
	repairM951Issue(t, err, "pinned_file_size_invalid")
	_, err = readPinned(filepath.Dir(path), strings.Repeat("a", 64), 1<<20)
	repairM951Issue(t, err, "pinned_file_size_invalid")
	repairM951Issue(t, checkBuild(filepath.Join(t.TempDir(), "absent"), strings.Repeat("a", 64)), "consumer_build_artifact_missing")
}
