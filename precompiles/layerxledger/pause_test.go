package layerxledger_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestGovernancePauseRefusesLedgerWrites(t *testing.T) {
	a := app.Setup(t, false, true, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	ctx := a.GetContextForDeliverTx(nil).WithChainID("hyperpax_125-1").WithBlockHeight(100).
		WithBlockTime(time.Unix(1_800_000_000, 0).UTC())
	a.UpgradeKeeper.SetDone(ctx, app.MergeUpgrade)
	a.RefreshPrecompiles(ctx)
	address := common.HexToAddress(layerxledger.LedgerAddress)
	contract, ok := a.EvmKeeper.CustomPrecompiles(ctx)[address]
	require.True(t, ok, "node must serve 0x101A after the merge plan")
	served, ok := contract.(precompiles.IPrecompile)
	require.True(t, ok, "node-served contract must expose its published ABI")
	require.Equal(t, layerxledger.PrecompileName, served.GetName())
	provider, ok := a.GetPrecompileKeepers().(layerxledger.LedgerKeepers)
	require.True(t, ok, "node must supply the layerxledger keeper to its served precompile")
	k := provider.LayerXLedgerK()
	require.NotNil(t, k)
	require.False(t, k.GetParams(ctx).Paused)

	caller := common.HexToAddress("0x2320000000000000000000000000000000000001")
	var seed [ed25519.SeedSize]byte
	seed[0] = 1
	private := ed25519.NewKeyFromSeed(seed[:])
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
	salt := [32]byte{0x41}
	material := append([]byte("LX:ASSET:v1"), actor.Authority.Actor[:]...)
	asset := sha256.Sum256(append(material, salt[:]...))
	_, err = call("register", asset, salt, "PAUSE", "Governance pause", uint8(6), big.NewInt(1000), uint8(types.AssetIssuerNative), []byte{})
	require.NoError(t, err)
	_, err = call("openAccount", asset)
	require.NoError(t, err)
	account, err := ledgerkeeper.AccountID(public, asset)
	require.NoError(t, err)
	_, err = call("mint", asset, account, big.NewInt(250))
	require.NoError(t, err)

	grantBody := append(append([]byte(nil), account[:]...), account[:]...)
	grantBody = append(grantBody, asset[:]...)
	for _, n := range []uint64{0, 40, 0, 100} {
		grantBody = binary.BigEndian.AppendUint64(grantBody, n)
	}
	grantBody = append(grantBody, 0)
	grantBody = binary.BigEndian.AppendUint64(grantBody, 0)
	grantBody = binary.BigEndian.AppendUint64(grantBody, uint64(ctx.BlockTime().UnixMilli())+1000)
	purpose := [32]byte{0x44}
	grantBody = append(grantBody, purpose[:]...)
	grantBody = append(grantBody, 0)
	grantBody = append(grantBody, make([]byte, 32)...)
	grantBody = binary.BigEndian.AppendUint64(grantBody, 0)
	grantBody = append(grantBody, public[:]...)
	grantID, err := codec.DomainHash(codec.DomainAuthorityHash, append([]byte("LXP:GRANT:v1"), grantBody...))
	require.NoError(t, err)
	grant := append(append([]byte(nil), grantID[:]...), grantBody...)
	grant = append(grant, ed25519.Sign(private, grantID[:])...)
	require.NoError(t, types.ValidateIssueGrantPayload(grant))
	data, err := call("issueGrant", grant)
	require.NoError(t, err, "precompile error: %v; revert data: %x", db.GetPrecompileError(), data)
	require.Equal(t, grantID[:], data)

	_, main, err := lxphost.MainAccountID(public)
	require.NoError(t, err)
	key := [32]byte{0x42}
	contextHash, err := codec.DomainHash(codec.DomainContextHash, key[:])
	require.NoError(t, err)
	envelope, err := types.EncodeSendPayload(types.SendArgs{
		From: main, To: account, Asset: asset, Amount: sdk.NewInt(1), Sequence: 0,
		IdempotencyKey: key, ExpiresAt: uint64(ctx.BlockTime().UnixMilli()) + 1000,
		ContextHash: contextHash, PublicKey: public, NetworkID: 125, ProtocolVersion: 3,
	})
	require.NoError(t, err)
	authorization := len(envelope) - 167
	preimage := append(append([]byte(nil), envelope[:2]...), envelope[4:authorization]...)
	preimage = append(preimage, envelope[authorization:authorization+33]...)
	preimage = append(preimage, envelope[authorization+129:]...)
	digest, err := codec.DomainHash(codec.DomainSignaturePreimage, preimage)
	require.NoError(t, err)
	copy(envelope[authorization+65:authorization+129], ed25519.Sign(private, digest[:]))
	require.NoError(t, types.ValidateSendEnvelope(envelope))

	msg := &types.MsgUpdateParams{Authority: govtypes.GovernanceAuthority(), Params: types.Params{Paused: true}}
	require.Equal(t, []sdk.AccAddress{sdk.MustAccAddressFromBech32(govtypes.GovernanceAuthority())}, msg.GetSigners())
	proposal, err := govtypes.NewLayerXProposal("Pause ledger writes", "Retain exits and balance queries", msg)
	require.NoError(t, err)
	stored, err := a.GovKeeper.SubmitProposal(ctx, proposal)
	require.NoError(t, err)
	onChain, found := a.GovKeeper.GetProposal(ctx, stored.ProposalId)
	require.True(t, found)
	require.NoError(t, a.GovKeeper.Router().GetRoute(govtypes.RouterKey)(ctx, onChain.GetContent()))
	require.True(t, k.GetParams(ctx).Paused)
	hostKey := a.GetKey(app.LayerXHostStoreKey)
	require.NotNil(t, hostKey)
	before := storeSnapshot(t, ctx.KVStore(hostKey))
	logCount := len(db.GetAllLogs())
	snapshot := db.Snapshot()
	data, err = call("send", envelope)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	assertLedgerRevert(t, data, "LXP_PAUSED_SCOPE")
	require.ErrorIs(t, db.GetPrecompileError(), govtypes.ErrPausedScope)
	db.RevertToSnapshot(snapshot)
	require.Equal(t, before, storeSnapshot(t, ctx.KVStore(hostKey)))
	require.Len(t, db.GetAllLogs(), logCount)

	data, err = call("balanceOf", account, asset)
	require.NoError(t, err)
	values, err := abi.Methods["balanceOf"].Outputs.Unpack(data)
	require.NoError(t, err)
	require.Equal(t, []interface{}{big.NewInt(250)}, values)
	sequence, err := a.LayerXHost.ReadIdentitySequence(ctx, actor.DID)
	require.NoError(t, err)
	data, err = call("revokeGrant", grantID, sequence)
	require.NoError(t, err, "precompile error: %v; revert data: %x", db.GetPrecompileError(), data)
	require.Empty(t, data)
	revoked, err := k.GetPayerGrant(ctx, grantID)
	require.NoError(t, err)
	require.True(t, revoked.Revoked)
	require.Len(t, db.GetAllLogs(), logCount+1)
	require.Equal(t, abi.Events["PayerGrantRevoked"].ID, db.GetAllLogs()[logCount].Topics[0])
	data, err = call("balanceOf", account, asset)
	require.NoError(t, err)
	values, err = abi.Methods["balanceOf"].Outputs.Unpack(data)
	require.NoError(t, err)
	require.Equal(t, []interface{}{big.NewInt(250)}, values)
}
