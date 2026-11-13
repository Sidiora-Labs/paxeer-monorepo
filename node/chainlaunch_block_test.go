package app_test

import (
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	launchtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

var chainLaunchAddress = common.HexToAddress(launchtypes.LaunchpadAddress)

type chainLaunchBlock struct {
	h        *chainsBlockHarness
	contract abi.ABI
	quote    common.Address
	holder   utils.TestAcct
	height   int64
	nonce    uint64
}

func chainLaunchTokenCall(t *testing.T, a *app.App, ctx sdk.Context, token, from common.Address, signature string, args ...[]byte) []byte {
	t.Helper()
	input := append([]byte(nil), crypto.Keccak256([]byte(signature))[:4]...)
	for _, arg := range args {
		input = append(input, arg...)
	}
	result, err := a.EvmKeeper.CallEVM(ctx, from, &token, nil, input)
	require.NoError(t, err)
	return result
}

func chainLaunchWord(value *big.Int) []byte { return common.LeftPadBytes(value.Bytes(), 32) }

func chainLaunchPrepare(t *testing.T, a *app.App, ctx sdk.Context, sender common.Address) common.Address {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("..", "modules", "launchpad", "keeper", "market.go"))
	require.NoError(t, err)
	creation := regexp.MustCompile(`var sidioraERC20Creation = common\.FromHex\("(0x[0-9a-f]+)"\)`).FindSubmatch(source)
	require.Len(t, creation, 2)
	stringType, err := abi.NewType("string", "", nil)
	require.NoError(t, err)
	uintType, err := abi.NewType("uint256", "", nil)
	require.NoError(t, err)
	addressType, err := abi.NewType("address", "", nil)
	require.NoError(t, err)
	reserve := common.BytesToAddress(crypto.Keccak256([]byte("chain launch quote reserve"))[12:])
	constructor, err := (abi.Arguments{{Type: stringType}, {Type: stringType}, {Type: uintType}, {Type: addressType}}).
		Pack("USD Ledger", "USDL", big.NewInt(1_000_000_000_000_000), reserve)
	require.NoError(t, err)
	deployer := common.BytesToAddress(crypto.Keccak256([]byte("chain launch quote deployer"))[12:])
	nonce := a.EvmKeeper.GetNonce(ctx, deployer)
	_, err = a.EvmKeeper.CallEVM(ctx, deployer, nil, nil, append(common.FromHex(string(creation[1])), constructor...))
	require.NoError(t, err)
	quote := crypto.CreateAddress(deployer, nonce)
	require.NotEmpty(t, a.EvmKeeper.GetCode(ctx, quote))
	params := a.LaunchpadKeeper.GetParams(ctx)
	params.QuoteToken = quote.Hex()
	require.NoError(t, a.LaunchpadKeeper.UpdateParams(ctx, a.LaunchpadKeeper.Authority(), params))
	result := chainLaunchTokenCall(t, a, ctx, quote, reserve, "transfer(address,uint256)",
		common.LeftPadBytes(sender.Bytes(), 32), chainLaunchWord(big.NewInt(100_000_000_000)))
	require.Equal(t, chainLaunchWord(big.NewInt(1)), result)
	result = chainLaunchTokenCall(t, a, ctx, quote, sender, "approve(address,uint256)",
		common.LeftPadBytes(chainLaunchAddress.Bytes(), 32), chainLaunchWord(launchtypes.MaxUint256()))
	require.Equal(t, chainLaunchWord(big.NewInt(1)), result)
	return quote
}

