package main

// Implementation-time extension of daemon/committed-fixture, loaded with a Go
// overlay. The Rust gate reads the resulting transcript without running Go.
import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/merkle"
	tmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client"
	tmhttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxauth"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func init() { registerVector("recovery-22-33-47", recordRecovery) }

func recovery64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

func recoveryQueryOptions(height int64) tmrpc.ABCIQueryOptions {
	return tmrpc.ABCIQueryOptions{Height: height, Prove: true}
}

func recoveryField(b, v []byte) []byte {
	b = binary.BigEndian.AppendUint32(b, uint32(len(v)))
	return append(b, v...)
}

func recoveryProof(p *merkle.Proof) string {
	b := binary.BigEndian.AppendUint64(nil, uint64(p.Total))
	b = binary.BigEndian.AppendUint64(b, uint64(p.Index))
	b = append(b, p.LeafHash...)
	b = append(b, byte(len(p.Aunts)))
	for _, a := range p.Aunts {
		b = append(b, a...)
	}
	return recovery64(b)
}

func recordRecovery(n *node) (any, error) {
	ctx, cancel := context.WithTimeout(n.ctx, 8*time.Minute)
	defer cancel()
	wallet, err := keyring.New(sdk.KeyringServiceName(), keyring.BackendTest, n.home, nil)
	if err != nil {
		return nil, err
	}
	hexKey, err := keyring.NewUnsafe(wallet).UnsafeExportPrivKeyHex("fixture")
	if err != nil {
		return nil, err
	}
	payer, err := crypto.HexToECDSA(strings.TrimSpace(hexKey))
	if err != nil {
		return nil, err
	}
	defer payer.D.SetInt64(0)
	var chain hexutil.Big
	if err := n.client.CallContext(ctx, &chain, "eth_chainId"); err != nil {
		return nil, err
	}
	id := (*big.Int)(&chain)
	send := func(to common.Address, data []byte) (*evmrpc.SequencerReceiptResult, []byte, error) {
		var nonce hexutil.Uint64
		var price hexutil.Big
		from := crypto.PubkeyToAddress(payer.PublicKey)
		if err := v4PendingCall(ctx, n, &nonce, "eth_getTransactionCount", from, "pending"); err != nil {
			return nil, nil, err
		}
		if err := v4PendingCall(ctx, n, &price, "eth_gasPrice"); err != nil {
			return nil, nil, err
		}
		tx, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.DynamicFeeTx{
			ChainID: id, Nonce: uint64(nonce), GasTipCap: big.NewInt(1), GasFeeCap: (*big.Int)(&price),
			Gas: 2_000_000, To: &to, Value: new(big.Int), Data: data,
		}), ethtypes.LatestSignerForChainID(id), payer)
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
			return nil, nil, errors.New("transaction identity mismatch")
		}
		r, err := v4WaitCommitted(ctx, n, hash)
		return r, raw, err
	}
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{73}, 32))
	var public [32]byte
	copy(public[:], private.Public().(ed25519.PublicKey))
	message, err := evmtypes.LayerXBindMessage(id, crypto.PubkeyToAddress(payer.PublicKey), 0)
	if err != nil {
		return nil, err
	}
	addressABI, err := abi.JSON(strings.NewReader(`[{"type":"function","name":"bindLayerX","inputs":[{"name":"didPublicKey","type":"bytes32"},{"name":"signature","type":"bytes"}],"outputs":[]}]`))
	if err != nil {
		return nil, err
	}
	call, err := addressABI.Pack("bindLayerX", public, ed25519.Sign(private, message))
	clear(private)
	if err != nil {
		return nil, err
	}
	r, _, err := send(common.HexToAddress("0x1004"), call)
	if err != nil {
		return nil, fmt.Errorf("bind: %w", err)
	}
	if uint64(r.Status) != 1 {
		return nil, errors.New("binding reverted")
	}
	did, err := lxphost.DIDID([]byte(evmtypes.LayerXDid(public)))
	if err != nil {
		return nil, err
	}
	auth := layerxauth.GetABI()
	registration := append(append([]byte{0x71, 1, 0, 2}, did[:]...), public[:]...)
	call, err = auth.Pack("recordLifecycle", registration, [32]byte{0x51})
	if err != nil {
		return nil, err
	}
	r, _, err = send(common.HexToAddress(layerxauth.AuthAddress), call)
	if err != nil {
		return nil, fmt.Errorf("register: %w", err)
	}
	if uint64(r.Status) != 1 {
		return nil, fmt.Errorf("registration reverted: %s", r.VMError)
	}
	root := [32]byte{0x61}
	payload := append(append([]byte{0x71, 3, 0, 3}, did[:]...), root[:]...)
	payload = binary.BigEndian.AppendUint16(payload, 2)
	var cases []any
	for index, name := range []string{"other", "recovery", "failed"} {
		var data []byte
		target := common.HexToAddress(layerxauth.AuthAddress)
		if index == 0 {
			target = common.HexToAddress("0x1234")
		} else {
			body := bytes.Clone(payload)
			if index == 2 {
				clear(body[36:68])
			}
			data, err = auth.Pack("recordLifecycle", body, [32]byte{byte(0x52 + index)})
			if err != nil {
				return nil, err
			}
		}
		r, raw, err := send(target, data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if (uint64(r.Status) == 1) != (index != 2) {
			return nil, fmt.Errorf("unexpected %s status: %d", name, r.Status)
		}
		c, err := recoveryCase(ctx, n, r, raw, did, root)
		if err != nil {
			return nil, fmt.Errorf("%s evidence: %w", name, err)
		}
		cases = append(cases, c)
	}
	return map[string]any{"provenance": "paxd RPC via daemon/committed-fixture; recovery.go overlay; unmodified node receipts, headers, commits and proofs", "cases": cases}, nil
}

