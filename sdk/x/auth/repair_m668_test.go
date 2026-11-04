package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	nodeante "github.com/Sidiora-Labs/Paxeer-X-Network/node/ante"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	clienttx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/ante"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/vesting"
	vestingcli "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/vesting/client/cli"
	vestexported "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/vesting/exported"
	vestingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/vesting/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/feegrant"
	feegrantkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/feegrant/keeper"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/gogo/gateway"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

// The fixture uses production keepers, codecs and isolated in-memory KV stores.
// No node is started and no peer implementation is replaced by a test double.
type repairM668Fixture struct {
	ctx                         sdk.Context
	cdc                         *codec.ProtoCodec
	registry                    codectypes.InterfaceRegistry
	config                      client.TxConfig
	ak                          authkeeper.AccountKeeper
	bk                          bankkeeper.BaseKeeper
	pk                          paramskeeper.Keeper
	fk                          feegrantkeeper.Keeper
	authKey, bankKey, paramsKey sdk.StoreKey
}

func newRepairM668Fixture(t *testing.T) *repairM668Fixture {
	t.Helper()
	registry := codectypes.NewInterfaceRegistry()
	sdk.RegisterInterfaces(registry)
	cryptocodec.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)
	banktypes.RegisterInterfaces(registry)
	vestingtypes.RegisterInterfaces(registry)
	feegrant.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	sdk.RegisterLegacyAminoCodec(amino)
	cryptocodec.RegisterCrypto(amino)
	authtypes.RegisterLegacyAminoCodec(amino)
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db)
	authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	bankKey := sdk.NewKVStoreKey(banktypes.StoreKey)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	deferredKey := sdk.NewKVStoreKey(banktypes.DeferredCacheStoreKey)
	feegrantKey := sdk.NewKVStoreKey(feegrant.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	for _, key := range []sdk.StoreKey{authKey, bankKey, paramsKey, deferredKey, feegrantKey} {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	cms.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, nil)
	require.NoError(t, cms.LoadLatestVersion())
	t.Cleanup(func() { require.NoError(t, cms.Close()) })
	ctx := sdk.NewContext(cms, tmproto.Header{Height: 10, ChainID: "repair-m668", Time: time.Unix(2000000000, 0)}, true)
	pk := paramskeeper.NewKeeper(cdc, amino, paramsKey, transientKey)
	ak := authkeeper.NewAccountKeeper(cdc, authKey, pk.Subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount,
		map[string][]string{authtypes.FeeCollectorName: nil})
	ak.SetParams(ctx, authtypes.DefaultParams())
	ak.SetModuleAccount(ctx, authtypes.NewEmptyModuleAccount(authtypes.FeeCollectorName))
	bk := bankkeeper.NewBaseKeeperWithDeferredCache(cdc, bankKey, ak, pk.Subspace(banktypes.ModuleName), nil, deferredKey)
	bk.SetParams(ctx, banktypes.DefaultParams())
	return &repairM668Fixture{ctx: ctx, cdc: cdc, registry: registry, config: authtx.NewTxConfig(cdc, authtx.DefaultSignModes),
		ak: ak, bk: bk, pk: pk, fk: feegrantkeeper.NewKeeper(cdc, feegrantKey, ak), authKey: authKey, bankKey: bankKey, paramsKey: paramsKey}
}

func repairM668Coins(amount int64) sdk.Coins { return sdk.NewCoins(sdk.NewInt64Coin("uhpx", amount)) }

func (f *repairM668Fixture) fund(t *testing.T, addr sdk.AccAddress, amount int64) {
	t.Helper()
	if !f.ak.HasAccount(f.ctx, addr) {
		f.ak.SetAccount(f.ctx, f.ak.NewAccountWithAddress(f.ctx, addr))
	}
	genesis := f.bk.ExportGenesis(f.ctx)
	genesis.Balances = append(genesis.Balances, banktypes.Balance{Address: addr.String(), Coins: repairM668Coins(amount)})
	genesis.Supply = genesis.Supply.Add(repairM668Coins(amount)...)
	f.bk.InitGenesis(f.ctx, genesis)
}

