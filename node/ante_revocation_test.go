package app_test

import (
	"bytes"
	"crypto/ed25519"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
	"time"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	layerxauthkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/keeper"
	layerxauthtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	clienttx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachemulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/ante"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz"
	authzkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	wasmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func revocationAnteStores(ctx sdk.Context, added ...sdk.StoreKey) sdk.Context {
	stores := map[storetypes.StoreKey]storetypes.CacheWrapper{}
	keys := map[string]storetypes.StoreKey{}
	for _, key := range ctx.MultiStore().StoreKeys() {
		stores[key] = ctx.MultiStore().GetStore(key)
		keys[key.Name()] = key
	}
	for _, key := range added {
		stores[key] = dbadapter.Store{DB: dbm.NewMemDB()}
		keys[key.Name()] = key
	}
	return ctx.WithMultiStore(cachemulti.NewStore(dbm.NewMemDB(), stores, keys, nil, nil, nil, 0))
}

func revocationAnteTx(t *testing.T, a *app.App, config client.TxConfig, ctx sdk.Context, private cryptotypes.PrivKey, msg sdk.Msg) sdk.Tx {
	t.Helper()
	builder := config.NewTxBuilder()
	require.NoError(t, builder.SetMsgs(msg))
	builder.SetGasLimit(1_000_000)
	builder.SetFeeAmount(sdk.NewCoins(sdk.NewInt64Coin("uhpx", 100_000)))
	account := a.AccountKeeper.GetAccount(ctx, sdk.AccAddress(private.PubKey().Address()))
	require.NotNil(t, account)
	mode := config.SignModeHandler().DefaultMode()
	require.NoError(t, builder.SetSignatures(signing.SignatureV2{
		PubKey: private.PubKey(), Data: &signing.SingleSignatureData{SignMode: mode}, Sequence: account.GetSequence(),
	}))
	signature, err := clienttx.SignWithPrivKey(mode, authsigning.SignerData{
		ChainID: ctx.ChainID(), AccountNumber: account.GetAccountNumber(), Sequence: account.GetSequence(),
	}, builder, private, config, account.GetSequence())
	require.NoError(t, err)
	require.NoError(t, builder.SetSignatures(signature))
	return builder.GetTx()
}

func TestAnteChainRunsRevocationDecoratorAfterAuthzNested(t *testing.T) {
	t.Run("decorator_order", func(t *testing.T) {
		file, err := parser.ParseFile(token.NewFileSet(), "ante.go", nil, 0)
		require.NoError(t, err)
		var constructors []string
		ast.Inspect(file, func(n ast.Node) bool {
			assignment, ok := n.(*ast.AssignStmt)
			if !ok || len(assignment.Lhs) != 1 {
				return true
			}
			name, ok := assignment.Lhs[0].(*ast.Ident)
			if !ok || name.Name != "anteDecorators" {
				return true
			}
			for _, element := range assignment.Rhs[0].(*ast.CompositeLit).Elts {
				if identifier, ok := element.(*ast.Ident); ok {
					constructors = append(constructors, identifier.Name)
				} else {
					constructors = append(constructors, element.(*ast.CallExpr).Fun.(*ast.SelectorExpr).Sel.Name)
				}
			}
			return false
		})
		require.Equal(t, []string{
			"NewSetUpContextDecorator", "NewGaslessDecorator", "NewLimitSimulationGasDecorator",
			"NewRejectExtensionOptionsDecorator", "NewSpammingPreventionDecorator", "NewOracleVoteAloneDecorator",
			"NewValidateBasicDecorator", "NewTxTimeoutHeightDecorator", "NewValidateMemoDecorator",
			"NewConsumeGasForTxSizeDecorator", "NewPriorityDecorator", "NewSetPubKeyDecorator",
			"NewValidateSigCountDecorator", "NewSigGasConsumeDecorator", "sequentialVerifyDecorator",
			"NewIncrementSequenceDecorator", "NewEVMAddressDecorator", "NewAuthzNestedMessageDecorator",
			"NewRevocationDecorator", "NewAnteDecorator",
		}, constructors)
	})

	a := app.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	ctx := a.GetContextForDeliverTx(nil).WithBlockHeight(11).WithBlockTime(time.Unix(1_800_000_000, 0))
	ctx, _ = ctx.CacheContext()
	hostKey := sdk.NewKVStoreKey("revocation_ante_host")
	authKey := sdk.NewKVStoreKey("revocation_ante_auth")
	authzKey := sdk.NewKVStoreKey("revocation_ante_authz")
	ctx = revocationAnteStores(ctx, hostKey, authKey, authzKey)
	host, err := lxphost.NewHost(lxphost.Config{
		StoreKey: hostKey, NetworkID: 7, ParameterVersion: 1, TimestampWindowMs: 60000, GasLimit: 1000000,
	})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	encoding := app.MakeEncodingConfig()
	layerxauthtypes.RegisterInterfaces(encoding.InterfaceRegistry)
	cdc := codec.NewProtoCodec(encoding.InterfaceRegistry)
	grants := authzkeeper.NewKeeper(authzKey, cdc, a.MsgServiceRouter())
	keeper := layerxauthkeeper.NewKeeper(cdc, authKey, host, grants, &a.EvmKeeper, "")
	wasmConfig := wasmtypes.DefaultWasmConfig()
	options := app.HandlerOptions{
		HandlerOptions: ante.HandlerOptions{
			AccountKeeper: a.AccountKeeper, BankKeeper: a.BankKeeper, FeegrantKeeper: a.FeeGrantKeeper,
			ParamsKeeper: a.ParamsKeeper, SignModeHandler: encoding.TxConfig.SignModeHandler(),
		},
		IBCKeeper: a.IBCKeeper, WasmConfig: &wasmConfig, WasmKeeper: &a.WasmKeeper,
		OracleKeeper: &a.OracleKeeper, PricefeedKeeper: a.PricefeedKeeper, EVMKeeper: &a.EvmKeeper,
		LayerXAuthKeeper: keeper, UpgradeKeeper: &a.UpgradeKeeper, TracingInfo: a.GetBaseApp().TracingInfo,
		LatestCtxGetter: func() sdk.Context { return ctx },
	}
	t.Run("keeper_required", func(t *testing.T) {
		missing := options
		missing.LayerXAuthKeeper = nil
		handler, tracer, err := app.NewAnteHandler(missing)
		require.ErrorIs(t, err, sdkerrors.ErrLogic)
		require.ErrorContains(t, err, "layerxauth keeper is required")
		require.Nil(t, handler)
		require.Nil(t, tracer)
	})
	handler, _, err := app.NewAnteHandler(options)
	require.NoError(t, err)
	private := secp256k1.GenPrivKey()
	grantee := sdk.AccAddress(private.PubKey().Address())
	grantor := sdk.AccAddress(bytes.Repeat([]byte{0x31}, 20))
	recipient := sdk.AccAddress(bytes.Repeat([]byte{0x32}, 20))
	for _, address := range []sdk.AccAddress{grantor, grantee} {
		a.AccountKeeper.SetAccount(ctx, a.AccountKeeper.NewAccountWithAddress(ctx, address))
		coins := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1_000_000))
		require.NoError(t, a.BankKeeper.MintCoins(ctx, "evm", coins))
		require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "evm", address, coins))
	}
	address := common.BytesToAddress(grantor)
	a.EvmKeeper.SetAddressMapping(ctx, grantor, address)
	identityPrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{71}, ed25519.SeedSize))
	var public [32]byte
	copy(public[:], identityPrivate.Public().(ed25519.PublicKey))
	bind, err := evmtypes.LayerXBindMessage(a.EvmKeeper.ChainID(ctx), address, 0)
	require.NoError(t, err)
	var signature [64]byte
	copy(signature[:], ed25519.Sign(identityPrivate, bind))
	_, err = a.EvmKeeper.BindLayerX(ctx, address, public, signature)
	require.NoError(t, err)
	did := []byte(evmtypes.LayerXDid(public))
	id, err := lxphost.DIDID(did)
	require.NoError(t, err)
	registration := lxphost.Activity{
		ProtocolVersion: 3, Type: 0x00070001, ActorDID: did, AuthorityKey: public,
		NotBefore: uint64(ctx.BlockTime().UnixMilli()) - 1000, NotAfter: uint64(ctx.BlockTime().UnixMilli()) + 1000,
		IdempotencyKey: public, Payload: append(append([]byte{0x71, 1, 0, 2}, id[:]...), public[:]...),
	}
	hash, err := host.SigningHash(registration)
	require.NoError(t, err)
	registration.Signature = ed25519.Sign(identityPrivate, hash[:])
	receipt, err := host.Execute(lxphost.WithSequence(ctx, 41), registration, lxphost.KernelAuthority{
		Actor: id, Principal: id, VerifiedKey: public,
	})
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), receipt.ResultCode)
	sequence, err := keeper.RevocationSequence(ctx, public)
	require.NoError(t, err)
	require.Equal(t, uint64(41), sequence)
	send := banktypes.NewMsgSend(grantor, recipient, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1)))
	exec := authz.NewMsgExec(grantee, []sdk.Msg{send})
	for _, epoch := range []uint64{41, 40} {
		t.Run(map[uint64]string{41: "current_grant", 40: "revoked_grant"}[epoch], func(t *testing.T) {
			branch, _ := ctx.CacheContext()
			grant := &layerxauthtypes.LayerXScopeAuthorization{
				GrantId: bytes.Repeat([]byte{0x51}, 32), MsgTypeUrl: sdk.MsgTypeURL(send), Kind: 3,
				ModuleMask: 1 << 9, OrdinalMin: 0, OrdinalMax: 0xffff, RevocationEpoch: epoch,
				NotBefore: uint64(ctx.BlockTime().UnixMilli()) - 1000, NotAfter: uint64(ctx.BlockTime().UnixMilli()) + 1000,
			}
			require.NoError(t, grant.ValidateBasic())
			require.NoError(t, grants.SaveGrant(branch, grantee, grantor, grant, ctx.BlockTime().Add(time.Hour)))
			transaction := revocationAnteTx(t, a, encoding.TxConfig, branch, private, &exec)
			encoded, err := encoding.TxConfig.TxEncoder()(transaction)
			require.NoError(t, err)
			anteCtx, err := handler(branch.WithTxBytes(encoded), transaction, false)
			require.NoError(t, err)
			before := a.BankKeeper.GetBalance(anteCtx, recipient, "uhpx")
			_, err = grants.Exec(sdk.WrapSDKContext(anteCtx), &exec)
			if epoch == sequence {
				require.NoError(t, err)
				require.Equal(t, before.Amount.AddRaw(1), a.BankKeeper.GetBalance(anteCtx, recipient, "uhpx").Amount)
			} else {
				require.ErrorIs(t, err, layerxauthtypes.ErrAuthRevoked)
				require.ErrorContains(t, err, "revocation epoch 40, grantor revocation 41")
				require.Equal(t, before, a.BankKeeper.GetBalance(anteCtx, recipient, "uhpx"))
			}
		})
	}
	t.Run("nested_message_check_precedes_revocation", func(t *testing.T) {
		branch, _ := ctx.CacheContext()
		tooDeep := exec
		for i := 0; i < 5; i++ {
			inner := tooDeep
			tooDeep = authz.NewMsgExec(grantee, []sdk.Msg{&inner})
		}
		transaction := revocationAnteTx(t, a, encoding.TxConfig, branch, private, &tooDeep)
		_, err := handler(branch, transaction, false)
		require.EqualError(t, err, "permission denied, more nested msgs than permitted")
	})
}
