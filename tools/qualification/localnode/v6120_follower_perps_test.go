package localnode

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	perpsabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/perps/abi"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

type followerPerpsOrder struct {
	MarketId         *big.Int
	IsLong           bool
	OrderType        uint8
	TimeInForce      uint8
	SizeUsd          *big.Int
	Leverage         *big.Int
	LimitPrice       *big.Int
	TriggerPrice     *big.Int
	AcceptablePrice  *big.Int
	CollateralAmount *big.Int
	ReduceOnly       bool
	TakeProfitPrice  *big.Int
	StopLossPrice    *big.Int
	Deadline         uint64
	Cloid            [16]byte
}

type followerPerpsJourney struct {
	n                  *followerNetwork
	contract, tokenABI abi.ABI
	perps, token       common.Address
	trader, maker      *ecdsa.PrivateKey
	accounts           map[common.Address]bool
	orders             map[string]*big.Int
	last               *types.Receipt
	evidence           *os.File
}

func followerPerpsUSD(value int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(value), big.NewInt(1_000_000_000_000_000_000))
}

func followerPerpsTokens(value int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(value), big.NewInt(1_000_000))
}

func followerPerpsMethod(t *testing.T, contract abi.ABI, name string, argc int) abi.Method {
	t.Helper()
	for _, method := range contract.Methods {
		if method.RawName == name && len(method.Inputs) == argc {
			return method
		}
	}
	t.Fatalf("contract has no %s with %d arguments", name, argc)
	return abi.Method{}
}

func followerPerpsPack(t *testing.T, method abi.Method, args ...interface{}) []byte {
	t.Helper()
	data, err := method.Inputs.Pack(args...)
	if err != nil {
		t.Fatalf("encode %s: %v", method.Sig, err)
	}
	return append(append([]byte(nil), method.ID...), data...)
}