func repairM668Doc(t *testing.T, path string, claims ...string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	for _, claim := range claims {
		require.Contains(t, string(data), claim, path)
	}
	return string(data)
}

func (f *repairM668Fixture) wireTx(t *testing.T, builder client.TxBuilder) (sdk.Tx, []byte) {
	t.Helper()
	wire, err := f.config.TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	decoded, err := f.config.TxDecoder()(wire)
	require.NoError(t, err)
	return decoded, wire
}

func (f *repairM668Fixture) signed(t *testing.T, ctx sdk.Context, priv cryptotypes.PrivKey, gas uint64, fee int64, memo string, seq uint64, granter sdk.AccAddress) (sdk.Tx, []byte) {
	t.Helper()
	addr := sdk.AccAddress(priv.PubKey().Address())
	builder := f.config.NewTxBuilder()
	require.NoError(t, builder.SetMsgs(&banktypes.MsgSend{FromAddress: addr.String(), ToAddress: addr.String(), Amount: repairM668Coins(1)}))
	builder.SetGasLimit(gas)
	builder.SetFeeAmount(repairM668Coins(fee))
	builder.SetMemo(memo)
	builder.SetFeeGranter(granter)
	require.NoError(t, builder.SetSignatures(signing.SignatureV2{PubKey: priv.PubKey(), Sequence: seq,
		Data: &signing.SingleSignatureData{SignMode: signing.SignMode_SIGN_MODE_DIRECT}}))
	account := f.ak.GetAccount(ctx, addr)
	sig, err := clienttx.SignWithPrivKey(signing.SignMode_SIGN_MODE_DIRECT, authsigning.SignerData{
		ChainID: ctx.ChainID(), AccountNumber: account.GetAccountNumber(), Sequence: seq}, builder, priv, f.config, seq)
	require.NoError(t, err)
	require.NoError(t, builder.SetSignatures(sig))
	return f.wireTx(t, builder)
}

func TestRepairM_668_FeePriority(t *testing.T) {
	repairM668Doc(t, "spec/01_concepts.md", "Union", "2000uhpx", "paxd start", "10^12", "WithPriority")
	f := newRepairM668Fixture(t)
	base, err := sdk.GetBaseDenom()
	require.NoError(t, err)
	require.Equal(t, "uhpx", base)
	for _, tc := range []struct {
		name, global, local string
		fee                 int64
		check, simulate     bool
		wantErr             bool
		priority            int64
	}{
		{"global floor rejects", "0.01uhpx", "0.005uhpx", 1999, true, false, true, 0},
		{"global floor accepts", "0.01uhpx", "0.005uhpx", 2000, true, false, false, 10000000000},
		{"local floor rejects", "0.01uhpx", "0.02uhpx", 3999, true, false, true, 0},
		{"local floor accepts", "0.01uhpx", "0.02uhpx", 4000, true, false, false, 20000000000},
		{"ceil rejects", "0.010001uhpx", "0uhpx", 2000, true, false, true, 0},
		{"ceil accepts", "0.010001uhpx", "0uhpx", 2001, true, false, false, 10005000000},
		{"deliver skips floor", "0.01uhpx", "0.02uhpx", 1, false, false, false, 5000000},
		{"simulation skips floor", "0.01uhpx", "0.02uhpx", 1, true, true, false, 5000000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			global, err := sdk.ParseDecCoins(tc.global)
			require.NoError(t, err)
			local, err := sdk.ParseDecCoins(tc.local)
			require.NoError(t, err)
			f.pk.SetFeesParams(f.ctx, paramtypes.NewFeesParams(global))
			builder := f.config.NewTxBuilder()
			builder.SetGasLimit(200000)
			builder.SetFeeAmount(repairM668Coins(tc.fee))
			tx, _ := f.wireTx(t, builder)
			fees, priority, err := ante.CheckTxFeeWithValidatorMinGasPrices(f.ctx.WithIsCheckTx(tc.check).WithMinGasPrices(local), tx, tc.simulate, f.pk)
			if tc.wantErr {
				require.ErrorIs(t, err, sdkerrors.ErrInsufficientFee)
				return
			}
			require.NoError(t, err)
			require.Equal(t, repairM668Coins(tc.fee), fees)
			require.Equal(t, tc.priority, priority)
		})
	}
	prices, err := sdk.ParseDecCoins("0.01uhpx,2uother")
	require.NoError(t, err)
	params := paramtypes.NewFeesParams(prices)
	params.AllowedFeeDenoms = []string{"uother"}
	f.pk.SetFeesParams(f.ctx, params)
	builder := f.config.NewTxBuilder()
	builder.SetGasLimit(10)
	builder.SetFeeAmount(sdk.NewCoins(sdk.NewInt64Coin("uother", 20), sdk.NewInt64Coin("urejected", 999)))
	tx, _ := f.wireTx(t, builder)
	fees, priority, err := ante.CheckTxFeeWithValidatorMinGasPrices(f.ctx, tx, false, f.pk)
	require.NoError(t, err)
	require.Equal(t, sdk.NewCoins(sdk.NewInt64Coin("uother", 20)), fees)
	require.Equal(t, int64(2), priority)
	builder.SetGasLimit(0)
	tx, _ = f.wireTx(t, builder)
	_, priority, err = ante.CheckTxFeeWithValidatorMinGasPrices(f.ctx.WithIsCheckTx(false), tx, false, f.pk)
	require.NoError(t, err)
	require.Zero(t, priority)
	require.Equal(t, int64(math.MaxInt64), ante.GetTxPriority(repairM668Coins(math.MaxInt64), 1))
}

