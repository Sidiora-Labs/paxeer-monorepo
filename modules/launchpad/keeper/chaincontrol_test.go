package keeper_test

import (
	"strconv"
	"testing"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func launchedChainControlEnv(t *testing.T) (*custodyEnv, types.Market, types.LaunchedChain) {
	t.Helper()
	c, value := launchChainEnv(t)
	market, record, err := launchChain(t, c, "controlled-market-home", 100, value)
	require.NoError(t, err)
	return c, market, record
}

func launchedChainControlBalance(c *custodyEnv, account sdk.AccAddress) sdk.Int {
	app := testkeeper.EVMTestApp
	return app.BankKeeper.GetBalance(c.ctx, account, app.EvmKeeper.GetBaseDenom(c.ctx)).Amount
}

func TestLaunchedChainControlHolderPausesAndResumes(t *testing.T) {
	c, _, record := launchedChainControlEnv(t)
	registry := testkeeper.EVMTestApp.LayerXChainsKeeper
	id := uint32(record.RegistryNumber)
	require.NoError(t, c.k.PauseLaunchedChain(c.ctx, record.RegistryNumber, recordedCreator))
	domain, found := registry.GetDomain(c.ctx, id)
	require.True(t, found)
	require.Equal(t, chainstypes.StatusPaused, domain.Status)
	require.NoError(t, c.k.ResumeLaunchedChain(c.ctx, record.RegistryNumber, recordedCreator))
	domain, found = registry.GetDomain(c.ctx, id)
	require.True(t, found)
	require.Equal(t, chainstypes.StatusActive, domain.Status)
	require.NoError(t, registry.GovernancePause(c.ctx, registry.GetParams(c.ctx).Authority, id, true))
	require.ErrorIs(t, c.k.ResumeLaunchedChain(c.ctx, record.RegistryNumber, recordedCreator), chainstypes.ErrNotAuthority)
	domain, found = registry.GetDomain(c.ctx, id)
	require.True(t, found)
	require.Equal(t, chainstypes.StatusGovernancePaused, domain.Status)
}

func TestLaunchedChainControlOtherCallerIsRefused(t *testing.T) {
	c, _, record := launchedChainControlEnv(t)
	other := common.BytesToAddress([]byte{0x42})
	registry := testkeeper.EVMTestApp.LayerXChainsKeeper
	before, found := registry.GetDomain(c.ctx, uint32(record.RegistryNumber))
	require.True(t, found)
	require.ErrorIs(t, c.k.PauseLaunchedChain(c.ctx, record.RegistryNumber, other), types.ErrNotFeeRightHolder)
	require.ErrorIs(t, c.k.ResumeLaunchedChain(c.ctx, record.RegistryNumber, other), types.ErrNotFeeRightHolder)
	require.ErrorIs(t, c.k.RetireLaunchedChain(c.ctx, record.RegistryNumber, other), types.ErrNotFeeRightHolder)
	after, found := registry.GetDomain(c.ctx, uint32(record.RegistryNumber))
	require.True(t, found)
	require.Equal(t, before, after)
}

func TestLaunchedChainControlFollowsTransferOfTheFeeRight(t *testing.T) {
	c, market, record := launchedChainControlEnv(t)
	newHolder := common.BytesToAddress([]byte{0x43})
	collection := common.HexToAddress(market.FeeRightCollection)
	require.NoError(t, c.k.TransferFeeRight(c.ctx, collection, recordedCreator, recordedCreator, newHolder, record.FeeRightID))
	owner, err := c.k.FeeRightOwnerOf(c.ctx, collection, record.FeeRightID)
	require.NoError(t, err)
	require.Equal(t, newHolder, owner)
	require.ErrorIs(t, c.k.PauseLaunchedChain(c.ctx, record.RegistryNumber, recordedCreator), types.ErrNotFeeRightHolder)
	require.NoError(t, c.k.PauseLaunchedChain(c.ctx, record.RegistryNumber, newHolder))
	require.ErrorIs(t, c.k.ResumeLaunchedChain(c.ctx, record.RegistryNumber, recordedCreator), types.ErrNotFeeRightHolder)
	require.NoError(t, c.k.ResumeLaunchedChain(c.ctx, record.RegistryNumber, newHolder))
	require.ErrorIs(t, c.k.RetireLaunchedChain(c.ctx, record.RegistryNumber, recordedCreator), types.ErrNotFeeRightHolder)
	require.NoError(t, c.k.RetireLaunchedChain(c.ctx, record.RegistryNumber, newHolder))
	domain, found := testkeeper.EVMTestApp.LayerXChainsKeeper.GetDomain(c.ctx, uint32(record.RegistryNumber))
	require.True(t, found)
	require.Equal(t, chainstypes.StatusRetired, domain.Status)
}

func TestLaunchedChainControlRetirePaysTheBondToTheHolder(t *testing.T) {
	c, _, record := launchedChainControlEnv(t)
	app := testkeeper.EVMTestApp
	registry := app.LayerXChainsKeeper
	registryID := uint32(record.RegistryNumber)
	domain, found := registry.GetDomain(c.ctx, registryID)
	require.True(t, found)
	bond := domain.Bond
	revenue := types.ChainRevenueAccount(record.RegistryNumber)
	holder := app.EvmKeeper.GetPaxAddressOrDefault(c.ctx, recordedCreator)
	beforeHolder := launchedChainControlBalance(c, holder)
	beforeRevenue := launchedChainControlBalance(c, revenue)
	epoch := types.ChainRevenueEpoch{Chain: record.RegistryNumber, Epoch: 1, Amount: sdk.OneInt(),
		SnapshotBlock: 1, EligibleSupply: sdk.OneInt(), ClaimedTotal: sdk.ZeroInt(), OpenedAt: 1}
	require.NoError(t, c.k.SetChainRevenueEpoch(c.ctx, epoch))
	require.NoError(t, c.k.RetireLaunchedChain(c.ctx, record.RegistryNumber, recordedCreator))
	require.True(t, launchedChainControlBalance(c, holder).Equal(beforeHolder.Add(bond)))
	require.True(t, launchedChainControlBalance(c, revenue).Equal(beforeRevenue))
	domain, found = registry.GetDomain(c.ctx, registryID)
	require.True(t, found)
	require.Equal(t, chainstypes.StatusRetired, domain.Status)
	require.True(t, domain.Bond.IsZero())
	stored, found, err := c.k.GetLaunchedChain(c.ctx, record.RegistryNumber)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, record, stored)
	storedEpoch, found, err := c.k.GetChainRevenueEpoch(c.ctx, record.RegistryNumber, 1)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, epoch, storedEpoch)
	require.Equal(t, uint64(1), c.k.LastChainRevenueEpoch(c.ctx, record.RegistryNumber))
	var retiredEvents int
	for _, event := range c.ctx.EventManager().Events() {
		if event.Type != types.EventTypeLaunchedChainRetired {
			continue
		}
		retiredEvents++
		require.Contains(t, event.Attributes, sdk.NewAttribute("chain", strconv.FormatUint(record.RegistryNumber, 10)).ToKVPair())
		require.Contains(t, event.Attributes, sdk.NewAttribute(types.AttributeKeyHolder, recordedCreator.Hex()).ToKVPair())
		require.Contains(t, event.Attributes, sdk.NewAttribute("returned_bond", bond.String()).ToKVPair())
	}
	require.Equal(t, 1, retiredEvents)
	require.ErrorIs(t, c.k.RetireLaunchedChain(c.ctx, record.RegistryNumber, recordedCreator), chainstypes.ErrDomainRetired)
}

