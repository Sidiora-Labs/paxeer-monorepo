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
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxauth"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestRepair_146_LifecycleReceipts(t *testing.T) {
	a := app.Setup(t, false, true, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	ctx := a.GetContextForDeliverTx(nil).WithChainID("pax-test").WithBlockHeight(11).
		WithBlockTime(time.Unix(1_800_000_000, 0).UTC())
	a.UpgradeKeeper.SetDone(ctx, app.MergeUpgrade)
	address := common.HexToAddress(layerxauth.AuthAddress)
	require.Contains(t, a.EvmKeeper.CustomPrecompiles(ctx), address)
	contractABI := layerxauth.GetABI()
	sender, unbound := utils.NewSigner(), utils.NewSigner()
	for _, account := range []utils.TestAcct{sender, unbound} {
		a.EvmKeeper.SetAddressMapping(ctx, account.AccountAddress, account.EvmAddress)
		funds := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 10_000_000_000))
		require.NoError(t, a.BankKeeper.MintCoins(ctx, "mint", funds))
		require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", account.AccountAddress, funds))
	}
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{146}, ed25519.SeedSize))
	var public [32]byte
	copy(public[:], private.Public().(ed25519.PublicKey))
	message, err := evmtypes.LayerXBindMessage(a.EvmKeeper.ChainID(ctx), sender.EvmAddress, 0)
	require.NoError(t, err)
	var signature [64]byte
	copy(signature[:], ed25519.Sign(private, message))
	_, err = a.EvmKeeper.BindLayerX(ctx, sender.EvmAddress, public, signature)
	require.NoError(t, err)
	actor, err := a.LayerXAuthKeeper.Actor(ctx, sender.EvmAddress)
	require.NoError(t, err)
	hostKey := a.GetKey(app.LayerXHostStoreKey)
	identityKey := append([]byte{1, 0, 7}, actor.Authority.Actor[:]...)
	txIndex := 0
	deliver := func(account utils.TestAcct, payload []byte, key [32]byte, reason string) (*evmtypes.Receipt, *evmtypes.MsgEVMTransactionResponse) {
		t.Helper()
		input, err := contractABI.Pack("recordLifecycle", payload, key)
		require.NoError(t, err)
		signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
			Nonce:    a.EvmKeeper.GetNonce(ctx, account.EvmAddress),
			GasPrice: big.NewInt(500_000_000_000), Gas: 2_000_000,
			To: &address, Value: big.NewInt(0), Data: input,
		}), ethtypes.LatestSignerForChainID(a.EvmKeeper.ChainID(ctx)), account.EvmPrivateKey)
		require.NoError(t, err)
		txData, err := ethtx.NewLegacyTx(signed)
		require.NoError(t, err)
		msg, err := evmtypes.NewMsgEVMTransaction(txData)
		require.NoError(t, err)
		builder := a.GetTxConfig().NewTxBuilder()
		require.NoError(t, builder.SetMsgs(msg))
		builder.SetGasLimit(signed.Gas())
		encoded, err := a.GetTxConfig().TxEncoder()(builder.GetTx())
		require.NoError(t, err)
		tx, err := a.GetTxConfig().TxDecoder()(encoded)
		require.NoError(t, err)
		at := ctx.WithTxIndex(txIndex).WithEventManager(sdk.NewEventManager())
		result := a.DeliverTx(at, abci.RequestDeliverTxV2{Tx: encoded}, tx, sha256.Sum256(encoded))
		wantCode := uint32(0)
		if reason != "" {
			wantCode = sdkerrors.ErrEVMVMError.ABCICode()
		}
		require.Equal(t, wantCode, result.Code, result.Log)
		var messages sdk.TxMsgData
		require.NoError(t, messages.Unmarshal(result.Data))
		require.Len(t, messages.Data, 1)
		var response evmtypes.MsgEVMTransactionResponse
		require.NoError(t, response.Unmarshal(messages.Data[0].Data))
		receipt, err := a.EvmKeeper.GetTransientReceipt(ctx, signed.Hash(), uint64(txIndex))
		require.NoError(t, err)
		require.NotNil(t, receipt)
		require.Equal(t, signed.Hash().Hex(), response.Hash)
		require.Equal(t, signed.Hash().Hex(), receipt.TxHashHex)
		require.Equal(t, uint32(txIndex), receipt.TransactionIndex)
		require.Equal(t, uint64(ctx.BlockHeight()), receipt.BlockNumber)
		require.Equal(t, account.EvmAddress.Hex(), receipt.From)
		require.Equal(t, address.Hex(), receipt.To)
		require.Positive(t, receipt.GasUsed)
		require.Equal(t, response.GasUsed, receipt.GasUsed)
		if reason == "" {
			require.Equal(t, uint32(ethtypes.ReceiptStatusSuccessful), receipt.Status)
			require.Empty(t, response.VmError)
			require.Empty(t, receipt.VmError)
		} else {
			require.Equal(t, uint32(ethtypes.ReceiptStatusFailed), receipt.Status)
			require.Equal(t, vm.ErrExecutionReverted.Error(), response.VmError)
			require.Equal(t, vm.ErrExecutionReverted.Error()+"|"+reason, receipt.VmError)
			decoded, err := abi.UnpackRevert(response.ReturnData)
			require.NoError(t, err)
			require.Equal(t, reason, decoded)
			require.Empty(t, receipt.Logs)
		}
		txIndex++
		return receipt, &response
	}

	registration := append([]byte{0x71, 1, 0, 2}, actor.Authority.Actor[:]...)
	registration = append(registration, public[:]...)
	root := [32]byte{0x61}
	recovery := append([]byte{0x71, 3, 0, 3}, actor.Authority.Actor[:]...)
	recovery = append(recovery, root[:]...)
	recovery = binary.BigEndian.AppendUint16(recovery, 2)
	expected := make([]byte, 223)
	copy(expected, "LXGI1")
	copy(expected[5:37], actor.Authority.Actor[:])
	copy(expected[37:69], public[:])
	var registrationSequence uint64
	for i, tc := range []struct {
		name    string
		payload []byte
	}{
		{"registration", registration},
		{"recovery", recovery},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := [32]byte{0x91, byte(i + 1)}
			before := hostSnapshot(t, ctx, hostKey)
			branch, _ := ctx.CacheContext()
			produced, err := a.LayerXAuthKeeper.RecordLifecycle(branch, actor, tc.payload, key)
			require.NoError(t, err)
			require.Zero(t, produced.ResultCode)
			require.Len(t, produced.Events, 1)
			require.Equal(t, uint16(0x7110), produced.Events[0].Code)
			require.Equal(t, before, hostSnapshot(t, ctx, hostKey))
			receipt, response := deliver(sender, tc.payload, key, "")
			values, err := contractABI.Unpack("recordLifecycle", response.ReturnData)
			require.NoError(t, err)
			require.Equal(t, []interface{}{produced.GlobalSequence}, values)
			if i == 0 {
				registrationSequence = produced.GlobalSequence
				require.Positive(t, registrationSequence)
				binary.BigEndian.PutUint64(expected[69:77], registrationSequence)
			} else {
				require.Greater(t, produced.GlobalSequence, registrationSequence)
				copy(expected[77:109], root[:])
				binary.BigEndian.PutUint16(expected[109:111], 2)
				binary.BigEndian.PutUint64(expected[175:183], 1)
			}
			binary.BigEndian.PutUint64(expected[215:223], produced.GlobalSequence)
			require.Equal(t, expected, produced.Events[0].Body)
			require.Len(t, receipt.Logs, 1)
			require.NotEmpty(t, receipt.LogsBloom)
			log := receipt.Logs[0]
			require.Equal(t, address.Hex(), log.Address)
			topic := crypto.Keccak256Hash([]byte("IdentityStateUpdated(bytes32,bytes)"))
			require.Equal(t, []string{topic.Hex(), common.Hash(actor.Authority.Actor).Hex()}, log.Topics)
			event, err := contractABI.EventByID(topic)
			require.NoError(t, err)
			decoded, err := event.Inputs.NonIndexed().Unpack(log.Data)
			require.NoError(t, err)
			require.Equal(t, []interface{}{produced.Events[0].Body}, decoded)
			require.Equal(t, expected, ctx.KVStore(hostKey).Get(identityKey))
			identity, err := a.LayerXHost.ReadIdentityState(ctx, actor.DID)
			require.NoError(t, err)
			require.Equal(t, actor.Authority.Actor, identity.DIDID)
			require.Equal(t, public, identity.PrimaryKey)
			require.Equal(t, uint64(i+1), identity.NextSequence)
			require.Equal(t, registrationSequence, identity.RevocationSequence)
			if i == 1 {
				require.Equal(t, root, identity.RecoveryRoot)
				require.Equal(t, uint16(2), identity.RecoveryThreshold)
			}
		})
	}

	invalidRecovery := bytes.Clone(recovery)
	clear(invalidRecovery[36:68])
	for i, tc := range []struct {
		name    string
		account utils.TestAcct
		payload []byte
		reason  string
	}{
		{"duplicate registration", sender, registration, "LXP_SEQUENCE_REUSED"},
		{"invalid recovery root", sender, invalidRecovery, "LXP_AUTH_SCOPE"},
		{"truncated recovery", sender, recovery[:len(recovery)-1], "LXP_NON_CANONICAL"},
		{"unbound caller", unbound, recovery, "LXP_AUTH_SCOPE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := hostSnapshot(t, ctx, hostKey)
			deliver(tc.account, tc.payload, [32]byte{0x92, byte(i + 1)}, tc.reason)
			require.Equal(t, before, hostSnapshot(t, ctx, hostKey))
			require.Equal(t, expected, ctx.KVStore(hostKey).Get(identityKey))
		})
	}
}
