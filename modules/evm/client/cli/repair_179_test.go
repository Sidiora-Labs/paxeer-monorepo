package cli_test

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/client/cli"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/bank"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/legacy"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func TestRepair_179_PayablePrecompileValue(t *testing.T) {
	initializeCallPrecompileRegistry(t)
	keys := keyring.NewInMemory()
	info, _, err := keys.NewMnemonic("payer", keyring.English, "", "", hd.Secp256k1)
	require.NoError(t, err)
	createdAddress := info.GetAddress()
	info, err = keys.Key("payer")
	require.NoError(t, err)
	require.Equal(t, createdAddress, info.GetAddress())
	local, ok := info.(keyring.LocalInfo)
	require.True(t, ok)
	private, err := legacy.PrivKeyFromBytes([]byte(local.PrivKeyArmor))
	require.NoError(t, err)
	payer, evmPayer := testkeeper.PrivateKeyToAddresses(private)
	require.Equal(t, info.GetAddress(), payer)
	require.NotEmpty(t, local.PrivKeyArmor)

	genesis := time.Now().UTC()
	validator := secp256k1.GenPrivKey().PubKey()
	wrapper := app.NewTestWrapper(t, genesis, validator, true)
	a := wrapper.App
	a.UpgradeKeeper.SetDone(wrapper.Ctx, app.V6120Upgrade)
	a.RefreshPrecompiles(wrapper.Ctx)
	a.EvmKeeper.SetAddressMapping(wrapper.Ctx, payer, evmPayer)
	wrapper.FundAcc(payer, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1_000_000_000_000)))
	_, err = a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{ChainID: "pax-test", Height: 1},
	})
	require.NoError(t, err)
	a.SetDeliverStateToCommit()
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	exec, err := sequencer.NewExecutor(a, sequencer.Options{
		ChainID: "pax-test", ProposerAddress: validator.Address().Bytes(),
	})
	require.NoError(t, err)
	require.NoError(t, exec.Open(2, genesis.Add(time.Second).UnixMilli()))

	keyDir := t.TempDir()
	consensusPath := filepath.Join(keyDir, "consensus.json")
	pv, err := privval.GenFilePV(consensusPath, filepath.Join(keyDir, "consensus-state.json"), "")
	require.NoError(t, err)
	require.NoError(t, pv.Key.Save())
	seed := sha256.Sum256([]byte("repair 179 receipt signing key"))
	receiptPrivate := ed25519.NewKeyFromSeed(seed[:])
	receiptPath := filepath.Join(keyDir, "receipt.key")
	require.NoError(t, os.WriteFile(receiptPath, []byte(hex.EncodeToString(seed[:])), 0o600))
	receiptKey, err := sequencer.LoadReceiptKey(receiptPath, consensusPath, []verify.SequencerAuthorization{{
		SequencerID:      sha256.Sum256([]byte("repair 179 sequencer")),
		PublicKey:        [32]byte(receiptPrivate.Public().(ed25519.PublicKey)),
		FirstBatchNumber: 1, LastBatchNumber: 10,
	}}, 2)
	require.NoError(t, err)
	receipter, err := sequencer.NewReceipter(exec, a, &a.EvmKeeper, receiptKey, app.EVMReceiptModuleID)
	require.NoError(t, err)
	journal, err := wal.OpenWAL(t.TempDir(), wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(journal.Close)
	replicator, err := sequencer.NewReplicator(sequencer.ReplicatorConfig{WAL: journal, Standalone: true})
	require.NoError(t, err)
	t.Cleanup(replicator.Close)
	metricsProvider := sdkmetric.NewMeterProvider()
	t.Cleanup(func() { require.NoError(t, metricsProvider.Shutdown(context.Background())) })
	metrics, err := sequencer.NewMetrics(metricsProvider.Meter("repair179"))
	require.NoError(t, err)
	source := sequencer.NewReceiptSource(16)
	accepter := sequencer.NewAccepter(receipter, replicator, source, metrics)
	openCtx, err := exec.Snapshot()
	require.NoError(t, err)
	ctxProvider := func(int64) sdk.Context { return openCtx }
	txConfig := a.GetTxConfig()
	txConfigProvider := func(int64) client.TxConfig { return txConfig }
	sendAPI := evmrpc.NewSendAPI(nil, txConfigProvider,
		&evmrpc.SendConfig{Sequencer: accepter, SequencerSource: source},
		&a.EvmKeeper, a.BeginBlockKeepers, ctxProvider, keyDir, nil,
		a.BaseApp, a.TracerAnteHandler, evmrpc.ConnectionTypeHTTP,
		utils.None[time.Duration](), nil, &sync.Mutex{}, nil)
	infoAPI := evmrpc.NewInfoAPI(nil, &a.EvmKeeper, ctxProvider, txConfigProvider,
		keyDir, 1024, evmrpc.ConnectionTypeHTTP, txConfig.TxDecoder(), nil)
	rpcServer := rpc.NewServer()
	require.NoError(t, rpcServer.RegisterName("eth", sendAPI))
	require.NoError(t, rpcServer.RegisterName("eth", infoAPI))
	t.Cleanup(rpcServer.Stop)
	endpoint := httptest.NewServer(rpcServer)
	t.Cleanup(endpoint.Close)
	clientCtx := client.Context{}.WithKeyring(keys).WithFromName("payer").WithFrom("payer").
		WithFromAddress(payer).WithTxConfig(txConfig).WithChainID("pax-test")
	commandContext := context.WithValue(context.Background(), client.ClientContextKey, &clientCtx)

	payments := []string{"1", "1000000000001", "18446744073709551617"}
	for i, payment := range payments {
		t.Run(payment, func(t *testing.T) {
			recipient := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
			cmd := cli.GetTxCmd()
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs([]string{"call-precompile", "bank", "sendNative", recipient.String(),
				"--value=" + payment, "--nonce=" + fmt.Sprint(i), "--evm-rpc=" + endpoint.URL})
			require.NoError(t, cmd.ExecuteContext(commandContext))
			ctx, err := exec.Snapshot()
			require.NoError(t, err)
			want, ok := new(big.Int).SetString(payment, 10)
			require.True(t, ok)
			require.Zero(t, want.Cmp(a.EvmKeeper.GetBalance(ctx, recipient)), "payable precompile must deliver the exact CLI payment")
		})
	}

	for _, invalid := range []string{"-1", "1.5", "0x10", "invalid"} {
		t.Run("invalid_"+invalid, func(t *testing.T) {
			cmd := cli.GetTxCmd()
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs([]string{"call-precompile", "bank", "sendNative", payer.String(),
				"--value=" + invalid, "--nonce=3", "--evm-rpc=" + endpoint.URL})
			require.EqualError(t, cmd.ExecuteContext(commandContext), invalid+" is not a valid value. Must be a decimal nonnegative integer")
		})
	}

	batch, err := exec.Seal(2)
	require.NoError(t, err)
	require.Len(t, batch.EVMMsgs, len(payments), "invalid payments must not submit transactions")
	for i, msg := range batch.EVMMsgs {
		transaction, _ := msg.AsTransaction()
		require.Equal(t, payments[i], transaction.Value().String())
		require.Equal(t, bank.BankAddress, transaction.To().Hex())
		receipt, err := a.EvmKeeper.GetTransientReceipt(batch.Ctx, transaction.Hash(), uint64(i+1))
		require.NoError(t, err)
		require.Equal(t, uint32(1), receipt.Status, receipt.VmError)
		require.Empty(t, receipt.VmError)
	}
}
