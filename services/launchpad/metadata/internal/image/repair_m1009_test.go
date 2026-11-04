package image

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func repairM1009PNG(t *testing.T, width, height int, noise bool) []byte {
	t.Helper()
	im := image.NewNRGBA(image.Rect(0, 0, width, height))
	if noise {
		// A committed deterministic fixture producer, encoded by the real PNG codec.
		state := uint32(1)
		for i := range im.Pix {
			state ^= state << 13
			state ^= state >> 17
			state ^= state << 5
			im.Pix[i] = byte(state)
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, im); err != nil {
		t.Fatal(err)
	}
	if b.Len() > MaxBytes {
		t.Fatalf("fixture exceeds upload admission: %d", b.Len())
	}
	return b.Bytes()
}

func repairM1009AssertRaster(t *testing.T, out Raster, width, height int) {
	t.Helper()
	if out.MIME != "image/png" || out.Width != width || out.Height != height || len(out.Variants) != 3 {
		t.Fatalf("unexpected raster: MIME=%s size=%dx%d variants=%d", out.MIME, out.Width, out.Height, len(out.Variants))
	}
	for _, size := range []int{256, 512, 1024} {
		b := out.Variants[size]
		if len(b) == 0 || len(b) > MaxBytes {
			t.Fatalf("variant %d has %d bytes", size, len(b))
		}
		im, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		w, h := width, height
		if w > size || h > size {
			if w >= h {
				h = max(1, h*size/w)
				w = size
			} else {
				w = max(1, w*size/h)
				h = size
			}
		}
		if im.Bounds().Dx() != w || im.Bounds().Dy() != h {
			t.Fatalf("variant %d: got %v, want %dx%d", size, im.Bounds(), w, h)
		}
	}
}

func TestRepairM_1009_RealCodecRoundTrip(t *testing.T) {
	worker := InProcessWorker{}
	if err := worker.Ready(); err != nil {
		t.Fatal(err)
	}
	pngBody := repairM1009PNG(t, 1536, 32, true)
	im, err := png.Decode(bytes.NewReader(pngBody))
	if err != nil {
		t.Fatal(err)
	}
	var jpegBody bytes.Buffer
	if err := jpeg.Encode(&jpegBody, im, nil); err != nil {
		t.Fatal(err)
	}
	// The package's recorded static WebP, also used by the production codec test.
	webpBody, err := base64.StdEncoding.DecodeString("UklGRrIBAABXRUJQVlA4TKUBAAAvSsAYAA8w//M///MfeJAkbXvaSG7m8Q3GfYSBJekwQztm/IcZlgwnmWImn2BK7aFmBtnVir6q//8VOkFE/xm4baTIu8c48ArEo6+B3zFKYln3pqClSCKX0begFTAXFOLXHSyF8cCNcZEG4OywuA4KVVfJCiArU7GAgJI8+lJP/OKMT/fBAjevg1cYB7YVkFuWga2lyPi5I0HFy5YTpWIHg0RZpkniRVW9odHAKOwosWuOGdxIyn2OvaCDvhg/we6TwadPBPbqBV58MsLmMJ8yZnOWk8SRz4N+QoyPL+MnamzMvcE1rHNEr91F9GKZPVUcS9w7PhhH36suB9qPeYb/oLk6cuTiJ0wOK3m5h1cKjW6EVZCYMK7dxcKCBdgP9HkKr9gkAO2P8GKZGWVdIAatQa+1IDpt6qyorVwdy01xdW8Jkfk6xjEXmVQQ+HQdFr6OKhIN34dXWq0+0qr6EJSCeeVLH9+gvGTLyqM65PQ44ihzlTXxQKjKbAvshXgir7Lil9w4L2bvMycmjQcqXaMCO6BlY28i+FOLzbfI1vEqxAhotocAAA==")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		body []byte
	}{{"png", pngBody}, {"jpeg", jpegBody.Bytes()}, {"webp", webpBody}} {
		t.Run(tc.name, func(t *testing.T) {
			config, _, err := image.DecodeConfig(bytes.NewReader(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			out, err := worker.Decode(context.Background(), tc.body)
			if err != nil {
				t.Fatal(err)
			}
			repairM1009AssertRaster(t, out, config.Width, config.Height)
			if tc.name == "png" {
				variant, err := png.Decode(bytes.NewReader(out.Variants[256]))
				if err != nil {
					t.Fatal(err)
				}
				want := color.NRGBAModel.Convert(im.At(6, 0))
				if got := color.NRGBAModel.Convert(variant.At(1, 0)); got != want {
					t.Fatalf("resized pixel = %v, want %v", got, want)
				}
			}
		})
	}
}

type repairM1009Job struct {
	done    chan struct{}
	cancel  context.CancelFunc
	out     Raster
	err     error
	pid     int
	started time.Time
}

func repairM1009Start(t *testing.T, ctx context.Context, body []byte) *repairM1009Job {
	t.Helper()
	ctx, cancel := context.WithCancel(ctx)
	job := &repairM1009Job{done: make(chan struct{}), cancel: cancel, started: time.Now()}
	go func() { job.out, job.err = (InProcessWorker{}).Decode(ctx, body); close(job.done) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-job.done:
		case <-time.After(time.Second):
			if job.pid > 0 {
				_ = syscall.Kill(job.pid, syscall.SIGKILL)
			}
			t.Error("canceled worker did not return and reap its child within one second")
		}
	})
	return job
}

