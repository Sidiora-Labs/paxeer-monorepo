package publisher

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func TestDurableStagerUsesManifestDeploymentVersion(t *testing.T) {
	version := deployment.Current().DeploymentVersion
	stager, err := NewDurableStager(&delivery.Store{}, version, map[string]string{"Swap": "chain.swap"})
	if err != nil {
		t.Fatal(err)
	}
	event := RawCanonicalEvent{
		EventType: "Swap", TxHash: "0x" + strings.Repeat("a", 64),
		SourceOrder: delivery.SourceOrder{Height: "9", TransactionIndex: "1", LogIndex: "2"},
		EmittedAt:   time.Unix(9, 0).UTC(), Payload: json.RawMessage(`{"amountRaw":"7"}`),
	}
	subject, first, err := stager.Envelope(event)
	if err != nil || subject != "chain.swap" {
		t.Fatalf("manifest deployment event: %s %v", subject, err)
	}
	wantID := delivery.Hash([]byte("125/" + version + "/" + event.TxHash + "/" + event.SourceOrder.LogIndex))
	if first.EventID != wantID {
		t.Fatalf("event deployment identity = %s, want %s", first.EventID, wantID)
	}
	_, second, err := stager.Envelope(event)
	if err != nil || second.EventID != first.EventID {
		t.Fatalf("event identity was not stable: %s/%s (%v)", first.EventID, second.EventID, err)
	}
}

func TestDurableStagerMatchesManifestDeploymentVersionRule(t *testing.T) {
	valid := []string{"a", "a.b", string([]byte{'a'}) + strings.Repeat("-", 95)}
	invalid := []string{"", "a b", "a/b", ".a", "_a", strings.Repeat("a", 97)}
	for _, value := range valid {
		if _, err := NewDurableStager(&delivery.Store{}, value, map[string]string{"Swap": "chain.swap"}); err != nil {
			t.Fatalf("manifest version %q refused: %v", value, err)
		}
	}
	for _, value := range invalid {
		if _, err := NewDurableStager(&delivery.Store{}, value, map[string]string{"Swap": "chain.swap"}); err == nil {
			t.Fatalf("invalid manifest version %q admitted", value)
		}
	}
}
