package operations

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
)

func GenerateBindings(ctx context.Context, runner Commands, root, generator string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	dirs, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	count := 0
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		count++
		if err := generateBinding(ctx, runner, filepath.Join(root, dir.Name()), dir.Name(), generator); err != nil {
			return err
		}
	}
	if count == 0 {
		return errors.New("no binding packages found")
	}
	return nil
}

func generateBinding(ctx context.Context, runner Commands, dir, pkg, generator string) error {
	abiPath := filepath.Join(dir, "abi.json")
	before, err := os.ReadFile(abiPath)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(before)) == 0 {
		return errors.New("empty ABI")
	}
	// The generator gets a fresh directory, never a previous binding it could
	// leave untouched while exiting zero.
	stage, err := os.MkdirTemp(dir, ".abigen-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	output := filepath.Join(stage, pkg+".go")
	_, err = runner.Run(ctx, Command{Args: []string{generator, "--abi=" + abiPath, "--pkg=" + pkg, "--out=" + output}})
	if err != nil {
		return err
	}
	info, err := os.Lstat(output)
	if err != nil {
		return fmt.Errorf("abigen did not create %s: %w", pkg, err)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return errors.New("abigen output is missing, empty or not a regular file")
	}
	data, err := os.ReadFile(output)
	if err != nil {
		return err
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), output, data, parser.AllErrors)
	if err != nil {
		return err
	}
	if parsed.Name.Name != pkg || len(parsed.Decls) == 0 {
		return errors.New("abigen output has wrong package or no declarations")
	}
	after, err := os.ReadFile(abiPath)
	if err != nil {
		return err
	}
	if sha256.Sum256(before) != sha256.Sum256(after) {
		return errors.New("ABI changed during binding generation")
	}
	return os.Rename(output, filepath.Join(dir, pkg+".go"))
}
