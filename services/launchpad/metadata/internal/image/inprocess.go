package image

import (
	"context"
	"os"
	"time"
)

// InProcessWorker is the legacy development-worker entry point. It now
// re-executes this binary using ProcessWorker's rlimit isolation: synchronous
// codecs cannot be safely interrupted inside the serving process. Admission
// permits at most two live workers across all instances. The five-second
// deadline includes admission, decoding, encoding and result transport.
type InProcessWorker struct{}

const developmentRasterWall = 5 * time.Second

var developmentRasterSlots = make(chan struct{}, 2)

func developmentRasterWorker() (ProcessWorker, error) {
	executable, err := os.Executable()
	if err != nil {
		return ProcessWorker{}, ErrUnavailable
	}
	return ProcessWorker{Executable: executable, Isolation: RlimitIsolation}, nil
}

func (InProcessWorker) Ready() error {
	worker, err := developmentRasterWorker()
	if err != nil {
		return err
	}
	return worker.Ready()
}

func (InProcessWorker) Decode(ctx context.Context, body []byte) (Raster, error) {
	if len(body) > MaxBytes {
		return Raster{}, ErrLimit
	}
	bounded, cancel := context.WithTimeout(ctx, developmentRasterWall)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return Raster{}, err
	}
	select {
	case developmentRasterSlots <- struct{}{}:
		defer func() { <-developmentRasterSlots }()
	default:
		return Raster{}, ErrLimit
	}
	worker, err := developmentRasterWorker()
	if err != nil {
		return Raster{}, err
	}
	result, err := worker.Decode(bounded, body)
	if ctx.Err() != nil {
		return Raster{}, ctx.Err()
	}
	if bounded.Err() != nil {
		return Raster{}, ErrLimit
	}
	return result, err
}
