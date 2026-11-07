package types_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	layerxkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/keeper"
	types "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	clienttx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz"
	authzkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz/keeper"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

const repair193Now = int64(1700000000000)

type repair193Chain struct {
	t                           *testing.T
	ctx                         sdk.Context
	store                       sdk.CommitMultiStore
	db                          dbm.DB
	dir                         string
	cdc                         *codec.ProtoCodec
	registry                    cdctypes.InterfaceRegistry
	keys                        []*sdk.KVStoreKey
	transient                   *sdk.TransientStoreKey
	keeper                      authzkeeper.Keeper
	bank                        bankkeeper.BaseKeeper
	router                      *baseapp.MsgServiceRouter
	granterKey, granteeKey      *secp256k1.PrivKey
	granter, grantee, recipient sdk.AccAddress
	asset                       [32]byte
}

func newRepair193Chain(t *testing.T) *repair193Chain {
	f := &repair193Chain{t: t}
	f.registry = cdctypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(f.registry)
	authtypes.RegisterInterfaces(f.registry)
	banktypes.RegisterInterfaces(f.registry)
	authz.RegisterInterfaces(f.registry)
	types.RegisterInterfaces(f.registry)
	f.cdc = codec.NewProtoCodec(f.registry)
	for _, name := range []string{authzkeeper.StoreKey, authtypes.StoreKey, banktypes.StoreKey, "params", "repair193host", "repair193auth"} {
		f.keys = append(f.keys, sdk.NewKVStoreKey(name))
	}
	f.transient = sdk.NewTransientStoreKey("repair193transient")
	var err error
	f.dir = t.TempDir()
	f.db, err = dbm.NewGoLevelDB("repair193", f.dir)
	require.NoError(t, err)
	f.open()
	f.granterKey = secp256k1.GenPrivKeyFromSecret([]byte("repair193granter"))
	f.granteeKey = secp256k1.GenPrivKeyFromSecret([]byte("repair193grantee"))
	f.granter = sdk.AccAddress(f.granterKey.PubKey().Address())
	f.grantee = sdk.AccAddress(f.granteeKey.PubKey().Address())
	f.recipient = sdk.AccAddress(bytes.Repeat([]byte{0x19}, 20))
	f.asset = [32]byte{0xa1}
	params := paramskeeper.NewKeeper(f.cdc, codec.NewLegacyAmino(), f.keys[3], f.transient)
	accounts := authkeeper.NewAccountKeeper(f.cdc, f.keys[1], params.Subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, map[string][]string{"repair193funds": {authtypes.Minter}})
	accounts.SetParams(f.ctx, authtypes.DefaultParams())
	f.bank = bankkeeper.NewBaseKeeper(f.cdc, f.keys[2], accounts, params.Subspace(banktypes.ModuleName), nil)
	f.bank.SetParams(f.ctx, banktypes.DefaultParams())
	f.router = baseapp.NewMsgServiceRouter()
	f.router.SetInterfaceRegistry(f.registry)
	banktypes.RegisterMsgServer(f.router, bankkeeper.NewMsgServerImpl(f.bank))
	f.keeper = authzkeeper.NewKeeper(f.keys[0], f.cdc, f.router)
	funds := sdk.NewCoins(sdk.NewInt64Coin(ledgertypes.MirrorDenom(f.asset), 1000))
	require.NoError(t, f.bank.MintCoins(f.ctx, "repair193funds", funds))
	require.NoError(t, f.bank.SendCoinsFromModuleToAccount(f.ctx, "repair193funds", f.granter, funds))
	t.Cleanup(func() { require.NoError(t, f.db.Close()) })
	return f
}

func (f *repair193Chain) open() {
	cms := store.NewCommitMultiStore(f.db)
	for _, key := range f.keys {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, f.db)
	}
	cms.MountStoreWithDB(f.transient, sdk.StoreTypeTransient, f.db)
	require.NoError(f.t, cms.LoadLatestVersion())
	f.store = cms
	f.ctx = sdk.NewContext(cms, tmproto.Header{ChainID: "repair193", Height: 1, Time: time.UnixMilli(repair193Now)}, false)
}

