package types_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	at "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/autobahn/types"
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

func m1114RPC(t *testing.T, serve func(context.Context, rpc.Server[giga.API]) error, work func(context.Context, rpc.Client[giga.API]) error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	left, right := tcp.TestPipe()
	defer left.Close()
	defer right.Close()
	server, client := rpc.NewServer[giga.API](), rpc.NewClient[giga.API]()
	r92Check(t, scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		s.SpawnBg(func() error { return utils.IgnoreAfterCancel(ctx, left.Run(ctx)) })
		s.SpawnBg(func() error { return utils.IgnoreAfterCancel(ctx, right.Run(ctx)) })
		s.SpawnBg(func() error { return utils.IgnoreAfterCancel(ctx, server.Run(ctx, left)) })
		s.SpawnBg(func() error { return utils.IgnoreAfterCancel(ctx, client.Run(ctx, right)) })
		s.SpawnBg(func() error { return utils.IgnoreAfterCancel(ctx, serve(ctx, server)) })
		return work(ctx, client)
	}))
}

func m1114Durable(t *testing.T, d *data.State, last at.GlobalBlockNumber) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	r92Check(t, scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		s.SpawnBg(func() error { return utils.IgnoreAfterCancel(ctx, d.Run(ctx)) })
		return d.PushAppHash(ctx, last, at.AppHash("executed fixture state"))
	}))
}

func m1114AppQC(keys []at.SecretKey, p *at.AppProposal) *at.AppQC {
	var votes []*at.Signed[*at.AppVote]
	for _, key := range keys {
		votes = append(votes, at.Sign(key, at.NewAppVote(p)))
	}
	return at.NewAppQC(votes)
}

func m1114AppHistory(t *testing.T, c *at.Committee, keys []at.SecretKey, app *at.AppProposal) (at.ViewSpec, *at.FullProposal, []*at.FullCommitQC, [][]*at.Block) {
	t.Helper()
	lane := c.Lanes().At(0)
	b0 := at.NewBlock(lane, 0, at.BlockHeaderHash{}, r92Payload())
	p0 := r92Proposal(c, keys, at.ViewSpec{}, map[at.LaneID]*at.LaneQC{lane: r92LaneQC(c, keys, b0, true)}, utils.None[*at.AppQC]())
	r92Check(t, p0.Verify(c, at.ViewSpec{}))
	q0 := r92Commit(keys, p0)
	r92Check(t, app.Verify(c, q0))
	b1 := at.NewBlock(lane, 1, b0.Header().Hash(), r92Payload())
	vs := at.ViewSpec{CommitQC: utils.Some(q0)}
	p1 := r92Proposal(c, keys, vs, map[at.LaneID]*at.LaneQC{lane: r92LaneQC(c, keys, b1, true)}, utils.Some(m1114AppQC(keys, app)))
	r92Check(t, p1.Verify(c, vs))
	return vs, p1, []*at.FullCommitQC{at.NewFullCommitQC(q0, []*at.BlockHeader{b0.Header()}), at.NewFullCommitQC(r92Commit(keys, p1), []*at.BlockHeader{b1.Header()})}, [][]*at.Block{{b0}, {b1}}
}

func m1114Equal(t *testing.T, got, want []byte, what string) {
	t.Helper()
	if !bytes.Equal(got, want) {
		t.Fatalf("%s changed after publication", what)
	}
}

