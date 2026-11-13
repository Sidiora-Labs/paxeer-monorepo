package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/config"
	canonicalsearch "github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/search"
	shareddb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRuntimeDocumentBindsEverySubscription(t *testing.T) {
	base := readDelivery(t, "testdata/delivery.production.json")
	extra := func(d *statsRuntimeDocument, kind, event string) {
		sub := d.Phase2.SearchConsumers[0]
		sub.Kind = kind
		sub.EventType = event
		sub.Consumer.Durable = "extra"
		d.Policy.EventTypes[event] = true
		d.Phase2.SearchConsumers = append(d.Phase2.SearchConsumers, sub)
	}
	for _, tc := range []struct {
		name, want string
		edit       func(*statsRuntimeDocument)
	}{
		{"missing phase", "incomplete ordered", func(d *statsRuntimeDocument) { d.Phase2 = nil }},
		{"market policy", "market accounting policy", func(d *statsRuntimeDocument) { delete(d.Policy.EventTypes, "accounting.market.v1") }},
		{"zero window", "invalid expiry windows", func(d *statsRuntimeDocument) { d.Phase2.Windows = []int64{0} }},
		{"duplicate window", "invalid expiry windows", func(d *statsRuntimeDocument) { d.Phase2.Windows = []int64{60, 60} }},
		{"broker bounds", "", func(d *statsRuntimeDocument) { d.Phase2.SearchConsumers[0].Consumer.Batch = 0 }},
		{"unapproved subscription", "unapproved or duplicate", func(d *statsRuntimeDocument) { delete(d.Policy.Subjects, "canonical.creation.v1") }},
		{"duplicate subscription", "unapproved or duplicate", func(d *statsRuntimeDocument) {
			d.Phase2.SearchConsumers = append(d.Phase2.SearchConsumers, d.Phase2.SearchConsumers[0])
		}},
		{"creation binding", "canonical creation source", func(d *statsRuntimeDocument) { d.Phase2.SearchConsumers[0].EventType = "stats.minute.v2" }},
		{"duplicate creation", "duplicate canonical creation", func(d *statsRuntimeDocument) { extra(d, "creation", "canonical_creation") }},
		{"invalidation binding", "invalid search invalidation", func(d *statsRuntimeDocument) { d.Phase2.SearchConsumers[1].EventType = "stats.minute.v2" }},
		{"duplicate invalidation", "invalid search invalidation", func(d *statsRuntimeDocument) {
			sub := d.Phase2.SearchConsumers[1]
			sub.Consumer.Durable = "extra"
			d.Phase2.SearchConsumers = append(d.Phase2.SearchConsumers, sub)
		}},
		{"missing source", "authorized search source", func(d *statsRuntimeDocument) { extra(d, "metadata", "metadata.updated") }},
		{"source version", "authorized search source", func(d *statsRuntimeDocument) {
			extra(d, "metadata", "metadata.updated")
			d.Phase2.SearchSources["metadata"] = canonicalsearch.SourceRelation{Schema: "metadata", View: "events_v1"}
		}},
		{"account event", "authorized account search event", func(d *statsRuntimeDocument) { extra(d, "account_profile", "unknown.event") }},
		{"unsupported kind", "unsupported search subscription", func(d *statsRuntimeDocument) { extra(d, "unknown", "unknown.event") }},
		{"required subscriptions", "creation and invalidation", func(d *statsRuntimeDocument) {
			d.Phase2.SearchConsumers[0].Kind = "account_profile"
			d.Phase2.SearchConsumers[0].EventType = "profile.updated"
			d.Policy.EventTypes["profile.updated"] = true
		}},
		{"market broker bounds", "", func(d *statsRuntimeDocument) { d.Consumers[0].Batch = 0 }},
		{"market subject", "unapproved subject", func(d *statsRuntimeDocument) { d.Consumers[0].Subject = "canonical.creation.v1" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(base)
			if err != nil {
				t.Fatal(err)
			}
			var d statsRuntimeDocument
			if err = json.Unmarshal(raw, &d); err != nil {
				t.Fatal(err)
			}
			tc.edit(&d)
			if err = d.validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("document binding: %v, want %s", err, tc.want)
			}
		})
	}
	for _, kind := range []string{"metadata", "lifecycle", "metrics", "visibility", "profile_deletion", "preferences", "account_profile"} {
		t.Run(kind, func(t *testing.T) {
			raw, err := json.Marshal(base)
			if err != nil {
				t.Fatal(err)
			}
			var d statsRuntimeDocument
			if err = json.Unmarshal(raw, &d); err != nil {
				t.Fatal(err)
			}
			extra(&d, kind, "profile.updated")
			d.Phase2.SearchSources[kind] = canonicalsearch.SourceRelation{Schema: "metadata", View: "events_v2"}
			if err = d.validate(); err != nil {
				t.Fatalf("authorized source rejected: %v", err)
			}
		})
	}
}

