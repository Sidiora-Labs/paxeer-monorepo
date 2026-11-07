package types_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	at "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/autobahn/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	cs "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/autobahn/consensus"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/autobahn/consensus/persist"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/autobahn/data"
	apb "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/autobahn/pb"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p/giga"
	gpb "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p/giga/pb"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/protoutils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/tcp"
	"google.golang.org/protobuf/proto"
)

func r92Check(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func r92Committee(n int) (*at.Committee, []at.SecretKey) {
	keys := make([]at.SecretKey, n)
	weights := map[at.PublicKey]uint64{}
	for i := range keys {
		keys[i] = at.TestSecretKey(at.NodeID(fmt.Sprintf("repair-92-%d", i)))
		weights[keys[i].Public()] = 1
	}
	c := utils.OrPanic1(at.NewCommittee("repair-92", weights, 1, time.Unix(1_700_000_000, 0)))
	for i := range keys {
		keys[i] = keys[i].ForCommittee(c)
	}
	return c, keys
}

func r92Leader(c *at.Committee, keys []at.SecretKey, v at.View) at.SecretKey {
	for _, k := range keys {
		if k.Public() == c.Leader(v) {
			return k
		}
	}
	panic("leader absent")
}

func r92Payload() *at.Payload {
	return utils.OrPanic1((at.PayloadBuilder{CreatedAt: time.Now(), Txs: [][]byte{[]byte("repair-92")}}).Build())
}

func r92LaneQC(c *at.Committee, keys []at.SecretKey, b *at.Block, all bool) *at.LaneQC {
	var votes []*at.Signed[*at.LaneVote]
	count := int(c.LaneQuorum())
	if all {
		count = len(keys)
	}
	for _, k := range keys[:count] {
		votes = append(votes, at.Sign(k, at.NewLaneVote(b.Header())))
	}
	return at.NewLaneQC(votes)
}

func r92Commit(keys []at.SecretKey, p *at.FullProposal) *at.CommitQC {
	var votes []*at.Signed[*at.CommitVote]
	for _, k := range keys {
		votes = append(votes, at.Sign(k, at.NewCommitVote(p.Proposal().Msg())))
	}
	return at.NewCommitQC(votes)
}

func r92Prepare(keys []at.SecretKey, p *at.FullProposal) *at.PrepareQC {
	var votes []*at.Signed[*at.PrepareVote]
	for _, k := range keys {
		votes = append(votes, at.Sign(k, at.NewPrepareVote(p.Proposal().Msg())))
	}
	return at.NewPrepareQC(votes)
}

func r92Timeout(keys []at.SecretKey, v at.View, p utils.Option[*at.PrepareQC]) *at.TimeoutQC {
	var votes []*at.FullTimeoutVote
	for _, k := range keys {
		votes = append(votes, at.NewFullTimeoutVote(k, v, p))
	}
	return at.NewTimeoutQC(votes)
}

func r92Proposal(c *at.Committee, keys []at.SecretKey, vs at.ViewSpec, lanes map[at.LaneID]*at.LaneQC, app utils.Option[*at.AppQC]) *at.FullProposal {
	return utils.OrPanic1(at.NewProposal(r92Leader(c, keys, vs.View()), c, vs, time.Now(), lanes, app))
}

// The adversarial producer changes a single field and signs it with the elected
// leader's real key; transport and verification still use the production codec.
func r92Resign(c *at.Committee, keys []at.SecretKey, p *at.FullProposal, change func(*apb.Proposal)) *at.FullProposal {
	raw := at.FullProposalConv.Encode(p)
	body := proto.Clone(at.ProposalConv.Encode(p.Proposal().Msg())).(*apb.Proposal)
	change(body)
	changed := utils.OrPanic1(at.ProposalConv.Decode(body))
	raw.Proposal = at.SignedMsgConv[*at.Proposal]().Encode(at.Sign(r92Leader(c, keys, p.View()), changed))
	return utils.OrPanic1(at.FullProposalConv.Decode(raw))
}

// Each harness uses actual TCP sockets, mux framing, registered RPC budgets and
// Wireguard scans. The callback is the real receiving state, never a substitute.
func r92ConsensusTransport(t *testing.T, consume func(context.Context, *at.FullProposal) error, work func(context.Context, func(*at.FullProposal) error) error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	left, right := tcp.TestPipe()
	defer left.Close()
	defer right.Close()
	server, client := rpc.NewServer[giga.API](), rpc.NewClient[giga.API]()
	results := make(chan error, 1)
	err := scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		s.SpawnBg(func() error { return utils.IgnoreAfterCancel(ctx, left.Run(ctx)) })
		s.SpawnBg(func() error { return utils.IgnoreAfterCancel(ctx, right.Run(ctx)) })
		s.SpawnBg(func() error { return utils.IgnoreAfterCancel(ctx, server.Run(ctx, left)) })
		s.SpawnBg(func() error { return utils.IgnoreAfterCancel(ctx, client.Run(ctx, right)) })
		s.SpawnBg(func() error {
			return utils.IgnoreAfterCancel(ctx, giga.Consensus.Serve(ctx, server, func(ctx context.Context, stream rpc.Stream[*gpb.ConsensusResp, *apb.ConsensusReq]) error {
				for {
					raw, err := stream.Recv(ctx)
					if err != nil {
						return err
					}
					req, err := at.ConsensusReqConv.DecodeReq(raw)
					if err == nil {
						p, ok := req.(*at.FullProposal)
						if !ok {
							return fmt.Errorf("unexpected request %T", req)
						}
						err = consume(ctx, p)
					}
					if sendErr := utils.Send(ctx, results, err); sendErr != nil {
						return sendErr
					}
				}
			}))
		})
		stream, err := giga.Consensus.Call(ctx, client)
		if err != nil {
			return err
		}
		return work(ctx, func(p *at.FullProposal) error {
			if err := stream.Send(ctx, at.ConsensusReqConv.Encode(p)); err != nil {
				return err
			}
			result, err := utils.Recv(ctx, results)
			if err != nil {
				return err
			}
			return result
		})
	})
	r92Check(t, err)
}