func TestRepairM_1114_MutableAppHash(t *testing.T) {
	for _, boundary := range []string{"construction", "decode"} {
		t.Run(boundary, func(t *testing.T) {
			c, keys := r92Committee(4)
			input := at.AppHash("the finalized application hash may have any length")
			original := bytes.Clone(input)
			app := at.NewAppProposal(1, 0, input)
			raw := at.AppProposalConv.Encode(app)
			if boundary == "decode" {
				var err error
				app, err = at.AppProposalConv.Decode(raw)
				r92Check(t, err)
			}
			vs, p, qcs, groups := m1114AppHistory(t, c, keys, app)
			qc := m1114AppQC(keys, app)
			wantQC, wantProposal := at.AppQCConv.Marshal(qc), at.FullProposalConv.Marshal(p)
			mutations := []func(){
				func() { input[0] ^= 1 },
				func() { raw.AppHash[0] ^= 1 },
				func() { app.AppHash()[0] ^= 1 },
				func() { at.AppProposalConv.Encode(app).AppHash[0] ^= 1 },
				func() { at.AppQCConv.Encode(qc).Vote.AppHash[0] ^= 1 },
				func() { at.FullProposalConv.Encode(p).AppQc.Vote.AppHash[0] ^= 1 },
			}
			for i, mutate := range mutations {
				mutate()
				m1114Equal(t, app.AppHash(), original, fmt.Sprintf("app hash at boundary %d", i))
				m1114Equal(t, at.AppQCConv.Marshal(qc), wantQC, "signed application certificate")
				m1114Equal(t, at.FullProposalConv.Marshal(p), wantProposal, "signed proposal")
				decoded, err := at.AppQCConv.Unmarshal(wantQC)
				r92Check(t, err)
				r92Check(t, decoded.Verify(c))
			}
			r92ConsensusTransport(t, func(_ context.Context, got *at.FullProposal) error {
				if err := got.Verify(c, vs); err != nil {
					return err
				}
				if !bytes.Equal(protoutils.Marshal(at.FullProposalConv.Encode(got).AppQc), wantQC) {
					return fmt.Errorf("transport changed certified state")
				}
				return nil
			}, func(_ context.Context, send func(*at.FullProposal) error) error { return send(p) })
			recovered := m1114SyncRecovery(t, c, keys, qcs, groups)
			final, err := recovered.GlobalBlock(t.Context(), 2)
			r92Check(t, err)
			state := final.FinalAppState.OrPanic("missing persisted application state")
			m1114Equal(t, state.AppHash(), original, "replayed application hash")
			state.AppHash()[0] ^= 1
			m1114Equal(t, state.AppHash(), original, "replayed application accessor")
		})
	}
}

