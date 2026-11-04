package testutil

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	ctypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/network"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz/keeper"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

var repair693BlockTime = time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

func repair693CLIGrant(t *testing.T, cfg network.Config, expiration time.Time) (*authz.MsgGrant, []byte) {
	t.Helper()
	granter := sdk.AccAddress(bytes.Repeat([]byte{1}, 20))
	grantee := sdk.AccAddress(bytes.Repeat([]byte{2}, 20))
	clientCtx := client.Context{}.
		WithCodec(cfg.Codec).
		WithLegacyAmino(cfg.LegacyAmino).
		WithInterfaceRegistry(cfg.InterfaceRegistry).
		WithTxConfig(cfg.TxConfig).
		WithChainID(cfg.ChainID)
	out, err := ExecGrant(&network.Validator{ClientCtx: clientCtx}, []string{
		grantee.String(), "send",
		fmt.Sprintf("--%s=%s", flags.FlagFrom, granter.String()),
		fmt.Sprintf("--%s=100%s", cli.FlagSpendLimit, cfg.BondDenom),
		fmt.Sprintf("--%s=%d", cli.FlagExpiration, expiration.Unix()),
		fmt.Sprintf("--%s=true", flags.FlagGenerateOnly),
		fmt.Sprintf("--%s=true", flags.FlagOffline),
		fmt.Sprintf("--%s=memory", flags.FlagKeyringBackend),
	})
	require.NoError(t, err, out.String())
	tx, err := cfg.TxConfig.TxJSONDecoder()(out.Bytes())
	require.NoError(t, err, out.String())
	wire, err := cfg.TxConfig.TxEncoder()(tx)
	require.NoError(t, err)
	decoded, err := cfg.TxConfig.TxDecoder()(wire)
	require.NoError(t, err)
	require.Len(t, decoded.GetMsgs(), 1)
	msg, ok := decoded.GetMsgs()[0].(*authz.MsgGrant)
	require.True(t, ok)
	require.NoError(t, msg.ValidateBasic())
	require.Equal(t, granter.String(), msg.Granter)
	require.Equal(t, grantee.String(), msg.Grantee)
	require.Equal(t, expiration, msg.Grant.Expiration)
	authorization, ok := msg.GetAuthorization().(*banktypes.SendAuthorization)
	require.True(t, ok)
	require.Equal(t, sdk.NewCoins(sdk.NewInt64Coin(cfg.BondDenom, 100)), authorization.SpendLimit)
	return msg, wire
}

func repair693Keeper(t *testing.T, cfg network.Config) (sdk.Context, keeper.Keeper, *sdk.KVStoreKey) {
	t.Helper()
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db)
	keys := sdk.NewKVStoreKeys(authz.ModuleName, authtypes.StoreKey, banktypes.StoreKey, paramtypes.StoreKey)
	for _, key := range keys {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	tkey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	cms.MountStoreWithDB(tkey, sdk.StoreTypeTransient, db)
	require.NoError(t, cms.LoadLatestVersion())
	t.Cleanup(func() { require.NoError(t, cms.Close()) })
	ctx := sdk.NewContext(cms, tmproto.Header{Height: 1, Time: repair693BlockTime}, false)
	pk := paramskeeper.NewKeeper(cfg.Codec, cfg.LegacyAmino, keys[paramtypes.StoreKey], tkey)
	ak := authkeeper.NewAccountKeeper(cfg.Codec, keys[authtypes.StoreKey], pk.Subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, nil)
	bk := bankkeeper.NewBaseKeeper(cfg.Codec, keys[banktypes.StoreKey], ak, pk.Subspace(banktypes.ModuleName), nil)
	router := baseapp.NewMsgServiceRouter()
	router.SetInterfaceRegistry(cfg.InterfaceRegistry)
	banktypes.RegisterMsgServer(router, bankkeeper.NewMsgServerImpl(bk))
	k := keeper.NewKeeper(keys[authz.ModuleName], cfg.Codec, router)
	authz.RegisterMsgServer(router, k)
	return ctx, k, keys[authz.ModuleName]
}

