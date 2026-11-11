package launchpad

import (
	"fmt"
	"math"
	"math/big"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	launchpadkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const chainLaunchRole = "ChainLaunch"

var chainLaunchABI = pcommon.MustGetABI(roleFS, "abi/ChainLaunch.json")

func init() { roleHandlers[chainLaunchRole] = serveChainLaunch }

func chainWei(amount sdk.Int) *big.Int {
	return new(big.Int).Mul(amount.BigInt(), state.UhpxToSweiMultiplier)
}

func chainNumber(number uint64) (uint32, error) {
	if number == 0 || number > math.MaxUint32 {
		return 0, types.ErrNotLaunchedChain
	}
	return uint32(number), nil
}

func chainRecord(c *call, record types.LaunchedChain) ([]byte, error) {
	number, err := chainNumber(record.RegistryNumber)
	if err != nil {
		return nil, err
	}
	if record.LaunchBlock < 0 {
		return nil, types.ErrInvalidGenesis
	}
	return c.method.Outputs.Pack(number, record.ChainID, common.HexToAddress(record.Pool),
		common.HexToAddress(record.Token), new(big.Int).SetUint64(record.FeeRightID),
		common.HexToAddress(record.Creator), uint64(record.LaunchBlock))
}

func serveChainLaunch(p PrecompileExecutor, c *call) ([]byte, error) {
	if err := pcommon.ValidateArgsLength(c.args, len(c.method.Inputs)); err != nil {
		return nil, err
	}
	switch c.method.RawName {
	case "launchChain", "launchChainWithPermit":
		return p.launchChain(c)
	case "launchedChain":
		record, found, err := p.keeper.GetLaunchedChain(c.ctx, uint64(c.args[0].(uint32)))
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, types.ErrNotLaunchedChain
		}
		return chainRecord(c, record)
	case "launchedChainOfPool":
		number, _ := p.keeper.RegistryNumberByPool(c.ctx, c.args[0].(common.Address))
		if number > math.MaxUint32 {
			return nil, types.ErrInvalidGenesis
		}
		return c.method.Outputs.Pack(uint32(number))
	case "homeChainOf":
		return c.method.Outputs.Pack(p.keeper.HomeChainByPool(c.ctx, c.args[0].(common.Address)))
	case "chainRevenueEpoch":
		record, found, err := p.keeper.GetChainRevenueEpoch(c.ctx, uint64(c.args[0].(uint32)), c.args[1].(uint64))
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, types.ErrNotLaunchedChain
		}
		number, err := chainNumber(record.Chain)
		if err != nil {
			return nil, err
		}
		if record.OpenedAt < 0 {
			return nil, types.ErrInvalidGenesis
		}
		return c.method.Outputs.Pack(number, record.Epoch, chainWei(record.Amount), record.SnapshotBlock,
			record.EligibleSupply.BigInt(), chainWei(record.ClaimedTotal), uint64(record.OpenedAt))
	case "chainRevenueClaimable":
		nested, err := runNested(c)
		if err != nil {
			return nil, err
		}
		amount, err := p.keeper.ChainRevenueClaimable(c.ctx, nested, uint64(c.args[0].(uint32)), c.args[1].(uint64), c.args[2].(common.Address))
		if err != nil {
			return nil, err
		}
		return c.method.Outputs.Pack(chainWei(amount))
	case "openChainRevenueEpoch":
		nested, err := runNested(c)
		if err != nil {
			return nil, err
		}
		record, err := p.keeper.OpenChainRevenueEpoch(c.ctx, nested, uint64(c.args[0].(uint32)))
		if err != nil {
			return nil, err
		}
		if err := p.emitFamily(c.evm, "ChainRevenueEpochOpened", uint32(record.Chain), record.Epoch,
			chainWei(record.Amount), record.SnapshotBlock, record.EligibleSupply.BigInt()); err != nil {
			return nil, err
		}
		return c.method.Outputs.Pack(record.Epoch, chainWei(record.Amount))
	case "claimChainRevenue":
		nested, err := runNested(c)
		if err != nil {
			return nil, err
		}
		chain, epoch := c.args[0].(uint32), c.args[1].(uint64)
		ctx, amount, err := p.keeper.ClaimChainRevenue(c.ctx, nested, uint64(chain), epoch, c.caller)
		if err != nil {
			return nil, err
		}
		c.ctx = ctx
		if err := p.emitFamily(c.evm, "ChainRevenueClaimed", chain, epoch, c.caller, chainWei(amount)); err != nil {
			return nil, err
		}
		return c.method.Outputs.Pack(chainWei(amount))
	case "claimChainRevenueBatch":
		nested, err := runNested(c)
		if err != nil {
			return nil, err
		}
		chain, epochs := c.args[0].(uint32), c.args[1].([]uint64)
		ctx, amounts, err := p.keeper.ClaimChainRevenueBatch(c.ctx, nested, uint64(chain), c.caller, epochs)
		if err != nil {
			return nil, err
		}
		c.ctx = ctx
		total := new(big.Int)
		for i, amount := range amounts {
			wei := chainWei(amount)
			total.Add(total, wei)
			if err := p.emitFamily(c.evm, "ChainRevenueClaimed", chain, epochs[i], c.caller, wei); err != nil {
				return nil, err
			}
		}
		return c.method.Outputs.Pack(total)
	case "pauseLaunchedChain":
		if err := p.keeper.PauseLaunchedChain(c.ctx, uint64(c.args[0].(uint32)), c.caller); err != nil {
			return nil, err
		}
		return c.method.Outputs.Pack()
	case "resumeLaunchedChain":
		if err := p.keeper.ResumeLaunchedChain(c.ctx, uint64(c.args[0].(uint32)), c.caller); err != nil {
			return nil, err
		}
		return c.method.Outputs.Pack()
	case "retireLaunchedChain":
		chain := c.args[0].(uint32)
		before := len(c.ctx.EventManager().Events())
		if err := p.keeper.RetireLaunchedChain(c.ctx, uint64(chain), c.caller); err != nil {
			return nil, err
		}
		var bond *big.Int
		for _, event := range c.ctx.EventManager().Events()[before:] {
			if event.Type != types.EventTypeLaunchedChainRetired {
				continue
			}
			for _, attr := range event.Attributes {
				if string(attr.Key) == "returned_bond" {
					amount, ok := new(big.Int).SetString(string(attr.Value), 10)
					if !ok {
						return nil, types.ErrInvalidGenesis
					}
					bond = new(big.Int).Mul(amount, state.UhpxToSweiMultiplier)
				}
			}
		}
		if bond == nil {
			return nil, fmt.Errorf("launchpad: retirement bond event is absent")
		}
		if err := p.emitFamily(c.evm, "LaunchedChainRetired", chain, c.caller, bond); err != nil {
			return nil, err
		}
		return c.method.Outputs.Pack()
	default:
		return nil, fmt.Errorf("launchpad: unknown chain launch method %s", c.method.RawName)
	}
}

