package keeper

import (
	"encoding/hex"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	custodytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func newAgentStreamHarness(t *testing.T, kind string) (*payHarness, types.StreamOpen, common.Address) {
	t.Helper()
	h, _ := newBudgetPullHarness(t)
	open := payStreamOpenFixture(t, h)
	token := common.HexToAddress("0x1234")
	mapping := custodytypes.AssetMapping{AssetId: hex.EncodeToString(open.AssetID[:]), Denom: "agenttoken", Enabled: true}
	switch kind {
	case "native":
		denom, err := sdk.GetBaseDenom()
		if err != nil {
			require.NoError(t, sdk.RegisterDenom("uhpx", sdk.OneDec()))
			denom = sdk.MustGetBaseDenom()
		}
		mapping.Denom = denom
		token = common.Address{}
	case "pointer":
		mapping.Pointer = token.Hex()
	case "factory":
		mapping.Denom = "factory/" + sdk.AccAddress(h.actors[0].address.Bytes()).String() + "/stream"
		require.NoError(t, h.evm.SetERC20NativePointer(h.ctx, mapping.Denom, token))
	default:
		t.Fatalf("unknown asset kind %q", kind)
	}
	require.NoError(t, h.custody.SetAsset(h.ctx, mapping))
	h.ctx = h.ctx.WithEVMTxHash(common.HexToHash("0xa901"))
	return h, open, token
}

func TestAgentStreamOpenMapsTokenAndUnits(t *testing.T) {
	for _, kind := range []string{"native", "pointer", "factory"} {
		t.Run(kind, func(t *testing.T) {
			h, fixture, token := newAgentStreamHarness(t, kind)
			payer, payee := h.actors[0], h.actors[1]
			start := uint64(payHarnessTimeMs / 1000)
			rate, cap := big.NewInt(2), big.NewInt(50)
			id, err := h.keeper.OpenAgentStream(h.ctx, payer.address, payee.address, token, rate, start, start+100, cap)
			require.NoError(t, err)
			_, payerAccount, err := lxphost.MainAccountID(payer.public)
			require.NoError(t, err)
			expectedID := types.DeriveStreamID([32]byte(h.ctx.EVMTxHash()), 0, payerAccount)
			require.Equal(t, new(big.Int).SetBytes(expectedID[:]), id)
			require.Greater(t, id.BitLen(), 160)
			record, err := h.keeper.GetStream(h.ctx, expectedID)
			require.NoError(t, err)
			expectedAccount, err := codec.DeriveAccountID([]byte("agent:" + string(payer.did) + ":stream:" + hex.EncodeToString(expectedID[:])))
			require.NoError(t, err)
			require.Equal(t, expectedID, record.StreamID)
			require.Equal(t, payerAccount, record.Payer)
			require.Equal(t, fixture.Recipient, record.Recipient)
			require.Equal(t, expectedAccount, record.StreamAccount)
			require.Equal(t, fixture.AssetID, record.AssetID)
			require.Equal(t, codec.U128{Lo: 2}, record.Rate)
			require.Equal(t, uint64(1000), record.RateUnit)
			require.Equal(t, uint64(payHarnessTimeMs), record.StartTimestamp)
			require.Equal(t, uint64(payHarnessTimeMs+100000), record.EndTimestamp)
			require.Equal(t, codec.U128{Lo: 50}, record.TotalCap)
			for _, balance := range []struct {
				account [32]byte
				want    string
			}{
				{payerAccount, "150"}, {record.StreamAccount, "50"}, {record.Recipient, "0"},
			} {
				got, err := h.host.ReadBalance(h.ctx, balance.account, fixture.AssetID)
				require.NoError(t, err)
				require.Equal(t, balance.want, got.String())
			}
			view, found, err := h.keeper.AgentStreamView(h.ctx, id)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, AgentStream{ID: id, Payer: payer.address, Payee: payee.address, Token: token,
				RatePerSecond: rate, Cap: cap, StartTime: start, StopTime: start + 100, Settled: big.NewInt(0), Active: true}, view)
			second, err := h.keeper.OpenAgentStream(h.ctx, payer.address, payee.address, token, rate, start, 0, cap)
			require.NoError(t, err)
			nextID := types.DeriveStreamID([32]byte(h.ctx.EVMTxHash()), 1, payerAccount)
			require.Equal(t, new(big.Int).SetBytes(nextID[:]), second)
			require.NotEqual(t, id, second)
			nextRecord, err := h.keeper.GetStream(h.ctx, nextID)
			require.NoError(t, err)
			require.Zero(t, nextRecord.EndTimestamp)
		})
	}
	t.Run("validation_and_refusal_are_atomic", func(t *testing.T) {
		h, _, token := newAgentStreamHarness(t, "pointer")
		start := uint64(payHarnessTimeMs / 1000)
		payer, payee := h.actors[0].address, h.actors[1].address
		for _, tc := range []struct {
			name         string
			payer, payee common.Address
			rate, cap    *big.Int
			start, stop  uint64
			want         error
		}{
			{"unbound_payee", payer, common.HexToAddress("0xbad"), big.NewInt(2), big.NewInt(50), start, start + 10, agentcommon.ErrStreamsInvalidPayee},
			{"self_payment", payer, payer, big.NewInt(2), big.NewInt(50), start, start + 10, agentcommon.ErrStreamsSelfPayment},
			{"unbound_payer", common.HexToAddress("0xbad"), payee, big.NewInt(2), big.NewInt(50), start, start + 10, types.ErrUnboundCaller},
			{"rate_overflow", payer, payee, new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(50), start, start + 10, agentcommon.ErrOverflow},
			{"cap_overflow", payer, payee, big.NewInt(2), new(big.Int).Lsh(big.NewInt(1), 128), start, start + 10, agentcommon.ErrOverflow},
			{"start_overflow", payer, payee, big.NewInt(2), big.NewInt(50), math.MaxUint64, 0, agentcommon.ErrOverflow},
			{"stop_overflow", payer, payee, big.NewInt(2), big.NewInt(50), start, math.MaxUint64, agentcommon.ErrOverflow},
			{"zero_rate", payer, payee, big.NewInt(0), big.NewInt(50), start, start + 10, nil},
			{"nil_cap", payer, payee, big.NewInt(2), nil, start, start + 10, nil},
			{"insufficient_funds", payer, payee, big.NewInt(2), big.NewInt(201), start, start + 10, nil},
			{"invalid_interval", payer, payee, big.NewInt(2), big.NewInt(50), start, start, nil},
		} {
			t.Run(tc.name, func(t *testing.T) {
				hostBefore, moduleBefore := payStoreSnapshot(h.ctx, h.hostKey), payStoreSnapshot(h.ctx, h.storeKey)
				id, err := h.keeper.OpenAgentStream(h.ctx, tc.payer, tc.payee, token, tc.rate, tc.start, tc.stop, tc.cap)
				require.Error(t, err)
				if tc.want != nil {
					require.ErrorIs(t, err, tc.want)
				}
				require.Nil(t, id)
				require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
				require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
			})
		}
		id, err := h.keeper.OpenAgentStream(h.ctx, payer, payee, token, big.NewInt(2), start, start+10, big.NewInt(50))
		require.NoError(t, err)
		_, account, err := lxphost.MainAccountID(h.actors[0].public)
		require.NoError(t, err)
		expected := types.DeriveStreamID([32]byte(h.ctx.EVMTxHash()), 0, account)
		require.Equal(t, new(big.Int).SetBytes(expected[:]), id)
	})
}

