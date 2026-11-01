package layerxledger_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"math/big"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/addr"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestLedgerEVMTransactionReceipt(t *testing.T) {
	a := app.Setup(t, false, true, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	ctx := a.GetContextForDeliverTx(nil).WithChainID("pax-test").WithBlockHeight(100).
		WithBlockTime(time.Unix(1_800_000_000, 0).UTC())
	a.UpgradeKeeper.SetDone(ctx, app.MergeUpgrade)
	a.RefreshPrecompiles(ctx)
	address := common.HexToAddress(layerxledger.LedgerAddress)
	require.Equal(t, common.HexToAddress("0x101A"), address)
	require.Contains(t, a.EvmKeeper.CustomPrecompiles(ctx), address)
	ledgerABI := layerxledger.GetABI()
	sender, recipient, unbound := utils.NewSigner(), utils.NewSigner(), utils.NewSigner()
	for _, signer := range []utils.TestAcct{sender, recipient, unbound} {
		a.EvmKeeper.SetAddressMapping(ctx, signer.AccountAddress, signer.EvmAddress)
		funding := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1_000_000_000))
		require.NoError(t, a.BankKeeper.MintCoins(ctx, "mint", funding))
		require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", signer.AccountAddress, funding))
	}

	var index uint64
	deliver := func(signer utils.TestAcct, to common.Address, input []byte, status uint32) (*evmtypes.Receipt, *evmtypes.MsgEVMTransactionResponse) {
		t.Helper()
		const gas uint64 = 1_000_000
		signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
			Nonce: a.EvmKeeper.GetNonce(ctx, signer.EvmAddress), GasPrice: big.NewInt(500_000_000_000),
			Gas: gas, To: &to, Value: big.NewInt(0), Data: input,
		}), ethtypes.LatestSignerForChainID(a.EvmKeeper.ChainID(ctx)), signer.EvmPrivateKey)
		require.NoError(t, err)
		require.Equal(t, uint8(ethtypes.LegacyTxType), signed.Type())
		recovered, err := ethtypes.Sender(ethtypes.LatestSignerForChainID(a.EvmKeeper.ChainID(ctx)), signed)
		require.NoError(t, err)
		require.Equal(t, signer.EvmAddress, recovered)
		txData, err := ethtx.NewLegacyTx(signed)
		require.NoError(t, err)
		msg, err := evmtypes.NewMsgEVMTransaction(txData)
		require.NoError(t, err)
		builder := a.GetTxConfig().NewTxBuilder()
		require.NoError(t, builder.SetMsgs(msg))
		builder.SetGasLimit(gas)
		encoded, err := a.GetTxConfig().TxEncoder()(builder.GetTx())
		require.NoError(t, err)
		tx, err := a.GetTxConfig().TxDecoder()(encoded)
		require.NoError(t, err)
		at := ctx.WithTxIndex(int(index)).WithEventManager(sdk.NewEventManager())
		result := a.DeliverTx(at, abci.RequestDeliverTxV2{Tx: encoded}, tx, sha256.Sum256(encoded))
		require.NotNil(t, result.EvmTxInfo, result.Log)
		if status == uint32(ethtypes.ReceiptStatusSuccessful) {
			require.Zero(t, result.Code, result.Log)
			require.Empty(t, result.EvmTxInfo.VmError)
		} else {
			require.NotZero(t, result.Code)
			require.Contains(t, result.EvmTxInfo.VmError, vm.ErrExecutionReverted.Error())
		}
		var data sdk.TxMsgData
		require.NoError(t, data.Unmarshal(result.Data))
		require.Len(t, data.Data, 1)
		var response evmtypes.MsgEVMTransactionResponse
		require.NoError(t, response.Unmarshal(data.Data[0].Data))
		require.Equal(t, signed.Hash().Hex(), response.Hash)
		receipt, err := a.EvmKeeper.GetTransientReceipt(ctx, signed.Hash(), index)
		require.NoError(t, err)
		require.Equal(t, status, receipt.Status, receipt.VmError)
		require.Equal(t, uint32(ethtypes.LegacyTxType), receipt.TxType)
		require.Equal(t, signed.Hash().Hex(), receipt.TxHashHex)
		require.Equal(t, signer.EvmAddress.Hex(), receipt.From)
		require.Equal(t, to.Hex(), receipt.To)
		require.Equal(t, uint64(ctx.BlockHeight()), receipt.BlockNumber)
		require.Equal(t, uint32(index), receipt.TransactionIndex)
		require.Positive(t, receipt.GasUsed)
		require.LessOrEqual(t, receipt.GasUsed, gas)
		require.Equal(t, receipt.GasUsed, response.GasUsed)
		require.Equal(t, signed.Nonce()+1, a.EvmKeeper.GetNonce(ctx, signer.EvmAddress))
		index++
		return receipt, &response
	}
	call := func(signer utils.TestAcct, method string, args ...interface{}) {
		t.Helper()
		input, err := ledgerABI.Pack(method, args...)
		require.NoError(t, err)
		receipt, response := deliver(signer, address, input, uint32(ethtypes.ReceiptStatusSuccessful))
		require.Empty(t, receipt.VmError)
		require.Empty(t, response.VmError)
	}
	addrPrecompile, err := addr.NewPrecompile(a.GetPrecompileKeepers())
	require.NoError(t, err)
	bind := func(signer utils.TestAcct, seedByte byte) ledgerkeeper.Actor {
		t.Helper()
		var seed [ed25519.SeedSize]byte
		seed[0] = seedByte
		private := ed25519.NewKeyFromSeed(seed[:])
		var public [32]byte
		copy(public[:], private.Public().(ed25519.PublicKey))
		message, err := evmtypes.LayerXBindMessage(a.EvmKeeper.ChainID(ctx), signer.EvmAddress, 0)
		require.NoError(t, err)
		input, err := addrPrecompile.GetABI().Pack(addr.BindLayerXMethod, public, ed25519.Sign(private, message))
		require.NoError(t, err)
		deliver(signer, common.HexToAddress(addr.AddrAddress), input, uint32(ethtypes.ReceiptStatusSuccessful))
		bound, found := a.EvmKeeper.GetLayerXDid(ctx, signer.EvmAddress)
		require.True(t, found)
		require.Equal(t, public, bound)
		actor, err := a.LayerXLedgerKeeper.Actor(ctx, signer.EvmAddress)
		require.NoError(t, err)
		return actor
	}
	payer := bind(sender, 1)
	payee := bind(recipient, 2)
	sendEnvelope := func(from, to, asset [32]byte, amount int64, sequence uint64, key [32]byte) []byte {
		t.Helper()
		material := append(append(append([]byte(nil), from[:]...), to[:]...), asset[:]...)
		var encoded [16]byte
		big.NewInt(amount).FillBytes(encoded[:])
		material = append(material, encoded[:]...)
		contextHash, err := codec.DomainHash(codec.DomainContextHash, append(material, key[:]...))
		require.NoError(t, err)
		payload, err := types.EncodeSendPayload(types.SendArgs{
			From: from, To: to, Asset: asset, Amount: sdk.NewInt(amount), Sequence: sequence,
			IdempotencyKey: key, ExpiresAt: uint64(ctx.BlockTime().UnixMilli()) + 1000,
			ContextHash: contextHash, PublicKey: payer.DIDKey,
			NetworkID: uint32(a.EvmKeeper.ChainID(ctx).Uint64()), ProtocolVersion: codec.StateCommitmentProtocolVersion,
		})
		require.NoError(t, err)
		var seed [ed25519.SeedSize]byte
		seed[0] = 1
		private := ed25519.NewKeyFromSeed(seed[:])
		authorization := len(payload) - 167
		message := append(append([]byte(nil), payload[:2]...), payload[4:authorization]...)
		message = append(message, payload[authorization:authorization+33]...)
		message = append(message, payload[authorization+129:]...)
		digest, err := codec.DomainHash(codec.DomainSignaturePreimage, message)
		require.NoError(t, err)
		copy(payload[authorization+65:authorization+129], ed25519.Sign(private, digest[:]))
		require.NoError(t, types.ValidateSendEnvelope(payload))
		return payload
	}
	salt := [32]byte{0x71}
	material := append([]byte("LX:ASSET:v1"), payer.Authority.Actor[:]...)
	asset := sha256.Sum256(append(material, salt[:]...))
	h := &harness{ctx: ctx, host: a.LayerXHost, hostKey: a.GetKey(app.LayerXHostStoreKey)}
	from := transferMainAccount(t, h, payer, asset)
	call(sender, "register", asset, salt, "RCT", "Receipt test", uint8(6), big.NewInt(1000), uint8(types.AssetIssuerNative), []byte{})
	call(sender, "openAccount", asset)
	call(recipient, "openAccount", asset)
	holding, err := ledgerkeeper.AccountID(payer.DIDKey, asset)
	require.NoError(t, err)
	to, err := ledgerkeeper.AccountID(payee.DIDKey, asset)
	require.NoError(t, err)
	call(sender, "mint", asset, holding, big.NewInt(250))
	fundingKey := [32]byte{0x72}
	fundingEnvelope := sendEnvelope(holding, from, asset, 250, 0, fundingKey)
	fundingCtx, err := ledgerkeeper.WithSequence(ctx)
	require.NoError(t, err)
	sequence, err := a.LayerXHost.ReadIdentitySequence(fundingCtx, payer.DID)
	require.NoError(t, err)
	now := uint64(ctx.BlockTime().UnixMilli())
	fundingReceipt, err := a.LayerXHost.Execute(fundingCtx, lxphost.Activity{
		ProtocolVersion: 3, Type: 0x00010005, ActorDID: payer.DID, AuthorityKey: payer.DIDKey,
		AccountSequence: sequence, NotBefore: now, NotAfter: now,
		IdempotencyKey: fundingKey, Payload: fundingEnvelope,
	}, payer.Authority)
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), fundingReceipt.ResultCode)
	require.NoError(t, a.LayerXLedgerKeeper.Mirror(ctx, 0x00010005, fundingReceipt))
	for account, want := range map[[32]byte]int64{holding: 0, from: 250, to: 0} {
		balance, err := a.LayerXLedgerKeeper.BalanceOf(ctx, account, asset)
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(want), balance)
	}
	envelope := sendEnvelope(from, to, asset, 40, 0, [32]byte{0x73})
	input, err := ledgerABI.Pack("send", envelope)
	require.NoError(t, err)
	receipt, response := deliver(sender, address, input, uint32(ethtypes.ReceiptStatusSuccessful))
	require.Empty(t, receipt.VmError)
	require.Empty(t, response.VmError)
	require.Empty(t, response.ReturnData)
	require.Len(t, receipt.Logs, 1)
	require.NotEmpty(t, receipt.LogsBloom)
	log := receipt.Logs[0]
	require.Equal(t, address.Hex(), log.Address)
	require.False(t, log.Synthetic)
	require.Len(t, log.Topics, 4)
	event, err := ledgerABI.EventByID(common.HexToHash(log.Topics[0]))
	require.NoError(t, err)
	require.Equal(t, "Transfer", event.Name)
	fields := make(map[string]interface{})
	require.NoError(t, event.Inputs.NonIndexed().UnpackIntoMap(fields, log.Data))
	var indexed abi.Arguments
	for _, argument := range event.Inputs {
		if argument.Indexed {
			indexed = append(indexed, argument)
		}
	}
	require.NoError(t, abi.ParseTopicsIntoMap(fields, indexed, []common.Hash{
		common.HexToHash(log.Topics[1]), common.HexToHash(log.Topics[2]), common.HexToHash(log.Topics[3]),
	}))
	require.Equal(t, map[string]interface{}{
		"from": from, "to": to, "assetId": asset, "amount": big.NewInt(40), "kind": uint8(5),
	}, fields)
	assertBalances := func() {
		t.Helper()
		for account, want := range map[[32]byte]int64{from: 210, to: 40} {
			balance, err := a.LayerXLedgerKeeper.BalanceOf(ctx, account, asset)
			require.NoError(t, err)
			require.Equal(t, sdk.NewInt(want), balance)
		}
	}
	assertBalances()
	_, found := a.EvmKeeper.GetLayerXDid(ctx, unbound.EvmAddress)
	require.False(t, found)
	hostBefore := storeSnapshot(t, ctx.KVStore(a.GetKey(app.LayerXHostStoreKey)))
	failed, reverted := deliver(unbound, address, input, uint32(ethtypes.ReceiptStatusFailed))
	require.Empty(t, failed.Logs)
	require.Contains(t, failed.VmError, vm.ErrExecutionReverted.Error())
	require.Contains(t, reverted.VmError, vm.ErrExecutionReverted.Error())
	reason, err := abi.UnpackRevert(reverted.ReturnData)
	require.NoError(t, err)
	require.Equal(t, "LXP_UNAUTHORIZED_DEBIT", reason)
	require.Contains(t, failed.VmError, reason)
	require.Equal(t, hostBefore, storeSnapshot(t, ctx.KVStore(a.GetKey(app.LayerXHostStoreKey))))
	assertBalances()
}