func r92Data(c *at.Committee, dir utils.Option[string]) (*data.State, *data.DataWAL) {
	wal := utils.OrPanic1(data.NewDataWAL(dir, c))
	return utils.OrPanic1(data.NewState(&data.Config{Committee: c}, wal)), wal
}

func r92Save(t *testing.T, wal *data.DataWAL, c *at.Committee, q *at.FullCommitQC, blocks []*at.Block) {
	t.Helper()
	r92Check(t, q.Verify(c))
	r92Check(t, wal.CommitQCs.PersistQC(q))
	for i, b := range blocks {
		r92Check(t, wal.Blocks.PersistBlock(q.QC().GlobalRange(c).First+at.GlobalBlockNumber(i), b))
	}
}

func r92State(c *at.Committee, key at.SecretKey, d *data.State, dir string) *cs.State {
	return utils.OrPanic1(cs.NewState(&cs.Config{Key: key, ViewTimeout: func(at.View) time.Duration { return time.Hour }, PersistentStateDir: utils.Some(dir)}, d))
}

func r92RunState(t *testing.T, s *cs.State) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	return func() {
		cancel()
		err := <-done
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("state shutdown: %v", err)
		}
	}
}

func r92WaitVote(ctx context.Context, s *cs.State, v at.View) (*at.ConsensusReqPrepareVote, error) {
	vote, err := s.SubscribePrepareVote().Wait(ctx, func(vote utils.Option[*at.ConsensusReqPrepareVote]) bool {
		p, ok := vote.Get()
		return ok && p.View() == v
	})
	if err != nil {
		return nil, err
	}
	return vote.OrPanic("missing prepare vote"), nil
}

