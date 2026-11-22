package processor

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

func (p *Processor) recordStandingHeight(previous, height int64) {
	if height > previous {
		p.standingHeightPasses.Store(0)
	} else {
		p.standingHeightPasses.Add(1)
	}
}

func (p *Processor) StandingHeightPasses() uint64 {
	return p.standingHeightPasses.Load()
}

func (p *Processor) WritePrometheus(w io.Writer) error {
	_, err := fmt.Fprintf(w, "# TYPE indexer_processor_standing_height_passes gauge\nindexer_processor_standing_height_passes %d\n", p.StandingHeightPasses())
	return err
}

func (p *Processor) MetricsHandler(metrics interface{ WritePrometheus(io.Writer) error }) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var output bytes.Buffer
		if err := metrics.WritePrometheus(&output); err != nil {
			http.Error(w, "telemetry unavailable", http.StatusServiceUnavailable)
			return
		}
		if err := p.WritePrometheus(&output); err != nil {
			http.Error(w, "telemetry unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = w.Write(output.Bytes())
	})
}