func (f *repairM668Fixture) gateway(t *testing.T) *httptest.Server {
	t.Helper()
	marshaler := &gateway.JSONPb{OrigName: true, EmitDefaults: true, AnyResolver: f.registry}
	mux := runtime.NewServeMux(runtime.WithMarshalerOption(runtime.MIMEWildcard, marshaler))
	require.NoError(t, authtypes.RegisterQueryHandlerServer(context.Background(), mux, f.ak))
	require.NoError(t, banktypes.RegisterQueryHandlerServer(context.Background(), mux, f.bk))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r.WithContext(sdk.WrapSDKContext(f.ctx)))
	}))
	t.Cleanup(server.Close)
	return server
}

func repairM668Get(t *testing.T, server *httptest.Server, path string, wantStatus int) []byte {
	t.Helper()
	response, err := server.Client().Get(server.URL + path)
	require.NoError(t, err)
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, wantStatus, response.StatusCode, string(body))
	return body
}

func TestRepairM_668_AccountBalances(t *testing.T) {
	repairM668Doc(t, "spec/02_state.md", "Coin balances belong to `x/bank`", "query.proto", "0x01", "0x02", "0x04")
	repairM668Doc(t, "types/account.go", "AccountI exposes account identity and signing metadata", "balances are persisted and queried separately by x/bank")
	f := newRepairM668Fixture(t)
	addr := sdk.AccAddress([]byte("m668-account-owner__"))
	to := sdk.AccAddress([]byte("m668-account-target_"))
	f.fund(t, addr, 100)
	authKey := authtypes.AddressStoreKey(addr)
	bankKey := banktypes.CreatePrefixedAccountStoreKey(addr, []byte("uhpx"))
	require.Equal(t, append([]byte{1}, addr...), authKey)
	require.Equal(t, append(append([]byte{2, byte(len(addr))}, addr...), []byte("uhpx")...), bankKey)
	before := append([]byte(nil), f.ctx.KVStore(f.authKey).Get(authKey)...)
	require.NotEmpty(t, before)
	require.NotEmpty(t, f.ctx.KVStore(f.bankKey).Get(bankKey))
	require.NoError(t, f.bk.SendCoins(f.ctx, addr, to, repairM668Coins(7)))
	require.Equal(t, before, f.ctx.KVStore(f.authKey).Get(authKey), "bank transfer preserves signing metadata")
	require.NoError(t, f.bk.AddWei(f.ctx, addr, sdk.NewInt(123)))
	require.Equal(t, sdk.NewInt(123), f.bk.GetWeiBalance(f.ctx, addr))
	require.NotEmpty(t, f.ctx.KVStore(f.bankKey).Get(append([]byte{4}, addr...)))
	server := f.gateway(t)
	var account authtypes.QueryAccountResponse
	require.NoError(t, f.cdc.UnmarshalAsJSON(repairM668Get(t, server, "/cosmos/auth/v1beta1/accounts/"+addr.String(), http.StatusOK), &account))
	var unpacked authtypes.AccountI
	require.NoError(t, f.registry.UnpackAny(account.Account, &unpacked))
	require.Equal(t, f.ak.GetAccount(f.ctx, addr), unpacked)
	accountJSON, err := f.cdc.MarshalInterfaceJSON(unpacked)
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(accountJSON, &fields))
	require.NotContains(t, fields, "coins")
	require.NotContains(t, fields, "balance")
	require.NotContains(t, fields, "balances")
	var balance banktypes.QueryAllBalancesResponse
	require.NoError(t, f.cdc.UnmarshalAsJSON(repairM668Get(t, server, "/cosmos/bank/v1beta1/balances/"+addr.String(), http.StatusOK), &balance))
	require.Equal(t, repairM668Coins(93), balance.Balances, "ordinary bank query excludes separate fractional wei")
}

