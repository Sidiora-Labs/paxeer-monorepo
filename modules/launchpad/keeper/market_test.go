package keeper_test

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"math/big"
	"os"
	"strconv"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

// The generation 2 market recorded on chain: SidioraFactory MarketCreated
// and EventEmitter TokenDeployed of its creation transaction, the creator's
// first market (nonce 0), no optical, created under the default virtual
// reserves and the USDL quote token.
var (
	recordedCreator = common.HexToAddress("0x1dae78b53c9f39930a2ff2b6b7aac12ec75fb154")
	recordedToken   = common.HexToAddress("0x0F92FDD087c96dD3413eF98646b94D0d986192FA")
	recordedPool    = common.HexToAddress("0xCE20Da50F92D21d8BeE497910c782E9Aa25b6263")
	recordedSalt    = common.HexToHash("0x732d9189f684205018e96c622d7409696126edadadd6af26bf8cab84a225297c")
)

const (
	recordedName   = "LiveTest549002"
	recordedSymbol = "LT549002"
	creationFee    = 1_000_000
	virtualToken   = 1_000_000_000_000_000
	surfaceCode    = "0x3415600957600080fd5b363d3d373360601b36523d3d601436013d3d6110175af13d600060003e602e573d6000fd5b3d6000f3"
)

func deploymentSurfaceHashes(t *testing.T) (installed, staged string) {
	t.Helper()
	const directory = "../../../sdk/launchpad/shared/config/deployment/"
	data, err := os.ReadFile(directory + "current.json")
	require.NoError(t, err)
	var manifest struct {
		Roles map[string]struct {
			Kind        string `json:"kind"`
			RuntimeHash string `json:"runtimeHash"`
		} `json:"roles"`
	}
	require.NoError(t, json.Unmarshal(data, &manifest))
	installed = manifest.Roles["PoolRegistry"].RuntimeHash
	require.Regexp(t, "^0x[0-9a-f]{64}$", installed)
	surfaces := 0
	for name, role := range manifest.Roles {
		if role.Kind == "surface" {
			require.Equal(t, installed, role.RuntimeHash, name)
			surfaces++
		}
	}
	require.Equal(t, 9, surfaces)

	// The deployment SDK is an independent Go module, so read its declared
	// staged identity without importing it into the chain module.
	file, err := parser.ParseFile(token.NewFileSet(), directory+"manifest.go", nil, 0)
	require.NoError(t, err)
	ast.Inspect(file, func(node ast.Node) bool {
		decl, ok := node.(*ast.ValueSpec)
		if !ok || len(decl.Names) != 1 || decl.Names[0].Name != "StagedSurfaceRuntimeHash" {
			return true
		}
		require.Len(t, decl.Values, 1)
		literal, ok := decl.Values[0].(*ast.BasicLit)
		require.True(t, ok)
		require.Equal(t, token.STRING, literal.Kind)
		staged, err = strconv.Unquote(literal.Value)
		require.NoError(t, err)
		return false
	})
	require.Regexp(t, "^0x[0-9a-f]{64}$", staged)
	require.NotEqual(t, installed, staged)
	return installed, staged
}

func packArguments(t *testing.T, kinds []string, values ...interface{}) []byte {
	t.Helper()
	args := make(abi.Arguments, len(kinds))
	for i, kind := range kinds {
		typ, err := abi.NewType(kind, "", nil)
		require.NoError(t, err)
		args[i] = abi.Argument{Type: typ}
	}
	out, err := args.Pack(values...)
	require.NoError(t, err)
	return out
}

// tokenSalt is keccak256(abi.encodePacked(creator, name, symbol, nonce)).
func tokenSalt(creator common.Address, name, symbol string, nonce uint64) common.Hash {
	return crypto.Keccak256Hash(creator.Bytes(), []byte(name), []byte(symbol),
		common.LeftPadBytes(new(big.Int).SetUint64(nonce).Bytes(), 32))
}

