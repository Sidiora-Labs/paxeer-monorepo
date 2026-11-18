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
		Quality:     qualityComplete,
		GeneratedAt: at.UTC().Format("2006-01-02T15:04:05.000Z"),
	})
}

func AdaptV2SquadRead(requestID string, q Squad, members []Membership, at time.Time) ([]byte, error) {
	return adaptV2SquadRead(requestID, q, members, "", at)
}

func adaptV2SquadRead(requestID string, q Squad, members []Membership, actor string, at time.Time) ([]byte, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}
	var count uint64
	for _, m := range members {
		if m.SquadID != q.ID {
			return nil, ErrInvalid
		}
		if m.Status == statusActive {
			count++
		}
	}
	revision := v2.PositiveUInt256(strconv.FormatUint(q.Revision, 10))
	return marshalV2SquadEnvelope("SquadReadResponse", v2.SquadReadResponse{
		RequestId:   requestID,
		Data:        v2.SquadReadData{Squad: squadV2(q, count), AllowedActions: squadAllowedActions(q, members, actor)},
		Revision:    &revision,
		Quality:     qualityComplete,
		GeneratedAt: at.UTC().Format("2006-01-02T15:04:05.000Z"),
	})
}

func squadAllowedActions(q Squad, members []Membership, actor string) []string {
	actions := []string{}
	if actor == "" {
		return actions
	}
	var current Membership
	for _, m := range members {
		if m.AccountID == actor {
			current = m
			break
		}
	}
	if current.Status == statusActive {
		if actor == q.OwnerAccountID || current.Role == roleModerator {
			actions = append(actions, "invite")
			if q.Policy == actionRequest {
				actions = append(actions, "approve")
			}
			if actor == q.OwnerAccountID {
				actions = append(actions, "remove", "ban", "promote", "demote")
			}
		}
	} else if (current.Status == "" || current.Status == stateRemoved) && (q.Policy == policyOpen || q.Policy == actionRequest) {
		actions = append(actions, actionRequest)
	}
	return actions
}

func AdaptV2SquadMembership(requestID string, m Membership, q Squad, at time.Time) ([]byte, error) {
	if q.Validate() != nil || q.ID != m.SquadID {
		return nil, ErrInvalid
	}
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
			SquadRevision: v2.UInt256(strconv.FormatUint(q.Revision, 10)),
		},
		Revision:    &revision,
		Quality:     qualityComplete,
		GeneratedAt: at.UTC().Format("2006-01-02T15:04:05.000Z"),
	})
}

func squadV2(q Squad, memberCount uint64) v2.Squad {
	return v2.Squad{
		SquadId:     q.ID,
		Name:        q.Name,
		Description: q.Description,
		Visibility:  q.Visibility,
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