func (p PrecompileExecutor) launchChain(c *call) ([]byte, error) {
	release, err := p.keeper.EnterLock(c.evm.StateDB, launchpadkeeper.RouterLock)
	if err != nil {
		return nil, err
	}
	defer release()
	nested, err := runNested(c)
	if err != nil {
		return nil, err
	}
	if c.method.RawName == "launchChainWithPermit" {
		if err := p.applyPermit(c, nested, p.usdl(c.ctx), c.args[8]); err != nil {
			return nil, err
		}
	}
	if c.value == nil || c.value.Sign() < 0 {
		return nil, types.ErrInvalidParams
	}
	bankSource, ok := p.evmKeeper.(interface{ BankKeeper() bankkeeper.Keeper })
	if !ok {
		return nil, fmt.Errorf("launchpad: payment keeper is unavailable")
	}
	coin, err := pcommon.HandlePaymentUhpx(c.ctx, p.account(c.ctx, p.address), p.account(c.ctx, c.caller),
		c.value, bankSource.BankKeeper(), p.evmKeeper, c.hooks, c.evm.GetDepth())
	if err != nil {
		return nil, err
	}
	nonce := p.keeper.FactoryNonce(c.ctx, c.caller)
	ctx, market, record, err := p.keeper.LaunchChain(c.ctx, nested, c.evm, c.caller,
		c.args[0].(string), c.args[1].(string), types.FeeStrategy(c.args[2].(uint8)), c.args[3].(common.Address),
		c.args[4].(string), c.args[5].(uint64), c.args[6].(uint64), c.args[7].(uint16), coin.Amount)
	if err != nil {
		return nil, err
	}
	c.ctx = ctx
	if err := p.emitLaunchedMarket(c, market, nonce, c.args[0].(string), c.args[1].(string), c.args[2].(uint8)); err != nil {
		return nil, err
	}
	number, err := chainNumber(record.RegistryNumber)
	if err != nil {
		return nil, err
	}
	token, pool := common.HexToAddress(record.Token), common.HexToAddress(record.Pool)
	nftID := new(big.Int).SetUint64(record.FeeRightID)
	if err := p.emitFamily(c.evm, "ChainLaunched", number, pool, token, record.ChainID, nftID, c.caller); err != nil {
		return nil, err
	}
	return c.method.Outputs.Pack(token, pool, nftID, number, record.ChainID)
}

