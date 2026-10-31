//go:build cgo

package lxphost

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func poolTestHost(t *testing.T, key sdk.StoreKey, workers int) *Host {
	t.Helper()
	h, err := NewHost(Config{
		StoreKey: key, Workers: workers, NetworkID: testNetworkID,
		ParameterVersion: 1, TimestampWindowMs: 60000, GasLimit: 1000000,
	})
	require.NoError(t, err)
	t.Cleanup(h.Close)
	return h
}

func poolTestChain(t *testing.T, key *sdk.KVStoreKey) *testChain {
	t.Helper()
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db)
	cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	require.NoError(t, cms.LoadLatestVersion())
	return &testChain{key: key, cms: cms}
}

func TestHostConcurrentExecuteMatchesSequential(t *testing.T) {
	const blockTime = int64(1700000000000)
	const lanes, steps = 8, 3
	key := sdk.NewKVStoreKey("lxphost")
	sequential := poolTestHost(t, key, 0)
	pooled := poolTestHost(t, key, 4)
	var chains [lanes]*testChain
	var actors [lanes][steps]testActor
	var activities [lanes][steps]Activity
	var expected, actual [lanes][steps]Receipt
	var failures [lanes][steps]error
	var identities [lanes][steps]IdentityState
	var readFailures [lanes][steps]error
	var roots [lanes][]byte
	var states [lanes]map[string][]byte
	for lane := 0; lane < lanes; lane++ {
		baseline := poolTestChain(t, key)
		chains[lane] = poolTestChain(t, key)
		for step := 0; step < steps; step++ {
			actor := newTestActor(t, byte(1+lane*steps+step))
			actors[lane][step] = actor
			activities[lane][step] = actor.register(t, sequential, 0, blockTime)
			receipt := execute(t, sequential, baseline.context(1, blockTime, uint64(step+1)), activities[lane][step], actor.authority())
			require.Equal(t, Code(0), receipt.ResultCode)
			expected[lane][step] = receipt
		}
		roots[lane] = baseline.commit()
		states[lane] = baseline.dump()
	}
	var wg sync.WaitGroup
	start := make(chan struct{})
	for lane := 0; lane < lanes; lane++ {
		wg.Add(1)
		go func(lane int) {
			defer wg.Done()
			<-start
			for step := 0; step < steps; step++ {
				actual[lane][step], failures[lane][step] = pooled.HostExecute(
					chains[lane].context(1, blockTime, uint64(step+1)), activities[lane][step], actors[lane][step].authority())
				identities[lane][step], readFailures[lane][step] = pooled.ReadIdentityState(
					chains[lane].context(1, blockTime, uint64(step+2)), actors[lane][step].did)
			}
		}(lane)
	}
	close(start)
	wg.Wait()
	for lane := 0; lane < lanes; lane++ {
		for step := 0; step < steps; step++ {
			require.NoError(t, failures[lane][step], "lane %d step %d", lane, step)
			require.Equal(t, expected[lane][step], actual[lane][step], "lane %d step %d", lane, step)
			require.NoError(t, readFailures[lane][step], "lane %d step %d", lane, step)
			require.Equal(t, actors[lane][step].id, identities[lane][step].DIDID)
			identity, err := pooled.ReadIdentityState(chains[lane].context(1, blockTime, steps+1), actors[lane][step].did)
			require.NoError(t, err)
			require.Equal(t, actors[lane][step].id, identity.DIDID)
		}
		require.Equal(t, roots[lane], chains[lane].commit())
		require.Equal(t, states[lane], chains[lane].dump())
	}
	pooled.Close()
	pooled.Close()
	_, err := pooled.HostExecute(chains[0].context(1, blockTime, steps+1), activities[0][0], actors[0][0].authority())
	require.Equal(t, codeNonCanonical, err)
	_, err = pooled.ReadIdentityState(chains[0].context(1, blockTime, steps+1), actors[0][0].did)
	require.Equal(t, codeNonCanonical, err)

	t.Run("close waits for borrowed nodes", func(t *testing.T) {
		h := poolTestHost(t, key, 2)
		first, err := h.borrowNode()
		require.NoError(t, err)
		second, err := h.borrowNode()
		require.NoError(t, err)
		require.NotSame(t, first, second)
		closed := make(chan struct{})
		go func() {
			h.Close()
			close(closed)
		}()
		select {
		case <-closed:
			t.Fatal("Close freed nodes still in use")
		default:
		}
		h.returnNode(first)
		h.returnNode(second)
		select {
		case <-closed:
		case <-time.After(5 * time.Second):
			t.Fatal("Close did not finish after nodes returned")
		}
		_, err = h.borrowNode()
		require.Equal(t, codeNonCanonical, err)
	})
}

func TestNewHostRefusesWorkersAboveLimit(t *testing.T) {
	chain := newTestChain(t)
	cfg := Config{StoreKey: chain.key, NetworkID: testNetworkID, ParameterVersion: 1, GasLimit: 1000000, Workers: 17}
	host, err := NewHost(cfg)
	require.Nil(t, host)
	require.Equal(t, codeNonCanonical, err)
	require.Equal(t, "LXP_ERR_NON_CANONICAL", err.Error())
	cfg.Workers = 16
	host, err = NewHost(cfg)
	require.NoError(t, err)
	require.NotNil(t, host)
	require.Equal(t, 16, cap(host.nodes))
	require.Len(t, host.nodes, 16)
	host.Close()
	cfg.Workers = -1
	host, err = NewHost(cfg)
	require.Nil(t, host)
	require.Equal(t, codeNonCanonical, err)
}
