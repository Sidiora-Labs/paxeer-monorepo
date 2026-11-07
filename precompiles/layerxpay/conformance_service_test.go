package layerxpay_test

import (
	"bytes"
	"crypto/sha256"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func serviceConformanceStore(t *testing.T, h *payPrecompileHarness, name string) sdk.KVStore {
	t.Helper()
	for _, key := range h.db.Ctx().MultiStore().StoreKeys() {
		if key.Name() == name {
			return h.db.Ctx().KVStore(key)
		}
	}
	t.Fatalf("store %s is not mounted", name)
	return nil
}

func serviceConformanceSnapshot(t *testing.T, h *payPrecompileHarness, name string) map[string][]byte {
	t.Helper()
	iterator := serviceConformanceStore(t, h, name).Iterator(nil, nil)
	defer iterator.Close()
	rows := make(map[string][]byte)
	for ; iterator.Valid(); iterator.Next() {
		rows[string(iterator.Key())] = bytes.Clone(iterator.Value())
	}
	return rows
}

func serviceConformanceExecute(t *testing.T, h *payPrecompileHarness, ctx sdk.Context, actor escrowTestActor, kind uint32, payload []byte, key [32]byte) lxphost.Receipt {
	t.Helper()
	host := h.keeper.Host()
	last, err := host.ReadLastSequence(ctx, actor.did)
	require.NoError(t, err)
	sequence, err := host.ReadIdentitySequence(ctx, actor.did)
	require.NoError(t, err)
	now := uint64(ctx.BlockTime().UnixMilli())
	receipt, err := host.HostExecute(lxphost.WithSequence(ctx, last+1), lxphost.Activity{
		ProtocolVersion: 3, Type: kind, ActorDID: actor.did, AuthorityKey: actor.public,
		AccountSequence: sequence, NotBefore: now, NotAfter: now + host.TimestampWindowMs(),
		IdempotencyKey: key, Payload: payload,
	}, lxphost.KernelAuthority{Actor: actor.actor, Principal: actor.account, VerifiedKey: actor.public, Bound: true})
	require.NoError(t, err)
	return receipt
}

func serviceConformanceRoot(t *testing.T, h *payPrecompileHarness, actor escrowTestActor) [32]byte {
	t.Helper()
	payload, err := paytypes.EncodeServiceIdentifier([32]byte{0xff})
	require.NoError(t, err)
	before := serviceConformanceSnapshot(t, h, "lxphost")
	branch, _ := h.db.Ctx().CacheContext()
	receipt := serviceConformanceExecute(t, h, branch, actor, paytypes.ActivityServiceAgreementAccept, payload, [32]byte{0xfe})
	require.Equal(t, lxphost.Code(-706), receipt.ResultCode)
	require.NotEqual(t, [32]byte{}, receipt.PreviousStateRoot)
	require.NotEqual(t, [32]byte{}, receipt.ResultingStateRoot)
	replayBranch, _ := h.db.Ctx().CacheContext()
	replay := serviceConformanceExecute(t, h, replayBranch, actor, paytypes.ActivityServiceAgreementAccept, payload, [32]byte{0xfe})
	require.Equal(t, receipt, replay)
	require.Equal(t, before, serviceConformanceSnapshot(t, h, "lxphost"))
	return receipt.PreviousStateRoot
}

func TestPayServicePathMatchesHostExecute(t *testing.T) {
	seed := newEscrowTestFixture(t)
	via := seed.h
	via.keeper.SetParams(via.db.Ctx(), paytypes.Params{EscrowEnabled: true, ServiceEnabled: true})
	stranger := escrowTestRegister(t, via, [32]byte{0x28, 0x05}, common.HexToAddress("0xa2285"), true)
	stranger.account = escrowTestMain(t, via, stranger, seed.asset)
	direct := newPayPrecompileHarness(t)
	for _, name := range []string{"lxphost", paytypes.StoreKey} {
		destination := serviceConformanceStore(t, direct, name)
		for key := range serviceConformanceSnapshot(t, direct, name) {
			destination.Delete([]byte(key))
		}
		for key, value := range serviceConformanceSnapshot(t, via, name) {
			destination.Set([]byte(key), value)
		}
		require.Equal(t, serviceConformanceSnapshot(t, via, name), serviceConformanceSnapshot(t, direct, name))
	}
	require.Equal(t, common.HexToAddress("0x101b"), via.precompile.Address())
	require.Equal(t, serviceConformanceRoot(t, via, seed.owner), serviceConformanceRoot(t, direct, seed.owner))

	offerID, agreementID, commitmentID := [32]byte{0x81}, [32]byte{0x82}, [32]byte{0x83}
	escrowID, terms, specification := [32]byte{0x84}, [32]byte{0x85}, [32]byte{0x86}
	deliveryID, disputeID := [32]byte{0x88}, [32]byte{0xa0}
	now := uint64(via.db.Ctx().BlockTime().UnixMilli())
	deadline := now + 1000
	a := via.precompile.GetABI()
	ordinalKey := crypto.Keccak256Hash([]byte("layerxpay/call-ordinal/v1"))
	viaStore := via.db.Ctx().MultiStore()

	compareRecords := func(t *testing.T) {
		t.Helper()
		gotOffer, gotFound := via.keeper.GetOffer(via.db.Ctx(), offerID)
		wantOffer, wantFound := direct.keeper.GetOffer(direct.db.Ctx(), offerID)
		require.Equal(t, wantFound, gotFound)
		require.Equal(t, wantOffer, gotOffer)
		gotAgreement, gotFound := via.keeper.GetAgreement(via.db.Ctx(), agreementID)
		wantAgreement, wantFound := direct.keeper.GetAgreement(direct.db.Ctx(), agreementID)
		require.Equal(t, wantFound, gotFound)
		require.Equal(t, wantAgreement, gotAgreement)
		gotCommitment, gotFound := via.keeper.GetCommitment(via.db.Ctx(), commitmentID)
		wantCommitment, wantFound := direct.keeper.GetCommitment(direct.db.Ctx(), commitmentID)
		require.Equal(t, wantFound, gotFound)
		require.Equal(t, wantCommitment, gotCommitment)
		gotDelivery, gotFound := via.keeper.GetDelivery(via.db.Ctx(), deliveryID)
		wantDelivery, wantFound := direct.keeper.GetDelivery(direct.db.Ctx(), deliveryID)
		require.Equal(t, wantFound, gotFound)
		require.Equal(t, wantDelivery, gotDelivery)
		gotDispute, gotFound := via.keeper.GetDispute(via.db.Ctx(), disputeID)
		wantDispute, wantFound := direct.keeper.GetDispute(direct.db.Ctx(), disputeID)
		require.Equal(t, wantFound, gotFound)
		require.Equal(t, wantDispute, gotDispute)
		gotEscrow, gotFound := via.keeper.GetEscrow(via.db.Ctx(), escrowID)
		wantEscrow, wantFound := direct.keeper.GetEscrow(direct.db.Ctx(), escrowID)
		require.Equal(t, wantFound, gotFound)
		require.Equal(t, wantEscrow, gotEscrow)
	}

	run := func(name string, actor escrowTestActor, method string, payload []byte, wantCode lxphost.Code, args ...interface{}) lxphost.Receipt {
		t.Helper()
		var receipt lxphost.Receipt
		ok := t.Run(name, func(t *testing.T) {
			input, err := a.Pack(method, args...)
			require.NoError(t, err)
			ordinal := via.db.GetTransientState(via.precompile.Address(), ordinalKey)
			var selector [4]byte
			copy(selector[:], a.Methods[method].ID)
			key := paytypes.DeriveIdempotencyKey([32]byte(via.db.Ctx().EVMTxHash()), uint32(ordinal.Big().Uint64()), selector)
			switch method {
			case "escrowOpen", "serviceOfferPublish", "serviceDisputeOpen":
				key = args[0].([32]byte)
			case "serviceDisputeResolve":
				key = sha256.Sum256(payload)
			}
			hostBefore := serviceConformanceSnapshot(t, via, "lxphost")
			payBefore := serviceConformanceSnapshot(t, via, paytypes.StoreKey)
			rootBefore := serviceConformanceRoot(t, via, seed.owner)
			logCount := len(via.db.GetAllLogs())
			branch, commit := direct.db.Ctx().CacheContext()
			escrowBefore, escrowFound := direct.keeper.GetEscrow(branch, escrowID)
			receipt = serviceConformanceExecute(t, direct, branch, actor, paytypes.MethodActivity[method].Type, payload, key)
			code := receipt.ResultCode
			if method == "serviceDisputeResolve" && code == 0 {
				require.True(t, escrowFound)
				escrowAfter, found := direct.keeper.GetEscrow(branch, escrowID)
				require.True(t, found)
				require.Equal(t, escrowBefore, escrowAfter, "the service activity must leave settlement to escrow")
				serviceDispute, found := direct.keeper.GetDispute(branch, disputeID)
				require.True(t, found)
				require.True(t, serviceDispute.Resolved)
				escrowKey := sha256.Sum256(append([]byte("layerxpay:serviceDisputeResolve:escrow:"), payload...))
				settlement := serviceConformanceExecute(t, direct, branch, actor, paytypes.ActivityEscrowDisputeResolve,
					paytypes.EncodeEscrowDisputeResolve(escrowID, args[2].(uint32), escrowKey), escrowKey)
				code = settlement.ResultCode
			}
			require.Equal(t, wantCode, code)
			if code == 0 {
				commit()
			}
			output, _, callErr := via.evm.Call(actor.caller, via.precompile.Address(), input, 2000000, uint256.NewInt(0))
			if wantCode == 0 {
				require.NoError(t, callErr, "%x", output)
				require.Empty(t, output)
				require.Len(t, via.db.GetAllLogs(), logCount+1)
				require.Equal(t, ordinal.Big().Uint64()+1, via.db.GetTransientState(via.precompile.Address(), ordinalKey).Big().Uint64())
			} else {
				require.ErrorIs(t, callErr, vm.ErrExecutionReverted)
				reason, err := abi.UnpackRevert(output)
				require.NoError(t, err)
				require.Equal(t, pcommon.LXPCodeName(int32(code)), strings.SplitN(reason, ":", 2)[0])
				require.Equal(t, hostBefore, serviceConformanceSnapshot(t, via, "lxphost"))
				require.Equal(t, payBefore, serviceConformanceSnapshot(t, via, paytypes.StoreKey))
				require.Len(t, via.db.GetAllLogs(), logCount)
				require.Equal(t, ordinal, via.db.GetTransientState(via.precompile.Address(), ordinalKey))
			}
			_, err = via.db.Finalize()
			require.NoError(t, err)
			via.db.WithCtx(via.db.Ctx().WithMultiStore(viaStore))
			compareRecords(t)
			for _, name := range []string{"lxphost", paytypes.StoreKey} {
				require.Equal(t, serviceConformanceSnapshot(t, direct, name), serviceConformanceSnapshot(t, via, name), "%s store", name)
			}
			root := serviceConformanceRoot(t, via, seed.owner)
			require.Equal(t, serviceConformanceRoot(t, direct, seed.owner), root)
			if wantCode == 0 {
				require.NotEqual(t, rootBefore, root)
				if paytypes.MethodActivity[method].Module == paytypes.ServiceModuleID && method != "serviceDisputeResolve" {
					require.Equal(t, rootBefore, receipt.PreviousStateRoot)
					require.Equal(t, receipt.ResultingStateRoot, root)
				}
			} else {
				require.Equal(t, rootBefore, root)
			}
		})
		if !ok {
			t.FailNow()
		}
		return receipt
	}

	openArgs := seed.openArgs(t, escrowID)
	openArgs[7], openArgs[9], openArgs[10] = deadline+10000, terms, agreementID
	openPayload := paytypes.EncodeEscrowOpen(escrowID, seed.owner.account, openArgs[2].([32]byte), seed.beneficiary.account,
		seed.arbiter.account, seed.asset, codec.U128{Lo: 100}, deadline+10000, openArgs[8].(uint64), terms, agreementID)
	run("escrow_open", seed.owner, "escrowOpen", openPayload, 0, openArgs...)

	payload, err := paytypes.EncodeServiceOffer(offerID, seed.asset, codec.U128{Lo: 100}, terms, specification, deadline, 1000, 3000, 2, deadline)
	require.NoError(t, err)
	published := run("offer_publish", seed.beneficiary, "serviceOfferPublish", payload, 0,
		offerID, seed.asset, big.NewInt(100), terms, specification, deadline, uint64(1000), uint64(3000), uint8(2), deadline)
	offer, found := via.keeper.GetOffer(via.db.Ctx(), offerID)
	require.True(t, found)
	require.Equal(t, seed.beneficiary.account, offer.OfferingAgent)
	require.Equal(t, published.GlobalSequence, offer.GlobalSequence)
	require.Equal(t, codec.U128{Lo: 100}, offer.Price)
	require.False(t, offer.Accepted)

	payload, err = paytypes.EncodeServicePropose(agreementID, offerID, terms, escrowID)
	require.NoError(t, err)
	run("agreement_propose", seed.owner, "serviceAgreementPropose", payload, 0, agreementID, offerID, terms, escrowID)
	agreement, found := via.keeper.GetAgreement(via.db.Ctx(), agreementID)
	require.True(t, found)
	require.Equal(t, uint8(8), agreement.State)
	require.Equal(t, seed.owner.account, agreement.Buyer)
	require.Equal(t, seed.beneficiary.account, agreement.Provider)
	require.Equal(t, escrowID, agreement.EscrowID)

	payload, err = paytypes.EncodeServiceIdentifier(agreementID)
	require.NoError(t, err)
	accepted := run("agreement_accept", seed.beneficiary, "serviceAgreementAccept", payload, 0, agreementID)
	agreement, found = via.keeper.GetAgreement(via.db.Ctx(), agreementID)
	require.True(t, found)
	require.Equal(t, uint8(1), agreement.State)
	require.Equal(t, accepted.GlobalSequence, agreement.AcceptedSequence)
	offer, found = via.keeper.GetOffer(via.db.Ctx(), offerID)
	require.True(t, found)
	require.True(t, offer.Accepted)

	payload, err = paytypes.EncodeServiceCommit(commitmentID, agreementID, [32]byte{0x87}, escrowID, deadline, 1000)
	require.NoError(t, err)
	run("commit_task", seed.beneficiary, "serviceCommitTask", payload, 0,
		commitmentID, agreementID, [32]byte{0x87}, escrowID, deadline, uint64(1000))
	items := []paytypes.ServiceDeliverable{{Hash: specification, ArtifactSize: 4096, AvailabilityReference: [32]byte{0x89}}}
	payload, err = paytypes.EncodeServiceDeliver(deliveryID, agreementID, items)
	require.NoError(t, err)
	delivered := run("deliver", seed.beneficiary, "serviceDeliver", payload, 0,
		deliveryID, agreementID, []serviceTestDeliverable{{specification, 4096, [32]byte{0x89}}})
	delivery, found := via.keeper.GetDelivery(via.db.Ctx(), deliveryID)
	require.True(t, found)
	require.Equal(t, items, delivery.Deliverables)
	require.Equal(t, delivered.GlobalSequence, delivery.GlobalSequence)
	payload, err = paytypes.EncodeServiceReject(agreementID, 7, [][32]byte{specification})
	require.NoError(t, err)
	run("reject_delivery", seed.owner, "serviceReject", payload, 0, agreementID, uint16(7), [][32]byte{specification})
	agreement, found = via.keeper.GetAgreement(via.db.Ctx(), agreementID)
	require.True(t, found)
	require.Equal(t, uint8(5), agreement.State)
	require.Equal(t, [][32]byte{specification}, agreement.ContestedHashes)

	evidence := [][32]byte{{0xa3}, {0xa1}, {0xa2}}
	payload, err = paytypes.EncodeServiceDisputeOpen(disputeID, agreementID, evidence)
	require.NoError(t, err)
	run("stranger_cannot_open", stranger, "serviceDisputeOpen", payload, -728, disputeID, agreementID, evidence)
	run("arbiter_is_not_an_agreement_party", seed.arbiter, "serviceDisputeOpen", payload, -728, disputeID, agreementID, evidence)
	setTime := func(timestamp uint64) {
		for _, h := range []*payPrecompileHarness{via, direct} {
			h.db.WithCtx(h.db.Ctx().WithBlockTime(time.UnixMilli(int64(timestamp))))
		}
	}
	setTime(agreement.DisputeWindowEndMs + 1)
	run("dispute_window_closed", seed.owner, "serviceDisputeOpen", payload, -711, disputeID, agreementID, evidence)
	setTime(agreement.DisputeWindowEndMs)
	opened := run("open_at_dispute_window_boundary", seed.owner, "serviceDisputeOpen", payload, 0, disputeID, agreementID, evidence)
	dispute, found := via.keeper.GetDispute(via.db.Ctx(), disputeID)
	require.True(t, found)
	require.Equal(t, [][32]byte{{0xa1}, {0xa2}, {0xa3}}, dispute.EvidenceHashes)
	require.Equal(t, seed.owner.account, dispute.Raiser)
	require.Equal(t, opened.GlobalSequence, dispute.GlobalSequence)
	require.False(t, dispute.Resolved)
	run("cannot_open_twice", seed.owner, "serviceDisputeOpen", payload, -706, disputeID, agreementID, evidence)

	payload, err = paytypes.EncodeServiceDisputeResolve(disputeID, 2, 4000, escrowID)
	require.NoError(t, err)
	for _, actor := range []struct {
		name  string
		actor escrowTestActor
	}{{"stranger", stranger}, {"buyer", seed.owner}, {"provider", seed.beneficiary}} {
		run(actor.name+"_cannot_resolve", actor.actor, "serviceDisputeResolve", payload, -738, disputeID, uint16(2), uint32(4000), escrowID)
	}
	run("failed_settlement_rolls_back_ruling", seed.arbiter, "serviceDisputeResolve", payload, -700, disputeID, uint16(2), uint32(4000), escrowID)
	run("escrow_dispute_open", seed.beneficiary, "escrowDisputeOpen", paytypes.EncodeEscrowDisputeOpen(escrowID), 0, escrowID)
	resolved := run("arbiter_resolves", seed.arbiter, "serviceDisputeResolve", payload, 0, disputeID, uint16(2), uint32(4000), escrowID)
	dispute, found = via.keeper.GetDispute(via.db.Ctx(), disputeID)
	require.True(t, found)
	require.True(t, dispute.Resolved)
	require.Equal(t, uint16(2), dispute.Ruling)
	require.Equal(t, uint32(4000), dispute.ProviderBPS)
	require.Equal(t, escrowID, dispute.EscrowResolutionID)
	require.Equal(t, opened.GlobalSequence, dispute.GlobalSequence)
	require.Equal(t, resolved.GlobalSequence, dispute.ResolutionSequence)
	agreement, found = via.keeper.GetAgreement(via.db.Ctx(), agreementID)
	require.True(t, found)
	require.Equal(t, uint8(7), agreement.State)
	escrow, found := via.keeper.GetEscrow(via.db.Ctx(), escrowID)
	require.True(t, found)
	require.Equal(t, uint8(6), escrow.State)
	require.Equal(t, codec.U128{Lo: 40}, escrow.CapturedAmount)
	require.True(t, escrow.LockedAmount.IsZero())
	for _, balance := range []struct {
		account [32]byte
		amount  string
	}{{seed.owner.account, "960"}, {seed.beneficiary.account, "40"}, {escrow.EscrowAccount, "0"}} {
		for _, h := range []*payPrecompileHarness{via, direct} {
			got, err := h.keeper.Host().ReadBalance(h.db.Ctx(), balance.account, seed.asset)
			require.NoError(t, err)
			require.Equal(t, balance.amount, got.String())
		}
	}
	run("cannot_resolve_twice", seed.arbiter, "serviceDisputeResolve", payload, -706, disputeID, uint16(2), uint32(4000), escrowID)
}
