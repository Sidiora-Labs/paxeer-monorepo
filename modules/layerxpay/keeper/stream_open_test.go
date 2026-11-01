package keeper

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/stretchr/testify/require"
)

func payStreamOpenFixture(t *testing.T, h *payHarness) types.StreamOpen {
	t.Helper()
	actor := h.actors[0]
	salt := [32]byte{0x73}
	asset := sha256.Sum256(append(append([]byte("LX:ASSET:v1"), actor.id[:]...), salt[:]...))
	main := paySubmitMain(t, h, actor, asset)
	beneficiary := paySubmitMain(t, h, h.actors[1], asset)
	assetName, assetAccount, err := lxphost.AssetAccountID(actor.public, asset)
	require.NoError(t, err)
	payload := append([]byte{0, 1}, asset[:]...)
	payload = append(payload, salt[:]...)
	payload = append(payload, 1, 'T', 1, 'T', 6)
	payload = binary.BigEndian.AppendUint64(payload, 0)
	payload = binary.BigEndian.AppendUint64(payload, 1000)
	payload = append(payload, 1, 0)
	paySubmitExecute(t, h, actor, 0x00010001, payload, [32]byte{40})
	paySubmitExecute(t, h, actor, 0x00010004, append([]byte{0, 1}, asset[:]...), [32]byte{41})
	payload = append(append([]byte{0, 1}, asset[:]...), assetAccount[:]...)
	payload = binary.BigEndian.AppendUint64(payload, 0)
	payload = binary.BigEndian.AppendUint64(payload, 200)
	paySubmitExecute(t, h, actor, 0x0001000a, payload, [32]byte{42})

	var amount [16]byte
	binary.BigEndian.PutUint64(amount[8:], 200)
	key := [32]byte{0x74}
	material := append(append(append(append([]byte(nil), assetAccount[:]...), main[:]...), asset[:]...), amount[:]...)
	contextHash, err := codec.DomainHash(codec.DomainContextHash, append(material, key[:]...))
	require.NoError(t, err)
	account, err := h.host.ReadAccount(h.ctx, assetName, assetAccount)
	require.NoError(t, err)
	transfer := binary.BigEndian.AppendUint64(material, account.NextSequence)
	transfer = append(transfer, key[:]...)
	transfer = binary.BigEndian.AppendUint64(transfer, uint64(payHarnessTimeMs)+10000)
	transfer = append(transfer, contextHash[:]...)
	transfer = append(transfer, 0)
	suffix := binary.BigEndian.AppendUint16(binary.BigEndian.AppendUint32(nil, 7), 3)
	message := append([]byte{0x53, 1}, transfer...)
	message = append(message, 1)
	message = append(message, assetAccount[:]...)
	message = append(message, contextHash[:]...)
	message = append(message, suffix...)
	digest, err := codec.DomainHash(codec.DomainSignaturePreimage, message)
	require.NoError(t, err)
	payload = append([]byte{0x53, 1, 0, 10}, transfer...)
	payload = append(payload, 1)
	payload = append(payload, assetAccount[:]...)
	payload = append(payload, actor.public[:]...)
	payload = append(payload, ed25519.Sign(actor.private, digest[:])...)
	payload = append(payload, contextHash[:]...)
	payload = append(payload, suffix...)
	paySubmitExecute(t, h, actor, 0x00010005, payload, key)
	balance, err := h.host.ReadBalance(h.ctx, main, asset)
	require.NoError(t, err)
	require.Equal(t, "200", balance.String())

	h.keeper.SetParams(h.ctx, types.Params{StreamEnabled: true})
	gate, err := types.KernelModuleEnableKey(types.StreamModuleID)
	require.NoError(t, err)
	h.ctx.KVStore(h.hostKey).Set(gate, payGateEnabledRecord())
	id := [32]byte{0x51}
	custody, err := codec.DeriveAccountID([]byte("agent:" + string(actor.did) + ":stream:" + hex.EncodeToString(id[:])))
	require.NoError(t, err)
	return types.StreamOpen{
		StreamID: id, StreamAccount: custody, Recipient: beneficiary, AssetID: asset,
		Mode: types.StreamModeTime, Rate: codec.U128{Lo: 2}, RateUnit: 1000,
		StartTimestamp: uint64(payHarnessTimeMs), EndTimestamp: uint64(payHarnessTimeMs) + 100000,
		TotalCap: codec.U128{Lo: 1000}, InitialFunding: codec.U128{Lo: 50},
	}
}

