package delivery

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
)

func archiveManifestForDeployment(version string) ArchiveManifest {
	return ArchiveManifest{
		Version: 1, ChainID: 125, SourceStream: "canonical", Deployment: version,
		DecoderDigest: strings.Repeat("a", 64), ABIDigest: strings.Repeat("b", 64), CodeDigest: strings.Repeat("c", 64),
		FromHeight: 10, ToHeight: 10, Blocks: []ArchiveBlock{{Height: 10, BlockHash: "0x" + strings.Repeat("d", 64), SegmentHash: Hash(nil)}}, Complete: true,
	}
}

func TestArchiveDeploymentVersionMatchesManifestRule(t *testing.T) {
	valid := []string{"a", "a.b", string([]byte{'a'}) + strings.Repeat("-", 95)}
	invalid := []string{"", "a b", "a/b", ".a", "_a", strings.Repeat("a", 97)}
	for _, version := range valid {
		if _, err := NewArchive(&Store{}, &storage.Client{}, "archive", "canonical", version, strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64)); err != nil {
			t.Fatalf("manifest version %q refused by constructor: %v", version, err)
		}
		if err := ValidateArchiveManifest(archiveManifestForDeployment(version)); err != nil {
			t.Fatalf("manifest version %q refused by manifest check: %v", version, err)
		}
	}
	for _, version := range invalid {
		if _, err := NewArchive(&Store{}, &storage.Client{}, "archive", "canonical", version, strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64)); err == nil {
			t.Fatalf("invalid manifest version %q admitted by constructor", version)
		}
		if err := ValidateArchiveManifest(archiveManifestForDeployment(version)); err == nil {
			t.Fatalf("invalid manifest version %q admitted by manifest check", version)
		}
	}
}

func TestArchiveDottedDeploymentKeepsManifestObjectIdentityDistinct(t *testing.T) {
	one, err := json.Marshal(archiveManifestForDeployment("a.b"))
	if err != nil {
		t.Fatal(err)
	}
	two, err := json.Marshal(archiveManifestForDeployment("ab"))
	if err != nil {
		t.Fatal(err)
	}
	if Hash(one) == Hash(two) {
		t.Fatal("dotted deployment versions share a manifest object identity")
	}
}

func TestDeliveryDeploymentVersionMatchesManifestAdmission(t *testing.T) {
	values := []string{
		deployment.Current().DeploymentVersion,
		"a", "0", "A", "a.b", "a..b", "a.", "a_b-c.0",
		strings.Repeat("a", 95), strings.Repeat("a", 96),
		"a" + strings.Repeat(".", 95), "a" + strings.Repeat("-", 95),
		"", "a b", "a/b", ".a", "_a", "-a", "a\n", "é", "aé",
		strings.Repeat("a", 97), "a" + strings.Repeat(".", 96),
	}
	for _, value := range values {
		manifest := deployment.Current()
		manifest.DeploymentVersion = value
		want := manifest.Validate() == nil
		if got := ValidDeploymentVersion(value); got != want {
			t.Fatalf("deployment version %q: delivery admission = %t, manifest admission = %t", value, got, want)
		}
	}
}

func TestArchiveUsesManifestDeploymentVersion(t *testing.T) {
	version := deployment.Current().DeploymentVersion
	archive, err := NewArchive(&Store{}, &storage.Client{}, "archive", "canonical", version, strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64))
	if err != nil {
		t.Fatal(err)
	}
	if archive.deployment != version {
		t.Fatalf("archive deployment = %q, want %q", archive.deployment, version)
	}
	if err := ValidateArchiveManifest(archiveManifestForDeployment(version)); err != nil {
		t.Fatal(err)
	}
}