// Suspend the actual production codec participant, rather than substituting a
// slow decoder or encoder. SIGSTOP stalls every instruction in either codec;
// the parent must still kill and reap it at the advertised wall deadline.
func repairM1009Suspend(t *testing.T, job *repairM1009Job, exclude map[int]bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-job.done:
			t.Fatalf("codec returned before suspension: %v", job.err)
		default:
		}
		tasks, err := os.ReadDir("/proc/self/task")
		if err != nil {
			t.Fatal(err)
		}
		for _, task := range tasks {
			raw, err := os.ReadFile(filepath.Join("/proc/self/task", task.Name(), "children"))
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, field := range strings.Fields(string(raw)) {
				pid, err := strconv.Atoi(field)
				if err != nil {
					t.Fatal(err)
				}
				if exclude[pid] {
					continue
				}
				args, err := os.ReadFile(filepath.Join("/proc", field, "cmdline"))
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.HasSuffix(args, append([]byte(workerArgument), 0)) {
					continue
				}
				if err := syscall.Kill(pid, syscall.SIGSTOP); err != nil {
					t.Fatal(err)
				}
				job.pid = pid
				return
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("the real isolated codec participant was not started")
}

func repairM1009Wait(t *testing.T, job *repairM1009Job, want error, bound time.Duration) {
	t.Helper()
	select {
	case <-job.done:
	case <-time.After(bound):
		t.Fatal("codec exceeded its wall bound")
	}
	if !errors.Is(job.err, want) || job.out.MIME != "" || len(job.out.Variants) != 0 {
		t.Fatalf("deadline published a result or wrong error: MIME=%q variants=%d err=%v, want %v", job.out.MIME, len(job.out.Variants), job.err, want)
	}
	if job.pid == 0 {
		t.Fatal("no codec child was exercised")
	}
	if err := syscall.Kill(job.pid, 0); err != syscall.ESRCH {
		t.Fatalf("codec child %d not reaped: %v", job.pid, err)
	}
}

func TestRepairM_1009_DecodeAndEncodeWallDeadline(t *testing.T) {
	for _, tc := range []struct {
		name          string
		width, height int
		noise         bool
	}{
		{"decode_heavy", 5000, 4000, false},
		{"encode_heavy", 1024, 1024, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := repairM1009PNG(t, tc.width, tc.height, tc.noise)
			job := repairM1009Start(t, context.Background(), body)
			repairM1009Suspend(t, job, nil)
			repairM1009Wait(t, job, ErrLimit, 6*time.Second)
			elapsed := time.Since(job.started)
			// The timer is five seconds; allow only scheduling/reaping overhead.
			if elapsed < 4900*time.Millisecond || elapsed > 5750*time.Millisecond {
				t.Fatalf("five-second wall deadline took %s", elapsed)
			}
			out, err := (InProcessWorker{}).Decode(context.Background(), repairM1009PNG(t, 16, 12, false))
			if err != nil {
				t.Fatal("worker slot was not recovered:", err)
			}
			repairM1009AssertRaster(t, out, 16, 12)
		})
	}
}