func payStreamExpectedReceipt(t *testing.T, h *payHarness, activityType uint32, payload []byte, key [32]byte) lxphost.Receipt {
	t.Helper()
	actor := h.actors[0]
	last, err := h.host.ReadLastSequence(h.ctx, actor.did)
	require.NoError(t, err)
	sequence, err := h.host.ReadIdentitySequence(h.ctx, actor.did)
	require.NoError(t, err)
	_, principal, err := lxphost.MainAccountID(actor.public)
	require.NoError(t, err)
	authority := actor.authority()
	authority.Principal, authority.Bound = principal, true
	branch, _ := h.ctx.CacheContext()
	expected, err := h.host.HostExecute(lxphost.WithSequence(branch, last+1), lxphost.Activity{
		ProtocolVersion: 3, Type: activityType, ActorDID: actor.did, AuthorityKey: actor.public,
		AccountSequence: sequence, NotBefore: uint64(h.ctx.BlockTime().UnixMilli()),
		NotAfter:       uint64(h.ctx.BlockTime().UnixMilli()) + h.host.TimestampWindowMs(),
		IdempotencyKey: key, Payload: payload,
	}, authority)
	require.NoError(t, err)
	require.Zero(t, expected.ResultCode)
	return expected
}

func payStreamBalances(t *testing.T, h *payHarness, open types.StreamOpen, payerWant, streamWant string) {
	t.Helper()
	_, payer, err := lxphost.MainAccountID(h.actors[0].public)
	require.NoError(t, err)
	for _, entry := range []struct {
		account [32]byte
		want    string
	}{{payer, payerWant}, {open.StreamAccount, streamWant}, {open.Recipient, "0"}} {
		balance, err := h.host.ReadBalance(h.ctx, entry.account, open.AssetID)
		require.NoError(t, err)
		require.Equal(t, entry.want, balance.String())
	}
}

func TestStreamOpenFundsFromPayer(t *testing.T) {
	for _, count := range []int{0, 1, types.StreamMaxMeterAuthorities} {
		t.Run(fmt.Sprintf("meter_authorities_%d", count), func(t *testing.T) {
			h := newPayHarness(t)
			open := payStreamOpenFixture(t, h)
			open.MeterAuthorities = make([][32]byte, count)
			if count != 0 {
				open.Mode = types.StreamModeMetered
				for i := range open.MeterAuthorities {
					open.MeterAuthorities[i] = [32]byte{byte(i + 1)}
				}
			}
			payload, err := types.EncodeStreamOpen(open)
			require.NoError(t, err)
			require.Len(t, payload, 204+32*count)
			require.Equal(t, byte(count), payload[203])
			expected := payStreamExpectedReceipt(t, h, 0x00040001, payload, open.StreamID)
			moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
			receipt, err := h.keeper.StreamOpen(h.ctx, h.actors[0].address, open)
			require.NoError(t, err)
			require.Equal(t, expected, receipt)
			decoded, err := codec.DecodeReceipt(receipt.Encoded)
			require.NoError(t, err)
			require.Equal(t, types.StreamModuleID, decoded.ModuleID)
			require.NotEqual(t, receipt.PreviousStateRoot, receipt.ResultingStateRoot)
			payStreamBalances(t, h, open, "150", "50")
			_, payer, err := lxphost.MainAccountID(h.actors[0].public)
			require.NoError(t, err)
			record, err := h.keeper.GetStream(h.ctx, open.StreamID)
			require.NoError(t, err)
			require.Equal(t, types.StreamRecord{
				StreamID: open.StreamID, Payer: payer, StreamAccount: open.StreamAccount,
				Recipient: open.Recipient, AssetID: open.AssetID, Mode: open.Mode,
				Rate: open.Rate, RateUnit: open.RateUnit, StartTimestamp: open.StartTimestamp,
				LastAccrualTimestamp: open.StartTimestamp, EndTimestamp: open.EndTimestamp,
				TotalCap: open.TotalCap, MeterAuthorities: open.MeterAuthorities,
			}, record)
			require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
			before := payStoreSnapshot(h.ctx, h.hostKey)
			_, err = h.keeper.StreamOpen(h.ctx, h.actors[0].address, open)
			require.Error(t, err)
			require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
		})
	}

	t.Run("invalid_payloads_do_not_write", func(t *testing.T) {
		h := newPayHarness(t)
		valid := payStreamOpenFixture(t, h)
		for _, edit := range []func(*types.StreamOpen){
			func(o *types.StreamOpen) { o.StreamID = [32]byte{} },
			func(o *types.StreamOpen) { o.InitialFunding = codec.U128{} },
			func(o *types.StreamOpen) {
				o.Mode = types.StreamModeMetered
				o.MeterAuthorities = make([][32]byte, 9)
				for i := range o.MeterAuthorities {
					o.MeterAuthorities[i] = [32]byte{byte(i + 1)}
				}
			},
		} {
			open := valid
			edit(&open)
			before := payStoreSnapshot(h.ctx, h.hostKey)
			receipt, err := h.keeper.StreamOpen(h.ctx, h.actors[0].address, open)
			require.EqualError(t, err, "LXP_NON_CANONICAL")
			require.Equal(t, lxphost.Receipt{}, receipt)
			require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
		}
	})
}