func TestRepairM_668_AnteOrder(t *testing.T) {
	doc := repairM668Doc(t, "spec/03_antehandlers.md", "DeductFeeDecorator", "UseGrantedFees", "node/ante.go")
	// Pin the full constructor order as well as exercising its fee/signature path below.
	file, err := parser.ParseFile(token.NewFileSet(), "ante/ante.go", nil, 0)
	require.NoError(t, err)
	var constructors []string
	ast.Inspect(file, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok || len(assignment.Lhs) != 1 {
			return true
		}
		name, ok := assignment.Lhs[0].(*ast.Ident)
		if !ok || name.Name != "anteDecorators" {
			return true
		}
		list, ok := assignment.Rhs[0].(*ast.CompositeLit)
		require.True(t, ok)
		for _, element := range list.Elts {
			if call, ok := element.(*ast.CallExpr); ok {
				constructors = append(constructors, call.Fun.(*ast.Ident).Name)
			} else {
				constructors = append(constructors, element.(*ast.Ident).Name)
			}
		}
		return false
	})
	require.Equal(t, []string{"NewDefaultSetUpContextDecorator", "NewRejectExtensionOptionsDecorator", "NewValidateBasicDecorator", "NewTxTimeoutHeightDecorator", "NewValidateMemoDecorator", "NewConsumeGasForTxSizeDecorator", "NewDeductFeeDecorator", "NewSetPubKeyDecorator", "NewValidateSigCountDecorator", "NewSigGasConsumeDecorator", "sigVerifyDecorator", "NewIncrementSequenceDecorator"}, constructors)
	last := -1
	for _, name := range []string{"SetUpContextDecorator", "RejectExtensionOptionsDecorator", "ValidateBasicDecorator", "TxTimeoutHeightDecorator", "ValidateMemoDecorator", "ConsumeTxSizeGasDecorator", "DeductFeeDecorator", "SetPubKeyDecorator", "ValidateSigCountDecorator", "SigGasConsumeDecorator", "SigVerificationDecorator", "IncrementSequenceDecorator"} {
		pos := strings.Index(doc, "`"+name+"`")
		require.Greater(t, pos, last, name)
		last = pos
	}
	f := newRepairM668Fixture(t)
	priv := secp256k1.GenPrivKeyFromSecret([]byte("m668-ante"))
	addr := sdk.AccAddress(priv.PubKey().Address())
	f.fund(t, addr, 100000)
	handler, err := ante.NewAnteHandler(ante.HandlerOptions{AccountKeeper: f.ak, BankKeeper: f.bk, ParamsKeeper: f.pk, FeegrantKeeper: f.fk, SignModeHandler: f.config.SignModeHandler()})
	require.NoError(t, err)
	for _, tc := range []struct {
		name, memo string
		gas        uint64
		fee        int64
		want       error
	}{
		{"memo before fees", strings.Repeat("x", 257), 200000, 1, sdkerrors.ErrMemoTooLarge},
		{"size gas before fees", "", 1, 0, sdkerrors.ErrOutOfGas},
		{"fee checker before signatures", "", 200000, 1999, sdkerrors.ErrInsufficientFee},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := f.ctx.CacheContext()
			tx, wire := f.signed(t, ctx, priv, tc.gas, tc.fee, tc.memo, 0, nil)
			_, err := handler(ctx.WithTxBytes(wire), tx, false)
			require.ErrorIs(t, err, tc.want)
			require.Equal(t, repairM668Coins(100000), f.bk.GetAllBalances(ctx, addr))
			require.Zero(t, f.ak.GetAccount(ctx, addr).GetSequence())
		})
	}
	// A validly encoded transaction signed for a different chain reaches fee
	// deduction, then fails signature verification. The caller discards this cache.
	cached, _ := f.ctx.CacheContext()
	wrongDomain, wrongWire := f.signed(t, cached.WithChainID("wrong-domain"), priv, 200000, 2000, "", 0, nil)
	_, err = handler(cached.WithTxBytes(wrongWire), wrongDomain, false)
	require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
	require.Equal(t, repairM668Coins(98000), f.bk.GetAllBalances(cached, addr))
	require.Equal(t, repairM668Coins(100000), f.bk.GetAllBalances(f.ctx, addr))
	require.Zero(t, f.ak.GetAccount(cached, addr).GetSequence())
	tx, wire := f.signed(t, f.ctx, priv, 200000, 2000, "", 0, nil)
	newCtx, err := handler(f.ctx.WithTxBytes(wire), tx, false)
	require.NoError(t, err)
	require.Equal(t, int64(10000000000), newCtx.Priority())
	require.Equal(t, uint64(1), f.ak.GetAccount(newCtx, addr).GetSequence())
	require.Equal(t, priv.PubKey(), f.ak.GetAccount(newCtx, addr).GetPubKey())
	require.Equal(t, repairM668Coins(98000), f.bk.GetAllBalances(newCtx, addr))
	f.bk.WriteDeferredBalances(newCtx)
	require.Equal(t, repairM668Coins(2000), f.bk.GetAllBalances(newCtx, f.ak.GetModuleAddress(authtypes.FeeCollectorName)))
	granter := sdk.AccAddress([]byte("m668-fee-granter____"))
	f.fund(t, granter, 5000)
	require.NoError(t, f.fk.GrantAllowance(f.ctx, granter, addr, &feegrant.BasicAllowance{SpendLimit: repairM668Coins(3000)}))
	tx, wire = f.signed(t, f.ctx, priv, 200000, 2000, "", 1, granter)
	newCtx, err = handler(f.ctx.WithTxBytes(wire), tx, false)
	require.NoError(t, err)
	require.Equal(t, repairM668Coins(98000), f.bk.GetAllBalances(newCtx, addr))
	require.Equal(t, repairM668Coins(3000), f.bk.GetAllBalances(newCtx, granter))
	allowance, err := f.fk.GetAllowance(newCtx, granter, addr)
	require.NoError(t, err)
	require.Equal(t, repairM668Coins(1000), allowance.(*feegrant.BasicAllowance).SpendLimit)
}

