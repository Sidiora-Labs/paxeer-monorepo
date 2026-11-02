package sequencer_test

import (
	"context"
	"math/big"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	xwebtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	xwebprecompile "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/xweb"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/stretchr/testify/require"
)

func TestExecutorOpenRefreshesPrecompilesItself(t *testing.T) {
	sender := utils.NewSigner()
	tm := time.Now().UTC()
	valPub := secp256k1.GenPrivKey().PubKey()
	tw := app.NewTestWrapper(t, tm, valPub, true)
	a := tw.App
	a.EvmKeeper.SetAddressMapping(tw.Ctx, sender.AccountAddress, sender.EvmAddress)
	tw.FundAcc(sender.AccountAddress, sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewInt(fundingUhpx))))

	store := tw.Ctx.KVStore(a.GetKey(xwebtypes.StoreKey))
	iter := store.Iterator(nil, nil)
	var stale [][]byte
	for ; iter.Valid(); iter.Next() {
		stale = append(stale, append([]byte(nil), iter.Key()...))
	}
	require.NoError(t, iter.Close())
	require.NotEmpty(t, stale)
	for _, key := range stale {
		store.Delete(key)
	}
	tw.Ctx.KVStore(a.GetKey(upgradetypes.StoreKey)).Delete(append([]byte{upgradetypes.VersionMapByte}, xwebtypes.ModuleName...))
	require.NoError(t, a.UpgradeKeeper.ScheduleUpgrade(tw.Ctx, upgradetypes.Plan{
		Name: precompiles.XWebUpgrade, Height: openHeight + 1, Info: minorXWebPlan,
	}))
	require.True(t, a.UpgradeKeeper.HasHandler(precompiles.XWebUpgrade))

	_, err := a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{ChainID: testChainID, Height: committedTip},
	})
	require.NoError(t, err)
	a.SetDeliverStateToCommit()
	_, err = a.Commit(context.Background())
	require.NoError(t, err)

	exec, err := sequencer.NewExecutor(a, sequencer.Options{
		ChainID: testChainID, ProposerAddress: valPub.Address().Bytes(),
	})
	require.NoError(t, err)
	h := &harness{t: t, app: a, exec: exec, proposer: valPub.Address().Bytes(), genesis: tm}
	openMs := h.open()
	open := h.openCtx()
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(open, precompiles.XWebUpgrade))
	require.NotContains(t, a.UpgradeKeeper.GetModuleVersionMap(open), xwebtypes.ModuleName)
	require.NotContains(t, a.EvmKeeper.CustomPrecompiles(open), xwebAddress)

	info := xwebInfo()
	feeInput, err := info.ABI.Pack(xwebprecompile.FeeMethod)
	require.NoError(t, err)
	_, bz := evmTx(t, sender, receiptLegacyTx(0, &xwebAddress, big.NewInt(0), callGas, feeInput))
	before, err := exec.Accept(bz)
	require.NoError(t, err)
	require.Equal(t, uint32(0), before.Result.Code, before.Result.Log)
	require.Empty(t, evmReturnData(t, before.Result))

	batch, err := exec.Seal(openHeight)
	require.NoError(t, err)
	results := finalizeSealed(t, a, batch, sealedBlock(t, batch, [][32]byte{before.ActivityID}))
	require.Equal(t, batch.Results[0], results[1])
	require.NotContains(t, a.EvmKeeper.CustomPrecompiles(committedAt(a, openHeight)), xwebAddress)

	require.NoError(t, exec.Open(openHeight+1, openMs+1000))
	open = h.openCtx()
	require.Equal(t, openHeight+1, a.UpgradeKeeper.GetDoneHeight(open, precompiles.XWebUpgrade))
	require.Contains(t, a.UpgradeKeeper.GetModuleVersionMap(open), xwebtypes.ModuleName)
	require.Contains(t, a.EvmKeeper.CustomPrecompiles(open), xwebAddress)
	_, bz = evmTx(t, sender, receiptLegacyTx(1, &xwebAddress, big.NewInt(0), callGas, feeInput))
	after, err := exec.Accept(bz)
	require.NoError(t, err)
	require.Equal(t, 1, after.TxIndex)
	require.Equal(t, before.Seq+1, after.Seq)
	require.Equal(t, uint32(0), after.Result.Code, after.Result.Log)
	values, err := info.ABI.Unpack(xwebprecompile.FeeMethod, evmReturnData(t, after.Result))
	require.NoError(t, err)
	require.Len(t, values, 1)
	require.Equal(t, xwebtypes.DefaultParams(xwebtypes.DefaultAuthority()).Fee.Mul(state.SdkUhpxToSweiMultiplier).BigInt(), values[0])
}
