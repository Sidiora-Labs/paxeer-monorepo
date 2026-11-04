package launchpad_test

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	executionprecompiles "github.com/Sidiora-Labs/Paxeer-X-Network/engine/executor/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	launchpadkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	tokenfactorykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/launchpad"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers"
	_ "github.com/ethereum/go-ethereum/eth/tracers/native"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

var repair947AbortAddress = common.HexToAddress("0x1001")

// The callback enters the production executor's fail-fast precompile through
// an actual EVM STATICCALL; no NestedCaller or launchpad handler is replaced.
func repair947EnableAbort(s *surfaceHarness) {
	contracts := map[common.Address]vm.PrecompiledContract{}
	for address, contract := range testkeeper.EVMTestApp.EvmKeeper.CustomPrecompiles(s.ctx()) {
		contracts[address] = contract
	}
	contracts[precompile] = s.contract
	contracts[repair947AbortAddress] = executionprecompiles.FailFastSingleton
	s.evm = vm.NewEVM(s.evm.Context, s.stateDB, s.evm.ChainConfig(), s.evm.Config, contracts)
}

func repair947Word(value []byte) []byte {
	code := append([]byte{0x7f}, common.LeftPadBytes(value, 32)...)
	return append(code, common.FromHex("60005260206000f3")...)
}

func repair947AbortCode() []byte {
	// Even a caller that ignores the success bit must stop at the abort.
	code := common.FromHex("6000600060006000")
	code = append(code, 0x73)
	code = append(code, repair947AbortAddress.Bytes()...)
	code = append(code, 0x5a, 0xfa, 0x50)
	return append(code, repair947Word([]byte{1})...)
}

type repair947Branch struct {
	signature string
	code      []byte
}

// Committed EVM bytecode fixtures select real token entry points by calldata.
func repair947Dispatch(branches ...repair947Branch) []byte {
	code := common.FromHex("60003560e01c")
	patches := []int{}
	for _, branch := range branches {
		code = append(code, 0x80, 0x63)
		code = append(code, selector(branch.signature)...)
		code = append(code, 0x14, 0x61, 0, 0, 0x57)
		patches = append(patches, len(code)-3)
	}
	code = append(code, common.FromHex("5060006000fd")...)
	for i, branch := range branches {
		binary.BigEndian.PutUint16(code[patches[i]:patches[i]+2], uint16(len(code)))
		code = append(code, 0x5b, 0x50)
		code = append(code, branch.code...)
	}
	return code
}

func repair947Catcher(target common.Address) []byte {
	code := common.FromHex("36600060003760006000366000600073")
	code = append(code, target.Bytes()...)
	return append(code, common.FromHex("5af150600160005500")...)
}

func repair947RequireAbort(t *testing.T, ret []byte, err error) {
	t.Helper()
	require.Same(t, executionprecompiles.ErrInvalidPrecompileCall, err)
	abort, ok := err.(vm.AbortError)
	require.True(t, ok, "the boundary must return the actual typed abort")
	require.True(t, abort.IsAbortError())
	require.Empty(t, ret, "an abort has no ABI revert payload")
}