func TestRepairM_1114_MutableHashedPayload(t *testing.T) {
	for _, boundary := range []string{"construction", "decode"} {
		t.Run(boundary, func(t *testing.T) {
			c, keys := r92Committee(4)
			builder := at.PayloadBuilder{CreatedAt: time.Now(), Txs: [][]byte{[]byte("first transaction"), []byte("second transaction")}, TotalGasWanted: 13, TotalGasEstimated: 17}
			payload, err := builder.Build()
			r92Check(t, err)
			raw := at.PayloadConv.Encode(payload)
			if boundary == "decode" {
				payload, err = at.PayloadConv.Decode(raw)
				r92Check(t, err)
			}
			lane := c.Lanes().At(0)
			block := at.NewBlock(lane, 0, at.BlockHeaderHash{}, payload)
			owner := keys[0]
			for _, k := range keys {
				if k.Public() == lane {
					owner = k
				}
			}
			signed := at.Sign(owner, at.NewLaneProposal(block))
			wantWire := at.SignedMsgConv[*at.LaneProposal]().Marshal(signed)
			wantHash := payload.Hash()
			wantBlock := at.BlockConv.Marshal(block)
			mutations := []func(){
				func() { builder.Txs[0][0] ^= 1 },
				func() { builder.Txs[1] = []byte("replacement builder transaction") },
				func() { payload.Txs()[0][0] ^= 1 },
				func() { payload.Txs()[1] = []byte("replacement accessor transaction") },
				func() {
					b := payload.ToBuilder()
					b.Txs[0][0] ^= 1
					b.Txs[1] = []byte("replacement rebuilt transaction")
					b.CreatedAt = time.Unix(1, 0)
				},
				func() { raw.Txs[0][0] ^= 1; raw.Txs[1] = []byte("replacement decode input") },
				func() {
					p := at.PayloadConv.Encode(payload)
					p.Txs[0][0] ^= 1
					p.Txs[1] = []byte("replacement encoding")
				},
				func() {
					b := at.BlockConv.Encode(block)
					b.Payload.Txs[0][0] ^= 1
					b.Header.PayloadHash[0] ^= 1
					b.Header.ParentHash[0] ^= 1
				},
			}
			for i, mutate := range mutations {
				mutate()
				if payload.Hash() != wantHash {
					t.Fatalf("payload commitment changed after mutation %d", i)
				}
				m1114Equal(t, at.BlockConv.Marshal(block), wantBlock, "block wire bytes")
				m1114Equal(t, at.SignedMsgConv[*at.LaneProposal]().Marshal(signed), wantWire, "signed lane proposal")
				decoded, err := at.SignedMsgConv[*at.LaneProposal]().Unmarshal(wantWire)
				r92Check(t, err)
				r92Check(t, decoded.VerifySig(c))
				r92Check(t, decoded.Msg().Verify(c))
			}
			p := r92Proposal(c, keys, at.ViewSpec{}, map[at.LaneID]*at.LaneQC{lane: r92LaneQC(c, keys, block, true)}, utils.None[*at.AppQC]())
			r92Check(t, p.Verify(c, at.ViewSpec{}))
			qc := at.NewFullCommitQC(r92Commit(keys, p), []*at.BlockHeader{block.Header()})
			recovered := m1114SyncRecovery(t, c, keys, []*at.FullCommitQC{qc}, [][]*at.Block{{block}})
			got, err := recovered.TryBlock(1)
			r92Check(t, err)
			m1114Equal(t, at.BlockConv.Marshal(got), wantBlock, "replayed block")
			got.Payload().Txs()[0][0] ^= 1
			m1114Equal(t, at.BlockConv.Marshal(got), wantBlock, "replayed payload accessor")
		})
	}
}

func TestRepairM_1114_AppQCBranches(t *testing.T) {
	c, keys := r92Committee(4)
	vs, p, _, _ := m1114AppHistory(t, c, keys, at.NewAppProposal(1, 0, at.AppHash("certified execution result")))
	r92ConsensusTransport(t, func(_ context.Context, got *at.FullProposal) error { return got.Verify(c, vs) }, func(_ context.Context, send func(*at.FullProposal) error) error {
		if err := send(p); err != nil {
			return fmt.Errorf("valid certificate rejected: %w", err)
		}
		raw := at.FullProposalConv.Encode(p)
		app := p.Proposal().Msg().App().OrPanic("no certified application state")
		raw.AppQc = at.AppQCConv.Encode(m1114AppQC(keys, at.NewAppProposal(app.GlobalNumber(), app.RoadIndex(), at.AppHash("different execution result"))))
		changed, err := at.FullProposalConv.Unmarshal(protoutils.Marshal(raw))
		if err != nil {
			return err
		}
		if err := send(changed); err == nil || err.Error() != "appQC doesn't match the proposal" {
			return fmt.Errorf("wrong content rejection: %v", err)
		}
		outsider := at.TestSecretKey("outside-committee").ForCommittee(c)
		raw = at.FullProposalConv.Encode(p)
		raw.AppQc = at.AppQCConv.Encode(m1114AppQC([]at.SecretKey{outsider}, app))
		changed, err = at.FullProposalConv.Unmarshal(protoutils.Marshal(raw))
		if err != nil {
			return err
		}
		want := fmt.Sprintf("appQC: %q is not a replica", outsider.Public())
		if err := send(changed); err == nil || err.Error() != want {
			return fmt.Errorf("wrong signer rejection: %v, want %s", err, want)
		}
		return send(p)
	})
}

