package store

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/decode"
)

func spotBoardRegister(c *chain, token common.Address, stable bool) {
	c.t.Helper()
	f := c.fact("token_registered", traderA, decode.SpotQuoterAddress, 0)
	c.put(decode.SpotTokenRegisteredRow{Fact: f, Token: token, Decimals: 18, IsStablecoin: stable},
		"token, decimals, is_stablecoin", token.Bytes(), 18, stable)
}

func spotBoardMarket(c *chain, account common.Address, buy bool, stable, tokens string) {
	c.t.Helper()
	f := c.fact("market_order", account, decode.SpotQuoterAddress, 0)
	c.put(decode.SpotMarketOrderRow{Fact: f, User: account, Stablecoin: USDL, Token: tokenPAX,
		StablecoinAmount: stable, TokenAmount: tokens, IsBuy: buy},
		`"user", stablecoin, token, stablecoin_amount, token_amount, is_buy`,
		account.Bytes(), usdlBytes, tokenPAX.Bytes(), stable, tokens, buy)
}

func spotBoardSimple(c *chain, account, in, out common.Address, amountIn, amountOut string) {
	c.t.Helper()
	f := c.fact("simple_swap", account, decode.SpotQuoterAddress, 0)
	c.put(decode.SpotSimpleSwapRow{Fact: f, User: account, TokenIn: in, TokenOut: out,
		AmountIn: amountIn, AmountOut: amountOut, ExecutionPrice: "1"},
		`"user", token_in, token_out, amount_in, amount_out, execution_price`,
		account.Bytes(), in.Bytes(), out.Bytes(), amountIn, amountOut, "1")
}

