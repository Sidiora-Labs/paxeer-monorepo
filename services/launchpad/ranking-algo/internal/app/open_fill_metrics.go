package app

import (
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/telemetry"
)

type openFillMetrics struct {
	reads *telemetry.Metrics
	all   atomic.Int64
	new   atomic.Int64
}

func (m *openFillMetrics) observe(newOnly bool, pools int) {
	if newOnly {
		m.new.Store(int64(pools))
	} else {
		m.all.Store(int64(pools))
	}
}

func (m *openFillMetrics) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.reads.ServeHTTP(w, r)
	if r.Method == http.MethodGet {
		_, _ = fmt.Fprintf(w, "# TYPE ranking_open_fill_pools gauge\nranking_open_fill_pools{run=%q} %d\nranking_open_fill_pools{run=%q} %d\n", "all", m.all.Load(), "new", m.new.Load())
	}
}
