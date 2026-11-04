package ecdsa

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/getamis/alice/crypto/elliptic"
	"github.com/getamis/alice/crypto/tss/ecdsa/cggmp/dkg"
	"github.com/getamis/alice/crypto/tss/ecdsa/cggmp/sign"
	"github.com/getamis/alice/types"
	"github.com/getamis/alice/types/message"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

type repairM129Frame struct {
	from string
	msg  types.Message
}

type repairM129Recorder struct {
	mu     sync.Mutex
	frames []repairM129Frame
}

type repairM129Network struct {
	*memPeer
	t        *testing.T
	recorder *repairM129Recorder
}

func repairM129Decode(t *testing.T, msg types.Message) types.Message {
	t.Helper()
	pm := msg.(proto.Message)
	payload, err := proto.Marshal(pm)
	if err != nil {
		t.Errorf("encode protocol message: %v", err)
		return nil
	}
	decoded := pm.ProtoReflect().New().Interface()
	if err := proto.Unmarshal(payload, decoded); err != nil {
		t.Errorf("decode protocol message: %v", err)
		return nil
	}
	return decoded.(types.Message)
}

func (n *repairM129Network) MustSend(to string, msg interface{}) {
	decoded := repairM129Decode(n.t, msg.(types.Message))
	if decoded == nil {
		return
	}
	n.recorder.mu.Lock()
	n.recorder.frames = append(n.recorder.frames, repairM129Frame{from: n.SelfID(), msg: decoded})
	n.recorder.mu.Unlock()
	n.memPeer.MustSend(to, decoded)
}

var repairM129TranscriptOnce sync.Once
var repairM129Recorded []repairM129Frame

func repairM129Transcript(t *testing.T) []repairM129Frame {
	t.Helper()
	repairM129TranscriptOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		hub := newMemHub()
		recorder := &repairM129Recorder{}
		routers := make([]*router, len(participantIDs))
		cores := make([]*dkg.DKG, len(participantIDs))
		listeners := make([]*listener, len(participantIDs))
		for i, id := range participantIDs {
			net := &repairM129Network{memPeer: hub.peer(id, participantIDs), t: t, recorder: recorder}
			routers[i] = newRouter(net, phaseDKG)
			defer routers[i].close()
			net.Bind(routers[i])
			listeners[i] = newListener()
			var err error
			cores[i], err = dkg.NewDKG(elliptic.Secp256k1(), net, []byte("repair-m129-transcript"), Threshold, 0, listeners[i])
			if err != nil {
				t.Fatal(err)
			}
			if err := routers[i].attach(phaseDKG, cores[i]); err != nil {
				t.Fatal(err)
			}
		}
		finished := make(chan error, len(cores))
		for i := range cores {
			go func(i int) { finished <- routers[i].run(ctx, cores[i], listeners[i], cores[i].Start) }(i)
		}
		for range cores {
			if err := <-finished; err != nil {
				t.Fatalf("real DKG producer/codec/receiver/consumer: %v", err)
			}
		}
		for i, core := range cores {
			result, err := core.GetResult()
			if err != nil {
				t.Fatal(err)
			}
			partials, err := routers[i].dkgPartialPublicKeys(participantIDs[i], result.Share, result.Bks, result.PublicKey)
			if err != nil || len(partials) != Participants {
				t.Fatalf("real DKG result lost partial public keys: %v, %d", err, len(partials))
			}
		}
		recorder.mu.Lock()
		repairM129Recorded = append([]repairM129Frame(nil), recorder.frames...)
		recorder.mu.Unlock()
	})
	return repairM129Recorded
}

func repairM129Original(t *testing.T, origin string, round dkg.Type) types.Message {
	t.Helper()
	for _, frame := range repairM129Transcript(t) {
		m := frame.msg.(*dkg.Message)
		if frame.from == origin && m.GetId() == origin && m.GetType() == round && m.GetEchoHashRelay() == nil {
			return repairM129Decode(t, m)
		}
	}
	t.Fatalf("real producer did not emit %s round %v", origin, round)
	return nil
}

type repairM129Listener struct {
	*listener
	stopped chan struct{}
	once    sync.Once
}

func (l *repairM129Listener) OnStateChanged(old, state types.MainState) {
	l.listener.OnStateChanged(old, state)
	if state == types.StateFailed {
		l.once.Do(func() { close(l.stopped) })
	}
}

func repairM129Core(t *testing.T, initial phase) (*router, *dkg.DKG, *repairM129Listener) {
	t.Helper()
	hub := newMemHub()
	net := hub.peer(participantIDs[0], participantIDs)
	r := newRouter(net, initial)
	net.Bind(r)
	l := &repairM129Listener{listener: newListener(), stopped: make(chan struct{})}
	core, err := dkg.NewDKG(elliptic.Secp256k1(), net, []byte("repair-m129-transcript"), Threshold, 0, l)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.close)
	t.Cleanup(core.Stop)
	return r, core, l
}