func TestRepair_92_CarriedAppState(t *testing.T) {
	c, keys := r92Committee(4)
	lane := c.Lanes().At(0)
	blocks := make([]*at.Block, 3)
	var parent at.BlockHeaderHash
	for i := range blocks {
		blocks[i] = at.NewBlock(lane, at.BlockNumber(i), parent, r92Payload())
		parent = blocks[i].Header().Hash()
	}
	p0 := r92Proposal(c, keys, at.ViewSpec{}, map[at.LaneID]*at.LaneQC{lane: r92LaneQC(c, keys, blocks[0], false)}, utils.None[*at.AppQC]())
	q0 := r92Commit(keys, p0)
	app := at.NewAppProposal(1, 0, bytes.Repeat([]byte{7}, 32))
	var av []*at.Signed[*at.AppVote]
	for _, k := range keys {
		av = append(av, at.Sign(k, at.NewAppVote(app)))
	}
	appQC := at.NewAppQC(av)
	vs1 := at.ViewSpec{CommitQC: utils.Some(q0)}
	p1 := r92Proposal(c, keys, vs1, map[at.LaneID]*at.LaneQC{lane: r92LaneQC(c, keys, blocks[1], false)}, utils.Some(appQC))
	r92Check(t, p1.Verify(c, vs1))
	q1 := r92Commit(keys, p1)
	vs2 := at.ViewSpec{CommitQC: utils.Some(q1)}
	p2 := r92Proposal(c, keys, vs2, map[at.LaneID]*at.LaneQC{lane: r92LaneQC(c, keys, blocks[2], false)}, utils.None[*at.AppQC]())
	r92Check(t, p2.Verify(c, vs2))
	dir := t.TempDir()
	d, wal := r92Data(c, utils.Some(dir))
	r92Save(t, wal, c, at.NewFullCommitQC(q0, []*at.BlockHeader{blocks[0].Header()}), blocks[:1])
	r92Save(t, wal, c, at.NewFullCommitQC(q1, []*at.BlockHeader{blocks[1].Header()}), blocks[1:2])
	// Reopen the data consumer with the committed prefix before voting.
	r92Check(t, wal.Close())
	d, wal = r92Data(c, utils.Some(dir))
	pers, _, err := persist.NewPersister[*apb.PersistedInner](utils.Some(dir), "inner")
	r92Check(t, err)
	r92Check(t, pers.Persist(&apb.PersistedInner{CommitQc: at.CommitQCConv.Encode(q1)}))
	receiver := keys[0]
	for _, k := range keys {
		if k.Public() != c.Leader(vs2.View()) {
			receiver = k
			break
		}
	}
	state := r92State(c, receiver, d, dir)
	stop := r92RunState(t, state)
	r92ConsensusTransport(t, state.PushProposal, func(ctx context.Context, send func(*at.FullProposal) error) error {
		for _, change := range []func(*apb.Proposal){func(p *apb.Proposal) { p.App.AppHash[0] ^= 1 }, func(p *apb.Proposal) { *p.App.GlobalNumber++ }} {
			bad := r92Resign(c, keys, p2, change)
			if err := send(bad); err == nil || !errors.Is(err, at.ErrCarriedAppState) {
				return fmt.Errorf("carried state mutation accepted or wrong rejection: %v", err)
			}
			if state.SubscribePrepareVote().Load().IsPresent() {
				return fmt.Errorf("voted for modified carried state")
			}
		}
		if err := send(p2); err != nil {
			return err
		}
		vote, err := r92WaitVote(ctx, state, p2.View())
		if err != nil {
			return err
		}
		got := vote.Msg().Proposal().App().OrPanic("no carried state")
		if got.GlobalNumber() != app.GlobalNumber() || !bytes.Equal(got.AppHash(), app.AppHash()) {
			return fmt.Errorf("vote changed carried state")
		}
		return nil
	})
	stop()
	// The persisted vote is replayed by the real consensus consumer.
	recovered := r92State(c, receiver, d, dir)
	stop = r92RunState(t, recovered)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	vote, err := r92WaitVote(ctx, recovered, p2.View())
	r92Check(t, err)
	got := vote.Msg().Proposal().App().OrPanic("no recovered app")
	if got.GlobalNumber() != 1 || got.RoadIndex() != 0 || !bytes.Equal(got.AppHash(), app.AppHash()) {
		t.Fatal("recovery replaced certified state")
	}
	stop()
	q2 := r92Commit(keys, p2)
	r92Save(t, wal, c, at.NewFullCommitQC(q2, []*at.BlockHeader{blocks[2].Header()}), blocks[2:])
	r92Check(t, wal.Close())
	d, wal = r92Data(c, utils.Some(dir))
	defer wal.Close()
	final, err := d.GlobalBlock(ctx, 3)
	r92Check(t, err)
	got = final.FinalAppState.OrPanic("missing final app state")
	if got.GlobalNumber() != 1 || got.RoadIndex() != 0 || !bytes.Equal(got.AppHash(), app.AppHash()) {
		t.Fatal("data WAL replay changed FinalAppState")
	}
}

