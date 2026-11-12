package auth

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestIdentityRequestDecoderCoversOwnerAndProofShapes(t *testing.T) {
	wallet := "0x" + strings.Repeat("a", 40)
	proof := `{"kind":"eoa_eip191","wallet":"` + wallet + `","signature":"0x` + strings.Repeat("00", 65) + `"}`
	linkID, challengeID := uuid.NewString(), uuid.NewString()
	valid := []struct {
		name, body string
		dest       any
	}{
		{"challenge", `{"wallet":"` + wallet + `","origin":"https://kindle.test"}`, new(ChallengeRequest)},
		{"link challenge", `{"wallet":"` + wallet + `","chainId":"125","visibility":"private"}`, new(WalletLinkChallengeRequest)},
		{"link verify", `{"linkId":"` + linkID + `","challengeId":"` + challengeID + `","proof":` + proof + `}`, new(WalletLinkVerifyRequest)},
		{"link remove", `{"linkId":"` + linkID + `","reauthProof":"owned-receipt"}`, new(WalletLinkRemoveRequest)},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			if err := DecodeIdentityRequest(strings.NewReader(tc.body), tc.dest); err != nil {
				t.Fatalf("valid owner request: %v", err)
			}
			encoded, err := json.Marshal(tc.dest)
			if err != nil {
				t.Fatal(err)
			}
			var want, got map[string]any
			if err := json.Unmarshal([]byte(tc.body), &want); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(encoded, &got); err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("decoded owner request: got=%s want=%s err=%v", encoded, tc.body, err)
			}
		})
	}
	for _, tc := range []struct {
		name, body string
		dest       any
	}{
		{"unknown destination", `{}`, new(struct{})},
		{"duplicate nested key", `{"wallet":"` + wallet + `","origin":"https://kindle.test","options":{"x":1,"x":2}}`, new(ChallengeRequest)},
		{"invalid challenge origin", `{"wallet":"` + wallet + `","origin":"http://kindle.test"}`, new(ChallengeRequest)},
		{"invalid challenge wallet", `{"wallet":"0xbroken","origin":"https://kindle.test"}`, new(ChallengeRequest)},
		{"invalid link chain", `{"wallet":"` + wallet + `","chainId":"1","visibility":"private"}`, new(WalletLinkChallengeRequest)},
		{"invalid link visibility", `{"wallet":"` + wallet + `","chainId":"125","visibility":"hidden"}`, new(WalletLinkChallengeRequest)},
		{"invalid link identifier", `{"linkId":"bad","challengeId":"` + challengeID + `","proof":` + proof + `}`, new(WalletLinkVerifyRequest)},
		{"invalid remove receipt", `{"linkId":"` + linkID + `","reauthProof":""}`, new(WalletLinkRemoveRequest)},
		{"proof missing signature", `{"linkId":"` + linkID + `","challengeId":"` + challengeID + `","proof":{"kind":"eoa_eip191","wallet":"` + wallet + `"}}`, new(WalletLinkVerifyRequest)},
		{"proof unknown field", `{"linkId":"` + linkID + `","challengeId":"` + challengeID + `","proof":{"kind":"eoa_eip191","wallet":"` + wallet + `","signature":"0x00","extra":true}}`, new(WalletLinkVerifyRequest)},
		{"proof invalid hex", `{"linkId":"` + linkID + `","challengeId":"` + challengeID + `","proof":{"kind":"eoa_eip191","wallet":"` + wallet + `","signature":"0xzz"}}`, new(WalletLinkVerifyRequest)},
		{"proof wrong size", `{"linkId":"` + linkID + `","challengeId":"` + challengeID + `","proof":{"kind":"eoa_eip191","wallet":"` + wallet + `","signature":"0x00"}}`, new(WalletLinkVerifyRequest)},
		{"proof wrong kind", `{"linkId":"` + linkID + `","challengeId":"` + challengeID + `","proof":{"kind":"other","wallet":"` + wallet + `","signature":"0x00"}}`, new(WalletLinkVerifyRequest)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := DecodeIdentityRequest(strings.NewReader(tc.body), tc.dest); err == nil {
				t.Fatalf("invalid owner request admitted: %s", tc.body)
			}
		})
	}
}
