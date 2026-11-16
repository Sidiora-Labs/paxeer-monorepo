package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
)

func TestHistoryBelowAnchorLifecycleRequests(t *testing.T) {
	values := t.TempDir()
	root := filepath.Clean(filepath.Join("..", "..", "..", "..", ".."))
	examples := filepath.Join(root, "deploy/launchpad/documents/values")
	write := func(name string, raw []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(values, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	read := func(name string) []byte {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(examples, name))
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	write("common.json", read("common.example.json"))
	if err := os.Mkdir(filepath.Join(values, "expected-schema"), 0700); err != nil {
		t.Fatal(err)
	}
	write("expected-schema/indexer.json", []byte(`{"service":"core/indexer"}`))
	var data map[string]any
	if err := json.Unmarshal(read("indexer.example.json"), &data); err != nil {
		t.Fatal(err)
	}
	activation := deployment.Current().ActivationHeight
	data["history_from_height"] = activation
	data["anchor_height"] = activation + 1_000_000
	data["history_operator"] = "launchpad_operator"
	data["history_expires_at"] = "2099-01-01T00:00:00Z"
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	write("indexer.json", raw)
	out := t.TempDir()
	command := exec.Command("python3", filepath.Join(root, "deploy/launchpad/documents/compose.py"), "--values", values, "--out", out, "--service", "indexer")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compose lifecycle: %s: %v", output, err)
	}
	raw, err = os.ReadFile(filepath.Join(out, "indexer.json"))
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]string
	if err = json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	decode := func(name string, target any) {
		t.Helper()
		raw, err := base64.StdEncoding.DecodeString(env[name])
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(raw, target); err != nil {
			t.Fatal(err)
		}
	}
	var doc lifecycleRuntimeDocument
	var delivery deliveryRuntimeDocument
	var authority receiptRuntimeDocument
	decode("INDEXER_LIFECYCLE_CONFIG_DOCUMENT", &doc)
	decode("INDEXER_RECEIPT_CONFIG_DOCUMENT", &authority)
	decode("DELIVERY_CONFIG_DOCUMENT", &delivery)
	cfg := config.Config{IndexerMode: config.ModeBackfill, BackfillBatchSize: 32}
	cfg.ChainID = 125
	if len(doc.BackfillRequests) != 2 {
		t.Fatalf("split history: %d requests", len(doc.BackfillRequests))
	}
	if err = doc.validate(delivery, cfg); err != nil {
		t.Fatal(err)
	}
	if err = bindRuntimeAnchor(&doc, authority.Production); err != nil {
		t.Fatal(err)
	}
	if doc.BackfillRequests[0].ToHeight-doc.BackfillRequests[0].FromHeight+1 != 1_000_000 || doc.BackfillRequests[1].FromHeight != doc.BackfillRequests[0].ToHeight+1 {
		t.Fatal("split history must preserve every height")
	}
	original, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func(*lifecycleRuntimeDocument)
	}{
		{"other stream", func(d *lifecycleRuntimeDocument) { d.BackfillRequests[0].SourceStream = "other-stream" }},
		{"oversized request", func(d *lifecycleRuntimeDocument) { d.BackfillRequests[0].ToHeight++ }},
		{"gap", func(d *lifecycleRuntimeDocument) { d.BackfillRequests[1].FromHeight++ }},
		{"overlap", func(d *lifecycleRuntimeDocument) { d.BackfillRequests[1].FromHeight-- }},
		{"other checkpoint", func(d *lifecycleRuntimeDocument) {
			d.BackfillRequests[1].CheckpointHash = "0x" + string(make([]byte, 64))
		}},
		{"replay mode", func(d *lifecycleRuntimeDocument) { d.BackfillRequests[0].Mode = "replay" }},
		{"exclusive sources", func(d *lifecycleRuntimeDocument) { r := d.BackfillRequests[0]; d.BackfillRequest = &r }},
		{"partial range", func(d *lifecycleRuntimeDocument) { d.BackfillRequests = d.BackfillRequests[:1] }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var changed lifecycleRuntimeDocument
			if err := json.Unmarshal(original, &changed); err != nil {
				t.Fatal(err)
			}
			tc.edit(&changed)
			if err := changed.validate(delivery, cfg); err == nil {
				t.Fatal("invalid lifecycle request accepted")
			}
		})
	}
	cfg.BackfillBatchSize = 65
	if err = doc.validate(delivery, cfg); err == nil {
		t.Fatal("backfill beyond qualified batch bound accepted")
	}
}

func TestHistoryBelowAnchorReadinessStartsWithLive(t *testing.T) {
	r, _ := canonicalSourceReadinessCase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := r.Ready(ctx); !errors.Is(err, errCanonicalSourceUnrefreshed) {
		t.Fatalf("historical bootstrap readiness: %v", err)
	}
	r.Start(ctx)
	for {
		err := r.Ready(ctx)
		if err == nil {
			break
		}
		if !errors.Is(err, errCanonicalSourceUnrefreshed) {
			t.Fatal(err)
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
	r.Close()
	if err := r.Ready(context.Background()); err != nil {
		t.Fatalf("live readiness refresh retained: %v", err)
	}
}