func TestRepair_92_FullCommitHeaderLimit(t *testing.T) {
	c, keys := r92Committee(11)
	dir := t.TempDir()
	producer, wal := r92Data(c, utils.Some(dir))
	var prev utils.Option[*at.CommitQC]
	progress := map[at.LaneID]bool{}
	var produced []*at.FullCommitQC
	parents := map[at.LaneID]at.BlockHeaderHash{}
	for round := range 11 {
		vs := at.ViewSpec{CommitQC: prev}
		lanes := map[at.LaneID]*at.LaneQC{}
		available := map[at.LaneID][]*at.Block{}
		for lane := range c.Lanes().All() {
			first := at.LaneRangeOpt(prev, lane).Next()
			parent := parents[lane]
			for i := range 10 {
				b := at.NewBlock(lane, first+at.BlockNumber(i), parent, r92Payload())
				parent = b.Header().Hash()
				available[lane] = append(available[lane], b)
			}
			lanes[lane] = r92LaneQC(c, keys, available[lane][9], false)
		}
		p := r92Proposal(c, keys, vs, lanes, utils.None[*at.AppQC]())
		r92Check(t, p.Verify(c, vs))
		if round == 0 {
			tooLarge := r92Resign(c, keys, p, func(body *apb.Proposal) {
				for _, lr := range body.LaneRanges {
					if *lr.First == *lr.Next {
						*lr.Next += 10
						lr.LastHash = make([]byte, 32)
					}
				}
			})
			if err := tooLarge.Verify(c, vs); err == nil {
				t.Fatalf("oversized proposal admission: %v", err)
			}
		}
		var blocks []*at.Block
		var headers []*at.BlockHeader
		for lane := range c.Lanes().All() {
			lr := p.Proposal().Msg().LaneRange(lane)
			if lr.Len() > 0 {
				progress[lane] = true
				parents[lane] = lr.LastHash()
			}
			for _, b := range available[lane][:int(lr.Len())] {
				blocks = append(blocks, b)
				headers = append(headers, b.Header())
			}
		}
		if len(headers) != at.MaxFullCommitQCHeaders {
			t.Fatalf("round %d finalized %d headers, want full budget", round, len(headers))
		}
		full := at.NewFullCommitQC(r92Commit(keys, p), headers)
		decoded, err := at.FullCommitQCConv.Unmarshal(at.FullCommitQCConv.Marshal(full))
		r92Check(t, err)
		r92Check(t, decoded.Verify(c))
		r92Check(t, producer.PushQC(t.Context(), decoded, blocks))
		r92Save(t, wal, c, decoded, blocks)
		produced = append(produced, decoded)
		prev = utils.Some(decoded.QC())
	}
	if len(progress) != 11 {
		t.Fatal("busy lane starvation")
	}
	// Retain the protocol scanner's rejection of certificates over 100 headers.
	oversized := at.FullCommitQCConv.Encode(produced[0])
	oversized.Headers = append(oversized.Headers, oversized.Headers[0])
	if _, err := at.FullCommitQCConv.Unmarshal(protoutils.Marshal(oversized)); err == nil {
		t.Fatal("wire scanner accepted 101 headers")
	}
	decoded, err := at.FullCommitQCConv.Decode(oversized)
	r92Check(t, err)
	if err := decoded.Verify(c); err == nil {
		t.Fatal("in-memory verifier accepted 101 headers")
	}
	// Exercise the registered synchronization service and its real data consumer.
	serving := r92State(c, keys[0], producer, t.TempDir())
	service := giga.NewService(serving)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	left, right := tcp.TestPipe()
	defer left.Close()
	defer right.Close()
	server, client := rpc.NewServer[giga.API](), rpc.NewClient[giga.API]()
	target, targetWAL := r92Data(c, utils.None[string]())
	defer targetWAL.Close()
	r92Check(t, scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		s.SpawnBg(func() error { return utils.IgnoreAfterCancel(ctx, left.Run(ctx)) })
		s.SpawnBg(func() error { return utils.IgnoreAfterCancel(ctx, right.Run(ctx)) })
		s.SpawnBg(func() error { return utils.IgnoreAfterCancel(ctx, server.Run(ctx, left)) })
		s.SpawnBg(func() error { return utils.IgnoreAfterCancel(ctx, client.Run(ctx, right)) })
		s.SpawnBg(func() error { return utils.IgnoreAfterCancel(ctx, service.RunServer(ctx, server)) })
		stream, err := giga.StreamFullCommitQCs.Call(ctx, client)
		if err != nil {
			return err
		}
		if err := stream.Send(ctx, giga.StreamFullCommitQCsReqConv.Encode(&giga.StreamFullCommitQCsReq{NextBlock: c.FirstBlock()})); err != nil {
			return err
		}
		for _, want := range produced {
			raw, err := stream.Recv(ctx)
			if err != nil {
				return err
			}
			got, err := at.FullCommitQCConv.DecodeReq(raw)
			if err != nil {
				return err
			}
			if err := target.PushQC(ctx, got, nil); err != nil {
				return err
			}
			observed, err := target.QC(ctx, want.QC().GlobalRange(c).First)
			if err != nil {
				return err
			}
			if observed.Index() != want.Index() || len(observed.Headers()) != 100 {
				return fmt.Errorf("sync consumer lost certificate")
			}
		}
		return nil
	}))
	r92Check(t, wal.Close())
	recovered, reopened := r92Data(c, utils.Some(dir))
	defer reopened.Close()
	if recovered.NextBlock() != 1101 {
		t.Fatalf("reopened data ended at %d", recovered.NextBlock())
	}
	for _, want := range produced {
		got, err := recovered.QC(ctx, want.QC().GlobalRange(c).First)
		r92Check(t, err)
		r92Check(t, got.Verify(c))
		if !bytes.Equal(at.FullCommitQCConv.Marshal(got), at.FullCommitQCConv.Marshal(want)) {
			t.Fatal("WAL changed certificate")
		}
	}
}

