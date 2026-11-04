package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	paxeer "github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/hosted/paxeer"
)

func TestRepairM_1323_NativeDeploymentProfile(t *testing.T) {
	profilePath := filepath.Join("..", "deployment-profile.json")
	deployment, err := paxeer.OpenDeployment(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := deployment.Profile()
	if err != nil {
		t.Fatal(err)
	}
	if profile.Node.ChainID != "hyperpax_125-1" || profile.Node.EVMChainID != 125 {
		t.Fatalf("native chain identity: %+v", profile.Node)
	}
	if err := deployment.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := deployment.Profile(); err == nil {
		t.Fatal("closed deployment remained readable")
	}
	deployment, err = paxeer.OpenDeployment(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Close()
	var manifest bytes.Buffer
	if err := deployment.WriteManifest(&manifest); err != nil {
		t.Fatal(err)
	}
	canonical, err := os.ReadFile(filepath.Join("..", profile.Node.Manifest))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(manifest.Bytes(), canonical) {
		t.Fatal("reopened deployment differs from the maintained node manifest")
	}

	reader, writer := io.Pipe()
	produced := make(chan error, 1)
	go func() {
		err := paxeer.RunDeployment([]string{"profile", profilePath}, writer)
		writer.CloseWithError(err)
		produced <- err
	}()
	defer reader.Close()
	var consumed paxeer.DeploymentProfile
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&consumed); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		t.Fatalf("profile stream has trailing data: %v", err)
	}
	if err := <-produced; err != nil {
		t.Fatal(err)
	}
	if consumed != profile {
		t.Fatalf("producer/transport/consumer changed profile: %+v != %+v", consumed, profile)
	}
}

func TestRepairM_1323_RetiredDeploymentInterface(t *testing.T) {
	for _, phase := range []string{"deploy", "permissions", "activate", "bond", "authority-schedule", "authority-execute", "finalize", "bootstrap", "check-profile", "status"} {
		t.Run(phase, func(t *testing.T) {
			var output bytes.Buffer
			if err := paxeer.RunDeployment([]string{phase, filepath.Join(t.TempDir(), "unopened-profile.json")}, &output); err == nil {
				t.Fatalf("retired phase %s was accepted", phase)
			}
			if output.Len() != 0 {
				t.Fatalf("retired phase %s emitted deployment data", phase)
			}
		})
	}
}
