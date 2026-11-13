package keeper_test

import (
	"math/big"
	"strconv"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestHomeChainRecordedAtCreation(t *testing.T) {
	for _, id := range []uint32{0, 7} {
		t.Run(strconv.FormatUint(uint64(id), 10), func(t *testing.T) {
			c := newCustodyEnv(t)
			c.fund(recordedCreator, creationFee)
			if id != 0 {
				c.ctx = chainstypes.WithDomain(c.ctx, id)
			}
			// Ante places the domain on the transaction context; the EVM state
			// retains it, and the precompile reads that state context for keeper calls.
			stateDB := state.NewDBImpl(c.ctx, &testkeeper.EVMTestApp.EvmKeeper, false)
			received, ok := chainstypes.DomainFrom(stateDB.Ctx())
			require.Equal(t, id != 0, ok)
			require.Equal(t, id, received)
			c.ctx = stateDB.Ctx()
			market, err := c.createMarket(recordedCreator, recordedName, recordedSymbol,
				types.FeeStrategyClaim, common.Address{})
			require.NoError(t, err)
			require.Equal(t, chainstypes.ChainIDFrom(c.ctx), c.k.HomeChainByPool(c.ctx, c.pool(market.Denom)))
		})
	}
}

func TestHomeChainTradeAcceptedOnItsChain(t *testing.T) {
	s := newSwapEnv(t, nil)
	const id uint32 = 7
	s.k.SetHomeChainByPool(s.ctx, s.pool, chainstypes.ChainIDOf(id).Uint64())
	s.transfer(s.holder, s.pool, 1_000_000)
	s.ctx = chainstypes.WithDomain(s.ctx, id)
	before := s.market(s.denom)
	result, err := s.swap(true, b(1_000_000), b(1), s.recipient, farDeadline)
	require.NoError(t, err)
	require.Positive(t, result.AmountOut.Sign())
	require.True(t, s.market(s.denom).TokenReserve.LT(before.TokenReserve))
	s.tokenTransfer(s.token, s.holder, s.pool, b(1_000_000))
	result, err = s.swap(false, b(1_000_000), b(1), s.recipient, farDeadline)
	require.NoError(t, err)
	require.Positive(t, result.AmountOut.Sign())
}

func TestHomeChainTradeRefusedOnRootAndOnAnotherChain(t *testing.T) {
	for _, id := range []uint32{0, 8} {
		t.Run(strconv.FormatUint(uint64(id), 10), func(t *testing.T) {
			s := newSwapEnv(t, nil)
			s.k.SetHomeChainByPool(s.ctx, s.pool, chainstypes.ChainIDOf(7).Uint64())
			s.transfer(s.holder, s.pool, 1_000_000)
			s.ctx = chainstypes.WithDomain(s.ctx, id)
			before := s.market(s.denom)
			balance := s.balanceOf(s.pool)
			_, err := s.swap(true, b(1_000_000), b(1), s.recipient, farDeadline)
			require.ErrorIs(t, err, types.ErrWrongChain)
			sameJSON(t, before, s.market(s.denom))
			require.Equal(t, balance, s.balanceOf(s.pool))
			require.Zero(t, s.ledger(keeper.HeldQuoteLedger(s.pool)))
		})
	}
}

func TestHomeChainRootMarketTradesOnEveryChain(t *testing.T) {
	for _, id := range []uint32{0, 7, 8} {
		t.Run(strconv.FormatUint(uint64(id), 10), func(t *testing.T) {
			s := newSwapEnv(t, nil)
			require.Zero(t, s.k.HomeChainByPool(s.ctx, s.pool))
			s.transfer(s.holder, s.pool, 1_000_000)
			s.ctx = chainstypes.WithDomain(s.ctx, id)
			result, err := s.swap(true, b(1_000_000), b(1), s.recipient, farDeadline)
			require.NoError(t, err)
			require.Positive(t, result.AmountOut.Sign())
		})
	}
}

func TestHomeChainEachLegOfTokenForTokenIsChecked(t *testing.T) {
	// Router's sell and buy legs both enter Keeper.Swap through routerSwapFor.
	for _, isBuy := range []bool{false, true} {
		t.Run(map[bool]string{false: "sell leg", true: "buy leg"}[isBuy], func(t *testing.T) {
			s := newSwapEnv(t, nil)
			s.k.SetHomeChainByPool(s.ctx, s.pool, chainstypes.ChainIDOf(7).Uint64())
			if isBuy {
				s.transfer(s.holder, s.pool, 1_000_000)
			} else {
				s.tokenTransfer(s.token, s.holder, s.pool, b(1_000_000))
			}
			s.ctx = chainstypes.WithDomain(s.ctx, 8)
			before := s.market(s.denom)
			_, err := s.swap(isBuy, b(1_000_000), b(0), s.recipient, farDeadline)
			require.ErrorIs(t, err, types.ErrWrongChain)
			sameJSON(t, before, s.market(s.denom))
		})
	}
}

func TestHomeChainReadsAreServedOnEveryChain(t *testing.T) {
	s := newSwapEnv(t, nil)
	s.transfer(s.holder, s.pool, 1_000_000)
	_, err := s.swap(true, b(1_000_000), b(1), s.recipient, farDeadline)
	require.NoError(t, err)
	s.k.SetHomeChainByPool(s.ctx, s.pool, chainstypes.ChainIDOf(7).Uint64())
	wantMarket := s.market(s.denom)
	wantBuy, err := s.k.QuoteBuy(s.ctx, s.denom, big.NewInt(1_000_000))
	require.NoError(t, err)
	wantSell, err := s.k.QuoteSell(s.ctx, s.denom, big.NewInt(1_000_000))
	require.NoError(t, err)
	for _, id := range []uint32{0, 7, 8} {
		ctx := chainstypes.WithDomain(s.ctx, id)
		market, found := s.k.GetMarket(ctx, s.denom)
		require.True(t, found)
		sameJSON(t, wantMarket, market)
		buy, err := s.k.QuoteBuy(ctx, s.denom, big.NewInt(1_000_000))
		require.NoError(t, err)
		require.Equal(t, wantBuy.AmountOut.String(), buy.AmountOut.String())
		sell, err := s.k.QuoteSell(ctx, s.denom, big.NewInt(1_000_000))
		require.NoError(t, err)
		require.Equal(t, wantSell.AmountOut.String(), sell.AmountOut.String())
	}
}
