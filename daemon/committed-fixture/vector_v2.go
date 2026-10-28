package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

const vectorV2Name = "vector-v2-receipted-revert"

type vectorV2Provenance struct {
	Revision    string `json:"revision"`
	ChainID     string `json:"chainId"`
	FirstHeight uint64 `json:"firstHeight"`
	LastHeight  uint64 `json:"lastHeight"`
	Generator   string `json:"generator"`
	Command     string `json:"command"`
	Source      string `json:"source,omitempty"`
}

type vectorV2 struct {
	Name           string                        `json:"name"`
	Provenance     vectorV2Provenance            `json:"provenance"`
	ChainID        *hexutil.Big                  `json:"chainId"`
	RawTransaction hexutil.Bytes                 `json:"rawTransaction"`
	ReceiptKeys    seqkeeper.ReceiptKeys         `json:"receiptKeys"`
	Receipt        evmrpc.SequencerReceiptResult `json:"receiptJson"`
	LXPCode        string                        `json:"lxpCode"`
	RevertReason   string                        `json:"revertReason"`
}

func init() {
	registerVector(vectorV2Name, recordV2)
}

func recordV2(n *node) (any, error) {
	ctx, cancel := context.WithTimeout(n.ctx, 30*time.Second)
	defer cancel()
	versionJSON, err := n.command("version", "--long", "--output", "json")
	if err != nil {
		return nil, err
	}
	var version struct {
		Commit string `json:"commit"`
	}
	if err := json.Unmarshal(versionJSON, &version); err != nil {
		return nil, err
	}
	if version.Commit == "" {
		return nil, errors.New("paxd must report its build revision")
	}
	var genesis struct {
		ChainID  string `json:"chain_id"`
		AppState struct {
			Sequencer struct {
				ReceiptKeys seqkeeper.ReceiptKeys `json:"receipt_keys"`
			} `json:"sequencer"`
		} `json:"app_state"`
	}
	if err := readJSON(filepath.Join(n.home, "config", "genesis.json"), &genesis); err != nil {
		return nil, err
	}
	keys := genesis.AppState.Sequencer.ReceiptKeys
	if err := keys.Validate(); err != nil {
		return nil, err
	}
	if len(keys) != 1 || !bytes.Equal(keys[0].PublicKey, n.receiptPublicKey) {
		return nil, errors.New("V2 requires this home's generated receipt key")
	}
	kr, err := keyring.New("paxd", keyring.BackendTest, n.home, strings.NewReader(""))
	if err != nil {
		return nil, err
	}
	secret, err := keyring.NewUnsafe(kr).UnsafeExportPrivKeyHex("fixture")
	if err != nil {
		return nil, fmt.Errorf("read temporary fixture transaction key: %w", err)
	}
	key, err := crypto.HexToECDSA(secret)
	if err != nil {
		return nil, err
	}
	defer key.D.SetInt64(0)
	sender := crypto.PubkeyToAddress(key.PublicKey)
	var chainID, gasPrice hexutil.Big
	var nonce hexutil.Uint64
	if err := n.client.CallContext(ctx, &chainID, "eth_chainId"); err != nil {
		return nil, err
	}
	if err := n.client.CallContext(ctx, &gasPrice, "eth_gasPrice"); err != nil {
		return nil, err
	}
	if err := n.client.CallContext(ctx, &nonce, "eth_getTransactionCount", sender, "pending"); err != nil {
		return nil, err
	}
	to := common.HexToAddress(layerxledger.LedgerAddress)
	// The invalid selector fails in the real precompile's LXP reason encoder,
	// without depending on a kernel account or asset existing in genesis.
	tx, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce: uint64(nonce), GasPrice: (*big.Int)(&gasPrice), Gas: 100000,
		To: &to, Value: new(big.Int), Data: []byte{0xde, 0xad, 0xbe, 0xef},
	}), ethtypes.LatestSignerForChainID((*big.Int)(&chainID)), key)
	if err != nil {
		return nil, err
	}
	raw, err := tx.MarshalBinary()
	if err != nil {
		return nil, err
	}
	var hash common.Hash
	if err := n.client.CallContext(ctx, &hash, "eth_sendRawTransaction", hexutil.Bytes(raw)); err != nil {
		return nil, err
	}
	if hash != tx.Hash() {
		return nil, errors.New("eth_sendRawTransaction returned another transaction hash")
	}
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		var result *evmrpc.SequencerReceiptResult
		if err := n.client.CallContext(ctx, &result, "pax_sequencerReceipt", hash); err != nil {
			return nil, err
		}
		if result != nil && result.Rung != evmrpc.RungSubmitted {
			reason, err := abi.UnpackRevert(result.ReturnData)
			if err != nil {
				return nil, fmt.Errorf("decode V2 Error(string): %w", err)
			}
			code := strings.SplitN(reason, ":", 2)[0]
			v := &vectorV2{
				Name: vectorV2Name, ChainID: &chainID, RawTransaction: raw,
				ReceiptKeys: keys, Receipt: *result, LXPCode: code, RevertReason: reason,
				Provenance: vectorV2Provenance{
					Revision: version.Commit, ChainID: genesis.ChainID,
					FirstHeight: uint64(result.Height), LastHeight: uint64(result.Height),
					Generator: "daemon/committed-fixture/vector_v2.go",
					Command:   "committed-fixture record --paxd paxd --out daemon/committed-fixture/vectors",
				},
			}
			if err := verifyVectorV2(v); err != nil {
				return nil, err
			}
			return v, nil
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("wait for V2 signed receipt: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func verifyVectorV2(v *vectorV2) error {
	if v.Name != vectorV2Name || v.ChainID == nil || (*big.Int)(v.ChainID).Sign() <= 0 {
		return errors.New("invalid V2 name or chain id")
	}
	if v.Provenance.Revision == "" || v.Provenance.ChainID == "" || v.Provenance.Generator == "" || v.Provenance.Command == "" {
		return errors.New("missing V2 provenance")
	}
	if len(v.ReceiptKeys) == 0 {
		return errors.New("V2 has no pinned receipt keys")
	}
	if err := v.ReceiptKeys.Validate(); err != nil {
		return err
	}
	r := &v.Receipt
	if r.Rung != evmrpc.RungReceipted && r.Rung != evmrpc.RungCommitted {
		return errors.New("V2 has no signed receipt")
	}
	if r.SequencerReceiptView == nil {
		return errors.New("V2 receipt view is absent")
	}
	if err := r.Verify(v.ReceiptKeys); err != nil {
		return err
	}
	verified, err := verify.ReceiptSignature(r.Receipt, [32]byte(r.SigningKey))
	if err != nil {
		return err
	}
	if r.Status != 0 || verified.Receipt.ResultCode != 1 {
		return errors.New("V2 receipt did not revert")
	}
	if sha256.Sum256(r.ReturnData) != verified.Receipt.ActivityRoot {
		return errors.New("V2 return data differs from signed digest")
	}
	if v.Provenance.FirstHeight == 0 || v.Provenance.FirstHeight > uint64(r.Height) || v.Provenance.LastHeight < uint64(r.Height) {
		return errors.New("V2 receipt height is outside provenance heights")
	}
	var tx ethtypes.Transaction
	if err := tx.UnmarshalBinary(v.RawTransaction); err != nil {
		return err
	}
	if tx.ChainId().Cmp((*big.Int)(v.ChainID)) != 0 || tx.Hash() != r.TxHash || [32]byte(tx.Hash()) != verified.Receipt.ActivityID || tx.Nonce() != uint64(r.Nonce) {
		return errors.New("V2 receipt does not bind the recorded transaction")
	}
	sender, err := ethtypes.Sender(ethtypes.LatestSignerForChainID(tx.ChainId()), &tx)
	if err != nil {
		return err
	}
	if sender != r.From || tx.To() == nil || *tx.To() != common.HexToAddress(layerxledger.LedgerAddress) || *tx.To() != r.To || len(tx.Data()) < 4 {
		return errors.New("V2 transaction is not the recorded ledger precompile call")
	}
	reason, err := abi.UnpackRevert(r.ReturnData)
	if err != nil {
		return fmt.Errorf("V2 return data is not Error(string): %w", err)
	}
	if !strings.HasPrefix(v.LXPCode, "LXP_") || len(v.LXPCode) <= 4 || strings.SplitN(reason, ":", 2)[0] != v.LXPCode || reason != v.RevertReason {
		return errors.New("V2 revert data does not preserve the recorded LXP code and reason")
	}
	return nil
}
