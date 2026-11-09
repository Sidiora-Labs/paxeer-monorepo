package appenv

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/bridge/deploy/chainconfig"
)

func TestGovernedSolanaRecordRoundTripsToAppEnv(t *testing.T) {
	path, err := chainconfig.Path("../../../..", "solana")
	if err != nil {
		t.Fatal(err)
	}
	config, err := chainconfig.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if config.Threshold != 1 || !reflect.DeepEqual(config.Attestors, []string{"0x12598F221f1e645322C7665537D4Ab130281f51d"}) {
		t.Fatalf("unexpected governed signer configuration: %+v", config)
	}
	record := readySolanaRecord(t)
	record.Attestors, record.Threshold = config.Attestors, config.Threshold
	dir := t.TempDir()
	recordPath := filepath.Join(dir, "solana.json")
	if err := WriteSolanaRecord(recordPath, record); err != nil {
		t.Fatal(err)
	}
	reopened, err := LoadRecord(recordPath)
	if err != nil || !reflect.DeepEqual(reopened, record) {
		t.Fatalf("governed record did not survive reopening: %+v, %v", reopened, err)
	}
	var output bytes.Buffer
	if err := Run([]string{dir}, &output); err != nil {
		t.Fatal(err)
	}
	want := "NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID=" + record.ProgramID + "\n"
	if output.String() != want {
		t.Fatalf("app environment %q, want %q", output.String(), want)
	}
	envPath := filepath.Join(t.TempDir(), "app.env")
	if err := os.WriteFile(envPath, output.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Check(envPath, []Record{reopened}); err != nil {
		t.Fatal(err)
	}
	var checked bytes.Buffer
	if err := Run([]string{"--check", envPath, dir}, &checked); err != nil || checked.Len() != 0 {
		t.Fatalf("reopened app environment refused: %v, %q", err, checked.String())
	}
}
