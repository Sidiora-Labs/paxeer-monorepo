package receipt

import (
	"bytes"
	"context"
	_ "embed"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

const virtualChainIDBase uint64 = 125000000

var virtualChainRegistry = common.HexToAddress("0x000000000000000000000000000000000000101e")

//go:embed virtual_chain_abi.json
var virtualChainABIJSON []byte

func validSignedChainID(chainID *big.Int) bool {
	if chainID == nil || !chainID.IsUint64() {
		return false
	}
	n := chainID.Uint64()
	return n == 125 || n > virtualChainIDBase && n <= virtualChainIDBase+math.MaxUint32
}

// ValidateSignedDomain checks registry admission immediately before execution,
// using parent state and the preceding committed receipts in the same block.
// End-of-block status cannot establish admission: a later transaction can pause
// or retire a domain after its valid transactions have executed.
func (a *RPCQualification) ValidateSignedDomain(ctx context.Context, b Bundle) error {
	if a == nil || b.Transaction == nil || !validSignedChainID(b.Transaction.ChainId()) {
		return ErrQualification
	}
	if b.Transaction.ChainId().Uint64() == 125 {
		return nil
	}
	if b.Header == nil || b.Header.Number == nil || b.Header.Number.Sign() == 0 || b.Receipt == nil {
		return ErrQualification
	}
	read, cancel := context.WithTimeout(ctx, a.options.Timeout)
	defer cancel()
	block, receipts, err := a.pinnedBlock(read, bundleBlockHash(b))
	if err != nil {
		return err
	}
	index := b.Receipt.TransactionIndex
	if index >= uint(len(receipts)) || block.Transactions[index].Hash() != b.Transaction.Hash() || !receiptEqual(receipts[index], b.Receipt) {
		return ErrQualification
	}
	claimedHeader, err := digest(b.Header)
	if err != nil {
		return err
	}
	actualHeader, err := digest(block.Header)
	if err != nil || claimedHeader != actualHeader {
		return ErrQualification
	}
	registryABI, err := abi.JSON(bytes.NewReader(virtualChainABIJSON))
	if err != nil {
		return err
	}
	id := uint32(b.Transaction.ChainId().Uint64() - virtualChainIDBase) //nolint:gosec // chain or log identity uses the existing 32-bit field
	created := registryABI.Events["ChainCreated"]
	changed := registryABI.Events["ChainStatusChanged"]
	topic := common.BigToHash(new(big.Int).SetUint64(uint64(id)))
	var prior []*types.Log
	createdHere := false
	for _, receipt := range receipts[:index] {
		if receipt.Status != types.ReceiptStatusSuccessful {
			continue
		}
		for _, log := range receipt.Logs {
			if log.Address != virtualChainRegistry || len(log.Topics) < 2 || log.Topics[1] != topic {
				continue
			}
			if log.Topics[0] == created.ID {
				if createdHere || len(log.Topics) != 3 {
					return ErrQualification
				}
				values, err := created.Inputs.NonIndexed().Unpack(log.Data)
				if err != nil || len(values) != 2 {
					return ErrQualification
				}
				canonical, err := created.Inputs.NonIndexed().Pack(values...)
				if err != nil || !bytes.Equal(canonical, log.Data) || values[1].(uint64) != b.Transaction.ChainId().Uint64() {
					return ErrQualification
				}
				createdHere = true
			}
			if log.Topics[0] == created.ID || log.Topics[0] == changed.ID {
				prior = append(prior, log)
			}
		}
	}
	var status uint8
	if !createdHere {
		parent, err := a.headerByHash(read, b.Header.ParentHash)
		if err != nil || new(big.Int).Add(parent.Number, big.NewInt(1)).Cmp(b.Header.Number) != 0 {
			return ErrQualification
		}
		_, canonical, err := a.headerByNumber(read, parent.Number)
		if err != nil || canonical != b.Header.ParentHash {
			return ErrQualification
		}
		data, err := registryABI.Pack("getChain", id)
		if err != nil {
			return err
		}
		call, release, err := a.scalarRead(read)
		if err != nil {
			return err
		}
		raw, err := a.client.CallContractAtHash(call, ethereum.CallMsg{To: &virtualChainRegistry, Data: data, Gas: rpcReadGas}, b.Header.ParentHash)
		release()
		if err != nil {
			return err
		}
		values, err := registryABI.Unpack("getChain", raw)
		if err != nil || len(values) != 11 {
			return ErrQualification
		}
		canonicalRaw, err := registryABI.Methods["getChain"].Outputs.Pack(values...)
		if err != nil || !bytes.Equal(raw, canonicalRaw) || values[0].(uint32) != id || values[4].(uint64) != b.Transaction.ChainId().Uint64() || values[10].(uint64) > parent.Number.Uint64() {
			return ErrQualification
		}
		status = values[3].(uint8)
		if status < 1 || status > 4 {
			return ErrQualification
		}
	}
	for _, log := range prior {
		if log.Topics[0] == created.ID {
			if status != 0 {
				return ErrQualification
			}
			status = 1
			continue
		}
		if len(log.Topics) != 2 || status == 0 || status == 4 {
			return ErrQualification
		}
		values, err := changed.Inputs.NonIndexed().Unpack(log.Data)
		if err != nil || len(values) != 1 {
			return ErrQualification
		}
		canonical, err := changed.Inputs.NonIndexed().Pack(values...)
		if err != nil || !bytes.Equal(canonical, log.Data) {
			return ErrQualification
		}
		status = values[0].(uint8)
		if status < 1 || status > 4 {
			return ErrQualification
		}
	}
	if status != 1 {
		return ErrQualification
	}
	return nil
}
