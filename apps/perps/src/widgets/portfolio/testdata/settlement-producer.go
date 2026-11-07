package main

import (
	"context"
	"fmt"
	"io"
	"math/big"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/graph-gophers/graphql-go/relay"

	perpsabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/perps/abi"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/decode"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/graphql"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/telemetry"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func integer(s string) *big.Int {
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic(s)
	}
	return n
}

func main() {
	ctx := context.Background()
	db, err := store.Open(os.Getenv("SETTLEMENT_PROOF_DATABASE_URL"))
	must(err)
	defer db.Close()
	must(store.Migrate(ctx, db))
	metrics, err := telemetry.New(config.Indexer)
	must(err)
	registry, err := decode.NewRegistry(metrics, decode.Options{
		PerpsModule: decode.ModuleAddress("perps"), SpotModule: decode.ModuleAddress("spot"),
		PricefeedModule: decode.ModuleAddress("pricefeed"),
	})
	must(err)
	user := common.HexToAddress("0x00000000000000000000000000000000000000a1")
	token := common.HexToAddress("0x85FcD13735F4309833A503EE804ea32395851479")
	at := time.Unix(1800000000, 0).UTC()
	hash := common.HexToHash("0xb100")
	txHash := common.HexToHash("0xa100")
	var logs []decode.Log
	emit := func(name string, args ...any) {
		topics, data, err := perpsabi.PackEvent(name, args...)
		must(err)
		logs = append(logs, decode.Log{
			Log: ethtypes.Log{Address: decode.PerpsAddress, Topics: topics, Data: data,
				BlockNumber: 100, BlockHash: hash, TxHash: txHash, TxIndex: 0, Index: uint(len(logs))},
			ReceiptKind: decode.ReceiptTx, TxFrom: &user, BlockTime: at,
		})
	}
	for i, funding := range []string{"2000000000000000000", "-2000000000000000000", "0"} {
		id := big.NewInt(int64(i + 1))
		emit("PositionOpened", id, user, big.NewInt(1), true, integer("1000000000000000000000"),
			integer("10000000000000000000"),
			integer("50000000000000000000000"), token, big.NewInt(100000000))
		payout := []int64{119000000, 119000001, 0}[i]
		emit("TradeSettled", id, user, big.NewInt(1), uint8(1), integer("1000000000000000000000"),
			integer("51000000000000000000000"), integer("20000000000000000000"),
			integer("1000000000000000000"), integer(funding), big.NewInt(payout))
		emit("PositionClosed", id, user, big.NewInt(1), integer("1000000000000000000000"),
			integer("51000000000000000000000"), integer("20000000000000000000"), true)
	}
	result, err := registry.Decode(ctx, db, logs)
	must(err)
	if len(result.Raw) != 0 || len(result.Decoded) != len(logs) {
		panic(fmt.Sprintf("not every real ABI log decoded: %+v", result))
	}
	tx, err := db.BeginTx(ctx, nil)
	must(err)
	defer tx.Rollback()
	block := store.Block{Number: 100, Hash: hash, Time: at, Proposer: strings.Repeat("AB", 20), TxCount: 1}
	_, err = tx.ExecContext(ctx, `INSERT INTO perps.blocks (number, hash, parent_hash, time, proposer, tx_count)
 VALUES ($1, $2, $3, $4, $5, $6)`, int64(block.Number), hash.Bytes(), common.Hash{}.Bytes(), at, block.Proposer, block.TxCount)
	must(err)
	must(store.WriteBlock(ctx, tx, block, result))
	must(tx.Commit())
	schema, err := graphql.NewSchema(db)
	must(err)
	server := httptest.NewServer(&relay.Handler{Schema: schema})
	defer server.Close()
	fmt.Println(server.URL)
	// The parent owns the disposable database and closes stdin after assertions.
	_, err = io.Copy(io.Discard, os.Stdin)
	must(err)
}
