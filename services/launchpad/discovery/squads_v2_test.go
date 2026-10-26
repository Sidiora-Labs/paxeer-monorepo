package discovery

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
	"time"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestAdaptV2SquadEnvelopesMatchFrozenSchema(t *testing.T) {
	const requestID = "ed084842-de8e-4909-88d9-a10237835afb"
	q := Squad{
		ID:             "fb496a71-90cd-4784-9b49-be00ddd67a71",
		OwnerAccountID: "c5a4386a-0037-48e7-8b3d-593f6770b50d",
		Name:           "Research",
		Policy:         "request",
		Revision:       3,
	}
	at := time.Date(2026, 10, 26, 12, 34, 56, 123456789, time.FixedZone("test", 2*60*60))
	members := []Membership{
		{SquadID: q.ID, AccountID: q.OwnerAccountID, Role: "owner", Status: "active", JoinedAt: at.Add(-time.Hour), Revision: 3},
		{SquadID: q.ID, AccountID: "ea18012a-5e1b-4bb9-9e0d-8d29e416a5d5", Role: "member", Status: "active", JoinedAt: at, ProofReference: "private-proof", Revision: 3},
	}
	wantSquad := v2.Squad{
		SquadId: q.ID, Name: q.Name, Description: "", Visibility: "private",
		JoinPolicy: "request", Revision: "3", MemberCount: "1",
	}
	assertEnvelope := func(t *testing.T, schema string, body []byte, err error, revision string) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s adapter: %v", schema, err)
		}
		if err := v2.ValidateJSON(schema, body); err != nil {
			t.Fatalf("%s rejected by frozen schema: %v: %s", schema, err, body)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(body, &fields); err != nil {
			t.Fatal(err)
		}
		for key, want := range map[string]string{
			"requestId": `"` + requestID + `"`, "revision": `"` + revision + `"`,
			"quality": `"complete"`, "nextCursor": "null", "watermark": "null",
			"generatedAt": `"2026-10-26T10:34:56.123Z"`,
		} {
			if got := string(fields[key]); got != want {
				t.Errorf("%s = %s, want %s", key, got, want)
			}
		}
	}
	t.Run("create", func(t *testing.T) {
		body, err := AdaptV2SquadCreate(requestID, q, at)
		assertEnvelope(t, "SquadCreateResponse", body, err, "3")
		var got v2.SquadCreateResponse
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		if got.Data.Squad != wantSquad {
			t.Errorf("squad = %+v, want %+v", got.Data.Squad, wantSquad)
		}
	})
	t.Run("read with two members", func(t *testing.T) {
		body, err := AdaptV2SquadRead(requestID, q, members, at)
		assertEnvelope(t, "SquadReadResponse", body, err, "3")
		var got v2.SquadReadResponse
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		want := wantSquad
		want.MemberCount = "2"
		if got.Data.Squad != want || !reflect.DeepEqual(got.Data.AllowedActions, []string{}) {
			t.Errorf("read data = %+v, want squad %+v and empty allowedActions", got.Data, want)
		}
	})
	t.Run("membership", func(t *testing.T) {
		body, err := AdaptV2SquadMembership(requestID, members[1], at)
		assertEnvelope(t, "SquadMembershipResponse", body, err, "3")
		var got v2.SquadMembershipResponse
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		want := v2.SquadMembership{
			SquadId: q.ID, AccountId: members[1].AccountID,
			Role: "member", State: "active", Revision: "3",
		}
		if got.Data.Membership != want || got.Data.SquadRevision != "3" {
			t.Errorf("membership data = %+v, want %+v and squadRevision 3", got.Data, want)
		}
	})
	t.Run("full uint64 precision", func(t *testing.T) {
		next := q
		next.Revision = math.MaxUint64
		next.Policy = "invite"
		body, err := AdaptV2SquadCreate(requestID, next, at)
		assertEnvelope(t, "SquadCreateResponse", body, err, "18446744073709551615")
		var got v2.SquadCreateResponse
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		if got.Data.Squad.Revision != "18446744073709551615" || got.Data.Squad.JoinPolicy != "invite" {
			t.Errorf("squad = %+v", got.Data.Squad)
		}
	})
	t.Run("empty membership list", func(t *testing.T) {
		body, err := AdaptV2SquadRead(requestID, q, nil, at)
		assertEnvelope(t, "SquadReadResponse", body, err, "3")
		var got v2.SquadReadResponse
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		if got.Data.Squad.MemberCount != "0" || got.Data.AllowedActions == nil {
			t.Errorf("empty read data = %+v", got.Data)
		}
	})
	t.Run("invalid input returns no bytes", func(t *testing.T) {
		invalidSquad := q
		invalidSquad.Revision = 0
		invalidMember := members[1]
		invalidMember.Role = "administrator"
		for _, tc := range []struct {
			name  string
			adapt func() ([]byte, error)
		}{
			{"request ID", func() ([]byte, error) { return AdaptV2SquadCreate("invalid", q, at) }},
			{"squad", func() ([]byte, error) { return AdaptV2SquadRead(requestID, invalidSquad, members, at) }},
			{"membership", func() ([]byte, error) { return AdaptV2SquadMembership(requestID, invalidMember, at) }},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if body, err := tc.adapt(); err == nil || body != nil {
					t.Fatalf("body = %s, err = %v; want nil bytes and an error", body, err)
				}
			})
		}
	})
}
