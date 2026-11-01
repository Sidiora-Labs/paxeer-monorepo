package streams

import (
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

type v2StreamListData struct {
	Items []V2Stream `json:"items"`
}

type v2StreamListEnvelope struct {
	RequestID   string              `json:"requestId"`
	Data        v2StreamListData    `json:"data"`
	Watermark   *v2.Watermark       `json:"watermark"`
	Revision    *v2.PositiveUInt256 `json:"revision"`
	Quality     string              `json:"quality"`
	NextCursor  *string             `json:"nextCursor"`
	GeneratedAt string              `json:"generatedAt"`
}

func AdaptV2StreamList(requestID string, items []V2Stream, nextCursor *string, generatedAt time.Time) ([]byte, error) {
	maximum := new(big.Int)
	var revision *v2.PositiveUInt256
	for _, item := range items {
		raw, err := json.Marshal(item.Revision)
		if err != nil {
			return nil, err
		}
		if err := v2.ValidateJSON("PositiveUInt256", raw); err != nil {
			return nil, fmt.Errorf("%w: stream revision: %v", ErrInvalid, err)
		}
		value, ok := new(big.Int).SetString(item.Revision, 10)
		if !ok {
			return nil, ErrInvalid
		}
		if value.Cmp(maximum) > 0 {
			maximum.Set(value)
		}
	}
	if len(items) > 0 {
		value := v2.PositiveUInt256(maximum.String())
		revision = &value
	}
	if items == nil {
		items = []V2Stream{}
	}
	raw, err := json.Marshal(v2StreamListEnvelope{
		RequestID:   requestID,
		Data:        v2StreamListData{Items: items},
		Revision:    revision,
		Quality:     "complete",
		NextCursor:  nextCursor,
		GeneratedAt: generatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
	})
	if err != nil {
		return nil, err
	}
	if err := v2.ValidateJSON("StreamListResponse", raw); err != nil {
		return nil, fmt.Errorf("%w: StreamListResponse: %v", ErrInvalid, err)
	}
	return raw, nil
}
