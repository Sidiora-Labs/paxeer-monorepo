package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/merkle"
	tmhttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	evmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/config"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

type v4Case struct {
	Name            string            `json:"name"`
	Receipt         hexutil.Bytes     `json:"receipt"`
	Header          hexutil.Bytes     `json:"header"`
	HeaderSignature hexutil.Bytes     `json:"header_signature"`
	Proof           codec.MerkleProof `json:"proof"`
	SigningKey      hexutil.Bytes     `json:"signing_key,omitempty"`
}

type v4Settlement struct {
	RawTransaction     hexutil.Bytes                  `json:"raw_transaction"`
	Receipt            *evmrpc.SequencerReceiptResult `json:"receipt"`
	Batch              seqtypes.MsgSequenceBatch      `json:"sequence_batch"`
	BatchProof         tmtypes.TxProof                `json:"sequence_batch_proof"`
	TransactionProof   tmtypes.TxProof                `json:"transaction_proof"`
	ExecTxResult       *abci.ExecTxResult             `json:"exec_tx_result"`
	ExecTxResultBytes  hexutil.Bytes                  `json:"exec_tx_result_bytes"`
	ExecTxResultProof  merkle.Proof                   `json:"exec_tx_result_proof"`
	ExecutionHeader    *tmtypes.Header                `json:"execution_header"`
	ExecutionCommit    *tmtypes.Commit                `json:"execution_commit"`
	Header             *tmtypes.Header                `json:"header"`
	Commit             *tmtypes.Commit                `json:"commit"`
	Validators         []*tmtypes.Validator           `json:"validators"`
	InsufficientCommit *tmtypes.Commit                `json:"commit_at_most_two_thirds"`
}

type v4Vector struct {
	Name          string                        `json:"name"`
	Provenance    string                        `json:"provenance"`
	Authorization verify.SequencerAuthorization `json:"authorization"`
	Positive      v4Case                        `json:"positive"`
	Refusals      []v4Case                      `json:"refusals"`
	Settlement    *v4Settlement                 `json:"settlement"`
}

func init() {
	registerVector("vector-v4-committed", recordV4)
}