func TestRepairM_947_AirdropProbeAbort(t *testing.T) {
	for position := 0; position < 3; position++ {
		t.Run(fmt.Sprint(position), func(t *testing.T) {
			s := newSurfaceHarness(t)
			s.stateDB.WithCtx(s.ctx().WithBlockHeight(12))
			repair947EnableAbort(s)
			pool, token := s.pools[2], s.tokens[2]
			poolCode := repair947Word(token.Bytes())
			totalCode, balanceCode := repair947Word([]byte{100}), repair947Word([]byte{1})
			switch position {
			case 0:
				poolCode = repair947AbortCode()
			case 1:
				totalCode = repair947AbortCode()
			case 2:
				balanceCode = repair947AbortCode()
			}
			s.stateDB.SetCode(pool, repair947Dispatch(repair947Branch{"tokenAddress()", poolCode}))
			s.stateDB.SetCode(token, repair947Dispatch(
				repair947Branch{"getPastTotalSupply(uint256)", totalCode},
				repair947Branch{"getPastBalance(address,uint256)", balanceCode}))
			before, logs := s.exported(), len(s.stateDB.GetAllLogs())
			s.frames = nil
			input := s.encode(types.RoleFeeAccumulator, "supportsAirdrop", pool)
			ret, _, err := s.evm.Call(s.alice, surfaceAt(types.RoleFeeAccumulator), input, surfaceGas, uint256.NewInt(0))
			repair947RequireAbort(t, ret, err)
			probes := []callFrame{}
			abortCalls := 0
			for _, frame := range s.frames {
				if frame.to == pool || frame.to == token {
					probes = append(probes, frame)
				}
				if frame.to == repair947AbortAddress {
					abortCalls++
				}
			}
			require.Len(t, probes, position+1, "no probe may follow the aborted probe")
			require.Equal(t, 1, abortCalls)
			signatures := []string{"tokenAddress()", "getPastTotalSupply(uint256)", "getPastBalance(address,uint256)"}
			for i, probe := range probes {
				require.Equal(t, vm.STATICCALL, probe.op)
				require.Equal(t, selector(signatures[i]), probe.input[:4])
			}
			require.Equal(t, before, s.exported())
			require.Len(t, s.stateDB.GetAllLogs(), logs)
		})
	}
}

func TestRepairM_947_RouterAbortLoss(t *testing.T) {
	for _, method := range []string{"createMarketWithPermit", "buyWithPermit", "sellWithPermit", "swapTokenForTokenWithPermit", "sell"} {
		for _, batched := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/batch=%t", method, batched), func(t *testing.T) {
				s := newSurfaceHarness(t)
				repair947EnableAbort(s)
				permit := routerPermit{Value: big.NewInt(1), Deadline: deadline}
				var input []byte
				token := s.tokens[2]
				switch method {
				case "createMarketWithPermit":
					token = s.usdl
					input = s.encode(types.RoleRouter, method, "Abort Token", "ABRT", uint8(types.FeeStrategyClaim), common.Address{}, permit)
				case "buyWithPermit":
					token = s.usdl
					input = s.encode(types.RoleRouter, method, s.pools[2], big.NewInt(1), big.NewInt(0), deadline, permit)
				case "sellWithPermit":
					input = s.encode(types.RoleRouter, method, s.pools[2], big.NewInt(1), big.NewInt(0), deadline, permit)
				case "swapTokenForTokenWithPermit":
					input = s.encode(types.RoleRouter, method, s.tokens[2], s.tokens[1], big.NewInt(1), big.NewInt(0), deadline, permit)
				case "sell":
					input = s.encode(types.RoleRouter, method, s.pools[2], big.NewInt(1), big.NewInt(0), deadline)
				}
				abortedSig := "permit(address,address,uint256,uint256,uint8,bytes32,bytes32)"
				if method == "sell" {
					abortedSig = "transferFrom(address,address,uint256)"
				}
				s.stateDB.SetCode(token, repair947Dispatch(repair947Branch{abortedSig, repair947AbortCode()}))
				role := repair36Grant(t, s, "repair 947 unreachable batch entry")
				if batched {
					input = s.encode(types.RoleRouter, "multicall", [][]byte{
						s.encode(types.RoleRouter, "multicall", [][]byte{input}),
						s.encode(types.RoleRouter, "renounceRole", role, s.alice)})
				}
				before, logs := s.exported(), len(s.stateDB.GetAllLogs())
				s.frames = nil
				ret, _, err := s.evm.Call(s.alice, surfaceAt(types.RoleRouter), input, surfaceGas, uint256.NewInt(0))
				repair947RequireAbort(t, ret, err)
				tokenCalls, abortCalls := 0, 0
				for _, frame := range s.frames {
					if frame.to == token {
						tokenCalls++
						require.Equal(t, selector(abortedSig), frame.input[:4])
					}
					if frame.to == repair947AbortAddress {
						abortCalls++
					}
				}
				require.Equal(t, 1, tokenCalls, "execution cannot pull tokens after the permit abort")
				require.Equal(t, 1, abortCalls)
				require.True(t, s.keeper.HasRole(s.ctx(), surfaceAt(types.RoleRouter), role, s.alice))
				require.Equal(t, before, s.exported())
				require.Len(t, s.stateDB.GetAllLogs(), logs)
			})
		}
	}
}

