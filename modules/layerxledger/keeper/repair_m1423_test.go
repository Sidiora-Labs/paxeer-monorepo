package keeper_test

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	tokenfactorytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

type repair1423Fixture struct {
	app                   *app.App
	ctx                   sdk.Context
	payer, payee          utils.TestAcct
	identity              ledgerViewActor
	actor, recipient      ledgerkeeper.Actor
	asset, salt, from, to [32]byte
	denom                 string
	index                 int
	closed                bool
}

func newRepair1423Fixture(t *testing.T) *repair1423Fixture {
	t.Helper()
	a := app.Setup(t, false, true, false)
	f := &repair1423Fixture{app: a, payer: utils.NewSigner(), payee: utils.NewSigner(), identity: newLedgerViewActor(t, 143)}
	t.Cleanup(func() {
		if !f.closed {
			require.NoError(t, a.Close())
		}
	})
	f.ctx = a.GetContextForDeliverTx(nil).WithChainID("pax-test").WithBlockHeight(100).
		WithBlockTime(time.Unix(1_800_000_000, 0).UTC())
	a.UpgradeKeeper.SetDone(f.ctx, app.MergeUpgrade)
	a.RefreshPrecompiles(f.ctx)
	address := common.HexToAddress(layerxledger.LedgerAddress)
	require.Contains(t, a.EvmKeeper.CustomPrecompiles(f.ctx), address)
	provider := a.GetPrecompileKeepers().(*app.PrecompileKeepers)
	require.Same(t, a.LayerXLedgerKeeper, provider.LayerXLedgerK())
	for _, signer := range []utils.TestAcct{f.payer, f.payee} {
		a.EvmKeeper.SetAddressMapping(f.ctx, signer.AccountAddress, signer.EvmAddress)
		funding := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1_000_000_000))
		require.NoError(t, a.BankKeeper.MintCoins(f.ctx, "mint", funding))
		require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(f.ctx, "mint", signer.AccountAddress, funding))
	}
	bind := func(signer utils.TestAcct, identity ledgerViewActor) ledgerkeeper.Actor {
		message, err := evmtypes.LayerXBindMessage(a.EvmKeeper.ChainID(f.ctx), signer.EvmAddress, 0)
		require.NoError(t, err)
		var signature [ed25519.SignatureSize]byte
		copy(signature[:], ed25519.Sign(identity.private, message))
		_, err = a.EvmKeeper.BindLayerX(f.ctx, signer.EvmAddress, identity.key, signature)
		require.NoError(t, err)
		actor, err := a.LayerXLedgerKeeper.Actor(f.ctx, signer.EvmAddress)
		require.NoError(t, err)
		return actor
	}
	f.actor = bind(f.payer, f.identity)
	f.recipient = bind(f.payee, newLedgerViewActor(t, 144))
	f.salt = [32]byte{0x14, 0x23}
	material := append([]byte("LX:ASSET:v1"), f.actor.Authority.Actor[:]...)
	f.asset = sha256.Sum256(append(material, f.salt[:]...))
	var err error
	f.from, err = ledgerkeeper.AccountID(f.actor.DIDKey, f.asset)
	require.NoError(t, err)
	f.to, err = ledgerkeeper.AccountID(f.recipient.DIDKey, f.asset)
	require.NoError(t, err)
	f.denom, err = tokenfactorytypes.GetTokenDenom(authtypes.NewModuleAddress(types.ModuleName).String(),
		fmt.Sprintf("%043s", new(big.Int).SetBytes(f.asset[:]).Text(62)))
	require.NoError(t, err)
	return f
}

func (f *repair1423Fixture) call(t *testing.T, signer utils.TestAcct, status uint64, method string, args ...interface{}) []byte {
	t.Helper()
	return f.callWithRevert(t, signer, status, "LXP_SEQUENCE_MISMATCH", method, args...)
}

