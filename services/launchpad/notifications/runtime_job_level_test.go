package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestRuntimeJobLevelConsecutiveFailuresAndRecovery(t *testing.T) {
	ctx := context.Background()
	db := notificationIdentityOpen(t, ctx, notificationIdentityDatabase(t, ctx))
	if _, err := db.ExecContext(ctx, Schema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`ALTER TABLE discovery_notifications.inbox RENAME TO level_inbox`); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	pass := 0
	run, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := runtimeJobs{logger: logger, wait: func(ctx context.Context, delay time.Duration) error {
		pass++
		want := []time.Duration{time.Second, 2 * time.Second, 30 * time.Second, time.Second}
		if pass > len(want) || delay != want[pass-1] {
			t.Fatalf("pass %d delay=%v", pass, delay)
		}
		switch pass {
		case 2:
			if _, err := db.Exec(`ALTER TABLE discovery_notifications.level_inbox RENAME TO inbox`); err != nil {
				t.Fatal(err)
			}
		case 3:
			if _, err := db.Exec(`ALTER TABLE discovery_notifications.inbox RENAME TO level_inbox`); err != nil {
				t.Fatal(err)
			}
		case 4:
			cancel()
			return ctx.Err()
		}
		return nil
	}}
	err := jobs.run(run, []runtimeJob{{name: "retention", interval: 30 * time.Second, timeout: time.Second, pass: func(ctx context.Context) error {
		_, err := (Store{DB: db}).Retain(ctx, time.Now().UTC(), 90, 1000)
		return err
	}}})
	if !errors.Is(err, context.Canceled) || pass != 4 {
		t.Fatalf("job completion: passes=%d error=%v", pass, err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("failed pass lines: %s", output.String())
	}
	for index, level := range []string{"WARN", "ERROR", "WARN"} {
		var record struct {
			Level    string `json:"level"`
			Message  string `json:"msg"`
			Job      string `json:"job"`
			Error    string `json:"error"`
			Failures int    `json:"failures"`
		}
		if err := json.Unmarshal([]byte(lines[index]), &record); err != nil {
			t.Fatal(err)
		}
		failures := []int{1, 2, 1}
		if record.Level != level || record.Message != "notification runtime job failed" || record.Job != "retention" || !strings.Contains(record.Error, "inbox") || record.Failures != failures[index] {
			t.Fatalf("failed pass %d: %+v", index, record)
		}
	}
}

func TestRuntimeJobLevelTerminalFailureUsesDefaultLogger(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	defer slog.SetDefault(previous)
	jobs := runtimeJobs{}
	worker := Worker{}
	err := jobs.run(context.Background(), []runtimeJob{{name: "push_delivery", interval: time.Second, timeout: time.Second, pass: func(ctx context.Context) error { return worker.Once(ctx, 20) }}})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("terminal job: %v", err)
	}
	var record struct {
		Level    string `json:"level"`
		Message  string `json:"msg"`
		Job      string `json:"job"`
		Error    string `json:"error"`
		Failures int    `json:"failures"`
	}
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record.Level != "ERROR" || record.Message != "notification runtime job failed" || record.Job != "push_delivery" || record.Error != ErrInvalid.Error() || record.Failures != 1 {
		t.Fatalf("terminal job record: %+v", record)
	}
}

func TestRuntimeJobLevelMaintenanceNamesRefreshWithoutLogging(t *testing.T) {
	runtime, _ := runtimeRetriesFixture(t)
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	defer slog.SetDefault(previous)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := runtime.maintain(ctx)
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "refresh sources") || output.Len() != 0 {
		t.Fatalf("maintenance result=%v diagnostics=%s", err, output.String())
	}
}

func TestRuntimeJobLevelConfigLoggerReceivesMaintenanceStep(t *testing.T) {
	original, peer := runtimeRetriesFixture(t)
	var output bytes.Buffer
	config := original.Config
	config.Logger = slog.New(slog.NewJSONHandler(&output, nil))
	runtime, err := NewHTTPRuntime(context.Background(), original.DB, func(r *http.Request) (string, error) {
		p, err := peer.principal(r)
		return p.AccountID, err
	}, config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.DB.Exec(`ALTER TABLE discovery_notifications.inbox RENAME TO level_inbox`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runtime.wait = func(ctx context.Context, _ time.Duration) error {
		cancel()
		return ctx.Err()
	}
	err = runtime.run(ctx, []runtimeJob{{name: "maintenance", interval: 30 * time.Second, timeout: time.Second, pass: runtime.maintain}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("maintenance shutdown: %v", err)
	}
	var record struct {
		Level string `json:"level"`
		Job   string `json:"job"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record.Level != "WARN" || record.Job != "maintenance" || !strings.Contains(record.Error, "retain notifications") || !strings.Contains(record.Error, "inbox") {
		t.Fatalf("configured runtime logger: %+v", record)
	}
}

func TestRuntimeJobLevelMaintenancePreservesDatabaseCauses(t *testing.T) {
	for _, step := range []struct{ table, name string }{
		{"indexer.indexer_canonical_creations_v1", "refresh sources"},
		{"discovery_notifications.inbox", "retain notifications"},
		{"narrative_discovery.snapshots", "expire snapshots"},
	} {
		t.Run(step.name, func(t *testing.T) {
			runtime, _ := runtimeRetriesFixture(t)
			if _, err := runtime.DB.Exec("ALTER TABLE " + step.table + " RENAME TO level_unavailable"); err != nil {
				t.Fatal(err)
			}
			err := runtime.maintain(context.Background())
			var cause *pgconn.PgError
			if err == nil || !strings.Contains(err.Error(), step.name) || !errors.As(err, &cause) || cause.Code != "42P01" {
				t.Fatalf("maintenance step %s: %v", step.name, err)
			}
		})
	}
}
