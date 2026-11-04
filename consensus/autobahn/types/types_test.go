package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/autobahn/pb"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/protoutils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
)

func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// msgTest tests a converter, generalized msg converter and signed msg converter.
func msgTest[T Msg, P protoutils.Message](rng utils.Rng, msg T, conv protoutils.Conv[T, P]) error {
	return firstErr(
		conv.Test(msg),
		MsgConv.Test(msg),
		SignedMsgConv[T]().Test(GenSigned(rng, msg)),
	)
}

// TestConv tests the protobuf converters.
func TestConv(t *testing.T) {
	rng := utils.TestRng()
	for range 10 {
		if err := firstErr(
			TimeConv.Test(utils.GenTimestamp(rng)),
			DurationConv.Test(time.Duration(int64(rng.Uint64()))),
			PublicKeyConv.Test(GenPublicKey(rng)),
			SignatureConv.Test(GenSignature(rng)),
			BlockHeaderConv.Test(GenBlockHeader(rng)),
			PayloadConv.Test(GenPayload(rng)),
			BlockConv.Test(GenBlock(rng)),
			LaneQCConv.Test(GenLaneQC(rng)),
			FullProposalConv.Test(GenFullProposal(rng)),
			PrepareQCConv.Test(GenPrepareQC(rng)),
			CommitQCConv.Test(GenCommitQC(rng)),
			FullCommitQCConv.Test(GenFullCommitQC(rng)),
			TimeoutQCConv.Test(GenTimeoutQC(rng)),
			FullTimeoutVoteConv.Test(GenFullTimeoutVote(rng)),
			AppProposalConv.Test(GenAppProposal(rng)),
			AppQCConv.Test(GenAppQC(rng)),
			AppVoteConv.Test(GenAppVote(rng)),
			LaneRangeConv.Test(GenLaneRange(rng)),
			ViewConv.Test(GenView(rng)),
			ConsensusReqConv.Test(GenFullProposal(rng)),
			ConsensusReqConv.Test(&ConsensusReqPrepareVote{GenSigned(rng, GenPrepareVote(rng))}),
			ConsensusReqConv.Test(&ConsensusReqCommitVote{GenSigned(rng, GenCommitVote(rng))}),
			ConsensusReqConv.Test(GenFullTimeoutVote(rng)),
			ConsensusReqConv.Test(GenTimeoutQC(rng)),
			msgTest(rng, GenLaneProposal(rng), LaneProposalConv),
			msgTest(rng, GenLaneVote(rng), LaneVoteConv),
			msgTest(rng, GenProposal(rng), ProposalConv),
			msgTest(rng, GenPrepareVote(rng), PrepareVoteConv),
			msgTest(rng, GenCommitVote(rng), CommitVoteConv),
			msgTest(rng, GenTimeoutVote(rng), TimeoutVoteConv),
			msgTest(rng, GenAppVote(rng), AppVoteConv),
		); err != nil {
			t.Error(err)
		}
	}
}

func TestMarshal(t *testing.T) {
	rng := utils.TestRng()
	var got, want struct {
		K  PublicKey
		Mk utils.Option[PublicKey]
		My utils.Option[PublicKey]
	}
	want.K = GenPublicKey(rng)
	want.Mk = utils.Some(GenPublicKey(rng))
	j, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(j, &got); err != nil {
		t.Fatal(err)
	}
	if err := utils.TestDiff(want, got); err != nil {
		t.Fatal(err)
	}
}

func TestNewRoundRobinElection_GenesisTimestamp(t *testing.T) {
	rng := utils.TestRng()
	replicas := []PublicKey{GenPublicKey(rng), GenPublicKey(rng)}
	firstBlock := GenGlobalBlockNumber(rng)
	genesisTimestamp := time.Now()

	committee, err := NewRoundRobinElection("test-chain", replicas, firstBlock, genesisTimestamp)
	if err != nil {
		t.Fatalf("NewRoundRobinElection(): %v", err)
	}

	if got := committee.FirstBlock(); got != firstBlock {
		t.Fatalf("FirstBlock() = %v, want %v", got, firstBlock)
	}
	if got := committee.GenesisTimestamp(); !got.Equal(genesisTimestamp) {
		t.Fatalf("GenesisTimestamp() = %v, want %v", got, genesisTimestamp)
	}
}

