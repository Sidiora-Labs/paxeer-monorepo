//go:build cgo

package lxphost

import (
	"bytes"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func TestReadBridgeDuringConcurrentExecute(t *testing.T) {
	const blockTime = int64(1700000000000)
	const workers, mints, reads = 4, 25, 100
	const salt = byte(0x39)
	key := sdk.NewKVStoreKey("lxphost")
	host := poolTestHost(t, key, workers)
	require.Equal(t, workers, cap(host.nodes))
	actor := newTestActor(t, 39)
	asset := newTestAsset(t, actor, salt)
	var chains [2 * workers]*testChain
	var contexts [2 * workers]sdk.Context
	var expected [2 * workers]AssetRecord
	var activities [mints]Activity
	for step := range activities {
		activities[step] = actor.mint(t, host, uint64(step+3), blockTime, asset, 1)
	}
	for lane := range chains {
		chain := poolTestChain(t, key)
		chains[lane] = chain
		funded := uint64(100 + lane)
		setup := []Activity{
			actor.registerAsset(t, host, 0, blockTime, asset, salt, 1000),
			actor.openAssetAccount(t, host, 1, blockTime, asset),
			actor.mint(t, host, 2, blockTime, asset, funded),
		}
		for step, activity := range setup {
			receipt := execute(t, host, chain.context(1, blockTime, uint64(step+1)), activity, actor.authority())
			require.Equal(t, Code(0), receipt.ResultCode, "setup lane %d step %d", lane, step)
		}
		chain.commit()
		contexts[lane] = chain.context(2, blockTime, 4)
		expected[lane] = AssetRecord{
			AssetID: asset.id, Symbol: "T", Name: "T", Decimals: 6,
			SupplyCap: sdk.NewInt(1000), IssuerDID: actor.id,
			IssuerKind: 1, CustodyKind: 1, CustodyReference: []byte{},
			TotalUnits: sdk.NewIntFromUint64(funded),
		}
		copy(expected[lane].Salt[:], bytes.Repeat([]byte{salt}, 32))
	}

	var ready, finished sync.WaitGroup
	start := make(chan struct{})
	ready.Add(2 * workers)
	finished.Add(2 * workers)
	for lane := 0; lane < workers; lane++ {
		go func(lane int) {
			defer finished.Done()
			ready.Done()
			<-start
			for step, activity := range activities {
				receipt, err := host.HostExecute(WithSequence(contexts[lane], uint64(step+4)), activity, actor.authority())
				assert.NoError(t, err, "mint lane %d step %d", lane, step)
				assert.Equal(t, Code(0), receipt.ResultCode, "mint lane %d step %d", lane, step)
			}
		}(lane)
		go func(lane int) {
			defer finished.Done()
			ready.Done()
			<-start
			for step := 0; step < reads; step++ {
				balance, err := host.ReadBalance(contexts[lane], asset.account, asset.id)
				assert.NoError(t, err, "balance lane %d step %d", lane, step)
				assert.Equal(t, expected[lane].TotalUnits, balance, "balance lane %d step %d", lane, step)
				record, err := host.ReadAsset(contexts[lane], asset.id)
				assert.NoError(t, err, "asset lane %d step %d", lane, step)
				assert.Equal(t, expected[lane], record, "asset lane %d step %d", lane, step)
			}
		}(workers + lane)
	}
	ready.Wait()
	close(start)
	finished.Wait()
	for lane := 0; lane < workers; lane++ {
		balance, err := host.ReadBalance(contexts[lane], asset.account, asset.id)
		require.NoError(t, err)
		require.Equal(t, expected[lane].TotalUnits.AddRaw(mints), balance, "mint lane %d", lane)
	}
}

func TestReadBridgeAfterCloseRefused(t *testing.T) {
	chain := newTestChain(t)
	host := poolTestHost(t, chain.key, 4)
	ctx := chain.context(1, 1700000000000, 1)
	name, accountID, err := MainAccountID([32]byte{1})
	require.NoError(t, err)
	id := [32]byte{2}
	host.Close()
	for _, test := range []struct {
		name string
		read func() error
	}{
		{"ReadBalance", func() error { _, err := host.ReadBalance(ctx, accountID, id); return err }},
		{"ReadAsset", func() error { _, err := host.ReadAsset(ctx, id); return err }},
		{"ReadAccount", func() error { _, err := host.ReadAccount(ctx, name, accountID); return err }},
		{"ReadSessionKey", func() error { _, err := host.ReadSessionKey(ctx, id); return err }},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := test.read()
			require.Equal(t, codeNonCanonical, err)
			require.EqualError(t, err, "LXP_ERR_NON_CANONICAL")
		})
	}
}