// tokenAddress is the CREATE2 address of SidioraERC20(name, symbol,
// virtualToken, factory) deployed by factory with salt.
func tokenAddress(t *testing.T, factory common.Address, salt common.Hash, name, symbol string) common.Address {
	t.Helper()
	initCode := append(common.FromHex(sidioraERC20Creation), packArguments(t,
		[]string{"string", "string", "uint256", "address"}, name, symbol, big.NewInt(virtualToken), factory)...)
	return crypto.CreateAddress2(factory, salt, crypto.Keccak256(initCode))
}

func (c *custodyEnv) fund(user common.Address, amount int64) {
	c.t.Helper()
	c.transfer(c.holder, user, amount)
	c.approve(user, amount)
}

func (c *custodyEnv) tokenCall(token common.Address, signature string, words ...[]byte) []byte {
	c.t.Helper()
	input := crypto.Keccak256([]byte(signature))[:4]
	for _, w := range words {
		input = append(input, w...)
	}
	ret, err := testkeeper.EVMTestApp.EvmKeeper.StaticCallEVM(c.ctx, c.k.ModuleAddress(), &token, input)
	require.NoError(c.t, err)
	return ret
}

func (c *custodyEnv) createMarket(creator common.Address, name, symbol string, strategy types.FeeStrategy,
	optical common.Address) (types.Market, error) {
	c.t.Helper()
	ctx, market, err := c.k.CreateMarket(c.ctx, c.caller, nil, creator, name, symbol, strategy, optical)
	if err == nil {
		c.ctx = ctx
	}
	return market, err
}

func requireSameMarket(t *testing.T, want, got types.Market) {
	t.Helper()
	wantJSON, err := json.Marshal(want)
	require.NoError(t, err)
	gotJSON, err := json.Marshal(got)
	require.NoError(t, err)
	require.JSONEq(t, string(wantJSON), string(gotJSON))
}

func TestCreateMarketEmitsMarketCreated(t *testing.T) {
	c := newCustodyEnv(t)
	c.fund(recordedCreator, 4*creationFee)
	c.ctx = c.ctx.WithEventManager(sdk.NewEventManager())
	for _, tc := range []struct {
		strategy types.FeeStrategy
		value    string
	}{
		{types.FeeStrategyClaim, "0"},
		{types.FeeStrategyBurn, "1"},
		{types.FeeStrategyAirdrop, "2"},
		{types.FeeStrategyLpRewards, "3"},
	} {
		t.Run(tc.value, func(t *testing.T) {
			before := len(c.ctx.EventManager().Events())
			market, err := c.createMarket(recordedCreator, recordedName, recordedSymbol, tc.strategy, common.Address{})
			require.NoError(t, err)
			stored, found := c.k.GetMarket(c.ctx, market.Denom)
			require.True(t, found)
			requireSameMarket(t, market, stored)
			var created sdk.Events
			for _, event := range c.ctx.EventManager().Events()[before:] {
				if event.Type == types.EventTypeMarketCreated {
					created = append(created, event)
				}
			}
			require.Len(t, created, 1)
			require.Equal(t, sdk.NewEvent(types.EventTypeMarketCreated,
				sdk.NewAttribute(types.AttributeKeyDenom, market.Denom),
				sdk.NewAttribute(types.AttributeKeyPointer, market.TokenAddress),
				sdk.NewAttribute(types.AttributeKeyCreator, market.Creator),
				sdk.NewAttribute(types.AttributeKeyStrategy, tc.value)), created[0])
		})
	}
}

