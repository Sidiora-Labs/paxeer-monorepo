package readpolicy

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMeasurementArtifactRequiresBoundedRegularFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "source")
	if err := os.WriteFile(path, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linked")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	for _, artifact := range []BoundMeasurementArtifact{
		{},
		{Path: path, SHA256: strings.Repeat("a", 64), MaxBytes: 0},
		{Path: path, SHA256: strings.Repeat("a", 64), MaxBytes: 1<<40 + 1},
		{Path: path, SHA256: "invalid", MaxBytes: 10},
		{Path: link, SHA256: strings.Repeat("a", 64), MaxBytes: 10},
		{Path: path, SHA256: strings.Repeat("a", 64), MaxBytes: 1},
	} {
		if file, err := artifact.Open(); err == nil || file != nil {
			t.Fatalf("unbound artifact opened: %+v %v", artifact, err)
		}
	}
}

func TestMeasurementCapturePropagatesFilesystemErrors(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{file, filepath.Join(file, "parent", "capture")} {
		if capture, err := NewMeasurementCapture(directory, measuredTestBinding(t), 1000, 1<<20); err == nil || capture != nil {
			t.Fatalf("capture admitted invalid directory: %s %v", directory, err)
		}
	}
	if capture, err := NewMeasurementCapture(filepath.Join(root, "invalid"), MeasurementBinding{}, 1000, 1<<20); err == nil || capture != nil {
		t.Fatalf("capture admitted invalid binding: %+v %v", capture, err)
	}
	capture, err := NewMeasurementCapture(filepath.Join(root, "capture"), measuredTestBinding(t), 1000, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if err := capture.file.Close(); err != nil {
		t.Fatal(err)
	}
	result, err := capture.Finish(true, "")
	if !errors.Is(err, os.ErrClosed) || result.LaneHarnessPassed {
		t.Fatalf("closed capture finalization: %+v %v", result, err)
	}
	if _, err := capture.Finish(true, ""); err == nil {
		t.Fatal("capture finalized twice")
	}
}