func TestRepair_92_CommitteeWireLimit(t *testing.T) {
	c, keys := r92Committee(at.MaxCommitteeSize)
	for _, n := range []int{at.MaxCommitteeSize + 1, 32, 101} {
		weights := map[at.PublicKey]uint64{}
		validators := make([]config.AutobahnValidator, n)
		for i := range n {
			k := at.TestSecretKey(at.NodeID(fmt.Sprintf("boundary-%d", i)))
			weights[k.Public()] = 1
			validators[i].ValidatorKey = k.Public()
		}
		if _, err := at.NewCommittee("boundary", weights, 1, c.GenesisTimestamp()); err == nil {
			t.Fatalf("accepted %d replicas", n)
		}
		cfg := config.AutobahnFileConfig{Validators: validators, MaxTxsPerBlock: 1, BlockInterval: 1, ViewTimeout: 1, DialInterval: 1, UnsafeTestOnlyDisablePersistence: true}
		if err := cfg.Validate(); err == nil {
			t.Fatalf("startup admitted %d validators: %v", n, err)
		}
	}
	cfg := config.AutobahnFileConfig{Validators: make([]config.AutobahnValidator, at.MaxCommitteeSize), MaxTxsPerBlock: 1, BlockInterval: 1, ViewTimeout: 1, DialInterval: 1, UnsafeTestOnlyDisablePersistence: true}
	r92Check(t, cfg.Validate())
	// Zero-weight replicas consume no wire slots and are not admitted members.
	weights := map[at.PublicKey]uint64{}
	for _, k := range keys {
		weights[k.Public()] = 1
	}
	weights[at.TestSecretKey("zero-weight").Public()] = 0
	admitted, err := at.NewCommittee(c.ChainID(), weights, c.FirstBlock(), c.GenesisTimestamp())
	r92Check(t, err)
	if admitted.Replicas().Len() != at.MaxCommitteeSize {
		t.Fatal("zero weight replica counted as a member")
	}
	lane := c.Lanes().At(0)
	b := at.NewBlock(lane, 0, at.BlockHeaderHash{}, r92Payload())
	p := r92Proposal(c, keys, at.ViewSpec{}, map[at.LaneID]*at.LaneQC{lane: r92LaneQC(c, keys, b, true)}, utils.None[*at.AppQC]())
	r92ConsensusTransport(t, func(_ context.Context, got *at.FullProposal) error { return got.Verify(c, at.ViewSpec{}) }, func(_ context.Context, send func(*at.FullProposal) error) error { return send(p) })
	q := r92Commit(keys, p)
	transported, err := at.CommitQCConv.Unmarshal(at.CommitQCConv.Marshal(q))
	r92Check(t, err)
	r92Check(t, transported.Verify(c))
	full := at.NewFullCommitQC(transported, []*at.BlockHeader{b.Header()})
	dir := t.TempDir()
	_, wal := r92Data(c, utils.Some(dir))
	r92Save(t, wal, c, full, []*at.Block{b})
	r92Check(t, wal.Close())
	d, wal := r92Data(c, utils.Some(dir))
	defer wal.Close()
	got, err := d.GlobalBlock(t.Context(), 1)
	r92Check(t, err)
	if got.Header.Hash() != b.Header().Hash() {
		t.Fatal("maximum committee replay lost block")
	}
}

