package streams

import (
	"time"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func AdaptV2StreamCreate(requestID string, o Operation) ([]byte, error) {
	operation, err := v2LifecycleOperation(o)
	if err != nil {
		return nil, err
	}
	return marshalV2Lifecycle("StreamCreateResponse", v2.StreamCreateResponse{
		RequestId:   requestID,
		Data:        v2.StreamCreateData{Operation: operation},
		Revision:    &operation.Revision,
		Quality:     "complete",
		GeneratedAt: time.Now().UTC().Format(v2LifecycleTimeLayout),
	})
}

func AdaptV2StreamOperationGet(requestID string, o Operation, at time.Time) ([]byte, error) {
	operation, err := v2LifecycleOperation(o)
	if err != nil {
		return nil, err
	}
	if at.IsZero() {
		return nil, ErrInvalid
	}
	return marshalV2Lifecycle("StreamOperationGetResponse", v2.StreamOperationGetResponse{
		RequestId:   requestID,
		Data:        v2.StreamOperationGetData{Operation: operation},
		Revision:    &operation.Revision,
		Quality:     "complete",
		GeneratedAt: at.UTC().Format(v2LifecycleTimeLayout),
	})
}
