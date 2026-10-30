package layerxpay_test

import (
	"crypto/ed25519"
	"encoding/binary"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

type serviceTestFixture struct {
	*escrowTestFixture
	offer, agreement, commitment, escrow, terms, spec [32]byte
	deadline                                          uint64
}

type serviceTestDeliverable struct {
	Hash                  [32]byte
	ArtifactSizeBytes     uint64
	AvailabilityReference [32]byte
}

func newServiceTestFixture(t *testing.T) *serviceTestFixture {
	t.Helper()
	f := &serviceTestFixture{escrowTestFixture: newEscrowTestFixture(t),
		offer: [32]byte{0x81}, agreement: [32]byte{0x82}, commitment: [32]byte{0x83},
		escrow: [32]byte{0x84}, terms: [32]byte{0x85}, spec: [32]byte{0x86}}
	f.h.keeper.SetParams(f.h.db.Ctx(), paytypes.Params{EscrowEnabled: true, ServiceEnabled: true})
	f.deadline = uint64(f.h.db.Ctx().BlockTime().UnixMilli()) + 1000
	args := f.openArgs(t, f.escrow)
	args[7] = f.deadline + 10000
	args[9], args[10] = f.terms, f.agreement
	a := f.h.precompile.GetABI()
	input, err := a.Pack("escrowOpen", args...)
	require.NoError(t, err)
	output, _, err := f.h.evm.Call(f.owner.caller, f.h.precompile.Address(), input, 2000000, uint256.NewInt(0))
	require.NoError(t, err, "%x", output)
	return f
}

func serviceTestCall(t *testing.T, f *serviceTestFixture, caller common.Address, name string, args ...interface{}) *ethtypes.Log {
	t.Helper()
	h := f.h
	a := h.precompile.GetABI()
	input, err := a.Pack(name, args...)
	require.NoError(t, err)
	before := h.keeper.ExportGenesis(h.db.Ctx())
	logCount := len(h.db.GetAllLogs())
	ordinalKey := crypto.Keccak256Hash([]byte("layerxpay/call-ordinal/v1"))
	ordinal := h.db.GetTransientState(h.precompile.Address(), ordinalKey).Big().Uint64()
	output, _, err := h.evm.StaticCall(caller, h.precompile.Address(), input, 2000000)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, err := abi.UnpackRevert(output)
	require.NoError(t, err)
	require.Contains(t, reason, "LXP_UNAUTHORIZED_DEBIT")
	require.Equal(t, before, h.keeper.ExportGenesis(h.db.Ctx()))
	require.Len(t, h.db.GetAllLogs(), logCount)
	require.Equal(t, ordinal, h.db.GetTransientState(h.precompile.Address(), ordinalKey).Big().Uint64())
	output, _, err = h.evm.Call(caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
	require.NoError(t, err, "%s: %x", name, output)
	require.Empty(t, output)
	require.Equal(t, ordinal+1, h.db.GetTransientState(h.precompile.Address(), ordinalKey).Big().Uint64())
	logs := h.db.GetAllLogs()
	require.Len(t, logs, logCount+1)
	return logs[logCount]
}

func serviceTestEvent(t *testing.T, f *serviceTestFixture, log *ethtypes.Log, name string, id, secondary [32]byte, code uint8, sequence uint64) {
	t.Helper()
	a := f.h.precompile.GetABI()
	event := a.Events[name]
	require.Equal(t, f.h.precompile.Address(), log.Address)
	require.Equal(t, []common.Hash{event.ID, common.Hash(id), common.Hash(secondary)}, log.Topics)
	values, err := event.Inputs.NonIndexed().Unpack(log.Data)
	require.NoError(t, err)
	require.NotZero(t, sequence)
	require.Equal(t, []interface{}{code, sequence}, values)
}

func serviceTestForm(t *testing.T, f *serviceTestFixture, defaultOutcome uint8) {
	t.Helper()
	log := serviceTestCall(t, f, f.beneficiary.caller, "serviceOfferPublish", f.offer, f.asset, big.NewInt(100), f.terms, f.spec, f.deadline, uint64(1000), uint64(3000), defaultOutcome, f.deadline)
	offer, found := f.h.keeper.GetOffer(f.h.db.Ctx(), f.offer)
	require.True(t, found)
	require.Equal(t, f.beneficiary.account, offer.OfferingAgent)
	require.Equal(t, f.asset, offer.AssetID)
	require.Equal(t, codec.U128{Lo: 100}, offer.Price)
	require.Equal(t, f.terms, offer.TermsHash)
	require.Equal(t, f.spec, offer.DeliverableSpecHash)
	require.Equal(t, defaultOutcome, offer.DefaultOutcome)
	serviceTestEvent(t, f, log, "OfferPublished", f.offer, offer.OfferingAgent, defaultOutcome, offer.GlobalSequence)
	log = serviceTestCall(t, f, f.owner.caller, "serviceAgreementPropose", f.agreement, f.offer, f.terms, f.escrow)
	agreement, found := f.h.keeper.GetAgreement(f.h.db.Ctx(), f.agreement)
	require.True(t, found)
	require.Equal(t, f.beneficiary.account, agreement.Provider)
	require.Equal(t, f.owner.account, agreement.Buyer)
	require.Equal(t, f.escrow, agreement.EscrowID)
	require.Equal(t, f.terms, agreement.TermsHash)
	require.Equal(t, uint8(8), agreement.State)
	serviceTestEvent(t, f, log, "AgreementProposed", f.agreement, agreement.Provider, 8, agreement.AcceptedSequence)
	log = serviceTestCall(t, f, f.beneficiary.caller, "serviceAgreementAccept", f.agreement)
	agreement, found = f.h.keeper.GetAgreement(f.h.db.Ctx(), f.agreement)
	require.True(t, found)
	require.Equal(t, uint8(1), agreement.State)
	serviceTestEvent(t, f, log, "AgreementAccepted", f.agreement, agreement.Buyer, 1, agreement.AcceptedSequence)
	offer, found = f.h.keeper.GetOffer(f.h.db.Ctx(), f.offer)
	require.True(t, found)
	require.True(t, offer.Accepted)
}

func serviceTestCommit(t *testing.T, f *serviceTestFixture) {
	t.Helper()
	log := serviceTestCall(t, f, f.beneficiary.caller, "serviceCommitTask", f.commitment, f.agreement, [32]byte{0x87}, f.escrow, f.deadline, uint64(1000))
	commitment, found := f.h.keeper.GetCommitment(f.h.db.Ctx(), f.commitment)
	require.True(t, found)
	require.Equal(t, f.beneficiary.account, commitment.Provider)
	require.Equal(t, f.escrow, commitment.EscrowID)
	require.Equal(t, [32]byte{0x87}, commitment.TaskHash)
	require.Equal(t, f.deadline, commitment.DeadlineMs)
	require.Equal(t, uint64(1000), commitment.ResourceBound)
	require.False(t, commitment.Abandoned)
	serviceTestEvent(t, f, log, "TaskCommitted", f.commitment, f.agreement, 0, commitment.GlobalSequence)
}

func serviceTestDeliver(t *testing.T, f *serviceTestFixture) {
	t.Helper()
	id := [32]byte{0x88}
	items := []serviceTestDeliverable{{f.spec, 4096, [32]byte{0x89}}, {f.spec, 1024, [32]byte{0x8a}}}
	log := serviceTestCall(t, f, f.beneficiary.caller, "serviceDeliver", id, f.agreement, items)
	delivery, found := f.h.keeper.GetDelivery(f.h.db.Ctx(), id)
	require.True(t, found)
	require.Equal(t, f.beneficiary.account, delivery.Provider)
	require.Equal(t, []paytypes.ServiceDeliverable{{Hash: f.spec, ArtifactSize: 4096, AvailabilityReference: [32]byte{0x89}}, {Hash: f.spec, ArtifactSize: 1024, AvailabilityReference: [32]byte{0x8a}}}, delivery.Deliverables)
	serviceTestEvent(t, f, log, "Delivered", id, f.agreement, 2, delivery.GlobalSequence)
}

func TestPrecompileServiceHappyPath(t *testing.T) {
	f := newServiceTestFixture(t)
	serviceTestForm(t, f, 1)
	serviceTestCommit(t, f)
	for i, bps := range []uint32{5000, 10000} {
		id := [32]byte{0x90 + byte(i)}
		log := serviceTestCall(t, f, f.beneficiary.caller, "serviceProgressReport", id, f.commitment, [32]byte{0x92}, [32]byte{0x93}, bps)
		progress, found := f.h.keeper.GetProgress(f.h.db.Ctx(), id)
		require.True(t, found)
		require.Equal(t, bps, progress.ProgressBPS)
		require.Equal(t, [32]byte{0x92}, progress.NoteHash)
		require.Equal(t, [32]byte{0x93}, progress.AvailabilityReference)
		serviceTestEvent(t, f, log, "ProgressReported", id, f.commitment, uint8(i), progress.GlobalSequence)
	}
	serviceTestDeliver(t, f)
	log := serviceTestCall(t, f, f.owner.caller, "serviceAccept", f.agreement)
	agreement, found := f.h.keeper.GetAgreement(f.h.db.Ctx(), f.agreement)
	require.True(t, found)
	require.Equal(t, uint8(4), agreement.State)
	require.False(t, agreement.DefaultApplied)
	require.Equal(t, uint64(f.h.db.Ctx().BlockTime().UnixMilli()), agreement.OutcomeTimestampMs)
	serviceTestEvent(t, f, log, "OutcomeAccepted", f.agreement, agreement.Provider, 4, agreement.OutcomeSequence)
}

func TestPrecompileServiceDisputePath(t *testing.T) {
	f := newServiceTestFixture(t)
	serviceTestForm(t, f, 2)
	serviceTestCommit(t, f)
	serviceTestDeliver(t, f)
	log := serviceTestCall(t, f, f.owner.caller, "serviceReject", f.agreement, uint16(7), [][32]byte{f.spec})
	agreement, found := f.h.keeper.GetAgreement(f.h.db.Ctx(), f.agreement)
	require.True(t, found)
	require.Equal(t, uint8(5), agreement.State)
	require.Equal(t, uint16(7), agreement.RejectionReason)
	require.Equal(t, [][32]byte{f.spec}, agreement.ContestedHashes)
	require.False(t, agreement.DefaultApplied)
	serviceTestEvent(t, f, log, "OutcomeRejected", f.agreement, agreement.Provider, 1, agreement.OutcomeSequence)
	disputeID := [32]byte{0xa0}
	log = serviceTestCall(t, f, f.owner.caller, "serviceDisputeOpen", disputeID, f.agreement, [][32]byte{{0xa2}, {0xa1}})
	dispute, found := f.h.keeper.GetDispute(f.h.db.Ctx(), disputeID)
	require.True(t, found)
	require.Equal(t, [][32]byte{{0xa1}, {0xa2}}, dispute.EvidenceHashes)
	require.False(t, dispute.Resolved)
	serviceTestEvent(t, f, log, "DisputeOpened", disputeID, f.agreement, 2, dispute.GlobalSequence)
	a := f.h.precompile.GetABI()
	input, err := a.Pack("serviceDisputeResolve", disputeID, uint16(2), uint32(3333), f.escrow)
	require.NoError(t, err)
	for _, caller := range []common.Address{f.owner.caller, f.beneficiary.caller} {
		before := f.h.keeper.ExportGenesis(f.h.db.Ctx())
		logs := len(f.h.db.GetAllLogs())
		ordinalKey := crypto.Keccak256Hash([]byte("layerxpay/call-ordinal/v1"))
		ordinal := f.h.db.GetTransientState(f.h.precompile.Address(), ordinalKey)
		output, _, err := f.h.evm.Call(caller, f.h.precompile.Address(), input, 2000000, uint256.NewInt(0))
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		reason, err := abi.UnpackRevert(output)
		require.NoError(t, err)
		require.Contains(t, reason, "LXP_UNAUTHORIZED_ARBITER")
		require.Equal(t, before, f.h.keeper.ExportGenesis(f.h.db.Ctx()))
		require.Len(t, f.h.db.GetAllLogs(), logs)
		require.Equal(t, ordinal, f.h.db.GetTransientState(f.h.precompile.Address(), ordinalKey))
	}
	escrowTestWrite(t, f.escrowTestFixture, f.beneficiary.caller, "escrowDisputeOpen", "EscrowDisputed", 6, 5, 0, 100, f.escrow)
	log = serviceTestCall(t, f, f.arbiter.caller, "serviceDisputeResolve", disputeID, uint16(2), uint32(3333), f.escrow)
	dispute, found = f.h.keeper.GetDispute(f.h.db.Ctx(), disputeID)
	require.True(t, found)
	require.True(t, dispute.Resolved)
	require.Equal(t, uint16(2), dispute.Ruling)
	require.Equal(t, uint32(3333), dispute.ProviderBPS)
	require.Equal(t, f.escrow, dispute.EscrowResolutionID)
	serviceTestEvent(t, f, log, "DisputeResolved", disputeID, f.agreement, 1, dispute.ResolutionSequence)
	agreement, found = f.h.keeper.GetAgreement(f.h.db.Ctx(), f.agreement)
	require.True(t, found)
	require.Equal(t, uint8(7), agreement.State)
	escrow, found := f.h.keeper.GetEscrow(f.h.db.Ctx(), f.escrow)
	require.True(t, found)
	require.Equal(t, uint8(6), escrow.State)
	require.Equal(t, codec.U128{Lo: 33}, escrow.CapturedAmount)
	require.True(t, escrow.LockedAmount.IsZero())
	for _, balance := range []struct {
		account [32]byte
		amount  uint64
	}{{f.owner.account, 967}, {f.beneficiary.account, 33}} {
		got, err := f.h.keeper.Host().ReadBalance(f.h.db.Ctx(), balance.account, f.asset)
		require.NoError(t, err)
		require.Equal(t, fmt.Sprint(balance.amount), got.String())
	}
}

func TestPrecompileServiceWithdrawAbandonAttest(t *testing.T) {
	t.Run("withdraw", func(t *testing.T) {
		f := newServiceTestFixture(t)
		serviceTestForm(t, f, 1)
		id := [32]byte{0xb0}
		serviceTestCall(t, f, f.beneficiary.caller, "serviceOfferPublish", id, f.asset, big.NewInt(100), f.terms, f.spec, f.deadline, uint64(1000), uint64(3000), uint8(1), f.deadline)
		offerBefore, found := f.h.keeper.GetOffer(f.h.db.Ctx(), id)
		require.True(t, found)
		log := serviceTestCall(t, f, f.beneficiary.caller, "serviceOfferWithdraw", id)
		offer, found := f.h.keeper.GetOffer(f.h.db.Ctx(), id)
		require.True(t, found)
		require.True(t, offer.Withdrawn)
		sequence, err := f.h.keeper.Host().ReadLastSequence(f.h.db.Ctx(), f.beneficiary.did)
		require.NoError(t, err)
		require.Greater(t, sequence, offerBefore.GlobalSequence)
		serviceTestEvent(t, f, log, "OfferWithdrawn", id, f.beneficiary.account, 1, sequence)
	})
	t.Run("attest_abandon", func(t *testing.T) {
		f := newServiceTestFixture(t)
		serviceTestForm(t, f, 1)
		serviceTestCommit(t, f)
		execution := make([]byte, paytypes.ServiceExecutionBytes)
		id, activityID := [32]byte{0xb1}, [32]byte{0xb2}
		for i, field := range [][32]byte{id, activityID, f.agreement, f.commitment, {0xb3}, {0xb4}, {0xb5}} {
			copy(execution[32*i:32*(i+1)], field[:])
		}
		now := uint64(f.h.db.Ctx().BlockTime().UnixMilli())
		binary.BigEndian.PutUint64(execution[224:232], now-100)
		binary.BigEndian.PutUint64(execution[232:240], now)
		binary.BigEndian.PutUint64(execution[240:248], 250)
		copy(execution[248:280], f.beneficiary.account[:])
		availability := [32]byte{0xb6}
		copy(execution[280:312], availability[:])
		copy(execution[312:344], f.beneficiary.public[:])
		digest, err := codec.DomainHash(codec.DomainSignaturePreimage, append([]byte("LXP:SERVICE:ATTEST:v1"), execution[:344]...))
		require.NoError(t, err)
		copy(execution[344:408], ed25519.Sign(f.beneficiary.private, digest[:]))
		log := serviceTestCall(t, f, f.beneficiary.caller, "serviceToolExecAttest", execution)
		stored, found := f.h.keeper.GetExecution(f.h.db.Ctx(), id)
		require.True(t, found)
		require.Equal(t, activityID, stored.ActivityID)
		require.Equal(t, f.agreement, stored.AgreementID)
		require.Equal(t, uint64(250), stored.ResourceUnits)
		require.True(t, ed25519.Verify(ed25519.PublicKey(stored.PublicKey[:]), digest[:], stored.Signature[:]))
		serviceTestEvent(t, f, log, "ExecutionAttested", id, f.commitment, 0, stored.GlobalSequence)
		log = serviceTestCall(t, f, f.beneficiary.caller, "serviceCommitAbandon", f.commitment, uint16(7))
		commitment, found := f.h.keeper.GetCommitment(f.h.db.Ctx(), f.commitment)
		require.True(t, found)
		require.True(t, commitment.Abandoned)
		require.Equal(t, uint16(7), commitment.AbandonReason)
		sequence, err := f.h.keeper.Host().ReadLastSequence(f.h.db.Ctx(), f.beneficiary.did)
		require.NoError(t, err)
		require.Greater(t, sequence, commitment.GlobalSequence)
		serviceTestEvent(t, f, log, "CommitAbandoned", f.commitment, f.agreement, 1, sequence)
		agreement, found := f.h.keeper.GetAgreement(f.h.db.Ctx(), f.agreement)
		require.True(t, found)
		require.Equal(t, uint8(1), agreement.State)
	})
	for _, outcome := range []uint8{1, 2} {
		for _, method := range []string{"serviceAccept", "serviceReject"} {
			t.Run(fmt.Sprintf("default_%d_%s", outcome, method), func(t *testing.T) {
				f := newServiceTestFixture(t)
				serviceTestForm(t, f, outcome)
				serviceTestCommit(t, f)
				serviceTestDeliver(t, f)
				agreement, found := f.h.keeper.GetAgreement(f.h.db.Ctx(), f.agreement)
				require.True(t, found)
				f.h.db.WithCtx(f.h.db.Ctx().WithBlockTime(time.UnixMilli(int64(agreement.AcceptanceWindowEndMs) + 1)))
				args := []interface{}{f.agreement}
				if method == "serviceReject" {
					args = append(args, uint16(7), [][32]byte{f.spec})
				}
				log := serviceTestCall(t, f, f.owner.caller, method, args...)
				agreement, found = f.h.keeper.GetAgreement(f.h.db.Ctx(), f.agreement)
				require.True(t, found)
				require.True(t, agreement.DefaultApplied)
				require.Equal(t, uint8(3)+outcome, agreement.State)
				require.Equal(t, uint64(f.h.db.Ctx().BlockTime().UnixMilli()), agreement.OutcomeTimestampMs)
				a := f.h.precompile.GetABI()
				event := a.Events["DefaultOutcomeApplied"]
				require.Equal(t, f.h.precompile.Address(), log.Address)
				require.Equal(t, []common.Hash{event.ID, common.Hash(f.agreement)}, log.Topics)
				values, err := event.Inputs.NonIndexed().Unpack(log.Data)
				require.NoError(t, err)
				require.Equal(t, []interface{}{agreement.State, true, agreement.OutcomeSequence, agreement.OutcomeTimestampMs}, values)
			})
		}
	}
}