func recordV4(n *node) (any, error) {
	ctx, cancel := context.WithTimeout(n.ctx, 3*time.Minute)
	defer cancel()
	wallet, err := keyring.New(sdk.KeyringServiceName(), keyring.BackendTest, n.home, nil)
	if err != nil {
		return nil, err
	}
	keyHex, err := keyring.NewUnsafe(wallet).UnsafeExportPrivKeyHex("fixture")
	if err != nil {
		return nil, err
	}
	payer, err := crypto.HexToECDSA(strings.TrimSpace(keyHex))
	if err != nil {
		return nil, err
	}
	defer payer.D.SetInt64(0)
	payee, err := crypto.GenerateKey()
	if err != nil {
		return nil, err
	}
	defer payee.D.SetInt64(0)
	var chainID hexutil.Big
	if err := n.client.CallContext(ctx, &chainID, "eth_chainId"); err != nil {
		return nil, err
	}
	id := (*big.Int)(&chainID)
	send := func(key *ecdsa.PrivateKey, to common.Address, value *big.Int, data []byte) (*evmrpc.SequencerReceiptResult, []byte, error) {
		from := crypto.PubkeyToAddress(key.PublicKey)
		var nonce hexutil.Uint64
		var gasPrice hexutil.Big
		if err := v4PendingCall(ctx, n, &nonce, "eth_getTransactionCount", from, "pending"); err != nil {
			return nil, nil, err
		}
		if err := v4PendingCall(ctx, n, &gasPrice, "eth_gasPrice"); err != nil {
			return nil, nil, err
		}
		tx, err := ethtypes.SignTx(ethtypes.NewTransaction(uint64(nonce), to, value, 2_000_000, (*big.Int)(&gasPrice), data), ethtypes.LatestSignerForChainID(id), key)
		if err != nil {
			return nil, nil, err
		}
		raw, err := tx.MarshalBinary()
		if err != nil {
			return nil, nil, err
		}
		var hash common.Hash
		if err := n.client.CallContext(ctx, &hash, "eth_sendRawTransaction", hexutil.Bytes(raw)); err != nil {
			return nil, nil, err
		}
		if hash != tx.Hash() {
			return nil, nil, errors.New("submitted transaction hash differs from signed bytes")
		}
		receipt, err := v4WaitCommitted(ctx, n, hash)
		if err != nil {
			return nil, nil, err
		}
		if uint64(receipt.Status) != 1 {
			return nil, nil, fmt.Errorf("fixture transaction %s reverted: %s", hash, receipt.VMError)
		}
		return receipt, raw, nil
	}
	if _, _, err := send(payer, crypto.PubkeyToAddress(payee.PublicKey), big.NewInt(1_000_000_000_000_000_000), nil); err != nil {
		return nil, fmt.Errorf("fund payee: %w", err)
	}
	dids := make([][32]byte, 2)
	addressABI, err := abi.JSON(strings.NewReader(`[{"type":"function","name":"bindLayerX","inputs":[{"name":"didPublicKey","type":"bytes32"},{"name":"signature","type":"bytes"}],"outputs":[],"stateMutability":"nonpayable"}]`))
	if err != nil {
		return nil, err
	}
	for i, key := range []*ecdsa.PrivateKey{payer, payee} {
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
		copy(dids[i][:], public)
		message, err := evmtypes.LayerXBindMessage(id, crypto.PubkeyToAddress(key.PublicKey), 0)
		if err != nil {
			return nil, err
		}
		data, err := addressABI.Pack("bindLayerX", dids[i], ed25519.Sign(private, message))
		clear(private)
		if err != nil {
			return nil, err
		}
		if _, _, err := send(key, common.HexToAddress("0x1004"), new(big.Int), data); err != nil {
			return nil, fmt.Errorf("bind DID %d: %w", i, err)
		}
	}
	actor, err := lxphost.DIDID([]byte(evmtypes.LayerXDid(dids[0])))
	if err != nil {
		return nil, err
	}
	salt := sha256.Sum256([]byte("committed fixture V4 asset"))
	material := append([]byte("LX:ASSET:v1"), actor[:]...)
	asset := sha256.Sum256(append(material, salt[:]...))
	ledgerABI := layerxledger.GetABI()
	call := func(key *ecdsa.PrivateKey, method string, args ...any) (*evmrpc.SequencerReceiptResult, []byte, error) {
		data, err := ledgerABI.Pack(method, args...)
		if err != nil {
			return nil, nil, err
		}
		receipt, raw, err := send(key, common.HexToAddress(layerxledger.LedgerAddress), new(big.Int), data)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", method, err)
		}
		return receipt, raw, nil
	}
	if _, _, err := call(payer, "register", asset, salt, "V4", "Committed fixture", uint8(6), big.NewInt(1000), uint8(1), []byte{}); err != nil {
		return nil, err
	}
	for _, key := range []*ecdsa.PrivateKey{payer, payee} {
		if _, _, err := call(key, "openAccount", asset); err != nil {
			return nil, err
		}
	}
	from, err := ledgerkeeper.AccountID(dids[0], asset)
	if err != nil {
		return nil, err
	}
	to, err := ledgerkeeper.AccountID(dids[1], asset)
	if err != nil {
		return nil, err
	}
	if _, _, err := call(payer, "mint", asset, from, big.NewInt(250)); err != nil {
		return nil, err
	}
	reference := sha256.Sum256([]byte("committed fixture V4 payExact"))
	receipt, raw, err := call(payer, "payExact", from, to, asset, big.NewInt(3), uint64(time.Now().Add(time.Hour).UnixMilli()), reference)
	if err != nil {
		return nil, err
	}
	// Include a second, independently executed receipt so the refusal changes a real sibling.
	// Its block also advances consensus past the payment's execution results.
	sibling, _, err := send(payer, crypto.PubkeyToAddress(payee.PublicKey), big.NewInt(1), nil)
	if err != nil {
		return nil, err
	}
	settlement, err := v4RecordSettlement(ctx, n, receipt, raw)
	if err != nil {
		return nil, err
	}
	return v4Package(n, receipt, sibling, settlement)
}