func m1114SyncRecovery(t *testing.T, c *at.Committee, keys []at.SecretKey, qcs []*at.FullCommitQC, groups [][]*at.Block) *data.State {
	t.Helper()
	sourceDir, targetDir := t.TempDir(), t.TempDir()
	source, sourceWAL := r92Data(c, utils.Some(sourceDir))
	for i, qc := range qcs {
		r92Check(t, qc.Verify(c))
		r92Check(t, source.PushQC(t.Context(), qc, groups[i]))
	}
	last := qcs[len(qcs)-1].QC().GlobalRange(c).Next - 1
	m1114Durable(t, source, last)
	target, targetWAL := r92Data(c, utils.Some(targetDir))
	service := giga.NewService(r92State(c, keys[0], source, t.TempDir()))
	m1114RPC(t, service.RunServer, func(ctx context.Context, client rpc.Client[giga.API]) error {
		stream, err := giga.StreamFullCommitQCs.Call(ctx, client)
		if err != nil {
			return err
		}
		defer stream.Close()
		if err := stream.Send(ctx, giga.StreamFullCommitQCsReqConv.Encode(&giga.StreamFullCommitQCsReq{NextBlock: c.FirstBlock()})); err != nil {
			return err
		}
		for i, want := range qcs {
			raw, err := stream.Recv(ctx)
			if err != nil {
				return err
			}
			qc, err := at.FullCommitQCConv.DecodeReq(raw)
			if err != nil {
				return err
			}
			if err := qc.Verify(c); err != nil {
				return err
			}
			if !bytes.Equal(at.FullCommitQCConv.Marshal(qc), at.FullCommitQCConv.Marshal(want)) {
				return fmt.Errorf("sync changed finalized certificate")
			}
			if err := target.PushQC(ctx, qc, nil); err != nil {
				return err
			}
			for j, wantBlock := range groups[i] {
				n := qc.QC().GlobalRange(c).First + at.GlobalBlockNumber(j)
				get, err := giga.GetBlock.Call(ctx, client)
				if err != nil {
					return err
				}
				if err := get.Send(ctx, giga.GetBlockReqConv.Encode(&giga.GetBlockReq{GlobalNumber: n})); err != nil {
					get.Close()
					return err
				}
				response, err := get.Recv(ctx)
				get.Close()
				if err != nil {
					return err
				}
				result, err := giga.GetBlockRespConv.DecodeReq(response)
				if err != nil {
					return err
				}
				block, ok := result.Get()
				if !ok {
					return fmt.Errorf("finalized block %d unreadable over RPC", n)
				}
				if err := block.Verify(c); err != nil {
					return err
				}
				if !bytes.Equal(at.BlockConv.Marshal(block), at.BlockConv.Marshal(wantBlock)) {
					return fmt.Errorf("sync changed finalized block %d", n)
				}
				if err := target.PushBlock(ctx, n, block); err != nil {
					return err
				}
			}
		}
		return nil
	})
	m1114Durable(t, target, last)
	r92Check(t, sourceWAL.Close())
	r92Check(t, targetWAL.Close())
	for _, dir := range []string{sourceDir, targetDir} {
		recovered, wal := r92Data(c, utils.Some(dir))
		if recovered.NextBlock() != last+1 {
			t.Fatalf("recovered cursor %d, want %d", recovered.NextBlock(), last+1)
		}
		for i, want := range qcs {
			got, err := recovered.QC(t.Context(), want.QC().GlobalRange(c).First)
			r92Check(t, err)
			r92Check(t, got.Verify(c))
			m1114Equal(t, at.FullCommitQCConv.Marshal(got), at.FullCommitQCConv.Marshal(want), "WAL certificate")
			for j, wantBlock := range groups[i] {
				n := want.QC().GlobalRange(c).First + at.GlobalBlockNumber(j)
				block, err := recovered.TryBlock(n)
				r92Check(t, err)
				r92Check(t, block.Verify(c))
				m1114Equal(t, at.BlockConv.Marshal(block), at.BlockConv.Marshal(wantBlock), "WAL block")
				global, err := recovered.GlobalBlock(t.Context(), n)
				r92Check(t, err)
				if global.GlobalNumber != n || global.Header.Hash() != block.Header().Hash() {
					t.Fatalf("recovered global block %d changed", n)
				}
			}
		}
		if dir == targetDir {
			t.Cleanup(func() { r92Check(t, wal.Close()) })
			return recovered
		}
		r92Check(t, wal.Close())
	}
	panic("missing recovered consumer")
}

