// Proves: Keeper.RegisterAsset/OpenAccount/Mint/Burn -> lxphost.Host and SDK KVStore/bank mirror -> AccountID and Keeper.AccountOwner/BalanceOf/GetAsset/GetParams/ExportGenesis and TestProofLayerxledgerKeeper.
//
// The consumer is this external Go package, as the keeper contract has no other
// caller anchor. Every activity, receipt, binding and balance is produced by the
// real keepers. The host and LevelDB store are closed and reopened before the
// consumer checks recovery, duplicate registration and the retired Draw refusal.
//
// Repairs modules/layerxledger/keeper/supply.go: Mint and Burn used to commit
// kernel transfers before a failing bank mirror, diverging the two ledgers.
// Real delayed-vesting accounts exercise that failure without a bank double;
// failed mirroring must preserve stores, sequence and events, including restart.
package keeper_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	vestingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/vesting/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
)

type proofLedgerKeeper struct {
	app    *app.App
	dir    string
	root   *rootmulti.Store
	ctx    sdk.Context
	host   *lxphost.Host
	keeper *ledgerkeeper.Keeper
	auth   authkeeper.AccountKeeper
	bank   bankkeeper.BaseKeeper
}

func newProofLedgerKeeper(t *testing.T) *proofLedgerKeeper {
	t.Helper()
	source := app.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, source.Close()) })
	p := &proofLedgerKeeper{app: source, dir: t.TempDir()}
	p.open(t, true)
	t.Cleanup(func() { p.close(t) })
	return p
}

func (p *proofLedgerKeeper) open(t *testing.T, initialize bool) {
	t.Helper()
	db, err := tmdb.NewGoLevelDB("proof-layerxledger", p.dir)
	require.NoError(t, err, "modules/layerxledger/keeper: open local producer store")
	p.root = rootmulti.NewStore(db)
	genesis := p.app.NewContext(false, tmproto.Header{})
	// Mount the application's actual keys. Genesis values come from InitChain,
	// and all subsequent records come exclusively from the real producers.
	for _, key := range genesis.MultiStore().StoreKeys() {
		var source storetypes.KVStore
		require.NotPanics(t, func() { source = genesis.MultiStore().GetKVStore(key) },
			"node.InitChain must mount producer store %s", key.Name())
		p.root.MountStoreWithDB(key, source.GetStoreType(), nil)
	}
	require.NoError(t, p.root.LoadLatestVersion())
	p.ctx = sdk.NewContext(p.root, tmproto.Header{
		ChainID: "hyperpax_125-1", Height: 11,
		Time: time.Unix(1_800_000_000, 0).UTC(),
	}, false)
	if initialize {
		for _, key := range p.root.StoreKeys() {
			if _, persistent := key.(*storetypes.KVStoreKey); !persistent {
				continue
			}
			iterator := genesis.KVStore(key).Iterator(nil, nil)
			for ; iterator.Valid(); iterator.Next() {
				p.ctx.KVStore(key).Set(append([]byte(nil), iterator.Key()...), append([]byte{}, iterator.Value()...))
			}
			require.NoError(t, iterator.Close())
		}
	}
	p.host, err = lxphost.NewHost(lxphost.Config{
		StoreKey: p.app.GetKey("lxphost"), NetworkID: 125, Epoch: 1,
		ParameterVersion: 1, TimestampWindowMs: 60_000, GasLimit: 10_000_000,
	})
	require.NoError(t, err, "modules/lxphost.NewHost: producer must start locally")
	p.auth = authkeeper.NewAccountKeeper(p.app.AppCodec(), p.app.GetKey(authtypes.StoreKey),
		p.app.GetSubspace(authtypes.ModuleName), authtypes.ProtoBaseAccount,
		map[string][]string{types.ModuleName: {authtypes.Minter}})
	p.bank = bankkeeper.NewBaseKeeper(p.app.AppCodec(), p.app.GetKey(banktypes.StoreKey), p.auth,
		p.app.GetSubspace(banktypes.ModuleName), nil)
	p.keeper = ledgerkeeper.NewKeeper(p.app.AppCodec(), p.app.GetKey(types.StoreKey), p.host,
		p.bank, &p.app.EvmKeeper, authtypes.NewModuleAddress("gov").String())
}

func (p *proofLedgerKeeper) close(t *testing.T) {
	t.Helper()
	if p.host != nil {
		p.host.Close()
		p.host = nil
	}
	if p.root != nil {
		require.NoError(t, p.root.Close())
		p.root = nil
	}
	p.keeper = nil
	p.ctx = sdk.Context{}
}

