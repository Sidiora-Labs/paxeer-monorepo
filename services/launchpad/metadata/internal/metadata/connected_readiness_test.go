package metadata

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	raster "github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/image"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	"github.com/jackc/pgx/v5"
)

type mediaReadinessWorker struct{ err error }

func (w mediaReadinessWorker) Ready() error { return w.err }
func (mediaReadinessWorker) Decode(context.Context, []byte) (raster.Raster, error) {
	return raster.Raster{}, nil
}

type mediaReadinessVerifier struct{}

func (mediaReadinessVerifier) AuthorizeInTx(context.Context, pgx.Tx, VerifiedAction) error {
	return nil
}

type mediaReadinessCanonical struct{}

func (mediaReadinessCanonical) ResolveTokenInTx(context.Context, pgx.Tx, string) (V2CanonicalIdentity, error) {
	return V2CanonicalIdentity{}, nil
}

func TestMediaReadinessIdentifiesRasterAndScanner(t *testing.T) {
	ctx := context.Background()
	base := ConnectedMedia{
		Metadata:   V2ContractHandlers{Store: &ProductionStore{Verifier: mediaReadinessVerifier{}}, Canonical: mediaReadinessCanonical{}},
		Moderation: DevelopmentVisibility{},
	}
	for _, tc := range []struct {
		name  string
		media ConnectedMedia
		part  string
	}{
		{name: "raster", media: base, part: MediaPartRaster},
		{name: "scanner", media: base, part: MediaPartScanner},
	} {
		t.Run(tc.name, func(t *testing.T) {
			media := tc.media
			if tc.part == MediaPartRaster {
				media.Worker = mediaReadinessWorker{err: raster.ErrUnavailable}
				media.Scanner = scannerReadinessOptions(t, true)
			} else {
				media.Worker = mediaReadinessWorker{}
				media.Scanner = scannerReadinessOptions(t, false)
			}
			err := media.Ready(ctx)
			var got *MediaReadinessError
			if !errors.As(err, &got) || got.Part != tc.part || got.Step != "" {
				t.Fatalf("readiness = %v, detail = %+v", err, got)
			}
			if !errors.Is(err, raster.ErrUnavailable) && tc.part == MediaPartRaster {
				t.Fatalf("raster readiness does not preserve sentinel: %v", err)
			}
		})
	}
}

func TestMediaReadinessIdentifiesObjectSteps(t *testing.T) {
	steps := []string{
		MediaObjectsStepConfiguration,
		MediaObjectsStepQuarantineACL,
		MediaObjectsStepQuarantinePolicy,
		MediaObjectsStepQuarantineAnonymous,
		MediaObjectsStepApprovedBucket,
	}
	for _, step := range steps {
		t.Run(step, func(t *testing.T) {
			media := ConnectedMedia{
				Metadata:   V2ContractHandlers{Store: &ProductionStore{Verifier: mediaReadinessVerifier{}}, Canonical: mediaReadinessCanonical{}},
				Moderation: DevelopmentVisibility{},
				Worker:     mediaReadinessWorker{},
				Scanner:    scannerReadinessOptions(t, true),
				Objects: S3AssetObjects{Readiness: func(context.Context) error {
					return &MediaReadinessError{Part: MediaPartObjects, Step: step, Err: raster.ErrUnavailable}
				}},
			}
			err := media.Ready(context.Background())
			var got *MediaReadinessError
			if !errors.As(err, &got) || got.Part != MediaPartObjects || got.Step != step {
				t.Fatalf("readiness = %v, detail = %+v", err, got)
			}
			if !errors.Is(err, raster.ErrUnavailable) {
				t.Fatalf("object readiness does not preserve sentinel: %v", err)
			}
		})
	}
}

func TestMediaReadinessErrorMatchesSentinelAndOmitsObjectDetails(t *testing.T) {
	secret := errors.New("bucket=private-bucket address=https://objects.example key=quarantine/private-key object-store=driver")
	err := mediaReadinessError(MediaPartObjects, MediaObjectsStepApprovedBucket, secret)
	var got *MediaReadinessError
	if !errors.As(err, &got) || !errors.Is(err, secret) {
		t.Fatalf("readiness wrapping = %v, detail = %+v", err, got)
	}
	message := err.Error()
	if !strings.Contains(message, MediaPartObjects) || !strings.Contains(message, MediaObjectsStepApprovedBucket) {
		t.Fatalf("readiness message = %q", message)
	}
	for _, value := range []string{"private-bucket", "https://objects.example", "private-key", "object-store", secret.Error()} {
		if strings.Contains(message, value) {
			t.Fatalf("readiness message contains %q: %q", value, message)
		}
	}
}

func scannerReadinessOptions(t *testing.T, healthy bool) sharedhttp.ScanOptions {
	t.Helper()
	if !healthy {
		return sharedhttp.ScanOptions{Host: "127.0.0.1", Port: 1}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		buffer := make([]byte, 128)
		_, _ = connection.Read(buffer)
		_, _ = connection.Write([]byte("ClamAV 1.2.3/17/2026-01-01\x00"))
	}()
	return sharedhttp.ScanOptions{Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port}
}