func (f *repair1423Fixture) callWithRevert(t *testing.T, signer utils.TestAcct, status uint64, revertReason, method string, args ...interface{}) []byte {
	t.Helper()
	address := common.HexToAddress(layerxledger.LedgerAddress)
	input, err := layerxledger.GetABI().Pack(method, args...)
	require.NoError(t, err)
	nonce := f.app.EvmKeeper.GetNonce(f.ctx, signer.EvmAddress)
	signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce: nonce, GasPrice: big.NewInt(500_000_000_000), Gas: 2_000_000,
		To: &address, Value: big.NewInt(0), Data: input,
	}), ethtypes.LatestSignerForChainID(f.app.EvmKeeper.ChainID(f.ctx)), signer.EvmPrivateKey)
	require.NoError(t, err)
	txData, err := ethtx.NewLegacyTx(signed)
	require.NoError(t, err)
	msg, err := evmtypes.NewMsgEVMTransaction(txData)
	require.NoError(t, err)
	builder := f.app.GetTxConfig().NewTxBuilder()
	require.NoError(t, builder.SetMsgs(msg))
	builder.SetGasLimit(signed.Gas())
	encoded, err := f.app.GetTxConfig().TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	result := f.deliver(t, encoded)
	require.NotNil(t, result.EvmTxInfo, result.Log)
	if status == ethtypes.ReceiptStatusSuccessful {
		require.Zero(t, result.Code, result.Log)
		require.Empty(t, result.EvmTxInfo.VmError)
	} else {
		require.NotZero(t, result.Code)
		require.Contains(t, result.EvmTxInfo.VmError, vm.ErrExecutionReverted.Error())
	}
	receipt, err := f.app.EvmKeeper.GetTransientReceipt(f.ctx, signed.Hash(), uint64(f.index-1))
	require.NoError(t, err)
	require.EqualValues(t, status, receipt.Status, receipt.VmError)
	require.Equal(t, nonce+1, f.app.EvmKeeper.GetNonce(f.ctx, signer.EvmAddress))
	if status == ethtypes.ReceiptStatusFailed {
		require.Empty(t, receipt.Logs)
		var messages sdk.TxMsgData
		require.NoError(t, messages.Unmarshal(result.Data))
		require.Len(t, messages.Data, 1)
		var response evmtypes.MsgEVMTransactionResponse
		require.NoError(t, response.Unmarshal(messages.Data[0].Data))
		reason, err := abi.UnpackRevert(response.ReturnData)
		require.NoError(t, err)
		require.Equal(t, revertReason, reason)
	}
	return encoded
}

func (f *repair1423Fixture) deliver(t *testing.T, encoded []byte) abci.ResponseDeliverTx {
	t.Helper()
	tx, err := f.app.GetTxConfig().TxDecoder()(encoded)
	require.NoError(t, err)
	ctx := f.ctx.WithTxIndex(f.index).WithEventManager(sdk.NewEventManager())
	f.index++
	return f.app.DeliverTx(ctx, abci.RequestDeliverTxV2{Tx: encoded}, tx, sha256.Sum256(encoded))
}

func (f *repair1423Fixture) register(t *testing.T) {
	t.Helper()
	f.call(t, f.payer, ethtypes.ReceiptStatusSuccessful, "register", f.asset, f.salt,
		"RPR", "Issuer recovery", uint8(6), big.NewInt(1000), uint8(types.AssetIssuerNative), []byte{})
}

func (f *repair1423Fixture) fund(t *testing.T) {
	t.Helper()
	f.register(t)
	f.call(t, f.payer, ethtypes.ReceiptStatusSuccessful, "openAccount", f.asset)
	f.call(t, f.payee, ethtypes.ReceiptStatusSuccessful, "openAccount", f.asset)
	f.call(t, f.payer, ethtypes.ReceiptStatusSuccessful, "mint", f.asset, f.from, big.NewInt(250))
	f.balance(t, 250, 0, 0)
}

func (f *repair1423Fixture) balance(t *testing.T, from, to int64, sequence uint64) {
	t.Helper()
	for _, item := range []struct {
		id     [32]byte
		amount int64
	}{{f.from, from}, {f.to, to}} {
		actual, err := f.app.LayerXLedgerKeeper.BalanceOf(f.ctx, item.id, f.asset)
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(item.amount), actual)
		require.Equal(t, actual, f.app.BankKeeper.GetBalance(f.ctx, lxphost.BankAddress(item.id), types.MirrorDenom(f.asset)).Amount)
	}
	next, err := f.app.LayerXLedgerKeeper.NextSequenceByAccountID(f.ctx, f.from, f.asset)
	require.NoError(t, err)
	require.Equal(t, sequence, next)
}

