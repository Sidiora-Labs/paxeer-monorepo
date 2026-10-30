package keeper

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func serviceAgreementHarness(t *testing.T) (*payHarness, [32]byte, [32]byte, [32]byte) {
	t.Helper()
	h := newPayHarness(t)
	escrowID, offerID, termsHash := [32]byte{70}, [32]byte{71}, [32]byte{72}
	payload := paySubmitFundedEscrow(t, h, escrowID)
	_, err := h.keeper.Submit(h.ctx, h.actors[0].address, "escrowOpen", payload, [32]byte{73})
	require.NoError(t, err)
	escrow, found := h.keeper.GetEscrow(h.ctx, escrowID)
	require.True(t, found)
	h.keeper.SetParams(h.ctx, types.Params{EscrowEnabled: true, ServiceEnabled: true})
	key, err := types.KernelModuleEnableKey(types.ServiceModuleID)
	require.NoError(t, err)
	h.ctx.KVStore(h.hostKey).Set(key, payGateEnabledRecord())
	payload, err = types.EncodeServiceOffer(offerID, escrow.AssetID, codec.U128{Lo: 50}, termsHash, [32]byte{74},
		uint64(payHarnessTimeMs)+100000, 10000, 20000, 1, uint64(payHarnessTimeMs)+50000)
	require.NoError(t, err)
	_, err = h.keeper.Submit(h.ctx, h.actors[1].address, "serviceOfferPublish", payload, [32]byte{75})
	require.NoError(t, err)
	return h, escrowID, offerID, termsHash
}

func serviceAgreementExpected(t *testing.T, h *payHarness, actor payActor, activityType uint32, payload []byte, key [32]byte) lxphost.Receipt {
	t.Helper()
	last, err := h.host.ReadLastSequence(h.ctx, actor.did)
	require.NoError(t, err)
	sequence, err := h.host.ReadIdentitySequence(h.ctx, actor.did)
	require.NoError(t, err)
	_, principal, err := lxphost.MainAccountID(actor.public)
	require.NoError(t, err)
	authority := actor.authority()
	authority.Bound, authority.Principal = true, principal
	branch, _ := h.ctx.CacheContext()
	receipt, err := h.host.HostExecute(lxphost.WithSequence(branch, last+1), lxphost.Activity{
		ProtocolVersion: 3, Type: activityType, ActorDID: actor.did, AuthorityKey: actor.public,
		AccountSequence: sequence, NotBefore: uint64(h.ctx.BlockTime().UnixMilli()),
		NotAfter:       uint64(h.ctx.BlockTime().UnixMilli()) + h.host.TimestampWindowMs(),
		IdempotencyKey: key, Payload: payload,
	}, authority)
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	return receipt
}

func TestServiceAgreementProposeAccept(t *testing.T) {
	h, escrowID, offerID, termsHash := serviceAgreementHarness(t)
	buyer, provider := h.actors[0], h.actors[1]
	agreementID, proposeKey, acceptKey := [32]byte{80}, [32]byte{81}, [32]byte{82}
	before := payStoreSnapshot(h.ctx, h.hostKey)
	_, err := h.keeper.ServiceAgreementPropose(h.ctx, buyer.address, agreementID, offerID, termsHash, [32]byte{}, proposeKey)
	require.EqualError(t, err, "LXP_NON_CANONICAL")
	_, err = h.keeper.ServiceAgreementAccept(h.ctx, provider.address, [32]byte{}, acceptKey)
	require.EqualError(t, err, "LXP_NON_CANONICAL")
	_, err = h.keeper.ServiceAgreementPropose(h.ctx, common.Address{}, agreementID, offerID, termsHash, escrowID, proposeKey)
	require.ErrorIs(t, err, types.ErrUnboundCaller)
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))

	payload, err := types.EncodeServicePropose(agreementID, offerID, termsHash, escrowID)
	require.NoError(t, err)
	expected := serviceAgreementExpected(t, h, buyer, 0x00050003, payload, proposeKey)
	proposed, err := h.keeper.ServiceAgreementPropose(h.ctx, buyer.address, agreementID, offerID, termsHash, escrowID, proposeKey)
	require.NoError(t, err)
	require.Equal(t, expected, proposed)
	agreement, found := h.keeper.GetAgreement(h.ctx, agreementID)
	require.True(t, found)
	require.Equal(t, agreementID, agreement.AgreementID)
	require.Equal(t, offerID, agreement.OfferID)
	require.Equal(t, termsHash, agreement.TermsHash)
	require.Equal(t, escrowID, agreement.EscrowID)
	_, buyerID, err := lxphost.MainAccountID(buyer.public)
	require.NoError(t, err)
	_, providerID, err := lxphost.MainAccountID(provider.public)
	require.NoError(t, err)
	require.Equal(t, buyerID, agreement.Buyer)
	require.Equal(t, providerID, agreement.Provider)
	require.Equal(t, uint8(8), agreement.State)

	before = payStoreSnapshot(h.ctx, h.hostKey)
	_, err = h.keeper.ServiceAgreementAccept(h.ctx, buyer.address, agreementID, acceptKey)
	require.ErrorContains(t, err, "LXP_UNAUTHORIZED_DEBIT")
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
	params := h.keeper.GetParams(h.ctx)
	params.Paused = true
	h.keeper.SetParams(h.ctx, params)
	_, err = h.keeper.ServiceAgreementPropose(h.ctx, buyer.address, [32]byte{83}, offerID, termsHash, escrowID, proposeKey)
	require.ErrorIs(t, err, govtypes.ErrPausedScope)
	_, err = h.keeper.ServiceAgreementAccept(h.ctx, provider.address, agreementID, acceptKey)
	require.ErrorIs(t, err, govtypes.ErrPausedScope)
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
	params.Paused = false
	h.keeper.SetParams(h.ctx, params)
	payload, err = types.EncodeServiceIdentifier(agreementID)
	require.NoError(t, err)
	expected = serviceAgreementExpected(t, h, provider, 0x00050004, payload, acceptKey)
	accepted, err := h.keeper.ServiceAgreementAccept(h.ctx, provider.address, agreementID, acceptKey)
	require.NoError(t, err)
	require.Equal(t, expected, accepted)
	require.Equal(t, proposed.GlobalSequence+1, accepted.GlobalSequence)
	agreement, found = h.keeper.GetAgreement(h.ctx, agreementID)
	require.True(t, found)
	require.Equal(t, uint8(1), agreement.State)
	require.Equal(t, accepted.GlobalSequence, agreement.AcceptedSequence)
	offer, found := h.keeper.GetOffer(h.ctx, offerID)
	require.True(t, found)
	require.True(t, offer.Accepted)
}