func (f *repair193Chain) reopen() {
	committed := f.store.Commit(true)
	require.Positive(f.t, committed.Version)
	require.NoError(f.t, f.db.Close())
	var err error
	f.db, err = dbm.NewGoLevelDB("repair193", f.dir)
	require.NoError(f.t, err)
	f.open()
	require.Equal(f.t, committed, f.store.LastCommitID())
	f.keeper = authzkeeper.NewKeeper(f.keys[0], f.cdc, f.router)
}

func (f *repair193Chain) authorization() types.LayerXScopeAuthorization {
	return types.LayerXScopeAuthorization{
		ModuleMask: 1 << types.BankScopeModule, OrdinalMin: uint32(types.BankScopeOrdinal), OrdinalMax: uint32(types.BankScopeOrdinal),
		AssetId: f.asset[:], MaxPerActivity: "10", MaxTotal: "20", RevocationEpoch: 41, GrantId: bytes.Repeat([]byte{0x33}, 32),
		MsgTypeUrl: sdk.MsgTypeURL(&banktypes.MsgSend{}), Kind: 3, NotBefore: uint64(repair193Now), NotAfter: uint64(repair193Now + 1000),
	}
}

func (f *repair193Chain) send(amount int64) *banktypes.MsgSend {
	return banktypes.NewMsgSend(f.granter, f.recipient, sdk.NewCoins(sdk.NewInt64Coin(ledgertypes.MirrorDenom(f.asset), amount)))
}

// signed transports the actual protobuf transaction, verifies its signature, and
// returns the decoded message that the real authz service consumes.
func (f *repair193Chain) signed(msg sdk.Msg, private *secp256k1.PrivKey) sdk.Msg {
	f.t.Helper()
	cfg := authtx.NewTxConfig(f.cdc, []signing.SignMode{signing.SignMode_SIGN_MODE_DIRECT})
	builder := cfg.NewTxBuilder()
	require.NoError(f.t, builder.SetMsgs(msg))
	builder.SetGasLimit(1000000)
	mode := cfg.SignModeHandler().DefaultMode()
	require.NoError(f.t, builder.SetSignatures(signing.SignatureV2{PubKey: private.PubKey(), Data: &signing.SingleSignatureData{SignMode: mode}}))
	data := authsigning.SignerData{ChainID: f.ctx.ChainID()}
	sig, err := clienttx.SignWithPrivKey(mode, data, builder, private, cfg, 0)
	require.NoError(f.t, err)
	require.NoError(f.t, builder.SetSignatures(sig))
	wire, err := cfg.TxEncoder()(builder.GetTx())
	require.NoError(f.t, err)
	decoded, err := cfg.TxDecoder()(wire)
	require.NoError(f.t, err)
	signatures, err := decoded.(authsigning.SigVerifiableTx).GetSignaturesV2()
	require.NoError(f.t, err)
	require.Len(f.t, signatures, 1)
	require.NoError(f.t, authsigning.VerifySignature(signatures[0].PubKey, data, signatures[0].Data, cfg.SignModeHandler(), decoded))
	messages := decoded.GetMsgs()
	require.Len(f.t, messages, 1)
	require.Equal(f.t, sdk.AccAddress(private.PubKey().Address()), messages[0].GetSigners()[0])
	return messages[0]
}

func (f *repair193Chain) grant(a types.LayerXScopeAuthorization) {
	f.t.Helper()
	require.NoError(f.t, a.ValidateBasic())
	msg, err := authz.NewMsgGrant(f.granter, f.grantee, &a, time.UnixMilli(int64(a.NotAfter)))
	require.NoError(f.t, err)
	require.NoError(f.t, msg.ValidateBasic())
	decoded := f.signed(msg, f.granterKey).(*authz.MsgGrant)
	_, err = f.keeper.Grant(sdk.WrapSDKContext(f.ctx), decoded)
	require.NoError(f.t, err)
}

func (f *repair193Chain) exec(msgs ...sdk.Msg) error {
	f.t.Helper()
	exec := authz.NewMsgExec(f.grantee, msgs)
	decoded := f.signed(&exec, f.granteeKey).(*authz.MsgExec)
	cache, write := types.WithGrantorRevocation(f.ctx, f.granter, 41).CacheContext()
	_, err := f.keeper.Exec(sdk.WrapSDKContext(cache), decoded)
	if err == nil {
		write()
	}
	return err
}