func (f *repair1423Fixture) envelope(t *testing.T, sequence uint64, reference [32]byte) []byte {
	t.Helper()
	material := append(append(append([]byte(nil), f.from[:]...), f.to[:]...), f.asset[:]...)
	var amount [16]byte
	big.NewInt(40).FillBytes(amount[:])
	material = append(material, amount[:]...)
	hash, err := codec.DomainHash(codec.DomainContextHash, append(material, reference[:]...))
	require.NoError(t, err)
	payload, err := types.EncodeSendPayload(types.SendArgs{
		From: f.from, To: f.to, Asset: f.asset, Amount: sdk.NewInt(40), Sequence: sequence,
		IdempotencyKey: reference, ExpiresAt: uint64(f.ctx.BlockTime().UnixMilli()) + 1000,
		ContextHash: hash, PublicKey: f.actor.DIDKey,
		NetworkID: uint32(f.app.EvmKeeper.ChainID(f.ctx).Uint64()), ProtocolVersion: codec.StateCommitmentProtocolVersion,
	})
	require.NoError(t, err)
	authorization := len(payload) - 167
	message := append(append([]byte(nil), payload[:2]...), payload[4:authorization]...)
	message = append(message, payload[authorization:authorization+33]...)
	message = append(message, payload[authorization+129:]...)
	digest, err := codec.DomainHash(codec.DomainSignaturePreimage, message)
	require.NoError(t, err)
	copy(payload[authorization+65:authorization+129], ed25519.Sign(f.identity.private, digest[:]))
	require.NoError(t, types.ValidateSendEnvelope(payload))
	return payload
}

func (f *repair1423Fixture) pauseState(t *testing.T, paused bool) {
	t.Helper()
	asset, err := f.app.LayerXLedgerKeeper.GetAsset(f.ctx, f.asset)
	require.NoError(t, err)
	require.Equal(t, f.actor.Authority.Actor, asset.IssuerDID)
	require.Equal(t, paused, asset.Paused)
	response, err := f.app.TokenFactoryKeeper.DenomPaused(sdk.WrapSDKContext(f.ctx),
		&tokenfactorytypes.QueryDenomPausedRequest{Denom: f.denom})
	require.NoError(t, err)
	require.Equal(t, paused, response.Paused)
	require.Equal(t, string(f.actor.DID), response.IssuerDid)
	key := append([]byte{2}, f.asset[:]...)
	require.Equal(t, []byte(f.denom), f.ctx.KVStore(f.app.GetKey(types.StoreKey)).Get(key))
}

func TestRepairM_1423_IssuerBindingNode(t *testing.T) {
	f := newRepair1423Fixture(t)
	f.register(t)
	f.pauseState(t, false)
	f.call(t, f.payer, ethtypes.ReceiptStatusSuccessful, "pause", f.asset)
	f.pauseState(t, true)
	f.call(t, f.payer, ethtypes.ReceiptStatusSuccessful, "unpause", f.asset)
	f.pauseState(t, false)
}

func TestRepairM_1423_IssuerBindingRecovery(t *testing.T) {
	f := newRepair1423Fixture(t)
	f.register(t)
	f.call(t, f.payer, ethtypes.ReceiptStatusSuccessful, "pause", f.asset)
	f.pauseState(t, true)
	f.app.SetDeliverStateToCommit()
	_, err := f.app.Commit(context.Background())
	require.NoError(t, err)
	exported, err := f.app.ExportAppStateAndValidators(false, nil)
	require.NoError(t, err)
	require.NoError(t, f.app.Close())
	f.closed = true
	restored := app.Setup(t, true, false, false)
	t.Cleanup(func() { require.NoError(t, restored.Close()) })
	_, err = restored.InitChain(context.Background(), &abci.RequestInitChain{
		Time: f.ctx.BlockTime(), ChainId: f.ctx.ChainID(), InitialHeight: exported.Height,
		ConsensusParams: exported.ConsensusParams, AppStateBytes: exported.AppState,
	})
	require.NoError(t, err)
	f.app = restored
	f.ctx = restored.NewContext(false, tmproto.Header{ChainID: "pax-test", Height: exported.Height, Time: f.ctx.BlockTime()})
	restored.UpgradeKeeper.SetDone(f.ctx, app.MergeUpgrade)
	restored.RefreshPrecompiles(f.ctx)
	f.pauseState(t, true)
	actor, err := restored.LayerXLedgerKeeper.Actor(f.ctx, f.payer.EvmAddress)
	require.NoError(t, err)
	require.Equal(t, f.actor, actor)
	// The resumed node's concrete keeper consumes the rebuilt map.
	_, err = restored.LayerXLedgerKeeper.UnpauseAsset(f.ctx, actor, f.asset, [32]byte{7})
	require.NoError(t, err)
	f.pauseState(t, false)
	_, err = restored.LayerXLedgerKeeper.PauseAsset(f.ctx, actor, f.asset, [32]byte{8})
	require.NoError(t, err)
	f.pauseState(t, true)
}