func TestRepairM_947_OuterAbortConversion(t *testing.T) {
	for _, batched := range []bool{false, true} {
		for _, target := range []common.Address{precompile, surfaceAt(types.RoleRouter)} {
			t.Run(fmt.Sprintf("%s/batch=%t", target.Hex(), batched), func(t *testing.T) {
				s := newSurfaceHarness(t)
				repair947EnableAbort(s)
				catcher := named("repair 947 outer catcher")
				s.stateDB.SetCode(catcher, repair947Catcher(target))
				input := s.encode(types.RoleRouter, "sell", s.pools[2], big.NewInt(1), big.NewInt(0), deadline)
				if batched {
					input = s.encode(types.RoleRouter, "multicall", [][]byte{s.encode(types.RoleRouter, "multicall", [][]byte{input})})
				}
				// The same outer contract catches an ordinary token refusal and continues.
				s.stateDB.SetCode(s.tokens[2], emptyReverterCode)
				snap := s.stateDB.Snapshot()
				_, _, err := s.evm.Call(s.alice, catcher, input, surfaceGas, uint256.NewInt(0))
				require.NoError(t, err)
				require.Equal(t, common.BigToHash(big.NewInt(1)), s.stateDB.GetState(catcher, common.Hash{}))
				s.stateDB.RevertToSnapshot(snap)
				s.stateDB.SetCode(s.tokens[2], repair947AbortCode())
				s.frames = nil
				ret, _, err := s.evm.Call(s.alice, catcher, input, surfaceGas, uint256.NewInt(0))
				repair947RequireAbort(t, ret, err)
				require.Equal(t, common.Hash{}, s.stateDB.GetState(catcher, common.Hash{}), "the outer SSTORE must be unreachable")
				reached := false
				for _, frame := range s.frames {
					if frame.to == repair947AbortAddress {
						reached = true
					}
				}
				require.True(t, reached, "the abort must originate in the nested EVM callback")
			})
		}
	}
}

func TestRepairM_947_OptionalPermitRefusal(t *testing.T) {
	r := newRouterHarness(t)
	permit := routerPermit{Value: big.NewInt(routerBuyAmount), Deadline: deadline}
	// The real ERC20 rejects this unsigned permit, but the existing allowance
	// still pays for the buy, preserving the optional-permit contract.
	before := r.usdlBalance(r.trader)
	ret, err := r.call(r.trader, r.router, r.encode(types.RoleRouter, "buyWithPermit", r.pools[2],
		big.NewInt(routerBuyAmount), big.NewInt(0), deadline, permit))
	require.NoError(t, err)
	require.Positive(t, r.decode(types.RoleRouter, "buyWithPermit", ret)[0].(*big.Int).Sign())
	require.Equal(t, new(big.Int).Sub(before, big.NewInt(routerBuyAmount)), r.usdlBalance(r.trader))
}

func TestRepairM_947_SurfaceValue(t *testing.T) {
	assertSurfaceValueRefusals(t, newSurfaceHarness(t))
}

