package evmrpc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type pxKernelAPI struct {
	tmClient         client.LocalClient
	txConfigProvider func(int64) client.TxConfig
	ctxProvider      func(int64) sdk.Context
	connectionType   ConnectionType
}

type pxCommitResult struct {
	Height       string               `json:"height"`
	SignedHeader tmtypes.SignedHeader `json:"signedHeader"`
}

type pxValidator struct {
	Address     string          `json:"address"`
	PubKey      json.RawMessage `json:"pubKey"`
	VotingPower string          `json:"votingPower"`
}

type pxValidatorsResult struct {
	Height     string        `json:"height"`
	Validators []pxValidator `json:"validators"`
	Total      string        `json:"total"`
}

type pxBatchProof struct {
	Total    int64           `json:"total"`
	Index    int64           `json:"index"`
	LeafHash hexutil.Bytes   `json:"leafHash"`
	Aunts    []hexutil.Bytes `json:"aunts"`
}

type pxSequenceBatchResult struct {
	Height      string          `json:"height"`
	Tx          hexutil.Bytes   `json:"tx"`
	Proof       pxBatchProof    `json:"proof"`
	ActivityIDs []hexutil.Bytes `json:"activityIds"`
}

func pxChainUnavailable(code string) error {
	return &pxError{-32003, "Chain read unavailable", map[string]interface{}{"code": code}}
}

// The public height is a positive decimal u64. Consensus uses int64; valid
// larger values are unavailable heights, rather than malformed parameters.
func pxConsensusHeight(value string) (int64, error) {
	if value == "" || strings.IndexFunc(value, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return 0, pxInvalidParams()
	}
	height, err := strconv.ParseUint(value, 10, 64)
	if err != nil || height == 0 {
		return 0, pxInvalidParams()
	}
	if height > math.MaxInt64 {
		return 0, pxChainUnavailable("consensus_unavailable")
	}
	return int64(height), nil
}

func (a *pxKernelAPI) GetCommit(ctx context.Context, value string) (result *pxCommitResult, returnErr error) {
	start := time.Now()
	defer func() { recordMetricsWithError(ctx, "px_getCommit", a.connectionType, start, returnErr, recover()) }()
	height, err := pxConsensusHeight(value)
	if err != nil {
		return nil, err
	}
	commit, err := a.tmClient.Commit(ctx, &height)
	if err != nil || commit == nil || commit.Header == nil || commit.Header.Height != height {
		logger.Error("Committed header acquisition failed", "height", height, "error", err)
		return nil, pxChainUnavailable("consensus_unavailable")
	}
	if err := commit.SignedHeader.ValidateBasic(a.ctxProvider(LatestCtxHeight).ChainID()); err != nil {
		logger.Error("Committed header validation failed", "height", height, "error", err)
		return nil, pxChainUnavailable("consensus_unavailable")
	}
	return &pxCommitResult{Height: strconv.FormatInt(height, 10), SignedHeader: commit.SignedHeader}, nil
}

func (a *pxKernelAPI) GetValidators(ctx context.Context, value string) (result *pxValidatorsResult, returnErr error) {
	start := time.Now()
	defer func() { recordMetricsWithError(ctx, "px_getValidators", a.connectionType, start, returnErr, recover()) }()
	height, err := pxConsensusHeight(value)
	if err != nil {
		return nil, err
	}
	validators := make([]pxValidator, 0)
	seen := make(map[string]bool)
	total := -1
	page, perPage := 1, 100
	for {
		response, err := a.tmClient.Validators(ctx, &height, &page, &perPage)
		if err != nil || response == nil || response.BlockHeight != height || response.Total < 0 || (total >= 0 && total != response.Total) || response.Count != len(response.Validators) || response.Count > perPage {
			return nil, pxChainUnavailable("consensus_unavailable")
		}
		total = response.Total
		for _, validator := range response.Validators {
			if validator.ValidateBasic() != nil || len(validator.PubKey.Bytes()) != 32 || bytes.Equal(validator.PubKey.Bytes(), make([]byte, 32)) || !bytes.Equal(validator.Address, validator.PubKey.Address()) || seen[validator.Address.String()] {
				return nil, pxChainUnavailable("consensus_unavailable")
			}
			// Use the consensus encoder for the concrete public-key type and envelope.
			encoded, err := json.Marshal(validator)
			if err != nil {
				return nil, pxChainUnavailable("consensus_unavailable")
			}
			var wire struct {
				PubKey json.RawMessage `json:"pub_key"`
			}
			if json.Unmarshal(encoded, &wire) != nil || len(wire.PubKey) == 0 || wire.PubKey[0] != '{' {
				return nil, pxChainUnavailable("consensus_unavailable")
			}
			seen[validator.Address.String()] = true
			validators = append(validators, pxValidator{Address: validator.Address.String(), PubKey: wire.PubKey, VotingPower: strconv.FormatInt(validator.VotingPower, 10)})
		}
		if len(validators) == total {
			break
		}
		if response.Count == 0 || len(validators) > total {
			return nil, pxChainUnavailable("consensus_unavailable")
		}
		page++
	}
	return &pxValidatorsResult{Height: strconv.FormatInt(height, 10), Validators: validators, Total: strconv.Itoa(total)}, nil
}

