package layerxpay

import (
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

type payEventCase struct {
	name      string
	args      []interface{}
	record    any
	want      []interface{}
	secondary *[32]byte
}

func payEventCases() []payEventCase {
	id, other, buyer, asset := [32]byte{1}, [32]byte{2}, [32]byte{3}, [32]byte{4}
	amount := func(n int64) *big.Int { return big.NewInt(n) }
	escrow := paytypes.EscrowRecord{EscrowID: id, State: 4, CapturedAmount: codec.U128{Hi: 1, Lo: 9}, LockedAmount: codec.U128{Hi: 2, Lo: 17}}
	captured := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 64), big.NewInt(9))
	locked := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(2), 64), big.NewInt(17))
	budget := paytypes.BudgetRecord{BudgetID: id, AssetID: asset, PerPeriodLimit: codec.U128{Lo: 123}, ExpiryMs: 456, RevocationSequence: 789}
	stream := paytypes.StreamRecord{StreamID: id, Payer: buyer, StreamAccount: other, Recipient: id, AssetID: asset, Mode: 2, Rate: codec.U128{Lo: 19}, LastAccrualTimestamp: 654, AccruedTotal: codec.U128{Lo: 101}, SettledTotal: codec.U128{Lo: 70}, CumulativeMeter: 876, Underfunded: false}
	before := stream
	before.Underfunded = true
	before.Rate = codec.U128{Lo: 13}
	before.AccruedTotal = codec.U128{Lo: 61}
	result := paytypes.StreamResult{StreamID: id, Paid: codec.U128{Lo: 31}, Refunded: codec.U128{Lo: 11}, Ordinal: 7, LegCount: 2}
	change := payStreamChangeRecord{Before: before, After: stream, Result: result}
	offer := paytypes.ServiceOffer{OfferID: id, OfferingAgent: other, DefaultOutcome: 2, GlobalSequence: 321, Withdrawn: true}
	agreement := paytypes.ServiceAgreement{AgreementID: id, Provider: other, Buyer: buyer, State: 4, AcceptedSequence: 432, OutcomeSequence: 543, OutcomeTimestampMs: 654, DefaultApplied: true, ContestedHashes: [][32]byte{asset, buyer}}
	commitment := paytypes.ServiceCommitment{CommitmentID: id, AgreementID: other, GlobalSequence: 765, Abandoned: true}
	dispute := paytypes.ServiceDispute{DisputeID: id, AgreementID: other, GlobalSequence: 876, ResolutionSequence: 987, Resolved: true, EvidenceHashes: [][32]byte{asset, buyer, id}}
	cases := []payEventCase{}
	for i, name := range []string{"EscrowOpened", "EscrowCaptured", "EscrowPartiallyCaptured", "EscrowReleased", "EscrowTimedOut", "EscrowDisputed", "EscrowResolved"} {
		cases = append(cases, payEventCase{name: name, args: []interface{}{id}, record: escrow, want: []interface{}{uint16(i + 1), uint8(4), captured, locked}})
	}
	cases = append(cases,
		payEventCase{name: "BudgetCreated", args: []interface{}{id, other, asset, buyer, amount(123), amount(0), amount(90)}, record: budget, want: []interface{}{asset, amount(90)}},
		payEventCase{name: "BudgetFunded", args: []interface{}{id, amount(91)}, record: budget, want: []interface{}{amount(91)}},
		payEventCase{name: "BudgetAmended", args: []interface{}{id, amount(999), amount(0), uint64(999)}, record: budget, want: []interface{}{amount(123), uint64(456)}},
		payEventCase{name: "BudgetDelegateAdded", args: []interface{}{id, other}, record: budget, want: []interface{}{other}},
		payEventCase{name: "BudgetDelegateRemoved", args: []interface{}{id, other}, record: budget, want: []interface{}{other}},
		payEventCase{name: "BudgetSpent", args: []interface{}{id, other, amount(92)}, record: budget, want: []interface{}{other, amount(92)}},
		payEventCase{name: "BudgetClosed", args: []interface{}{id, uint64(788)}, record: payBudgetDrainRecord{Budget: budget, Drained: codec.U128{Lo: 93}}, want: []interface{}{amount(93), uint64(789)}},
		payEventCase{name: "BudgetDefunded", args: []interface{}{id, amount(94)}, record: budget, want: []interface{}{amount(94), amount(123)}},
		payEventCase{name: "BudgetRevoked", args: []interface{}{id, uint64(788)}, record: payBudgetDrainRecord{Budget: budget, Drained: codec.U128{Lo: 95}}, want: []interface{}{amount(95), uint64(789)}},
		payEventCase{name: "StreamOpened", args: []interface{}{id, other, id, asset, uint8(2), amount(19), uint64(1), uint64(2), uint64(3), amount(200), amount(96)}, record: stream, want: []interface{}{buyer, other, id, asset, uint8(2), amount(96)}},
		payEventCase{name: "StreamToppedUp", args: []interface{}{id, amount(97)}, record: change, want: []interface{}{amount(97), true}},
		payEventCase{name: "StreamMetered", args: []interface{}{id, uint64(999)}, record: change, want: []interface{}{uint64(876), amount(40), amount(101)}},
		payEventCase{name: "StreamSettled", args: []interface{}{id, other}, record: change, want: []interface{}{amount(31), amount(70), false}},
		payEventCase{name: "StreamPaused", args: []interface{}{id}, record: stream, want: []interface{}{uint64(654), amount(101)}},
		payEventCase{name: "StreamResumed", args: []interface{}{id}, record: stream, want: []interface{}{uint64(654), amount(101)}},
		payEventCase{name: "StreamClosed", args: []interface{}{id, other}, record: result, want: []interface{}{amount(31), amount(11)}},
		payEventCase{name: "StreamRateUpdated", args: []interface{}{id, amount(19)}, record: change, want: []interface{}{amount(13), amount(19), amount(31)}},
		payEventCase{name: "OfferPublished", args: []interface{}{id}, record: offer, want: []interface{}{other, uint8(2), uint64(321)}, secondary: &other},
		payEventCase{name: "OfferWithdrawn", args: []interface{}{id}, record: payServiceSequence[paytypes.ServiceOffer]{Record: offer, GlobalSequence: 322}, want: []interface{}{other, uint8(1), uint64(322)}, secondary: &other},
		payEventCase{name: "AgreementProposed", args: []interface{}{id}, record: agreement, want: []interface{}{other, uint8(4), uint64(432)}, secondary: &other},
		payEventCase{name: "AgreementAccepted", args: []interface{}{id}, record: agreement, want: []interface{}{buyer, uint8(4), uint64(432)}, secondary: &buyer},
		payEventCase{name: "TaskCommitted", args: []interface{}{id}, record: commitment, want: []interface{}{other, uint8(1), uint64(765)}, secondary: &other},
		payEventCase{name: "CommitAbandoned", args: []interface{}{id}, record: payServiceSequence[paytypes.ServiceCommitment]{Record: commitment, GlobalSequence: 766}, want: []interface{}{other, uint8(1), uint64(766)}, secondary: &other},
		payEventCase{name: "ExecutionAttested", args: []interface{}{[]byte{1, 2, 3}}, record: paytypes.ServiceExecution{AttestationID: id, CommitmentID: other, GlobalSequence: 678}, want: []interface{}{other, uint8(0), uint64(678)}, secondary: &other},
		payEventCase{name: "ProgressReported", args: []interface{}{id}, record: paytypes.ServiceProgress{ReportID: id, CommitmentID: other, ProgressBPS: 10000, GlobalSequence: 789}, want: []interface{}{other, uint8(1), uint64(789)}, secondary: &other},
		payEventCase{name: "Delivered", args: []interface{}{id}, record: paytypes.ServiceDelivery{AgreementID: other, Deliverables: make([]paytypes.ServiceDeliverable, 2), GlobalSequence: 890}, want: []interface{}{other, uint8(2), uint64(890)}, secondary: &other},
		payEventCase{name: "OutcomeAccepted", args: []interface{}{id}, record: agreement, want: []interface{}{other, uint8(4), uint64(543)}, secondary: &other},
		payEventCase{name: "OutcomeRejected", args: []interface{}{id}, record: agreement, want: []interface{}{other, uint8(2), uint64(543)}, secondary: &other},
		payEventCase{name: "DisputeOpened", args: []interface{}{id}, record: dispute, want: []interface{}{other, uint8(3), uint64(876)}, secondary: &other},
		payEventCase{name: "DisputeResolved", args: []interface{}{id}, record: dispute, want: []interface{}{other, uint8(1), uint64(987)}, secondary: &other},
		payEventCase{name: "DefaultOutcomeApplied", args: []interface{}{id}, record: agreement, want: []interface{}{uint8(4), true, uint64(543), uint64(654)}},
	)
	return cases
}

