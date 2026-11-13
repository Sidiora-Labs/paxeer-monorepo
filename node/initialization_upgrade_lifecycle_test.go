package app_test

import (
	"testing"
	"time"

	tmed25519 "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/stretchr/testify/require"
)

func TestNodeLifecycleMigrationBatchConfiguration(t *testing.T) {
	app.CheckNodeLifecycleMigrationConfiguration(t)
}

func TestNodeLifecycleUnknownChainAndNativeReceipt(t *testing.T) {
	TestProofNodeAbci(t)
}

func TestNodeLifecycleSequencedSuccessAndMalformedHash(t *testing.T) {
	t.Run("delivery", TestSequencedBlockDeliversBatchAtTxZero)
	t.Run("refusals", TestSequencedBlockRefusesBadBatch)
	t.Run("OCC", TestSequencedBlockOCCEqualsSequential)
	t.Run("empty", TestSequencedBlockGenesisOneKeySet)
	t.Run("rotation", TestSequencedBlockReceiptKeyRotation)
	t.Run("export", TestSequencedBlockGenesisRoundTrip)
}

func TestNodeLifecycleHelperPreservesStrictValidation(t *testing.T) {
	require.False(t, tmtypes.SkipLastResultsHashValidation.Load())
	for _, sc := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "state-commit"}[sc], func(t *testing.T) {
			var w *app.TestWrapper
			if sc {
				w = app.NewTestWrapperWithSc(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), false)
			} else {
				w = app.NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), false)
			}
			t.Cleanup(func() { require.NoError(t, w.App.Close()) })
			require.False(t, tmtypes.SkipLastResultsHashValidation.Load())
			// A nested helper must restore the incoming process setting even on panic.
			t.Run("cleanup", func(t *testing.T) {
				defer func() { require.Equal(t, "helper exit", recover()) }()
				_ = app.NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), false)
				tmtypes.SkipLastResultsHashValidation.Store(true)
				panic("helper exit")
			})
			require.False(t, tmtypes.SkipLastResultsHashValidation.Load())
		})
		require.False(t, tmtypes.SkipLastResultsHashValidation.Load())
	}
	TestSequencedExecutionStrictResultCommitment(t)
}

func TestNodeLifecycleInitializationResultsAndFailures(t *testing.T) {
	app.CheckNodeLifecycleInitializationFailure(t)
	for _, name := range []string{"default-home", "SC", "SC-receipts", "accounts", "validators"} {
		t.Run(name, func(t *testing.T) {
			var a *app.App
			genesisTime := time.Unix(1_700_000_000, 0).UTC()
			switch name {
			case "default-home":
				a = app.SetupWithDefaultHome(false, false, false)
			case "SC":
				a = app.SetupWithSc(t, false, false, app.TestAppOpts{GenesisTime: genesisTime})
			case "SC-receipts":
				a = app.SetupWithScReceiptFromOpts(t, false, false, app.TestAppOpts{})
			case "accounts":
				a = app.SetupWithGenesisAccounts(t, nil)
			case "validators":
				sender := utils.NewSigner()
				vals := tmtypes.NewValidatorSet([]*tmtypes.Validator{tmtypes.NewValidator(tmed25519.GenerateSecretKey().Public(), 1)})
				a = app.SetupWithGenesisValSet(t, vals, []authtypes.GenesisAccount{authtypes.NewBaseAccountWithAddress(sender.AccountAddress)})
			}
			require.NotNil(t, a)
			t.Cleanup(func() { require.NoError(t, a.Close()) })
			ctx := a.NewContext(false, a.GetCheckCtx().BlockHeader())
			if name == "SC" {
				require.Equal(t, genesisTime, ctx.BlockTime())
			}
			require.NotEmpty(t, a.UpgradeKeeper.GetModuleVersionMap(ctx))
			require.NotEmpty(t, a.StakingKeeper.GetParams(ctx).BondDenom)
		})
	}
}

func TestNodeLifecycleActivationTagsAndOrderedPlans(t *testing.T) {
	app.CheckNodeLifecycleUpgradeLifecycle(t)
}
func TestNodeLifecycleChainsAddressActivation(t *testing.T) {
	app.CheckNodeLifecycleChainsActivation(t)
}
func TestNodeLifecycleMergedStoresPreserveState(t *testing.T) { app.CheckNodeLifecycleMergedStores(t) }