func (f *repair193Chain) stored() *types.LayerXScopeAuthorization {
	a, _ := f.keeper.GetCleanAuthorization(f.ctx, f.grantee, f.granter, sdk.MsgTypeURL(&banktypes.MsgSend{}))
	require.NotNil(f.t, a)
	return a.(*types.LayerXScopeAuthorization)
}

func TestRepair_193_unscoped_message_bypasses_policy(t *testing.T) {
	f := newRepair193Chain(t)
	base := f.authorization()
	cases := []struct {
		name   string
		change func(*types.LayerXScopeAuthorization)
	}{
		{"module", func(a *types.LayerXScopeAuthorization) { a.ModuleMask = 1 << 2 }},
		{"ordinal", func(a *types.LayerXScopeAuthorization) { a.OrdinalMin++; a.OrdinalMax++ }},
		{"asset", func(a *types.LayerXScopeAuthorization) { a.AssetId = bytes.Repeat([]byte{2}, 32) }},
		{"activity_limit", func(a *types.LayerXScopeAuthorization) { a.MaxPerActivity = "4" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := base
			tc.change(&a)
			f.grant(a)
			require.ErrorIs(t, f.exec(f.send(5)), types.ErrAuthScope)
			require.Empty(t, f.stored().SpentTotal)
			require.True(t, f.bank.GetBalance(f.ctx, f.recipient, ledgertypes.MirrorDenom(f.asset)).IsZero())
		})
	}
	f.grant(base)
	require.NoError(t, f.exec(f.send(5)))
	require.Equal(t, "5", f.stored().SpentTotal)
	require.Equal(t, sdk.NewInt(5), f.bank.GetBalance(f.ctx, f.recipient, ledgertypes.MirrorDenom(f.asset)).Amount)
	// Unsupported denominations and multiple assets cannot inherit the first coin's policy.
	unsupported := banktypes.NewMsgSend(f.granter, f.recipient, sdk.NewCoins(sdk.NewInt64Coin("upax", 1)))
	require.ErrorIs(t, f.exec(unsupported), types.ErrAuthScope)
	multi := f.send(1)
	multi.Amount = multi.Amount.Add(sdk.NewInt64Coin("upax", 1))
	require.ErrorIs(t, f.exec(multi), types.ErrAuthScope)
	require.Equal(t, "5", f.stored().SpentTotal)
}

func TestRepair_193_grant_validity_not_enforced(t *testing.T) {
	f := newRepair193Chain(t)
	a := f.authorization()
	f.grant(a)
	for _, tc := range []struct {
		offset  int64
		allowed bool
	}{{-1, false}, {0, true}, {999, true}, {1000, false}} {
		f.ctx = f.ctx.WithBlockTime(time.UnixMilli(repair193Now + tc.offset))
		err := f.exec(f.send(1))
		if tc.allowed {
			require.NoError(t, err)
		} else {
			require.ErrorIs(t, err, types.ErrAuthScope)
		}
	}
	require.Equal(t, "2", f.stored().SpentTotal)
	// Even a separately stored SDK expiration beyond NotAfter cannot extend authority.
	require.NoError(t, f.keeper.SaveGrant(f.ctx, f.grantee, f.granter, &a, time.UnixMilli(repair193Now+2000)))
	require.ErrorIs(t, f.exec(f.send(1)), types.ErrAuthScope)
}