func TestRepairM_1114_ProductionBounds(t *testing.T) {
	for _, invalid := range []struct {
		name, want string
		txs        [][]byte
	}{
		{"transaction_count_over_limit", "too many transactions", make([][]byte, at.MaxTxsPerBlock+1)},
		{"transaction_bytes_over_limit", "total txs bytes too large", [][]byte{make([]byte, at.MaxTxsBytesPerBlock+1)}},
	} {
		t.Run(invalid.name, func(t *testing.T) {
			builder := at.PayloadBuilder{CreatedAt: time.Now(), Txs: invalid.txs}
			if _, err := builder.Build(); err == nil || err.Error() != invalid.want {
				t.Fatalf("wrong local payload bound rejection: %v", err)
			}
			raw := &apb.Payload{CreatedAt: at.TimeConv.Encode(builder.CreatedAt), TotalGasWanted: proto.Uint64(0), TotalGasEstimated: proto.Uint64(0), Txs: invalid.txs}
			if _, err := at.PayloadConv.Unmarshal(protoutils.Marshal(raw)); err == nil {
				t.Fatal("decoder admitted oversized payload")
			}
		})
	}
	for _, profile := range []struct {
		name               string
		committee, perLane int
		all                bool
	}{
		{"hundred_headers_and_payload_limits", 4, 25, true},
		{"max_committee_and_consensus_bytes", at.MaxCommitteeSize, 10, true},
	} {
		t.Run(profile.name, func(t *testing.T) {
			c, keys := r92Committee(profile.committee)
			lanes, available := map[at.LaneID]*at.LaneQC{}, map[at.LaneID][]*at.Block{}
			for lane := range c.Lanes().All() {
				var parent at.BlockHeaderHash
				for i := range profile.perLane {
					txs := [][]byte{[]byte(fmt.Sprintf("lane %s block %d", lane, i))}
					if i == 0 {
						txs = make([][]byte, at.MaxTxsPerBlock)
						for j := range txs {
							txs[j] = bytes.Repeat([]byte{byte(j)}, int(at.MaxTxsBytesPerBlock/at.MaxTxsPerBlock))
						}
					} else if i == 1 {
						txs = [][]byte{bytes.Repeat([]byte{0x42}, int(at.MaxTxsBytesPerBlock))}
					}
					payload, err := (at.PayloadBuilder{CreatedAt: time.Now(), Txs: txs}).Build()
					r92Check(t, err)
					b := at.NewBlock(lane, at.BlockNumber(i), parent, payload)
					if uint64(proto.Size(at.BlockConv.Encode(b))) > at.MaxBlockProtoSize {
						t.Fatal("producer exceeded block RPC bound")
					}
					available[lane] = append(available[lane], b)
					parent = b.Header().Hash()
				}
				lanes[lane] = r92LaneQC(c, keys, available[lane][profile.perLane-1], profile.all)
			}
			p := r92Proposal(c, keys, at.ViewSpec{}, lanes, utils.None[*at.AppQC]())
			r92Check(t, p.Verify(c, at.ViewSpec{}))
			wireSize := proto.Size(at.ConsensusReqConv.Encode(p))
			if wireSize > at.MaxConsensusReqBytes {
				t.Fatalf("producer exceeded consensus bytes: %d", wireSize)
			}
			var blocks []*at.Block
			var headers []*at.BlockHeader
			var omitted *at.LaneQC
			for lane := range c.Lanes().All() {
				lr := p.Proposal().Msg().LaneRange(lane)
				if lr.Len() == 0 {
					omitted = lanes[lane]
				}
				for _, b := range available[lane][:int(lr.Len())] {
					blocks = append(blocks, b)
					headers = append(headers, b.Header())
				}
			}
			if profile.committee == 4 && len(headers) != at.MaxFullCommitQCHeaders {
				t.Fatalf("header boundary not exercised: %d", len(headers))
			}
			if profile.committee == at.MaxCommitteeSize {
				if omitted == nil || wireSize+proto.Size(at.LaneQCConv.Encode(omitted)) <= at.MaxConsensusReqBytes {
					t.Fatalf("production byte admission bound not exercised: %d bytes", wireSize)
				}
			}
			r92ConsensusTransport(t, func(_ context.Context, got *at.FullProposal) error { return got.Verify(c, at.ViewSpec{}) }, func(_ context.Context, send func(*at.FullProposal) error) error { return send(p) })
			full := at.NewFullCommitQC(r92Commit(keys, p), headers)
			r92Check(t, full.Verify(c))
			m1114SyncRecovery(t, c, keys, []*at.FullCommitQC{full}, [][]*at.Block{blocks})
		})
	}
}