func newChainLaunchBlock(t *testing.T) *chainLaunchBlock {
	t.Helper()
	f := &chainLaunchBlock{height: 2, holder: utils.NewSigner()}
	f.h = newChainsBlockHarnessWithSetup(t, func(t *testing.T, h *chainsBlockHarness, a *app.App, ctx sdk.Context) {
		f.quote = chainLaunchPrepare(t, a, ctx, h.sender.EvmAddress)
		a.EvmKeeper.SetAddressMapping(ctx, f.holder.AccountAddress, f.holder.EvmAddress)
		funds := sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewIntWithDecimal(100, 18)))
		require.NoError(t, a.BankKeeper.MintCoins(ctx, "mint", funds))
		require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", f.holder.AccountAddress, funds))
	})
	data, err := os.ReadFile("../precompiles/launchpad/abi.json")
	require.NoError(t, err)
	f.contract, err = abi.JSON(strings.NewReader(string(data)))
	require.NoError(t, err)
	return f
}

func (f *chainLaunchBlock) buy(t *testing.T, chainID int64, pool common.Address) {
	t.Helper()
	f.send(t, chainID, f.h.sender, 1_000_000, chainLaunchAddress, big.NewInt(0),
		f.pack(t, "buy", pool, big.NewInt(1_000_000_000), big.NewInt(1),
			big.NewInt(f.h.at.Add(24*time.Hour).Unix())))
}

func (f *chainLaunchBlock) transferToken(t *testing.T, chainID int64, token, to common.Address, amount *big.Int) {
	t.Helper()
	data := append(crypto.Keccak256([]byte("transfer(address,uint256)"))[:4], common.LeftPadBytes(to.Bytes(), 32)...)
	data = append(data, chainLaunchWord(amount)...)
	f.send(t, chainID, f.h.sender, 300_000, token, big.NewInt(0), data)
}

func (f *chainLaunchBlock) revenueBalance(ctx sdk.Context, chain uint64) sdk.Int {
	revenue := launchtypes.ChainRevenueAccount(chain)
	whole := f.h.a.BankKeeper.GetBalance(ctx, revenue, "uhpx").Amount
	return whole.Mul(sdk.NewIntFromBigInt(state.UhpxToSweiMultiplier)).Add(f.h.a.BankKeeper.GetWeiBalance(ctx, revenue))
}

func (f *chainLaunchBlock) makeRevenueEpoch(t *testing.T, beforeFunding func(*chainLaunchBlock, launchtypes.LaunchedChain)) (launchtypes.LaunchedChain, launchtypes.ChainRevenueEpoch) {
	t.Helper()
	f.open(t)
	record := f.launch(t, "revenue-chain")
	f.commit(t)
	if beforeFunding != nil {
		beforeFunding(f, record)
	}
	before := f.revenueBalance(f.h.a.GetCheckCtx(), record.RegistryNumber)
	for i := 0; i < 3; i++ {
		f.open(t)
		count := 1
		if i == 0 {
			count = 6
		}
		for j := 0; j < count; j++ {
			f.send(t, int64(record.ChainID), f.h.sender, seqCtxTransferGas,
				f.holder.EvmAddress, big.NewInt(1), nil)
		}
		f.commit(t)
		current := f.revenueBalance(f.h.a.GetCheckCtx(), record.RegistryNumber)
		require.True(t, current.GT(before), "operator share in block %d", f.height-1)
		before = current
	}
	f.open(t)
	f.buy(t, int64(record.ChainID), common.HexToAddress(record.Pool))
	f.commit(t)
	ctx := f.h.a.GetCheckCtx()
	token := common.HexToAddress(record.Token)
	owned := chainLaunchTokenBalance(t, f.h.a, ctx, token, f.h.sender.EvmAddress)
	require.True(t, owned.IsPositive())
	f.open(t)
	f.transferToken(t, int64(record.ChainID), token, f.holder.EvmAddress, new(big.Int).Quo(owned.BigInt(), big.NewInt(2)))
	f.commit(t)
	ctx = f.h.a.GetCheckCtx()
	require.True(t, chainLaunchTokenBalance(t, f.h.a, ctx, token, f.holder.EvmAddress).IsPositive())
	require.True(t, chainLaunchTokenBalance(t, f.h.a, ctx, token, common.HexToAddress(record.Pool)).IsPositive())
	f.h.at = f.h.at.Add(3601 * time.Second)
	f.open(t)
	f.send(t, 125, f.h.sender, 1_000_000, chainLaunchAddress, big.NewInt(0),
		f.pack(t, "openChainRevenueEpoch", uint32(record.RegistryNumber)))
	f.commit(t)
	epoch, found, err := f.h.a.LaunchpadKeeper.GetChainRevenueEpoch(f.h.a.GetCheckCtx(), record.RegistryNumber, 1)
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, epoch.Amount.GTE(sdk.NewInt(2)))
	require.Equal(t, uint64(f.height-2), epoch.SnapshotBlock)
	return record, epoch
}

