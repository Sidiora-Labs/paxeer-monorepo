package store

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker"
)

// CanonicalRankingEvidence is an owner observation of pool state at the exact
// frozen block, including independently qualified self-trade volume. Absence
// of a pool or field is unknown evidence, never an unpaused observation.
type CanonicalRankingEvidence struct {
	Revision     string                           `json:"revision"`
	CutoffHeight uint64                           `json:"cutoffHeight"`
	CutoffTime   int64                            `json:"cutoffTime"`
	BlockHash    string                           `json:"blockHash"`
	Pools        map[string]CanonicalPoolEvidence `json:"pools"`
}
type CanonicalPoolEvidence struct {
	Paused        *bool                `json:"paused"`
	PauseEvidence string               `json:"pauseEvidence"`
	SelfVolume    *CanonicalSelfVolume `json:"selfVolume"`
}
type CanonicalSelfVolume struct {
	Raw         string `json:"raw"`
	WindowStart int64  `json:"windowStart"`
	WindowEnd   int64  `json:"windowEnd"`
	Evidence    string `json:"evidence"`
}

func evidenceDigest(value string) bool {
	raw, err := hex.DecodeString(value)
	return err == nil && len(raw) == 32 && hex.EncodeToString(raw) == value
}
func (v CanonicalSelfVolume) validate(cutoff int64) error {
	if v.WindowStart != cutoff-86400 || v.WindowEnd != cutoff || !evidenceDigest(v.Evidence) {
		return ranker.ErrUnavailable
	}
	_, err := rawUSDLDecimal(v.Raw)
	return err
}

// HTTPCanonicalEvidence reads the source owner's cutoff-bound observations.
// An unavailable or mismatched response fails the freeze; no latest-state
// response may stand in for the requested canonical block.
type HTTPCanonicalEvidence struct {
	endpoint string
	client   *http.Client
}

func NewHTTPCanonicalEvidence(endpoint string, client *http.Client) (*HTTPCanonicalEvidence, error) {
	u, err := url.Parse(endpoint)
	if err != nil || client == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.Fragment != "" {
		return nil, ranker.ErrUnavailable
	}
	return &HTTPCanonicalEvidence{endpoint, client}, nil
}
func (r *HTTPCanonicalEvidence) Read(ctx context.Context, height uint64, cutoff int64, blockHash string) (CanonicalRankingEvidence, error) {
	var out CanonicalRankingEvidence
	if height == 0 || cutoff < 86400 {
		return out, ranker.ErrUnavailable
	}
	u, err := url.Parse(r.endpoint)
	if err != nil {
		return out, err
	}
	q := u.Query()
	q.Set("cutoffHeight", strconv.FormatUint(height, 10))
	q.Set("cutoffTime", strconv.FormatInt(cutoff, 10))
	q.Set("blockHash", blockHash)
	u.RawQuery = q.Encode()
	bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(bounded, http.MethodGet, u.String(), nil)
	if err != nil {
		return out, err
	}
	response, err := r.client.Do(req)
	if err != nil {
		return out, err
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			slog.Warn("canonical evidence response close failed", "error", closeErr)
		}
	}()
	if response.StatusCode != http.StatusOK {
		return out, ranker.ErrUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, (64<<20)+1))
	if err != nil {
		return out, err
	}
	if len(raw) > 64<<20 || json.Unmarshal(raw, &out) != nil || !evidenceDigest(out.Revision) || out.CutoffHeight != height || out.CutoffTime != cutoff || out.BlockHash != blockHash || out.Pools == nil || len(out.Pools) > 1000000 {
		return out, ranker.ErrUnavailable
	}
	for pool, evidence := range out.Pools {
		if len(pool) != 42 || pool[:2] != "0x" {
			return out, ranker.ErrUnavailable
		}
		if _, err = hex.DecodeString(pool[2:]); err != nil {
			return out, ranker.ErrUnavailable
		}
		if evidence.Paused != nil && !evidenceDigest(evidence.PauseEvidence) {
			return out, ranker.ErrUnavailable
		}
		if evidence.SelfVolume != nil {
			if err = evidence.SelfVolume.validate(cutoff); err != nil {
				return out, err
			}
		}
	}
	return out, nil
}
