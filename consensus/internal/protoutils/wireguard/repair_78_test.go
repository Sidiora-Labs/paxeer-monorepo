package wireguard_test

import (
	"context"
	"testing"
	"time"

	gogoproto "github.com/gogo/protobuf/proto"
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/blocksync"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/protoutils/wireguard"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/protoutils/wireguard/wgtest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	bcproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/blocksync"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
)

func repair78BlockResponse(block []byte) []byte {
	response := appendBytesField(nil, wireguard.MustFieldNum[bcproto.BlockResponse]("block"), block)
	return appendBytesField(nil, wireguard.MustFieldNum[bcproto.Message_BlockResponse]("block_response"), response)
}

func repair78LastCommit(t *testing.T, counts ...int) []byte {
	t.Helper()
	var block []byte
	for _, count := range counts {
		// Concatenating real producer output preserves duplicate singular
		// fields, which a normal unmarshal/marshal round trip would collapse.
		block = append(block, wgtest.Marshal(t, &tmproto.Block{LastCommit: wgtest.CommitWith(count)})...)
	}
	return repair78BlockResponse(block)
}

func repair78SignedHeader(t *testing.T, counts ...int) []byte {
	t.Helper()
	var header []byte
	for _, count := range counts {
		header = append(header, wgtest.Marshal(t, &tmproto.SignedHeader{Commit: wgtest.CommitWith(count)})...)
	}
	lightBlock := appendBytesField(nil, wireguard.MustFieldNum[tmproto.LightBlock]("signed_header"), header)
	attack := appendBytesField(nil, wireguard.MustFieldNum[tmproto.LightClientAttackEvidence]("conflicting_block"), lightBlock)
	evidence := appendBytesField(nil, wireguard.MustFieldNum[tmproto.Evidence_LightClientAttackEvidence]("light_client_attack_evidence"), attack)
	list := appendBytesField(nil, wireguard.MustFieldNum[tmproto.EvidenceList]("evidence"), evidence)
	block := appendBytesField(nil, wireguard.MustFieldNum[tmproto.Block]("evidence"), list)
	return repair78BlockResponse(block)
}

func repair78Commit(t *testing.T, message *bcproto.Message, evidence bool) *tmproto.Commit {
	t.Helper()
	block := message.GetBlockResponse().Block
	require.NotNil(t, block)
	if evidence {
		require.Len(t, block.Evidence.Evidence, 1)
		return block.Evidence.Evidence[0].GetLightClientAttackEvidence().ConflictingBlock.SignedHeader.Commit
	}
	return block.LastCommit
}