func payEventTestEVM() (*vm.EVM, *state.DBImpl) {
	app := testkeeper.EVMTestApp
	ctx := app.GetContextForDeliverTx(nil).WithEventManager(sdk.NewEventManager())
	db := state.NewDBImpl(ctx, &app.EvmKeeper, false)
	return &vm.EVM{StateDB: db}, db
}

func TestPayEventTopicsMatchABI(t *testing.T) {
	parsed := pcommon.MustGetABI(abiTestFS, "abi.json")
	require.Len(t, parsed.Events, 38)
	require.Len(t, payEvents, 38)
	cases := payEventCases()
	require.Len(t, cases, 38)
	evm, db := payEventTestEVM()
	seen := map[string]bool{}
	singles, services := 0, 0
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.False(t, seen[tc.name], "duplicate fixture")
			seen[tc.name] = true
			packer, ok := payEvents[tc.name]
			require.True(t, ok)
			require.Equal(t, tc.name, packer.name)
			event, ok := parsed.Events[tc.name]
			require.True(t, ok)
			values, err := packer.pack(tc.args, tc.record)
			require.NoError(t, err)
			require.Equal(t, tc.want, values)
			id := [32]byte{1}
			start := len(db.GetAllLogs())
			require.NoError(t, emitPayEvent(evm, tc.name, id, values))
			logs := db.GetAllLogs()
			require.Len(t, logs, start+1)
			log := logs[start]
			topics := []common.Hash{crypto.Keccak256Hash([]byte(event.Sig)), common.Hash(id)}
			wantData := tc.want
			require.True(t, event.Inputs[0].Indexed)
			require.Equal(t, "bytes32", event.Inputs[0].Type.String())
			if tc.secondary != nil {
				services++
				require.True(t, event.Inputs[1].Indexed)
				require.Equal(t, "primary", event.Inputs[0].Name)
				require.Equal(t, "secondary", event.Inputs[1].Name)
				topics = append(topics, common.Hash(*tc.secondary))
				wantData = wantData[1:]
			} else {
				singles++
				require.False(t, event.Inputs[1].Indexed)
			}
			for _, input := range event.Inputs[len(topics)-1:] {
				require.False(t, input.Indexed)
			}
			require.Equal(t, event.ID, log.Topics[0])
			require.Equal(t, topics, log.Topics)
			data, err := event.Inputs.NonIndexed().Pack(wantData...)
			require.NoError(t, err)
			require.Equal(t, data, log.Data)
			decoded, err := event.Inputs.NonIndexed().Unpack(log.Data)
			require.NoError(t, err)
			require.Equal(t, wantData, decoded)
			_, err = packer.pack(nil, tc.record)
			require.Error(t, err, "missing call arguments")
		})
	}
	require.Len(t, seen, 38)
	require.Equal(t, 25, singles)
	require.Equal(t, 13, services)
}

