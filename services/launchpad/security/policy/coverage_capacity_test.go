package policy

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestFrozenCapacityEvidenceAcceptsCompleteSamples(t *testing.T) {
	profile := FrozenAttackProfiles()["steady"]
	start := time.Now().UTC().Truncate(time.Second)
	evidence := AdversarialEvidence{
		ArtifactDigest:        strings.Repeat("a", 64),
		ConfigurationRevision: "v1",
		Started:               start,
		Ended:                 start.Add(1200 * time.Millisecond),
		RedisLossStarted:      start.Add(600 * time.Millisecond),
		RedisLossEnded:        start.Add(700 * time.Millisecond),
		Replicas:              2,
		GeneratorRPS:          profile.API,
		CPUCores:              2,
		MemoryBytes:           1 << 30,
	}
	for n := 0; n < 11001; n++ {
		class := classPublicRead
		if n < 2300 {
			class = classAccountWrite
		} else if n < 4600 {
			class = classQuote
		}
		evidence.Samples = append(evidence.Samples, AdversarialSample{Class: class, RequestID: fmt.Sprintf("valid-%d", n), Observed: start.Add(100 * time.Millisecond), Valid: true, Admitted: true, Status: 200, Duration: time.Millisecond, Authorization: time.Microsecond, Limiter: time.Microsecond})
	}
	evidence.Samples = append(evidence.Samples, AdversarialSample{Class: classPublicRead, RequestID: "rejected-attack", Observed: start.Add(100 * time.Millisecond), Status: 403, Duration: time.Millisecond})
	for n := 0; n < 201; n++ {
		evidence.Samples = append(evidence.Samples, AdversarialSample{Class: classAccountWrite, RequestID: fmt.Sprintf("outage-%d", n), Observed: start.Add(650 * time.Millisecond), RedisUnavailable: true, Valid: true, Status: 503, Duration: time.Millisecond})
	}
	evidence.Samples = append(evidence.Samples, AdversarialSample{Class: classPublicRead, RequestID: "cached-read", Observed: start.Add(650 * time.Millisecond), RedisUnavailable: true, CachedRead: true, Valid: true, Admitted: true, Status: 200, Duration: time.Millisecond})
	if err := ValidateAdversarialEvidence(profile, evidence); err != nil {
		t.Fatalf("complete frozen workload: %v", err)
	}
	duplicate := evidence
	duplicate.Samples = append([]AdversarialSample(nil), evidence.Samples...)
	duplicate.Samples[1].RequestID = duplicate.Samples[0].RequestID
	if err := ValidateAdversarialEvidence(profile, duplicate); err == nil {
		t.Fatal("duplicate request accepted")
	}
	wrongOutage := evidence
	wrongOutage.Samples = append([]AdversarialSample(nil), evidence.Samples...)
	wrongOutage.Samples[len(wrongOutage.Samples)-1].RedisUnavailable = false
	if err := ValidateAdversarialEvidence(profile, wrongOutage); err == nil {
		t.Fatal("mismatched outage observation accepted")
	}
	insufficient := evidence
	insufficient.Samples = evidence.Samples[:len(evidence.Samples)-202]
	if err := ValidateAdversarialEvidence(profile, insufficient); err == nil {
		t.Fatal("missing outage samples accepted")
	}
	for _, variation := range []struct {
		name   string
		change func(*AdversarialEvidence)
	}{
		{"sample before run", func(e *AdversarialEvidence) { e.Samples[0].Observed = e.Started.Add(-time.Nanosecond) }},
		{"unknown class", func(e *AdversarialEvidence) { e.Samples[0].Class = "unknown" }},
		{"unreported outage", func(e *AdversarialEvidence) { e.Samples[len(e.Samples)-1].RedisUnavailable = false }},
		{"invalid cached read", func(e *AdversarialEvidence) { e.Samples[len(e.Samples)-1].CachedRead = false }},
		{"slow read", func(e *AdversarialEvidence) {
			for i := 4600; i < len(e.Samples)-203; i++ {
				e.Samples[i].Duration = 500 * time.Millisecond
			}
		}},
		{"slow authorization", func(e *AdversarialEvidence) {
			for i := 0; i < len(e.Samples)-203; i++ {
				e.Samples[i].Duration = 20 * time.Millisecond
				e.Samples[i].Authorization = 20 * time.Millisecond
			}
		}},
	} {
		t.Run(variation.name, func(t *testing.T) {
			candidate := evidence
			candidate.Samples = append([]AdversarialSample(nil), evidence.Samples...)
			variation.change(&candidate)
			if err := ValidateAdversarialEvidence(profile, candidate); err == nil {
				t.Fatal("invalid capacity evidence accepted")
			}
		})
	}
}