func (p *proofLedgerKeeper) restart(t *testing.T) {
	t.Helper()
	committed := p.root.Commit(true)
	require.Positive(t, committed.Version)
	require.NotEmpty(t, committed.Hash)
	p.close(t)
	p.open(t, false)
	require.Equal(t, committed, p.root.LastCommitID(), "reopen must load the committed database")
}

func (p *proofLedgerKeeper) bind(t *testing.T, seed byte) (ledgerkeeper.Actor, common.Address) {
	t.Helper()
	var material [ed25519.SeedSize]byte
	material[0] = seed
	private := ed25519.NewKeyFromSeed(material[:])
	var key [32]byte
	copy(key[:], private.Public().(ed25519.PublicKey))
	caller := common.Address{0x21, 0x16, seed}
	message, err := evmtypes.LayerXBindMessage(p.app.EvmKeeper.ChainID(p.ctx), caller, 0)
	require.NoError(t, err)
	var signature [ed25519.SignatureSize]byte
	copy(signature[:], ed25519.Sign(private, message))
	_, err = p.app.EvmKeeper.BindLayerX(p.ctx, caller, key, signature)
	require.NoError(t, err)
	actor, err := p.keeper.Actor(p.ctx, caller)
	require.NoError(t, err)
	require.Equal(t, key, actor.DIDKey)
	return actor, caller
}

func (p *proofLedgerKeeper) produce(t *testing.T) (ledgerkeeper.Actor, ledgerkeeper.Actor, common.Address, common.Address, types.RegisterArgs, [32]byte) {
	t.Helper()
	issuer, issuerCaller := p.bind(t, 161)
	holder, holderCaller := p.bind(t, 162)
	salt := [32]byte{0x44}
	material := append([]byte("LX:ASSET:v1"), issuer.Authority.Actor[:]...)
	asset := sha256.Sum256(append(material, salt[:]...))
	args := types.RegisterArgs{AssetID: asset, Salt: salt, Symbol: "PRF", Name: "Keeper proof",
		Decimals: 6, SupplyCap: sdk.NewInt(1000), IssuerKind: types.AssetIssuerNative}
	registered, err := p.keeper.RegisterAsset(p.ctx, issuer, args, [32]byte{1})
	require.NoError(t, err)
	require.Zero(t, registered.Receipt.ResultCode)
	require.NotEmpty(t, registered.Receipt.Encoded)
	opened, err := p.keeper.OpenAccount(p.ctx, holder, asset, [32]byte{2})
	require.NoError(t, err)
	account, err := ledgerkeeper.AccountID(holder.DIDKey, asset)
	require.NoError(t, err)
	events, err := ledgerkeeper.LedgerEvents(0x00010004, opened.Receipt, ledgerkeeper.EventArgs{})
	require.NoError(t, err)
	require.Equal(t, []ledgerkeeper.Event{{Name: "AccountOpened", Fields: map[string]any{"accountId": account}}}, events)
	minted, err := p.keeper.Mint(p.ctx, issuer, asset, account, sdk.NewInt(80), [32]byte{3})
	require.NoError(t, err)
	require.Len(t, minted.Receipt.Transfers, 1)
	require.Equal(t, account, minted.Receipt.Transfers[0].To)
	p.assertBalances(t, asset, account, 80)
	return issuer, holder, issuerCaller, holderCaller, args, account
}

func (p *proofLedgerKeeper) assertBalances(t *testing.T, asset, account [32]byte, want int64) {
	t.Helper()
	balance, err := p.keeper.BalanceOf(p.ctx, account, asset)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(want), balance)
	denom := ledgerkeeper.MirrorDenom(asset)
	require.Equal(t, balance, p.bank.GetBalance(p.ctx, ledgerkeeper.BankAddress(account), denom).Amount)
	_, issuance, err := ledgerkeeper.IssuanceAccountID(asset)
	require.NoError(t, err)
	reserve, err := p.keeper.BalanceOf(p.ctx, issuance, asset)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(1000-want), reserve)
	require.Equal(t, reserve, p.bank.GetBalance(p.ctx, ledgerkeeper.BankAddress(issuance), denom).Amount)
	total, err := p.keeper.TotalUnits(p.ctx, asset)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(want), total)
	require.Equal(t, sdk.NewInt(1000), p.bank.GetSupply(p.ctx, denom).Amount)
}