func TestPayEventEmittedAtPayAddress(t *testing.T) {
	evm, db := payEventTestEVM()
	id := [32]byte{0xaa}
	record := paytypes.EscrowRecord{EscrowID: id, State: 3, CapturedAmount: codec.U128{Lo: 44}, LockedAmount: codec.U128{Lo: 56}}
	args := []interface{}{id, big.NewInt(1), [32]byte{0xbb}}
	values, err := payEvents["EscrowCaptured"].pack(args, record)
	require.NoError(t, err)
	require.NoError(t, emitPayEvent(evm, "EscrowCaptured", id, values))
	logs := db.GetAllLogs()
	require.Len(t, logs, 1)
	require.Equal(t, common.HexToAddress("0x000000000000000000000000000000000000101B"), logs[0].Address)
	require.Equal(t, []common.Hash{crypto.Keccak256Hash([]byte("EscrowCaptured(bytes32,uint16,uint8,uint128,uint128)")), common.Hash(id)}, logs[0].Topics)
	decoded, err := gasABI.Events["EscrowCaptured"].Inputs.NonIndexed().Unpack(logs[0].Data)
	require.NoError(t, err)
	require.Equal(t, []interface{}{uint16(2), uint8(3), big.NewInt(44), big.NewInt(56)}, decoded)
	require.Equal(t, big.NewInt(1), args[1], "packing must not change call arguments")
	snapshot := db.Snapshot()
	require.NoError(t, emitPayEvent(evm, "EscrowCaptured", id, values))
	require.Len(t, db.GetAllLogs(), 2)
	db.RevertToSnapshot(snapshot)
	require.Len(t, db.GetAllLogs(), 1, "EVM rollback must discard emitted logs")
	for _, tc := range []struct {
		name   string
		values []interface{}
	}{
		{"Unknown", nil},
		{"EscrowCaptured", nil},
		{"EscrowCaptured", []interface{}{uint16(2), uint8(3), "invalid", big.NewInt(56)}},
		{"OfferPublished", []interface{}{[]byte{1}, uint8(2), uint64(3)}},
		{"OfferPublished", []interface{}{[32]byte{1}, uint8(2)}},
	} {
		require.Error(t, emitPayEvent(evm, tc.name, id, tc.values))
		require.Len(t, db.GetAllLogs(), 1, "invalid event must not emit a log")
	}
	require.Error(t, emitPayEvent(nil, "EscrowCaptured", id, values))
	require.Error(t, emitPayEvent(&vm.EVM{}, "EscrowCaptured", id, values))
	_, err = payEvents["EscrowCaptured"].pack(args, struct{}{})
	require.Error(t, err, "a receipt without a post-call escrow record is insufficient")
	_, err = payEvents["BudgetFunded"].pack([]interface{}{id, "invalid"}, nil)
	require.Error(t, err)
	for _, name := range []string{"BudgetClosed", "BudgetRevoked", "StreamToppedUp", "StreamSettled", "StreamRateUpdated"} {
		_, err = payEvents[name].pack(args, nil)
		require.Error(t, err, "missing event context must not invent zero amounts")
	}
	after := paytypes.StreamRecord{AccruedTotal: codec.U128{Lo: 1}}
	before := paytypes.StreamRecord{AccruedTotal: codec.U128{Lo: 2}}
	_, err = payEvents["StreamMetered"].pack(args, payStreamChangeRecord{Before: before, After: after})
	require.Error(t, err, "accrual underflow must fail")
	progress := paytypes.ServiceProgress{CommitmentID: [32]byte{2}, ProgressBPS: 9999, GlobalSequence: 123}
	partial, err := payEvents["ProgressReported"].pack(args, progress)
	require.NoError(t, err)
	require.Equal(t, []interface{}{progress.CommitmentID, uint8(0), uint64(123)}, partial)
	commitment := paytypes.ServiceCommitment{AgreementID: [32]byte{3}, Abandoned: false, GlobalSequence: 456}
	committed, err := payEvents["TaskCommitted"].pack(args, commitment)
	require.NoError(t, err)
	require.Equal(t, []interface{}{commitment.AgreementID, uint8(0), uint64(456)}, committed)
}
