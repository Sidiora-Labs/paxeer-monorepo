package layerxauth_test

import (
	"bytes"
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxauth"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestGovernancePauseRefusesAuthGrants(t *testing.T) {
	a := app.Setup(t, false, true, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	ctx := a.GetContextForDeliverTx(nil).WithChainID("hyperpax_125-1").WithBlockHeight(100).
		WithBlockTime(time.Unix(1_800_000_000, 0).UTC())
	a.UpgradeKeeper.SetDone(ctx, app.MergeUpgrade)
	a.RefreshPrecompiles(ctx)
	address := common.HexToAddress(layerxauth.AuthAddress)
	contract, ok := a.EvmKeeper.CustomPrecompiles(ctx)[address]
	require.True(t, ok, "node must serve 0x101C after the merge plan")
	served, ok := contract.(precompiles.IPrecompile)
	require.True(t, ok, "node-served contract must expose its published ABI")
	require.Equal(t, layerxauth.PrecompileName, served.GetName())
	provider, ok := a.GetPrecompileKeepers().(layerxauth.AuthKeepers)
	require.True(t, ok, "node must supply the layerxauth keeper to its served precompile")
	k := provider.LayerXAuthK()
	require.NotNil(t, k)
	require.False(t, k.GetParams(ctx).Paused)
	caller := common.HexToAddress("0x2111010000000000000000000000000000000001")
	private := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	var public [32]byte
	copy(public[:], private.Public().(ed25519.PublicKey))
	message, err := evmtypes.LayerXBindMessage(a.EvmKeeper.ChainID(ctx), caller, 0)
	require.NoError(t, err)
	var signature [64]byte
	copy(signature[:], ed25519.Sign(private, message))
	_, err = a.EvmKeeper.BindLayerX(ctx, caller, public, signature)
	require.NoError(t, err)
	actor, err := k.Actor(ctx, caller)
	require.NoError(t, err)
	hostKey := a.GetKey(app.LayerXHostStoreKey)
	require.NotNil(t, hostKey)
	host := a.LayerXHost
	require.NotNil(t, host)
	timestamp := uint64(ctx.BlockTime().UnixMilli())
	registration := append(append([]byte{0x71, 1, 0, 2}, actor.Authority.Actor[:]...), public[:]...)
	activity := lxphost.Activity{ProtocolVersion: 3, Type: 0x00070001, ActorDID: actor.DID,
		AuthorityKey: public, NotBefore: timestamp, NotAfter: timestamp,
		IdempotencyKey: [32]byte{0x41}, Payload: registration}
	digest, err := host.SigningHash(activity)
	require.NoError(t, err)
	activity.Signature = ed25519.Sign(private, digest[:])
	receipt, err := host.Execute(lxphost.WithSequence(ctx, 41), activity, actor.Authority)
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), receipt.ResultCode)
	db := state.NewDBImpl(ctx.WithEventManager(sdk.NewEventManager()), &a.EvmKeeper, true)
	ctx = db.Ctx()
	evm := &vm.EVM{StateDB: db, TxContext: vm.TxContext{Origin: caller}}
	abi := served.GetABI()
	call := func(name string, args ...interface{}) ([]byte, error) {
		t.Helper()
		input, err := abi.Pack(name, args...)
		require.NoError(t, err)
		return served.Run(evm, caller, address, input, nil, abi.Methods[name].IsConstant(), false, nil)
	}
	var delegate [32]byte
	copy(delegate[:], ed25519.NewKeyFromSeed(bytes.Repeat([]byte{72}, ed25519.SeedSize)).Public().(ed25519.PublicKey))
	body := authRevokeGrantBody(actor.Authority.Actor, delegate, timestamp, 2)
	data, err := call("grantSession", body, uint64(100), [32]byte{0x42})
	require.NoError(t, err)
	values, err := abi.Methods["grantSession"].Outputs.Unpack(data)
	require.NoError(t, err)
	require.Len(t, values, 1)
	grantID := values[0].([32]byte)
	live, reason, err := k.IsLive(ctx, grantID, 42, timestamp)
	require.NoError(t, err)
	require.True(t, live)
	require.Empty(t, reason)

	params := k.GetParams(ctx)
	params.Paused = true
	msg := &types.MsgUpdateParams{Authority: govtypes.GovernanceAuthority(), Params: params}
	require.Equal(t, []sdk.AccAddress{sdk.MustAccAddressFromBech32(govtypes.GovernanceAuthority())}, msg.GetSigners())
	proposal, err := govtypes.NewLayerXProposal("Pause auth grants", "Retain grant revocation", msg)
	require.NoError(t, err)
	stored, err := a.GovKeeper.SubmitProposal(ctx, proposal)
	require.NoError(t, err)
	onChain, found := a.GovKeeper.GetProposal(ctx, stored.ProposalId)
	require.True(t, found)
	require.NoError(t, a.GovKeeper.Router().GetRoute(govtypes.RouterKey)(ctx, onChain.GetContent()))
	require.True(t, k.GetParams(ctx).Paused)
	var nextDelegate [32]byte
	copy(nextDelegate[:], ed25519.NewKeyFromSeed(bytes.Repeat([]byte{73}, ed25519.SeedSize)).Public().(ed25519.PublicKey))
	nextBody := authRevokeGrantBody(actor.Authority.Actor, nextDelegate, timestamp, 2)
	before := hostSnapshot(t, ctx, hostKey)
	logCount := len(db.GetAllLogs())
	snapshot := db.Snapshot()
	data, err = call("grantSession", nextBody, uint64(100), [32]byte{0x43})
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	assertLxpRevert(t, data, "LXP_PAUSED_SCOPE")
	require.ErrorIs(t, db.GetPrecompileError(), govtypes.ErrPausedScope)
	db.RevertToSnapshot(snapshot)
	require.Equal(t, before, hostSnapshot(t, ctx, hostKey))
	require.Len(t, db.GetAllLogs(), logCount)
	sequence, err := lxphost.NextSequence(ctx, hostKey)
	require.NoError(t, err)
	data, err = call("revokeGrant", grantID, uint8(1), sequence)
	require.NoError(t, err)
	require.Empty(t, data)
	live, reason, err = k.IsLive(ctx, grantID, sequence, timestamp)
	require.NoError(t, err)
	require.False(t, live)
	require.Equal(t, "LXP_AUTH_REVOKED", reason)
	revoked, err := host.ReadGrant(ctx, grantID)
	require.NoError(t, err)
	require.True(t, revoked.Revoked)
	require.Equal(t, sequence, revoked.RevokedAtSequence)
	require.Len(t, db.GetAllLogs(), logCount+2)
	require.Equal(t, abi.Events["GrantRevoked"].ID, db.GetAllLogs()[logCount].Topics[0])
	require.True(t, k.GetParams(ctx).Paused)
}