func TestRepairM_668_VestingCreationAndGenesis(t *testing.T) {
	repairM668Doc(t, "spec/05_vesting.md", "CreateVestingAccount", "must be zero", "rejects nonzero markers", "google.protobuf.Any accounts", "PeriodicVestingAccount", "PermanentLockedAccount")
	f := newRepairM668Fixture(t)
	sender := sdk.AccAddress([]byte("m668-vesting-sender_"))
	f.fund(t, sender, 1000)
	amount := repairM668Coins(100)
	for i, delayed := range []bool{false, true} {
		to := sdk.AccAddress(bytes.Repeat([]byte{byte(90 + i)}, 20))
		produced := vestingtypes.NewMsgCreateVestingAccount(sender, to, amount, f.ctx.BlockTime().Unix()+100, delayed, sender)
		require.NoError(t, produced.ValidateBasic())
		wire, err := f.cdc.Marshal(produced)
		require.NoError(t, err)
		var received vestingtypes.MsgCreateVestingAccount
		require.NoError(t, f.cdc.Unmarshal(wire, &received))
		_, err = vesting.NewMsgServerImpl(f.ak, f.bk).CreateVestingAccount(sdk.WrapSDKContext(f.ctx), &received)
		require.NoError(t, err)
		account := f.ak.GetAccount(f.ctx, to)
		vest := account.(vestexported.VestingAccount)
		require.Equal(t, amount, f.bk.GetAllBalances(f.ctx, to))
		require.True(t, f.bk.SpendableCoins(f.ctx, to).IsZero())
		require.Equal(t, amount, vest.GetVestedCoins(time.Unix(produced.EndTime, 0)))
		if delayed {
			require.IsType(t, &vestingtypes.DelayedVestingAccount{}, account)
		} else {
			require.Equal(t, f.ctx.BlockTime().Unix(), vest.GetStartTime())
		}
	}
	// The two live account types differ at the end-time boundary.
	for i, delayed := range []bool{false, true} {
		to := sdk.AccAddress(bytes.Repeat([]byte{byte(94 + i)}, 20))
		msg := vestingtypes.NewMsgCreateVestingAccount(sender, to, amount, f.ctx.BlockTime().Unix(), delayed, sender)
		require.NoError(t, msg.ValidateBasic())
		cached, _ := f.ctx.CacheContext()
		_, err := vesting.NewMsgServerImpl(f.ak, f.bk).CreateVestingAccount(sdk.WrapSDKContext(cached), msg)
		if delayed {
			require.NoError(t, err)
			require.Equal(t, amount, f.bk.SpendableCoins(cached, to))
		} else {
			require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
			require.Nil(t, f.ak.GetAccount(cached, to))
		}
	}
	periodic := vestingtypes.NewPeriodicVestingAccount(authtypes.NewBaseAccountWithAddress(sdk.AccAddress(bytes.Repeat([]byte{92}, 20))), amount, f.ctx.BlockTime().Unix(), vestingtypes.Periods{{Length: 100, Amount: amount}}, sender)
	permanent := vestingtypes.NewPermanentLockedAccount(authtypes.NewBaseAccountWithAddress(sdk.AccAddress(bytes.Repeat([]byte{93}, 20))), amount, sender)
	for _, account := range []authtypes.GenesisAccount{periodic, permanent} {
		f.fund(t, account.GetAddress(), 100)
		f.ak.SetAccount(f.ctx, f.ak.NewAccount(f.ctx, account))
	}
	exported := auth.ExportGenesis(f.ctx, f.ak)
	require.NoError(t, authtypes.ValidateGenesis(*exported))
	authJSON, err := f.cdc.MarshalAsJSON(exported)
	require.NoError(t, err)
	require.Contains(t, string(authJSON), "/cosmos.vesting.v1beta1.ContinuousVestingAccount")
	require.Contains(t, string(authJSON), "\"admin\":\""+sender.String()+"\"")
	require.Contains(t, string(authJSON), "\"cancelled_time\":\"0\"")
	bankJSON, err := f.cdc.MarshalAsJSON(f.bk.ExportGenesis(f.ctx))
	require.NoError(t, err)
	restored := newRepairM668Fixture(t)
	var authGenesis authtypes.GenesisState
	require.NoError(t, restored.cdc.UnmarshalAsJSON(authJSON, &authGenesis))
	require.NoError(t, authtypes.ValidateGenesis(authGenesis))
	auth.InitGenesis(restored.ctx, restored.ak, authGenesis)
	var bankGenesis banktypes.GenesisState
	require.NoError(t, restored.cdc.UnmarshalAsJSON(bankJSON, &bankGenesis))
	restored.bk.InitGenesis(restored.ctx, &bankGenesis)
	f.ak.IterateAccounts(f.ctx, func(account authtypes.AccountI) bool {
		got := restored.ak.GetAccount(restored.ctx, account.GetAddress())
		require.IsType(t, account, got)
		if vest, ok := account.(vestexported.VestingAccount); ok {
			restoredVest := got.(vestexported.VestingAccount)
			require.Equal(t, vest.GetOriginalVesting(), restoredVest.GetOriginalVesting())
			before, err := f.cdc.MarshalInterfaceJSON(account)
			require.NoError(t, err)
			after, err := restored.cdc.MarshalInterfaceJSON(got)
			require.NoError(t, err)
			// InitGenesis allocates fresh account numbers; compare the lifecycle fields directly.
			var a, b map[string]interface{}
			require.NoError(t, json.Unmarshal(before, &a))
			require.NoError(t, json.Unmarshal(after, &b))
			aBase := a["base_vesting_account"].(map[string]interface{})
			bBase := b["base_vesting_account"].(map[string]interface{})
			require.Equal(t, aBase["admin"], bBase["admin"])
			require.Equal(t, aBase["cancelled_time"], bBase["cancelled_time"])
			for _, when := range []int64{2000000000, 2000000050, 2000000100} {
				ctx := restored.ctx.WithBlockTime(time.Unix(when, 0))
				require.Equal(t, vest.GetVestedCoins(ctx.BlockTime()), restoredVest.GetVestedCoins(ctx.BlockTime()))
				require.Equal(t, f.bk.SpendableCoins(f.ctx.WithBlockTime(ctx.BlockTime()), account.GetAddress()), restored.bk.SpendableCoins(ctx, account.GetAddress()))
			}
		}
		return false
	})
	periodic.CancelledTime = 1
	invalid := authtypes.NewGenesisState(authtypes.DefaultParams(), authtypes.GenesisAccounts{periodic})
	invalidJSON, err := f.cdc.MarshalAsJSON(invalid)
	require.NoError(t, err)
	var imported authtypes.GenesisState
	require.NoError(t, restored.cdc.UnmarshalAsJSON(invalidJSON, &imported))
	require.ErrorContains(t, authtypes.ValidateGenesis(imported), "cancellation")
}

