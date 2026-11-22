package app

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
)

func TestMarketEventTypeStartupPolicy(t *testing.T) {
	cfg, _, document, _ := productionChartRuntime(t)
	for _, tc := range []struct {
		name, field string
		disabled    bool
	}{
		{name: "enabled subject and event type"},
		{name: "missing subject", field: "Subjects"},
		{name: "missing event type", field: "EventTypes"},
		{name: "disabled subject", field: "Subjects", disabled: true},
		{name: "disabled event type", field: "EventTypes", disabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := document
			d.Policy.Subjects = map[string]bool{accounting.MarketSubject: true, d.ChartOutbox.Subject: true}
			d.Policy.EventTypes = map[string]bool{accounting.MarketSubject: true}
			entries := d.Policy.Subjects
			if tc.field == "EventTypes" {
				entries = d.Policy.EventTypes
			}
			if tc.field != "" {
				delete(entries, accounting.MarketSubject)
				if tc.disabled {
					entries[accounting.MarketSubject] = false
				}
			}
			wire, err := json.Marshal(d)
			if err != nil {
				t.Fatal(err)
			}
			variant := cfg
			variant.DeliveryConfigFile = filepath.Join(t.TempDir(), "delivery.json")
			variant.DeliveryConfigSHA256 = production.Checksum(wire)
			if err = os.WriteFile(variant.DeliveryConfigFile, wire, 0600); err != nil {
				t.Fatal(err)
			}
			application, err := New(context.Background(), variant, slog.New(slog.NewTextHandler(io.Discard, nil)))
			if application != nil {
				defer application.Close()
			}
			if tc.field == "" {
				if err != nil {
					t.Fatal(err)
				}
				if application == nil || application.accounting == nil || application.transport == nil {
					t.Fatal("admitted market policy did not connect the accounting consumer")
				}
				return
			}
			want := "policy." + tc.field + " must enable " + accounting.MarketSubject
			if application != nil || err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("market policy admission: application=%v error=%v, want %q", application != nil, err, want)
			}
		})
	}
}