func makePrepareQC(keys []SecretKey, vote *PrepareVote) *PrepareQC {
	var votes []*Signed[*PrepareVote]
	for _, k := range keys {
		votes = append(votes, Sign(k, vote))
	}
	return NewPrepareQC(votes)
}

func TestNewTimeoutQC(t *testing.T) {
	rng := utils.TestRng()
	_, keys := GenCommittee(rng, 10)
	view := GenView(rng)
	var votes []*FullTimeoutVote
	wantView := View{}
	for _, k := range keys {
		pView := View{
			Index:  view.Index,
			Number: GenViewNumber(rng) % view.Number,
		}
		p := newProposal(pView, time.Now(), utils.GenSlice(rng, GenLaneRange), utils.Some(GenAppProposal(rng)))
		if wantView.Less(pView) {
			wantView = pView
		}
		votes = append(votes, NewFullTimeoutVote(k, view, utils.Some(makePrepareQC(keys, NewPrepareVote(p)))))
	}
	tQC := NewTimeoutQC(votes)
	pQC, ok := tQC.LatestPrepareQC().Get()
	if !ok {
		t.Fatalf("tQC.LatestPrepareQC() missing")
	}
	if gotView := pQC.View(); gotView != wantView {
		t.Fatalf("tQC.LatestPrepareQC().View() = %v, want %v", gotView, wantView)
	}
}

func TestTimeoutQCConvDecode_EmptyVotesReturnsError(t *testing.T) {
	if _, err := TimeoutQCConv.Decode(&pb.TimeoutQC{}); err == nil {
		t.Fatal("Decode() succeeded, want error")
	}
}