func TestProofLayerxledgerKeeper(t *testing.T) {
	p := newProofLedgerKeeper(t)
	require.Equal(t, types.DefaultParams(), p.keeper.GetParams(p.ctx))
	issuer, holder, issuerCaller, holderCaller, args, account := p.produce(t)
	burned, err := p.keeper.Burn(p.ctx, holder, args.AssetID, account, sdk.NewInt(30), [32]byte{4})
	require.NoError(t, err)
	require.Zero(t, burned.Receipt.ResultCode)
	require.Len(t, burned.Receipt.Transfers, 1)
	require.Equal(t, account, burned.Receipt.Transfers[0].From)
	require.Equal(t, sdk.NewInt(30), burned.Receipt.Transfers[0].Amount)
	events, err := ledgerkeeper.LedgerEvents(0x0001000b, burned.Receipt, ledgerkeeper.EventArgs{})
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "Transfer", events[0].Name)
	p.assertBalances(t, args.AssetID, account, 50)
	_, err = p.keeper.PauseAsset(p.ctx, issuer, args.AssetID, [32]byte{5})
	require.NoError(t, err)
	require.NoError(t, p.keeper.SetParams(p.ctx, types.Params{Paused: true}))
	exported := p.keeper.ExportGenesis(p.ctx)
	require.NoError(t, exported.Validate())
	p.restart(t)

	issuerAfter, err := p.keeper.Actor(p.ctx, issuerCaller)
	require.NoError(t, err)
	holderAfter, err := p.keeper.Actor(p.ctx, holderCaller)
	require.NoError(t, err)
	require.Equal(t, issuer, issuerAfter)
	require.Equal(t, holder, holderAfter)
	derived, err := ledgerkeeper.AccountID(holderAfter.DIDKey, args.AssetID)
	require.NoError(t, err)
	require.Equal(t, account, derived)
	owner, err := p.keeper.AccountOwner(p.ctx, derived)
	require.NoError(t, err)
	require.Equal(t, holder.Authority.Actor, owner)
	asset, err := p.keeper.GetAsset(p.ctx, args.AssetID)
	require.NoError(t, err)
	require.Equal(t, args.AssetID, asset.AssetID)
	require.Equal(t, args.Symbol, asset.Symbol)
	require.Equal(t, args.Name, asset.Name)
	require.Equal(t, issuer.Authority.Actor, asset.IssuerDID)
	require.Equal(t, sdk.NewInt(1000), asset.SupplyCap)
	require.Equal(t, sdk.NewInt(50), asset.TotalUnits)
	require.True(t, asset.Paused)
	require.Equal(t, types.Params{Paused: true}, p.keeper.GetParams(p.ctx))
	require.Equal(t, exported, p.keeper.ExportGenesis(p.ctx))
	encoded, err := json.Marshal(p.keeper.ExportGenesis(p.ctx))
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &fields))
	require.Len(t, fields, 1)
	require.Contains(t, fields, "params")
	p.assertBalances(t, args.AssetID, account, 50)

	before := ledgerViewStore(t, p.ctx, p.app.GetKey("lxphost"))
	_, err = p.keeper.Burn(p.ctx, holderAfter, args.AssetID, account, sdk.OneInt(), [32]byte{6})
	require.ErrorIs(t, err, ledgerkeeper.ErrPausedScope)
	require.Equal(t, before, ledgerViewStore(t, p.ctx, p.app.GetKey("lxphost")))
	_, err = p.keeper.GetAsset(p.ctx, [32]byte{})
	require.ErrorIs(t, err, types.ErrAssetMismatch)
	_, err = p.keeper.AccountOwner(p.ctx, [32]byte{})
	require.Equal(t, lxphost.Code(-208), err)
	_, err = p.keeper.BalanceOf(p.ctx, account, [32]byte{})
	require.ErrorIs(t, err, types.ErrAssetMismatch)
	require.NoError(t, p.keeper.SetParams(p.ctx, types.DefaultParams()))
	_, err = p.keeper.RegisterAsset(p.ctx, issuerAfter, args, [32]byte{1})
	require.ErrorIs(t, err, types.ErrAssetAlreadyRegistered, "restart must refuse duplicate production")
	p.assertBalances(t, args.AssetID, account, 50)
	_, err = p.keeper.UnpauseAsset(p.ctx, issuerAfter, args.AssetID, [32]byte{7})
	require.NoError(t, err)
	_, err = p.keeper.Burn(p.ctx, issuerAfter, args.AssetID, account, sdk.OneInt(), [32]byte{8})
	require.ErrorIs(t, err, types.ErrUnauthorizedDebit)
	before = ledgerViewStore(t, p.ctx, p.app.GetKey("lxphost"))
	draw := types.DrawArgs{From: account, Asset: args.AssetID, Amount: sdk.OneInt(), GrantID: [32]byte{9}}
	result, err := p.keeper.Draw(p.ctx, holderAfter, draw)
	require.ErrorIs(t, err, types.ErrNoPayerGrant)
	require.Equal(t, ledgerkeeper.Result{}, result)
	_, err = p.keeper.Draw(p.ctx, ledgerkeeper.Actor{}, draw)
	require.ErrorIs(t, err, types.ErrUnauthorizedDebit)
	require.Equal(t, before, ledgerViewStore(t, p.ctx, p.app.GetKey("lxphost")))
	p.assertBalances(t, args.AssetID, account, 50)
}

