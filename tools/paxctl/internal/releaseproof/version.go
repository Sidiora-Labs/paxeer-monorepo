// Package releaseproof verifies release versions and the annotations emitted by
// the shipped CLI against a local Kubernetes discovery/SSA fixture.
package releaseproof

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
)

const VersionPattern = `^v[0-9]+\.[0-9]+\.[0-9]+(-[A-Za-z0-9.-]+)?$`

func ValidateVersion(version string) error {
	if !regexp.MustCompile(VersionPattern).MatchString(version) {
		return fmt.Errorf("release version %q does not match %s", version, VersionPattern)
	}
	return nil
}

func ReadVersion(path, expected string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", err
	}
	if err := ValidateVersion(manifest.Version); err != nil {
		return "", err
	}
	if expected != "" && expected != manifest.Version {
		return "", fmt.Errorf("release tag %q differs from version.json %q", expected, manifest.Version)
	}
	return manifest.Version, nil
}