func TestRepairM_668_ClientBindings(t *testing.T) {
	repairM668Doc(t, "spec/07_client.md", "GET /cosmos/auth/v1beta1/accounts/{address}", "paxd tx vesting create-vesting-account", "--delayed --admin", "no registered `create-periodic-vesting-account`")
	f := newRepairM668Fixture(t)
	sender := sdk.AccAddress([]byte("m668-client-sender__"))
	f.fund(t, sender, 1000)
	commands := vestingcli.GetTxCmd().Commands()
	require.Len(t, commands, 1)
	require.Equal(t, "create-vesting-account", commands[0].Name())
	server := f.gateway(t)
	for i, delayed := range []bool{false, true} {
		to := sdk.AccAddress(bytes.Repeat([]byte{byte(100 + i)}, 20))
		var output bytes.Buffer
		clientCtx := client.Context{}.WithCodec(f.cdc).WithInterfaceRegistry(f.registry).WithTxConfig(f.config).
			WithKeyring(keyring.NewInMemory()).WithOutput(&output).WithOutputFormat("json").WithChainID(f.ctx.ChainID())
		cmd := vestingcli.GetTxCmd()
		cmd.SetContext(context.WithValue(context.Background(), client.ClientContextKey, &clientCtx))
		args := []string{"create-vesting-account", to.String(), "100uhpx", strconv.FormatInt(f.ctx.BlockTime().Unix()+100, 10), "--from", sender.String(), "--generate-only", "--offline", "--admin", sender.String()}
		if delayed {
			args = append(args, "--delayed")
		}
		cmd.SetArgs(args)
		require.NoError(t, cmd.Execute())
		decoded, err := f.config.TxJSONDecoder()(output.Bytes())
		require.NoError(t, err)
		require.Len(t, decoded.GetMsgs(), 1)
		msg := decoded.GetMsgs()[0].(*vestingtypes.MsgCreateVestingAccount)
		require.Equal(t, delayed, msg.Delayed)
		require.Equal(t, sender.String(), msg.Admin)
		require.Equal(t, sender.String(), msg.FromAddress)
		require.Equal(t, []sdk.AccAddress{sender}, msg.GetSigners())
		_, err = vesting.NewMsgServerImpl(f.ak, f.bk).CreateVestingAccount(sdk.WrapSDKContext(f.ctx), msg)
		require.NoError(t, err)
		body := repairM668Get(t, server, "/cosmos/auth/v1beta1/accounts/"+to.String(), http.StatusOK)
		var response authtypes.QueryAccountResponse
		require.NoError(t, f.cdc.UnmarshalAsJSON(body, &response))
		var account authtypes.AccountI
		require.NoError(t, f.registry.UnpackAny(response.Account, &account))
		require.Equal(t, f.ak.GetAccount(f.ctx, to), account)
		if delayed {
			require.IsType(t, &vestingtypes.DelayedVestingAccount{}, account)
		} else {
			require.IsType(t, &vestingtypes.ContinuousVestingAccount{}, account)
		}
		require.Equal(t, repairM668Coins(100), f.bk.GetAllBalances(f.ctx, to))
	}
	repairM668Get(t, server, "/cosmos/auth/v1beta1/account?address="+sender.String(), http.StatusNotFound)
}

