package evmrpc

import (
	"runtime"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

type Web3API struct{}

func (w *Web3API) ClientVersion() string {
	name := "Geth" // Pax EVM is backed by go-ethereum
	name += "/" + runtime.GOOS + "-" + runtime.GOARCH
	name += "/" + runtime.Version()
	return name
}

func (w *Web3API) Sha3(input hexutil.Bytes) hexutil.Bytes {
	return crypto.Keccak256(input)
}