func TestRepairM_1009_ParentCancellationAndDeadline(t *testing.T) {
	body := repairM1009PNG(t, 5000, 4000, false)
	t.Run("cancel", func(t *testing.T) {
		job := repairM1009Start(t, context.Background(), body)
		repairM1009Suspend(t, job, nil)
		canceled := time.Now()
		job.cancel()
		repairM1009Wait(t, job, context.Canceled, time.Second)
		if time.Since(canceled) > time.Second {
			t.Fatal("parent cancellation did not interrupt the codec")
		}
	})
	t.Run("deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		defer cancel()
		job := repairM1009Start(t, ctx, body)
		repairM1009Suspend(t, job, nil)
		repairM1009Wait(t, job, context.DeadlineExceeded, time.Second)
		if time.Since(job.started) > time.Second {
			t.Fatal("parent deadline was replaced by the worker's five-second deadline")
		}
	})
}

func TestRepairM_1009_BoundedAdmission(t *testing.T) {
	body := repairM1009PNG(t, 5000, 4000, false)
	first := repairM1009Start(t, context.Background(), body)
	repairM1009Suspend(t, first, nil)
	second := repairM1009Start(t, context.Background(), body)
	repairM1009Suspend(t, second, map[int]bool{first.pid: true})
	start := time.Now()
	out, err := (InProcessWorker{}).Decode(context.Background(), body)
	if err != ErrLimit || len(out.Variants) != 0 || time.Since(start) > 250*time.Millisecond {
		t.Fatalf("saturated admission: err=%v elapsed=%s", err, time.Since(start))
	}
	first.cancel()
	second.cancel()
	repairM1009Wait(t, first, context.Canceled, time.Second)
	repairM1009Wait(t, second, context.Canceled, time.Second)
	out, err = (InProcessWorker{}).Decode(context.Background(), repairM1009PNG(t, 16, 12, false))
	if err != nil {
		t.Fatal("admission did not recover after cancellation:", err)
	}
	repairM1009AssertRaster(t, out, 16, 12)
}

func TestRepairM_1009_AdmissionAndValidation(t *testing.T) {
	worker := InProcessWorker{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := worker.Decode(ctx, nil); err != context.Canceled {
		t.Fatal("pre-canceled request:", err)
	}
	if _, err := worker.Decode(context.Background(), make([]byte, MaxBytes+1)); err != ErrLimit {
		t.Fatal("oversized input:", err)
	}
	if _, err := worker.Decode(context.Background(), []byte("not an image")); err != ErrInvalid {
		t.Fatal("invalid image:", err)
	}
	bomb := repairM1009PNG(t, 1, 1, false)
	binary.BigEndian.PutUint32(bomb[16:20], 8193)
	binary.BigEndian.PutUint32(bomb[29:33], crc32.ChecksumIEEE(bomb[12:29]))
	if _, err := worker.Decode(context.Background(), bomb); err != ErrLimit {
		t.Fatal("dimension bomb:", err)
	}
	animation := append([]byte("RIFF"), make([]byte, 4)...)
	animation = append(animation, []byte("WEBPANIM")...)
	animation = append(animation, make([]byte, 4)...)
	binary.LittleEndian.PutUint32(animation[4:8], uint32(len(animation)-8))
	if _, err := worker.Decode(context.Background(), animation); err != ErrInvalid {
		t.Fatal("animated image:", err)
	}
}