func TestStreamTopUpAddsFunding(t *testing.T) {
	h := newPayHarness(t)
	open := payStreamOpenFixture(t, h)
	_, err := h.keeper.StreamOpen(h.ctx, h.actors[0].address, open)
	require.NoError(t, err)
	payStreamBalances(t, h, open, "150", "50")
	recordBefore, err := h.keeper.GetStream(h.ctx, open.StreamID)
	require.NoError(t, err)
	moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
	for i, amount := range []uint64{25, 10} {
		key := [32]byte{0x61, byte(i + 1)}
		payload, err := types.EncodeStreamTopUp(open.StreamID, codec.U128{Lo: amount})
		require.NoError(t, err)
		require.Len(t, payload, 50)
		require.Equal(t, open.StreamID[:], payload[2:34])
		expected := payStreamExpectedReceipt(t, h, 0x00040002, payload, key)
		receipt, err := h.keeper.StreamTopUp(h.ctx, h.actors[0].address, open.StreamID, codec.U128{Lo: amount}, key)
		require.NoError(t, err)
		require.Equal(t, expected, receipt)
		decoded, err := codec.DecodeReceipt(receipt.Encoded)
		require.NoError(t, err)
		require.Equal(t, types.StreamModuleID, decoded.ModuleID)
		if i == 0 {
			payStreamBalances(t, h, open, "125", "75")
		} else {
			payStreamBalances(t, h, open, "115", "85")
		}
	}
	recordAfter, err := h.keeper.GetStream(h.ctx, open.StreamID)
	require.NoError(t, err)
	require.Equal(t, recordBefore, recordAfter)
	require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))

	t.Run("only_payer_may_top_up", func(t *testing.T) {
		before := payStoreSnapshot(h.ctx, h.hostKey)
		receipt, err := h.keeper.StreamTopUp(h.ctx, h.actors[1].address, open.StreamID, codec.U128{Lo: 1}, [32]byte{0x62})
		require.Error(t, err)
		require.ErrorContains(t, err, "LXP_UNAUTHORIZED_DEBIT")
		require.ErrorIs(t, err, receipt.ResultCode)
		require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
	})
	t.Run("insufficient_funds_do_not_write", func(t *testing.T) {
		before := payStoreSnapshot(h.ctx, h.hostKey)
		_, err := h.keeper.StreamTopUp(h.ctx, h.actors[0].address, open.StreamID, codec.U128{Lo: 116}, [32]byte{0x63})
		require.Error(t, err)
		require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
		payStreamBalances(t, h, open, "115", "85")
	})
	t.Run("invalid_payloads_do_not_write", func(t *testing.T) {
		for _, tc := range []struct {
			id     [32]byte
			amount codec.U128
		}{
			{[32]byte{}, codec.U128{Lo: 1}}, {open.StreamID, codec.U128{}},
		} {
			before := payStoreSnapshot(h.ctx, h.hostKey)
			receipt, err := h.keeper.StreamTopUp(h.ctx, h.actors[0].address, tc.id, tc.amount, [32]byte{0x64})
			require.EqualError(t, err, "LXP_NON_CANONICAL")
			require.Equal(t, lxphost.Receipt{}, receipt)
			require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
		}
	})
}
