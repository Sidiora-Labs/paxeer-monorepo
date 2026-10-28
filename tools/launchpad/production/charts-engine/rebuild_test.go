package main

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestRebuildArgumentsTargetServiceDirectory(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("test source path unavailable")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", "..", ".."))
	directory := filepath.Join(root, "services", "launchpad", "trading-charts")
	for _, extra := range [][]string{nil, {"-from", "123", "-to", "456"}} {
		got := rebuildArguments(source, extra)
		want := append([]string{"-C", directory, "run", "./cmd/charts-rebuild"}, extra...)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("rebuildArguments(%q, %q) = %q, want %q", source, extra, got, want)
		}
	}
	entrypoint := filepath.Join(directory, "cmd", "charts-rebuild", "main.go")
	info, err := os.Stat(entrypoint)
	if err != nil {
		t.Fatalf("stat chart rebuild entry point: %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("chart rebuild entry point %q is not a regular file", entrypoint)
	}
}
