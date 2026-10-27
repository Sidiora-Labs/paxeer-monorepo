package evmrpc

import (
	"context"
	"runtime"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

type Web3API struct {
	connectionType ConnectionType
}

func (w *Web3API) ClientVersion() string {
	defer recordMetrics(context.Background(), "web3_clientVersion", w.connectionType, time.Now())
	name := "Geth" // Pax EVM is backed by go-ethereum
	name += "/" + runtime.GOOS + "-" + runtime.GOARCH
	name += "/" + runtime.Version()
	return name
}

func (w *Web3API) Sha3(input hexutil.Bytes) hexutil.Bytes {
	defer recordMetrics(context.Background(), "web3_sha3", w.connectionType, time.Now())
	return crypto.Keccak256(input)
}