func TestRepairM_1423_IssuerBindingRejectsInconsistentGenesis(t *testing.T) {
	f := newRepair1423Fixture(t)
	f.register(t)
	hostGenesis, err := lxphost.ExportStoreGenesis(f.ctx, f.app.GetKey(app.LayerXHostStoreKey))
	require.NoError(t, err)
	encoded, err := json.Marshal(hostGenesis)
	require.NoError(t, err)
	var transported lxphost.StoreGenesis
	require.NoError(t, json.Unmarshal(encoded, &transported))
	tokenGenesis := f.app.TokenFactoryKeeper.ExportGenesis(f.ctx)
	ledgerGenesis := f.app.LayerXLedgerKeeper.ExportGenesis(f.ctx)
	for _, kind := range []string{"issuer", "pause", "unknown_asset", "noncanonical_denom"} {
		t.Run(kind, func(t *testing.T) {
			a := app.Setup(t, false, false, false)
			t.Cleanup(func() { require.NoError(t, a.Close()) })
			ctx := a.NewContext(false, tmproto.Header{ChainID: "pax-test", Time: f.ctx.BlockTime()})
			require.NoError(t, lxphost.InitStoreGenesis(ctx, a.GetKey(app.LayerXHostStoreKey), transported))
			bz, err := json.Marshal(tokenGenesis)
			require.NoError(t, err)
			var genesis tokenfactorytypes.GenesisState
			require.NoError(t, json.Unmarshal(bz, &genesis))
			require.Len(t, genesis.FactoryDenoms, 1)
			switch kind {
			case "issuer":
				genesis.FactoryDenoms[0].AuthorityMetadata.IssuerDid = string(f.recipient.DID)
			case "pause":
				genesis.PausedDenoms = []string{f.denom}
			case "unknown_asset":
				genesis.FactoryDenoms[0].Denom, err = tokenfactorytypes.GetTokenDenom(authtypes.NewModuleAddress(types.ModuleName).String(), fmt.Sprintf("%043s", "1"))
				require.NoError(t, err)
			case "noncanonical_denom":
				genesis.FactoryDenoms[0].Denom, err = tokenfactorytypes.GetTokenDenom(authtypes.NewModuleAddress(types.ModuleName).String(), "other")
				require.NoError(t, err)
			}
			a.TokenFactoryKeeper.InitGenesis(ctx, genesis)
			before := ledgerViewStore(t, ctx, a.GetKey(types.StoreKey))
			require.Panics(t, func() { a.LayerXLedgerKeeper.InitGenesis(ctx, *ledgerGenesis) })
			require.Equal(t, before, ledgerViewStore(t, ctx, a.GetKey(types.StoreKey)))
		})
	}
}

func TestRepairM_1423_PublicAccountSend(t *testing.T) {
	TestSendMovesBalance(t)
	TestSendRefusals(t)
}

func TestRepairM_1423_PayExactBalancesAndRollback(t *testing.T) {
	TestPayExact(t)
	TestPayExactExpired(t)
}

func TestRepairM_1423_EVMNonceAndConsumedSequence(t *testing.T) {
	f := newRepair1423Fixture(t)
	f.fund(t)
	original := f.call(t, f.payer, ethtypes.ReceiptStatusSuccessful, "send", f.envelope(t, 0, [32]byte{1}))
	f.balance(t, 210, 40, 1)
	hostBefore := ledgerViewStore(t, f.ctx, f.app.GetKey(app.LayerXHostStoreKey))
	bankBefore := ledgerViewStore(t, f.ctx, f.app.GetKey(banktypes.StoreKey))
	nonce := f.app.EvmKeeper.GetNonce(f.ctx, f.payer.EvmAddress)
	replay := f.deliver(t, original)
	require.NotZero(t, replay.Code, "consumed EVM nonce must refuse")
	require.Equal(t, nonce, f.app.EvmKeeper.GetNonce(f.ctx, f.payer.EvmAddress))
	require.Equal(t, hostBefore, ledgerViewStore(t, f.ctx, f.app.GetKey(app.LayerXHostStoreKey)))
	require.Equal(t, bankBefore, ledgerViewStore(t, f.ctx, f.app.GetKey(banktypes.StoreKey)))
	// A fresh EVM transaction still cannot consume an old SEND source sequence.
	f.call(t, f.payer, ethtypes.ReceiptStatusFailed, "send", f.envelope(t, 0, [32]byte{2}))
	require.Equal(t, hostBefore, ledgerViewStore(t, f.ctx, f.app.GetKey(app.LayerXHostStoreKey)))
	// Decision 2337 also refuses a consumed reference at the next valid sequence.
	f.callWithRevert(t, f.payer, ethtypes.ReceiptStatusFailed, "LXP_IDEMPOTENT_REPLAY", "send", f.envelope(t, 1, [32]byte{1}))
	require.Equal(t, hostBefore, ledgerViewStore(t, f.ctx, f.app.GetKey(app.LayerXHostStoreKey)))
	f.balance(t, 210, 40, 1)
	f.call(t, f.payer, ethtypes.ReceiptStatusSuccessful, "payExact", f.from, f.to, f.asset,
		big.NewInt(10), uint64(f.ctx.BlockTime().UnixMilli())+1000, [32]byte{3})
	f.balance(t, 200, 50, 2)
	f.call(t, f.payer, ethtypes.ReceiptStatusSuccessful, "send", f.envelope(t, 2, [32]byte{4}))
	f.balance(t, 160, 90, 3)
}

