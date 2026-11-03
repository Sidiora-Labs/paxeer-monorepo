// Proves: layerxauth.NewMergePrecompile/NewPrecompile -> App.DeliverTx and EVM receipts -> paxeer::decode_block, Store::commit and api::route.
//
// Repairs the proof gap in services/platform/hosted/indexer/tests/history_position.rs:
// holder_records_carry_their_latest_state_by_holder previously only consumed fabricated
// GrantBody/GrantRevoked bytes. The companion consumer now consumes these actual EVM
// receipts, closes and reopens SQLite, and checks duplicate-ingestion refusal.
// Existing behavior outside scope: services/platform/hosted/indexer/src/store.rs,
// Store::commit refuses duplicate units through its uniqueness constraints; the
// consumer asserts that refusal preserves the original holder projection.
// The Go ABI, address, AuthKeepers.LayerXAuthK and both constructors are checked against
// the served application surface before signed writes and an unbound-signer refusal.
package layerxauth_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
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

// GrantBody is the decoded non-indexed payload of the real producer log.
type GrantBody struct {
	Sub   uint8
	Chunk uint8
	Body  []byte
}

// GrantRevoked is the decoded canonical revocation record emitted by 0x101C.
type GrantRevoked struct{ Record []byte }

func TestProofLayerxauth0x101c(t *testing.T) {
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
		expectedVMError := ""
		if expectedCode == sdkerrors.ErrEVMVMError.ABCICode() {
			expectedVMError = vm.ErrExecutionReverted.Error()
		}
		require.Equal(t, expectedVMError, response.VmError)
		if expectedVMError == "" {
			require.Empty(t, receipt.VmError)
		} else {
			require.Contains(t, receipt.VmError, expectedVMError)
		}
		txIndex++
		return receipt, &response
	}

	keepers, ok := a.GetPrecompileKeepers().(layerxauth.AuthKeepers)
	require.True(t, ok)
	require.Same(t, a.LayerXAuthKeeper, keepers.LayerXAuthK())
	merge, err := layerxauth.NewMergePrecompile(a.GetPrecompileKeepers(), precompiles.MergeUpgrade)
	require.NoError(t, err)
	direct, err := layerxauth.NewPrecompile(a.GetPrecompileKeepers())
	require.NoError(t, err)
	authABI := layerxauth.GetABI()
	require.Equal(t, authAddress, merge.Address())
	require.Equal(t, authAddress, direct.Address())
	require.Equal(t, authABI, merge.GetABI())
	require.Equal(t, authABI, direct.GetABI())
	rawABI, err := os.ReadFile("abi.json")
	require.NoError(t, err)
	parsed, err := abi.JSON(bytes.NewReader(rawABI))
	require.NoError(t, err)
	require.Equal(t, authABI, parsed)
	require.Equal(t, "GrantBody(bytes32,uint8,uint8,bytes)", authABI.Events["GrantBody"].Sig)
	require.Equal(t, "GrantRevoked(bytes32,bytes)", authABI.Events["GrantRevoked"].Sig)
	for _, name := range []string{"GrantBody", "GrantRevoked"} {
		event := authABI.Events[name]
		require.True(t, event.Inputs[0].Indexed)
		for _, input := range event.Inputs[1:] {
			require.False(t, input.Indexed)
		}
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
	actor, err := a.LayerXAuthKeeper.Actor(ctx, sender.EvmAddress)
	require.NoError(t, err)
	timestamp := uint64(ctx.BlockTime().UnixMilli())
	registration := append(append([]byte{0x71, 1, 0, 2}, actor.Authority.Actor[:]...), didKey[:]...)
	lifecycleInput, err := authABI.Pack("recordLifecycle", registration, [32]byte{0x41})
	require.NoError(t, err)
	_, registrationResponse := deliver(sender, authAddress, lifecycleInput, 0)
	registered, err := authABI.Unpack("recordLifecycle", registrationResponse.ReturnData)
	require.NoError(t, err)
	sequence := registered[0].(uint64)
	require.Positive(t, sequence)
	sessionPrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{74}, ed25519.SeedSize))
	var sessionKey [32]byte
	copy(sessionKey[:], sessionPrivate.Public().(ed25519.PublicKey))
	fixture := &grantFixture{actor: actor, timestamp: timestamp}
	body := fixture.body(2, sessionKey)
	const revocationOffset = 114 + 180 + 2*8
	binary.BigEndian.PutUint64(body[revocationOffset:revocationOffset+8], sequence)
	grantID, err := codec.DomainHash(codec.DomainAuthorityHash, body)
	require.NoError(t, err)
	actionKey := [32]byte{0x33}
	input, err := authABI.Pack("grantSession", body, uint64(100), actionKey)
	require.NoError(t, err)
	granted, response := deliver(sender, authAddress, input, 0)
	output, err := authABI.Unpack("grantSession", response.ReturnData)
	require.NoError(t, err)
	require.Equal(t, []interface{}{grantID}, output)
	chunks := (len(body) + 255) / 256
	require.Len(t, granted.Logs, chunks+2)
	var reconstructed []byte
	for i, log := range granted.Logs[1 : chunks+1] {
		require.Equal(t, authAddress.Hex(), log.Address)
		require.Equal(t, []string{authABI.Events["GrantBody"].ID.Hex(), common.Hash(grantID).Hex()}, log.Topics)
		values, err := authABI.Events["GrantBody"].Inputs.NonIndexed().Unpack(log.Data)
		require.NoError(t, err)
		event := GrantBody{Sub: values[0].(uint8), Chunk: values[1].(uint8), Body: values[2].([]byte)}
		require.Equal(t, uint8(5), event.Sub)
		require.Equal(t, uint8(i), event.Chunk)
		require.Len(t, event.Body, min(256, len(body)-i*256))
		reconstructed = append(reconstructed, event.Body...)
	}
	require.Equal(t, body, reconstructed)
	view := func(name string, args ...interface{}) []interface{} {
		t.Helper()
		data, err := authABI.Pack(name, args...)
		require.NoError(t, err)
		_, result := deliver(sender, authAddress, data, 0)
		values, err := authABI.Unpack(name, result.ReturnData)
		require.NoError(t, err)
		return values
	}
	require.Equal(t, []interface{}{body, false, uint64(0)}, view("getGrant", grantID))
	require.Equal(t, []interface{}{true, ""}, view("isLive", grantID, uint64(42), timestamp))
	summary := view("sessionKey", grantID)[0].([]byte)
	require.Len(t, summary, 209)
	require.Equal(t, sessionKey[:], summary[133:165])
	failed, refusal := deliver(unbound, authAddress, input, sdkerrors.ErrEVMVMError.ABCICode())
	require.Equal(t, uint32(ethtypes.ReceiptStatusFailed), failed.Status)
	require.Empty(t, failed.Logs)
	reason, err := abi.UnpackRevert(refusal.ReturnData)
	require.NoError(t, err)
	require.Equal(t, "LXP_AUTH_SCOPE", reason)
	require.Equal(t, []interface{}{body, false, uint64(0)}, view("getGrant", grantID))
	// Repeating the producer is refused before any additional logs or state are committed.
	duplicate, replay := deliver(sender, authAddress, input, sdkerrors.ErrEVMVMError.ABCICode())
	require.Empty(t, duplicate.Logs)
	reason, err = abi.UnpackRevert(replay.ReturnData)
	require.NoError(t, err)
	require.Equal(t, "LXP_SEQUENCE_REUSED", reason)
	next, err := lxphost.NextSequence(ctx, a.GetKey(app.LayerXHostStoreKey))
	require.NoError(t, err)
	revokeInput, err := authABI.Pack("revokeGrant", grantID, uint8(1), next)
	require.NoError(t, err)
	revoked, _ := deliver(sender, authAddress, revokeInput, 0)
	require.Len(t, revoked.Logs, 2)
	require.Equal(t, []string{authABI.Events["GrantRevoked"].ID.Hex(), common.Hash(grantID).Hex()}, revoked.Logs[0].Topics)
	values, err := authABI.Events["GrantRevoked"].Inputs.NonIndexed().Unpack(revoked.Logs[0].Data)
	require.NoError(t, err)
	event := GrantRevoked{Record: values[0].([]byte)}
	require.Len(t, event.Record, 41)
	require.Equal(t, grantID[:], event.Record[:32])
	require.Equal(t, byte(1), event.Record[32])
	require.Equal(t, next, binary.BigEndian.Uint64(event.Record[33:]))
	require.Equal(t, []interface{}{false, "LXP_AUTH_REVOKED"}, view("isLive", grantID, next, timestamp))
	grantView := view("getGrant", grantID)
	require.Equal(t, true, grantView[1])
	require.Equal(t, next, grantView[2])
	// These are the receipt's actual emitter, topics and bytes, serialized as JSON-RPC logs.
	rpcReceipt := func(receipt *evmtypes.Receipt) map[string]interface{} {
		logs := make([]map[string]interface{}, 0, len(receipt.Logs))
		for index, log := range receipt.Logs {
			logs = append(logs, map[string]interface{}{
				"address": log.Address, "topics": log.Topics, "data": "0x" + hex.EncodeToString(log.Data),
				"logIndex": fmt.Sprintf("0x%x", index), "transactionIndex": "0x0", "removed": false,
			})
		}
		return map[string]interface{}{"status": fmt.Sprintf("0x%x", receipt.Status), "logs": logs}
	}
	proof := map[string]interface{}{
		"holder": sender.EvmAddress.Hex(), "grantId": common.Hash(grantID).Hex(),
		"body": "0x" + hex.EncodeToString(body), "record": "0x" + hex.EncodeToString(event.Record),
		"granted": rpcReceipt(granted), "revoked": rpcReceipt(revoked), "refused": rpcReceipt(failed),
		"grantTx": granted.TxHashHex, "revokeTx": revoked.TxHashHex, "refusedTx": failed.TxHashHex,
		"abi": json.RawMessage(rawABI),
	}
	encoded, err := json.Marshal(proof)
	require.NoError(t, err)
	path := os.Getenv("LAYERX_AUTH_PROOF_INPUT")
	producerOnly := path != ""
	if !producerOnly {
		path = filepath.Join(t.TempDir(), "evm-receipts.json")
	}
	require.NoError(t, os.WriteFile(path, encoded, 0600))
	if producerOnly {
		return
	} // Rust's standalone test requested this real application producer.
	root, err := filepath.Abs("../..")
	require.NoError(t, err)
	runCtx, cancel := context.WithTimeout(context.Background(), 7*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "sh", filepath.Join(root, "tools/runtime/run-with-clock.sh"), "cargo", "test", "--manifest-path", filepath.Join(root, "services/platform/Cargo.toml"),
		"-p", "layerx-indexer", "--test", "history_position", "proof_layerxauth_0x101c_receipts", "--", "--exact", "--nocapture")
	cmd.Env = append(os.Environ(), "LAYERX_AUTH_PROOF_INPUT="+path)
	result, err := cmd.CombinedOutput()
	require.NoError(t, err, "real indexer consumer: %s", result)
	require.Contains(t, string(result), "1 passed; 0 failed", "consumer must actually execute: %s", result)
}