func repair947Logs(s *surfaceHarness, start int) []goldenLog {
	logs := []goldenLog{}
	for _, log := range s.stateDB.GetAllLogs()[start:] {
		logs = append(logs, goldenLog{Address: log.Address, Topics: append([]common.Hash(nil), log.Topics...), Data: common.CopyBytes(log.Data)})
	}
	return logs
}

func TestRepairM_947_GoldenEventPayload(t *testing.T) {
	r := newRouterHarness(t)
	r.stateDB.WithCtx(r.ctx().WithBlockHeight(12))
	r.evm.Context.BlockNumber = big.NewInt(12)
	start := len(r.stateDB.GetAllLogs())
	_, err := r.call(r.trader, r.router, r.encode(types.RoleRouter, "buy", r.pools[2],
		big.NewInt(routerBuyAmount), big.NewInt(0), deadline))
	require.NoError(t, err)
	logs := repair947Logs(r.surfaceHarness, start)
	tr := goldenTranslation{tokens: map[common.Address]bool{r.usdl: true, r.tokens[2]: true}}
	want, err := tr.events(logs, false)
	require.NoError(t, err)
	buyEvent := parseABIFile(t, "abi/Router.json").Events["Buy"]
	family := parseABIFile(t, "abi.json")
	deposited := parseABIFile(t, "abi/Treasury.json").Events["Deposited"]
	feeRecorded := parseABIFile(t, "abi/FeeAccumulator.json").Events["FeeRecorded"]
	retainedIDs, tradeIDs := []common.Hash{}, []common.Hash{}
	for _, log := range logs {
		if tr.tokens[log.Address] {
			continue
		}
		retainedIDs = append(retainedIDs, log.Topics[0])
		if log.Topics[0] == family.Events["Swap"].ID || log.Topics[0] == buyEvent.ID || log.Topics[0] == family.Events["RouterTrade"].ID {
			tradeIDs = append(tradeIDs, log.Topics[0])
		}
	}
	require.Len(t, tradeIDs, 3, "real buy produces ordered Swap, Buy and RouterTrade events")
	require.Equal(t, []common.Hash{deposited.ID, feeRecorded.ID, family.Events["Swap"].ID,
		buyEvent.ID, family.Events["RouterTrade"].ID}, retainedIDs, "include both fee events before the trade events")
	require.Len(t, want, len(retainedIDs), "comparison retains every non-token event")
	sameEvents, err := tr.events(logs, true)
	require.NoError(t, err)
	require.Equal(t, want, sameEvents)

	for _, event := range []abi.Event{deposited, feeRecorded, buyEvent, family.Events["Swap"], family.Events["RouterTrade"]} {
		t.Run(event.Name, func(t *testing.T) {
			changed := append([]goldenLog(nil), logs...)
			index := -1
			for i := range changed {
				if len(changed[i].Topics) > 0 && changed[i].Topics[0] == event.ID {
					index = i
					break
				}
			}
			require.NotEqual(t, -1, index)
			values, err := event.Inputs.NonIndexed().Unpack(changed[index].Data)
			require.NoError(t, err)
			dataIndex := 0
			changedFields := 0
			for _, arg := range event.Inputs {
				if arg.Indexed {
					continue
				}
				if arg.Type.T == abi.UintTy && arg.Type.Size == 256 {
					original := values[dataIndex].(*big.Int)
					values[dataIndex] = new(big.Int).Add(original, big.NewInt(1))
					changed[index].Data, err = event.Inputs.NonIndexed().Pack(values...)
					require.NoError(t, err)
					got, err := tr.events(changed, false)
					require.NoError(t, err)
					require.NotEqual(t, want, got, "%s.%s must affect replay", event.Name, arg.Name)
					values[dataIndex] = original
					changedFields++
				}
				dataIndex++
			}
			require.Positive(t, changedFields)
		})
	}
	reordered := append([]goldenLog(nil), logs...)
	first, last := -1, -1
	for i, log := range reordered {
		if log.Topics[0] == family.Events["Swap"].ID {
			first = i
		}
		if log.Topics[0] == family.Events["RouterTrade"].ID {
			last = i
		}
	}
	require.NotEqual(t, -1, first)
	require.NotEqual(t, -1, last)
	reordered[first], reordered[last] = reordered[last], reordered[first]
	got, err := tr.events(reordered, false)
	require.NoError(t, err)
	require.NotEqual(t, want, got, "dependent event reordering must affect replay")
	malformed := append([]goldenLog(nil), logs...)
	malformed[first].Data = append(common.CopyBytes(malformed[first].Data), 0xff)
	_, err = tr.events(malformed, false)
	require.Error(t, err, "noncanonical trailing event data must be refused")

	// Decode actual producer events and reverse only declared translations to
	// represent their H encodings; replay must recover their exact H+1 bytes.
	event := family.Events["RouterTrade"]
	var current goldenLog
	for _, log := range logs {
		if log.Topics[0] == event.ID {
			current = log
		}
	}
	previous := current
	previous.Topics = append([]common.Hash(nil), current.Topics...)
	oldEmitter, oldTrader, oldToken := named("repair 947 old emitter"), named("repair 947 old trader"), named("repair 947 old token")
	previous.Address = oldEmitter
	previous.Topics[3] = common.BytesToHash(oldTrader.Bytes())
	values, err := event.Inputs.NonIndexed().Unpack(current.Data)
	require.NoError(t, err)
	require.Equal(t, big.NewInt(12), values[len(values)-1], "event height follows the producer EVM block context")
	values[0] = oldToken // tokenIn is an unindexed address.
	values[len(values)-2] = new(big.Int).Sub(values[len(values)-2].(*big.Int), big.NewInt(7))
	values[len(values)-1] = new(big.Int).Sub(values[len(values)-1].(*big.Int), big.NewInt(1))
	previous.Data, err = event.Inputs.NonIndexed().Pack(values...)
	require.NoError(t, err)
	translated := goldenTranslation{moves: map[common.Address]common.Address{oldTrader: r.trader, oldToken: r.usdl},
		emitters: map[common.Address]bool{oldEmitter: true}, elapsed: 7, heightDelta: 1}
	expected, err := translated.events([]goldenLog{current}, false)
	require.NoError(t, err)
	actual, err := translated.events([]goldenLog{previous}, true)
	require.NoError(t, err)
	require.Equal(t, expected, actual, "translate addresses, timestamp and block number with their ABI types")

	// A uint256 whose bytes happen to look like an address is still an amount.
	// Select Buy explicitly rather than depending on ERC20 log placement.
	for _, log := range logs {
		if log.Topics[0] != buyEvent.ID {
			continue
		}
		values, err = buyEvent.Inputs.NonIndexed().Unpack(log.Data)
		require.NoError(t, err)
		numeric := log
		values[0] = new(big.Int).SetBytes(oldToken.Bytes())
		numeric.Data, err = buyEvent.Inputs.NonIndexed().Pack(values...)
		require.NoError(t, err)
		a, err := translated.events([]goldenLog{numeric}, true)
		require.NoError(t, err)
		b, err := translated.events([]goldenLog{numeric}, false)
		require.NoError(t, err)
		require.Equal(t, a, b, "amount words must never be address-translated")
	}
}