func newFollowerPerpsJourney(t *testing.T) *followerPerpsJourney {
	t.Helper()
	contract, err := perpsabi.ABI()
	if err != nil {
		t.Fatal(err)
	}
	tokenABI, err := abi.JSON(strings.NewReader(`[
		{"type":"function","name":"transfer","inputs":[{"type":"address"},{"type":"uint256"}],"outputs":[{"type":"bool"}]},
		{"type":"function","name":"approve","inputs":[{"type":"address"},{"type":"uint256"}],"outputs":[{"type":"bool"}]},
		{"type":"function","name":"balanceOf","inputs":[{"type":"address"}],"outputs":[{"type":"uint256"}]},
		{"type":"function","name":"allowance","inputs":[{"type":"address"},{"type":"address"}],"outputs":[{"type":"uint256"}]}
	]`))
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := os.CreateTemp("", "lx-follower-perps-*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("trading results: %s", evidence.Name())
	t.Cleanup(func() {
		if err := evidence.Close(); err != nil {
			t.Error(err)
		}
	})
	j := &followerPerpsJourney{
		n: newFollowerNetwork(t), contract: contract, tokenABI: tokenABI,
		perps:    common.HexToAddress("0x0000000000000000000000000000000000001015"),
		token:    common.HexToAddress("0x85FcD13735F4309833A503EE804ea32395851479"),
		accounts: make(map[common.Address]bool), orders: make(map[string]*big.Int), evidence: evidence,
	}
	j.trader, j.maker = j.n.freshKey(t), j.n.freshKey(t)
	source, err := crypto.HexToECDSA(strings.TrimPrefix(os.Getenv("LOCAL_NODE_KEY"), "0x"))
	if err != nil {
		t.Fatalf("load local collateral holder: %v", err)
	}
	for _, key := range []*ecdsa.PrivateKey{j.trader, j.maker} {
		address := crypto.PubkeyToAddress(key.PublicKey)
		j.send(t, "FundWallet", source, true, "transfer", address, followerPerpsTokens(20_000))
		j.send(t, "ApproveCollateral", key, true, "approve", j.perps, followerPerpsTokens(11_000))
	}
	return j
}

func (j *followerPerpsJourney) read(t *testing.T, receipt *types.Receipt, token bool, name string, args ...interface{}) []interface{} {
	t.Helper()
	contract, address := j.contract, j.perps
	if token {
		contract, address = j.tokenABI, j.token
	}
	method := followerPerpsMethod(t, contract, name, len(args))
	data := followerPerpsPack(t, method, args...)
	ctx, cancel := context.WithTimeout(j.n.ctx, 15*time.Second)
	defer cancel()
	call := ethereum.CallMsg{From: crypto.PubkeyToAddress(j.trader.PublicKey), To: &address, Data: data}
	leader, err := j.n.primary.CallContract(ctx, call, receipt.BlockNumber)
	if err != nil {
		t.Fatalf("leader %s(%v) at block %s transaction %s: %v", name, args, receipt.BlockNumber, receipt.TxHash, err)
	}
	follower, err := j.n.follower.CallContract(ctx, call, receipt.BlockNumber)
	if err != nil {
		t.Fatalf("follower %s(%v) at block %s transaction %s: %v", name, args, receipt.BlockNumber, receipt.TxHash, err)
	}
	if !bytes.Equal(leader, follower) {
		t.Fatalf("state differs for %s(%v) at block %s transaction %s: leader=%x follower=%x", name, args, receipt.BlockNumber, receipt.TxHash, leader, follower)
	}
	values, err := method.Outputs.Unpack(leader)
	if err != nil {
		t.Fatalf("decode %s at block %s transaction %s: %v", name, receipt.BlockNumber, receipt.TxHash, err)
	}
	return values
}

func (j *followerPerpsJourney) send(t *testing.T, kind string, key *ecdsa.PrivateKey, token bool, name string, args ...interface{}) *types.Receipt {
	t.Helper()
	contract, address := j.contract, j.perps
	if token {
		contract, address = j.tokenABI, j.token
	}
	method := followerPerpsMethod(t, contract, name, len(args))
	data := followerPerpsPack(t, method, args...)
	tx := j.n.sign(t, key, address, new(big.Int), data)
	receipt := j.n.compareCodeReceipt(t, tx, types.ReceiptStatusSuccessful, true)
	ctx, cancel := context.WithTimeout(j.n.ctx, 15*time.Second)
	defer cancel()
	follower, err := j.n.follower.TransactionReceipt(ctx, tx.Hash())
	if err != nil || !reflect.DeepEqual(receipt, follower) {
		t.Fatalf("receipt differs at block %s transaction %s: leader=%+v follower=%+v error=%v", receipt.BlockNumber, tx.Hash(), receipt, follower, err)
	}
	leaderLogs, err := j.n.primary.FilterLogs(ctx, ethereum.FilterQuery{BlockHash: &receipt.BlockHash})
	if err != nil {
		t.Fatalf("leader logs at block %s transaction %s: %v", receipt.BlockNumber, tx.Hash(), err)
	}
	followerLogs, err := j.n.follower.FilterLogs(ctx, ethereum.FilterQuery{BlockHash: &receipt.BlockHash})
	if err != nil || !reflect.DeepEqual(leaderLogs, followerLogs) {
		t.Fatalf("block logs differ at block %s transaction %s: leader=%+v follower=%+v error=%v", receipt.BlockNumber, tx.Hash(), leaderLogs, followerLogs, err)
	}
	for _, log := range receipt.Logs {
		if log.Address != j.perps || len(log.Topics) == 0 {
			continue
		}
		if log.Topics[0] == j.contract.Events["OrderPlaced"].ID && len(log.Topics) == 4 {
			id := log.Topics[1].Big()
			j.orders[id.String()] = id
		}
		if log.Topics[0] == j.contract.Events["CollateralDeposited"].ID && len(log.Topics) == 2 {
			j.accounts[common.BytesToAddress(log.Topics[1].Bytes())] = true
		}
	}
	j.last = receipt
	j.compareState(t, receipt)
	if err := json.NewEncoder(j.evidence).Encode(map[string]interface{}{
		"kind": kind, "function": method.Sig, "block": receipt.BlockNumber.String(), "transaction": tx.Hash().Hex(),
		"leader": receipt.Status, "follower": follower.Status, "receiptsEqual": true, "logsEqual": true, "stateEqual": true, "applicationHashEqual": true,
	}); err != nil {
		t.Fatal(err)
	}
	t.Logf("%s: leader and follower applied %s at block %s transaction %s", kind, method.Sig, receipt.BlockNumber, tx.Hash())
	return receipt
}

func (j *followerPerpsJourney) compareState(t *testing.T, receipt *types.Receipt) {
	t.Helper()
	ctx, cancel := context.WithTimeout(j.n.ctx, 15*time.Second)
	defer cancel()
	for _, key := range []*ecdsa.PrivateKey{j.trader, j.maker} {
		address := crypto.PubkeyToAddress(key.PublicKey)
		j.read(t, receipt, true, "balanceOf", address)
		j.read(t, receipt, true, "allowance", address, j.perps)
		leaderBalance, err := j.n.primary.BalanceAt(ctx, address, receipt.BlockNumber)
		if err != nil {
			t.Fatal(err)
		}
		followerBalance, err := j.n.follower.BalanceAt(ctx, address, receipt.BlockNumber)
		if err != nil || leaderBalance.Cmp(followerBalance) != 0 {
			t.Fatalf("native balance differs at block %s transaction %s for %s: %v/%v error=%v", receipt.BlockNumber, receipt.TxHash, address, leaderBalance, followerBalance, err)
		}
		if !j.accounts[address] {
			continue
		}
		for _, name := range []string{"getAccount", "getUserOrderIds"} {
			j.read(t, receipt, false, name, address)
		}
		for _, name := range []string{"getAccountSummary", "getBalance", "getLockedBalance"} {
			j.read(t, receipt, false, name, address, j.token)
		}
		ids := j.read(t, receipt, false, "getUserPositionIds", address)[0].([]*big.Int)
		for _, id := range ids {
			for _, name := range []string{"getPosition", "getPositionDetails", "getPositionMargin"} {
				j.read(t, receipt, false, name, id)
			}
		}
	}
	ids := make([]string, 0, len(j.orders))
	for id := range j.orders {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		j.read(t, receipt, false, "getOrder", j.orders[id])
		j.read(t, receipt, false, "getOrderDetails", j.orders[id])
	}
	j.read(t, receipt, false, "getOpenInterest", new(big.Int))
	j.read(t, receipt, false, "getMarkPriceDetails", new(big.Int))
}

func followerPerpsField(t *testing.T, values []interface{}, name string) interface{} {
	t.Helper()
	if len(values) != 1 {
		t.Fatalf("expected one tuple, got %v", values)
	}
	field := reflect.ValueOf(values[0]).FieldByName(name)
	if !field.IsValid() {
		t.Fatalf("tuple %T has no field %s", values[0], name)
	}
	return field.Interface()
}

func followerPerpsEqual(t *testing.T, label string, got, want *big.Int) {
	t.Helper()
	if got.Cmp(want) != 0 {
		t.Fatalf("%s=%s, want %s on both nodes", label, got, want)
	}
}

func (j *followerPerpsJourney) event(t *testing.T, receipt *types.Receipt, name string, account common.Address) *types.Log {
	t.Helper()
	for _, log := range receipt.Logs {
		if log.Address == j.perps && len(log.Topics) > 0 && log.Topics[0] == j.contract.Events[name].ID {
			if account == (common.Address{}) || (len(log.Topics) > 2 && common.BytesToAddress(log.Topics[2].Bytes()) == account) {
				return log
			}
		}
	}
	t.Fatalf("%s event absent at block %s transaction %s", name, receipt.BlockNumber, receipt.TxHash)
	return nil
}

func (j *followerPerpsJourney) order(kind uint8, isLong bool, price int64, cloid byte) followerPerpsOrder {
	return followerPerpsOrder{
		MarketId: new(big.Int), IsLong: isLong, OrderType: kind, SizeUsd: followerPerpsUSD(2_000),
		Leverage: followerPerpsUSD(2), LimitPrice: followerPerpsUSD(price), TriggerPrice: new(big.Int),
		AcceptablePrice: new(big.Int), CollateralAmount: followerPerpsTokens(1_000),
		TakeProfitPrice: new(big.Int), StopLossPrice: new(big.Int), Cloid: [16]byte{15: cloid},
	}
}

func (j *followerPerpsJourney) position(t *testing.T, id *big.Int) []interface{} {
	t.Helper()
	return j.read(t, j.last, false, "getPosition", id)
}

func (j *followerPerpsJourney) accountCollateral(t *testing.T) *big.Int {
	t.Helper()
	return followerPerpsField(t, j.read(t, j.last, false, "getAccount", crypto.PubkeyToAddress(j.trader.PublicKey)), "Collateral").(*big.Int)
}

func (j *followerPerpsJourney) wallet(t *testing.T) *big.Int {
	t.Helper()
	return j.read(t, j.last, true, "balanceOf", crypto.PubkeyToAddress(j.trader.PublicKey))[0].(*big.Int)
}

func (j *followerPerpsJourney) open(t *testing.T, kind string) *types.Receipt {
	t.Helper()
	quote := j.read(t, j.last, false, "quoteOpenPosition", new(big.Int), j.token, followerPerpsTokens(300), followerPerpsUSD(2), true)
	entry := followerPerpsField(t, quote, "EntryPrice").(*big.Int)
	order := j.order(4, true, 0, 0)
	order.TimeInForce = 1
	order.CollateralAmount = followerPerpsTokens(300)
	order.SizeUsd = followerPerpsField(t, quote, "SizeUsd").(*big.Int)
	order.AcceptablePrice = new(big.Int).Quo(new(big.Int).Mul(entry, big.NewInt(10_050)), big.NewInt(10_000))
	order.Deadline = uint64(time.Now().Unix() + 60)
	return j.send(t, kind, j.trader, false, "placeOrder", order)
}

func TestLocalNodeV6120FollowerPerpsTradingApplication(t *testing.T) {
	j := newFollowerPerpsJourney(t)
	trader := crypto.PubkeyToAddress(j.trader.PublicKey)
	var positionID, askID, limitID, stopID *big.Int
	run := func(name string, test func(*testing.T)) {
		t.Helper()
		if !t.Run(name, test) {
			t.Fatalf("trading journey requires %s to apply on both nodes", name)
		}
	}
	run("Deposit", func(t *testing.T) {
		before := j.wallet(t)
		r := j.send(t, "Deposit", j.trader, false, "deposit", j.token, followerPerpsTokens(10_000))
		j.event(t, r, "CollateralDeposited", common.Address{})
		followerPerpsEqual(t, "collateral", j.accountCollateral(t), followerPerpsTokens(10_000))
		followerPerpsEqual(t, "wallet after deposit", j.wallet(t), new(big.Int).Sub(before, followerPerpsTokens(10_000)))
	})
	run("Open", func(t *testing.T) {
		j.send(t, "MakerDeposit", j.maker, false, "deposit", j.token, followerPerpsTokens(10_000))
		config := j.read(t, j.last, false, "getMarketConfig", new(big.Int))
		if followerPerpsField(t, config, "Enabled") != true {
			t.Fatal("BTC market must be enabled")
		}
		price := j.read(t, j.last, false, "getMarkPriceDetails", new(big.Int))
		if price[3] != true {
			t.Fatal("BTC price must be fresh")
		}
		followerPerpsEqual(t, "BTC index", price[1].(*big.Int), followerPerpsUSD(60_000))
		maker := j.send(t, "MakerAsk", j.maker, false, "placeOrder", j.order(0, false, 60_000, 1))
		askID = j.event(t, maker, "OrderPlaced", common.Address{}).Topics[1].Big()
		r := j.open(t, "Open")
		positionID = j.event(t, r, "PositionOpened", trader).Topics[1].Big()
		position := j.position(t, positionID)
		if position[0] != trader || position[2] != true || position[9] != true || position[3].(*big.Int).Sign() <= 0 {
			t.Fatalf("long position on both nodes: %v", position)
		}
		j.event(t, r, "Fill", common.Address{})
	})
	run("AddSize", func(t *testing.T) {
		before := j.position(t, positionID)[3].(*big.Int)
		r := j.open(t, "AddSize")
		j.event(t, r, "PositionModified", common.Address{})
		after := j.position(t, positionID)[3].(*big.Int)
		if after.Cmp(before) <= 0 {
			t.Fatalf("position size after increase=%s, before=%s on both nodes", after, before)
		}
		ids := j.read(t, r, false, "getUserPositionIds", trader)[0].([]*big.Int)
		if len(ids) != 1 || ids[0].Cmp(positionID) != 0 {
			t.Fatalf("increase must retain position %s on both nodes: %v", positionID, ids)
		}
	})
	run("TakeProfit", func(t *testing.T) {
		r := j.send(t, "TakeProfit", j.trader, false, "setPositionTpSl", positionID, followerPerpsUSD(70_000), new(big.Int))
		j.event(t, r, "PositionTpSlSet", common.Address{})
		details := j.read(t, r, false, "getPositionDetails", positionID)
		followerPerpsEqual(t, "take profit", followerPerpsField(t, details, "TakeProfitPrice").(*big.Int), followerPerpsUSD(70_000))
		followerPerpsEqual(t, "unset stop loss", followerPerpsField(t, details, "StopLossPrice").(*big.Int), new(big.Int))
	})
	run("StopLoss", func(t *testing.T) {
		r := j.send(t, "StopLoss", j.trader, false, "setPositionTpSl", positionID, followerPerpsUSD(70_000), followerPerpsUSD(50_000))
		j.event(t, r, "PositionTpSlSet", common.Address{})
		details := j.read(t, r, false, "getPositionDetails", positionID)
		followerPerpsEqual(t, "take profit retained", followerPerpsField(t, details, "TakeProfitPrice").(*big.Int), followerPerpsUSD(70_000))
		followerPerpsEqual(t, "stop loss", followerPerpsField(t, details, "StopLossPrice").(*big.Int), followerPerpsUSD(50_000))
	})
	for _, action := range []struct {
		name, method string
		delta        int64
	}{
		{"AddCollateral", "addCollateral", 10}, {"RemoveCollateral", "removeCollateral", -5}, {"TopUpPosition", "topUpPosition", 10},
	} {
		run(action.name, func(t *testing.T) {
			before, wallet, size := j.accountCollateral(t), j.wallet(t), j.position(t, positionID)[3].(*big.Int)
			amount := followerPerpsTokens(action.delta)
			j.send(t, action.name, j.trader, false, action.method, positionID, new(big.Int).Abs(amount))
			followerPerpsEqual(t, "collateral", j.accountCollateral(t), new(big.Int).Add(before, amount))
			followerPerpsEqual(t, "wallet", j.wallet(t), new(big.Int).Sub(wallet, amount))
			followerPerpsEqual(t, "position size", j.position(t, positionID)[3].(*big.Int), size)
		})
	}
	run("LimitOrder", func(t *testing.T) {
		order := j.order(0, true, 40_000, 2)
		r := j.send(t, "LimitOrder", j.trader, false, "placeOrder", order)
		limitID = j.event(t, r, "OrderPlaced", trader).Topics[1].Big()
		details := j.read(t, r, false, "getOrderDetails", limitID)
		if followerPerpsField(t, details, "Status") != uint8(1) || followerPerpsField(t, details, "Cloid") != order.Cloid {
			t.Fatalf("limit order must rest with its client id on both nodes: %v", details)
		}
		followerPerpsEqual(t, "limit price", followerPerpsField(t, details, "LimitPrice").(*big.Int), order.LimitPrice)
	})
	run("StopLimitOrder", func(t *testing.T) {
		order := j.order(1, true, 66_000, 3)
		order.TriggerPrice = followerPerpsUSD(65_000)
		r := j.send(t, "StopLimitOrder", j.trader, false, "placeOrder", order)
		stopID = j.event(t, r, "OrderPlaced", trader).Topics[1].Big()
		details := j.read(t, r, false, "getOrderDetails", stopID)
		if followerPerpsField(t, details, "Status") != uint8(0) || followerPerpsField(t, details, "OrderType") != uint8(1) {
			t.Fatalf("stop-limit must wait for its trigger on both nodes: %v", details)
		}
		followerPerpsEqual(t, "stop-limit trigger", followerPerpsField(t, details, "TriggerPrice").(*big.Int), order.TriggerPrice)
		followerPerpsEqual(t, "stop-limit price", followerPerpsField(t, details, "LimitPrice").(*big.Int), order.LimitPrice)
	})
	run("Cancel", func(t *testing.T) {
		r := j.send(t, "Cancel", j.trader, false, "cancelOrder", limitID)
		j.event(t, r, "OrderCancelled", trader)
		if j.read(t, r, false, "getOrder", limitID)[10] != false {
			t.Fatal("cancelled limit order must be inactive on both nodes")
		}
	})
	run("CancelByCloid", func(t *testing.T) {
		r := j.send(t, "CancelByCloid", j.trader, false, "cancelByCloid", new(big.Int), [16]byte{15: 3})
		j.event(t, r, "OrderCancelled", trader)
		if j.read(t, r, false, "getOrder", stopID)[10] != false {
			t.Fatal("cancelled stop-limit order must be inactive on both nodes")
		}
	})
	run("PartialClose", func(t *testing.T) {
		j.send(t, "MakerCancelAsk", j.maker, false, "cancelOrder", askID)
		j.send(t, "MakerBid", j.maker, false, "placeOrder", j.order(0, true, 60_000, 4))
		before := j.position(t, positionID)[3].(*big.Int)
		size := followerPerpsUSD(60)
		quote := j.read(t, j.last, false, "quotePartialClose", positionID, size)
		bound := new(big.Int).Quo(new(big.Int).Mul(quote[0].(*big.Int), big.NewInt(9_950)), big.NewInt(10_000))
		r := j.send(t, "PartialClose", j.trader, false, "closePositionWithParams", positionID, size, bound, uint64(time.Now().Unix()+60))
		j.event(t, r, "PositionClosed", trader)
		followerPerpsEqual(t, "size after partial close", j.position(t, positionID)[3].(*big.Int), new(big.Int).Sub(before, size))
		if j.position(t, positionID)[9] != true {
			t.Fatal("partially closed position must remain active on both nodes")
		}
	})
	run("Close", func(t *testing.T) {
		quote := j.read(t, j.last, false, "quoteClosePosition", positionID)
		price := followerPerpsField(t, quote, "ExitPrice").(*big.Int)
		bound := new(big.Int).Quo(new(big.Int).Mul(price, big.NewInt(9_950)), big.NewInt(10_000))
		r := j.send(t, "Close", j.trader, false, "closePositionWithParams", positionID, new(big.Int), bound, uint64(time.Now().Unix()+60))
		log := j.event(t, r, "PositionClosed", trader)
		values, err := j.contract.Events["PositionClosed"].Inputs.NonIndexed().Unpack(log.Data)
		if err != nil || values[3] != true {
			t.Fatalf("full close event on both nodes=%v error=%v", values, err)
		}
		ids := j.read(t, r, false, "getUserPositionIds", trader)[0].([]*big.Int)
		if len(ids) != 0 {
			t.Fatalf("full close must remove the trader position on both nodes: %v", ids)
		}
		orders := j.read(t, r, false, "getUserOrderIds", trader)[0].([]*big.Int)
		if len(orders) != 0 {
			t.Fatalf("full close must cancel the attached orders on both nodes: %v", orders)
		}
	})
	run("Withdraw", func(t *testing.T) {
		account := j.read(t, j.last, false, "getAccount", trader)
		amount := followerPerpsField(t, account, "Withdrawable").(*big.Int)
		before, wallet := j.accountCollateral(t), j.wallet(t)
		if amount.Sign() <= 0 {
			t.Fatal("closed trader account must have withdrawable collateral")
		}
		r := j.send(t, "Withdraw", j.trader, false, "withdraw", j.token, amount)
		j.event(t, r, "CollateralWithdrawn", common.Address{})
		followerPerpsEqual(t, "collateral after withdrawal", j.accountCollateral(t), new(big.Int).Sub(before, amount))
		followerPerpsEqual(t, "wallet after withdrawal", j.wallet(t), new(big.Int).Add(wallet, amount))
		followerPerpsEqual(t, "fully withdrawn collateral", j.accountCollateral(t), new(big.Int))
	})
	if err := j.evidence.Sync(); err != nil {
		t.Fatal(fmt.Errorf("persist trading results: %w", err))
	}
}
