package layerxledger_test

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"math"
	"math/big"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmcfg "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/addr"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

type repair144Fixture struct {
	p                 *ledgerPath
	dir               string
	sender, recipient common.Address
	payer, payee      ledgerkeeper.Actor
	asset, from, to   [32]byte
}

func (f *repair144Fixture) open(t *testing.T, restart bool) {
	t.Helper()
	db, err := sdk.NewLevelDB("repair-144", f.dir)
	require.NoError(t, err)
	encoding := app.MakeEncodingConfig()
	a := app.New(db, nil, true, map[int64]bool{}, f.dir, 1, true,
		tmcfg.DefaultConfig().SetRoot(f.dir), encoding, nil, app.TestAppOpts{}, app.EmptyWasmOpts, nil)
	if !restart {
		genesis, err := json.Marshal(app.NewDefaultGenesisState(encoding.Marshaler))
		require.NoError(t, err)
		_, err = a.InitChain(context.Background(), &abci.RequestInitChain{
			Time: time.Unix(1_800_000_000, 0).UTC(), ChainId: "pax-test",
			ConsensusParams: app.DefaultConsensusParams, AppStateBytes: genesis,
		})
		require.NoError(t, err)
	}
	header := tmproto.Header{ChainID: "pax-test", Height: 1, Time: time.Unix(1_800_000_000, 0).UTC()}
	var ctx sdk.Context
	if restart {
		header.Height = a.LastBlockHeight() + 1
		ctx, _ = a.NewUncachedContext(false, header).CacheContext()
	} else {
		ctx = a.NewContext(false, header)
		a.UpgradeKeeper.SetDone(ctx, app.MergeUpgrade)
	}
	a.RefreshPrecompiles(ctx)
	f.p = &ledgerPath{app: a, ctx: ctx, abi: layerxledger.GetABI()}
}

func (f *repair144Fixture) reopen(t *testing.T) {
	t.Helper()
	require.NoError(t, f.p.app.SetSealedStateToCommit(f.p.ctx, f.p.ctx.MultiStore().(sdk.CacheMultiStore)))
	_, err := f.p.app.Commit(context.Background())
	require.NoError(t, err)
	require.NoError(t, f.p.app.Close())
	f.p = nil
	f.open(t, true)
}