func TestRepair_92_ConsensusMessageSize(t *testing.T) {
	c, keys := r92Committee(at.MaxCommitteeSize)
	lanes := map[at.LaneID]*at.LaneQC{}
	for lane := range c.Lanes().All() {
		b := at.NewBlock(lane, 0, at.BlockHeaderHash{}, r92Payload())
		lanes[lane] = r92LaneQC(c, keys, b, true)
	}
	p := r92Proposal(c, keys, at.ViewSpec{}, lanes, utils.None[*at.AppQC]())
	admitted := 0
	for lane := range c.Lanes().All() {
		if _, ok := p.LaneQC(lane); ok {
			admitted++
		}
	}
	if admitted == 0 || admitted == len(lanes) {
		t.Fatalf("expected bounded nonempty lane subset, got %d", admitted)
	}
	progress := map[at.LaneID]bool{}
	for round := range at.MaxCommitteeSize {
		vs := at.ViewSpec{}
		if round > 0 {
			vs.TimeoutQC = utils.Some(r92Timeout(keys, at.View{Number: at.ViewNumber(round - 1)}, utils.None[*at.PrepareQC]()))
		}
		candidate := r92Proposal(c, keys, vs, lanes, utils.None[*at.AppQC]())
		r92Check(t, candidate.Verify(c, vs))
		for lane := range c.Lanes().All() {
			if _, ok := candidate.LaneQC(lane); ok {
				progress[lane] = true
			}
		}
	}
	if len(progress) != at.MaxCommitteeSize {
		t.Fatal("transport budget starved a busy lane")
	}
	// All committee signatures and timeout evidence travel through the
	// registered Consensus stream, including the widest supported view number.
	var variants []*at.FullProposal
	variants = append(variants, p)
	tq := r92Timeout(keys, p.View(), utils.None[*at.PrepareQC]())
	vs := at.ViewSpec{TimeoutQC: utils.Some(tq)}
	freshTimeout := r92Proposal(c, keys, vs, lanes, utils.None[*at.AppQC]())
	variants = append(variants, freshTimeout)
	prepared := r92Prepare(keys, p)
	tq = r92Timeout(keys, p.View(), utils.Some(prepared))
	reproVS := at.ViewSpec{TimeoutQC: utils.Some(tq)}
	repro := r92Proposal(c, keys, reproVS, nil, utils.None[*at.AppQC]())
	variants = append(variants, repro)
	widestVS := at.ViewSpec{TimeoutQC: utils.Some(r92Timeout(keys, at.View{Number: math.MaxUint64 - 1}, utils.None[*at.PrepareQC]()))}
	variants = append(variants, r92Proposal(c, keys, widestVS, lanes, utils.None[*at.AppQC]()))
	for i, proposal := range variants {
		viewSpec := at.ViewSpec{}
		if i == 1 {
			viewSpec = vs
		}
		if i == 2 {
			viewSpec = reproVS
		}
		if i == 3 {
			viewSpec = widestVS
		}
		size := len(at.ConsensusReqConv.Marshal(proposal))
		if size > int(giga.Consensus.Req.MsgSize) || size > at.MaxConsensusReqBytes {
			t.Fatalf("proposal %d has %d bytes", i, size)
		}
		r92ConsensusTransport(t, func(_ context.Context, got *at.FullProposal) error { return got.Verify(c, viewSpec) }, func(_ context.Context, send func(*at.FullProposal) error) error { return send(proposal) })
	}
	// Fill an admitted body right up to its declared budget, using the real
	// application certificate producer and proposal admission rather than padding
	// the transported bytes. The app hash is opaque at this interface.
	first := c.Lanes().At(0)
	b := at.NewBlock(first, 0, at.BlockHeaderHash{}, r92Payload())
	base := r92Proposal(c, keys, at.ViewSpec{}, map[at.LaneID]*at.LaneQC{first: r92LaneQC(c, keys, b, false)}, utils.None[*at.AppQC]())
	baseQC := r92Commit(keys, base)
	var largest *at.FullProposal
	hashSize := 0
	for n := 1; n < at.MaxProposalProtoBytes; n++ {
		app := at.NewAppProposal(1, 0, bytes.Repeat([]byte{3}, n))
		var votes []*at.Signed[*at.AppVote]
		for _, k := range keys {
			votes = append(votes, at.Sign(k, at.NewAppVote(app)))
		}
		candidate, err := at.NewProposal(r92Leader(c, keys, at.View{Index: 1}), c, at.ViewSpec{CommitQC: utils.Some(baseQC)}, time.Now(), nil, utils.Some(at.NewAppQC(votes)))
		if err != nil {
			if !errors.Is(err, at.ErrProposalBudget) {
				t.Fatal(err)
			}
			break
		}
		largest = candidate
		hashSize = n
	}
	if largest == nil || hashSize == 0 {
		t.Fatal("no maximum admitted proposal")
	}
	if size := proto.Size(at.ProposalConv.Encode(largest.Proposal().Msg())); size != at.MaxProposalProtoBytes {
		t.Fatalf("largest body has %d bytes", size)
	}
	largestVS := at.ViewSpec{CommitQC: utils.Some(baseQC)}
	r92ConsensusTransport(t, func(_ context.Context, got *at.FullProposal) error { return got.Verify(c, largestVS) }, func(_ context.Context, send func(*at.FullProposal) error) error { return send(largest) })
	freshLargestVS := at.ViewSpec{CommitQC: utils.Some(baseQC), TimeoutQC: utils.Some(r92Timeout(keys, largest.View(), utils.None[*at.PrepareQC]()))}
	var appVotes []*at.Signed[*at.AppVote]
	for _, k := range keys {
		appVotes = append(appVotes, at.Sign(k, at.NewAppVote(largest.Proposal().Msg().App().OrPanic("no largest app"))))
	}
	freshLargest := r92Proposal(c, keys, freshLargestVS, nil, utils.Some(at.NewAppQC(appVotes)))
	r92ConsensusTransport(t, func(_ context.Context, got *at.FullProposal) error { return got.Verify(c, freshLargestVS) }, func(_ context.Context, send func(*at.FullProposal) error) error { return send(freshLargest) })
	prepared = r92Prepare(keys, largest)
	tq = r92Timeout(keys, largest.View(), utils.Some(prepared))
	largestVS.TimeoutQC = utils.Some(tq)
	largestRepro := r92Proposal(c, keys, largestVS, nil, utils.None[*at.AppQC]())
	r92ConsensusTransport(t, func(_ context.Context, got *at.FullProposal) error { return got.Verify(c, largestVS) }, func(_ context.Context, send func(*at.FullProposal) error) error { return send(largestRepro) })
}