func v4PendingCall(ctx context.Context, n *node, result any, method string, args ...any) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		err := n.client.CallContext(ctx, result, method, args...)
		if err == nil {
			return nil
		}
		if err.Error() != "pending block unavailable: sequencer: no block is open" && err.Error() != "pending base fee unavailable: sequencer: no block is open" {
			return err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for %s on an open pending block: %w (last RPC error: %v)", method, ctx.Err(), err)
		case <-ticker.C:
		}
	}
}

func v4WaitCommitted(ctx context.Context, n *node, hash common.Hash) (*evmrpc.SequencerReceiptResult, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	keyID := sha256.Sum256(n.receiptPublicKey)
	pins := seqkeeper.ReceiptKeys{{KeyId: keyID[:], PublicKey: n.receiptPublicKey, FirstHeight: 1, LastHeight: 1<<63 - 1}}
	for {
		var receipt *evmrpc.SequencerReceiptResult
		if err := n.client.CallContext(ctx, &receipt, evmrpc.SequencerReceiptMethod, hash); err != nil {
			return nil, err
		}
		if receipt != nil {
			if receipt.TxHash != hash {
				return nil, errors.New("receipt RPC returned another transaction")
			}
			if err := receipt.Verify(pins); err != nil {
				return nil, err
			}
			if receipt.Rung == evmrpc.RungCommitted {
				return receipt, nil
			}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func v4RecordSettlement(ctx context.Context, n *node, receipt *evmrpc.SequencerReceiptResult, raw []byte) (*v4Settlement, error) {
	var cfg struct {
		RPC struct {
			Listen string `toml:"laddr"`
		} `toml:"rpc"`
	}
	if _, err := toml.DecodeFile(filepath.Join(n.home, "config", "config.toml"), &cfg); err != nil {
		return nil, err
	}
	client, err := tmhttp.New(strings.Replace(cfg.RPC.Listen, "tcp://", "http://", 1))
	if err != nil {
		return nil, err
	}
	height := int64(*receipt.BlockNumber)
	block, err := client.Block(ctx, &height)
	if err != nil {
		return nil, err
	}
	if block == nil || block.Block == nil || block.Block.Height != height || !bytes.Equal(block.BlockID.Hash, receipt.BlockHash.Bytes()) {
		return nil, errors.New("committed receipt block does not match Comet block")
	}
	txs := block.Block.Data.Txs
	if len(txs) < 2 {
		return nil, errors.New("committed block is missing its batch or payment transaction")
	}
	batch, err := seqtypes.DecodeSequenceBatchTx(txs[0])
	if err != nil {
		return nil, err
	}
	if err := batch.VerifySignature(fixtureChainID, [32]byte(n.receiptPublicKey)); err != nil {
		return nil, err
	}
	if batch.Height != uint64(height) || uint64(receipt.Seq) < batch.FromSeq || uint64(receipt.Seq) > batch.ToSeq {
		return nil, errors.New("payment is outside committed batch")
	}
	index := int(receipt.Index)
	if index < 1 || index >= len(txs) {
		return nil, errors.New("payment receipt index is outside committed block")
	}
	results, err := client.BlockResults(ctx, &height)
	if err != nil {
		return nil, err
	}
	if results == nil || len(results.TxsResults) != len(txs) {
		return nil, errors.New("Comet results do not match block transactions")
	}
	encoded, err := abci.MarshalTxResults(results.TxsResults)
	if err != nil {
		return nil, err
	}
	root, proofs := merkle.ProofsFromByteSlices(encoded)
	// H+1 commits the execution results of H in LastResultsHash.
	committedHeight := height + 1
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		status, err := client.Status(ctx)
		if err != nil {
			return nil, err
		}
		if status.SyncInfo.LatestBlockHeight >= committedHeight {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
	commit, err := client.Commit(ctx, &committedHeight)
	if err != nil {
		return nil, err
	}
	executionCommit, err := client.Commit(ctx, &height)
	if err != nil {
		return nil, err
	}
	validators, err := client.Validators(ctx, &committedHeight, nil, nil)
	if err != nil {
		return nil, err
	}
	if commit == nil || commit.Header == nil || commit.Commit == nil || validators == nil || validators.Total != len(validators.Validators) || !bytes.Equal(root, commit.Header.LastResultsHash) {
		return nil, errors.New("results root or validator set is incomplete at committed header H")
	}
	if commit.Header.Height != committedHeight || !bytes.Equal(commit.Commit.BlockID.Hash, commit.Header.Hash()) || !bytes.Equal(commit.Header.LastBlockID.Hash, block.Block.Header.Hash()) {
		return nil, errors.New("Comet commit does not bind header H and the payment execution block")
	}
	set := tmtypes.NewValidatorSet(validators.Validators)
	if !bytes.Equal(set.Hash(), commit.Header.ValidatorsHash) {
		return nil, errors.New("validators do not hash to committed header")
	}
	if err := set.VerifyCommit(fixtureChainID, commit.Commit.BlockID, committedHeight, commit.Commit); err != nil {
		return nil, err
	}
	if executionCommit == nil || executionCommit.Commit == nil || executionCommit.Commit.Height != block.Block.Header.Height || !bytes.Equal(executionCommit.Commit.BlockID.Hash, block.Block.Header.Hash()) {
		return nil, errors.New("Comet execution commit does not bind the payment execution header")
	}
	if err := set.VerifyCommit(fixtureChainID, executionCommit.Commit.BlockID, height, executionCommit.Commit); err != nil {
		return nil, err
	}
	if !bytes.Equal(commit.Header.LastCommitHash, executionCommit.Commit.Hash()) {
		return nil, errors.New("committed header does not bind the execution commit")
	}
	insufficient := *commit.Commit
	insufficient.Signatures = make([]tmtypes.CommitSig, len(commit.Commit.Signatures))
	for i := range insufficient.Signatures {
		insufficient.Signatures[i] = tmtypes.NewCommitSigAbsent()
	}
	return &v4Settlement{RawTransaction: raw, Receipt: receipt, Batch: batch, BatchProof: txs.Proof(0), TransactionProof: txs.Proof(index), ExecTxResult: results.TxsResults[index], ExecTxResultBytes: encoded[index], ExecTxResultProof: *proofs[index], ExecutionHeader: &block.Block.Header, ExecutionCommit: executionCommit.Commit, Header: commit.Header, Commit: commit.Commit, Validators: validators.Validators, InsufficientCommit: &insufficient}, nil
}

func v4Package(n *node, receipt, sibling *evmrpc.SequencerReceiptResult, settlement *v4Settlement) (*v4Vector, error) {
	decoded, err := codec.DecodeReceipt(receipt.Receipt)
	if err != nil {
		return nil, err
	}
	last, err := codec.DecodeReceipt(sibling.Receipt)
	if err != nil {
		return nil, err
	}
	if last.GlobalSequence <= decoded.GlobalSequence {
		return nil, errors.New("sibling sequence does not follow payment")
	}
	private, err := v3ReceiptPrivateKey(n)
	if err != nil {
		return nil, err
	}
	defer clear(private)
	public := private.Public().(ed25519.PublicKey)
	if !bytes.Equal(public, n.receiptPublicKey) {
		return nil, errors.New("fixture header signer differs from genesis pin")
	}
	left, right := codec.MerkleLeafHash(receipt.Receipt), codec.MerkleLeafHash(sibling.Receipt)
	h := codec.BatchHeader{ProtocolVersion: decoded.ProtocolVersion, NetworkID: uint32(evmconfig.GetEVMChainID(fixtureChainID).Uint64()), BatchNumber: uint64(*receipt.BlockNumber), FirstSequence: decoded.GlobalSequence, LastSequence: last.GlobalSequence, PreviousStateRoot: decoded.PreviousStateRoot, ResultingStateRoot: last.ResultingStateRoot, ActivityMerkleRoot: codec.MerkleNodeHash(codec.MerkleLeafHash(decoded.ActivityID[:]), codec.MerkleLeafHash(last.ActivityID[:])), ReceiptMerkleRoot: codec.MerkleNodeHash(left, right), TimestampMs: last.Timestamp, SequencerID: [32]byte(public)}
	header := v4EncodeHeader(h)
	digest := codec.BatchHeaderDigest(header)
	signature := ed25519.Sign(private, digest[:])
	positive := v4Case{Name: "committed-payExact", Receipt: receipt.Receipt, Header: header, HeaderSignature: signature, Proof: codec.MerkleProof{LeafIndex: 0, LeafCount: 2, Siblings: [][32]byte{right}}}
	flipped := positive
	flipped.Name = "flipped-proof-sibling"
	flipped.Proof.Siblings = append([][32]byte(nil), positive.Proof.Siblings...)
	flipped.Proof.Siblings[0][0] ^= 1
	otherPublic, otherPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	defer clear(otherPrivate)
	unpinned := positive
	unpinned.Name = "header-signature-by-unpinned-key"
	unpinned.HeaderSignature = ed25519.Sign(otherPrivate, digest[:])
	unpinned.SigningKey = hexutil.Bytes(otherPublic)
	vector := &v4Vector{Name: "vector-v4-committed", Provenance: "Local paxd chain; LayerX inclusion envelope assembled and signed by the recorder under the genesis receipt pin; Comet evidence is collected independently from the committed chain.", Authorization: verify.SequencerAuthorization{SequencerID: [32]byte(public), PublicKey: [32]byte(public), FirstBatchNumber: h.BatchNumber, LastBatchNumber: h.BatchNumber}, Positive: positive, Refusals: []v4Case{flipped, unpinned}, Settlement: settlement}
	if _, _, err := verify.ReceiptInclusion(positive.Receipt, &positive.Proof, positive.Header, [64]byte(signature), vector.Authorization); err != nil {
		return nil, err
	}
	return vector, nil
}

func v4EncodeHeader(h codec.BatchHeader) []byte {
	b := binary.BigEndian.AppendUint16(nil, h.ProtocolVersion)
	b = binary.BigEndian.AppendUint16(b, 0x1701)
	b = append(b, 15, 1)
	b = binary.BigEndian.AppendUint16(b, h.ProtocolVersion)
	b = append(b, 2)
	b = binary.BigEndian.AppendUint32(b, h.NetworkID)
	for i, v := range []uint64{h.Epoch, h.BatchNumber, h.FirstSequence, h.LastSequence} {
		b = append(b, byte(i+3))
		b = binary.BigEndian.AppendUint64(b, v)
	}
	for i, v := range [][32]byte{h.PreviousStateRoot, h.ResultingStateRoot, h.ActivityMerkleRoot, h.ReceiptMerkleRoot, h.EventMerkleRoot, h.DataAvailabilityRoot, h.OracleRoot} {
		b = append(b, byte(i+7))
		b = binary.BigEndian.AppendUint32(b, 32)
		b = append(b, v[:]...)
	}
	b = append(b, 14)
	b = binary.BigEndian.AppendUint64(b, h.TimestampMs)
	b = append(b, 15)
	b = binary.BigEndian.AppendUint32(b, 32)
	return append(b, h.SequencerID[:]...)
}
