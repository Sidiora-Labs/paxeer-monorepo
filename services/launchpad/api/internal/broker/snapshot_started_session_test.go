package broker

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestSnapshotStartedTopicsSignedIn(t *testing.T) {
	_, stream := postgresStreamStorage(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	databaseURL, err := url.Parse(stream.pool.Config().ConnString())
	if err != nil {
		t.Fatal(err)
	}
	query := databaseURL.Query()
	query.Set("search_path", strings.Trim(stream.schema, `"`))
	databaseURL.RawQuery = query.Encode()
	dsn := databaseURL.String()
	chain := internaltest.NewAnvil(t)
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	moduleSource, err := os.ReadFile(filepath.Join("testdata", "identity_session", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	module := string(moduleSource)
	for relative, absolute := range map[string]string{
		"../../../../../../../sdk/launchpad/shared":   filepath.Join(root, "sdk/launchpad/shared"),
		"../../../../../../../sdk/launchpad/protocol": filepath.Join(root, "sdk/launchpad/protocol"),
		"../../../../../security/control":             filepath.Join(root, "services/launchpad/security/control"),
		"../../../../../gateway":                      filepath.Join(root, "services/launchpad/gateway"),
	} {
		module = strings.ReplaceAll(module, relative, absolute)
	}
	moduleFile := filepath.Join(t.TempDir(), "identity.mod")
	if err := os.WriteFile(moduleFile, []byte(module), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, "go", "run", "-mod=mod", "-modfile="+moduleFile, ".")
	cmd.Dir = filepath.Join("testdata", "identity_session")
	cmd.Env = append(os.Environ(), "GOWORK=off", "SNAPSHOT_IDENTITY_DSN="+dsn, "SNAPSHOT_REPOSITORY_ROOT="+root, "SNAPSHOT_CHAIN_RPC="+chain.URL)
	stderr := filepath.Join(t.TempDir(), "identity.log")
	logs, err := os.Create(stderr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = logs.Close() })
	cmd.Stderr = logs
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = input.Close()
		if err := cmd.Wait(); err != nil {
			raw, _ := os.ReadFile(stderr)
			t.Errorf("identity session: %v %s", err, raw)
		}
	})
	var session struct {
		URL       string
		Principal sharedauth.StreamPrincipal
	}
	if err := json.NewDecoder(output).Decode(&session); err != nil {
		raw, _ := os.ReadFile(stderr)
		t.Fatalf("identity session: %v %s", err, raw)
	}
	stream.Authority = sharedauth.HTTPStreamAuthority{Endpoint: session.URL, ServiceToken: strings.Repeat("t", 32)}
	if err := stream.Authority.CheckStreamSession(ctx, session.Principal); err != nil {
		t.Fatal(err)
	}
	first := publicPriceFrame(t, "1")
	empty := "market.price.0x3333333333333333333333333333333333333333"
	missing := "market.price.0x4444444444444444444444444444444444444444"
	topics := []string{first.Topic, empty, missing}
	binding := sharedauth.StreamTicketBinding{SchemaMajor: 2, Topics: topics, FilterDigest: sharedauth.StreamFilterDigest(topics), Origin: "https://web.example", Endpoint: "/v2/stream"}
	if _, err := stream.pool.Exec(ctx, "INSERT INTO "+stream.table("stream_projection")+"(topic,sequence) VALUES($1,0)", empty); err != nil {
		t.Fatal(err)
	}
	commitStreamFrame(t, ctx, stream, first)
	raw, err := stream.Snapshot(ctx, session.Principal, binding)
	if err != nil || interfaces.ValidateJSON("TopicSnapshot", raw) != nil {
		t.Fatalf("snapshot: %s %v", raw, err)
	}
	var snapshot struct {
		SnapshotID   string `json:"snapshotId"`
		ResumeCursor string `json:"resumeCursor"`
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatal(err)
	}
	var stored []byte
	var projectionCount int
	if err := stream.pool.QueryRow(ctx, "SELECT initial_offsets FROM "+stream.table("stream_snapshots")+" WHERE snapshot_id=$1", snapshot.SnapshotID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	var initial map[string]string
	if err := json.Unmarshal(stored, &initial); err != nil || len(initial) != 1 {
		t.Fatalf("initial offsets: %s %v", stored, err)
	}
	replay, err := stream.Resume(ctx, session.Principal, binding, snapshot.ResumeCursor)
	if err != nil {
		t.Fatal(err)
	}
	defer replay.Close()
	cursor, err := VerifyCursorV2(snapshot.ResumeCursor, stream.key, replay.Cursor.Scope, time.Now())
	if err != nil || len(cursor.Offsets) != 1 || len(replay.waiting) != 2 {
		t.Fatalf("cursor: %+v %v", cursor, err)
	}
	for partition, offset := range initial {
		if offset != "1" || cursor.Offsets[partition] != offset || replay.partitions[partition] != first.Topic {
			t.Fatalf("stored offset: %s %s", partition, offset)
		}
	}
	commitStreamFrame(t, ctx, stream, publicPriceFrame(t, "2"))
	if _, err := replay.Batch(ctx); err != nil {
		t.Fatal(err)
	}
	batch, err := replay.Batch(ctx)
	if err != nil || len(batch) != 1 {
		t.Fatalf("signed in delivery: %d %v", len(batch), err)
	}
	delivered, err := parseStreamV2(batch[0])
	if err != nil || delivered.Topic != first.Topic || delivered.Sequence != "2" {
		t.Fatalf("signed in frame: %+v %v", delivered, err)
	}
	if err := stream.pool.QueryRow(ctx, "SELECT count(*) FROM "+stream.table("stream_projection")).Scan(&projectionCount); err != nil || projectionCount != 2 {
		t.Fatalf("projection rows: %d %v", projectionCount, err)
	}
}