func TestRepairM_1423_PayExactEVMReferenceReplay(t *testing.T) {
	f := newRepair1423Fixture(t)
	f.fund(t)
	expires := uint64(f.ctx.BlockTime().UnixMilli()) + 1000
	reference := [32]byte{0x14, 0x23}
	original := f.call(t, f.payer, ethtypes.ReceiptStatusSuccessful, "payExact", f.from, f.to, f.asset, big.NewInt(10), expires, reference)
	f.balance(t, 240, 10, 1)
	hostBefore := ledgerViewStore(t, f.ctx, f.app.GetKey(app.LayerXHostStoreKey))
	bankBefore := ledgerViewStore(t, f.ctx, f.app.GetKey(banktypes.StoreKey))
	nonce := f.app.EvmKeeper.GetNonce(f.ctx, f.payer.EvmAddress)
	replay := f.deliver(t, original)
	require.NotZero(t, replay.Code, "consumed EVM nonce must refuse")
	require.Equal(t, nonce, f.app.EvmKeeper.GetNonce(f.ctx, f.payer.EvmAddress))
	require.Equal(t, hostBefore, ledgerViewStore(t, f.ctx, f.app.GetKey(app.LayerXHostStoreKey)))
	require.Equal(t, bankBefore, ledgerViewStore(t, f.ctx, f.app.GetKey(banktypes.StoreKey)))
	f.callWithRevert(t, f.payer, ethtypes.ReceiptStatusFailed, "LXP_IDEMPOTENT_REPLAY", "payExact", f.from, f.to, f.asset, big.NewInt(10), expires, reference)
	require.Equal(t, hostBefore, ledgerViewStore(t, f.ctx, f.app.GetKey(app.LayerXHostStoreKey)))
	f.balance(t, 240, 10, 1)
	f.call(t, f.payer, ethtypes.ReceiptStatusSuccessful, "payExact", f.from, f.to, f.asset, big.NewInt(10), expires, [32]byte{0x14, 0x24})
	f.balance(t, 230, 20, 2)
}

func TestRepairM_1423_SendNextSequenceKeyReuse(t *testing.T) {
	TestSendNextSequenceKeyReuse(t)
}

func TestRepairM_1423_PayExactNextSequenceKeyReuse(t *testing.T) {
	TestPayExactNextSequenceKeyReuse(t)
}

func TestRepairM_1423_IssuerBindingRollback(t *testing.T) {
	f := newRepair1423Fixture(t)
	f.register(t)
	require.NoError(t, f.app.TokenFactoryKeeper.SetDenomPausedByIssuer(f.ctx, f.denom, string(f.actor.DID), true))
	before := map[string]map[string][]byte{}
	for _, name := range []string{app.LayerXHostStoreKey, types.StoreKey, tokenfactorytypes.StoreKey, banktypes.StoreKey, authtypes.StoreKey} {
		before[name] = ledgerViewStore(t, f.ctx, f.app.GetKey(name))
	}
	events := f.ctx.EventManager().Events()
	_, err := f.app.LayerXLedgerKeeper.PauseAsset(f.ctx, f.actor, f.asset, [32]byte{9})
	require.ErrorIs(t, err, tokenfactorytypes.ErrPauseUnchanged)
	for name, records := range before {
		require.Equal(t, records, ledgerViewStore(t, f.ctx, f.app.GetKey(name)), name)
	}
	require.Equal(t, events, f.ctx.EventManager().Events())
	asset, err := f.app.LayerXLedgerKeeper.GetAsset(f.ctx, f.asset)
	require.NoError(t, err)
	require.False(t, asset.Paused)
}