func TestCreateMarketRefusalEmitsNoMarketCreated(t *testing.T) {
	for _, occupy := range []string{"nonce", "code"} {
		t.Run(occupy, func(t *testing.T) {
			c := newCustodyEnv(t)
			c.fund(recordedCreator, creationFee)
			token := tokenAddress(t, c.k.Generation2().SidioraFactory,
				tokenSalt(recordedCreator, recordedName, recordedSymbol, 0), recordedName, recordedSymbol)
			pool, err := c.k.PredictPoolAddress(c.ctx, token, recordedCreator, common.Address{})
			require.NoError(t, err)
			evm := &testkeeper.EVMTestApp.EvmKeeper
			if occupy == "nonce" {
				evm.SetNonce(c.ctx, pool, 1)
			} else {
				evm.SetCode(c.ctx, pool, []byte{0x00})
			}
			c.ctx = c.ctx.WithEventManager(sdk.NewEventManager())
			ctx, _, err := c.k.CreateMarket(c.ctx, c.caller, nil, recordedCreator, recordedName, recordedSymbol,
				types.FeeStrategyClaim, common.Address{})
			var taken *keeper.AddressTakenError
			require.ErrorAs(t, err, &taken)
			require.Equal(t, pool, taken.Address)
			require.Zero(t, c.k.GetMarketCount(ctx))
			for _, event := range ctx.EventManager().Events() {
				require.NotEqual(t, types.EventTypeMarketCreated, event.Type)
			}
		})
	}
}

func TestCreateMarketRefusesUnapprovedOptical(t *testing.T) {
	c := newCustodyEnv(t)
	c.fund(recordedCreator, 2*creationFee)
	evm := &testkeeper.EVMTestApp.EvmKeeper
	roles := c.k.Generation2()
	optical := address("reviewed optical")
	evm.SetCode(c.ctx, optical, common.FromHex("0x600060005260206000f3"))
	token := tokenAddress(t, roles.SidioraFactory,
		tokenSalt(recordedCreator, recordedName, recordedSymbol, 0), recordedName, recordedSymbol)
	pool, err := c.k.PredictPoolAddress(c.ctx, token, recordedCreator, optical)
	require.NoError(t, err)
	denom := types.ModuleName + "/" + pool.Hex()
	before := c.k.GetMarketCount(c.ctx)
	approvalRuntime := func(response string) []byte {
		code := common.FromHex("0x366000600037366000207f")
		code = append(code, crypto.Keccak256(calldata("isApproved(address)", addressWord(optical)))...)
		return append(code, common.FromHex("0x1460345760006000fd5b"+response)...)
	}
	for _, tc := range []struct {
		name, response string
	}{
		{"unapproved", "600060005260206000f3"},
		{"registry reverts", "60006000fd"},
		{"empty response", "00"},
		{"short true", "60016000526001601ff3"},
		{"oversized true", "600160005260406000f3"},
		{"noncanonical bool", "600260005260206000f3"},
		{"nonzero high byte", "60016000536001601f5360206000f3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			evm.SetCode(c.ctx, roles.OpticalRegistry, approvalRuntime(tc.response))
			ctx, _ := c.ctx.CacheContext()
			ctx, market, err := c.k.CreateMarket(ctx, c.caller, nil, recordedCreator, recordedName,
				recordedSymbol, types.FeeStrategyClaim, optical)
			require.ErrorIs(t, err, types.ErrOpticalNotApproved)
			require.Equal(t, types.Market{}, market)
			require.Equal(t, before, c.k.GetMarketCount(ctx))
			require.Zero(t, c.k.FactoryNonce(ctx, recordedCreator))
			require.Empty(t, evm.GetCode(ctx, token))
			require.Zero(t, evm.GetNonce(ctx, token))
			require.Empty(t, evm.GetCode(ctx, pool))
			require.Zero(t, evm.GetNonce(ctx, pool))
			_, found, err := c.k.GetSurface(ctx, pool)
			require.NoError(t, err)
			require.False(t, found)
			_, found, err = c.k.GetFeeRight(ctx, roles.SidioraNFT, 1)
			require.NoError(t, err)
			require.False(t, found)
			require.Zero(t, c.k.FeeRightPoolTokenID(ctx, roles.SidioraNFT, pool))
			require.False(t, c.k.HasRole(ctx, roles.FeeAccumulator, keeper.PoolRole, pool))
			_, found, err = c.k.GetPoolMetadata(ctx, pool)
			require.NoError(t, err)
			require.False(t, found)
			_, found = c.k.PoolByToken(ctx, token)
			require.False(t, found)
			_, found = c.k.GetMarket(ctx, denom)
			require.False(t, found)
			_, found = c.k.GetMarketByIndex(ctx, before+1)
			require.False(t, found)
		})
	}
	evm.SetCode(c.ctx, roles.OpticalRegistry, approvalRuntime("600160005260206000f3"))
	market, err := c.createMarket(recordedCreator, recordedName, recordedSymbol, types.FeeStrategyClaim, optical)
	require.NoError(t, err)
	require.Equal(t, before+1, c.k.GetMarketCount(c.ctx))
	require.Equal(t, optical.Hex(), market.Optical)
	require.Equal(t, token.Hex(), market.TokenAddress)
	require.Equal(t, pool.Hex(), market.PoolAddress)
	stored, found := c.k.GetMarket(c.ctx, denom)
	require.True(t, found)
	requireSameMarket(t, market, stored)
	metadata, found, err := c.k.GetPoolMetadata(c.ctx, pool)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, optical, metadata.Optical)
	contractError, found := types.ContractErrorOf(types.ErrOpticalNotApproved)
	require.True(t, found)
	require.Equal(t, "OpticalNotApproved()", contractError.Signature())
	require.Equal(t, uint32(76), types.ErrOpticalNotApproved.ABCICode())

	evm.SetCode(c.ctx, roles.OpticalRegistry, common.FromHex("0x60006000fd"))
	market, err = c.createMarket(recordedCreator, recordedName, recordedSymbol, types.FeeStrategyClaim,
		common.Address{})
	require.NoError(t, err)
	require.Equal(t, before+2, c.k.GetMarketCount(c.ctx))
	require.Equal(t, common.Address{}.Hex(), market.Optical)
}