func TestRepairM_668_SequenceParameter(t *testing.T) {
	repairM668Doc(t, "spec/07_params.md", "`DisableSeqnoCheck`", "`KeyDisableSeqnoCheck` | `bool` | `false`", "auth/KeyDisableSeqnoCheck", "cosmos_checktx.go", "sequence increments")
	f := newRepairM668Fixture(t)
	require.False(t, f.ak.GetParams(f.ctx).DisableSeqnoCheck)
	require.Equal(t, []byte("false"), f.ctx.KVStore(f.paramsKey).Get([]byte("auth/KeyDisableSeqnoCheck")))
	require.Equal(t, []byte("KeyDisableSeqnoCheck"), authtypes.KeyDisableSeqnoCheck)
	priv := secp256k1.GenPrivKeyFromSecret([]byte("m668-sequence"))
	addr := sdk.AccAddress(priv.PubKey().Address())
	f.fund(t, addr, 10000)
	account := f.ak.GetAccount(f.ctx, addr)
	require.NoError(t, account.SetPubKey(priv.PubKey()))
	require.NoError(t, account.SetSequence(7))
	f.ak.SetAccount(f.ctx, account)
	tx, _ := f.signed(t, f.ctx, priv, 200000, 2000, "", 6, nil)
	handler := sdk.ChainAnteDecorators(ante.NewSigVerificationDecorator(f.ak, f.config.SignModeHandler()), ante.NewIncrementSequenceDecorator(f.ak))
	_, err := handler(f.ctx, tx, false)
	require.ErrorIs(t, err, sdkerrors.ErrWrongSequence)
	require.Equal(t, uint64(7), f.ak.GetAccount(f.ctx, addr).GetSequence())
	_, err = nodeante.CheckSignatures(f.ctx, f.config, tx, []authtypes.AccountI{account}, f.ak.GetParams(f.ctx))
	require.ErrorIs(t, err, sdkerrors.ErrWrongSequence)
	params := f.ak.GetParams(f.ctx)
	params.DisableSeqnoCheck = true
	f.ak.SetParams(f.ctx, params)
	require.Equal(t, []byte("true"), f.ctx.KVStore(f.paramsKey).Get([]byte("auth/KeyDisableSeqnoCheck")))
	server := f.gateway(t)
	var queried authtypes.QueryParamsResponse
	require.NoError(t, f.cdc.UnmarshalAsJSON(repairM668Get(t, server, "/cosmos/auth/v1beta1/params", http.StatusOK), &queried))
	require.True(t, queried.Params.DisableSeqnoCheck)
	_, err = nodeante.CheckSignatures(f.ctx, f.config, tx, []authtypes.AccountI{account}, f.ak.GetParams(f.ctx))
	require.NoError(t, err)
	_, err = nodeante.CheckSignatures(f.ctx.WithChainID("different-chain"), f.config, tx, []authtypes.AccountI{account}, f.ak.GetParams(f.ctx))
	require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
	// Direct sign bytes carry the transaction sequence. The explicit comparison is
	// what rejects this valid signature at default settings; it is now bypassed.
	newCtx, err := handler(f.ctx, tx, false)
	require.NoError(t, err)
	require.Equal(t, uint64(8), f.ak.GetAccount(newCtx, addr).GetSequence(), "enabling the flag does not suppress increments")
	wrongChain := f.ctx.WithChainID("different-chain")
	_, err = handler(wrongChain, tx, false)
	require.ErrorIs(t, err, sdkerrors.ErrUnauthorized, "cryptographic chain binding remains active")
}
