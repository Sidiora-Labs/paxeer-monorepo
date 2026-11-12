package holdings

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPhase4FeederCopiesBoundReceipt(t *testing.T) {
	copyCommand, err := exec.LookPath("cp")
	if err != nil {
		t.Fatal(err)
	}
	copyCommand, err = filepath.EvalSymlinks(copyCommand)
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.ReadFile(copyCommand)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	source := filepath.Join(directory, "source.ndjson")
	receipt := filepath.Join(directory, "receipt.ndjson")
	content := []byte("{\"source\":\"canonical\",\"count\":2}\n")
	if err := os.WriteFile(source, content, 0600); err != nil {
		t.Fatal(err)
	}
	p := Phase4Plan{SourceRoot: directory, MaxReceiptBytes: 1024, Feeder: Phase4Command{Executable: copyCommand, SHA256: phase4Hash(executable), Args: []string{source, receipt}, Output: receipt}}
	a := &phase4Artifacts{directory: filepath.Join(directory, "artifacts"), files: map[string]*os.File{}}
	if err := os.Mkdir(a.directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := phase4Feeder(context.Background(), p, a); err == nil {
		t.Fatal("unadmitted source command ran")
	}
	t.Setenv("L14_PHASE4_FEED_ADMITTED", "YES")
	if err := phase4Feeder(context.Background(), p, a); err != nil {
		output, _ := os.ReadFile(filepath.Join(a.directory, "source-feeder.stderr"))
		t.Fatalf("%v: %s", err, output)
	}
	if err := a.close(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(a.directory, "source-receipts.ndjson"))
	if err != nil || string(got) != string(content) {
		t.Fatalf("receipt: %q %v", got, err)
	}
	artifact, err := os.ReadFile(filepath.Join(a.directory, "source-feeder.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	var record struct {
		ReceiptFileSHA256 string `json:"receiptFileSha256"`
		ReceiptBytes      int    `json:"receiptBytes"`
	}
	if err := json.Unmarshal(artifact, &record); err != nil || record.ReceiptFileSHA256 != phase4Hash(content) || record.ReceiptBytes != len(content) {
		t.Fatalf("artifact: %+v %v", record, err)
	}
}
