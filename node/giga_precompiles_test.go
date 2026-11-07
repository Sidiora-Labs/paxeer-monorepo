package app

import (
	"fmt"
	"testing"
	"time"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/ethereum/go-ethereum/core/vm"

	gigaprecompiles "github.com/Sidiora-Labs/Paxeer-X-Network/engine/executor/precompiles"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestGigaCustomPrecompilesSwitchAtHeight(t *testing.T) {
	for _, mergedGenesis := range []bool{false, true} {
		t.Run(fmt.Sprint("merged-genesis=", mergedGenesis), func(t *testing.T) {
			w := NewTestWrapperWithSc(t, time.Unix(1_800_000_000, 0).UTC(), secp256k1.GenPrivKey().PubKey(), true)
			t.Cleanup(func() { require.NoError(t, w.App.Close()) })
			if !mergedGenesis {
				w.App.UpgradeKeeper.SetDone(w.Ctx.WithBlockHeight(10), MergeUpgrade)
			}
			for _, height := range []int64{1, 9, 10, gigaLatePrecompileHeight - 1, gigaLatePrecompileHeight, gigaLatePrecompileHeight + 1} {
				ctx := w.Ctx.WithBlockHeight(height)
				w.App.RefreshPrecompiles(ctx)
				want := make(map[common.Address]vm.PrecompiledContract)
				for addr, contract := range gigaV6120FailFast {
					want[addr] = contract
				}
				if height < gigaLatePrecompileHeight {
					if !mergedGenesis && height < 10 {
						// Every latest contract carries the future merge version;
						// only the historical fail-fast set remains below it.
						want = make(map[common.Address]vm.PrecompiledContract)
						for _, addr := range gigaprecompiles.FailFastPrecompileAddresses {
							want[addr] = gigaprecompiles.FailFastSingleton
						}
					} else if mergedGenesis {
						// Fresh genesis activates table entries through their
						// module versions, including absence of uninitialised modules.
						versions := w.App.UpgradeKeeper.GetModuleVersionMap(ctx)
						for _, gate := range []evmkeeper.CustomPrecompileActivation{activationPrecompileGate(), v6120PrecompileGate(), mergePrecompileGate()} {
							for addr, modules := range gate.Modules {
								for _, name := range modules {
									if _, present := versions[name]; !present {
										delete(want, addr)
									}
								}
							}
						}
					}
				}
				require.Equal(t, want, w.App.gigaCustomPrecompiles(ctx), "height %d", height)
				for addr := range mergePrecompileGate().Modules {
					_, served := w.App.EvmKeeper.CustomPrecompiles(ctx)[addr]
					expected := height >= 10
					if mergedGenesis {
						expected = true
						versions := w.App.UpgradeKeeper.GetModuleVersionMap(ctx)
						for _, name := range mergePrecompileGate().Modules[addr] {
							if _, present := versions[name]; !present {
								expected = false
							}
						}
					}
					require.Equal(t, expected, served, "address %s at %d", addr, height)
				}
			}
		})
	}

	require.Len(t, gigaV6120FailFast, len(gigaprecompiles.AllCustomPrecompilesFailFastMerge)+6)
	for addr, contract := range gigaprecompiles.AllCustomPrecompilesFailFastMerge {
		require.Contains(t, gigaV6120FailFast, addr)
		require.Same(t, contract, gigaV6120FailFast[addr])
	}
	for _, addr := range []string{
		"0x0000000000000000000000000000000000001015",
		"0x0000000000000000000000000000000000001017",
		"0x0000000000000000000000000000000000001020",
		"0x0000000000000000000000000000000000001021",
		"0x0000000000000000000000000000000000001022",
		"0x0000000000000000000000000000000000001023",
		"0x0000000000000000000000000000000000001024",
		"0x0000000000000000000000000000000000001025",
	} {
		require.Contains(t, gigaV6120FailFast, common.HexToAddress(addr))
		require.Same(t, gigaprecompiles.FailFastSingleton, gigaV6120FailFast[common.HexToAddress(addr)])
	}
	require.NotContains(t, gigaV6120FailFast, common.HexToAddress("0x0000000000000000000000000000000000001026"))
	require.NotContains(t, gigaV6120FailFast, common.HexToAddress("0x0000000000000000000000000000000000001027"))
}

func TestGigaFailFastCoversMergeAddresses(t *testing.T) {
	for _, addr := range []string{
		"0x101A",
		"0x101B",
		"0x101C",
		"0x101D",
		"0x0901",
		"0x0902",
		"0x0903",
		"0x0904",
		"0x0905",
		"0x0906",
		"0x0907",
		"0x0908",
	} {
		t.Run(addr, func(t *testing.T) {
			address := common.HexToAddress(addr)
			require.Contains(t, gigaV6120FailFast, address)
			require.Same(t, gigaprecompiles.FailFastSingleton, gigaV6120FailFast[address])
		})
	}
}
