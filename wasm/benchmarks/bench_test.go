package benchmarks

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	storageconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"

	wasmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/types"
)

type txSendingBenchmark struct {
	db          func(testing.TB) benchmarkDatabase
	txBuilder   func(*testing.B, *AppInfo) []sdk.Tx
	blockSize   int
	numAccounts int
}

func txSendingBenchmarkCases() map[string]txSendingBenchmark {
	return map[string]txSendingBenchmark{
		"basic send - pebbledb": {
			db:          buildPebbleDB,
			blockSize:   20,
			txBuilder:   buildTxFromMsg(bankSendMsg),
			numAccounts: 50,
		},
		"cw20 transfer - pebbledb": {
			db:          buildPebbleDB,
			blockSize:   20,
			txBuilder:   buildTxFromMsg(cw20TransferMsg),
			numAccounts: 50,
		},
		"basic send - pebbledb buffered": {
			db:          buildBufferedPebbleDB,
			blockSize:   20,
			txBuilder:   buildTxFromMsg(bankSendMsg),
			numAccounts: 50,
		},
		"cw20 transfer - pebbledb buffered": {
			db:          buildBufferedPebbleDB,
			blockSize:   20,
			txBuilder:   buildTxFromMsg(cw20TransferMsg),
			numAccounts: 50,
		},
		"basic send - pebbledb buffered - 8k accounts": {
			db:          buildBufferedPebbleDB,
			blockSize:   20,
			txBuilder:   buildTxFromMsg(bankSendMsg),
			numAccounts: 8000,
		},
		"cw20 transfer - pebbledb buffered - 8k accounts": {
			db:          buildBufferedPebbleDB,
			blockSize:   20,
			txBuilder:   buildTxFromMsg(cw20TransferMsg),
			numAccounts: 8000,
		},
		"basic send - pebbledb buffered - 8k accounts - huge blocks": {
			db:          buildBufferedPebbleDB,
			blockSize:   1000,
			txBuilder:   buildTxFromMsg(bankSendMsg),
			numAccounts: 8000,
		},
		"cw20 transfer - pebbledb buffered - 8k accounts - huge blocks": {
			db:          buildBufferedPebbleDB,
			blockSize:   1000,
			txBuilder:   buildTxFromMsg(cw20TransferMsg),
			numAccounts: 8000,
		},
		"basic send - pebbledb buffered - 80k accounts": {
			db:          buildBufferedPebbleDB,
			blockSize:   20,
			txBuilder:   buildTxFromMsg(bankSendMsg),
			numAccounts: 80000,
		},
		"cw20 transfer - pebbledb buffered - 80k accounts": {
			db:          buildBufferedPebbleDB,
			blockSize:   20,
			txBuilder:   buildTxFromMsg(cw20TransferMsg),
			numAccounts: 80000,
		},
	}

}

func BenchmarkTxSending(b *testing.B) {
	for name, tc := range txSendingBenchmarkCases() {
		b.Run(name, tc.run)
	}
}

func (tc txSendingBenchmark) run(b *testing.B) {
	db := tc.db(b)
	appInfo := InitializeWasmApp(b, db, tc.numAccounts)
	txs := tc.txBuilder(b, &appInfo)
	require.Len(b, txs, b.N)

	b.ResetTimer()
	delivered := deliverBenchmarkTxs(b, &appInfo, txs, tc.blockSize)
	b.StopTimer()
	require.Equal(b, b.N, delivered)
	b.ReportMetric(float64(delivered), "delivered-txs")
}

func deliverBenchmarkTxs(b testing.TB, info *AppInfo, txs []sdk.Tx, blockSize int) int {
	b.Helper()
	require.Positive(b, blockSize)
	delivered := 0
	for delivered < len(txs) {
		end := min(delivered+blockSize, len(txs))
		batch := txs[delivered:end]
		for _, tx := range batch {
			_, _, err := info.App.Check(info.TxConfig.TxEncoder(), tx)
			require.NoError(b, err)
		}
		results := commitBenchmarkBlock(b, info.App, info.TxConfig, batch)
		delivered += len(results)
	}
	require.Equal(b, len(txs), delivered)
	return delivered
}

func bankSendMsg(info *AppInfo) ([]sdk.Msg, error) {
	// Precompute all txs
	rcpt := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	coins := sdk.Coins{sdk.NewInt64Coin(info.Denom, 100)}
	sendMsg := banktypes.NewMsgSend(info.MinterAddr, rcpt, coins)
	return []sdk.Msg{sendMsg}, nil
}

func cw20TransferMsg(info *AppInfo) ([]sdk.Msg, error) {
	rcpt := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	transfer := cw20ExecMsg{Transfer: &transferMsg{
		Recipient: rcpt.String(),
		Amount:    765,
	}}
	transferBz, err := json.Marshal(transfer)
	if err != nil {
		return nil, err
	}

	sendMsg := &wasmtypes.MsgExecuteContract{
		Sender:   info.MinterAddr.String(),
		Contract: info.ContractAddr,
		Msg:      transferBz,
	}
	return []sdk.Msg{sendMsg}, nil
}

func buildTxFromMsg(builder func(info *AppInfo) ([]sdk.Msg, error)) func(b *testing.B, info *AppInfo) []sdk.Tx {
	return func(b *testing.B, info *AppInfo) []sdk.Tx {
		return GenSequenceOfTxs(b, info, builder, b.N)
	}
}

type benchmarkDatabase struct {
	backend     string
	directory   string
	writeBuffer int
}

type benchmarkAppOptions struct {
	paxapp.TestAppOpts
	database benchmarkDatabase
}

func (o benchmarkAppOptions) Get(key string) interface{} {
	switch key {
	case paxapp.FlagSSBackend:
		return o.database.backend
	case paxapp.FlagSSDirectory:
		return o.database.directory
	case paxapp.FlagSSAsyncWriterBuffer:
		return o.database.writeBuffer
	default:
		return o.TestAppOpts.Get(key)
	}
}

func buildPebbleDB(b testing.TB) benchmarkDatabase {
	b.Helper()
	return benchmarkDatabase{backend: storageconfig.PebbleDBBackend, directory: b.TempDir()}
}

func buildBufferedPebbleDB(b testing.TB) benchmarkDatabase {
	b.Helper()
	db := buildPebbleDB(b)
	db.writeBuffer = storageconfig.DefaultSSAsyncBuffer
	return db
}
