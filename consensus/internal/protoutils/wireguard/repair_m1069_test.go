package wireguard_test

import (
	"testing"

	gogoproto "github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/protoutils/wireguard"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/protoutils/wireguard/wgtest"
	bcproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/blocksync"
)

func TestRepairM_1069_DuplicateSingularCommitScan(t *testing.T) {
	for _, path := range []struct {
		name     string
		message  func(*testing.T, ...int) []byte
		evidence bool
	}{
		{name: "block_last_commit", message: repair78LastCommit},
		{name: "signed_header_commit", message: repair78SignedHeader, evidence: true},
	} {
		t.Run(path.name, func(t *testing.T) {
			limit := wgtest.MaxCommitSignatures
			for _, split := range []struct {
				name   string
				counts []int
			}{
				{name: "two_occurrences", counts: []int{limit - 1, 2}},
				{name: "three_occurrences", counts: []int{limit / 2, limit - limit/2, 1}},
				{name: "empty_occurrence", counts: []int{0, limit, 1}},
			} {
				t.Run(split.name, func(t *testing.T) {
					for _, count := range split.counts {
						require.LessOrEqual(t, count, limit)
						require.NoError(t, wireguard.Scan(path.message(t, count), bcproto.SchemaForMessage))
					}

					payload := path.message(t, split.counts...)
					var decoded bcproto.Message
					require.NoError(t, gogoproto.Unmarshal(payload, &decoded))
					require.Len(t, repair78Commit(t, &decoded, path.evidence).Signatures, limit+1)
					require.Error(t, wireguard.Scan(payload, bcproto.SchemaForMessage))
					require.Error(t, decoded.WireguardScan(payload))

					boundary := append([]int(nil), split.counts...)
					boundary[len(boundary)-1]--
					atCap := path.message(t, boundary...)
					var accepted bcproto.Message
					require.NoError(t, gogoproto.Unmarshal(atCap, &accepted))
					require.Len(t, repair78Commit(t, &accepted, path.evidence).Signatures, limit)
					require.NoError(t, wireguard.Scan(atCap, bcproto.SchemaForMessage))
					require.NoError(t, accepted.WireguardScan(atCap))
				})
			}
		})
	}
}

func TestRepairM_1069_DuplicateLastCommitTransport(t *testing.T) {
	limit := wgtest.MaxCommitSignatures
	repair78Channel(t,
		repair78LastCommit(t, limit-1, 1),
		repair78LastCommit(t, limit-1, 2), false)
}

func TestRepairM_1069_DuplicateSignedHeaderCommitTransport(t *testing.T) {
	limit := wgtest.MaxCommitSignatures
	repair78Channel(t,
		repair78SignedHeader(t, limit/2, limit-limit/2, 0),
		repair78SignedHeader(t, limit/2, limit-limit/2, 1), true)
}
