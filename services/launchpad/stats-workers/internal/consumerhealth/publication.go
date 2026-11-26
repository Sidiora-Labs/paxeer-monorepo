package consumerhealth

import (
	"encoding/json"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func RetryKey(wire []byte) string {
	var envelope delivery.Envelope
	if json.Unmarshal(wire, &envelope) != nil {
		return ""
	}
	publication, err := accounting.DecodePublicationV1(envelope.Payload)
	if err != nil || len(publication.Positions) == 0 {
		return ""
	}
	return publication.Positions[0].PartitionKey
}