func TestRepairM_947_GoldenLogOrder(t *testing.T) {
	r := newRouterHarness(t)
	tracer, err := tracers.DefaultDirectory.New("callTracer", nil, json.RawMessage(`{"withLog":true}`), r.evm.ChainConfig())
	require.NoError(t, err)
	r.evm.Config.Tracer = tracer.Hooks
	r.stateDB.SetLogger(tracer.Hooks)
	tracer.OnTxStart(r.evm.GetVMContext(), ethtypes.NewTx(&ethtypes.LegacyTx{Gas: surfaceGas}), r.trader)
	start := len(r.stateDB.GetAllLogs())
	_, left, err := r.evm.Call(r.trader, r.router, r.encode(types.RoleRouter, "buy", r.pools[2],
		big.NewInt(routerBuyAmount), big.NewInt(0), deadline), surfaceGas, uint256.NewInt(0))
	require.NoError(t, err)
	tracer.OnTxEnd(&ethtypes.Receipt{GasUsed: surfaceGas - left}, nil)
	logs := repair947Logs(r.surfaceHarness, start)
	require.GreaterOrEqual(t, len(logs), 3)
	encoded, err := tracer.GetResult()
	require.NoError(t, err)
	var decoded goldenFrame
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	var got []goldenLog
	collectLogs(decoded, &got)
	require.Len(t, got, len(logs), "all real producer logs must survive trace transport")
	interleaved := false
	for i := range got {
		if got[i].Position > 0 {
			interleaved = true
		}
		got[i].Position = 0
	}
	require.True(t, interleaved, "the fixture must include parent logs after child calls")
	require.Equal(t, logs, got, "callTracer JSON must preserve the actual EVM emission order")
}

