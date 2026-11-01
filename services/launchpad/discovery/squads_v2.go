package discovery

import (
	"encoding/json"
	"strconv"
	"time"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func AdaptV2SquadCreate(requestID string, q Squad, at time.Time) ([]byte, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}
	revision := v2.PositiveUInt256(strconv.FormatUint(q.Revision, 10))
	return marshalV2SquadEnvelope("SquadCreateResponse", v2.SquadCreateResponse{
		RequestId:   requestID,
		Data:        v2.SquadCreateData{Squad: squadV2(q, 1)},
		Revision:    &revision,
		Quality:     "complete",
		GeneratedAt: at.UTC().Format("2006-01-02T15:04:05.000Z"),
	})
}

func AdaptV2SquadRead(requestID string, q Squad, members []Membership, at time.Time) ([]byte, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}
	var count uint64
	for _, m := range members {
		if m.SquadID != q.ID {
			return nil, ErrInvalid
		}
		if m.Status == "active" {
			count++
		}
	}
	revision := v2.PositiveUInt256(strconv.FormatUint(q.Revision, 10))
	return marshalV2SquadEnvelope("SquadReadResponse", v2.SquadReadResponse{
		RequestId:   requestID,
		Data:        v2.SquadReadData{Squad: squadV2(q, count), AllowedActions: []string{}},
		Revision:    &revision,
		Quality:     "complete",
		GeneratedAt: at.UTC().Format("2006-01-02T15:04:05.000Z"),
	})
}

func AdaptV2SquadMembership(requestID string, m Membership, at time.Time) ([]byte, error) {
	revision := v2.PositiveUInt256(strconv.FormatUint(m.Revision, 10))
	return marshalV2SquadEnvelope("SquadMembershipResponse", v2.SquadMembershipResponse{
		RequestId: requestID,
		Data: v2.SquadMembershipData{
			Membership: v2.SquadMembership{
				SquadId:   m.SquadID,
				AccountId: m.AccountID,
				Role:      m.Role,
				State:     m.Status,
				Revision:  revision,
			},
			SquadRevision: v2.UInt256(revision),
		},
		Revision:    &revision,
		Quality:     "complete",
		GeneratedAt: at.UTC().Format("2006-01-02T15:04:05.000Z"),
	})
}

func squadV2(q Squad, memberCount uint64) v2.Squad {
	return v2.Squad{
		SquadId:     q.ID,
		Name:        q.Name,
		Description: "",
		Visibility:  "private",
		JoinPolicy:  q.Policy,
		Revision:    v2.UInt256(strconv.FormatUint(q.Revision, 10)),
		MemberCount: v2.UInt256(strconv.FormatUint(memberCount, 10)),
	}
}

func marshalV2SquadEnvelope(schema string, envelope any) ([]byte, error) {
	body, err := json.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	if err := v2.ValidateJSON(schema, body); err != nil {
		return nil, err
	}
	return body, nil
}
