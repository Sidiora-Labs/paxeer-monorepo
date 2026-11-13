package auth

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/cleanup"
)

type IdentityRuntimeEvidence struct {
	Binding  IdentityMeasurementBinding `json:"binding"`
	Snapshot IdentityRuntimeMeasurement `json:"snapshot"`
}

func (c IdentityMeasurementConfig) captureRuntimeMeasurement() (IdentityRuntimeMeasurement, error) {
	if c.Runtime != nil {
		return c.Runtime.CaptureRuntimeMeasurement()
	}
	if c.RuntimeArchive != nil {
		return c.RuntimeArchive.CaptureRuntimeMeasurement()
	}
	return IdentityRuntimeMeasurement{}, errors.New("identity measurement: runtime provider missing")
}

type IdentityRuntimeArchive struct {
	Directory string
	Binding   IdentityMeasurementBinding
}

func readIdentityEvidence(path string, dest any) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > 32*1024*1024 {
		return errors.New("identity measurement: bounded regular evidence required")
	}
	file, err := os.Open(path) //nolint:gosec // The evidence file is regular and bounded before decoding.
	if err != nil {
		return err
	}
	defer cleanup.Close(file)
	decoder := json.NewDecoder(io.LimitReader(file, 32*1024*1024+1))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(dest); err != nil {
		return err
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return errors.New("identity measurement: trailing evidence")
	}
	return nil
}

// WriteRuntimeMeasurementEvidence is called in the owning process, never in the load generator.
func (s *Identity) WriteRuntimeMeasurementEvidence(directory, sourceRoot string, binding IdentityMeasurementBinding) error {
	_, candidate, err := identityMeasurementSources(sourceRoot)
	if err != nil || candidate != binding.CandidateSHA256 {
		return errors.New("identity measurement: runtime candidate mismatch")
	}
	snapshot, err := s.CaptureRuntimeMeasurement()
	if err != nil {
		return err
	}
	return writeIdentityImmutableJSON(filepath.Join(directory, fmt.Sprintf("runtime-%020d.json", snapshot.CapturedAt.UnixNano())), IdentityRuntimeEvidence{binding, snapshot})
}
func (p IdentityRuntimeArchive) CaptureRuntimeMeasurement() (IdentityRuntimeMeasurement, error) {
	entries, err := os.ReadDir(p.Directory)
	if err != nil || len(entries) > 86410 {
		return IdentityRuntimeMeasurement{}, errors.New("identity measurement: runtime archive unavailable or saturated")
	}
	latest := ""
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "runtime-") && strings.HasSuffix(entry.Name(), ".json") && entry.Name() > latest {
			latest = entry.Name()
		}
	}
	var evidence IdentityRuntimeEvidence
	if latest == "" || readIdentityEvidence(filepath.Join(p.Directory, latest), &evidence) != nil || evidence.Binding != p.Binding || evidence.Snapshot.CapturedAt.IsZero() || evidence.Snapshot.ProcessID < 1 {
		return IdentityRuntimeMeasurement{}, errors.New("identity measurement: runtime archive binding invalid")
	}
	return evidence.Snapshot, nil
}

type IdentityFaultReceipt struct {
	Binding   IdentityMeasurementBinding `json:"binding"`
	Kind      string                     `json:"kind"`
	StartedAt time.Time                  `json:"startedAt"`
	EndedAt   time.Time                  `json:"endedAt"`
	Recovered bool                       `json:"recovered"`
}
type IdentityRecoveryEvidence struct {
	Before             IdentityDurabilityCheckpoint `json:"before"`
	After              IdentityDurabilityCheckpoint `json:"after"`
	FaultReceiptSHA256 string                       `json:"faultReceiptSha256"`
	FaultStarted       time.Time                    `json:"faultStarted"`
	FaultEnded         time.Time                    `json:"faultEnded"`
	DurabilityPassed   bool                         `json:"durabilityPassed"`
	AZLossAcceptance   bool                         `json:"azLossAcceptance"`
	Failure            string                       `json:"failure"`
}
type IdentitySharedAcceptance struct {
	Binding     IdentityMeasurementBinding `json:"binding"`
	Gate        string                     `json:"gate"`
	CompletedAt time.Time                  `json:"completedAt"`
	Assertions  map[string]bool            `json:"assertions"`
}
type IdentityFinalEvidence struct {
	Binding      IdentityMeasurementBinding    `json:"binding"`
	Recovery     IdentityRecoveryEvidence      `json:"recovery"`
	FaultReceipt json.RawMessage               `json:"faultReceipt"`
	Replay       IdentityReplayMeasurement     `json:"replay"`
	Revocation   IdentityRevocationMeasurement `json:"revocation"`
	Shared       IdentitySharedAcceptance      `json:"shared"`
}