func TestRepair_193_period_budget_not_enforced(t *testing.T) {
	f := newRepair193Chain(t)
	a := f.authorization()
	a.PeriodLength = 100
	a.PeriodStart = a.NotBefore
	a.MaxPerPeriod = "10"
	purpose, err := types.BankSendPurpose(f.send(6))
	require.NoError(t, err)
	a.PurposeHash = purpose[:]
	f.grant(a)
	require.ErrorIs(t, f.exec(f.send(5)), types.ErrAuthScope)
	redirected := f.send(6)
	redirected.ToAddress = sdk.AccAddress(bytes.Repeat([]byte{0x20}, 20)).String()
	require.ErrorIs(t, f.exec(redirected), types.ErrAuthScope)
	require.Empty(t, f.stored().SpentTotal)
	require.NoError(t, f.exec(f.send(6)))
	require.ErrorIs(t, f.exec(f.send(6)), types.ErrAuthScope)
	require.Equal(t, "6", f.stored().SpentThisPeriod)
	f.reopen()
	require.Equal(t, "6", f.stored().SpentThisPeriod)
	require.Equal(t, a.PeriodStart, f.stored().PeriodStart)
	require.ErrorIs(t, f.exec(f.send(6)), types.ErrAuthScope)
	f.ctx = f.ctx.WithBlockTime(time.UnixMilli(repair193Now + 99))
	require.ErrorIs(t, f.exec(f.send(6)), types.ErrAuthScope)
	f.ctx = f.ctx.WithBlockTime(time.UnixMilli(repair193Now + 100))
	require.NoError(t, f.exec(f.send(6)))
	require.Equal(t, "12", f.stored().SpentTotal)
	require.Equal(t, "6", f.stored().SpentThisPeriod)
	require.Equal(t, a.PeriodStart+100, f.stored().PeriodStart)
	// Two actions share a transaction cache: a later refusal rolls back the first charge and transfer.
	a = f.authorization()
	a.PeriodLength = 100
	a.MaxPerPeriod = "10"
	f.grant(a)
	before := f.bank.GetBalance(f.ctx, f.recipient, ledgertypes.MirrorDenom(f.asset))
	require.ErrorIs(t, f.exec(f.send(6), f.send(6)), types.ErrAuthScope)
	require.Empty(t, f.stored().SpentTotal)
	require.Equal(t, before, f.bank.GetBalance(f.ctx, f.recipient, ledgertypes.MirrorDenom(f.asset)))
	// A failed bank handler must also roll back authorization updates.
	a.MaxPerActivity = "2000"
	a.MaxTotal = "3000"
	a.MaxPerPeriod = "3000"
	f.grant(a)
	require.Error(t, f.exec(f.send(1500)))
	require.Empty(t, f.stored().SpentTotal)
}

// issueKernelGrant submits a signed registration and capability activity to the
// real kernel, then reads back its canonical persisted grant for both consumers.
func (f *repair193Chain) issueKernelGrant(total uint64) (*lxphost.Host, lxphost.AuthorityGrant) {
	host, err := lxphost.NewHost(lxphost.Config{StoreKey: f.keys[4], NetworkID: 1, ParameterVersion: 1, TimestampWindowMs: 60000, GasLimit: 10000000})
	require.NoError(f.t, err)
	f.t.Cleanup(host.Close)
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{71}, 32))
	var public, delegate [32]byte
	copy(public[:], private.Public().(ed25519.PublicKey))
	copy(delegate[:], bytes.Repeat([]byte{72}, 32))
	did := []byte(evmtypes.LayerXDid(public))
	id, err := lxphost.DIDID(did)
	require.NoError(f.t, err)
	execute := func(typ uint32, accountSeq, globalSeq uint64, payload []byte) lxphost.Receipt {
		activity := lxphost.Activity{ProtocolVersion: 3, Type: typ, ActorDID: did, AuthorityKey: public, AccountSequence: accountSeq, NotBefore: uint64(repair193Now - 1000), NotAfter: uint64(repair193Now + 1000), Payload: payload, IdempotencyKey: sha256.Sum256(payload)}
		hash, err := host.SigningHash(activity)
		require.NoError(f.t, err)
		activity.Signature = ed25519.Sign(private, hash[:])
		receipt, err := host.Execute(lxphost.WithSequence(f.ctx, globalSeq), activity, lxphost.KernelAuthority{Actor: id, Principal: id, VerifiedKey: public})
		require.NoError(f.t, err)
		require.Zero(f.t, receipt.ResultCode)
		return receipt
	}
	execute(0x00070001, 0, 41, append(append([]byte{0x71, 1, 0, 2}, id[:]...), public[:]...))
	body := []byte{0, 1, 0x20, 1, 1}
	field := func(b []byte) { body = binary.BigEndian.AppendUint32(body, uint32(len(b))); body = append(body, b...) }
	u64 := func(n uint64) { body = binary.BigEndian.AppendUint64(body, n) }
	u128 := func(n uint64) { u64(0); u64(n) }
	field(id[:])
	field(id[:])
	body = append(body, 3)
	field(delegate[:])
	u64(1 << types.BankScopeModule)
	body = binary.BigEndian.AppendUint16(body, types.BankScopeOrdinal)
	body = binary.BigEndian.AppendUint16(body, types.BankScopeOrdinal)
	field(f.asset[:])
	u128(10)
	u128(total)
	u128(0)
	u64(100)
	u128(15)
	u128(0)
	u64(uint64(repair193Now))
	purpose, err := types.BankSendPurpose(f.send(5))
	require.NoError(f.t, err)
	field(purpose[:])
	u64(uint64(repair193Now))
	u64(uint64(repair193Now + 1000))
	u64(41)
	body = append(body, 0)
	u64(0)
	field(make([]byte, 64))
	payload, err := types.EncodeCapabilityPayload(body)
	require.NoError(f.t, err)
	receipt := execute(0x00070008, 1, 42, payload)
	events, err := layerxkeeper.AuthEvents(receipt)
	require.NoError(f.t, err)
	var grantID [32]byte
	for _, event := range events {
		if event.Name == "CapabilityGranted" {
			grantID = event.GrantID
		}
	}
	grant, err := host.ReadGrant(f.ctx, grantID)
	require.NoError(f.t, err)
	return host, grant
}

