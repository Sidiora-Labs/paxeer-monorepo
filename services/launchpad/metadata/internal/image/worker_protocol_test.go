package image

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestRasterWorkerProtocol(t *testing.T) {
	var raster bytes.Buffer
	if err := png.Encode(&raster, image.NewNRGBA(image.Rect(0, 0, 3, 2))); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, failure string
		body          []byte
	}{
		{name: "png", body: raster.Bytes()},
		{name: "invalid", body: []byte("invalid image"), failure: "invalid"},
		{name: "limit", body: bytes.Repeat([]byte("x"), MaxBytes+1), failure: "limit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := runRasterWorker(bytes.NewReader(tc.body), &output); err != nil {
				t.Fatal(err)
			}
			var response workerResponse
			if err := json.Unmarshal(output.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Failure != tc.failure {
				t.Fatalf("worker failure = %q, want %q", response.Failure, tc.failure)
			}
			if tc.failure == "" && (response.Raster.Width != 3 || response.Raster.Height != 2 || len(response.Raster.Variants) != 3) {
				t.Fatalf("worker raster = %+v", response.Raster)
			}
			if tc.failure != "" && len(response.Raster.Variants) != 0 {
				t.Fatalf("rejected raster has variants: %+v", response.Raster)
			}
		})
	}
	closed, err := os.CreateTemp(t.TempDir(), "closed-output")
	if err != nil {
		t.Fatal(err)
	}
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := runRasterWorker(bytes.NewReader(raster.Bytes()), closed); err == nil {
		t.Fatal("closed worker output accepted the response")
	}
}

func TestRasterCgroupConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name, available, enabled string
		want                     error
	}{
		{"all", "memory cpu pids", "memory cpu pids", nil},
		{"missing available", "memory cpu", "memory cpu pids", ErrUnavailable},
		{"missing enabled", "memory cpu pids", "memory pids", ErrUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := cgroupControllersAvailable([]byte(tc.available), []byte(tc.enabled)); got != tc.want {
				t.Fatalf("controller readiness = %v, want %v", got, tc.want)
			}
		})
	}
	group := t.TempDir()
	if err := configureRasterGroup(group); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"memory.max": "268435456", "memory.swap.max": "0", "pids.max": "32", "cpu.max": "100000 100000"} {
		got, err := os.ReadFile(filepath.Join(group, name))
		if err != nil || string(got) != want {
			t.Fatalf("%s = %q, %v; want %q", name, got, err, want)
		}
	}
	if err := configureRasterGroup(filepath.Join(group, "missing")); err != ErrUnavailable {
		t.Fatalf("missing group = %v", err)
	}
	blocked := t.TempDir()
	if err := os.Mkdir(filepath.Join(blocked, "memory.max"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := configureRasterGroup(blocked); err != ErrUnavailable {
		t.Fatalf("unwritable controller = %v", err)
	}
}

func TestBoundedWorkerOutput(t *testing.T) {
	var output boundedWorkerOutput
	data := bytes.Repeat([]byte("x"), 26<<20)
	if n, err := output.Write(data); err != nil || n != len(data) || output.Len() != len(data) {
		t.Fatalf("bounded response = %d, %v, %d", n, err, output.Len())
	}
	if n, err := output.Write([]byte("x")); n != 0 || err != ErrLimit || output.Len() != len(data) {
		t.Fatalf("oversized response = %d, %v, %d", n, err, output.Len())
	}
}

func TestRasterGroupLifecycle(t *testing.T) {
	parent := t.TempDir()
	group, cleanup, err := prepareRasterGroup(parent)
	if err != nil || !strings.HasPrefix(group, parent+"/metadata-raster-") || cleanup == nil {
		t.Fatalf("raster group = %q, %v", group, err)
	}
	for name, want := range map[string]string{"memory.max": "268435456", "memory.swap.max": "0", "pids.max": "32", "cpu.max": "100000 100000"} {
		body, err := os.ReadFile(filepath.Join(group, name))
		if err != nil || string(body) != want {
			t.Fatalf("%s = %q, %v", name, body, err)
		}
	}
	pid := os.Getpid()
	if err := joinRasterGroup(group, pid); err != nil {
		t.Fatalf("worker membership: %v", err)
	}
	membership, err := os.ReadFile(filepath.Join(group, "cgroup.procs"))
	if err != nil || string(membership) != strconv.Itoa(pid)+"\n" {
		t.Fatalf("worker membership = %q, %v", membership, err)
	}
	for _, name := range []string{"memory.max", "memory.swap.max", "pids.max", "cpu.max", "cgroup.procs"} {
		if err := os.Remove(filepath.Join(group, name)); err != nil {
			t.Fatal(err)
		}
	}
	cleanup()
	if _, err := os.Stat(group); !os.IsNotExist(err) {
		t.Fatalf("raster group remains: %v", err)
	}
	if _, _, err := prepareRasterGroup(filepath.Join(parent, "missing")); err != ErrUnavailable {
		t.Fatalf("missing group parent = %v", err)
	}
	if err := joinRasterGroup(filepath.Join(parent, "missing"), pid); err != ErrUnavailable {
		t.Fatalf("missing group membership = %v", err)
	}
}
