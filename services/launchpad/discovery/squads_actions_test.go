package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
)

func TestPostgresV2SquadMembershipCoversFrozenActionEnum(t *testing.T) {
	raw, err := os.ReadFile("../../../sdk/launchpad/packages/production-contracts-v2/src/schemas.gen.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Defs map[string]struct {
			Properties map[string]struct {
				Enum []string `json:"enum"`
			} `json:"properties"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	actions := schema.Defs["SquadMembershipBody"].Properties["action"].Enum
	if len(actions) == 0 {
		t.Fatal("SquadMembershipBody.action has no frozen enum")
	}
	seen := make(map[string]bool, len(actions))
	for _, action := range actions {
		if action == "" || seen[action] {
			t.Fatalf("empty or duplicate action in frozen enum: %q", action)
		}
		seen[action] = true
		t.Run(action, func(t *testing.T) {
			s, _ := realStore(t)
			ctx := context.Background()
			id, owner, account := uuid.NewString(), uuid.NewString(), uuid.NewString()
			q := Squad{ID: id, OwnerAccountID: owner, Name: "Research", Visibility: "private", Policy: "request", Revision: 1}
			if action == "invite" {
				q.Policy = "invite"
			}
			t.Cleanup(func() {
				for _, table := range []string{"transfers", "invitations", "members"} {
					if _, err := s.DB.ExecContext(ctx, `DELETE FROM narrative_discovery.`+table+` WHERE squad_id=$1`, id); err != nil {
						t.Error(err)
					}
				}
				if _, err := s.DB.ExecContext(ctx, `DELETE FROM narrative_discovery.squads WHERE id=$1`, id); err != nil {
					t.Error(err)
				}
			})
			if err := s.CreateSquad(ctx, owner, q); err != nil {
				t.Fatal(err)
			}

			actor, state, role := owner, "active", "member"
			var expected uint64
			switch action {
			case "invite":
				state = "invited"
			case "request":
				actor, state = account, "requested"
			case "approve", "remove", "ban", "promote", "demote":
				m, err := s.RequestJoin(ctx, account, id, 0)
				if err != nil {
					t.Fatal(err)
				}
				expected = m.Revision
				if action != "approve" {
					m, err = s.ApproveRequest(ctx, owner, id, account, expected, time.Now().UTC())
					if err != nil {
						t.Fatal(err)
					}
					expected = m.Revision
				}
				switch action {
				case "remove":
					state = "removed"
				case "ban":
					state = "banned"
				case "promote":
					role = "moderator"
				case "demote":
					if err := s.SetMemberRole(ctx, owner, id, account, "moderator", expected); err != nil {
						t.Fatal(err)
					}
					expected++
				}
			default:
				t.Fatalf("frozen action %q needs a membership fixture and assertions", action)
			}

			handler := (HTTP{Store: s, Authenticate: func(*http.Request) (string, error) {
				return actor, nil
			}}).Handler()
			body, err := json.Marshal(v2.SquadMembershipBody{Action: action, ExpectedRevision: v2.UInt256(strconv.FormatUint(expected, 10))})
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest(http.MethodPut, "/v2/squads/"+id+"/members/"+account, bytes.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("X-Request-Id", uuid.NewString())
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("action %q returned %d, want 200: %s", action, w.Code, w.Body.String())
			}
			if err := v2.ValidateJSON("SquadMembershipResponse", w.Body.Bytes()); err != nil {
				t.Fatalf("invalid membership response: %v: %s", err, w.Body.String())
			}
			var response v2.SquadMembershipResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			want := v2.SquadMembership{SquadId: id, AccountId: account, State: state, Role: role, Revision: v2.PositiveUInt256(strconv.FormatUint(expected+1, 10))}
			if response.Data.Membership != want {
				t.Fatalf("membership = %+v, want %+v", response.Data.Membership, want)
			}
			var document []byte
			if err := s.DB.QueryRowContext(ctx, `SELECT document FROM narrative_discovery.members WHERE squad_id=$1 AND account_id=$2`, id, account).Scan(&document); err != nil {
				t.Fatal(err)
			}
			var stored Membership
			if err := json.Unmarshal(document, &stored); err != nil {
				t.Fatal(err)
			}
			if stored.SquadID != id || stored.AccountID != account || stored.Status != state || stored.Revision != expected+1 || (action != "invite" && stored.Role != role) {
				t.Fatalf("persisted membership = %+v, want %+v", stored, want)
			}
		})
	}
}
