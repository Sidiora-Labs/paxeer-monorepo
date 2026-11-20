package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/app"
)

func TestExampleNetworkDocumentMatchesCommand(t *testing.T) {
	root := repositoryRoot(t)
	example, err := os.ReadFile(filepath.Join(root, "deploy", "launchpad", "documents", "values", documentName))
	if err != nil {
		t.Fatal(err)
	}
	var document app.NetworkAuthorityConfig
	if err := json.Unmarshal(example, &document); err != nil {
		t.Fatal(err)
	}
	endpoints := filepath.Join(t.TempDir(), "endpoints.json")
	writeJSON(t, endpoints, document.Endpoints)
	out := filepath.Join(t.TempDir(), "documents")
	if err := run([]string{
		"-repo", root,
		"-source-schema", document.SourceSchema,
		"-endpoints", endpoints,
		"-pool-runtime", document.ReviewedPoolRuntime,
		"-out", out,
	}, io.Discard); err != nil {
		t.Fatal(err)
	}
	generated, err := os.ReadFile(filepath.Join(out, documentName))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(example, generated) {
		return
	}
	var want, got map[string]json.RawMessage
	if err := json.Unmarshal(example, &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(generated, &got); err != nil {
		t.Fatal(err)
	}
	members := make([]string, 0, len(want)+len(got))
	for member := range want {
		members = append(members, member)
	}
	for member := range got {
		if _, exists := want[member]; !exists {
			members = append(members, member)
		}
	}
	sort.Strings(members)
	for _, member := range members {
		if !bytes.Equal(want[member], got[member]) {
			t.Fatal(member)
		}
	}
	t.Fatal(documentName)
}