func TestRepairM_947_RegistryBoundedReads(t *testing.T) {
	creatorAcc, creator := testkeeper.MockAddressPair()
	otherAcc, other := testkeeper.MockAddressPair()
	poolA, tokenA := named("repair 947 pool A"), named("repair 947 token A")
	poolB, tokenB := named("repair 947 pool B"), named("repair 947 token B")
	poolC, tokenC := named("repair 947 pool C"), named("repair 947 token C")
	markets := []types.Market{
		viewNative(1, creatorAcc),
		viewMarket(2, types.Generation2, creatorAcc, poolA, tokenA, 100, 0, 1000, 1000),
		viewMarket(3, types.Generation1, otherAcc, poolB, tokenB, 100, 0, 1000, 1000),
		viewMarket(4, types.Generation2, creatorAcc, poolC, tokenC, 100, 0, 1000, 1000),
	}
	e := newViewEnv(t, markets, nil, map[string]common.Address{creatorAcc.String(): creator, otherAcc.String(): other})
	e.evm = vm.NewEVM(e.evm.Context, e.stateDB, e.evm.ChainConfig(), e.evm.Config, map[common.Address]vm.PrecompiledContract{precompile: e.precompile})
	call := func(name string, visits uint64, args ...interface{}) []interface{} {
		t.Helper()
		input, err := e.abis[types.RolePoolRegistry].Pack(name, args...)
		require.NoError(t, err)
		gas := launchpad.Gas(uint64(len(input)), 0) + visits*launchpad.BaseGas
		ret, left, err := e.evm.StaticCall(creator, precompile, input, gas)
		require.NoError(t, err, "%s: %x", name, ret)
		require.Zero(t, left, "each visited market is metered")
		values, err := e.abis[types.RolePoolRegistry].Methods[name].Outputs.Unpack(ret)
		require.NoError(t, err)
		return values
	}
	require.Equal(t, []common.Address{poolA}, call("getAllPools", 2, big.NewInt(0), big.NewInt(1))[0])
	require.Equal(t, []common.Address{poolB}, call("getAllPools", 3, big.NewInt(1), big.NewInt(1))[0])
	require.Equal(t, []common.Address{poolC}, call("getPoolsByCreatorPage", 4, creator, big.NewInt(1), big.NewInt(1))[0])
	require.Equal(t, []common.Address{poolA, poolC}, call("getPoolsByCreator", 4, creator)[0])
	require.Equal(t, big.NewInt(3), call("getPoolCount", 4)[0])
	require.Equal(t, big.NewInt(2), call("getPoolCountByCreator", 4, creator)[0])
	huge := new(big.Int).Lsh(big.NewInt(1), 255)
	require.Empty(t, call("getAllPools", 0, huge, big.NewInt(1))[0])
	require.Empty(t, call("getAllPools", 0, big.NewInt(0), big.NewInt(0))[0])
	require.Empty(t, call("getPoolsByCreatorPage", 0, creator, huge, big.NewInt(1))[0])
	require.Equal(t, poolC, call("getPoolByToken", 0, tokenC)[0])
	require.Equal(t, common.Address{}, call("getPoolByToken", 0, common.Address{})[0])
	require.Equal(t, common.Address{}, call("getPoolByToken", 0, named("repair 947 missing token"))[0])

	// A later corrupt market is a tripwire: a one-item window and a token
	// lookup must not decode it. The full scan must pay before reaching it.
	store := e.stateDB.Ctx().KVStore(sdk.NewKVStoreKey(types.StoreKey))
	store.Set(types.MarketKey(markets[3].Denom), []byte("corrupt later market"))
	require.Equal(t, []common.Address{poolA}, call("getAllPools", 2, big.NewInt(0), big.NewInt(1))[0])
	require.Equal(t, poolC, call("getPoolByToken", 0, tokenC)[0])
	input, err := e.abis[types.RolePoolRegistry].Pack("getPoolCount")
	require.NoError(t, err)
	ret, left, err := e.evm.StaticCall(creator, precompile, input, launchpad.Gas(uint64(len(input)), 0)+3*launchpad.BaseGas)
	require.ErrorIs(t, err, vm.ErrOutOfGas, "stop before reading a market the budget cannot pay for")
	require.Empty(t, ret)
	require.Zero(t, left)
}