func TestRuntimeConstructorReportsDependencyAndDocumentErrors(t *testing.T) {
	base, document := productionRuntimeConfig(t)
	for _, tc := range []struct {
		name, want   string
		editConfig   func(*config.Config)
		editDocument func(*statsRuntimeDocument)
	}{
		{name: "database URL", editConfig: func(c *config.Config) { c.DatabaseURL = "postgres://%" }, want: "parse"},
		{name: "cache URL", editConfig: func(c *config.Config) { c.RedisURL = "redis://%" }, want: "parse"},
		{name: "cache unavailable", editConfig: func(c *config.Config) { c.RedisURL = "redis://127.0.0.1:1" }, want: "connect"},
		{name: "RPC URL", editConfig: func(c *config.Config) { c.RPCURL = "://invalid" }, want: "parse"},
		{name: "epoch", editDocument: func(d *statsRuntimeDocument) { d.Epoch = 0 }, want: "invalid recovery authority"},
		{name: "read version", editDocument: func(d *statsRuntimeDocument) { d.ProjectionVersion = strings.Repeat("v", 65) }, want: "canonical accounting projection required"},
		{name: "replay bound", editDocument: func(d *statsRuntimeDocument) { d.MaxReplayFills = 0 }, want: "bounded replay"},
		{name: "broker unavailable", editConfig: func(c *config.Config) { c.NATSURL = "nats://127.0.0.1:1" }, want: "connect"},
		{name: "missing market durable", editDocument: func(d *statsRuntimeDocument) { d.Consumers[0].Durable = "absent" }, want: "consumer"},
		{name: "missing search durable", editDocument: func(d *statsRuntimeDocument) { d.Phase2.SearchConsumers[0].Consumer.Durable = "absent" }, want: "consumer"},
		{name: "invalid document", editDocument: func(d *statsRuntimeDocument) { d.Phase2.Windows = []int64{0} }, want: "invalid expiry"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := base
			raw, err := json.Marshal(document)
			if err != nil {
				t.Fatal(err)
			}
			var d statsRuntimeDocument
			if err = json.Unmarshal(raw, &d); err != nil {
				t.Fatal(err)
			}
			if tc.editDocument != nil {
				tc.editDocument(&d)
				raw, err = json.Marshal(d)
				if err != nil {
					t.Fatal(err)
				}
				c.DeliveryConfigFile = filepath.Join(t.TempDir(), "delivery.json")
				if err = os.WriteFile(c.DeliveryConfigFile, raw, 0600); err != nil {
					t.Fatal(err)
				}
				sum := sha256.Sum256(raw)
				c.DeliveryConfigSHA256 = hex.EncodeToString(sum[:])
			}
			if tc.editConfig != nil {
				tc.editConfig(&c)
			}
			for k, v := range map[string]string{"DATABASE_URL": c.DatabaseURL, "REDIS_URL": c.RedisURL, "RPC_URL": c.RPCURL, "NATS_URL": c.NATSURL} {
				t.Setenv(k, v)
			}
			a, err := newWithRuntimePool(context.Background(), c, slog.New(slog.NewTextHandler(io.Discard, nil)), func(ctx context.Context, c *config.Config, opts shareddb.PoolOptions) (*pgxpool.Pool, error) {
				return shareddb.NewPool(ctx, c.DatabaseURL, opts)
			})
			if a != nil {
				a.Close()
			}
			if a != nil || err == nil || !strings.Contains(strings.ToLower(err.Error()), tc.want) {
				t.Fatalf("runtime dependency: initialized=%t err=%v, want %s", a != nil, err, tc.want)
			}
		})
	}
}