func chainLaunchTokenBalance(t *testing.T, a *app.App, ctx sdk.Context, token, holder common.Address) sdk.Int {
	t.Helper()
	data := append(crypto.Keccak256([]byte("balanceOf(address)"))[:4], common.LeftPadBytes(holder.Bytes(), 32)...)
	result, err := a.EvmKeeper.StaticCallEVM(ctx, a.LaunchpadKeeper.ModuleAddress(), &token, data)
	require.NoError(t, err)
	require.Len(t, result, 32)
	return sdk.NewIntFromBigInt(new(big.Int).SetBytes(result))
}

func TestLaunchedChainMarketTradesOnlyInItsChain(t *testing.T) {
	f := newChainLaunchBlock(t)
	f.open(t)
	first := f.launch(t, "first-market-chain")
	f.commit(t)
	f.open(t)
	second := f.launch(t, "second-market-chain")
	f.commit(t)
	f.open(t)
	f.send(t, 125, f.h.sender, 3_000_000, chainLaunchAddress, big.NewInt(0),
		f.pack(t, "createMarket", "Root Token", "ROOT", uint8(launchtypes.FeeStrategyClaim), common.Address{}))
	f.commit(t)
	markets := f.h.a.LaunchpadKeeper.ExportGenesis(f.h.a.GetCheckCtx()).Markets
	require.Len(t, markets, 3)
	rootPool := common.HexToAddress(markets[2].PoolAddress)
	require.Zero(t, f.h.a.LaunchpadKeeper.HomeChainByPool(f.h.a.GetCheckCtx(), rootPool))
	f.open(t)
	f.buy(t, int64(first.ChainID), common.HexToAddress(first.Pool))
	f.sendRevert(t, 125, 1_000_000, chainLaunchAddress, big.NewInt(0),
		f.pack(t, "buy", common.HexToAddress(first.Pool), big.NewInt(1_000_000_000), big.NewInt(1),
			big.NewInt(f.h.at.Add(24*time.Hour).Unix())), "WrongChain()")
	f.sendRevert(t, int64(second.ChainID), 1_000_000, chainLaunchAddress, big.NewInt(0),
		f.pack(t, "buy", common.HexToAddress(first.Pool), big.NewInt(1_000_000_000), big.NewInt(1),
			big.NewInt(f.h.at.Add(24*time.Hour).Unix())), "WrongChain()")
	for _, chainID := range []int64{125, int64(first.ChainID), int64(second.ChainID)} {
		f.buy(t, chainID, rootPool)
	}
	f.commit(t)
}