func TestCreateMarket(t *testing.T) {
	t.Run("derives the recorded generation 2 token salt and pool address", func(t *testing.T) {
		e := newEnv(t)
		params := e.k.GetParams(e.ctx)
		require.Equal(t, e.usdl.Hex(), params.QuoteToken)
		require.Equal(t, "10000000000", params.VirtualQuoteDefault.String())
		require.Equal(t, "1000000000000000", params.VirtualTokenDefault.String())
		require.Equal(t, recordedSalt, tokenSalt(recordedCreator, recordedName, recordedSymbol, 0))
		require.Equal(t, recordedToken, tokenAddress(t, e.k.Generation2().SidioraFactory, recordedSalt, recordedName,
			recordedSymbol))
		params.QuoteToken = types.DefaultQuoteToken
		require.NoError(t, e.k.UpdateParams(e.ctx, e.k.Authority(), params))
		pool, err := e.k.PredictPoolAddress(e.ctx, recordedToken, recordedCreator, common.Address{})
		require.NoError(t, err)
		require.Equal(t, recordedPool, pool)
	})

	t.Run("deploys the token, the pool surface, the fee right and the registry entry", func(t *testing.T) {
		c := newCustodyEnv(t)
		c.ctx = c.ctx.WithBlockHeight(41)
		evm := &testkeeper.EVMTestApp.EvmKeeper
		roles := c.k.Generation2()
		c.fund(recordedCreator, 3*creationFee)
		require.Zero(t, c.k.FactoryNonce(c.ctx, recordedCreator))

		market, err := c.createMarket(recordedCreator, recordedName, recordedSymbol, types.FeeStrategyBurn,
			common.Address{})
		require.NoError(t, err)

		pool, err := c.k.PredictPoolAddress(c.ctx, recordedToken, recordedCreator, common.Address{})
		require.NoError(t, err)
		account := evm.GetPaxAddressOrDefault(c.ctx, recordedCreator).String()
		zeros := make([]sdk.Int, types.SnapshotSlots)
		for i := range zeros {
			zeros[i] = sdk.ZeroInt()
		}
		want := types.Market{
			Denom: types.ModuleName + "/" + pool.Hex(), Index: 1, Name: recordedName, Symbol: recordedSymbol,
			Creator: account, Guardian: account, FeeRightsHolder: account, FeeStrategy: types.FeeStrategyBurn,
			TotalSupply: sdk.NewInt(virtualToken), VirtualQuoteReserve: sdk.NewInt(10_000_000_000),
			RealQuoteBalance: sdk.ZeroInt(), TokenReserve: sdk.NewInt(virtualToken),
			CreationTime: c.ctx.BlockTime().Unix(), CumulativeVolume: sdk.ZeroInt(), PriceSnapshots: zeros,
			SnapshotHeights: make([]uint64, types.SnapshotSlots), AccumulatedQuoteFees: sdk.ZeroInt(),
			AccumulatedTokenFees: sdk.ZeroInt(), AccumulatedFees: sdk.ZeroInt(), AirdropBalance: sdk.ZeroInt(),
			Generation: types.Generation2, PoolAddress: pool.Hex(), TokenAddress: recordedToken.Hex(),
			Optical: common.Address{}.Hex(), Factory: roles.SidioraFactory.Hex(),
			FeeAccumulator: roles.FeeAccumulator.Hex(), ProtocolConfig: roles.ProtocolConfig.Hex(),
			EventEmitter: roles.EventEmitter.Hex(), CumulativeBuyQuoteVolume: sdk.ZeroInt(),
			CumulativeSellQuoteVolume: sdk.ZeroInt(), FeeRightCollection: roles.SidioraNFT.Hex(), FeeRightID: 1,
		}
		requireSameMarket(t, want, market)
		stored, found := c.k.GetMarket(c.ctx, want.Denom)
		require.True(t, found)
		requireSameMarket(t, want, stored)
		byIndex, found := c.k.GetMarketByIndex(c.ctx, 1)
		require.True(t, found)
		require.Equal(t, want.Denom, byIndex.Denom)
		require.Equal(t, uint64(1), c.k.GetMarketCount(c.ctx))
		require.Equal(t, uint64(1), c.k.FactoryNonce(c.ctx, recordedCreator))

		// The creation fee went from the creator to custody, into TreasuryHeld
		// and the Treasury ledger.
		require.Equal(t, int64(2*creationFee), c.balanceOf(recordedCreator))
		require.Equal(t, int64(2*creationFee), c.allowance(recordedCreator, common.HexToAddress(types.LaunchpadAddress)))
		require.Equal(t, int64(creationFee), c.balanceOf(c.custody))
		require.Equal(t, int64(creationFee), c.ledger(keeper.TreasuryHeldLedger()))
		treasury, err := c.k.TreasuryGetBalance(c.ctx, c.usdl)
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(creationFee).String(), treasury.String())

		// The pool address holds the surface runtime with nonce 1.
		code := evm.GetCode(c.ctx, pool)
		require.Len(t, code, 51)
		require.Equal(t, common.FromHex(surfaceCode), code)
		require.Equal(t, keeper.SurfaceRuntime, code)
		_, stagedHash := deploymentSurfaceHashes(t)
		require.Equal(t, stagedHash, crypto.Keccak256Hash(code).Hex())
		require.Equal(t, uint64(1), evm.GetNonce(c.ctx, pool))
		surface, found, err := c.k.GetSurface(c.ctx, pool)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, keeper.Surface{Role: types.RolePool, Generation: types.Generation2, Market: want.Denom}, surface)

		// authorizePool, the fee right and the registry entry.
		require.Equal(t, crypto.Keccak256Hash([]byte("POOL_ROLE")), keeper.PoolRole)
		require.True(t, c.k.HasRole(c.ctx, roles.FeeAccumulator, keeper.PoolRole, pool))
		require.Equal(t, uint64(1), c.k.FeeRightPoolTokenID(c.ctx, roles.SidioraNFT, pool))
		owner, err := c.k.FeeRightOwnerOf(c.ctx, roles.SidioraNFT, 1)
		require.NoError(t, err)
		require.Equal(t, recordedCreator, owner)
		right, found, err := c.k.GetFeeRight(c.ctx, roles.SidioraNFT, 1)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, types.FeeStrategyBurn, right.Strategy)
		require.Equal(t, pool.Hex(), right.Pool)
		metadata, found, err := c.k.GetPoolMetadata(c.ctx, pool)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, keeper.PoolMetadata{Creator: recordedCreator, Token: recordedToken, NFTID: 1,
			CreatedAt: c.ctx.BlockTime().Unix(), CreatedBlock: 41}, metadata)
		registered, found := c.k.PoolByToken(c.ctx, recordedToken)
		require.True(t, found)
		require.Equal(t, pool, registered)

		// The token is a real SidioraERC20 bound to the pool, its whole supply
		// moved from the factory to the pool.
		require.NotEmpty(t, evm.GetCode(c.ctx, recordedToken))
		require.Equal(t, packArguments(t, []string{"string"}, recordedName), c.tokenCall(recordedToken, "name()"))
		require.Equal(t, packArguments(t, []string{"string"}, recordedSymbol), c.tokenCall(recordedToken, "symbol()"))
		require.Equal(t, word(big.NewInt(6)), c.tokenCall(recordedToken, "decimals()"))
		require.Equal(t, word(big.NewInt(virtualToken)), c.tokenCall(recordedToken, "totalSupply()"))
		require.Equal(t, common.LeftPadBytes(roles.SidioraFactory.Bytes(), 32),
			c.tokenCall(recordedToken, "deploymentFactory()"))
		require.Equal(t, common.LeftPadBytes(pool.Bytes(), 32), c.tokenCall(recordedToken, "pool()"))
		require.Equal(t, word(big.NewInt(virtualToken)),
			c.tokenCall(recordedToken, "balanceOf(address)", common.LeftPadBytes(pool.Bytes(), 32)))
		require.Equal(t, word(new(big.Int)),
			c.tokenCall(recordedToken, "balanceOf(address)", common.LeftPadBytes(roles.SidioraFactory.Bytes(), 32)))

		// The creator's second market takes nonce 1, the next fee-right id and
		// the next index.
		second, err := c.createMarket(recordedCreator, recordedName, recordedSymbol, types.FeeStrategyClaim,
			common.Address{})
		require.NoError(t, err)
		secondToken := tokenAddress(t, roles.SidioraFactory, tokenSalt(recordedCreator, recordedName, recordedSymbol, 1),
			recordedName, recordedSymbol)
		secondPool, err := c.k.PredictPoolAddress(c.ctx, secondToken, recordedCreator, common.Address{})
		require.NoError(t, err)
		require.Equal(t, secondToken.Hex(), second.TokenAddress)
		require.Equal(t, secondPool.Hex(), second.PoolAddress)
		require.Equal(t, uint64(2), second.Index)
		require.Equal(t, uint64(2), second.FeeRightID)
		require.Equal(t, uint64(2), c.k.FactoryNonce(c.ctx, recordedCreator))
		require.Equal(t, int64(creationFee), c.balanceOf(recordedCreator))
		require.Equal(t, int64(2*creationFee), c.ledger(keeper.TreasuryHeldLedger()))
	})

	t.Run("refuses as the Router and the Factory do", func(t *testing.T) {
		c := newCustodyEnv(t)
		creator := address("creator C")
		c.fund(creator, creationFee)
		cases := []struct {
			name           string
			creator        common.Address
			market, symbol string
			strategy       types.FeeStrategy
			optical        common.Address
			want           error
		}{
			{"zero creator", common.Address{}, "Kindle Token", "KNDL", types.FeeStrategyClaim, common.Address{}, types.ErrZeroAddress},
			{"blank name", creator, " ", "KNDL", types.FeeStrategyClaim, common.Address{}, types.ErrInvalidMarket},
			{"long symbol", creator, "Kindle Token", "SEVENTEEN-LETTERS", types.FeeStrategyClaim, common.Address{}, types.ErrInvalidMarket},
			{"strategy above 3", creator, "Kindle Token", "KNDL", types.FeeStrategy(4), common.Address{}, types.ErrZeroAddress},
			{"optical without code", creator, "Kindle Token", "KNDL", types.FeeStrategyClaim, address("optical O"), types.ErrZeroAddress},
			{"creation fee without allowance", address("creator D"), "Kindle Token", "KNDL", types.FeeStrategyClaim, common.Address{}, types.ErrTransferFromFailed},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				ctx, _ := c.ctx.CacheContext()
				_, _, err := c.k.CreateMarket(ctx, c.caller, nil, tc.creator, tc.market, tc.symbol, tc.strategy, tc.optical)
				require.ErrorIs(t, err, tc.want)
				require.Zero(t, c.k.GetMarketCount(ctx))
			})
		}
	})

	t.Run("refuses a pool address that holds code or a nonce with empty revert data", func(t *testing.T) {
		evm := &testkeeper.EVMTestApp.EvmKeeper
		for _, occupy := range []string{"nonce", "code"} {
			t.Run(occupy, func(t *testing.T) {
				c := newCustodyEnv(t)
				creator := address("creator E")
				c.fund(creator, creationFee)
				roles := c.k.Generation2()
				token := tokenAddress(t, roles.SidioraFactory, tokenSalt(creator, "Kindle Token", "KNDL", 0), "Kindle Token",
					"KNDL")
				pool, err := c.k.PredictPoolAddress(c.ctx, token, creator, common.Address{})
				require.NoError(t, err)
				if occupy == "nonce" {
					evm.SetNonce(c.ctx, pool, 1)
				} else {
					evm.SetCode(c.ctx, pool, []byte{0x00})
				}
				_, err = c.createMarket(creator, "Kindle Token", "KNDL", types.FeeStrategyClaim, common.Address{})
				var taken *keeper.AddressTakenError
				require.ErrorAs(t, err, &taken)
				require.Equal(t, pool, taken.Address)
				require.ErrorIs(t, err, types.ErrInvalidMarket)
				revert, err := taken.Revert()
				require.NoError(t, err)
				require.Empty(t, revert)
			})
		}
	})

	t.Run("fee reads the price snapshots oldest first", func(t *testing.T) {
		e := newEnv(t)
		params := e.k.GetParams(e.ctx)
		chronological := []int64{1000, 1100, 1200, 1300, 1400, 1500, 1600, 1700}
		for _, ring := range []struct {
			name         string
			index, count uint64
			start        uint64
		}{
			{"full ring", 3, 8, 3},
			{"partial ring", 5, 5, 0},
		} {
			t.Run(ring.name, func(t *testing.T) {
				market := types.Market{CreationTime: e.ctx.BlockTime().Unix() - 3600, SnapshotIndex: ring.index,
					SnapshotCount: ring.count, PriceSnapshots: make([]sdk.Int, types.SnapshotSlots)}
				var ordered [types.SnapshotSlots]*big.Int
				for i := range ordered {
					ordered[i] = new(big.Int)
					market.PriceSnapshots[i] = sdk.ZeroInt()
				}
				for i := uint64(0); i < ring.count; i++ {
					slot := (ring.start + i) % types.SnapshotSlots
					market.PriceSnapshots[slot] = sdk.NewInt(chronological[i])
					ordered[i] = big.NewInt(chronological[i])
				}
				age := big.NewInt(3600)
				volatility, err := types.CalculateVolatility(ordered, ring.count)
				require.NoError(t, err)
				want, err := types.CalculateDynamicFee(params.FeeInputs(age, volatility))
				require.NoError(t, err)
				got, err := e.k.FeeBps(e.ctx, market)
				require.NoError(t, err)
				require.Equal(t, want.String(), got.String())
				if ring.start != 0 {
					rawVolatility, err := types.CalculateVolatility(market.Snapshots(), ring.count)
					require.NoError(t, err)
					raw, err := types.CalculateDynamicFee(params.FeeInputs(age, rawVolatility))
					require.NoError(t, err)
					require.NotEqual(t, raw.String(), got.String())
				}
			})
		}
	})
}