func (f *repair144Fixture) bind(t *testing.T, caller common.Address, tag byte) ledgerkeeper.Actor {
	t.Helper()
	var seed [32]byte
	seed[0] = tag
	private := ed25519.NewKeyFromSeed(seed[:])
	var public [32]byte
	copy(public[:], private.Public().(ed25519.PublicKey))
	message, err := evmtypes.LayerXBindMessage(f.p.app.EvmKeeper.ChainID(f.p.ctx), caller, 0)
	require.NoError(t, err)
	precompile, err := addr.NewPrecompile(f.p.app.GetPrecompileKeepers())
	require.NoError(t, err)
	input, err := precompile.GetABI().Pack(addr.BindLayerXMethod, public, ed25519.Sign(private, message))
	require.NoError(t, err)
	db := state.NewDBImpl(f.p.ctx.WithEventManager(sdk.NewEventManager()), &f.p.app.EvmKeeper, false)
	block, err := f.p.app.EvmKeeper.GetVMBlockContext(db.Ctx(), core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	config := evmtypes.DefaultChainConfig().EthereumConfig(f.p.app.EvmKeeper.ChainID(f.p.ctx))
	evm := vm.NewEVM(*block, db, config, vm.Config{}, f.p.app.EvmKeeper.CustomPrecompiles(db.Ctx()))
	evm.TxContext.Origin = caller
	data, _, err := evm.Call(caller, common.HexToAddress(addr.AddrAddress), input, 2_000_000, uint256.NewInt(0))
	require.NoError(t, err, "bindLayerX: %x", data)
	_, err = db.Finalize()
	require.NoError(t, err)
	actor, err := f.p.app.LayerXLedgerKeeper.Actor(f.p.ctx, caller)
	require.NoError(t, err)
	require.Equal(t, public, actor.DIDKey)
	require.True(t, actor.Authority.Bound)
	return actor
}

func newRepair144Fixture(t *testing.T) *repair144Fixture {
	t.Helper()
	f := &repair144Fixture{dir: t.TempDir(), sender: common.HexToAddress("0x2320000000000000000000000000000000000001"),
		recipient: common.HexToAddress("0x2320000000000000000000000000000000000002")}
	f.open(t, false)
	t.Cleanup(func() {
		if f.p != nil {
			require.NoError(t, f.p.app.Close())
		}
	})
	f.payer = f.bind(t, f.sender, 1)
	f.payee = f.bind(t, f.recipient, 2)
	salt := [32]byte{0x44}
	material := append(append([]byte("LX:ASSET:v1"), f.payer.Authority.Actor[:]...), salt[:]...)
	f.asset = sha256.Sum256(material)
	values, _ := f.p.success(t, f.sender, "register", f.asset, salt, "RPR", "Repair ledger", uint8(6), big.NewInt(1000), uint8(ledgertypes.AssetIssuerNative), []byte{})
	require.Equal(t, []interface{}{f.asset}, values)
	values, _ = f.p.success(t, f.sender, "openAccount", f.asset)
	require.Len(t, values, 1)
	f.from = values[0].([32]byte)
	values, _ = f.p.success(t, f.recipient, "openAccount", f.asset)
	require.Len(t, values, 1)
	f.to = values[0].([32]byte)
	require.Equal(t, f.from, accountId(t, f.p, f.sender, f.payer.DIDKey, f.asset))
	require.Equal(t, f.to, accountId(t, f.p, f.recipient, f.payee.DIDKey, f.asset))
	return f
}

func (f *repair144Fixture) sequence(t *testing.T, id [32]byte, want uint64) {
	t.Helper()
	values, logs := f.p.success(t, f.sender, "nextSequence", id, f.asset)
	require.Empty(t, logs)
	require.Equal(t, []interface{}{want}, values)
	actual, err := f.p.app.LayerXLedgerKeeper.NextSequenceByAccountID(f.p.ctx, id, f.asset)
	require.NoError(t, err)
	require.Equal(t, want, actual)
}

func (f *repair144Fixture) balances(t *testing.T, from, to int64) {
	t.Helper()
	for id, want := range map[[32]byte]int64{f.from: from, f.to: to} {
		values, _ := f.p.success(t, f.sender, "balanceOf", id, f.asset)
		require.Len(t, values, 1)
		require.Zero(t, values[0].(*big.Int).Cmp(big.NewInt(want)))
		balance, err := f.p.app.LayerXLedgerKeeper.BalanceOf(f.p.ctx, id, f.asset)
		require.NoError(t, err)
		require.True(t, balance.Equal(sdk.NewInt(want)))
		bank := f.p.app.BankKeeper.GetBalance(f.p.ctx, lxphost.BankAddress(id), ledgertypes.MirrorDenom(f.asset))
		require.True(t, balance.Equal(bank.Amount))
	}
}

func (f *repair144Fixture) refused(t *testing.T, caller common.Address, reason, method string, args ...interface{}) {
	t.Helper()
	before := storeSnapshot(t, f.p.ctx.KVStore(f.p.app.GetKey(app.LayerXHostStoreKey)))
	data, logs, err := f.p.call(t, caller, method, args...)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	assertLedgerRevert(t, data, reason)
	require.Empty(t, logs)
	require.Equal(t, before, storeSnapshot(t, f.p.ctx.KVStore(f.p.app.GetKey(app.LayerXHostStoreKey))))
}

func (f *repair144Fixture) envelope(t *testing.T, sequence uint64, amount int64, reference byte) []byte {
	t.Helper()
	key := [32]byte{reference}
	material := append(append(append([]byte(nil), f.from[:]...), f.to[:]...), f.asset[:]...)
	var units [16]byte
	big.NewInt(amount).FillBytes(units[:])
	material = append(material, units[:]...)
	contextHash, err := codec.DomainHash(codec.DomainContextHash, append(material, key[:]...))
	require.NoError(t, err)
	payload, err := ledgertypes.EncodeSendPayload(ledgertypes.SendArgs{
		From: f.from, To: f.to, Asset: f.asset, Amount: sdk.NewInt(amount), Sequence: sequence,
		IdempotencyKey: key, ExpiresAt: uint64(f.p.ctx.BlockTime().UnixMilli()) + 60_000,
		ContextHash: contextHash, PublicKey: f.payer.DIDKey, NetworkID: uint32(f.p.app.EvmKeeper.ChainID(f.p.ctx).Uint64()), ProtocolVersion: codec.StateCommitmentProtocolVersion,
	})
	require.NoError(t, err)
	require.NoError(t, ledgertypes.ValidateSendEnvelope(payload))
	return payload
}

func TestRepair_144_OpenAccountNextSequence(t *testing.T) {
	f := newRepair144Fixture(t)
	f.sequence(t, f.from, 0)
	f.sequence(t, f.to, 0)
	f.balances(t, 0, 0)
	// A public key is not an account ID, and an account cannot be queried for another asset.
	f.refused(t, f.sender, "LXP_UNKNOWN_ACCOUNT_NAMESPACE", "nextSequence", f.payer.DIDKey, f.asset)
	f.refused(t, f.sender, "LXP_ASSET_MISMATCH", "nextSequence", f.from, [32]byte{0xff})
	f.reopen(t)
	f.sequence(t, f.from, 0)
	f.sequence(t, f.to, 0)
	f.balances(t, 0, 0)
}

func TestRepair_144_PayExactExpiresAtMs(t *testing.T) {
	f := newRepair144Fixture(t)
	method := f.p.abi.Methods["payExact"]
	require.Equal(t, "expiresAtMs", method.Inputs[4].Name)
	require.Equal(t, "uint64", method.Inputs[4].Type.String())
	require.Equal(t, []byte{0x78, 0xea, 0x11, 0xdb}, method.ID)
	f.p.success(t, f.sender, "mint", f.asset, f.from, big.NewInt(250))
	now := uint64(f.p.ctx.BlockTime().UnixMilli())
	for i := uint64(0); i < 2; i++ {
		f.sequence(t, f.from, i)
		values, logs := f.p.success(t, f.sender, "payExact", f.from, f.to, f.asset, big.NewInt(7), now+60_000, [32]byte{0x71, byte(i)})
		require.Empty(t, values)
		require.Len(t, logs, 2)
		proofLedgerLog(t, f.p.abi, logs[1], "PaymentExact", map[string]interface{}{
			"paymentReference": [32]byte{0x71, byte(i)}, "from": f.from, "to": f.to, "assetId": f.asset, "amount": big.NewInt(7),
		})
		f.sequence(t, f.from, i+1)
	}
	f.balances(t, 236, 14)
	for _, expiry := range []uint64{0, now - 1, now} {
		f.refused(t, f.sender, "LXP_EXPIRED", "payExact", f.from, f.to, f.asset, big.NewInt(7), expiry, [32]byte{0x72})
		f.sequence(t, f.from, 2)
		f.balances(t, 236, 14)
	}
	f.reopen(t)
	f.sequence(t, f.from, 2)
	f.balances(t, 236, 14)
	f.p.success(t, f.sender, "payExact", f.from, f.to, f.asset, big.NewInt(7), now+60_000, [32]byte{0x73})
	f.sequence(t, f.from, 3)
	f.balances(t, 229, 21)
}

func TestRepair_144_AssetAccountSend(t *testing.T) {
	f := newRepair144Fixture(t)
	f.p.success(t, f.sender, "mint", f.asset, f.from, big.NewInt(250))
	f.sequence(t, f.from, 0)
	f.balances(t, 250, 0)
	payload := f.envelope(t, 0, 40, 0x81)
	f.refused(t, f.recipient, "LXP_UNAUTHORIZED_DEBIT", "send", payload)
	f.sequence(t, f.from, 0)
	f.balances(t, 250, 0)
	otherAsset := *f
	otherAsset.asset = [32]byte{0xff}
	f.refused(t, f.sender, "LXP_ASSET_MISMATCH", "send", otherAsset.envelope(t, 0, 40, 0x85))
	values, logs := f.p.success(t, f.sender, "send", payload)
	require.Empty(t, values)
	require.Len(t, logs, 1)
	proofLedgerLog(t, f.p.abi, logs[0], "Transfer", map[string]interface{}{
		"from": f.from, "to": f.to, "assetId": f.asset, "amount": big.NewInt(40), "kind": uint8(5),
	})
	f.sequence(t, f.from, 1)
	f.sequence(t, f.to, 0)
	f.balances(t, 210, 40)
	f.refused(t, f.sender, "LXP_SEQUENCE_MISMATCH", "send", f.envelope(t, 0, 1, 0x82))
	next, err := lxphost.NextSequence(f.p.ctx, f.p.app.GetKey(app.LayerXHostStoreKey))
	require.NoError(t, err)
	identity, err := f.p.app.LayerXHost.ReadIdentitySequence(f.p.ctx, f.payer.DID)
	require.NoError(t, err)
	f.reopen(t)
	f.sequence(t, f.from, 1)
	f.sequence(t, f.to, 0)
	f.balances(t, 210, 40)
	recovered, err := lxphost.NextSequence(f.p.ctx, f.p.app.GetKey(app.LayerXHostStoreKey))
	require.NoError(t, err)
	require.Equal(t, next, recovered)
	recoveredIdentity, err := f.p.app.LayerXHost.ReadIdentitySequence(f.p.ctx, f.payer.DID)
	require.NoError(t, err)
	require.Equal(t, identity, recoveredIdentity)
	f.refused(t, f.recipient, "LXP_UNAUTHORIZED_DEBIT", "send", f.envelope(t, 1, 10, 0x83))
	f.p.success(t, f.sender, "send", f.envelope(t, 1, 10, 0x84))
	f.sequence(t, f.from, 2)
	f.balances(t, 200, 50)
	_, issuance, err := lxphost.IssuanceAccountID(f.asset)
	require.NoError(t, err)
	balance, err := f.p.app.LayerXHost.ReadBalance(f.p.ctx, issuance, f.asset)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(750), balance)
	require.Equal(t, balance, f.p.app.BankKeeper.GetBalance(f.p.ctx, lxphost.BankAddress(issuance), ledgertypes.MirrorDenom(f.asset)).Amount)
	require.Equal(t, sdk.NewInt(1000), f.p.app.BankKeeper.GetSupply(f.p.ctx, ledgertypes.MirrorDenom(f.asset)).Amount)
}