func TestLaunchedChainRevenueReachesHolders(t *testing.T) {
	f := newChainLaunchBlock(t)
	record, epoch := f.makeRevenueEpoch(t, func(f *chainLaunchBlock, record launchtypes.LaunchedChain) {
		ctx := f.h.a.GetCheckCtx()
		before := f.h.a.LaunchpadKeeper.ExportGenesis(ctx)
		balance := f.revenueBalance(ctx, record.RegistryNumber)
		require.True(t, balance.LT(sdk.NewIntFromBigInt(state.UhpxToSweiMultiplier)))
		_, err := f.h.a.LaunchpadKeeper.OpenChainRevenueEpoch(ctx,
			f.h.a.LaunchpadKeeper.ModuleCaller(), record.RegistryNumber)
		require.ErrorIs(t, err, launchtypes.ErrNothingToDistribute)
		require.Equal(t, before, f.h.a.LaunchpadKeeper.ExportGenesis(ctx))
		require.Equal(t, balance, f.revenueBalance(ctx, record.RegistryNumber))
		require.Zero(t, f.h.a.LaunchpadKeeper.LastChainRevenueEpoch(ctx, record.RegistryNumber))
	})
	ctx := f.h.a.GetCheckCtx()
	require.True(t, f.revenueBalance(ctx, record.RegistryNumber).LT(
		sdk.NewIntFromBigInt(new(big.Int).Mul(epoch.Amount.BigInt(), state.UhpxToSweiMultiplier))))
	require.True(t, f.h.a.BankKeeper.GetBalance(ctx, launchtypes.ChainRevenueAccount(record.RegistryNumber), "uhpx").Amount.IsZero())
	f.open(t)
	first := f.send(t, 125, f.h.sender, 1_000_000, chainLaunchAddress, big.NewInt(0),
		f.pack(t, "claimChainRevenue", uint32(record.RegistryNumber), uint64(1)))
	other, hash := f.h.signedTransaction(t, f.holder, 125, 0, 1_000_000, chainLaunchAddress, big.NewInt(0),
		f.pack(t, "claimChainRevenue", uint32(record.RegistryNumber), uint64(1)))
	second := f.h.accept(t, other, hash)
	firstAmount := f.output(t, first, "claimChainRevenue")[0].(*big.Int)
	secondAmount := f.output(t, second, "claimChainRevenue")[0].(*big.Int)
	require.True(t, firstAmount.Sign() > 0)
	require.True(t, secondAmount.Sign() > 0)
	require.Equal(t, new(big.Int).Mul(epoch.Amount.BigInt(), state.UhpxToSweiMultiplier),
		new(big.Int).Add(firstAmount, secondAmount))
	f.commit(t)
	ctx = f.h.a.GetCheckCtx()
	settled, found, err := f.h.a.LaunchpadKeeper.GetChainRevenueEpoch(ctx, record.RegistryNumber, 1)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, epoch.Amount, settled.ClaimedTotal)
	require.True(t, f.h.a.LaunchpadKeeper.HasChainRevenueClaim(ctx, record.RegistryNumber, 1, f.h.sender.EvmAddress))
	require.True(t, f.h.a.LaunchpadKeeper.HasChainRevenueClaim(ctx, record.RegistryNumber, 1, f.holder.EvmAddress))
	_, err = f.h.a.LaunchpadKeeper.ChainRevenueClaimable(ctx, f.h.a.LaunchpadKeeper.ModuleCaller(),
		record.RegistryNumber, 1, common.HexToAddress(record.Pool))
	require.ErrorIs(t, err, launchtypes.ErrUnauthorized)
}

