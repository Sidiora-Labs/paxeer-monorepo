package streams

import (
	"encoding/json"
	"fmt"
	"time"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

const v2LifecycleTimeLayout = "2006-01-02T15:04:05.000Z"

func AdaptV2StreamStart(requestID string, o Operation) ([]byte, error) {
	operation, err := v2LifecycleOperation(o)
	if err != nil {
		return nil, err
	}
	return marshalV2Lifecycle("StreamStartResponse", v2.StreamStartResponse{
		RequestId:   requestID,
		Data:        v2.StreamStartData{Operation: operation},
		Revision:    &operation.Revision,
		Quality:     "complete",
		GeneratedAt: time.Now().UTC().Format(v2LifecycleTimeLayout),
	})
}

func AdaptV2StreamEnd(requestID string, o Operation) ([]byte, error) {
	operation, err := v2LifecycleOperation(o)
	if err != nil {
		return nil, err
	}
	return marshalV2Lifecycle("StreamEndResponse", v2.StreamEndResponse{
		RequestId:   requestID,
		Data:        v2.StreamEndData{Operation: operation},
		Revision:    &operation.Revision,
		Quality:     "complete",
		GeneratedAt: time.Now().UTC().Format(v2LifecycleTimeLayout),
	})
}

func AdaptV2StreamHeartbeat(requestID string, streamID string, viewers int64, at time.Time) ([]byte, error) {
	if !frozenUUID.MatchString(streamID) || viewers < 0 || viewers > 9007199254740991 || at.IsZero() {
		return nil, ErrInvalid
	}
	return marshalV2Lifecycle("StreamHeartbeatResponse", v2.StreamHeartbeatResponse{
		RequestId:   requestID,
		Data:        v2.StreamHeartbeatData{ViewerCount: v2.SafeInteger(viewers), ViewerQuality: "verified"},
		Quality:     "complete",
		GeneratedAt: at.UTC().Format(v2LifecycleTimeLayout),
	})
}

func v2LifecycleOperation(o Operation) (v2.StreamOperation, error) {
	operation, err := AdaptV2Operation(o)
	if err != nil {
		return v2.StreamOperation{}, err
	}
	return v2.StreamOperation{
		OperationId: operation.OperationID,
		StreamId:    operation.StreamID,
		Status:      operation.Status,
		Revision:    v2.PositiveUInt256(operation.Revision),
		RetryAt:     operation.RetryAt,
	}, nil
}

func marshalV2Lifecycle(name string, response any) ([]byte, error) {
	raw, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}
	if err := v2.ValidateJSON(name, raw); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return raw, nil
}
