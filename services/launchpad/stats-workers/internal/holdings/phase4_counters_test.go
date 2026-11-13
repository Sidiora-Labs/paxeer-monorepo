package holdings

import (
	"bytes"
	"testing"
	"time"
)

func TestPhase4LocalGeneratorCountersAndBoundedOutput(t *testing.T) {
	first, err := phase4Counters(Phase4Plan{GeneratorNIC: "lo"})
	if err != nil || first.CPU <= first.Idle || first.At.IsZero() {
		t.Fatalf("generator counters: %+v %v", first, err)
	}
	second, err := phase4Counters(Phase4Plan{GeneratorNIC: "lo"})
	if err != nil || second.CPU < first.CPU || second.Idle < first.Idle || second.RX < first.RX || second.TX < first.TX || second.At.Before(first.At) {
		t.Fatalf("counter progression: %+v %+v %v", first, second, err)
	}
	if _, err := phase4Counters(Phase4Plan{GeneratorNIC: "missing-interface"}); err == nil {
		t.Fatal("unknown interface accepted")
	}
	var output bytes.Buffer
	writer := &phase4BoundedWriter{target: &output, remaining: 4}
	if n, err := writer.Write([]byte("abc")); err != nil || n != 3 || output.String() != "abc" || writer.remaining != 1 {
		t.Fatalf("bounded write: %d %v %q %d", n, err, output.String(), writer.remaining)
	}
	if n, err := writer.Write([]byte("de")); err == nil || n != 0 || output.String() != "abc" {
		t.Fatalf("over budget: %d %v %q", n, err, output.String())
	}
	before := phase4GeneratorCounters{At: time.Now(), CPU: 100, Idle: 25, RX: 10, TX: 10}
	after := phase4GeneratorCounters{At: before.At.Add(time.Second), CPU: 200, Idle: 75, RX: 20, TX: 30}
	cpu, nic, err := phase4Saturation(before, after, 1000000000)
	if err != nil || cpu != "1/2" || nic != "1/6250000" {
		t.Fatalf("saturation: %q %q %v", cpu, nic, err)
	}
	if _, _, err := phase4Saturation(after, before, 1000000000); err == nil {
		t.Fatal("reversed interval accepted")
	}
}