func TestRepair_193_zero_total_disagrees_with_kernel(t *testing.T) {
	for _, total := range []uint64{0, 20} {
		t.Run(sdk.NewIntFromUint64(total).String(), func(t *testing.T) {
			f := newRepair193Chain(t)
			host, grant := f.issueKernelGrant(total)
			scope := grant.Scope
			a := f.authorization()
			a.GrantId = grant.GrantID[:]
			a.MaxTotal = scope.MaximumTotal.String()
			a.PeriodLength = scope.PeriodLength
			a.MaxPerPeriod = scope.MaximumPerPeriod.String()
			a.PeriodStart = scope.PeriodStart
			a.SpentThisPeriod = scope.SpentThisPeriod.String()
			a.PurposeHash = scope.PurposeHash[:]
			kernel := layerxkeeper.NewKeeper(f.cdc, f.keys[5], host, nil, nil, "")
			// SaveGrant is the production mirror transport; it does not apply MsgGrant.ValidateBasic.
			require.NoError(t, f.keeper.SaveGrant(f.ctx, f.grantee, f.granter, &a, time.UnixMilli(int64(a.NotAfter))))
			reason, err := kernel.CheckActivity(f.ctx, grant.GrantID, uint32(types.BankScopeModule)<<16|uint32(types.BankScopeOrdinal), f.asset, sdk.NewInt(5))
			require.NoError(t, err)
			require.Empty(t, reason)
			require.NoError(t, f.exec(f.send(5)))
			require.Equal(t, "5", f.stored().SpentTotal)
			reason, err = kernel.CheckActivity(f.ctx, grant.GrantID, uint32(types.BankScopeModule)<<16|uint32(types.BankScopeOrdinal), f.asset, sdk.NewInt(11))
			require.NoError(t, err)
			require.NotEmpty(t, reason)
			require.ErrorIs(t, f.exec(f.send(11)), types.ErrAuthScope)
			// Overflow is refused even when zero disables the lifetime cap.
			candidate := a
			candidate.SpentTotal = "340282366920938463463374607431768211455"
			ctx := types.WithGrantorRevocation(f.ctx, f.granter, 41)
			response, err := candidate.Accept(ctx, f.send(5))
			require.ErrorIs(t, err, types.ErrAuthScope)
			require.False(t, response.Accept)
			// Finite lifetime caps are checked independently of the per-activity cap.
			if total != 0 {
				candidate = a
				candidate.SpentTotal = "16"
				response, err = candidate.Accept(ctx, f.send(5))
				require.ErrorIs(t, err, types.ErrAuthScope)
				require.False(t, response.Accept)
			}
		})
	}
}
