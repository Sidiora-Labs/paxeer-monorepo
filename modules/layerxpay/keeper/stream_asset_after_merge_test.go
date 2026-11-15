package keeper

import (
	"crypto/sha256"
	"fmt"
	"math/big"
	"testing"

	custodykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/keeper"
	custodytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/keeper/internal/streamassetfixture"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	upgradekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/keeper"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func streamAssetAfterMerge(t *testing.T, h *payHarness) {
	t.Helper()
	h.cms.Commit(true)
	upgradeKey := sdk.NewKVStoreKey(upgradetypes.StoreKey)
	h.cms.MountStoreWithDB(upgradeKey, sdk.StoreTypeIAVL, nil)
	require.NoError(t, h.cms.LoadLatestVersion())
	h.ctx = sdk.NewContext(h.cms, h.ctx.BlockHeader(), false)
	upgrade := upgradekeeper.NewKeeper(nil, upgradeKey, h.cdc, "", nil)
	upgrade.SetDone(h.ctx.WithBlockHeight(10), custodytypes.MergePlanUpgrade)
	var custodyKey sdk.StoreKey
	for _, key := range h.cms.StoreKeys() {
		if key.Name() == custodytypes.StoreKey {
			custodyKey = key
		}
	}
	require.NotNil(t, custodyKey)
	h.custody = custodykeeper.NewKeeper(h.cdc, custodyKey, nil, nil, h.evm, upgrade)
	h.keeper.custody = h.custody
	h.ctx = h.ctx.WithBlockHeight(11).WithEVMTxHash(common.HexToHash("0xa515"))
	require.True(t, h.custody.Retired(h.ctx))
	h.keeper.SetParams(h.ctx, types.Params{StreamEnabled: true})
	gate, err := types.KernelModuleEnableKey(types.StreamModuleID)
	require.NoError(t, err)
	h.ctx.KVStore(h.hostKey).Set(gate, payGateEnabledRecord())
}

func streamAssetNativeHarness(t *testing.T) (*payHarness, [32]byte) {
	t.Helper()
	h, _ := newBudgetPullHarness(t)
	if _, err := sdk.GetBaseDenom(); err != nil {
		require.NoError(t, sdk.RegisterDenom("uhpx", sdk.OneDec()))
	}
	asset, row, err := streamassetfixture.NativeAsset(h.actors[0].id)
	require.NoError(t, err)
	h.ctx.KVStore(h.hostKey).Set(types.KernelKVKey(1, append([]byte("asset:"), asset[:]...)), row)
	for i, actor := range h.actors {
		name, id, err := lxphost.MainAccountID(actor.public)
		require.NoError(t, err)
		var balance uint64
		if i == 0 {
			balance = 200
		}
		row, err := streamassetfixture.Account(name, id, asset, actor.public, balance)
		require.NoError(t, err)
		h.ctx.KVStore(h.hostKey).Set(append([]byte{0x06}, id[:]...), row)
	}
	streamAssetAfterMerge(t, h)
	return h, asset
}

func streamAssetOpen(t *testing.T, h *payHarness, token common.Address, asset [32]byte) {
	t.Helper()
	start := uint64(payHarnessTimeMs / 1000)
	id, err := h.keeper.OpenAgentStream(h.ctx, h.actors[0].address, h.actors[1].address,
		token, big.NewInt(2), start, start+100, big.NewInt(50))
	require.NoError(t, err)
	var key [32]byte
	id.FillBytes(key[:])
	record, err := h.keeper.GetStream(h.ctx, key)
	require.NoError(t, err)
	require.Equal(t, asset, record.AssetID)
	payEscrowBalance(t, h, record.Payer, asset, "150")
	payEscrowBalance(t, h, record.StreamAccount, asset, "50")
	payEscrowBalance(t, h, record.Recipient, asset, "0")
}

func TestStreamAssetAfterMergeNativeOpensWithoutCustodyMapping(t *testing.T) {
	for _, custodyReader := range []bool{true, false} {
		t.Run(fmt.Sprintf("custody_reader_%t", custodyReader), func(t *testing.T) {
			h, asset := streamAssetNativeHarness(t)
			_, found := h.custody.GetAssetByDenom(h.ctx, sdk.MustGetBaseDenom())
			require.False(t, found)
			require.Empty(t, h.custody.ExportGenesis(h.ctx).Assets)
			require.ErrorIs(t, h.custody.SetAsset(h.ctx, custodytypes.AssetMapping{
				AssetId: custodytypes.Hash32(asset), Denom: sdk.MustGetBaseDenom(), Enabled: true,
			}), custodytypes.ErrModuleRetired)
			ledgerAsset, err := h.host.ReadAsset(h.ctx, asset)
			require.NoError(t, err)
			require.Equal(t, "PAX", ledgerAsset.Symbol)
			if !custodyReader {
				h.keeper.custody = nil
			}
			streamAssetOpen(t, h, common.Address{}, ledgerAsset.AssetID)
		})
	}
}

func TestStreamAssetAfterMergeNativeIDMatchesKernel(t *testing.T) {
	h, kernelAsset := streamAssetNativeHarness(t)
	require.Equal(t, int64(125), h.evm.ChainID(h.ctx).Int64())
	require.Equal(t, sha256.Sum256([]byte("layerx-asset:125:PAX")), kernelAsset)
	asset, err := h.keeper.agentStreamAsset(h.ctx, common.Address{})
	require.NoError(t, err)
	require.Equal(t, kernelAsset, agentStreamNativeAssetID())
	require.Equal(t, kernelAsset, asset)
}

func TestStreamAssetAfterMergeCustodyMappingKeepsPrecedence(t *testing.T) {
	for _, kind := range []string{"native", "pointer", "factory"} {
		t.Run(kind, func(t *testing.T) {
			h, fixture, token := newAgentStreamHarness(t, kind)
			streamAssetAfterMerge(t, h)
			require.NotEqual(t, agentStreamNativeAssetID(), fixture.AssetID)
			native, row, err := streamassetfixture.NativeAsset(h.actors[0].id)
			require.NoError(t, err)
			h.ctx.KVStore(h.hostKey).Set(types.KernelKVKey(1, append([]byte("asset:"), native[:]...)), row)
			if kind == "pointer" {
				require.NoError(t, h.evm.SetERC20NativePointer(h.ctx, ledgertypes.MirrorDenom(native), token))
			}
			streamAssetOpen(t, h, token, fixture.AssetID)
		})
	}
}

func TestStreamAssetAfterMergePointerReadsLedgerRecord(t *testing.T) {
	for _, kind := range []string{"mirror", "issuer"} {
		t.Run(kind, func(t *testing.T) {
			h, _ := newBudgetPullHarness(t)
			fixture := payStreamOpenFixture(t, h)
			denom := ledgertypes.MirrorDenom(fixture.AssetID)
			if kind == "issuer" {
				denom = "factory/" + authtypes.NewModuleAddress(ledgertypes.ModuleName).String() + "/" +
					fmt.Sprintf("%043s", new(big.Int).SetBytes(fixture.AssetID[:]).Text(62))
			}
			token := common.HexToAddress("0x1515")
			require.NoError(t, h.evm.SetERC20NativePointer(h.ctx, denom, token))
			streamAssetAfterMerge(t, h)
			require.Empty(t, h.custody.ExportGenesis(h.ctx).Assets)
			streamAssetOpen(t, h, token, fixture.AssetID)
		})
	}
}

func TestStreamAssetAfterMergeUnknownTokenKeepsError(t *testing.T) {
	h, _ := newBudgetPullHarness(t)
	fixture := payStreamOpenFixture(t, h)
	streamAssetAfterMerge(t, h)
	missing := [32]byte{0xff}
	creator := authtypes.NewModuleAddress(ledgertypes.ModuleName).String()
	for i, denom := range []string{
		"", ledgertypes.MirrorDenom(missing),
		"factory/" + creator + "/" + fmt.Sprintf("%043s", new(big.Int).SetBytes(missing[:]).Text(62)),
		"factory/" + sdk.AccAddress(h.actors[0].address.Bytes()).String() + "/" + fmt.Sprintf("%043s", new(big.Int).SetBytes(fixture.AssetID[:]).Text(62)),
		"factory/" + creator + "/0" + fmt.Sprintf("%043s", new(big.Int).SetBytes(fixture.AssetID[:]).Text(62)),
		"factory/" + creator + "/" + new(big.Int).Lsh(big.NewInt(1), 256).Text(62),
	} {
		t.Run(fmt.Sprintf("token_%d", i), func(t *testing.T) {
			token := common.BytesToAddress([]byte{0xee, byte(i + 1)})
			if denom != "" {
				require.NoError(t, h.evm.SetERC20NativePointer(h.ctx, denom, token))
			}
			hostBefore, payBefore := payStoreSnapshot(h.ctx, h.hostKey), payStoreSnapshot(h.ctx, h.storeKey)
			start := uint64(payHarnessTimeMs / 1000)
			id, err := h.keeper.OpenAgentStream(h.ctx, h.actors[0].address, h.actors[1].address,
				token, big.NewInt(2), start, start+100, big.NewInt(50))
			require.ErrorIs(t, err, agentcommon.ErrStreamsUnknownToken)
			require.Nil(t, id)
			require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
			require.Equal(t, payBefore, payStoreSnapshot(h.ctx, h.storeKey))
		})
	}
}

func TestStreamAssetAfterMergeMissingNativeRecordKeepsError(t *testing.T) {
	h, _ := newBudgetPullHarness(t)
	payStreamOpenFixture(t, h)
	streamAssetAfterMerge(t, h)
	if _, err := sdk.GetBaseDenom(); err != nil {
		require.NoError(t, sdk.RegisterDenom("uhpx", sdk.OneDec()))
	}
	hostBefore, payBefore := payStoreSnapshot(h.ctx, h.hostKey), payStoreSnapshot(h.ctx, h.storeKey)
	start := uint64(payHarnessTimeMs / 1000)
	id, err := h.keeper.OpenAgentStream(h.ctx, h.actors[0].address, h.actors[1].address,
		common.Address{}, big.NewInt(2), start, start+100, big.NewInt(50))
	require.ErrorIs(t, err, agentcommon.ErrStreamsUnknownToken)
	require.Nil(t, id)
	require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	require.Equal(t, payBefore, payStoreSnapshot(h.ctx, h.storeKey))
}
