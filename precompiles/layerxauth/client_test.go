package layerxauth_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"math/big"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/addr"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxauth"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestAuthEVMTransactionReceipt(t *testing.T) {
	a := app.Setup(t, false, true, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	ctx := a.GetContextForDeliverTx(nil).WithChainID("pax-test").
		WithBlockHeight(11).WithBlockTime(time.Unix(1_800_000_000, 0).UTC())
	a.UpgradeKeeper.SetDone(ctx, app.MergeUpgrade)
	authAddress := common.HexToAddress(layerxauth.AuthAddress)
	require.Equal(t, common.HexToAddress("0x101C"), authAddress)
	require.Contains(t, a.EvmKeeper.CustomPrecompiles(ctx), authAddress)
	sender, unbound := utils.NewSigner(), utils.NewSigner()
	for _, account := range []utils.TestAcct{sender, unbound} {
		a.EvmKeeper.SetAddressMapping(ctx, account.AccountAddress, account.EvmAddress)
		funds := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 10_000_000_000))
		require.NoError(t, a.BankKeeper.MintCoins(ctx, "mint", funds))
		require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", account.AccountAddress, funds))
	}

	txIndex := 0
	deliver := func(account utils.TestAcct, to common.Address, input []byte, expectedCode uint32) (*evmtypes.Receipt, *evmtypes.MsgEVMTransactionResponse) {
		t.Helper()
		signer := ethtypes.LatestSignerForChainID(a.EvmKeeper.ChainID(ctx))
		signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
			Nonce:    a.EvmKeeper.GetNonce(ctx, account.EvmAddress),
			GasPrice: big.NewInt(500_000_000_000), Gas: 2_000_000,
			To: &to, Value: big.NewInt(0), Data: input,
		}), signer, account.EvmPrivateKey)
		require.NoError(t, err)
		require.Equal(t, uint8(ethtypes.LegacyTxType), signed.Type())
		recovered, err := ethtypes.Sender(signer, signed)
		require.NoError(t, err)
		require.Equal(t, account.EvmAddress, recovered)
		txData, err := ethtx.NewLegacyTx(signed)
		require.NoError(t, err)
		msg, err := evmtypes.NewMsgEVMTransaction(txData)
		require.NoError(t, err)
		builder := a.GetTxConfig().NewTxBuilder()
		require.NoError(t, builder.SetMsgs(msg))
		builder.SetGasLimit(signed.Gas())
		encoded, err := a.GetTxConfig().TxEncoder()(builder.GetTx())
		require.NoError(t, err)
		deliveryCtx := ctx.WithTxIndex(txIndex).WithEventManager(sdk.NewEventManager())
		result := a.DeliverTx(deliveryCtx, abci.RequestDeliverTxV2{Tx: encoded}, builder.GetTx(), sha256.Sum256(encoded))
		require.Equal(t, expectedCode, result.Code, result.Log)
		var messages sdk.TxMsgData
		require.NoError(t, messages.Unmarshal(result.Data))
		require.Len(t, messages.Data, 1)
		var response evmtypes.MsgEVMTransactionResponse
		require.NoError(t, response.Unmarshal(messages.Data[0].Data))
		require.Equal(t, signed.Hash().Hex(), response.Hash)
		receipt, err := a.EvmKeeper.GetTransientReceipt(ctx, signed.Hash(), uint64(txIndex))
		require.NoError(t, err)
		require.NotNil(t, receipt)
		require.Equal(t, signed.Hash().Hex(), receipt.TxHashHex)
		require.Equal(t, uint32(ethtypes.LegacyTxType), receipt.TxType)
		require.Equal(t, account.EvmAddress.Hex(), receipt.From)
		require.Equal(t, to.Hex(), receipt.To)
		require.Equal(t, uint64(ctx.BlockHeight()), receipt.BlockNumber)
		require.Equal(t, uint32(txIndex), receipt.TransactionIndex)
		require.Positive(t, receipt.GasUsed)
		require.Equal(t, response.GasUsed, receipt.GasUsed)
		expectedVMError, expectedReceiptError := "", ""
		if expectedCode == sdkerrors.ErrEVMVMError.ABCICode() {
			expectedVMError = vm.ErrExecutionReverted.Error()
			expectedReceiptError = expectedVMError + "|LXP_AUTH_SCOPE"
		}
		require.Equal(t, expectedVMError, response.VmError)
		require.Equal(t, expectedReceiptError, receipt.VmError)
		txIndex++
		return receipt, &response
	}

	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{73}, ed25519.SeedSize))
	var didKey [32]byte
	copy(didKey[:], private.Public().(ed25519.PublicKey))
	message, err := evmtypes.LayerXBindMessage(a.EvmKeeper.ChainID(ctx), sender.EvmAddress, 0)
	require.NoError(t, err)
	addressPrecompile, err := addr.NewPrecompile(a.GetPrecompileKeepers())
	require.NoError(t, err)
	bindInput, err := addressPrecompile.GetABI().Pack("bindLayerX", didKey, ed25519.Sign(private, message))
	require.NoError(t, err)
	boundReceipt, _ := deliver(sender, common.HexToAddress(addr.AddrAddress), bindInput, 0)
	require.Equal(t, uint32(ethtypes.ReceiptStatusSuccessful), boundReceipt.Status)
	require.Empty(t, boundReceipt.VmError)
	boundKey, found := a.EvmKeeper.GetLayerXDid(ctx, sender.EvmAddress)
	require.True(t, found)
	require.Equal(t, didKey, boundKey)
	actor, err := a.LayerXAuthKeeper.Actor(ctx, sender.EvmAddress)
	require.NoError(t, err)
	timestamp := uint64(ctx.BlockTime().UnixMilli())
	registration := append(append([]byte{0x71, 1, 0, 2}, actor.Authority.Actor[:]...), didKey[:]...)
	registered, err := a.LayerXHost.Execute(lxphost.WithSequence(ctx, 41), lxphost.Activity{
		ProtocolVersion: 3, Type: 0x00070001, ActorDID: actor.DID, AuthorityKey: didKey,
		NotBefore: timestamp, NotAfter: timestamp, IdempotencyKey: [32]byte{0x41}, Payload: registration,
	}, actor.Authority)
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), registered.ResultCode)
	sessionPrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{74}, ed25519.SeedSize))
	var sessionKey [32]byte
	copy(sessionKey[:], sessionPrivate.Public().(ed25519.PublicKey))
	fixture := &grantFixture{actor: actor, timestamp: timestamp}
	body := fixture.body(2, sessionKey)
	grantID, err := codec.DomainHash(codec.DomainAuthorityHash, body)
	require.NoError(t, err)
	authABI := layerxauth.GetABI()
	actionKey := [32]byte{0x33}
	input, err := authABI.Pack("grantSession", body, uint64(100), actionKey)
	require.NoError(t, err)
	receipt, response := deliver(sender, authAddress, input, 0)
	require.Equal(t, uint32(ethtypes.ReceiptStatusSuccessful), receipt.Status, receipt.VmError)
	require.Empty(t, receipt.VmError)
	output, err := authABI.Unpack("grantSession", response.ReturnData)
	require.NoError(t, err)
	require.Equal(t, []interface{}{grantID}, output)
	require.NotEmpty(t, receipt.LogsBloom)
	chunks := (len(body) + 255) / 256
	require.Len(t, receipt.Logs, chunks+2)
	var reconstructed []byte
	for i, log := range receipt.Logs {
		require.Equal(t, authAddress.Hex(), log.Address)
		require.Len(t, log.Topics, 2)
		event, err := authABI.EventByID(common.HexToHash(log.Topics[0]))
		require.NoError(t, err)
		values, err := event.Inputs.NonIndexed().Unpack(log.Data)
		require.NoError(t, err)
		switch {
		case i == 0:
			require.Equal(t, "SessionGranted", event.Name)
			require.Equal(t, common.Hash(grantID).Hex(), log.Topics[1])
			require.Len(t, values, 1)
			summary := values[0].([]byte)
			require.Len(t, summary, 209)
			require.Equal(t, []byte("LXGS2"), summary[:5])
			require.Equal(t, grantID[:], summary[5:37])
			require.Equal(t, actor.Authority.Actor[:], summary[37:69])
			require.Equal(t, didKey[:], summary[69:101])
			require.Equal(t, actionKey[:], summary[101:133])
			require.Equal(t, sessionKey[:], summary[133:165])
			require.Equal(t, uint64(100), binary.BigEndian.Uint64(summary[165:173]))
			stored, err := a.LayerXAuthKeeper.SessionKey(ctx, grantID)
			require.NoError(t, err)
			require.Equal(t, stored, summary)
		case i <= chunks:
			require.Equal(t, "GrantBody", event.Name)
			require.Equal(t, common.Hash(grantID).Hex(), log.Topics[1])
			require.Len(t, values, 3)
			require.Equal(t, uint8(5), values[0])
			require.Equal(t, uint8(i-1), values[1])
			chunk := values[2].([]byte)
			require.Len(t, chunk, min(256, len(body)-(i-1)*256))
			reconstructed = append(reconstructed, chunk...)
		default:
			require.Equal(t, "IdentityStateUpdated", event.Name)
			require.Equal(t, common.Hash(actor.Authority.Actor).Hex(), log.Topics[1])
			require.Len(t, values, 1)
			require.NotEmpty(t, values[0].([]byte))
		}
	}
	require.Equal(t, body, reconstructed)
	_, found = a.EvmKeeper.GetLayerXDid(ctx, unbound.EvmAddress)
	require.False(t, found)
	failed, refusal := deliver(unbound, authAddress, input, sdkerrors.ErrEVMVMError.ABCICode())
	require.Equal(t, uint32(ethtypes.ReceiptStatusFailed), failed.Status)
	require.Empty(t, failed.Logs)
	reason, err := abi.UnpackRevert(refusal.ReturnData)
	require.NoError(t, err)
	require.Equal(t, "LXP_AUTH_SCOPE", reason)
	require.Contains(t, failed.VmError, reason)
	stored, err := a.LayerXAuthKeeper.GetGrant(ctx, grantID)
	require.NoError(t, err)
	require.Equal(t, body, stored)
}