// TestNewTimeoutQC_MixedPrepareQCs verifies quorum-intersection behavior:
// even if only one vote carries a PrepareQC, NewTimeoutQC picks it up
// and Verify accepts the result.
func TestNewTimeoutQC_MixedPrepareQCs(t *testing.T) {
	rng := utils.TestRng()
	committee, keys := GenCommittee(rng, 4)
	view := View{Index: 0, Number: 0}

	var laneRanges []*LaneRange
	for lane := range committee.Lanes().All() {
		laneRanges = append(laneRanges, NewLaneRange(lane, 0, utils.None[*BlockHeader]()))
	}
	pqc := makePrepareQC(keys, NewPrepareVote(
		newProposal(view, time.Now(), laneRanges, utils.Some(GenAppProposal(rng))),
	))

	// Only keys[0] carries the PrepareQC; the rest carry None.
	votes := make([]*FullTimeoutVote, len(keys))
	votes[0] = NewFullTimeoutVote(keys[0], view, utils.Some(pqc))
	for i := 1; i < len(keys); i++ {
		votes[i] = NewFullTimeoutVote(keys[i], view, utils.None[*PrepareQC]())
	}

	tqc := NewTimeoutQC(votes)
	got, ok := tqc.LatestPrepareQC().Get()
	if !ok {
		t.Fatal("LatestPrepareQC must be present when at least one vote carries it")
	}
	if got.View() != view {
		t.Fatalf("LatestPrepareQC.View() = %v, want %v", got.View(), view)
	}
	if err := tqc.Verify(committee, utils.None[*CommitQC]()); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

// TestNewTimeoutQC_AllNone verifies that when no vote carries a PrepareQC,
// the resulting TimeoutQC has None and Verify accepts it.
func TestNewTimeoutQC_AllNone(t *testing.T) {
	rng := utils.TestRng()
	committee, keys := GenCommittee(rng, 4)
	view := View{Index: 0, Number: 0}

	votes := make([]*FullTimeoutVote, len(keys))
	for i, k := range keys {
		votes[i] = NewFullTimeoutVote(k, view, utils.None[*PrepareQC]())
	}

	tqc := NewTimeoutQC(votes)
	if tqc.LatestPrepareQC().IsPresent() {
		t.Fatal("LatestPrepareQC should be None when no vote carries one")
	}
	if err := tqc.Verify(committee, utils.None[*CommitQC]()); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

// TestTimeoutQCVerify_HighestPrepareQCSelected verifies that when votes carry
// PrepareQCs at different views, the highest is selected and Verify passes.
func TestTimeoutQCVerify_HighestPrepareQCSelected(t *testing.T) {
	rng := utils.TestRng()
	committee, keys := GenCommittee(rng, 4)
	view := View{Index: 0, Number: 5}

	var laneRanges []*LaneRange
	for lane := range committee.Lanes().All() {
		laneRanges = append(laneRanges, NewLaneRange(lane, 0, utils.None[*BlockHeader]()))
	}
	makePQCAt := func(vn ViewNumber) *PrepareQC {
		pView := View{Index: view.Index, Number: vn}
		return makePrepareQC(keys, NewPrepareVote(
			newProposal(pView, time.Now(), laneRanges, utils.Some(GenAppProposal(rng))),
		))
	}

	// keys[0] has PrepareQC at view number 2, keys[1] at 4, rest None.
	votes := make([]*FullTimeoutVote, len(keys))
	votes[0] = NewFullTimeoutVote(keys[0], view, utils.Some(makePQCAt(2)))
	votes[1] = NewFullTimeoutVote(keys[1], view, utils.Some(makePQCAt(4)))
	votes[2] = NewFullTimeoutVote(keys[2], view, utils.None[*PrepareQC]())
	votes[3] = NewFullTimeoutVote(keys[3], view, utils.None[*PrepareQC]())

	tqc := NewTimeoutQC(votes)
	got, ok := tqc.LatestPrepareQC().Get()
	if !ok {
		t.Fatal("LatestPrepareQC must be present")
	}
	wantView := View{Index: 0, Number: 4}
	if got.View() != wantView {
		t.Fatalf("LatestPrepareQC.View() = %v, want %v", got.View(), wantView)
	}
	if err := tqc.Verify(committee, utils.None[*CommitQC]()); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestConvProductionBounds(t *testing.T) {
	for _, size := range []int{4, MaxCommitteeSize} {
		t.Run(fmt.Sprintf("committee_%d", size), func(t *testing.T) {
			committee, keys := GenCommittee(utils.TestRng(), size)
			lanes := map[LaneID]*LaneQC{}
			blocks := map[LaneID][]*Block{}
			perLane := MaxFullCommitQCHeaders / size
			for lane := range committee.Lanes().All() {
				var parent BlockHeaderHash
				for i := range perLane {
					payload, err := (PayloadBuilder{CreatedAt: time.Now(), Txs: [][]byte{[]byte("producer codec input")}}).Build()
					if err != nil {
						t.Fatal(err)
					}
					block := NewBlock(lane, BlockNumber(i), parent, payload)
					blocks[lane] = append(blocks[lane], block)
					parent = block.Header().Hash()
				}
				var votes []*Signed[*LaneVote]
				for _, key := range keys {
					votes = append(votes, Sign(key, NewLaneVote(blocks[lane][perLane-1].Header())))
				}
				lanes[lane] = NewLaneQC(votes)
			}
			proposal, err := NewProposal(leaderKey(committee, keys, View{}), committee, ViewSpec{}, time.Now(), lanes, utils.None[*AppQC]())
			if err != nil {
				t.Fatal(err)
			}
			transported, err := ConsensusReqConv.Unmarshal(ConsensusReqConv.Marshal(proposal))
			if err != nil {
				t.Fatal(err)
			}
			received, ok := transported.(*FullProposal)
			if !ok {
				t.Fatalf("unexpected consensus message %T", transported)
			}
			if err := received.Verify(committee, ViewSpec{}); err != nil {
				t.Fatal(err)
			}
			var headers []*BlockHeader
			for lane := range committee.Lanes().All() {
				lr := received.Proposal().Msg().LaneRange(lane)
				for _, block := range blocks[lane][:int(lr.Len())] {
					replayed, err := BlockConv.Unmarshal(BlockConv.Marshal(block))
					if err != nil {
						t.Fatal(err)
					}
					if err := replayed.Verify(committee); err != nil {
						t.Fatal(err)
					}
					if replayed.Header().Hash() != block.Header().Hash() {
						t.Fatal("block codec changed committed header")
					}
					headers = append(headers, replayed.Header())
				}
			}
			if size == 4 && len(headers) != MaxFullCommitQCHeaders {
				t.Fatalf("header boundary not covered: %d", len(headers))
			}
			if len(headers) == 0 {
				t.Fatal("busy committee finalized no blocks")
			}
			full := NewFullCommitQC(makeCommitQC(keys, received), headers)
			recovered, err := FullCommitQCConv.Unmarshal(FullCommitQCConv.Marshal(full))
			if err != nil {
				t.Fatal(err)
			}
			if err := recovered.Verify(committee); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(FullCommitQCConv.Marshal(full), FullCommitQCConv.Marshal(recovered)) {
				t.Fatal("full certificate codec changed committed history")
			}
		})
	}
}
