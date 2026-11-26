package app

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/config"
	canonicalsearch "github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/search"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
)

func TestAccountProfileSubscriptionDocuments(t *testing.T) {
	root, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}
	database := coordinatortest.Bootstrap(t, os.Getenv("COORDINATOR_TEST_POSTGRES_DSN"), "stats_runtime")
	values, output := t.TempDir(), t.TempDir()
	for _, name := range []string{"common.example.json", "stats.example.json"} {
		raw, err := os.ReadFile(filepath.Join(root, "deploy/launchpad/documents/values", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(values, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(values, "expected-schema"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(values, "expected-schema/stats.json"), database.Runtimes["stats_runtime"].ExpectedSchema, 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("python3", filepath.Join(root, "deploy/launchpad/documents/compose.py"),
		"--examples", values, "--out", output, "--service", "stats")
	if raw, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compose account profile subscriptions: %v %s", err, raw)
	}
	raw, err := os.ReadFile(filepath.Join(output, "stats.json"))
	if err != nil {
		t.Fatal(err)
	}
	var environment map[string]string
	if err := json.Unmarshal(raw, &environment); err != nil {
		t.Fatal(err)
	}
	raw, err = base64.StdEncoding.DecodeString(environment["DELIVERY_CONFIG_DOCUMENT"])
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(output, "delivery.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	var composed statsRuntimeDocument
	if err := config.ReadBoundDocument(path, environment["DELIVERY_CONFIG_SHA256"], &composed); err != nil {
		t.Fatal(err)
	}
	fixture := readDelivery(t, "testdata/delivery.production.json")
	for name, document := range map[string]statsRuntimeDocument{"composed": composed, "production": fixture} {
		t.Run(name, func(t *testing.T) {
			if err := document.validate(); err != nil {
				t.Fatal(err)
			}
			expected := map[string]string{
				"profile.updated":            "stats_search_profile_updated",
				"account.deletion.requested": "stats_search_account_deletion_requested",
				"account.backfill.completed": "stats_search_account_backfill_completed",
			}
			seen := map[string]bool{}
			projection := &canonicalsearch.Projection{}
			want := runtime.FuncForPC(reflect.ValueOf(projection.ConsumeAccountProfile).Pointer()).Name()
			for _, sub := range document.Phase2.SearchConsumers {
				if sub.Kind != "account_profile" {
					continue
				}
				durable, ok := expected[sub.EventType]
				if !ok || seen[sub.EventType] || sub.Consumer.Stream != "USERS" || sub.Consumer.Durable != durable || sub.Consumer.Subject != "paxeer.125.users."+sub.EventType {
					t.Fatalf("account profile subscription: %+v", sub)
				}
				seen[sub.EventType] = true
				got := runtime.FuncForPC(reflect.ValueOf(statsSearchProjectionConsumer(sub.Kind, projection)).Pointer()).Name()
				if got != want {
					t.Fatalf("%s consumer: got %s, want %s", sub.EventType, got, want)
				}
			}
			if len(seen) != len(expected) {
				t.Fatalf("account profile subscriptions: got %v, want %v", seen, expected)
			}
		})
	}
}