func recoveryCase(ctx context.Context, n *node, r *evmrpc.SequencerReceiptResult, raw []byte, did, root [32]byte) (any, error) {
	client, err := tmhttp.New("http://" + n.consensusAddress)
	if err != nil {
		return nil, err
	}
	height := int64(*r.BlockNumber)
	next := height + 1
	for {
		status, err := client.Status(ctx)
		if err != nil {
			return nil, err
		}
		if status.SyncInfo.LatestBlockHeight >= next {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	block, err := client.Block(ctx, &height)
	if err != nil {
		return nil, err
	}
	execution, err := client.Commit(ctx, &height)
	if err != nil {
		return nil, err
	}
	successor, err := client.Commit(ctx, &next)
	if err != nil {
		return nil, err
	}
	validators, err := client.Validators(ctx, &next, nil, nil)
	if err != nil {
		return nil, err
	}
	results, err := client.BlockResults(ctx, &height)
	if err != nil {
		return nil, err
	}
	txs := block.Block.Data.Txs
	if len(txs) != 2 || len(results.TxsResults) != 2 || uint64(r.Index) != 1 {
		return nil, errors.New("expected one admitted transaction in block")
	}
	encoded, err := abci.MarshalTxResults(results.TxsResults)
	if err != nil {
		return nil, err
	}
	resultRoot, proofs := merkle.ProofsFromByteSlices(encoded)
	if !bytes.Equal(resultRoot, successor.Header.LastResultsHash) {
		return nil, errors.New("node result commitment mismatch")
	}
	query, err := client.ABCIQueryWithOptions(ctx, "/store/sequencer/key", []byte{3} /* explicit H proof, H+1 header */, recoveryQueryOptions(height))
	if err != nil {
		return nil, err
	}
	if query.Response.Code != 0 || query.Response.ProofOps == nil {
		return nil, errors.New("receipt key proof missing")
	}
	sh, err := successor.Header.ToProto().Marshal()
	if err != nil {
		return nil, err
	}
	sc, err := successor.Commit.ToProto().Marshal()
	if err != nil {
		return nil, err
	}
	eh, err := execution.Header.ToProto().Marshal()
	if err != nil {
		return nil, err
	}
	ec, err := execution.Commit.ToProto().Marshal()
	if err != nil {
		return nil, err
	}
	keyProof := []byte("LXCP\x01")
	for _, v := range [][]byte{sh, sc, nil, []byte("sequencer"), {3}} {
		keyProof = recoveryField(keyProof, v)
	}
	keyProof = append(keyProof, 1)
	keyProof = recoveryField(keyProof, query.Response.Value)
	keyProof = binary.BigEndian.AppendUint32(keyProof, uint32(len(query.Response.ProofOps.Ops)))
	for _, op := range query.Response.ProofOps.Ops {
		keyProof = recoveryField(keyProof, []byte(op.Type))
		keyProof = recoveryField(keyProof, op.Key)
		keyProof = recoveryField(keyProof, op.Data)
	}
	decoded, err := codec.DecodeReceipt(r.Receipt)
	if err != nil {
		return nil, err
	}
	var pins []any
	for _, v := range validators.Validators {
		pins = append(pins, map[string]any{"public_key": recovery64(v.PubKey.Bytes()), "power": v.VotingPower})
	}
	result := results.TxsResults[1]
	batchProof := txs.Proof(0).Proof
	return map[string]any{
		"validators": pins,
		"receipt":    map[string]any{"version": 1, "principal": "alice", "evidence_id": "recovery-1", "canonical_receipt": recovery64(r.Receipt), "header": recovery64(sh), "commit": recovery64(sc), "transaction_proof": ""},
		"inclusion": map[string]any{
			"containing_transaction": recovery64(txs[0]), "transaction_proof": recoveryProof(&batchProof),
			"execution_header": recovery64(eh), "execution_commit": recovery64(ec), "activity_ids": [][32]byte{decoded.ActivityID}, "receipt_index": 0,
			"raw_transaction": recovery64(raw), "receipt_keys_proof": recovery64(keyProof),
			"result_code": result.Code, "result_data": recovery64(result.Data), "gas_wanted": result.GasWanted, "gas_used": result.GasUsed,
			"result_proof": recoveryProof(proofs[1]), "recovery_did": did, "recovery_root": root, "recovery_threshold": 2,
		},
	}, nil
}