func (a *pxKernelAPI) GetSequenceBatch(ctx context.Context, value string) (result *pxSequenceBatchResult, returnErr error) {
	start := time.Now()
	defer func() {
		recordMetricsWithError(ctx, "px_getSequenceBatch", a.connectionType, start, returnErr, recover())
	}()
	height, err := pxConsensusHeight(value)
	if err != nil {
		return nil, err
	}
	response, err := a.tmClient.Block(ctx, &height)
	if err != nil || response == nil || response.Block == nil {
		return nil, pxChainUnavailable("consensus_unavailable")
	}
	block := response.Block
	if block.Height != height || block.ChainID != a.ctxProvider(LatestCtxHeight).ChainID() || !bytes.Equal(block.DataHash, block.Txs.Hash()) {
		return nil, pxChainUnavailable("invalid_consensus_response")
	}
	if len(block.Txs) == 0 || !bytes.HasPrefix(block.Txs[0], []byte(seqtypes.SequenceBatchDomain)) {
		return nil, &pxError{-32602, "Invalid params", map[string]interface{}{"code": "no_sequence_batch"}}
	}
	batch, err := seqtypes.DecodeSequenceBatchTx(block.Txs[0])
	if err != nil || batch.Height != uint64(height) {
		return nil, pxChainUnavailable("invalid_consensus_response")
	}
	if a.txConfigProvider == nil {
		return nil, pxChainUnavailable("invalid_consensus_response")
	}
	decode := a.txConfigProvider(height).TxDecoder()
	ids := make([][32]byte, len(block.Txs)-1)
	activityIDs := make([]hexutil.Bytes, len(ids))
	for i, tx := range block.Txs[1:] {
		ids[i] = pxBatchActivityID(tx, decode)
		activityIDs[i] = append(hexutil.Bytes(nil), ids[i][:]...)
	}
	if seqtypes.VerifyBatchTxs(batch, ids) != nil {
		return nil, pxChainUnavailable("invalid_consensus_response")
	}
	proof := block.Txs.Proof(0)
	if proof.Validate(block.DataHash) != nil {
		return nil, pxChainUnavailable("invalid_consensus_response")
	}
	aunts := make([]hexutil.Bytes, len(proof.Proof.Aunts))
	for i, aunt := range proof.Proof.Aunts {
		aunts[i] = aunt
	}
	return &pxSequenceBatchResult{Height: strconv.FormatInt(height, 10), Tx: hexutil.Bytes(block.Txs[0]), ActivityIDs: activityIDs, Proof: pxBatchProof{Total: proof.Proof.Total, Index: proof.Proof.Index, LeafHash: proof.Proof.LeafHash, Aunts: aunts}}, nil
}

func pxBatchActivityID(raw []byte, decode sdk.TxDecoder) (id [32]byte) {
	id = sha256.Sum256(raw)
	defer func() {
		if recover() != nil {
			id = sha256.Sum256(raw)
		}
	}()
	tx, err := decode(raw)
	if err != nil || tx == nil {
		return id
	}
	msg := evmtypes.GetEVMTransactionMessage(tx)
	if msg == nil || msg.IsAssociateTx() {
		return id
	}
	if _, err := evmtypes.UnpackTxData(msg.Data); err != nil {
		return id
	}
	if etx, _ := msg.AsTransaction(); etx != nil {
		id = [32]byte(etx.Hash())
	}
	return id
}
