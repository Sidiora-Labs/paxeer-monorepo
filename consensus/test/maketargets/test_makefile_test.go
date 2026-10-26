package maketargets_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMakeRetiredIntegrationTargets(t *testing.T) {
	output, err := makeDryRun(t, "test")
	if err != nil {
		t.Fatalf("make test dry run failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "go test ") {
		t.Fatalf("make test dry run did not print a go test command:\n%s", output)
	}

	for _, target := range []string{"test_abci_apps", "test_integrations", "test100"} {
		t.Run(target, func(t *testing.T) {
			output, err := makeDryRun(t, target)
			if err == nil {
				t.Fatalf("retired target %s still resolves:\n%s", target, output)
			}
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 2 {
				t.Fatalf("expected make error exit code 2 for %s, got %v:\n%s", target, err, output)
			}
		})
	}
}

func TestMakeRetiredDockerTestImageTarget(t *testing.T) {
	output, err := makeDryRun(t, "test")
	if err != nil {
		t.Fatalf("make test dry run failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "go test ") {
		t.Fatalf("make test dry run did not print a go test command:\n%s", output)
	}

	output, err = makeDryRun(t, "build_docker_test_image")
	if err == nil {
		t.Fatalf("retired target build_docker_test_image still resolves:\n%s", output)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 2 {
		t.Fatalf("expected make error exit code 2 for build_docker_test_image, got %v:\n%s", err, output)
	}

	makefile, err := os.ReadFile(filepath.Join("..", "Makefile"))
	if err != nil {
		t.Fatalf("read consensus test Makefile: %v", err)
	}
	if strings.Contains(string(makefile), "build_docker_test_image") {
		t.Error("consensus test Makefile still references build_docker_test_image")
	}

	for _, name := range []string{"Dockerfile", "build.sh", "config-template.toml"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("..", "docker", name)
			if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("expected retired Docker file %s to be absent, got %v", path, err)
			}
		})
	}
}