func repair693Grants(t *testing.T, cfg network.Config, ctx sdk.Context, k keeper.Keeper, msg *authz.MsgGrant) *authz.QueryGrantsResponse {
	t.Helper()
	transport := baseapp.NewQueryServerTestHelper(ctx, cfg.InterfaceRegistry)
	authz.RegisterQueryServer(transport, k)
	grants, err := authz.NewQueryClient(transport).Grants(context.Background(), &authz.QueryGrantsRequest{
		Granter: msg.Granter, Grantee: msg.Grantee,
	})
	require.NoError(t, err)
	var decoded authz.QueryGrantsResponse
	require.NoError(t, cfg.Codec.UnmarshalAsJSON(cfg.Codec.MustMarshalJSON(grants), &decoded))
	return &decoded
}

func TestRepairM_693_AuthzCLIConfig(t *testing.T) {
	// The owning suite is tagged; keep its call checked by the untagged gate too.
	file, err := parser.ParseFile(token.NewFileSet(), "cli_test.go", nil, 0)
	require.NoError(t, err)
	checked := false
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "DefaultConfig" {
			return true
		}
		owner, ok := selector.X.(*ast.Ident)
		require.True(t, ok)
		require.Equal(t, "network", owner.Name)
		require.Len(t, call.Args, 1)
		arg, ok := call.Args[0].(*ast.Ident)
		require.True(t, ok)
		require.Equal(t, "t", arg.Name)
		checked = true
		return true
	})
	require.True(t, checked)

	cfg := network.DefaultConfig(t)
	cfg.NumValidators = 1
	s := NewIntegrationTestSuite(cfg)
	s.SetT(t)
	require.Same(t, cfg.Codec, s.cfg.Codec)
	require.NotNil(t, s.cfg.AppConstructor)
	msg, _ := repair693CLIGrant(t, s.cfg, repair693BlockTime.Add(time.Hour))
	ctx, k, _ := repair693Keeper(t, s.cfg)
	_, err = k.Grant(sdk.WrapSDKContext(ctx), msg)
	require.NoError(t, err)
	grants := repair693Grants(t, s.cfg, ctx, k, msg)
	require.Len(t, grants.Grants, 1)
	require.Equal(t, msg.Grant.Expiration, grants.Grants[0].Expiration)
	require.Equal(t, msg.Grant.Authorization.TypeUrl, grants.Grants[0].Authorization.TypeUrl)
	require.Len(t, ctx.EventManager().Events(), 1)
	event, err := sdk.ParseTypedEvent(ctx.EventManager().ABCIEvents()[0])
	require.NoError(t, err)
	require.Equal(t, &authz.EventGrant{Granter: msg.Granter, Grantee: msg.Grantee, MsgTypeUrl: typeMsgSend}, event)
}

func TestRepairM_693_ExpiredGrantCLIFixture(t *testing.T) {
	for _, delta := range []time.Duration{-time.Hour, 0} {
		t.Run(delta.String(), func(t *testing.T) {
			cfg := network.DefaultConfig(t)
			msg, _ := repair693CLIGrant(t, cfg, repair693BlockTime.Add(delta))
			ctx, k, key := repair693Keeper(t, cfg)
			response, err := k.Grant(sdk.WrapSDKContext(ctx), msg)
			require.Nil(t, response)
			require.ErrorIs(t, err, authz.ErrInvalidExpirationTime)
			require.Empty(t, ctx.EventManager().Events())
			iterator := ctx.KVStore(key).Iterator(nil, nil)
			require.False(t, iterator.Valid(), "inspect raw storage before any query can clean expired grants")
			require.NoError(t, iterator.Close())

			delivery := sdkerrors.ResponseDeliverTxWithEvents(err, 0, 0, ctx.EventManager().ABCIEvents(), false)
			txResponse := sdk.NewResponseResultTx(&ctypes.ResultTx{TxResult: abci.ExecTxResult(delivery)}, nil, "")
			var output bytes.Buffer
			clientCtx := client.Context{}.WithCodec(cfg.Codec).WithOutput(&output).WithOutputFormat("json")
			require.NoError(t, clientCtx.PrintProto(txResponse))
			var decoded sdk.TxResponse
			require.NoError(t, cfg.Codec.UnmarshalAsJSON(output.Bytes(), &decoded))
			require.Equal(t, uint32(3), decoded.Code)
			require.Equal(t, "authz", decoded.Codespace)
			s := NewIntegrationTestSuite(cfg)
			s.SetT(t)
			s.assertExpiredGrantRejected(&decoded, repair693Grants(t, cfg, ctx, k, msg))
			require.Empty(t, k.ExportGenesis(ctx).Authorization)
			for _, event := range decoded.Events {
				require.NotEqual(t, proto.MessageName(&authz.EventGrant{}), event.Type)
			}
		})
	}
}