func TestChainLaunchSurvivesRestart(t *testing.T) {
	f := newChainLaunchBlock(t)
	record, epoch := f.makeRevenueEpoch(t, nil)
	f.open(t)
	f.send(t, 125, f.h.sender, 1_000_000, chainLaunchAddress, big.NewInt(0),
		f.pack(t, "claimChainRevenue", uint32(record.RegistryNumber), uint64(1)))
	f.commit(t)
	ctx := f.h.a.GetCheckCtx()
	claim := f.h.a.LaunchpadKeeper.HasChainRevenueClaim(ctx, record.RegistryNumber, 1, f.h.sender.EvmAddress)
	require.True(t, claim)
	claimed, found, err := f.h.a.LaunchpadKeeper.GetChainRevenueEpoch(ctx, record.RegistryNumber, 1)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, epoch.Amount, claimed.Amount)
	launchExport := f.h.a.LaunchpadKeeper.ExportGenesis(ctx)
	chainExport := f.h.a.LayerXChainsKeeper.ExportGenesisWithHistory(ctx)
	require.Len(t, launchExport.LaunchedChains, 1)
	require.Equal(t, record, launchExport.LaunchedChains[0])
	require.Len(t, launchExport.ChainRevenueEpochs, 1)
	require.Equal(t, claimed, launchExport.ChainRevenueEpochs[0])
	commit := f.h.a.LastCommitID()
	f.h.close()
	restarted, _ := f.h.open()
	require.Equal(t, commit, restarted.LastCommitID())
	ctx = restarted.GetCheckCtx()
	restored, found, err := restarted.LaunchpadKeeper.GetLaunchedChain(ctx, record.RegistryNumber)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, record, restored)
	require.Equal(t, record.ChainID, restarted.LaunchpadKeeper.HomeChainByPool(ctx, common.HexToAddress(record.Pool)))
	restoredEpoch, found, err := restarted.LaunchpadKeeper.GetChainRevenueEpoch(ctx, record.RegistryNumber, 1)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, claimed, restoredEpoch)
	require.True(t, restarted.LaunchpadKeeper.HasChainRevenueClaim(ctx, record.RegistryNumber, 1, f.h.sender.EvmAddress))
	fresh := newChainsBlockHarness(t)
	importCtx := fresh.a.GetCheckCtx().WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
	require.NoError(t, fresh.a.LayerXChainsKeeper.InitGenesisWithHistory(importCtx, *chainExport))
	importedDomain, found := fresh.a.LayerXChainsKeeper.GetDomain(importCtx, uint32(record.RegistryNumber))
	require.True(t, found)
	require.Equal(t, chainstypes.StatusActive, importedDomain.Status)
	fresh.a.LaunchpadKeeper.InitGenesis(importCtx, *launchExport)
	imported, found, err := fresh.a.LaunchpadKeeper.GetLaunchedChain(importCtx, record.RegistryNumber)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, record, imported)
	require.Equal(t, record.ChainID, fresh.a.LaunchpadKeeper.HomeChainByPool(importCtx, common.HexToAddress(record.Pool)))
	importedEpoch, found, err := fresh.a.LaunchpadKeeper.GetChainRevenueEpoch(importCtx, record.RegistryNumber, 1)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, claimed, importedEpoch)
	require.True(t, fresh.a.LaunchpadKeeper.HasChainRevenueClaim(importCtx, record.RegistryNumber, 1, f.h.sender.EvmAddress))
}

func TestLaunchedChainControlInABlock(t *testing.T) {
	f := newChainLaunchBlock(t)
	f.open(t)
	record := f.launch(t, "holder-control-chain")
	f.commit(t)
	ctx := f.h.a.GetCheckCtx()
	denom := f.h.a.EvmKeeper.GetBaseDenom(ctx)
	before := f.h.a.BankKeeper.GetBalance(ctx, f.h.sender.AccountAddress, denom).Amount
	f.open(t)
	f.send(t, 125, f.h.sender, 500_000, chainLaunchAddress, big.NewInt(0),
		f.pack(t, "pauseLaunchedChain", uint32(record.RegistryNumber)))
	transfer, hash := f.h.transaction(t, int64(record.ChainID), f.nonce, seqCtxTransferGas,
		f.holder.EvmAddress, big.NewInt(1), nil)
	accepted, err := f.h.exec.Accept(transfer)
	require.ErrorIs(t, err, sequencer.ErrNotAdmitted)
	require.Nil(t, accepted)
	_, pending := f.h.exec.PendingTx([32]byte(hash))
	require.False(t, pending)
	f.send(t, 125, f.h.sender, 500_000, chainLaunchAddress, big.NewInt(0),
		f.pack(t, "resumeLaunchedChain", uint32(record.RegistryNumber)))
	f.send(t, int64(record.ChainID), f.h.sender, seqCtxTransferGas,
		f.holder.EvmAddress, big.NewInt(1), nil)
	f.send(t, 125, f.h.sender, 500_000, chainLaunchAddress, big.NewInt(0),
		f.pack(t, "retireLaunchedChain", uint32(record.RegistryNumber)))
	f.commit(t)
	ctx = f.h.a.GetCheckCtx()
	domain, found := f.h.a.LayerXChainsKeeper.GetDomain(ctx, uint32(record.RegistryNumber))
	require.True(t, found)
	require.Equal(t, chainstypes.StatusRetired, domain.Status)
	require.True(t, domain.Bond.IsZero())
	after := f.h.a.BankKeeper.GetBalance(ctx, f.h.sender.AccountAddress, denom).Amount
	require.True(t, after.GT(before), "retirement returns the bond to the fee right holder")
}