func TestProofLayerxledgerKeeperSupplyMirrorFailure(t *testing.T) {
	for _, operation := range []string{"mint", "burn"} {
		t.Run(operation, func(t *testing.T) {
			p := newProofLedgerKeeper(t)
			issuer, holder, _, _, args, account := p.produce(t)
			source := account
			if operation == "mint" {
				_, issuance, err := ledgerkeeper.IssuanceAccountID(args.AssetID)
				require.NoError(t, err)
				source = issuance
			}
			address := ledgerkeeper.BankAddress(source)
			base, ok := p.auth.GetAccount(p.ctx, address).(*authtypes.BaseAccount)
			require.True(t, ok, "real bank transfer must create a base account")
			locked := sdk.NewCoins(p.bank.GetBalance(p.ctx, address, ledgerkeeper.MirrorDenom(args.AssetID)))
			p.auth.SetAccount(p.ctx, vestingtypes.NewDelayedVestingAccount(base, locked, p.ctx.BlockTime().Unix()+3600, nil))
			require.Equal(t, locked, p.bank.LockedCoins(p.ctx, address))
			snapshots := map[sdk.StoreKey]map[string][]byte{}
			for _, key := range []sdk.StoreKey{p.app.GetKey("lxphost"), p.app.GetKey(types.StoreKey), p.app.GetKey(banktypes.StoreKey), p.app.GetKey(authtypes.StoreKey)} {
				snapshots[key] = ledgerViewStore(t, p.ctx, key)
			}
			eventCount := len(p.ctx.EventManager().Events())
			execute := func() (ledgerkeeper.Result, error) {
				if operation == "mint" {
					return p.keeper.Mint(p.ctx, issuer, args.AssetID, account, sdk.NewInt(10), [32]byte{10})
				}
				return p.keeper.Burn(p.ctx, holder, args.AssetID, account, sdk.NewInt(10), [32]byte{10})
			}
			result, err := execute()
			require.ErrorIs(t, err, types.ErrFatalSupplyMismatch)
			require.Zero(t, result.Receipt.ResultCode, "the kernel accepted; the real bank refused its mirror")
			require.Len(t, result.Receipt.Transfers, 1)
			for key, before := range snapshots {
				require.Equal(t, before, ledgerViewStore(t, p.ctx, key), "failed %s must roll back %s", operation, key.Name())
			}
			require.Len(t, p.ctx.EventManager().Events(), eventCount, "rolled-back transfers must not emit bank events")
			p.assertBalances(t, args.AssetID, account, 80)
			p.restart(t)
			for key, before := range snapshots {
				require.Equal(t, before, ledgerViewStore(t, p.ctx, key), "restart must preserve rollback of %s", key.Name())
			}
			p.assertBalances(t, args.AssetID, account, 80)
			p.ctx = p.ctx.WithBlockTime(p.ctx.BlockTime().Add(2 * time.Hour))
			require.Empty(t, p.bank.LockedCoins(p.ctx, address))
			result, err = execute()
			require.NoError(t, err, "retry after vesting must use the unconsumed kernel sequence")
			require.Zero(t, result.Receipt.ResultCode)
			require.NotEmpty(t, p.ctx.EventManager().Events(), "successful mirror publishes its bank events")
			want := int64(70)
			if operation == "mint" {
				want = 90
			}
			p.assertBalances(t, args.AssetID, account, want)
		})
	}
}
