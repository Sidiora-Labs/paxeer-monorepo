package keeper

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/stretchr/testify/require"
)

func serviceDisputeSetup(t *testing.T, withArbiter, disputed bool) (*payHarness, payActor, [32]byte, [32]byte, [32]byte) {
	t.Helper()
	h := newPayHarness(t)
	arbiter := h.registerActor(t, 3)
	escrowID, agreementID, disputeID := [32]byte{150}, [32]byte{151}, [32]byte{152}
	payload := paySubmitFundedEscrow(t, h, escrowID)
	asset := [32]byte(payload[160:192])
	if withArbiter {
		account := paySubmitMain(t, h, arbiter, asset)
		copy(payload[128:160], account[:])
	}
	binary.BigEndian.PutUint64(payload[216:224], uint64(payHarnessTimeMs+10000))
	_, err := h.keeper.Submit(h.ctx, h.actors[0].address, "escrowOpen", payload, [32]byte{153})
	require.NoError(t, err)
	if disputed {
		_, err = h.keeper.EscrowDisputeOpen(h.ctx, h.actors[0].address, escrowID, [32]byte{154})
		require.NoError(t, err)
	}
	h.keeper.SetParams(h.ctx, types.Params{EscrowEnabled: true, ServiceEnabled: true})
	gate, err := types.KernelModuleEnableKey(types.ServiceModuleID)
	require.NoError(t, err)
	h.ctx.KVStore(h.hostKey).Set(gate, payGateEnabledRecord())
	escrow, found := h.keeper.GetEscrow(h.ctx, escrowID)
	require.True(t, found)
	offerID, terms, specification := [32]byte{155}, [32]byte{156}, [32]byte{157}
	h.ctx.KVStore(h.hostKey).Set(serviceKey("offer::1", offerID), serviceViewWire(t, 252,
		offerID, [32]byte{158}, escrow.Beneficiary, asset, (codec.U128{Lo: 50}).Bytes(),
		terms, specification, uint64(payHarnessTimeMs+1000), uint64(1000), uint64(8000),
		uint8(2), uint64(payHarnessTimeMs+10000), uint64(1), false, true))
	state := uint8(5)
	if disputed {
		state = 6
	}
	h.ctx.KVStore(h.hostKey).Set(serviceKey("agree::1", agreementID), serviceViewWire(t, 247,
		agreementID, offerID, escrow.Beneficiary, escrow.Owner, terms, escrowID,
		uint64(payHarnessTimeMs+1000), uint64(payHarnessTimeMs+2000), uint64(payHarnessTimeMs+10000),
		uint8(2), state, uint64(2), uint16(1), false, uint64(3), uint64(payHarnessTimeMs), uint8(0)))
	if disputed {
		h.ctx.KVStore(h.hostKey).Set(serviceKey("disput:1", disputeID), serviceViewWire(t, 217,
			disputeID, [32]byte{159}, agreementID, escrow.Owner, uint64(4), false,
			uint16(0), uint32(0), [32]byte{}, uint64(0), uint8(1), [32]byte{160}))
	}
	return h, arbiter, escrowID, agreementID, disputeID
}