func repairM129CleanFailure(t *testing.T, r *router, err, cause error) {
	t.Helper()
	if !errors.Is(err, ErrSessionFailed) || !errors.Is(err, cause) {
		t.Fatalf("failure %v does not preserve session failure and %v", err, cause)
	}
	select {
	case <-r.failed:
	default:
		t.Fatal("ceremony was not cancelled")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != err || len(r.pending) != 0 || len(r.budgets) != 0 || len(r.seen) != 0 || len(r.mains) != 0 || len(r.siG) != 0 || len(r.conflict) != 0 {
		t.Fatalf("failed ceremony retained state: %#v", r)
	}
}

func TestRepairM_129_RealDKG(t *testing.T) {
	for _, round := range []dkg.Type{dkg.Type_Peer, dkg.Type_Decommit, dkg.Type_Verify, dkg.Type_Result} {
		if msg := repairM129Original(t, participantIDs[1], round); !msg.IsValid() {
			t.Fatalf("producer emitted invalid message for round %v", round)
		}
	}
}

func TestRepairM_129_PendingDuplicateAndRelay(t *testing.T) {
	r, _, _ := repairM129Core(t, phaseDKG)
	original := repairM129Original(t, participantIDs[1], dkg.Type_Peer)
	if err := r.AddMessage(participantIDs[1], original); err != nil {
		t.Fatal(err)
	}
	foundRelay := false
	for _, frame := range repairM129Transcript(t) {
		m := frame.msg.(*dkg.Message)
		if frame.from == participantIDs[1] && m.GetId() == participantIDs[2] && m.GetType() == dkg.Type_Peer && m.GetEchoHashRelay() != nil {
			if err := r.AddMessage(frame.from, repairM129Decode(t, m)); err != nil {
				t.Fatalf("legitimate echo relay rejected: %v", err)
			}
			foundRelay = true
			break
		}
	}
	if !foundRelay || len(r.pending[phaseDKG]) != 2 {
		t.Fatal("origin-specific relay slots were not admitted")
	}
	err := r.AddMessage(participantIDs[1], repairM129Decode(t, original))
	repairM129CleanFailure(t, r, err, ErrDuplicateMessage)
	if again := r.AddMessage(participantIDs[2], repairM129Original(t, participantIDs[2], dkg.Type_Peer)); again != err {
		t.Fatalf("failure was replaced: %v", again)
	}
}

func repairM129Padded(t *testing.T, msg types.Message, size int) types.Message {
	t.Helper()
	pm := proto.Clone(msg.(proto.Message))
	unknown := protowire.AppendTag(nil, 1000, protowire.BytesType)
	unknown = protowire.AppendBytes(unknown, make([]byte, size-proto.Size(pm)-5))
	pm.ProtoReflect().SetUnknown(unknown)
	decoded := repairM129Decode(t, pm.(types.Message))
	if got := proto.Size(decoded.(proto.Message)); got != size {
		t.Fatalf("padded wire size %d, want %d", got, size)
	}
	return decoded
}

func TestRepairM_129_PendingParticipantBudget(t *testing.T) {
	r, _, _ := repairM129Core(t, phaseDKG)
	size := maxPendingBytesPerPeer/2 + 1
	for _, id := range participantIDs[1:3] {
		msg := repairM129Padded(t, repairM129Original(t, id, dkg.Type_Peer), size)
		if err := r.AddMessage(id, msg); err != nil {
			t.Fatalf("independent participant budget: %v", err)
		}
	}
	if len(r.pending[phaseDKG]) != 2 || r.budgets[phaseDKG][participantIDs[1]].bytes != size || r.budgets[phaseDKG][participantIDs[2]].bytes != size {
		t.Fatal("pending budget did not account for each participant separately")
	}
	msg := repairM129Padded(t, repairM129Original(t, participantIDs[1], dkg.Type_Decommit), size)
	err := r.AddMessage(participantIDs[1], msg)
	repairM129CleanFailure(t, r, err, ErrPendingLimit)
}

func TestRepairM_129_UnexpectedPhaseAndParticipant(t *testing.T) {
	original := repairM129Original(t, participantIDs[1], dkg.Type_Peer)
	for _, tc := range []struct {
		name    string
		initial phase
		sender  string
		msg     types.Message
	}{
		{"keygen_during_sign", phaseSign, participantIDs[1], original},
		{"sign_during_keygen", phaseDKG, participantIDs[1], &sign.Message{Id: participantIDs[1]}},
		{"unknown_sender", phaseDKG, "outsider", original},
		{"self_sender", phaseDKG, participantIDs[0], original},
		{"nil_message", phaseDKG, participantIDs[1], nil},
		{"typed_nil_message", phaseDKG, participantIDs[1], (*dkg.Message)(nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, _, _ := repairM129Core(t, tc.initial)
			err := r.AddMessage(tc.sender, tc.msg)
			repairM129CleanFailure(t, r, err, ErrUnexpectedMessage)
		})
	}
}

func TestRepairM_129_QueuedMessageOwnership(t *testing.T) {
	r, core, _ := repairM129Core(t, phaseDKG)
	original := repairM129Original(t, participantIDs[1], dkg.Type_Peer).(*dkg.Message)
	if err := r.AddMessage(participantIDs[1], original); err != nil {
		t.Fatal(err)
	}
	original.Id = "mutated-after-delivery"
	if err := r.attach(phaseDKG, core); err != nil {
		t.Fatalf("retained message changed after enqueue: %v", err)
	}
	if len(r.pending) != 0 || len(r.budgets) != 0 {
		t.Fatal("successful replay retained pending storage")
	}
}

func TestRepairM_129_ReplayAndDirectRejection(t *testing.T) {
	// The Verify message is not echo tracked. A different participant relaying it
	// is rejected by the real EchoMsgMain, rather than by a test receiver.
	invalid := repairM129Original(t, participantIDs[1], dkg.Type_Verify)
	for _, buffered := range []bool{true, false} {
		name := "direct"
		if buffered {
			name = "buffered"
		}
		t.Run(name, func(t *testing.T) {
			r, core, l := repairM129Core(t, phaseDKG)
			var err error
			if buffered {
				if err := r.AddMessage(participantIDs[2], repairM129Decode(t, invalid)); err != nil {
					t.Fatalf("pre-attachment delivery: %v", err)
				}
				if err := r.AddMessage(participantIDs[3], repairM129Original(t, participantIDs[3], dkg.Type_Peer)); err != nil {
					t.Fatal(err)
				}
				err = r.attach(phaseDKG, core)
			} else {
				if err := r.attach(phaseDKG, core); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				started := make(chan struct{})
				finished := make(chan error, 1)
				go func() { finished <- r.run(ctx, core, l.listener, func() { core.Start(); close(started) }) }()
				<-started
				err = r.AddMessage(participantIDs[2], repairM129Decode(t, invalid))
				select {
				case runErr := <-finished:
					if runErr != err {
						t.Fatalf("ceremony returned %v, want first rejection %v", runErr, err)
					}
				case <-ctx.Done():
					t.Fatal("receiver rejection left ceremony waiting for timeout")
				}
				select {
				case <-l.stopped:
				case <-ctx.Done():
					t.Fatal("rejected ceremony did not stop its core")
				}
			}
			repairM129CleanFailure(t, r, err, message.ErrInvalidRelay)
			if runErr := r.run(context.Background(), core, l.listener, core.Start); runErr != err {
				t.Fatalf("failed phase restarted: %v", runErr)
			}
			if attachErr := r.attach(phaseDKG, core); attachErr != err {
				t.Fatalf("failed phase reattached: %v", attachErr)
			}
		})
	}
}

func TestRepairM_129_PublicKeygenCancellation(t *testing.T) {
	// Keygen binds the real receiver before constructing the core. The test's
	// in-memory network delivers an oversized recorded protocol message to it.
	original := repairM129Original(t, participantIDs[1], dkg.Type_Peer)
	oversized := repairM129Padded(t, original, maxPendingBytesPerPeer+1)
	hub := newMemHub()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		_, err := Keygen(ctx, hub.peer(participantIDs[0], participantIDs), []byte("repair-m129-cancel"))
		finished <- err
	}()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	var receiver Receiver
	for receiver == nil {
		hub.mu.Lock()
		receiver = hub.receivers[participantIDs[0]]
		hub.mu.Unlock()
		if receiver != nil {
			break
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal("Keygen did not bind its receiver")
		}
	}
	err := receiver.AddMessage(participantIDs[1], repairM129Decode(t, oversized))
	repairM129CleanFailure(t, receiver.(*router), err, ErrPendingLimit)
	select {
	case runErr := <-finished:
		if !errors.Is(runErr, ErrPendingLimit) || !errors.Is(runErr, ErrSessionFailed) || errors.Is(runErr, context.DeadlineExceeded) {
			t.Fatalf("Keygen did not propagate immediate typed cancellation: %v", runErr)
		}
	case <-ctx.Done():
		t.Fatal("Keygen waited for timeout after pending limit failure")
	}
}
