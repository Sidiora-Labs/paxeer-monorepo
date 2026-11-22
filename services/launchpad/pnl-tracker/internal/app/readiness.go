package app

import (
	"context"

	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	"golang.org/x/sync/singleflight"
)

// serviceReadiness shares in-flight verification across probes. A disconnected
// caller stops waiting without canceling verification for the next caller.
func serviceReadiness(parent context.Context, check sharedhttp.Check) sharedhttp.Check {
	var pending singleflight.Group
	return func(ctx context.Context) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		result := pending.DoChan("database", func() (any, error) {
			bounded, cancel := context.WithTimeout(parent, sharedhttp.ReadinessTimeout)
			defer cancel()
			return nil, check(bounded)
		})
		select {
		case <-ctx.Done():
			return ctx.Err()
		case completed := <-result:
			return completed.Err
		}
	}
}
