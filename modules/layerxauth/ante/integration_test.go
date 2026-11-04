package ante_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/apptesting"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxauth"
	clienttx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/utils/tracing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/ante"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	wasmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
)

type integrationAuthKeepers struct {
	*app.PrecompileKeepers
	auth *authkeeper.Keeper
}

func (k integrationAuthKeepers) LayerXAuthK() *authkeeper.Keeper { return k.auth }

func TestAuthzExecRefusedAfterKernelRevoke(t *testing.T) {
	testAuthzExecProducerGrant(t, false)
}

func testAuthzExecProducerGrant(t *testing.T, budgetProbes bool) {
	t.Helper()
	application := app.Setup(t, false, true, false)
	t.Cleanup(func() { require.NoError(t, application.Close()) })
	ctx, _ := application.NewContext(false, tmproto.Header{
		ChainID: "hyperpax_125-1", Height: 11, Time: time.Unix(1_800_000_000, 0).UTC(),
	}).CacheContext()

	grantorPrivate := secp256k1.GenPrivKeyFromSecret([]byte("authz kernel revoke grantor"))
	granteePrivate := secp256k1.GenPrivKeyFromSecret([]byte("authz kernel revoke grantee"))
	grantor := sdk.AccAddress(grantorPrivate.PubKey().Address())
	grantee := sdk.AccAddress(granteePrivate.PubKey().Address())
	recipient := sdk.AccAddress(bytes.Repeat([]byte{0x63}, 20))
	asset := [32]byte{0x11}
	denom := ledgertypes.MirrorDenom(asset)
	for _, address := range []sdk.AccAddress{grantor, grantee} {
		require.NoError(t, apptesting.FundAccount(application.BankKeeper, ctx, address,
			sdk.NewCoins(sdk.NewInt64Coin(denom, 1_000_000), sdk.NewInt64Coin(sdk.DefaultBondDenom, 1_000_000))))
	}
	send := banktypes.NewMsgSend(grantor, recipient, sdk.NewCoins(sdk.NewInt64Coin(denom, 7)))
	purpose, err := types.BankSendPurpose(send)
	require.NoError(t, err)
	caller := common.BytesToAddress(grantor)
	delegate := common.BytesToAddress(grantee)
	application.EvmKeeper.SetAddressMapping(ctx, grantor, caller)
	application.EvmKeeper.SetAddressMapping(ctx, grantee, delegate)
	bind := func(address common.Address, seed byte) [32]byte {
		t.Helper()
		private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))
		var public [32]byte
		copy(public[:], private.Public().(ed25519.PublicKey))
		message, err := evmtypes.LayerXBindMessage(application.EvmKeeper.ChainID(ctx), address,
			application.EvmKeeper.GetLayerXBindNonce(ctx, address))
		require.NoError(t, err)
		var signature [64]byte
		copy(signature[:], ed25519.Sign(private, message))
		_, err = application.EvmKeeper.BindLayerX(ctx, address, public, signature)
		require.NoError(t, err)
		return public
	}
	public := bind(caller, 71)
	delegateKey := bind(delegate, 72)
	actor, err := application.LayerXAuthKeeper.Actor(ctx, caller)
	require.NoError(t, err)
	timestamp := uint64(ctx.BlockTime().UnixMilli())
	registration := append(append([]byte{0x71, 1, 0, 2}, actor.Authority.Actor[:]...), public[:]...)
	receipt, err := application.LayerXHost.Execute(lxphost.WithSequence(ctx, 41), lxphost.Activity{
		ProtocolVersion: 3, Type: 0x00070001, ActorDID: actor.DID, AuthorityKey: public,
		NotBefore: timestamp, NotAfter: timestamp, IdempotencyKey: [32]byte{0x41}, Payload: registration,
	}, actor.Authority)
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), receipt.ResultCode)

	// Encode the canonical capability body for the bound grantee at epoch 41.
	body := []byte{0, 1, 0x20, 1, 1}
	field := func(value []byte) {
		body = binary.BigEndian.AppendUint32(body, uint32(len(value)))
		body = append(body, value...)
	}
	u64 := func(value uint64) { body = binary.BigEndian.AppendUint64(body, value) }
	u128 := func(value uint64) { u64(0); u64(value) }
	field(actor.Authority.Actor[:])
	field(actor.Authority.Actor[:])
	body = append(body, 3)
	delegateKeyOffset := len(body) + 4
	field(delegateKey[:])
	u64(1 << types.BankScopeModule)
	body = binary.BigEndian.AppendUint16(body, 4)
	body = binary.BigEndian.AppendUint16(body, 5)
	field(asset[:])
	u128(10)
	u128(20)
	u128(0)
	u64(1000)
	maxPerPeriodOffset := len(body)
	u128(15)
	u128(0)
	u64(timestamp - 1000)
	purposeOffset := len(body) + 4
	field(purpose[:])
	u64(timestamp - 1000)
	u64(timestamp + 1000)
	revocationEpochOffset := len(body)
	u64(41)
	body = append(body, 0)
	u64(0)
	field(make([]byte, ed25519.SignatureSize))

	precompile, err := layerxauth.NewPrecompile(integrationAuthKeepers{
		PrecompileKeepers: application.GetPrecompileKeepers().(*app.PrecompileKeepers),
		auth:              application.LayerXAuthKeeper,
	})
	require.NoError(t, err)
	require.Equal(t, common.HexToAddress("0x101C"), precompile.Address())
	abi := precompile.GetABI()
	call := func(method string, args ...interface{}) []interface{} {
		t.Helper()
		input, err := abi.Pack(method, args...)
		require.NoError(t, err)
		db := state.NewDBImpl(ctx.WithEventManager(sdk.NewEventManager()), &application.EvmKeeper, false)
		evm := &vm.EVM{StateDB: db, TxContext: vm.TxContext{Origin: caller}}
		output, err := precompile.Run(evm, caller, precompile.Address(), input, nil, false, false, nil)
		require.NoError(t, err, "%s returned %x (precompile error: %v)", method, output, db.GetPrecompileError())
		require.NoError(t, db.GetPrecompileError())
		ctx = db.Ctx()
		values, err := abi.Unpack(method, output)
		require.NoError(t, err)
		return values
	}
	issuanceCtx := ctx
	ctx, _ = ctx.CacheContext()
	grantValues := call("grantCapability", body)
	require.Len(t, grantValues, 1)
	grantID := grantValues[0].([32]byte)
	grant, err := application.LayerXHost.ReadGrant(ctx, grantID)
	require.NoError(t, err)
	require.False(t, grant.Revoked)
	require.Equal(t, uint64(41), grant.GrantorRevocationSequence)
	live, reason, err := application.LayerXAuthKeeper.IsLive(ctx, grantID, 42, timestamp)
	require.NoError(t, err)
	require.True(t, live)
	require.Empty(t, reason)

	assertMirror := func(issued lxphost.AuthorityGrant, spent string, periodStart uint64, periodSpent string) {
		t.Helper()
		stored, expiry := application.AuthzKeeper.GetCleanAuthorization(ctx, grantee, grantor, sdk.MsgTypeURL(send))
		require.IsType(t, &types.LayerXScopeAuthorization{}, stored)
		require.NoError(t, stored.ValidateBasic())
		require.Equal(t, &types.LayerXScopeAuthorization{
			GrantId: issued.GrantID[:], MsgTypeUrl: sdk.MsgTypeURL(send), Kind: uint32(issued.Kind),
			ModuleMask: issued.Scope.ModuleMask, OrdinalMin: uint32(issued.Scope.ActivityOrdinalMin),
			OrdinalMax: uint32(issued.Scope.ActivityOrdinalMax), AssetId: issued.Scope.AssetID[:],
			PurposeHash: issued.Scope.PurposeHash[:], MaxPerActivity: issued.Scope.MaximumPerActivity.String(),
			MaxTotal: issued.Scope.MaximumTotal.String(), PeriodLength: issued.Scope.PeriodLength,
			MaxPerPeriod: issued.Scope.MaximumPerPeriod.String(), NotBefore: issued.NotBefore,
			NotAfter: issued.NotAfter, RevocationEpoch: issued.GrantorRevocationSequence,
			SpentTotal: spent, PeriodStart: periodStart, SpentThisPeriod: periodSpent,
		}, stored)
		require.Equal(t, time.UnixMilli(int64(issued.NotAfter)).UTC(), expiry)
	}
	require.Equal(t, uint64(1<<types.BankScopeModule), grant.Scope.ModuleMask)
	require.Equal(t, uint16(4), grant.Scope.ActivityOrdinalMin)
	require.Equal(t, uint16(5), grant.Scope.ActivityOrdinalMax)
	require.Equal(t, asset, grant.Scope.AssetID)
	require.Equal(t, purpose, grant.Scope.PurposeHash)
	require.Equal(t, "10", grant.Scope.MaximumPerActivity.String())
	require.Equal(t, "20", grant.Scope.MaximumTotal.String())
	require.Equal(t, uint64(1000), grant.Scope.PeriodLength)
	require.Equal(t, "15", grant.Scope.MaximumPerPeriod.String())
	require.Equal(t, timestamp-1000, grant.NotBefore)
	require.Equal(t, timestamp+1000, grant.NotAfter)
	assertMirror(grant, "0", 0, "")
	require.Len(t, application.AuthzKeeper.GetAuthorizations(ctx, grantee, grantor), 2)

	encoding := app.MakeEncodingConfig()
	wasmConfig := wasmtypes.DefaultWasmConfig()
	anteHandler, _, err := app.NewAnteHandler(app.HandlerOptions{
		HandlerOptions: ante.HandlerOptions{
			AccountKeeper: application.AccountKeeper, BankKeeper: application.BankKeeper,
			FeegrantKeeper: application.FeeGrantKeeper, ParamsKeeper: application.ParamsKeeper,
			SignModeHandler: encoding.TxConfig.SignModeHandler(),
		},
		IBCKeeper: application.IBCKeeper, WasmConfig: &wasmConfig, WasmKeeper: &application.WasmKeeper,
		OracleKeeper: &application.OracleKeeper, PricefeedKeeper: application.PricefeedKeeper,
		EVMKeeper: &application.EvmKeeper, LayerXAuthKeeper: application.LayerXAuthKeeper,
		UpgradeKeeper: &application.UpgradeKeeper, LatestCtxGetter: func() sdk.Context { return ctx },
		TracingInfo: tracing.NewTracingInfo(otel.Tracer("authz-kernel-revoke-test"), false),
	})
	require.NoError(t, err)
	deliver := func(message *banktypes.MsgSend) error {
		t.Helper()
		exec := authz.NewMsgExec(grantee, []sdk.Msg{message})
		builder := encoding.TxConfig.NewTxBuilder()
		require.NoError(t, builder.SetMsgs(&exec))
		builder.SetGasLimit(1_000_000)
		account := application.AccountKeeper.GetAccount(ctx, grantee)
		require.NotNil(t, account)
		mode := encoding.TxConfig.SignModeHandler().DefaultMode()
		require.NoError(t, builder.SetSignatures(signing.SignatureV2{
			PubKey: granteePrivate.PubKey(), Sequence: account.GetSequence(),
			Data: &signing.SingleSignatureData{SignMode: mode},
		}))
		signature, err := clienttx.SignWithPrivKey(mode, authsigning.SignerData{
			ChainID: ctx.ChainID(), AccountNumber: account.GetAccountNumber(), Sequence: account.GetSequence(),
		}, builder, granteePrivate, encoding.TxConfig, account.GetSequence())
		require.NoError(t, err)
		require.NoError(t, builder.SetSignatures(signature))
		txBytes, err := encoding.TxConfig.TxEncoder()(builder.GetTx())
		require.NoError(t, err)
		decoded, err := encoding.TxConfig.TxDecoder()(txBytes)
		require.NoError(t, err)
		branch, write := ctx.WithEventManager(sdk.NewEventManager()).CacheContext()
		anteCtx, err := anteHandler(branch.WithTxBytes(txBytes), decoded, false)
		require.NoError(t, err)
		messages := decoded.GetMsgs()
		require.Len(t, messages, 1)
		_, err = application.AuthzKeeper.Exec(sdk.WrapSDKContext(anteCtx), messages[0].(*authz.MsgExec))
		if err == nil {
			write()
		}
		return err
	}
	before := application.BankKeeper.GetBalance(ctx, recipient, denom).Amount
	grantorBefore := application.BankKeeper.GetBalance(ctx, grantor, denom).Amount
	require.NoError(t, deliver(send))
	require.Equal(t, before.AddRaw(7), application.BankKeeper.GetBalance(ctx, recipient, denom).Amount)
	require.Equal(t, grantorBefore.SubRaw(7), application.BankKeeper.GetBalance(ctx, grantor, denom).Amount)
	assertMirror(grant, "7", timestamp, "7")

	// Probes use the producer's stored authorization and discard their branch.
	parent := ctx
	ctx, _ = ctx.CacheContext()
	if !budgetProbes {
		changed := banktypes.NewMsgSend(grantor, grantee, send.Amount)
		err = deliver(changed)
		require.ErrorIs(t, err, types.ErrAuthScope)
		require.ErrorContains(t, err, "purpose differs")
	} else {
		for _, probe := range []struct {
			denom        string
			amount       int64
			periodLimit  uint64
			successCount int
			reason       string
		}{
			{ledgertypes.MirrorDenom([32]byte{0x12}), 7, 15, 0, "asset differs"},
			{denom, 11, 15, 0, "above max per activity"},
			{denom, 7, 10, 1, "period budget exceeded"},
			{denom, 7, 15, 2, "above max total"},
		} {
			ctx, _ = issuanceCtx.CacheContext()
			message := banktypes.NewMsgSend(grantor, recipient, sdk.NewCoins(sdk.NewInt64Coin(probe.denom, probe.amount)))
			probePurpose, err := types.BankSendPurpose(message)
			require.NoError(t, err)
			probeBody := bytes.Clone(body)
			copy(probeBody[purposeOffset:purposeOffset+32], probePurpose[:])
			binary.BigEndian.PutUint64(probeBody[maxPerPeriodOffset+8:maxPerPeriodOffset+16], probe.periodLimit)
			values := call("grantCapability", probeBody)
			require.Len(t, values, 1)
			issued, err := application.LayerXHost.ReadGrant(ctx, values[0].([32]byte))
			require.NoError(t, err)
			require.Equal(t, probePurpose, issued.Scope.PurposeHash)
			require.Equal(t, sdk.NewIntFromUint64(probe.periodLimit), issued.Scope.MaximumPerPeriod)
			assertMirror(issued, "0", 0, "")
			spent, start, periodSpent := sdk.ZeroInt(), uint64(0), ""
			for i := 0; i < probe.successCount; i++ {
				require.NoError(t, deliver(message))
				spent = spent.AddRaw(probe.amount)
				start, periodSpent = timestamp, spent.String()
				assertMirror(issued, spent.String(), start, periodSpent)
			}
			recipientBefore := application.BankKeeper.GetBalance(ctx, recipient, probe.denom).Amount
			senderBefore := application.BankKeeper.GetBalance(ctx, grantor, probe.denom).Amount
			require.Equal(t, spent, recipientBefore)
			err = deliver(message)
			require.ErrorIs(t, err, types.ErrAuthScope)
			require.ErrorContains(t, err, probe.reason)
			assertMirror(issued, spent.String(), start, periodSpent)
			require.Equal(t, recipientBefore, application.BankKeeper.GetBalance(ctx, recipient, probe.denom).Amount)
			require.Equal(t, senderBefore, application.BankKeeper.GetBalance(ctx, grantor, probe.denom).Amount)
		}
	}
	ctx = parent.WithBlockTime(time.UnixMilli(int64(grant.NotBefore - 1)))
	err = deliver(send)
	require.ErrorIs(t, err, types.ErrAuthScope)
	require.ErrorContains(t, err, "outside validity interval")
	ctx = parent.WithBlockTime(time.UnixMilli(int64(grant.NotAfter)))
	err = deliver(send)
	require.ErrorContains(t, err, "authorization not found")
	ctx = parent
	assertMirror(grant, "7", timestamp, "7")
	require.Equal(t, before.AddRaw(7), application.BankKeeper.GetBalance(ctx, recipient, denom).Amount)
	require.Equal(t, grantorBefore.SubRaw(7), application.BankKeeper.GetBalance(ctx, grantor, denom).Amount)

	call("revokeGrant", grantID, uint8(1), uint64(43))
	grant, err = application.LayerXHost.ReadGrant(ctx, grantID)
	require.NoError(t, err)
	require.True(t, grant.Revoked)
	require.Equal(t, uint64(43), grant.RevokedAtSequence)
	sequence, err := application.LayerXAuthKeeper.RevocationSequence(ctx, public)
	require.NoError(t, err)
	require.Equal(t, uint64(43), sequence)
	live, reason, err = application.LayerXAuthKeeper.IsLive(ctx, grantID, 43, timestamp)
	require.NoError(t, err)
	require.False(t, live)
	require.Equal(t, "LXP_AUTH_REVOKED", reason)
	before = application.BankKeeper.GetBalance(ctx, recipient, denom).Amount
	grantorBefore = application.BankKeeper.GetBalance(ctx, grantor, denom).Amount

	assertMirror(grant, "7", timestamp, "7")
	err = deliver(send)
	require.Equal(t, before, application.BankKeeper.GetBalance(ctx, recipient, denom).Amount)
	require.Equal(t, grantorBefore, application.BankKeeper.GetBalance(ctx, grantor, denom).Amount)
	require.ErrorIs(t, err, types.ErrAuthRevoked)
	require.ErrorContains(t, err, "revocation epoch 41, grantor revocation 43")
	assertMirror(grant, "7", timestamp, "7")
	require.NoError(t, application.LayerXAuthKeeper.DeleteGrant(ctx, grantID))
	require.Empty(t, application.AuthzKeeper.GetAuthorizations(ctx, grantee, grantor))

	unboundKey, _, err := application.EvmKeeper.UnbindLayerX(ctx, delegate)
	require.NoError(t, err)
	require.Equal(t, delegateKey, unboundKey)
	replacementKey := bind(delegate, 73)
	regrantBody := bytes.Clone(body)
	copy(regrantBody[delegateKeyOffset:delegateKeyOffset+32], replacementKey[:])
	binary.BigEndian.PutUint64(regrantBody[revocationEpochOffset:revocationEpochOffset+8], sequence)
	regrantValues := call("grantCapability", regrantBody)
	require.Len(t, regrantValues, 1)
	regrantID := regrantValues[0].([32]byte)
	require.NotEqual(t, grantID, regrantID)
	regrant, err := application.LayerXHost.ReadGrant(ctx, regrantID)
	require.NoError(t, err)
	require.False(t, regrant.Revoked)
	require.Equal(t, sequence, regrant.GrantorRevocationSequence)
	live, reason, err = application.LayerXAuthKeeper.IsLive(ctx, regrantID, 44, timestamp)
	require.NoError(t, err)
	require.True(t, live)
	require.Empty(t, reason)
	require.Equal(t, grant.Scope, regrant.Scope)
	require.Equal(t, grant.NotBefore, regrant.NotBefore)
	require.Equal(t, grant.NotAfter, regrant.NotAfter)
	assertMirror(regrant, "0", 0, "")
	require.NoError(t, deliver(send))
	assertMirror(regrant, "7", timestamp, "7")
	require.Equal(t, before.AddRaw(7), application.BankKeeper.GetBalance(ctx, recipient, denom).Amount)
	require.Equal(t, grantorBefore.SubRaw(7), application.BankKeeper.GetBalance(ctx, grantor, denom).Amount)
	grant, err = application.LayerXHost.ReadGrant(ctx, grantID)
	require.NoError(t, err)
	require.True(t, grant.Revoked)
	require.Equal(t, uint64(43), grant.RevokedAtSequence)
}
