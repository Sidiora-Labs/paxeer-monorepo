package engine

import (
	"encoding/json"
	"sort"
	"strconv"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

type reconstructionProjection struct {
	Bars    []QualifiedCandle
	Candles []store.StagedCandle
	Traders []store.StagedTrader
	Digest  string
}

func buildReconstructionProjection(fills []accounting.FillV1, from, to int64, checkpoint SeriesCheckpoint, now time.Time) (reconstructionProjection, error) {
	if from < 0 || to <= from || len(fills) > 100000 {
		return reconstructionProjection{}, ErrInvalidFill
	}
	buckets := map[string]QualifiedCandle{}
	members := map[string]map[string]bool{}
	for _, fill := range fills {
		copied, err := QualifiedFill(fill)
		if err != nil {
			return reconstructionProjection{}, err
		}
		for _, interval := range ExactIntervals {
			start, err := BucketStart(copied.BlockTimestamp, interval)
			if err != nil {
				return reconstructionProjection{}, err
			}
			duration, _ := IntervalSeconds(interval)
			if start >= to || start+duration <= from {
				continue
			}
			key := interval + ":" + strconv.FormatInt(start, 10)
			var old *QualifiedCandle
			if previous, exists := buckets[key]; exists {
				p := previous
				old = &p
			}
			if members[key] == nil {
				members[key] = map[string]bool{}
			}
			newTrader := false
			if copied.Sender != "" {
				newTrader = !members[key][copied.Sender]
				members[key][copied.Sender] = true
			}
			next, err := FoldQualified(old, fill, interval, newTrader)
			if err != nil {
				return reconstructionProjection{}, err
			}
			buckets[key] = next
		}
	}
	actual := []QualifiedCandle{}
	stage := []store.StagedCandle{}
	traders := []store.StagedTrader{}
	keys := make([]string, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		bar := buckets[key]
		actual = append(actual, bar)
		state, err := DescribeSeries(bar, checkpoint, now)
		if err != nil {
			return reconstructionProjection{}, err
		}
		state.CorrectionReason = "archive_reconstruction"
		raw, err := json.Marshal(state)
		if err != nil {
			return reconstructionProjection{}, err
		}
		stage = append(stage, store.StagedCandle{Interval: bar.Interval, Start: bar.Start, Snapshot: raw})
		actors := make([]string, 0, len(members[key]))
		for actor := range members[key] {
			actors = append(actors, actor)
		}
		sort.Strings(actors)
		for _, actor := range actors {
			traders = append(traders, store.StagedTrader{Interval: bar.Interval, Start: bar.Start, Actor: actor})
		}
	}
	for _, parent := range actual {
		if parent.Interval == "1m" {
			continue
		}
		duration, _ := IntervalSeconds(parent.Interval)
		children := []QualifiedCandle{}
		actors := []string{}
		for key, child := range buckets {
			if child.Interval == "1m" && child.Start >= parent.Start && child.Start < parent.Start+duration {
				children = append(children, child)
				for actor := range members[key] {
					actors = append(actors, actor)
				}
			}
		}
		rolled, _, err := RollupQualified(children, actors, parent.Interval, parent.Start)
		if err != nil {
			return reconstructionProjection{}, err
		}
		a, _ := EconomicDigest(parent)
		b, _ := EconomicDigest(rolled)
		if a != b {
			return reconstructionProjection{}, store.ErrIntegrity
		}
	}
	digest, err := projectionDigest(actual)
	if err != nil {
		return reconstructionProjection{}, err
	}
	return reconstructionProjection{Bars: actual, Candles: stage, Traders: traders, Digest: digest}, nil
}