func TestServiceDisputeResolvedByEscrowArbiter(t *testing.T) {
	t.Run("open", func(t *testing.T) {
		h, arbiter, escrowID, agreementID, disputeID := serviceDisputeSetup(t, true, false)
		t.Run("nonparty", func(t *testing.T) {
			hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
			moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
			_, err := h.keeper.ServiceDisputeOpen(h.ctx, arbiter.address, disputeID, agreementID, [][32]byte{{161}})
			require.ErrorIs(t, err, lxphost.Code(-728))
			require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
			require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
		})
		evidence := [][32]byte{{162}, {161}}
		receipt, err := h.keeper.ServiceDisputeOpen(h.ctx, h.actors[0].address, disputeID, agreementID, evidence)
		require.NoError(t, err)
		require.Zero(t, receipt.ResultCode)
		decoded, err := codec.DecodeReceipt(receipt.Encoded)
		require.NoError(t, err)
		require.Equal(t, uint16(5), decoded.ModuleID)
		dispute, found := h.keeper.GetDispute(h.ctx, disputeID)
		require.True(t, found)
		require.Equal(t, agreementID, dispute.AgreementID)
		require.Equal(t, [][32]byte{{161}, {162}}, dispute.EvidenceHashes)
		require.False(t, dispute.Resolved)
		require.Equal(t, receipt.GlobalSequence, dispute.GlobalSequence)
		agreement, found := h.keeper.GetAgreement(h.ctx, agreementID)
		require.True(t, found)
		require.Equal(t, uint8(6), agreement.State)
		escrow, found := h.keeper.GetEscrow(h.ctx, escrowID)
		require.True(t, found)
		require.Equal(t, codec.U128{Lo: 50}, escrow.LockedAmount)
	})
	for _, bps := range []uint32{0, 3333, 10000} {
		t.Run(fmt.Sprintf("resolve_%d", bps), func(t *testing.T) {
			h, arbiter, escrowID, agreementID, disputeID := serviceDisputeSetup(t, true, true)
			receipt, err := h.keeper.ServiceDisputeResolve(h.ctx, arbiter.address, disputeID, 2, bps, escrowID)
			require.NoError(t, err)
			require.Zero(t, receipt.ResultCode)
			decoded, err := codec.DecodeReceipt(receipt.Encoded)
			require.NoError(t, err)
			require.Equal(t, uint16(5), decoded.ModuleID)
			dispute, found := h.keeper.GetDispute(h.ctx, disputeID)
			require.True(t, found)
			require.True(t, dispute.Resolved)
			require.Equal(t, uint16(2), dispute.Ruling)
			require.Equal(t, bps, dispute.ProviderBPS)
			require.Equal(t, escrowID, dispute.EscrowResolutionID)
			require.Equal(t, receipt.GlobalSequence, dispute.ResolutionSequence)
			agreement, found := h.keeper.GetAgreement(h.ctx, agreementID)
			require.True(t, found)
			require.Equal(t, uint8(7), agreement.State)
			escrow, found := h.keeper.GetEscrow(h.ctx, escrowID)
			require.True(t, found)
			providerAmount := uint64(50) * uint64(bps) / 10000
			require.Equal(t, uint8(6), escrow.State)
			require.Equal(t, codec.U128{}, escrow.LockedAmount)
			require.Equal(t, codec.U128{Lo: providerAmount}, escrow.CapturedAmount)
			escrowSettleBalance(t, h, escrow.Owner, escrow.AssetID, 200-providerAmount)
			escrowSettleBalance(t, h, escrow.Beneficiary, escrow.AssetID, providerAmount)
			escrowSettleBalance(t, h, escrow.EscrowAccount, escrow.AssetID, 0)
			payload, err := types.EncodeServiceDisputeResolve(disputeID, 2, bps, escrowID)
			require.NoError(t, err)
			key := sha256.Sum256(append([]byte("layerxpay:serviceDisputeResolve:escrow:"), payload...))
			result, found := h.keeper.GetEscrowResult(h.ctx, key)
			require.True(t, found)
			require.Equal(t, escrowID, result.EscrowID)
			require.Equal(t, uint16(7), result.Ordinal)
			require.Equal(t, codec.U128{Lo: providerAmount}, result.Amount)
			require.Equal(t, codec.U128{Lo: 50 - providerAmount}, result.SecondaryAmount)
			require.Equal(t, receipt.GlobalSequence+1, result.GlobalSequence)
		})
	}
	t.Run("settlement_failure_is_atomic", func(t *testing.T) {
		h, arbiter, escrowID, _, disputeID := serviceDisputeSetup(t, true, true)
		_, err := h.keeper.EscrowDisputeResolve(h.ctx, arbiter.address, escrowID, 3333, [32]byte{163})
		require.NoError(t, err)
		before := payStoreSnapshot(h.ctx, h.hostKey)
		moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
		_, err = h.keeper.ServiceDisputeResolve(h.ctx, arbiter.address, disputeID, 2, 3333, escrowID)
		require.ErrorIs(t, err, lxphost.Code(-700))
		require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
		require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
	})
}