func TestFoldSpotBoard(t *testing.T) {
	db := testDB(t)
	migrate(t, db)
	at := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)
	const accountSQL = `SELECT realized_usd::text, volume_usd::text, trades::text, updated_block::text
FROM perps.spot_account_legs WHERE account = $1`
	wantRank := func(c *chain, kind string, rank int, account common.Address, value string, block int) {
		t.Helper()
		c.want(`SELECT (account = $3)::text, (value = $4::numeric)::text, (updated_block = $5)::text
FROM perps.spot_leaderboard WHERE kind = $1 AND rank = $2`, []string{"true", "true", "true"}, kind, rank, account.Bytes(), value, block)
	}

	t.Run("two_blocks", func(t *testing.T) {
		c := newChain(t, db)
		c.next(10, at)
		spotBoardRegister(c, USDL, true)
		spotBoardRegister(c, tokenPAX, false)
		spotBoardMarket(c, traderA, true, "100", "10")
		spotBoardSimple(c, traderA, USDL, tokenPAX, "50", "10")
		spotBoardMarket(c, traderB, true, "200", "10")
		c.project()
		c.want(accountSQL, []string{"0", "150", "2", "10"}, traderA.Bytes())
		c.want(accountSQL, []string{"0", "200", "1", "10"}, traderB.Bytes())
		wantRank(c, "pnl", 1, traderB, "0", 10)
		wantRank(c, "pnl", 2, traderA, "0", 10)
		c.want(`SELECT count(*)::text FROM perps.spot_leaderboard WHERE kind = 'win-rate'`, []string{"0"})

		c.next(11, at.Add(time.Minute))
		spotBoardSimple(c, traderA, tokenPAX, USDL, "8", "80")
		spotBoardMarket(c, traderB, false, "150", "5")
		replay := append([]decode.Row(nil), c.rows...)
		c.project()
		// The request fold consumes 8/20 of A's 150 basis and 5/10 of B's 200.
		c.want(accountSQL, []string{"20", "230", "3", "11"}, traderA.Bytes())
		c.want(accountSQL, []string{"50", "350", "2", "11"}, traderB.Bytes())
		for _, kind := range []string{"pnl", "volume", "win-rate"} {
			values := map[string][]string{"pnl": {"50", "20"}, "volume": {"350", "230"}, "win-rate": {"100", "100"}}
			wantRank(c, kind, 1, traderB, values[kind][0], 11)
			wantRank(c, kind, 2, traderA, values[kind][1], 11)
		}
		if err := Project(c.ctx, c.tx, replay); err != nil {
			t.Fatalf("reproject: %v", err)
		}
		if err := Project(c.ctx, c.tx, nil); err != nil {
			t.Fatalf("empty project: %v", err)
		}
		c.want(accountSQL, []string{"20", "230", "3", "11"}, traderA.Bytes())

		c.next(12, at.Add(2*time.Minute))
		spotBoardMarket(c, traderA, false, "120", "12")
		if _, err := c.tx.ExecContext(c.ctx, `SAVEPOINT spot_board`); err != nil {
			t.Fatal(err)
		}
		if err := Project(c.ctx, c.tx, c.rows); err != nil {
			t.Fatal(err)
		}
		wantRank(c, "pnl", 2, traderA, "50", 12)
		if _, err := c.tx.ExecContext(c.ctx, `ROLLBACK TO SAVEPOINT spot_board`); err != nil {
			t.Fatal(err)
		}
		c.want(accountSQL, []string{"20", "230", "3", "11"}, traderA.Bytes())
		wantRank(c, "pnl", 2, traderA, "20", 11)
		c.project()
		c.want(accountSQL, []string{"50", "350", "4", "12"}, traderA.Bytes())
		c.want(accountSQL, []string{"50", "350", "2", "11"}, traderB.Bytes())
		for _, kind := range []string{"pnl", "volume", "win-rate"} {
			value := map[string]string{"pnl": "50", "volume": "350", "win-rate": "100"}[kind]
			wantRank(c, kind, 1, traderB, value, 12)
			wantRank(c, kind, 2, traderA, value, 12)
		}

		c.next(13, at.Add(3*time.Minute))
		spotBoardMarket(c, traderA, false, "10", "1")
		c.project()
		c.want(`SELECT count(*)::text FROM perps.spot_account_legs WHERE account = $1`, []string{"0"}, traderA.Bytes())
		c.want(`SELECT count(*)::text FROM perps.spot_leaderboard`, []string{"3"})
		wantRank(c, "pnl", 1, traderB, "50", 13)
	})

	t.Run("gross_order_fills", func(t *testing.T) {
		c := newChain(t, db)
		c.next(20, at)
		spotBoardRegister(c, USDL, true)
		spotBoardRegister(c, tokenPAX, false)
		spotBoardMarket(c, traderA, true, "100", "10")
		f := c.fact("limit_created", traderA, spotLimit, 0)
		c.put(decode.SpotLimitCreatedRow{Fact: f, OrderID: "1", User: traderA, OrderType: spotLimitBuy,
			Amount: "30", TargetPrice: "5000000000000000000", Token: tokenPAX, Stablecoin: USDL},
			`order_id, "user", order_type, amount, target_price, token, stablecoin`,
			"1", traderA.Bytes(), spotLimitBuy, "30", "5000000000000000000", tokenPAX.Bytes(), usdlBytes)
		f = c.fact("stop_limit_created", traderA, spotStop, 0)
		c.put(decode.SpotStopLimitCreatedRow{Fact: f, OrderID: "2", User: traderA, OrderType: spotStopLimitSell,
			Amount: "4", StopPrice: "1", LimitPrice: "1", Token: tokenPAX, Stablecoin: USDL},
			`order_id, "user", order_type, amount, stop_price, limit_price, token, stablecoin`,
			"2", traderA.Bytes(), spotStopLimitSell, "4", "1", "1", tokenPAX.Bytes(), usdlBytes)
		f = c.fact("stop_loss_created", traderA, spotStop, 0)
		c.put(decode.SpotStopLossCreatedRow{Fact: f, OrderID: "3", User: traderA, Amount: "2",
			TriggerPrice: "1", Token: tokenPAX, Stablecoin: USDL},
			`order_id, "user", amount, trigger_price, token, stablecoin`,
			"3", traderA.Bytes(), "2", "1", tokenPAX.Bytes(), usdlBytes)
		c.project()
		c.next(21, at.Add(time.Minute))
		c.endBlock()
		for _, fill := range []struct{ kind, id, price string }{
			{"limit_executed", "1", "5000000000000000000"},
			{"stop_limit_executed", "2", "20000000000000000000"},
			{"stop_loss_executed", "3", "4000000000000000000"},
		} {
			f := c.fact(fill.kind, perpsVenue, spotStop, 0)
			c.put(decode.SpotOrderExecutedRow{Fact: f, OrderID: fill.id, Token: tokenPAX, Stablecoin: USDL, ExecutionPrice: fill.price},
				"order_id, token, stablecoin, execution_price", fill.id, tokenPAX.Bytes(), usdlBytes, fill.price)
		}
		c.project()
		c.want(accountSQL, []string{"40", "218", "4", "21"}, traderA.Bytes())
		wantRank(c, "pnl", 1, traderA, "40", 21)
		wantRank(c, "volume", 1, traderA, "218", 21)
		wantRank(c, "win-rate", 1, traderA, "50", 21)
		c.want(`SELECT count(*)::text FROM perps.spot_account_legs WHERE account = $1`, []string{"0"}, perpsVenue.Bytes())
	})

	t.Run("fractional_win_rate_ties", func(t *testing.T) {
		c := newChain(t, db)
		c.next(40, at)
		spotBoardRegister(c, USDL, true)
		spotBoardRegister(c, tokenPAX, false)
		for _, account := range []common.Address{traderA, traderB} {
			spotBoardMarket(c, account, true, "60", "6")
			spotBoardMarket(c, account, false, "20", "1")
			spotBoardMarket(c, account, false, "5", "1")
			spotBoardMarket(c, account, false, "30", "1")
			spotBoardMarket(c, account, false, "10", "1")
		}
		c.project()
		const rate = "66.6666666666666666666666666666666666666667"
		wantRank(c, "win-rate", 1, traderB, rate, 40)
		wantRank(c, "win-rate", 2, traderA, rate, 40)
		c.next(41, at.Add(time.Minute))
		spotBoardMarket(c, traderA, true, "10", "1")
		c.project()
		wantRank(c, "win-rate", 1, traderB, rate, 41)
		wantRank(c, "win-rate", 2, traderA, rate, 41)
		c.want(accountSQL, []string{"25", "135", "6", "41"}, traderA.Bytes())
		c.want(accountSQL, []string{"25", "125", "5", "40"}, traderB.Bytes())
	})

	t.Run("excluded_legs", func(t *testing.T) {
		c := newChain(t, db)
		c.next(30, at)
		spotBoardRegister(c, USDL, true)
		spotBoardRegister(c, tokenPAX, false)
		spotBoardSimple(c, traderA, tokenPAX, perpsVenue, "10", "10")
		spotBoardSimple(c, traderA, USDL, USDL, "10", "10")
		spotBoardMarket(c, traderA, true, "10", "0")
		f := c.fact("native_swap", traderB, decode.SpotQuoterAddress, 0)
		c.put(decode.SpotNativeSwapRow{Fact: f, User: traderB, TokenOther: USDL, NativeAmount: "10", TokenAmount: "100", NativeIsInput: false},
			`"user", token_other, native_amount, token_amount, native_is_input`, traderB.Bytes(), usdlBytes, "10", "100", false)
		c.project()
		c.want(`SELECT count(*)::text FROM perps.spot_account_legs`, []string{"0"})
		c.want(`SELECT count(*)::text FROM perps.spot_leaderboard`, []string{"0"})
	})
}