func TestRepairM_1114_TimestampCanonicalRange(t *testing.T) {
	c, keys := r92Committee(4)
	base := r92Proposal(c, keys, at.ViewSpec{}, nil, utils.None[*at.AppQC]())
	block := at.NewBlock(c.Lanes().At(0), 0, at.BlockHeaderHash{}, r92Payload())
	cases := []struct {
		name    string
		seconds int64
		nanos   int32
		field   string
	}{
		{"seconds_before_year_one", -62135596801, 0, "seconds"},
		{"seconds_after_year_9999", 253402300800, 0, "seconds"},
		{"minimum_int64", math.MinInt64, 0, "seconds"},
		{"maximum_int64", math.MaxInt64, 0, "seconds"},
		{"negative_nanos", 0, -1, "nanos"},
		{"billion_nanos", 0, 1000000000, "nanos"},
		{"minimum_int32_nanos", 0, math.MinInt32, "nanos"},
		{"maximum_int32_nanos", 0, math.MaxInt32, "nanos"},
	}
	proposals := make([]*apb.ConsensusReq, len(cases))
	responses := make([]*gpb.GetBlockResp, len(cases))
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			timestamp := &apb.Timestamp{Seconds: proto.Int64(tc.seconds), Nanos: proto.Int32(tc.nanos)}
			raw := at.FullProposalConv.Encode(base)
			raw.Proposal.Msg.GetProposal().Timestamp = timestamp
			_, err := at.FullProposalConv.Unmarshal(protoutils.Marshal(raw))
			if err == nil || !strings.Contains(err.Error(), "timestamp: "+tc.field+":") {
				t.Fatalf("malformed proposal timestamp: %v", err)
			}
			proposals[i] = &apb.ConsensusReq{T: &apb.ConsensusReq_Proposal{Proposal: raw}}
			payload := at.PayloadConv.Encode(block.Payload())
			payload.CreatedAt = timestamp
			_, err = at.PayloadConv.Unmarshal(protoutils.Marshal(payload))
			if err == nil || !strings.Contains(err.Error(), "created_at: "+tc.field+":") {
				t.Fatalf("malformed payload timestamp: %v", err)
			}
			responses[i] = giga.GetBlockRespConv.Encode(utils.Some(block))
			responses[i].Block.Payload.CreatedAt = timestamp
		})
	}
	results := make(chan error, 1)
	m1114RPC(t, func(ctx context.Context, server rpc.Server[giga.API]) error {
		return scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
			s.SpawnBg(func() error {
				return giga.GetBlock.Serve(ctx, server, func(ctx context.Context, stream rpc.Stream[*gpb.GetBlockResp, *gpb.GetBlockReq]) error {
					req, err := stream.Recv(ctx)
					if err != nil {
						return err
					}
					if req.GlobalNumber >= uint64(len(responses)) {
						return fmt.Errorf("unexpected fixture request")
					}
					return stream.Send(ctx, responses[req.GlobalNumber])
				})
			})
			return giga.Consensus.Serve(ctx, server, func(ctx context.Context, stream rpc.Stream[*gpb.ConsensusResp, *apb.ConsensusReq]) error {
				for {
					raw, err := stream.Recv(ctx)
					if err != nil {
						return err
					}
					_, err = at.ConsensusReqConv.DecodeReq(raw)
					if err := utils.Send(ctx, results, err); err != nil {
						return err
					}
				}
			})
		})
	}, func(ctx context.Context, client rpc.Client[giga.API]) error {
		consensus, err := giga.Consensus.Call(ctx, client)
		if err != nil {
			return err
		}
		defer consensus.Close()
		for i, tc := range cases {
			if err := consensus.Send(ctx, proposals[i]); err != nil {
				return err
			}
			rejection, err := utils.Recv(ctx, results)
			if err != nil {
				return err
			}
			if rejection == nil || !strings.Contains(rejection.Error(), "timestamp: "+tc.field+":") {
				return fmt.Errorf("RPC accepted %s proposal: %v", tc.name, rejection)
			}
			get, err := giga.GetBlock.Call(ctx, client)
			if err != nil {
				return err
			}
			if err := get.Send(ctx, &gpb.GetBlockReq{GlobalNumber: uint64(i)}); err != nil {
				get.Close()
				return err
			}
			raw, err := get.Recv(ctx)
			get.Close()
			if err != nil {
				return err
			}
			_, err = giga.GetBlockRespConv.DecodeReq(raw)
			if err == nil || !strings.Contains(err.Error(), "created_at: "+tc.field+":") {
				return fmt.Errorf("RPC accepted %s payload: %v", tc.name, err)
			}
		}
		return nil
	})
	for _, tc := range []struct {
		seconds int64
		nanos   int32
	}{{-62135596800, 0}, {-62135596800, 999999999}, {-1, 999999999}, {0, 0}, {253402300799, 0}, {253402300799, 999999999}} {
		timestamp := time.Unix(tc.seconds, int64(tc.nanos)).UTC()
		encoded := at.TimeConv.Encode(timestamp)
		decoded, err := at.TimeConv.Unmarshal(protoutils.Marshal(encoded))
		r92Check(t, err)
		if !timestamp.Equal(decoded) || encoded.GetSeconds() != tc.seconds || encoded.GetNanos() != tc.nanos {
			t.Fatal("canonical timestamp changed")
		}
		_, err = json.Marshal(decoded)
		r92Check(t, err)
		payload, err := (at.PayloadBuilder{CreatedAt: timestamp, Txs: [][]byte{[]byte("canonical timestamp input")}}).Build()
		r92Check(t, err)
		got, err := at.PayloadConv.Unmarshal(at.PayloadConv.Marshal(payload))
		r92Check(t, err)
		if !got.CreatedAt().Equal(timestamp) {
			t.Fatal("payload timestamp changed")
		}
	}
	for _, seconds := range []int64{-62135596801, 253402300800} {
		t.Run(fmt.Sprintf("local_%d", seconds), func(t *testing.T) {
			value := time.Unix(seconds, 0)
			if _, err := (at.PayloadBuilder{CreatedAt: value}).Build(); err == nil {
				t.Fatal("local producer accepted unsupported timestamp")
			}
			defer func() {
				rejection := recover()
				err, ok := rejection.(error)
				if !ok || !strings.Contains(err.Error(), "seconds: outside supported timestamp range") {
					t.Errorf("encoder rejection = %v, want timestamp range error", rejection)
				}
			}()
			at.TimeConv.Encode(value)
		})
	}
	if _, err := at.NewProposal(r92Leader(c, keys, at.View{}), c, at.ViewSpec{}, time.Unix(253402300800, 0), nil, utils.None[*at.AppQC]()); err == nil {
		t.Fatal("local proposal producer accepted unsupported timestamp")
	}
}