func TestLaunchedChainControlRetireKeepsWaitingFeeShare(t *testing.T) {
	c, _, record := launchedChainControlEnv(t)
	app := testkeeper.EVMTestApp
	denom := app.EvmKeeper.GetBaseDenom(c.ctx)
	waiting := sdk.NewInt(77)
	coins := sdk.NewCoins(sdk.NewCoin(denom, waiting))
	require.NoError(t, app.BankKeeper.MintCoins(c.ctx, evmtypes.ModuleName, coins))
	revenue := types.ChainRevenueAccount(record.RegistryNumber)
	require.NoError(t, app.BankKeeper.SendCoinsFromModuleToAccount(c.ctx, evmtypes.ModuleName, revenue, coins))
	holder := app.EvmKeeper.GetPaxAddressOrDefault(c.ctx, recordedCreator)
	beforeHolder := launchedChainControlBalance(c, holder)
	domain, found := app.LayerXChainsKeeper.GetDomain(c.ctx, uint32(record.RegistryNumber))
	require.True(t, found)
	require.NoError(t, c.k.RetireLaunchedChain(c.ctx, record.RegistryNumber, recordedCreator))
	require.True(t, launchedChainControlBalance(c, holder).Equal(beforeHolder.Add(domain.Bond)))
	require.True(t, launchedChainControlBalance(c, revenue).Equal(waiting))
}

func TestLaunchedChainControlChainWithoutLaunchRecordIsRefused(t *testing.T) {
	c, value := launchChainEnv(t)
	app := testkeeper.EVMTestApp
	registry := app.LayerXChainsKeeper
	params := registry.GetParams(c.ctx)
	payer := app.EvmKeeper.GetPaxAddressOrDefault(c.ctx, recordedCreator)
	domain, err := registry.CreateWithPayer(c.ctx, payer, recordedCreator, "independent-chain",
		params.MinGasAllowance, params.MinByteAllowance, 100, value, evmtypes.DefaultMinFeePerGas)
	require.NoError(t, err)
	_, found, err := c.k.GetLaunchedChain(c.ctx, uint64(domain.Id))
	require.NoError(t, err)
	require.False(t, found)
	require.ErrorIs(t, c.k.PauseLaunchedChain(c.ctx, uint64(domain.Id), recordedCreator), types.ErrNotLaunchedChain)
	require.ErrorIs(t, c.k.ResumeLaunchedChain(c.ctx, uint64(domain.Id), recordedCreator), types.ErrNotLaunchedChain)
	require.ErrorIs(t, c.k.RetireLaunchedChain(c.ctx, uint64(domain.Id), recordedCreator), types.ErrNotLaunchedChain)
	after, found := registry.GetDomain(c.ctx, domain.Id)
	require.True(t, found)
	require.Equal(t, domain, after)
}