func TestAgentStreamSettleCloseRateAndView(t *testing.T) {
	h, fixture, token := newAgentStreamHarness(t, "pointer")
	payer, payee := h.actors[0], h.actors[1]
	start := uint64(payHarnessTimeMs / 1000)
	id, err := h.keeper.OpenAgentStream(h.ctx, payer.address, payee.address, token, big.NewInt(2), start, start+100, big.NewInt(100))
	require.NoError(t, err)
	var kernelID [32]byte
	id.FillBytes(kernelID[:])
	assertView := func(rate, settled int64, active bool) {
		t.Helper()
		before := payStoreSnapshot(h.ctx, h.hostKey)
		view, found, err := h.keeper.AgentStreamView(h.ctx, id)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, AgentStream{ID: id, Payer: payer.address, Payee: payee.address, Token: token,
			RatePerSecond: big.NewInt(rate), Cap: big.NewInt(100), StartTime: start, StopTime: start + 100, Settled: big.NewInt(settled), Active: active}, view)
		record, err := h.keeper.GetStream(h.ctx, kernelID)
		require.NoError(t, err)
		require.Equal(t, codec.U128{Lo: uint64(rate)}, record.Rate)
		require.Equal(t, codec.U128{Lo: uint64(settled)}, record.SettledTotal)
		require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
	}
	h.ctx = h.ctx.WithBlockHeight(2).WithBlockTime(time.UnixMilli(payHarnessTimeMs + 5000))
	paid, err := h.keeper.SettleAgentStream(h.ctx, payee.address, id)
	require.NoError(t, err)
	require.Equal(t, big.NewInt(10), paid)
	settleKey := types.DeriveIdempotencyKey([32]byte(h.ctx.EVMTxHash()), 1, [4]byte{0x8d, 0xf8, 0x28, 0x00})
	result, err := h.keeper.GetStreamResult(h.ctx, settleKey)
	require.NoError(t, err)
	require.Equal(t, kernelID, result.StreamID)
	require.Equal(t, codec.U128{Lo: 10}, result.Paid)
	assertView(2, 10, true)
	h.ctx = h.ctx.WithBlockHeight(3).WithBlockTime(time.UnixMilli(payHarnessTimeMs + 8000))
	before := payStoreSnapshot(h.ctx, h.hostKey)
	_, err = h.keeper.UpdateAgentStreamRate(h.ctx, payee.address, id, big.NewInt(4))
	require.Error(t, err)
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
	oldRate, err := h.keeper.UpdateAgentStreamRate(h.ctx, payer.address, id, big.NewInt(4))
	require.NoError(t, err)
	require.Equal(t, big.NewInt(2), oldRate)
	rateKey := types.DeriveIdempotencyKey([32]byte(h.ctx.EVMTxHash()), 2, [4]byte{0x40, 0x5a, 0xbb, 0x41})
	result, err = h.keeper.GetStreamResult(h.ctx, rateKey)
	require.NoError(t, err)
	require.Equal(t, kernelID, result.StreamID)
	require.Equal(t, codec.U128{Lo: 6}, result.Paid)
	assertView(4, 16, true)
	h.ctx = h.ctx.WithBlockHeight(4).WithBlockTime(time.UnixMilli(payHarnessTimeMs + 10000))
	paid, err = h.keeper.SettleAgentStream(h.ctx, payer.address, id)
	require.NoError(t, err)
	require.Equal(t, big.NewInt(8), paid)
	assertView(4, 24, true)
	h.ctx = h.ctx.WithBlockHeight(5).WithBlockTime(time.UnixMilli(payHarnessTimeMs + 12000))
	before = payStoreSnapshot(h.ctx, h.hostKey)
	_, _, err = h.keeper.CloseAgentStream(h.ctx, payee.address, id)
	require.Error(t, err)
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
	paid, refund, err := h.keeper.CloseAgentStream(h.ctx, payer.address, id)
	require.NoError(t, err)
	require.Equal(t, big.NewInt(8), paid)
	require.Equal(t, big.NewInt(68), refund)
	closeKey := types.DeriveIdempotencyKey([32]byte(h.ctx.EVMTxHash()), 4, [4]byte{0x0a, 0xeb, 0xeb, 0x4e})
	result, err = h.keeper.GetStreamResult(h.ctx, closeKey)
	require.NoError(t, err)
	require.Equal(t, kernelID, result.StreamID)
	require.Equal(t, codec.U128{Lo: 8}, result.Paid)
	require.Equal(t, codec.U128{Lo: 68}, result.Refunded)
	assertView(4, 32, false)
	record, err := h.keeper.GetStream(h.ctx, kernelID)
	require.NoError(t, err)
	require.True(t, record.Closed)
	for _, tc := range []struct {
		account [32]byte
		want    string
	}{
		{record.Payer, "168"}, {record.Recipient, "32"}, {record.StreamAccount, "0"},
	} {
		balance, err := h.host.ReadBalance(h.ctx, tc.account, fixture.AssetID)
		require.NoError(t, err)
		require.Equal(t, tc.want, balance.String())
	}
	_, found, err := h.keeper.AgentStreamView(h.ctx, big.NewInt(123))
	require.NoError(t, err)
	require.False(t, found)
	for _, invalid := range []*big.Int{nil, big.NewInt(0), big.NewInt(-1), new(big.Int).Lsh(big.NewInt(1), 256)} {
		before := payStoreSnapshot(h.ctx, h.hostKey)
		_, err := h.keeper.SettleAgentStream(h.ctx, payer.address, invalid)
		require.Error(t, err)
		_, _, err = h.keeper.CloseAgentStream(h.ctx, payer.address, invalid)
		require.Error(t, err)
		_, err = h.keeper.UpdateAgentStreamRate(h.ctx, payer.address, invalid, big.NewInt(4))
		require.Error(t, err)
		_, _, err = h.keeper.AgentStreamView(h.ctx, invalid)
		require.Error(t, err)
		require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
	}
}

func TestAgentStreamUnknownTokenRefused(t *testing.T) {
	h, _, _ := newAgentStreamHarness(t, "pointer")
	unknown := common.HexToAddress("0xdeadbeef")
	k := h.keeper
	k.host = nil
	start := uint64(payHarnessTimeMs / 1000)
	for _, token := range []common.Address{unknown, {}} {
		before, moduleBefore := payStoreSnapshot(h.ctx, h.hostKey), payStoreSnapshot(h.ctx, h.storeKey)
		id, err := k.OpenAgentStream(h.ctx, h.actors[0].address, h.actors[1].address, token, big.NewInt(2), start, start+10, big.NewInt(50))
		require.ErrorIs(t, err, agentcommon.ErrStreamsUnknownToken)
		require.Nil(t, id)
		require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
		require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
	}
}
