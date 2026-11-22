package l16

import (
	"encoding/json"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

func receiptHeaderDigest(header *types.Header) (string, error) {
	if header == nil {
		return "", delivery.ErrIntegrity
	}
	raw, err := json.Marshal(struct {
		ParentHash          common.Hash      `json:"parentHash"`
		UncleHash           common.Hash      `json:"sha3Uncles"`
		Coinbase            common.Address   `json:"miner"`
		Root                common.Hash      `json:"stateRoot"`
		TxHash              common.Hash      `json:"transactionsRoot"`
		ReceiptHash         common.Hash      `json:"receiptsRoot"`
		Bloom               types.Bloom      `json:"logsBloom"`
		Difficulty          *hexutil.Big     `json:"difficulty"`
		Number              *hexutil.Big     `json:"number"`
		GasLimit            hexutil.Uint64   `json:"gasLimit"`
		GasUsed             hexutil.Uint64   `json:"gasUsed"`
		Time                hexutil.Uint64   `json:"timestamp"`
		Extra               hexutil.Bytes    `json:"extraData"`
		MixDigest           common.Hash      `json:"mixHash"`
		Nonce               types.BlockNonce `json:"nonce"`
		BaseFee             *hexutil.Big     `json:"baseFeePerGas"`
		WithdrawalsHash     *common.Hash     `json:"withdrawalsRoot"`
		BlobGasUsed         *hexutil.Uint64  `json:"blobGasUsed"`
		ExcessBlobGas       *hexutil.Uint64  `json:"excessBlobGas"`
		ParentBeaconRoot    *common.Hash     `json:"parentBeaconBlockRoot"`
		RequestsHash        *common.Hash     `json:"requestsHash"`
		BlockAccessListHash *common.Hash     `json:"balHash"`
		SlotNumber          *hexutil.Uint64  `json:"slotNumber"`
		Hash                common.Hash      `json:"hash"`
	}{header.ParentHash, header.UncleHash, header.Coinbase, header.Root,
		header.TxHash, header.ReceiptHash, header.Bloom, (*hexutil.Big)(header.Difficulty),
		(*hexutil.Big)(header.Number), hexutil.Uint64(header.GasLimit), hexutil.Uint64(header.GasUsed),
		hexutil.Uint64(header.Time), hexutil.Bytes(header.Extra), header.MixDigest, header.Nonce,
		(*hexutil.Big)(header.BaseFee), header.WithdrawalsHash, (*hexutil.Uint64)(header.BlobGasUsed),
		(*hexutil.Uint64)(header.ExcessBlobGas), header.ParentBeaconRoot, header.RequestsHash,
		header.BlockAccessListHash, (*hexutil.Uint64)(header.SlotNumber), header.Hash()})
	if err != nil {
		return "", err
	}
	return delivery.Hash(raw), nil
}
