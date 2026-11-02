// Proves: evmkeeper.Keeper.BindLayerX -> types.EVMKeeper.GetLayerXDid -> keeper.Keeper.Actor -> TestProofDidBinding
//
// The consumer uses the exported auth and ledger Actor methods over the real
// EVM keeper and a committed, closed and reopened LevelDB multistore. No binding
// records or authority results are injected. A repeated bind is refused without
// changing the recovered identity or nonce; unbinding survives another restart
// and removes both consumers' authority while keeping old signatures unusable.
package keeper_test

import (
	"crypto/ed25519"
	"fmt"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	keeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
)

type proofDidBindingComponent struct {
	stores storetypes.CommitMultiStore
	ctx    sdk.Context
	evm    *evmkeeper.Keeper
	auth   *keeper.Keeper
	ledger *ledgerkeeper.Keeper
	closed bool
}

func (component *proofDidBindingComponent) close(t *testing.T) {
	t.Helper()
	if !component.closed {
		require.NoError(t, component.stores.Close())
		component.closed = true
	}
}

func openProofDidBinding(t *testing.T, directory string) *proofDidBindingComponent {
	t.Helper()
	app := testkeeper.EVMTestApp
	database, err := tmdb.NewGoLevelDB("did-binding", directory)
	require.NoError(t, err)
	stores := store.NewCommitMultiStore(database)
	component := &proofDidBindingComponent{stores: stores}
	t.Cleanup(func() { component.close(t) })
	// Keep the production keeper's store keys and dependencies; only its store
	// lifecycle belongs to this proof, independent of the shared test app.
	for _, name := range []string{evmtypes.StoreKey, types.StoreKey, ledgertypes.StoreKey} {
		stores.MountStoreWithDB(app.GetKey(name), sdk.StoreTypeIAVL, database)
	}
	require.NoError(t, stores.LoadLatestVersion())
	ctx := sdk.NewContext(stores, tmproto.Header{
		ChainID: "hyperpax_125-1", Height: stores.LastCommitID().Version + 1,
		Time: time.Unix(1_800_000_000, 0).UTC(),
	}, false)
	evm := app.EvmKeeper
	authority := authtypes.NewModuleAddress(govtypes.ModuleName).String()
	auth := keeper.NewKeeper(app.AppCodec(), app.GetKey(types.StoreKey), app.LayerXHost,
		&app.AuthzKeeper, &evm, authority)
	ledger := ledgerkeeper.NewKeeper(app.AppCodec(), app.GetKey(ledgertypes.StoreKey), app.LayerXHost,
		app.BankKeeper, &evm, authority)
	component.ctx, component.evm, component.auth, component.ledger = ctx, &evm, auth, ledger
	return component
}