func TestServiceDisputeArbiterRefusals(t *testing.T) {
	for _, tc := range []struct {
		name        string
		withArbiter bool
		caller      int
		mismatch    bool
		code        lxphost.Code
		codeName    string
	}{
		{"buyer", true, 0, false, -738, "LXP_UNAUTHORIZED_ARBITER"},
		{"provider", true, 1, false, -738, "LXP_UNAUTHORIZED_ARBITER"},
		{"no_arbiter", false, 2, false, -737, "LXP_ARBITER_REQUIRED"},
		{"wrong_escrow", true, 2, true, -739, "LXP_ESCROW_LINK_MISMATCH"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, arbiter, escrowID, _, disputeID := serviceDisputeSetup(t, tc.withArbiter, true)
			caller := arbiter
			if tc.caller < 2 {
				caller = h.actors[tc.caller]
			}
			if tc.mismatch {
				escrowID = [32]byte{199}
			}
			hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
			moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
			receipt, err := h.keeper.ServiceDisputeResolve(h.ctx, caller.address, disputeID, 1, 5000, escrowID)
			require.ErrorIs(t, err, tc.code)
			require.ErrorContains(t, err, tc.codeName)
			require.Equal(t, tc.codeName, pcommon.LXPCodeName(int32(tc.code)))
			require.Equal(t, lxphost.Receipt{}, receipt)
			require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
			require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
		})
	}
	for _, operation := range []string{"open", "resolve"} {
		for _, link := range []struct {
			name   string
			offset int
		}{
			{"escrow_id", 0},
			{"owner", 32},
			{"beneficiary", 96},
			{"asset", 160},
			{"missing_escrow", -1},
			{"missing_offer", -2},
		} {
			t.Run(operation+"_link_"+link.name, func(t *testing.T) {
				h, arbiter, escrowID, agreementID, disputeID := serviceDisputeSetup(t, true, operation == "resolve")
				store := h.ctx.KVStore(h.hostKey)
				key := types.KernelKVKey(types.EscrowModuleID, append([]byte("hold:"), escrowID[:]...))
				switch link.offset {
				case -1:
					store.Delete(key)
				case -2:
					agreement, found := h.keeper.GetAgreement(h.ctx, agreementID)
					require.True(t, found)
					store.Delete(serviceKey("offer::1", agreement.OfferID))
				default:
					record := bytes.Clone(store.Get(key))
					record[link.offset] ^= 1
					store.Set(key, record)
				}
				hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
				moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
				var receipt lxphost.Receipt
				var err error
				if operation == "open" {
					receipt, err = h.keeper.ServiceDisputeOpen(h.ctx, h.actors[0].address, disputeID, agreementID, [][32]byte{{161}})
				} else {
					receipt, err = h.keeper.ServiceDisputeResolve(h.ctx, arbiter.address, disputeID, 1, 5000, escrowID)
				}
				require.ErrorIs(t, err, lxphost.Code(-739))
				require.ErrorContains(t, err, "LXP_ESCROW_LINK_MISMATCH")
				require.Equal(t, lxphost.Receipt{}, receipt)
				require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
				require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
			})
		}
	}
	t.Run("open_requires_arbiter", func(t *testing.T) {
		h, _, _, agreementID, disputeID := serviceDisputeSetup(t, false, false)
		before := payStoreSnapshot(h.ctx, h.hostKey)
		moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
		receipt, err := h.keeper.ServiceDisputeOpen(h.ctx, h.actors[0].address, disputeID, agreementID, [][32]byte{{161}})
		require.ErrorIs(t, err, lxphost.Code(-737))
		require.ErrorContains(t, err, "LXP_ARBITER_REQUIRED")
		require.Equal(t, lxphost.Receipt{}, receipt)
		require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
		require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
	})
}
