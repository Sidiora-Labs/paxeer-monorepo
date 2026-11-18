package storage

import (
	"strings"
	"testing"
)

func TestEnvelopePlaybackProgressReports(t *testing.T) {
	const boundMicros int64 = 1000000
	for _, test := range []struct {
		name        string
		report      string
		frames      int64
		mediaMicros int64
		ended       bool
		complete    bool
	}{
		{
			name: "last frame end reaches bound",
			report: `frame=15
fps=15.00
stream_0_0_q=-0.0
bitrate=N/A
total_size=N/A
out_time_us=1000000
out_time_ms=1000000
out_time=00:00:01.000000
dup_frames=0
drop_frames=0
speed=1.00x
progress=end
`,
			frames: 15, mediaMicros: boundMicros, ended: true, complete: true,
		},
		{
			name: "last frame start reaches bound at fifteen fps",
			report: `frame=15
fps=15.00
stream_0_0_q=-0.0
bitrate=N/A
total_size=N/A
out_time_us=933333
out_time_ms=933333
out_time=00:00:00.933333
dup_frames=0
drop_frames=0
speed=1.00x
progress=end
`,
			frames: 15, mediaMicros: boundMicros, ended: true, complete: true,
		},
		{
			name: "frame count supplies frame duration",
			report: `frame=1
fps=0.00
out_time_us=0
progress=continue
frame=15
fps=0.00
out_time_us=933333
progress=end
`,
			frames: 15, mediaMicros: boundMicros, ended: true, complete: true,
		},
		{
			name: "half duration stays incomplete",
			report: `frame=8
fps=15.00
out_time_us=500000
out_time=00:00:00.500000
progress=end
`,
			frames: 8, mediaMicros: 500000, ended: true,
		},
		{
			name: "derived duration keeps early end incomplete",
			report: `frame=8
fps=0.00
out_time_us=466667
progress=end
`,
			frames: 8, mediaMicros: 466667, ended: true,
		},
		{
			name: "missing end stays incomplete",
			report: `frame=15
fps=15.00
out_time_us=933333
progress=continue
`,
			frames: 15, mediaMicros: 933333,
		},
		{
			name: "no shown frame stays incomplete",
			report: `frame=0
fps=15.00
out_time_us=1000000
progress=end
`,
			mediaMicros: boundMicros, ended: true,
		},
		{
			name: "more than one frame short stays incomplete",
			report: `frame=15
fps=15.00
out_time_us=933332
progress=end
`,
			frames: 15, mediaMicros: 933332, ended: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			frames, mediaMicros, ended, err := parseEnvelopePlaybackProgress(strings.NewReader(test.report), boundMicros)
			if err != nil {
				t.Fatal(err)
			}
			if frames != test.frames || mediaMicros != test.mediaMicros || ended != test.ended {
				t.Fatalf("progress frames=%d mediaMicros=%d ended=%t, want frames=%d mediaMicros=%d ended=%t", frames, mediaMicros, ended, test.frames, test.mediaMicros, test.ended)
			}
			if complete := ended && frames > 0 && mediaMicros >= boundMicros; complete != test.complete {
				t.Fatalf("playback complete=%t, want %t", complete, test.complete)
			}
		})
	}
}
