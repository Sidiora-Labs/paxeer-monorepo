package receipt

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"strconv"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func attributeRoutedSwap(b Bundle, decoded []*decode.DecodedEnvelope, f accounting.FillV1, sender string, receiptHash [32]byte, fills []accounting.FillV1) accounting.Attribution {
	unavailable := f.Attribution
	data, to := b.Transaction.Data(), b.Transaction.To()
	if to == nil || len(data) != 164 || b.Transaction.Value().Sign() != 0 || len(fills) != 2 || len(decoded) != len(b.Receipt.Logs) {
		return unavailable
	}
	if !bytes.Equal(data[:4], crypto.Keccak256([]byte("swapTokenForToken(address,address,uint256,uint256,uint256)"))[:4]) || !bytes.Equal(data[4:16], make([]byte, 12)) || !bytes.Equal(data[36:48], make([]byte, 12)) || new(big.Int).SetBytes(data[132:164]).Cmp(new(big.Int).SetUint64(b.Header.Time)) < 0 {
		return unavailable
	}
	sell, buy := fills[0], fills[1]
	tokenIn, tokenOut := common.BytesToAddress(data[16:36]), common.BytesToAddress(data[48:68])
	amountIn := new(big.Int).SetBytes(data[68:100]).String()
	amountOut, err := accounting.Uint(buy.TokenAmountRaw)
	if err != nil || new(big.Int).SetBytes(data[100:132]).Cmp(amountOut) > 0 || sell.Side != valueSell || buy.Side != valueBuy || sell.Token.Address == buy.Token.Address || common.HexToAddress(sell.Token.Address) != tokenIn || common.HexToAddress(buy.Token.Address) != tokenOut || sell.TokenAmountRaw != amountIn || sell.USDLAmountRaw != buy.USDLAmountRaw || sell.Settlement != buy.Settlement || sell.PoolAddress == buy.PoolAddress {
		return unavailable
	}
	primary, routes := 0, 0
	for i, e := range decoded {
		if e == nil || e.Status != valueDecoded || e.Event == nil {
			continue
		}
		if e.Family != nil && e.Family.EconomicFill {
			if primary >= len(fills) || !routedSwapFillMatches(e, b.Receipt.Logs[i], fills[primary], sender) {
				return unavailable
			}
			primary++
		}
		if e.EmitterRole == valueRouter && e.EventName == "MultihopSwap" {
			routes++
			args := e.Event.Args
			if b.Receipt.Logs[i].Address != *to || args["sender"] != sender || args["tokenIn"] != sell.Token.Address || args["tokenOut"] != buy.Token.Address || args["amountIn"] != amountIn || args["intermediateUsdl"] != sell.USDLAmountRaw || args["amountOut"] != buy.TokenAmountRaw {
				return unavailable
			}
		}
	}
	if primary != 2 || routes != 1 {
		return unavailable
	}
	indices, ok := routedSwapTransfers(b.Receipt.Logs, sell, buy, common.HexToAddress(sender))
	if !ok {
		return unavailable
	}
	leg := -1
	for i, candidate := range fills {
		if candidate.SourceFillID == f.SourceFillID && candidate.Position == f.Position && candidate.PoolAddress == f.PoolAddress && candidate.Token == f.Token && candidate.Settlement == f.Settlement && candidate.Side == f.Side && candidate.TokenAmountRaw == f.TokenAmountRaw && candidate.USDLAmountRaw == f.USDLAmountRaw {
			leg = i
		}
	}
	if leg < 0 {
		return unavailable
	}
	inLog, outLog := strconv.FormatUint(uint64(indices[leg*2]), 10), strconv.FormatUint(uint64(indices[leg*2+1]), 10)
	txDigest := sha256.Sum256(data)
	rd := hex.EncodeToString(receiptHash[:])
	return accounting.Attribution{TransactionSender: sender, RouterCaller: &sender, Trader: &sender, Payer: &sender, Recipient: &sender, Quality: "qualified_calldata", Evidence: []accounting.EvidenceRef{{Kind: "router-calldata", ArtifactSHA256: hex.EncodeToString(txDigest[:]), ArtifactID: f.Position.TransactionHash}, {Kind: "input-transfer", ArtifactSHA256: rd, ArtifactID: f.Position.TransactionHash, LogIndex: &inLog}, {Kind: "output-transfer", ArtifactSHA256: rd, ArtifactID: f.Position.TransactionHash, LogIndex: &outLog}}}
}

func routedSwapFillMatches(e *decode.DecodedEnvelope, log *types.Log, f accounting.FillV1, sender string) bool {
	input, output := f.USDLAmountRaw, f.TokenAmountRaw
	if f.Side == valueSell {
		input, output = output, input
	}
	return qualifiedSwapFamily(e, log) && strconv.FormatUint(uint64(log.Index), 10) == f.Position.LogIndex && common.HexToAddress(f.ActualEmitter) == log.Address && e.Event.Args["poolId"] == common.HexToHash(f.PoolAddress).Hex() && e.Event.Args["sender"] == sender && e.Event.Args["isBuy"] == (f.Side == valueBuy) && e.Event.Args["amountIn"] == input && e.Event.Args["amountOut"] == output
}

func routedSwapTransfers(logs []*types.Log, sell, buy accounting.FillV1, sender common.Address) ([4]uint, bool) {
	var indices [4]uint
	var counts [4]int
	var relayOut, relayIn common.Address
	sellPool, buyPool := common.HexToAddress(sell.PoolAddress), common.HexToAddress(buy.PoolAddress)
	sellHolder, buyHolder := sellPool, buyPool
	if common.HexToAddress(sell.ActualEmitter) == decode.LaunchpadPrecompileAddress {
		sellHolder = decode.LaunchpadModuleAddress
	}
	if common.HexToAddress(buy.ActualEmitter) == decode.LaunchpadPrecompileAddress {
		buyHolder = decode.LaunchpadModuleAddress
	}
	for _, log := range logs {
		if log == nil || len(log.Topics) != 3 || log.Topics[0] != crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)")) || len(log.Data) != 32 || !bytes.Equal(log.Topics[1].Bytes()[:12], make([]byte, 12)) || !bytes.Equal(log.Topics[2].Bytes()[:12], make([]byte, 12)) {
			continue
		}
		from, to := common.BytesToAddress(log.Topics[1].Bytes()[12:]), common.BytesToAddress(log.Topics[2].Bytes()[12:])
		amount := new(big.Int).SetBytes(log.Data).String()
		matches := [4]bool{
			log.Address == common.HexToAddress(sell.Token.Address) && from == sender && to == sellPool && amount == sell.TokenAmountRaw,
			log.Address == common.HexToAddress(sell.Settlement.Address) && from == sellHolder && amount == sell.USDLAmountRaw,
			log.Address == common.HexToAddress(buy.Settlement.Address) && to == buyHolder && amount == buy.USDLAmountRaw,
			log.Address == common.HexToAddress(buy.Token.Address) && from == buyPool && to == sender && amount == buy.TokenAmountRaw,
		}
		if log.Address == common.HexToAddress(sell.Settlement.Address) && (from == sender || to == sender) {
			return indices, false
		}
		for i, match := range matches {
			if match {
				counts[i]++
				indices[i] = log.Index
			}
		}
		if matches[1] {
			relayOut = to
		}
		if matches[2] {
			relayIn = from
		}
	}
	return indices, counts == [4]int{1, 1, 1, 1} && relayOut == relayIn && relayOut != sender
}
