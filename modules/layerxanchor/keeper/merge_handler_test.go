package keeper_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachemulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func TestMergeHandlerRetiresAnchor(t *testing.T) {
	for _, name := range []string{"guarantor_bonds", "mixed_escrow", "refund_error"} {
		t.Run(name, func(t *testing.T) {
			testApp := app.Setup(t, false, false, false)
			t.Cleanup(func() { require.NoError(t, testApp.Close()) })
			ctx := testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(8).WithBlockTime(time.Unix(1_800_000_000, 0))
			stores := make(map[storetypes.StoreKey]storetypes.CacheWrapper)
			keys := make(map[string]storetypes.StoreKey)
			for _, key := range ctx.MultiStore().StoreKeys() {
				stores[key] = ctx.MultiStore().GetKVStore(key)
				keys[key.Name()] = key
			}
			anchorKey := sdk.NewKVStoreKey(types.StoreKey)
			anchorDB := dbm.NewMemDB()
			t.Cleanup(func() { require.NoError(t, anchorDB.Close()) })
			stores[anchorKey] = dbadapter.Store{DB: anchorDB}
			keys[anchorKey.Name()] = anchorKey
			cacheDB := dbm.NewMemDB()
			t.Cleanup(func() { require.NoError(t, cacheDB.Close()) })
			ctx = ctx.WithMultiStore(cachemulti.NewStore(cacheDB, stores, keys, nil, nil, nil, 0))
			testApp.LayerXAnchorKeeper = keeper.NewKeeper(anchorKey, testApp.AccountKeeper, testApp.BankKeeper)
			s := initializeSuite(t, testApp, ctx, nil)
			require.Equal(t, "v7.0", app.MergeUpgrade)
			require.True(t, s.app.UpgradeKeeper.HasHandler(app.MergeUpgrade))
			require.Zero(t, s.app.UpgradeKeeper.GetDoneHeight(s.ctx, app.MergeUpgrade))
			s.bondAll()
			challengeBond := sdk.ZeroInt()
			if name == "mixed_escrow" {
				_, err := s.submit("batch_1_quorum")
				require.NoError(t, err)
				_, err = s.k.BeginUnbond(s.ctx, s.operators[0], s.ids[0], sdk.NewInt(1_000_000))
				require.NoError(t, err)
				require.Len(t, s.k.GetUnbondings(s.ctx), 1)
				challengeBond = s.k.GetParams(s.ctx).ChallengeBond
				s.fund(s.reporter, challengeBond.Int64())
				_, err = s.k.OpenChallenge(s.ctx, s.reporter, 1, types.ChallengeFraud, [32]byte{1}, challengeBond)
				require.NoError(t, err)
				require.Len(t, s.k.GetChallenges(s.ctx), 1)
			}
			total := sdk.NewInt(int64(len(s.operators)) * bond).Add(challengeBond)
			require.Equal(t, total, s.k.ModuleBalance(s.ctx))
			require.Equal(t, total, s.k.Escrowed(s.ctx))
			if name == "refund_error" {
				coins := sdk.NewCoins(sdk.NewInt64Coin(s.k.GetParams(s.ctx).BondDenom, 1))
				require.NoError(t, s.app.BankKeeper.SendCoinsFromAccountToModule(s.ctx, s.operators[0], types.ModuleName, coins))
			}
			balances := make([]sdk.Int, len(s.operators))
			for i, operator := range s.operators {
				balances[i] = s.balance(operator)
			}
			reporterBalance := s.balance(s.reporter)
			before := s.k.ExportGenesis(s.ctx)
			moduleBalance := s.k.ModuleBalance(s.ctx)
			supply := s.app.BankKeeper.GetSupply(s.ctx, s.k.GetParams(s.ctx).BondDenom)
			versions := s.app.UpgradeKeeper.GetModuleVersionMap(s.ctx)
			sequencerBefore, err := s.app.SequencerKeeper.ExportGenesis(s.ctx)
			require.NoError(t, err)

			const mergeHeight = int64(42)
			publicKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x11}, ed25519.SeedSize)).Public().(ed25519.PublicKey)
			info, err := json.Marshal(map[string]any{
				"leaders": [][]byte{bytes.Repeat([]byte{0x51}, 32)},
				"receipt_keys": seqkeeper.ReceiptKeys{{
					KeyId: bytes.Repeat([]byte{0x44}, seqtypes.KeyIDLength), PublicKey: publicKey,
					FirstHeight: uint64(mergeHeight + 1), LastHeight: uint64(mergeHeight + 100),
				}},
			})
			require.NoError(t, err)
			plan := upgradetypes.Plan{Name: app.MergeUpgrade, Height: mergeHeight, Info: string(info)}
			require.NoError(t, s.app.UpgradeKeeper.ScheduleUpgrade(s.ctx, plan))
			s.ctx = s.ctx.WithBlockHeight(mergeHeight).WithEventManager(sdk.NewEventManager())

			if name == "refund_error" {
				var refusal any
				func() {
					defer func() { refusal = recover() }()
					s.app.UpgradeKeeper.ApplyUpgrade(s.ctx, plan)
				}()
				upgradeErr, ok := refusal.(error)
				require.True(t, ok, "expected upgrade to panic with a refund error, got %v", refusal)
				require.ErrorIs(t, upgradeErr, types.ErrBond)
				require.ErrorContains(t, upgradeErr, "upgrade v7.0: retire anchor")
				require.Equal(t, before, s.k.ExportGenesis(s.ctx))
				require.Equal(t, moduleBalance, s.k.ModuleBalance(s.ctx))
				for i, operator := range s.operators {
					require.Equal(t, balances[i], s.balance(operator))
				}
				require.Equal(t, reporterBalance, s.balance(s.reporter))
				require.Equal(t, supply, s.app.BankKeeper.GetSupply(s.ctx, s.k.GetParams(s.ctx).BondDenom))
				require.Equal(t, versions, s.app.UpgradeKeeper.GetModuleVersionMap(s.ctx))
				sequencerAfter, err := s.app.SequencerKeeper.ExportGenesis(s.ctx)
				require.NoError(t, err)
				require.Equal(t, sequencerBefore, sequencerAfter)
				require.Zero(t, s.app.UpgradeKeeper.GetDoneHeight(s.ctx, app.MergeUpgrade))
				pending, found := s.app.UpgradeKeeper.GetUpgradePlan(s.ctx)
				require.True(t, found)
				require.Equal(t, plan, pending)
				require.Empty(t, s.ctx.EventManager().Events())
				return
			}

			require.NotPanics(t, func() { s.app.UpgradeKeeper.ApplyUpgrade(s.ctx, plan) })
			for i, operator := range s.operators {
				require.Equal(t, balances[i].AddRaw(bond), s.balance(operator))
			}
			require.Equal(t, reporterBalance.Add(challengeBond), s.balance(s.reporter))
			require.Len(t, s.k.GetGuarantors(s.ctx), len(s.ids))
			for _, guarantor := range s.k.GetGuarantors(s.ctx) {
				require.True(t, guarantor.Bond.IsZero())
			}
			require.Empty(t, s.k.GetUnbondings(s.ctx))
			for _, challenge := range s.k.GetChallenges(s.ctx) {
				require.Equal(t, types.ChallengeUpheld, challenge.Status)
				require.Equal(t, mergeHeight, challenge.ResolvedHeight)
			}
			require.True(t, s.k.ModuleBalance(s.ctx).IsZero())
			require.True(t, s.k.Escrowed(s.ctx).IsZero())
			s.invariant()
			require.Equal(t, supply, s.app.BankKeeper.GetSupply(s.ctx, s.k.GetParams(s.ctx).BondDenom))
			require.Equal(t, mergeHeight, s.app.UpgradeKeeper.GetDoneHeight(s.ctx, app.MergeUpgrade))
			_, pending := s.app.UpgradeKeeper.GetUpgradePlan(s.ctx)
			require.False(t, pending)
			require.True(t, s.app.Sequenced(s.ctx, mergeHeight+1))
			var retired sdk.Events
			for _, event := range s.ctx.EventManager().Events() {
				if event.Type == types.EventAnchorRetired {
					retired = append(retired, event)
				}
			}
			require.Equal(t, sdk.Events{sdk.NewEvent(types.EventAnchorRetired,
				sdk.NewAttribute(types.AttributeReturned, total.String()))}, retired)
		})
	}
}
