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

func TestServiceOfferPublishWithdraw(t *testing.T) {
	h := newPayHarness(t)
	funded := payEscrowFundedArgs(t, h)
	actor := h.actors[0]
	params := types.Params{ServiceEnabled: true}
	h.keeper.SetParams(h.ctx, params)
	enableKey, err := types.KernelModuleEnableKey(types.ServiceModuleID)
	require.NoError(t, err)
	h.ctx.KVStore(h.hostKey).Set(enableKey, payGateEnabledRecord())

	offerID, assetID := [32]byte{0x51}, funded.AssetID
	termsHash, specHash := [32]byte{0x53}, [32]byte{0x54}
	price := codec.U128{Lo: 100}
	deadline, expiry := uint64(payHarnessTimeMs+60000), uint64(payHarnessTimeMs+120000)
	const acceptanceWindow, disputeWindow uint64 = 30000, 45000
	const defaultOutcome uint8 = 2
	publish := func(caller common.Address, id [32]byte, amount codec.U128) (lxphost.Receipt, error) {
		return h.keeper.ServiceOfferPublish(h.ctx, caller, id, assetID, amount, termsHash, specHash,
			deadline, acceptanceWindow, disputeWindow, defaultOutcome, expiry)
	}
	expectedReceipt := func(activityType uint32, payload []byte) lxphost.Receipt {
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
			AccountSequence: sequence, NotBefore: uint64(payHarnessTimeMs),
			NotAfter:       uint64(payHarnessTimeMs) + h.host.TimestampWindowMs(),
			IdempotencyKey: offerID, Payload: payload,
		}, authority)
		require.NoError(t, err)
		require.Zero(t, receipt.ResultCode)
		return receipt
	}

	t.Run("invalid_payloads_do_not_write", func(t *testing.T) {
		hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
		moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
		for _, input := range []struct {
			id     [32]byte
			amount codec.U128
		}{{[32]byte{}, price}, {offerID, codec.U128{}}} {
			receipt, err := publish(actor.address, input.id, input.amount)
			require.EqualError(t, err, "LXP_NON_CANONICAL")
			require.Equal(t, lxphost.Receipt{}, receipt)
		}
		receipt, err := h.keeper.ServiceOfferWithdraw(h.ctx, actor.address, [32]byte{})
		require.EqualError(t, err, "LXP_NON_CANONICAL")
		require.Equal(t, lxphost.Receipt{}, receipt)
		require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
		require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
	})

	moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
	hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
	_, err = publish(common.Address{}, offerID, price)
	require.ErrorIs(t, err, types.ErrUnboundCaller)
	require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	payload, err := types.EncodeServiceOffer(offerID, assetID, price, termsHash, specHash,
		deadline, acceptanceWindow, disputeWindow, defaultOutcome, expiry)
	require.NoError(t, err)
	require.Len(t, payload, 179)
	expected := expectedReceipt(0x00050001, payload)
	require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	published, err := publish(actor.address, offerID, price)
	require.NoError(t, err)
	require.Equal(t, expected, published)
	require.Empty(t, published.Transfers)
	payEscrowBalance(t, h, funded.Owner, assetID, "1000")
	payEscrowBalance(t, h, funded.Beneficiary, assetID, "0")
	require.NotEqual(t, published.PreviousStateRoot, published.ResultingStateRoot)
	decoded, err := codec.DecodeReceipt(published.Encoded)
	require.NoError(t, err)
	require.Equal(t, types.ServiceModuleID, decoded.ModuleID)
	_, principal, err := lxphost.MainAccountID(actor.public)
	require.NoError(t, err)
	offer, found := h.keeper.GetOffer(h.ctx, offerID)
	require.True(t, found)
	require.NotEqual(t, [32]byte{}, offer.ActivityID)
	require.Equal(t, types.ServiceOffer{
		OfferID: offerID, ActivityID: offer.ActivityID, OfferingAgent: principal,
		AssetID: assetID, Price: price, TermsHash: termsHash, DeliverableSpecHash: specHash,
		DeliveryDeadlineMs: deadline, AcceptanceWindowMs: acceptanceWindow,
		DisputeWindowMs: disputeWindow, DefaultOutcome: defaultOutcome,
		OfferExpiryMs: expiry, GlobalSequence: published.GlobalSequence,
	}, offer)
	require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))

	hostBefore = payStoreSnapshot(h.ctx, h.hostKey)
	unauthorized, err := h.keeper.ServiceOfferWithdraw(h.ctx, h.actors[1].address, offerID)
	require.ErrorIs(t, err, lxphost.Code(-207))
	require.Equal(t, lxphost.Code(-207), unauthorized.ResultCode)
	require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))

	params.Paused = true
	h.keeper.SetParams(h.ctx, params)
	moduleBefore = payStoreSnapshot(h.ctx, h.storeKey)
	_, err = publish(actor.address, [32]byte{0x55}, price)
	require.ErrorIs(t, err, govtypes.ErrPausedScope)
	require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	payload, err = types.EncodeServiceIdentifier(offerID)
	require.NoError(t, err)
	require.Len(t, payload, 34)
	require.Equal(t, append([]byte{0, 1}, offerID[:]...), payload)
	expected = expectedReceipt(0x00050002, payload)
	withdrawn, err := h.keeper.ServiceOfferWithdraw(h.ctx, actor.address, offerID)
	require.NoError(t, err)
	require.Equal(t, expected, withdrawn)
	require.Empty(t, withdrawn.Transfers)
	payEscrowBalance(t, h, funded.Owner, assetID, "1000")
	payEscrowBalance(t, h, funded.Beneficiary, assetID, "0")
	require.Equal(t, published.GlobalSequence+1, withdrawn.GlobalSequence)
	require.NotEqual(t, published.Encoded, withdrawn.Encoded)
	afterWithdraw, found := h.keeper.GetOffer(h.ctx, offerID)
	require.True(t, found)
	offer.Withdrawn = true
	require.Equal(t, offer, afterWithdraw)
	require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))

	hostBefore = payStoreSnapshot(h.ctx, h.hostKey)
	refused, err := h.keeper.ServiceOfferWithdraw(h.ctx, actor.address, offerID)
	require.ErrorIs(t, err, lxphost.Code(-723))
	require.ErrorContains(t, err, "LXP_OFFER_UNAVAILABLE")
	require.Equal(t, lxphost.Code(-723), refused.ResultCode)
	require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
	finalOffer, found := h.keeper.GetOffer(h.ctx, offerID)
	require.True(t, found)
	require.Equal(t, afterWithdraw, finalOffer)
	payEscrowBalance(t, h, funded.Owner, assetID, "1000")
	payEscrowBalance(t, h, funded.Beneficiary, assetID, "0")
}
