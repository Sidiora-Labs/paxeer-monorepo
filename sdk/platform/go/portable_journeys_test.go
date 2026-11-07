package layerx

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestPortableCataloguesHaveNoRetiredJourneys(t *testing.T) {
	for path, source := range portableCatalogueFiles(t) {
		t.Run(filepath.Ext(path), func(t *testing.T) {
			if !bytes.Contains(source, []byte("public enum PlatformOperation")) {
				t.Fatal("generated operation catalogue is missing its operation enum")
			}
			lower := strings.ToLower(string(source))
			for _, journey := range []string{"binding", "deposit", "exit", "withdraw"} {
				if strings.Contains(lower, journey) {
					t.Errorf("%s still names a retired %s operation", path, journey)
				}
			}
			for _, active := range []string{"HumanAccountBalance", "HumanIntentPlan", "HumanMoveCommit"} {
				if !strings.Contains(lower, strings.ToLower(active)) {
					t.Errorf("%s lost active operation %s", path, active)
				}
			}
		})
	}
}