func TestServiceCommitAndAbandon(t *testing.T) {
	h, escrowID, offerID, termsHash := serviceAgreementHarness(t)
	buyer, provider := h.actors[0], h.actors[1]
	agreementID, commitmentID, taskHash := [32]byte{90}, [32]byte{91}, [32]byte{92}
	_, err := h.keeper.ServiceAgreementPropose(h.ctx, buyer.address, agreementID, offerID, termsHash, escrowID, [32]byte{93})
	require.NoError(t, err)
	_, err = h.keeper.ServiceAgreementAccept(h.ctx, provider.address, agreementID, [32]byte{94})
	require.NoError(t, err)
	deadline, resourceBound := uint64(payHarnessTimeMs)+90000, uint64(12345)
	commitKey, abandonKey := [32]byte{95}, [32]byte{96}
	before := payStoreSnapshot(h.ctx, h.hostKey)
	_, err = h.keeper.ServiceCommitTask(h.ctx, provider.address, commitmentID, agreementID, taskHash, escrowID, deadline, 0, commitKey)
	require.EqualError(t, err, "LXP_NON_CANONICAL")
	_, err = h.keeper.ServiceCommitAbandon(h.ctx, provider.address, commitmentID, 0, abandonKey)
	require.EqualError(t, err, "LXP_NON_CANONICAL")
	_, err = h.keeper.ServiceCommitTask(h.ctx, buyer.address, commitmentID, agreementID, taskHash, escrowID, deadline, resourceBound, commitKey)
	require.ErrorContains(t, err, "LXP_AGREEMENT_STATE")
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))

	payload, err := types.EncodeServiceCommit(commitmentID, agreementID, taskHash, escrowID, deadline, resourceBound)
	require.NoError(t, err)
	expected := serviceAgreementExpected(t, h, provider, 0x00050005, payload, commitKey)
	committed, err := h.keeper.ServiceCommitTask(h.ctx, provider.address, commitmentID, agreementID, taskHash, escrowID, deadline, resourceBound, commitKey)
	require.NoError(t, err)
	require.Equal(t, expected, committed)
	commitment, found := h.keeper.GetCommitment(h.ctx, commitmentID)
	require.True(t, found)
	_, providerID, err := lxphost.MainAccountID(provider.public)
	require.NoError(t, err)
	require.Equal(t, commitmentID, commitment.CommitmentID)
	require.Equal(t, providerID, commitment.Provider)
	require.Equal(t, agreementID, commitment.AgreementID)
	require.Equal(t, taskHash, commitment.TaskHash)
	require.Equal(t, escrowID, commitment.EscrowID)
	require.Equal(t, deadline, commitment.DeadlineMs)
	require.Equal(t, resourceBound, commitment.ResourceBound)
	require.Equal(t, committed.GlobalSequence, commitment.GlobalSequence)
	require.False(t, commitment.Abandoned)
	agreement, found := h.keeper.GetAgreement(h.ctx, agreementID)
	require.True(t, found)
	require.Equal(t, uint8(2), agreement.State)

	params := h.keeper.GetParams(h.ctx)
	params.Paused = true
	h.keeper.SetParams(h.ctx, params)
	before = payStoreSnapshot(h.ctx, h.hostKey)
	_, err = h.keeper.ServiceCommitTask(h.ctx, provider.address, [32]byte{97}, agreementID, taskHash, escrowID, deadline, resourceBound, commitKey)
	require.ErrorIs(t, err, govtypes.ErrPausedScope)
	_, err = h.keeper.ServiceCommitAbandon(h.ctx, buyer.address, commitmentID, 7, abandonKey)
	require.ErrorContains(t, err, "LXP_AGREEMENT_STATE")
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
	payload, err = types.EncodeServiceAbandon(commitmentID, 7)
	require.NoError(t, err)
	expected = serviceAgreementExpected(t, h, provider, 0x00050006, payload, abandonKey)
	abandoned, err := h.keeper.ServiceCommitAbandon(h.ctx, provider.address, commitmentID, 7, abandonKey)
	require.NoError(t, err)
	require.Equal(t, expected, abandoned)
	require.Equal(t, committed.GlobalSequence+1, abandoned.GlobalSequence)
	commitment, found = h.keeper.GetCommitment(h.ctx, commitmentID)
	require.True(t, found)
	require.True(t, commitment.Abandoned)
	require.Equal(t, uint16(7), commitment.AbandonReason)
	agreement, found = h.keeper.GetAgreement(h.ctx, agreementID)
	require.True(t, found)
	require.Equal(t, uint8(1), agreement.State)
}
