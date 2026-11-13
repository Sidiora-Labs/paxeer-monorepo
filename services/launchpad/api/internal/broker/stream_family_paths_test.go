package broker

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestCommittedStreamParserClassifiesFrozenTopicFamilies(t *testing.T) {
	for family, raw := range testFixtures(t) {
		t.Run(family, func(t *testing.T) {
			frame, err := parseStreamV2(raw)
			if family == "StreamsEvent" {
				if !errors.Is(err, ErrFrame) {
					t.Fatalf("stream control event: %+v, %v", frame, err)
				}
				return
			}
			if err != nil || frame.Topic == "" || frame.Sequence == "" || len(frame.Payload) == 0 {
				t.Fatalf("committed frame: %+v, %v", frame, err)
			}
		})
	}
	for _, raw := range [][]byte{nil, []byte(`{`), []byte(`{"op":"event"}`)} {
		if _, err := parseStreamV2(raw); !errors.Is(err, ErrFrame) {
			t.Fatalf("invalid committed frame: %s, %v", raw, err)
		}
	}
	var frame Frame
	if err := json.Unmarshal(testFixtures(t)["CandleEvent"], &frame); err != nil {
		t.Fatal(err)
	}
	frame.Topic = "market.candles." + "0x1111111111111111111111111111111111111111"
	raw, err := json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseStreamV2(raw); !errors.Is(err, ErrFrame) {
		t.Fatalf("candle without interval: %v", err)
	}
}