func repair78Channel(t *testing.T, atCap, overCap []byte, evidence bool) {
	t.Helper()
	// The ordinary codec demonstrates the exact decoded allocation that the
	// pre-decode hook must prevent, without replacing any transport component.
	for _, tc := range []struct {
		payload []byte
		count   int
	}{{atCap, wgtest.MaxCommitSignatures}, {overCap, wgtest.MaxCommitSignatures + 1}} {
		var decoded bcproto.Message
		require.NoError(t, gogoproto.Unmarshal(tc.payload, &decoded))
		require.Len(t, repair78Commit(t, &decoded, evidence).Signatures, tc.count)
	}

	network := p2p.MakeTestNetwork(t, p2p.TestNetworkOptions{NumNodes: 2})
	nodes := network.Nodes()
	sender, receiver := nodes[0], nodes[1]
	desc := blocksync.GetChannelDescriptor()
	inbound := p2p.TestMakeChannel(t, receiver, desc)
	// Empty's real generated codec retains unknown wire fields verbatim. This
	// lets the adversarial peer send duplicate fields through the real router,
	// encryption and framing without normalizing away the attack first.
	outbound := p2p.TestMakeChannel(t, sender, p2p.ChannelDescriptor[*gogotypes.Empty]{
		ID:                  desc.ID,
		MessageType:         &gogotypes.Empty{},
		Priority:            desc.Priority,
		SendQueueCapacity:   desc.SendQueueCapacity,
		RecvBufferCapacity:  desc.RecvBufferCapacity,
		RecvMessageCapacity: desc.RecvMessageCapacity,
		Name:                desc.Name,
	})
	network.Start(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	peer := receiver.WaitForConnAndGet(ctx, sender.NodeID)

	send := func(payload []byte) {
		wire := &gogotypes.Empty{}
		require.NoError(t, gogoproto.Unmarshal(payload, wire))
		require.Equal(t, payload, wgtest.Marshal(t, wire))
		outbound.Send(wire, receiver.NodeID)
	}
	send(atCap)
	got, err := inbound.Recv(ctx)
	require.NoError(t, err)
	require.Equal(t, sender.NodeID, got.From)
	require.Len(t, repair78Commit(t, got.Message, evidence).Signatures, wgtest.MaxCommitSignatures)

	send(overCap)
	receiver.WaitForDisconnect(ctx, peer)
	require.Zero(t, inbound.ReceiveLen(), "over-cap message reached the channel consumer")
}

func TestRepair_78_DuplicateLastCommitChannel(t *testing.T) {
	half := wgtest.MaxCommitSignatures / 2
	repair78Channel(t, repair78LastCommit(t, half, half), repair78LastCommit(t, half, half+1), false)
}

func TestRepair_78_DuplicateSignedHeaderCommitChannel(t *testing.T) {
	half := wgtest.MaxCommitSignatures / 2
	repair78Channel(t, repair78SignedHeader(t, half, half), repair78SignedHeader(t, half, half+1), true)
}

func TestRepair_78_MergedAncestorChannel(t *testing.T) {
	half := wgtest.MaxCommitSignatures / 2
	// Duplicate the singular BlockResponse.block rather than its child. State
	// must survive every merged ancestor to reach the same LastCommit.
	message := func(second int) []byte {
		var response []byte
		for _, count := range []int{half, second} {
			response = append(response, wgtest.Marshal(t, &bcproto.BlockResponse{
				Block: &tmproto.Block{LastCommit: wgtest.CommitWith(count)},
			})...)
		}
		return appendBytesField(nil, wireguard.MustFieldNum[bcproto.Message_BlockResponse]("block_response"), response)
	}
	repair78Channel(t, message(half), message(half+1), false)
}

func TestRepair_78_RepeatedChildrenRemainIndependent(t *testing.T) {
	// This is the cardinality rule the updated plugin emits for EvidenceList.
	schema := &wireguard.Schema{Rules: map[wireguard.Number]wireguard.Rule{
		wireguard.MustFieldNum[tmproto.EvidenceList]("evidence"): {
			Nested: utils.Some(tmproto.SchemaForEvidence), Repeated: true,
		},
	}}
	list := &tmproto.EvidenceList{Evidence: wgtest.EvidenceList(
		wgtest.CommitWith(wgtest.MaxCommitSignatures), wgtest.CommitWith(wgtest.MaxCommitSignatures))}
	payload := wgtest.Marshal(t, list)
	require.NoError(t, schema.Scan(payload))
	var decoded tmproto.EvidenceList
	require.NoError(t, gogoproto.Unmarshal(payload, &decoded))
	require.Len(t, decoded.Evidence, 2)
	for _, evidence := range decoded.Evidence {
		require.Len(t, evidence.GetLightClientAttackEvidence().ConflictingBlock.SignedHeader.Commit.Signatures, wgtest.MaxCommitSignatures)
	}
	list.Evidence = wgtest.EvidenceList(wgtest.CommitWith(wgtest.MaxCommitSignatures), wgtest.CommitWith(wgtest.MaxCommitSignatures+1))
	require.Error(t, schema.Scan(wgtest.Marshal(t, list)))
}

func TestRepair_78_SameSchemaDistinctFields(t *testing.T) {
	// Independent fields must not share counters merely because their types
	// are the same; only duplicate occurrences of the same field merge.
	leaf := &wireguard.Schema{Rules: map[wireguard.Number]wireguard.Rule{4: {MaxCount: 2}}}
	root := &wireguard.Schema{Rules: map[wireguard.Number]wireguard.Rule{
		1: {Nested: utils.Some(leaf)}, 2: {Nested: utils.Some(leaf)},
	}}
	commit := wgtest.Marshal(t, wgtest.CommitWith(2))
	payload := appendBytesField(nil, 1, commit)
	payload = appendBytesField(payload, 2, commit)
	require.NoError(t, root.Scan(payload))
	payload = appendBytesField(payload, 1, wgtest.Marshal(t, wgtest.CommitWith(1)))
	require.Error(t, root.Scan(payload))
}