func (p PrecompileExecutor) emitLaunchedMarket(c *call, market types.Market, nonce uint64, name, symbol string, strategy uint8) error {
	roles := p.keeper.Generation2()
	token, pool := common.HexToAddress(market.TokenAddress), common.HexToAddress(market.PoolAddress)
	id := new(big.Int).SetUint64(market.FeeRightID)
	fee := p.keeper.GetParams(c.ctx).CreationFee
	if fee.IsPositive() {
		if err := emitABIEvent(c.evm, roles.Treasury, roleABIs[types.RoleTreasury].Events["Deposited"],
			[]interface{}{p.usdl(c.ctx), c.target, fee.BigInt()}); err != nil {
			return err
		}
	}
	if err := emitABIEvent(c.evm, pool, roleABIs[types.RolePool].Events["Initialized"], []interface{}{uint64(1)}); err != nil {
		return err
	}
	collection := common.HexToAddress(market.FeeRightCollection)
	nft := roleABIs[types.RoleFeeRights]
	if err := emitABIEvent(c.evm, collection, nft.Events["Transfer"], []interface{}{common.Address{}, c.caller, id}); err != nil {
		return err
	}
	if err := emitABIEvent(c.evm, collection, nft.Events["PoolNFTMinted"], []interface{}{id, c.caller, pool}); err != nil {
		return err
	}
	if p.familyEnabled() {
		if err := p.emitFamily(c.evm, "NftMint", id, c.caller, pool, strategy); err != nil {
			return err
		}
	}
	if err := emitABIEvent(c.evm, roles.PoolRegistry, roleABIs[types.RolePoolRegistry].Events["PoolRegistered"],
		[]interface{}{pool, token, c.caller, common.HexToAddress(market.Optical), id, big.NewInt(c.ctx.BlockTime().Unix())}); err != nil {
		return err
	}
	if p.familyEnabled() {
		salt := crypto.Keccak256Hash(c.caller.Bytes(), []byte(name), []byte(symbol),
			common.LeftPadBytes(new(big.Int).SetUint64(nonce).Bytes(), 32))
		if err := p.emitFamily(c.evm, "PoolRegistered", pool, token, c.caller, common.HexToAddress(market.Optical), id); err != nil {
			return err
		}
		if err := p.emitFamily(c.evm, "TokenDeployed", token, pool, c.caller, salt, name, symbol, uint8(6), market.TotalSupply.BigInt()); err != nil {
			return err
		}
		if err := p.emitFamily(c.evm, "MarketCreated", poolID(pool), token, c.caller, pool, common.HexToAddress(market.Optical)); err != nil {
			return err
		}
	}
	if err := emitABIEvent(c.evm, roles.SidioraFactory, roleABIs[types.RoleFactory].Events["MarketCreated"],
		[]interface{}{token, pool, c.caller, id, common.HexToAddress(market.Optical)}); err != nil {
		return err
	}
	if err := emitABIEvent(c.evm, c.target, routerABI.Events["MarketCreated"], []interface{}{token, pool, c.caller, id}); err != nil {
		return err
	}
	return nil
}