func (f *chainLaunchBlock) open(t *testing.T) {
	t.Helper()
	require.NoError(t, f.h.exec.Open(f.height, f.h.at.Add(time.Duration(f.height)*time.Second).UnixMilli()))
}

func (f *chainLaunchBlock) commit(t *testing.T) {
	t.Helper()
	f.h.commit(t, f.height)
	f.height++
}

func (f *chainLaunchBlock) send(t *testing.T, chainID int64, signer utils.TestAcct, gas uint64, to common.Address, value *big.Int, data []byte) *sequencer.Accepted {
	t.Helper()
	tx, hash := f.h.signedTransaction(t, signer, chainID, f.nonce, gas, to, value, data)
	accepted := f.h.accept(t, tx, hash)
	f.nonce++
	return accepted
}

func (f *chainLaunchBlock) sendRevert(t *testing.T, chainID int64, gas uint64, to common.Address, value *big.Int, data []byte, signature string) {
	t.Helper()
	tx, hash := f.h.transaction(t, chainID, f.nonce, gas, to, value, data)
	accepted, err := f.h.exec.Accept(tx)
	require.NoError(t, err)
	require.NotNil(t, accepted)
	f.nonce++
	require.Equal(t, uint32(45), accepted.Result.Code)
	if f.h.wantCodes == nil {
		f.h.wantCodes = make(map[int]uint32)
	}
	f.h.wantCodes[accepted.TxIndex] = 45
	var messages sdk.TxMsgData
	require.NoError(t, messages.Unmarshal(accepted.Result.Data))
	require.Len(t, messages.Data, 1)
	var execution evmtypes.MsgEVMTransactionResponse
	require.NoError(t, execution.Unmarshal(messages.Data[0].Data))
	require.NotEmpty(t, execution.VmError)
	require.GreaterOrEqual(t, len(execution.ReturnData), 4)
	require.Equal(t, crypto.Keccak256([]byte(signature))[:4], execution.ReturnData[:4])
	if signature == "Error(string)" {
		reason, unpackErr := abi.UnpackRevert(execution.ReturnData)
		require.NoError(t, unpackErr)
		require.Contains(t, reason, "chain name already reserved")
	}
	receipt, err := f.h.a.EvmKeeper.GetTransientReceipt(accepted.ReceiptContext, hash, uint64(accepted.TxIndex))
	require.NoError(t, err)
	require.Equal(t, uint32(ethtypes.ReceiptStatusFailed), receipt.Status)
}

func (f *chainLaunchBlock) output(t *testing.T, accepted *sequencer.Accepted, method string) []interface{} {
	t.Helper()
	var messages sdk.TxMsgData
	require.NoError(t, messages.Unmarshal(accepted.Result.Data))
	require.Len(t, messages.Data, 1)
	var execution evmtypes.MsgEVMTransactionResponse
	require.NoError(t, execution.Unmarshal(messages.Data[0].Data))
	require.Empty(t, execution.VmError)
	values, err := f.contract.Unpack(method, execution.ReturnData)
	require.NoError(t, err)
	return values
}

func (f *chainLaunchBlock) pack(t *testing.T, method string, args ...interface{}) []byte {
	t.Helper()
	input, err := f.contract.Pack(method, args...)
	require.NoError(t, err)
	return input
}