func TestProofDidBinding(t *testing.T) {
	directory := t.TempDir()
	caller := common.HexToAddress("0x2110401000000000000000000000000000000001")
	stranger := common.HexToAddress("0x2110401000000000000000000000000000000002")
	var seed [ed25519.SeedSize]byte
	seed[0] = 104
	private := ed25519.NewKeyFromSeed(seed[:])
	var key [ed25519.PublicKeySize]byte
	copy(key[:], private.Public().(ed25519.PublicKey))
	first := openProofDidBinding(t, directory)
	message, err := evmtypes.LayerXBindMessage(first.evm.ChainID(first.ctx), caller, 0)
	require.NoError(t, err)
	var signature [ed25519.SignatureSize]byte
	copy(signature[:], ed25519.Sign(private, message))
	nonce, err := first.evm.BindLayerX(first.ctx, caller, key, signature)
	require.NoError(t, err)
	require.Zero(t, nonce)
	produced, err := first.auth.Actor(first.ctx, caller)
	require.NoError(t, err)
	did := []byte(fmt.Sprintf("did:layerx:%x", key))
	id, err := lxphost.DIDID(did)
	require.NoError(t, err)
	require.NotEqual(t, [32]byte{}, id)
	expected := keeper.Actor{DIDKey: key, DID: did, Authority: lxphost.KernelAuthority{
		Bound: true, Actor: id, Principal: id, VerifiedKey: key,
	}}
	require.Equal(t, expected, produced)
	commit := first.stores.Commit(true)
	require.Equal(t, int64(1), commit.Version)
	first.close(t)
	first = nil

	recovered := openProofDidBinding(t, directory)
	require.Equal(t, commit, recovered.stores.LastCommitID())
	assertBound := func() {
		t.Helper()
		actor, err := recovered.auth.Actor(recovered.ctx, caller)
		require.NoError(t, err)
		require.Equal(t, produced, actor)
		ledgerActor, err := recovered.ledger.Actor(recovered.ctx, caller)
		require.NoError(t, err)
		require.Equal(t, expected.DIDKey, ledgerActor.DIDKey)
		require.Equal(t, expected.DID, ledgerActor.DID)
		require.Equal(t, expected.Authority, ledgerActor.Authority)
		address, bound := recovered.evm.GetEVMAddressByLayerXDid(recovered.ctx, key)
		require.True(t, bound)
		require.Equal(t, caller, address)
		require.Equal(t, uint64(1), recovered.evm.GetLayerXBindNonce(recovered.ctx, caller))
		require.NoError(t, recovered.evm.ValidateLayerXBindings(recovered.ctx))
	}
	assertBound()
	_, err = recovered.evm.BindLayerX(recovered.ctx, caller, key, signature)
	require.ErrorIs(t, err, evmtypes.ErrLayerXAddressBound)
	assertBound()
	actor, err := recovered.auth.Actor(recovered.ctx, stranger)
	require.ErrorIs(t, err, types.ErrAuthScope)
	require.Equal(t, keeper.Actor{}, actor)
	ledgerActor, err := recovered.ledger.Actor(recovered.ctx, stranger)
	require.ErrorIs(t, err, ledgertypes.ErrUnauthorizedDebit)
	require.Equal(t, ledgerkeeper.Actor{}, ledgerActor)

	unboundKey, nonce, err := recovered.evm.UnbindLayerX(recovered.ctx, caller)
	require.NoError(t, err)
	require.Equal(t, key, unboundKey)
	require.Equal(t, uint64(1), nonce)
	actor, err = recovered.auth.Actor(recovered.ctx, caller)
	require.ErrorIs(t, err, types.ErrAuthScope)
	require.Equal(t, keeper.Actor{}, actor)
	commit = recovered.stores.Commit(true)
	require.Equal(t, int64(2), commit.Version)
	recovered.close(t)
	recovered = nil

	retired := openProofDidBinding(t, directory)
	require.Equal(t, commit, retired.stores.LastCommitID())
	actor, err = retired.auth.Actor(retired.ctx, caller)
	require.ErrorIs(t, err, types.ErrAuthScope)
	require.Equal(t, keeper.Actor{}, actor)
	ledgerActor, err = retired.ledger.Actor(retired.ctx, caller)
	require.ErrorIs(t, err, ledgertypes.ErrUnauthorizedDebit)
	require.Equal(t, ledgerkeeper.Actor{}, ledgerActor)
	_, bound := retired.evm.GetEVMAddressByLayerXDid(retired.ctx, key)
	require.False(t, bound)
	require.Equal(t, uint64(2), retired.evm.GetLayerXBindNonce(retired.ctx, caller))
	_, err = retired.evm.BindLayerX(retired.ctx, caller, key, signature)
	require.ErrorIs(t, err, evmtypes.ErrLayerXSignature)
	actor, err = retired.auth.Actor(retired.ctx, caller)
	require.ErrorIs(t, err, types.ErrAuthScope)
	require.Equal(t, keeper.Actor{}, actor)
	require.Equal(t, uint64(2), retired.evm.GetLayerXBindNonce(retired.ctx, caller))
	require.NoError(t, retired.evm.ValidateLayerXBindings(retired.ctx))
}