func TestRepairM_947_RegistryIndexReopen(t *testing.T) {
	s := newSurfaceHarness(t)
	app := testkeeper.EVMTestApp
	key := sdk.NewKVStoreKey(types.StoreKey)
	// Flush the production cache store, release the old keeper/EVM, and create
	// fresh consumers of the same stored genesis and newly created market.
	ctx := s.ctx()
	ctx.MultiStore().GetKVStore(key).(interface{ Write() }).Write()
	ctx = ctx.WithMultiStore(ctx.MultiStore().CacheMultiStore())
	blockCtx, cfg := s.evm.Context, s.evm.ChainConfig()
	wantPools, wantTokens := s.pools, s.tokens
	s.evm, s.contract, s.keeper = nil, nil, nil
	k := launchpadkeeper.NewKeeper(key, app.BankKeeper, tokenfactorykeeper.NewMsgServerImpl(app.TokenFactoryKeeper), &app.EvmKeeper)
	contract, err := launchpad.NewPrecompileWithKeeper(k, &app.EvmKeeper)
	require.NoError(t, err)
	db := state.NewDBImpl(ctx, &app.EvmKeeper, false)
	evm := vm.NewEVM(blockCtx, db, cfg, vm.Config{}, map[common.Address]vm.PrecompiledContract{precompile: contract})
	registry := parseABIFile(t, "abi/PoolRegistry.json")
	view := func(name string, args ...interface{}) []interface{} {
		t.Helper()
		input, err := registry.Pack(name, args...)
		require.NoError(t, err)
		ret, _, err := evm.StaticCall(s.alice, precompile, input, surfaceGas)
		require.NoError(t, err)
		values, err := registry.Methods[name].Outputs.Unpack(ret)
		require.NoError(t, err)
		return values
	}
	for i, token := range wantTokens {
		pool, found := k.PoolByToken(db.Ctx(), token)
		require.True(t, found)
		require.Equal(t, wantPools[i], pool)
		require.Equal(t, wantPools[i], view("getPoolByToken", token)[0])
	}
	require.Equal(t, wantPools[:], view("getAllPools", big.NewInt(0), big.NewInt(100))[0])
	require.Equal(t, big.NewInt(3), view("getPoolCount")[0])
	require.Equal(t, wantPools[:], view("getPoolsByCreatorPage", s.creator, big.NewInt(0), big.NewInt(100))[0])
}