func (f *chainLaunchBlock) launch(t *testing.T, name string) launchtypes.LaunchedChain {
	t.Helper()
	params := chainstypes.DefaultParams()
	value := new(big.Int).Mul(params.MinBond.Add(params.CreationFee).BigInt(), state.UhpxToSweiMultiplier)
	f.send(t, 125, f.h.sender, 5_000_000, chainLaunchAddress, value,
		f.pack(t, "launchChain", "Chain Token", "CHN", uint8(launchtypes.FeeStrategyClaim),
			common.Address{}, name, uint64(2_000_000), uint64(65536), uint16(5000)))
	ctx, err := f.h.exec.Snapshot()
	require.NoError(t, err)
	record, found, err := f.h.a.LaunchpadKeeper.GetLaunchedChain(ctx, uint64(f.h.a.LayerXChainsKeeper.NextID(ctx)-1))
	require.NoError(t, err)
	require.True(t, found)
	return record
}

func (f *chainLaunchBlock) view(t *testing.T, method string, args ...interface{}) []interface{} {
	t.Helper()
	ctx := f.h.a.NewContext(true, f.h.a.GetCheckCtx().BlockHeader()).WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
	f.h.a.RefreshPrecompiles(ctx)
	block, err := f.h.a.EvmKeeper.GetVMBlockContext(ctx, core.GasPool(10_000_000))
	require.NoError(t, err)
	db := state.NewDBImpl(ctx, &f.h.a.EvmKeeper, false)
	evm := vm.NewEVM(*block, db, evmtypes.DefaultChainConfig().EthereumConfig(f.h.a.EvmKeeper.ChainID(ctx)), vm.Config{}, f.h.a.EvmKeeper.CustomPrecompiles(ctx))
	result, _, err := evm.StaticCall(f.h.sender.EvmAddress, chainLaunchAddress, f.pack(t, method, args...), 5_000_000)
	require.NoError(t, err, "%x", result)
	values, err := f.contract.Unpack(method, result)
	require.NoError(t, err)
	return values
}

func TestOneBlockLaunchesChainAndToken(t *testing.T) {
	f := newChainLaunchBlock(t)
	f.open(t)
	record := f.launch(t, "one-block-chain")
	snapshot, err := f.h.exec.Snapshot()
	require.NoError(t, err)
	before := f.h.a.LaunchpadKeeper.GetMarketCount(snapshot)
	params := chainstypes.DefaultParams()
	value := new(big.Int).Mul(params.MinBond.Add(params.CreationFee).BigInt(), state.UhpxToSweiMultiplier)
	f.sendRevert(t, 125, 5_000_000, chainLaunchAddress, value,
		f.pack(t, "launchChain", "Second Token", "SECOND", uint8(launchtypes.FeeStrategyClaim),
			common.Address{}, "one-block-chain", uint64(2_000_000), uint64(65536), uint16(100)), "Error(string)")
	f.commit(t)
	ctx := f.h.a.GetCheckCtx()
	domain, found := f.h.a.LayerXChainsKeeper.GetDomain(ctx, uint32(record.RegistryNumber))
	require.True(t, found)
	require.Equal(t, chainstypes.StatusActive, domain.Status)
	require.Equal(t, common.BytesToAddress(launchtypes.ChainRevenueAccount(record.RegistryNumber)), domain.Operator)
	require.Equal(t, before, f.h.a.LaunchpadKeeper.GetMarketCount(ctx))
	require.Equal(t, uint32(2), f.h.a.LayerXChainsKeeper.NextID(ctx))
	values := f.view(t, "launchedChain", uint32(record.RegistryNumber))
	require.Equal(t, uint32(record.RegistryNumber), values[0])
	require.Equal(t, record.ChainID, values[1])
	require.Equal(t, common.HexToAddress(record.Pool), values[2])
	require.Equal(t, common.HexToAddress(record.Token), values[3])
	require.Equal(t, new(big.Int).SetUint64(record.FeeRightID), values[4])
	require.Equal(t, f.h.sender.EvmAddress, values[5])
	require.Equal(t, record.ChainID, f.h.a.LaunchpadKeeper.HomeChainByPool(ctx, common.HexToAddress(record.Pool)))
}
