package candlewire

import (
	"encoding/json"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/engine"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

const SeriesModelVersion = engine.SeriesAlgorithm

type Candle = engine.CanonicalSeriesCandle

func FromStoredBar(raw []byte, sources map[string]accounting.FillV1) (Candle, error) {
	var bar engine.SeriesCandle
	if err := json.Unmarshal(raw, &bar); err != nil {
		return Candle{}, err
	}
	return engine.CanonicalCandle(bar, sources)
}
