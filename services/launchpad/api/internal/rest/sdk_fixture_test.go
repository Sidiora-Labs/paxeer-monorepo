package rest_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
)

func productionSDKFixture(t *testing.T) string {
	t.Helper()
	root := filepath.Join(internaltest.RepositoryRoot(t), "sdk", "launchpad", "packages")
	workspace := t.TempDir()
	for _, name := range []string{"sdk", "sdk-v2"} {
		source := filepath.Join(root, name)
		target := filepath.Join(workspace, name)
		if err := os.CopyFS(filepath.Join(target, "src"), os.DirFS(filepath.Join(source, "src"))); err != nil {
			t.Fatal(err)
		}
		files := []string{"package.json"}
		if name == "sdk" {
			files = append(files, "tsconfig.json")
			if err := os.CopyFS(filepath.Join(target, "scripts"), os.DirFS(filepath.Join(source, "scripts"))); err != nil {
				t.Fatal(err)
			}
		}
		for _, name := range files {
			raw, err := os.ReadFile(filepath.Join(source, name))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(target, name), raw, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	run := func(dir, program string, args ...string) {
		t.Helper()
		command := exec.CommandContext(ctx, program, args...)
		command.Dir = dir
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("build repository SDK: %v\n%s", err, output)
		}
	}
	sdk := filepath.Join(workspace, "sdk")
	run(sdk, "npm", "install", "--ignore-scripts", "--no-audit", "--no-fund", "--package-lock=false")
	run(filepath.Join(workspace, "sdk-v2"), "node", filepath.Join(sdk, "node_modules", "typescript", "bin", "tsc"), "--target", "ES2022", "--module", "NodeNext", "--moduleResolution", "NodeNext", "--strict", "--declaration", "--outDir", "dist", "src/types.gen.ts")
	run(sdk, "npm", "run", "build")
	return filepath.Join(sdk, "dist", "index.js")
}
