package consumer_test

import (
	"context"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/app"
)

// Include the actual application entry point in the focused package gate so
// its completeness lifecycle wiring is compiled alongside the consumer proof.
var _ func(context.Context) error = app.Run