func TestRepair_92_FutureConsensusTime(t *testing.T) {
	c, keys := r92Committee(4)
	lane := c.Lanes().At(0)
	b0 := at.NewBlock(lane, 0, at.BlockHeaderHash{}, r92Payload())
	b1 := at.NewBlock(lane, 1, b0.Header().Hash(), r92Payload())
	lanes := map[at.LaneID]*at.LaneQC{lane: r92LaneQC(c, keys, b1, false)}
	p := r92Proposal(c, keys, at.ViewSpec{}, lanes, utils.None[*at.AppQC]())
	bad := r92Resign(c, keys, p, func(p *apb.Proposal) { *p.Timestamp.Seconds += int64((365 * 24 * time.Hour) / time.Second) })
	if _, err := at.NewProposal(r92Leader(c, keys, at.View{}), c, at.ViewSpec{}, time.Now().Add(365*24*time.Hour), nil, utils.None[*at.AppQC]()); err == nil {
		t.Fatal("producer signed far future timestamp")
	}
	// At the exact drift boundary the proposal is valid; one nanosecond beyond is refused.
	stamp := p.Proposal().Msg().Timestamp()
	last := p.Proposal().Msg().BlockTimestamp(c, 2).OrPanic("missing last block time")
	r92Check(t, p.VerifyAt(c, at.ViewSpec{}, last.Add(-at.MaxFutureProposalDrift)))
	if err := p.VerifyAt(c, at.ViewSpec{}, last.Add(-at.MaxFutureProposalDrift-time.Nanosecond)); err == nil {
		t.Fatal("timeliness upper bound is open")
	}
	tq := r92Timeout(keys, at.View{}, utils.None[*at.PrepareQC]())
	vs := at.ViewSpec{TimeoutQC: utils.Some(tq)}
	good := r92Proposal(c, keys, vs, lanes, utils.None[*at.AppQC]())
	receiver := keys[0]
	for _, k := range keys {
		if k.Public() != c.Leader(p.View()) && k.Public() != c.Leader(good.View()) {
			receiver = k
			break
		}
	}
	dir := t.TempDir()
	d, wal := r92Data(c, utils.Some(dir))
	state := r92State(c, receiver, d, dir)
	stop := r92RunState(t, state)
	r92ConsensusTransport(t, state.PushProposal, func(ctx context.Context, send func(*at.FullProposal) error) error {
		if err := send(bad); err == nil || !errors.Is(err, at.ErrFutureProposalTime) {
			return fmt.Errorf("far future not rejected: %v", err)
		}
		if state.SubscribePrepareVote().Load().IsPresent() {
			return fmt.Errorf("voted for faulty future leader")
		}
		if err := state.PushTimeoutQC(ctx, tq); err != nil {
			return err
		}
		if err := send(good); err != nil {
			return err
		}
		vote, err := r92WaitVote(ctx, state, good.View())
		if err != nil {
			return err
		}
		if !vote.Msg().Proposal().Timestamp().Equal(good.Proposal().Msg().Timestamp()) {
			return fmt.Errorf("faulty timestamp reached prepare vote")
		}
		return nil
	})
	stop()
	full := at.NewFullCommitQC(r92Commit(keys, good), []*at.BlockHeader{b0.Header(), b1.Header()})
	r92Save(t, wal, c, full, []*at.Block{b0, b1})
	r92Check(t, wal.Close())
	d, wal = r92Data(c, utils.Some(dir))
	defer wal.Close()
	for n := at.GlobalBlockNumber(1); n <= 2; n++ {
		final, err := d.GlobalBlock(t.Context(), n)
		r92Check(t, err)
		want := good.Proposal().Msg().BlockTimestamp(c, n).OrPanic("missing expected block time")
		if !final.Timestamp.Equal(want) || final.Timestamp.After(time.Now().Add(at.MaxFutureProposalDrift)) {
			t.Fatal("faulty leader advanced recovered application time")
		}
	}
	recovered := r92State(c, receiver, d, dir)
	stop = r92RunState(t, recovered)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	vote, err := r92WaitVote(ctx, recovered, good.View())
	r92Check(t, err)
	if vote.Msg().Proposal().Timestamp().After(time.Now().Add(at.MaxFutureProposalDrift)) {
		t.Fatal("recovered application time jumped")
	}
	stop()
	// Certified reproposals retain their timestamp even if the validator's clock
	// moves backwards. The verified PrepareQC supplies quorum time evidence.
	pq := r92Prepare(keys, p)
	locked := at.ViewSpec{TimeoutQC: utils.Some(r92Timeout(keys, p.View(), utils.Some(pq)))}
	repro := r92Proposal(c, keys, locked, nil, utils.None[*at.AppQC]())
	r92Check(t, repro.VerifyAt(c, locked, stamp.Add(-24*time.Hour)))
	if !repro.Proposal().Msg().Timestamp().Equal(stamp) {
		t.Fatal("reproposal changed certified timestamp")
	}
	// A leader cannot claim that exception with forged timeout evidence.
	raw := at.FullProposalConv.Encode(repro)
	raw.TimeoutQc.Votes[0].Sig.Sig[0] ^= 1
	forged, err := at.FullProposalConv.Decode(raw)
	r92Check(t, err)
	if err := forged.VerifyAt(c, locked, stamp.Add(-24*time.Hour)); err == nil {
		t.Fatal("unverified timeout bypassed timeliness")
	}
}