func validMeasurementDigest(value string) bool {
	raw, err := hex.DecodeString(value)
	return err == nil && len(raw) == 32 && strings.ToLower(value) == value
}
func validIdentityFaultReceipt(raw []byte, binding IdentityMeasurementBinding, start, end time.Time) bool {
	var receipt IdentityFaultReceipt
	return strictJSON(raw, &receipt) == nil && receipt.Binding == binding && receipt.Kind == "az_loss" && receipt.Recovered && receipt.StartedAt.Equal(start) && receipt.EndedAt.Equal(end) && end.After(start)
}
func applyIdentityFinalEvidence(summary *IdentityMeasurementSummary, evidence IdentityFinalEvidence) {
	if evidence.Binding != summary.Binding || summary.Checkpoint == nil {
		summary.Failures = append(summary.Failures, "final_evidence_binding_mismatch")
		return
	}
	recovery := evidence.Recovery
	summary.Assertions["AZ_loss_recovery"] = recovery.Before.Binding == summary.Binding && recovery.Before.SHA256 == summary.Checkpoint.SHA256 && recovery.Before.CapturedAt.Equal(summary.Checkpoint.CapturedAt) && CompareIdentityRecovery(recovery.Before, recovery.After) == nil && recovery.FaultStarted.After(recovery.Before.CapturedAt) && recovery.FaultEnded.After(recovery.FaultStarted) && recovery.After.CapturedAt.After(recovery.FaultEnded) && measurementHash(evidence.FaultReceipt) == recovery.FaultReceiptSHA256 && validIdentityFaultReceipt(evidence.FaultReceipt, summary.Binding, recovery.FaultStarted, recovery.FaultEnded) && recovery.DurabilityPassed && recovery.AZLossAcceptance && recovery.Failure == ""
	replay := evidence.Replay
	summary.Assertions["event_replay_integrity"] = replay.Binding == summary.Binding && replay.StartedAt.After(recovery.FaultEnded) && replay.CompletedAt.After(replay.StartedAt) && replay.Fetched >= replay.Committed && replay.Committed > 0 && replay.Committed == replay.Expected && replay.InboxCount > 0 && replay.InboxCount <= int64(replay.Committed) && validMeasurementDigest(replay.InboxSHA256) && replay.Failure == ""
	revocation := evidence.Revocation
	valid := revocation.Binding == summary.Binding && revocation.Passed && !revocation.LogoutSentAt.Before(summary.StartedAt) && revocation.AcknowledgedAt.After(revocation.LogoutSentAt) && len(revocation.Observations) >= 2
	lags := []int64{}
	requests := map[string]bool{}
	for _, observation := range revocation.Observations {
		latest := observation.HTTPDeniedAt
		if observation.SocketClosedAt.After(latest) {
			latest = observation.SocketClosedAt
		}
		lag := latest.Sub(revocation.LogoutSentAt).Nanoseconds()
		valid = valid && validUUID(observation.RequestID) && !requests[observation.RequestID] && observation.Failure == "" && !observation.HTTPDeniedAt.Before(revocation.LogoutSentAt) && !observation.SocketClosedAt.Before(revocation.LogoutSentAt) && lag == observation.UpperBoundLagNS && lag > 0 && lag <= int64(5*time.Second)
		requests[observation.RequestID] = true
		lags = append(lags, lag)
	}
	summary.Assertions["private_socket_revocation_p99"] = valid && identityPercentile(lags, 99) == revocation.P99UpperBoundNS && revocation.P99UpperBoundNS <= int64(5*time.Second)
	shared := evidence.Shared
	sharedOK := shared.Binding == summary.Binding && shared.Gate == summary.FinalSharedGate && shared.CompletedAt.After(replay.CompletedAt) && shared.CompletedAt.After(recovery.After.CapturedAt) && len(shared.Assertions) > 0
	for name, passed := range shared.Assertions {
		sharedOK = sharedOK && name != "" && passed
	}
	for name := range summary.Assertions {
		if name != "final_shared_acceptance" {
			sharedOK = sharedOK && shared.Assertions[name]
		}
	}
	summary.Assertions["final_shared_acceptance"] = sharedOK
}

// AwaitIdentityFinalEvidence hands the durable checkpoint to the serial coordinator,
// then waits for its actual fault, replay, socket and shared-gate evidence.
func AwaitIdentityFinalEvidence(path, checkpointPath string) func(context.Context, IdentityDurabilityCheckpoint) (IdentityFinalEvidence, error) {
	return func(ctx context.Context, checkpoint IdentityDurabilityCheckpoint) (IdentityFinalEvidence, error) {
		if _, ok := ctx.Deadline(); !ok {
			return IdentityFinalEvidence{}, errors.New("identity measurement: serial evidence deadline required")
		}
		if err := writeIdentityImmutableJSON(checkpointPath, checkpoint); err != nil {
			return IdentityFinalEvidence{}, err
		}
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			if _, err := os.Lstat(path); err == nil {
				var evidence IdentityFinalEvidence
				err = readIdentityEvidence(path, &evidence)
				return evidence, err
			} else if !os.IsNotExist(err) {
				return IdentityFinalEvidence{}, err
			}
			select {
			case <-ctx.Done():
				return IdentityFinalEvidence{}, ctx.Err()
			case <-ticker.C:
			}
		}
	}
}
