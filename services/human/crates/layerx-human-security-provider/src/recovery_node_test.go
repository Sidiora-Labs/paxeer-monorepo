package recovery_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmed25519 "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/merkle"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	tmversion "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/version"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxauth"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdked25519 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func b64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

func proofBytes(p *merkle.Proof) []byte {
	b := binary.BigEndian.AppendUint64(nil, uint64(p.Total))
	b = binary.BigEndian.AppendUint64(b, uint64(p.Index))
	b = append(b, p.LeafHash...)
	b = append(b, byte(len(p.Aunts)))
	for _, aunt := range p.Aunts {
		b = append(b, aunt...)
	}
	return b
}

func field(b []byte, v []byte) []byte {
	b = binary.BigEndian.AppendUint32(b, uint32(len(v)))
	return append(b, v...)
}

func signedHeader(t *testing.T, h *tmtypes.Header, key *sdked25519.PrivKey) ([]byte, []byte) {
	t.Helper()
	id := tmtypes.BlockID{Hash: h.Hash(), PartSetHeader: tmtypes.PartSetHeader{Total: 1, Hash: bytes.Repeat([]byte{8}, 32)}}
	v := &tmproto.Vote{Type: tmproto.PrecommitType, Height: h.Height, Round: 0, BlockID: id.ToProto(), Timestamp: h.Time, ValidatorAddress: key.PubKey().Address(), ValidatorIndex: 0}
	sig, err := key.Sign(tmtypes.VoteSignBytes(h.ChainID, v))
	require.NoError(t, err)
	c := &tmproto.Commit{Height: h.Height, Round: 0, BlockID: id.ToProto(), Signatures: []tmproto.CommitSig{{BlockIdFlag: tmproto.BlockIDFlagCommit, ValidatorAddress: v.ValidatorAddress, Timestamp: h.Time, Signature: sig}}}
	hb, err := h.ToProto().Marshal()
	require.NoError(t, err)
	cb, err := c.Marshal()
	require.NoError(t, err)
	return hb, cb
}

func TestRecoveryEvidenceProducer(t *testing.T) {
	output := os.Getenv("LAYERX_RECOVERY_EVIDENCE_OUTPUT")
	require.NotEmpty(t, output)
	at := time.Unix(1_800_000_000, 123_000_000).UTC()
	validator := sdked25519.GenPrivKey()
	oldOcc := app.EnableOCC
	app.EnableOCC = false
	t.Cleanup(func() { app.EnableOCC = oldOcc })
	w := app.NewTestWrapper(t, at, validator.PubKey(), true, func(ba *baseapp.BaseApp) { ba.SetOccEnabled(false); ba.SetConcurrencyWorkers(1) })
	a := w.App
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	sender := utils.NewSigner()
	a.EvmKeeper.SetAddressMapping(w.Ctx, sender.AccountAddress, sender.EvmAddress)
	w.FundAcc(sender.AccountAddress, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1_000_000_000_000)))
	nativeKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{73}, 32))
	var didKey [32]byte
	copy(didKey[:], nativeKey.Public().(ed25519.PublicKey))
	bindMessage, err := evmtypes.LayerXBindMessage(a.EvmKeeper.ChainID(w.Ctx), sender.EvmAddress, 0)
	require.NoError(t, err)
	var bindSig [64]byte
	copy(bindSig[:], ed25519.Sign(nativeKey, bindMessage))
	_, err = a.EvmKeeper.BindLayerX(w.Ctx, sender.EvmAddress, didKey, bindSig)
	require.NoError(t, err)
	actor, err := a.LayerXAuthKeeper.Actor(w.Ctx, sender.EvmAddress)
	require.NoError(t, err)
	registration := append(append([]byte{0x71, 1, 0, 2}, actor.Authority.Actor[:]...), didKey[:]...)
	registered, err := a.LayerXAuthKeeper.RecordLifecycle(w.Ctx, actor, registration, [32]byte{0x51})
	require.NoError(t, err)
	require.Zero(t, registered.ResultCode)
	receiptSeed := bytes.Repeat([]byte{37}, 32)
	var keyID, public [32]byte
	keyID[0] = 55
	copy(public[:], ed25519.NewKeyFromSeed(receiptSeed).Public().(ed25519.PublicKey))
	require.NoError(t, a.SequencerKeeper.InitGenesis(w.Ctx, seqtypes.GenesisState{
		ActivationHeight: 2, Leaders: [][32]byte{[32]byte(validator.PubKey().Bytes())},
		ReceiptKeys: []seqtypes.ReceiptKey{{KeyId: keyID, PublicKey: public, FirstHeight: 1, LastHeight: math.MaxUint64}},
	}))
	a.UpgradeKeeper.SetDone(w.Ctx, app.MergeUpgrade)
	params := a.GetConsensusParams(w.Ctx)
	params.Block.MaxGasWanted = params.Block.MaxGas
	a.StoreConsensusParams(w.Ctx, params)
	_, err = a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{Header: &tmproto.Header{ChainID: "pax-test", Height: 1, Time: at}})
	require.NoError(t, err)
	a.SetDeliverStateToCommit()
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	exec, err := sequencer.NewExecutor(a, sequencer.Options{ChainID: "pax-test", ProposerAddress: validator.PubKey().Address()})
	require.NoError(t, err)
	dir := t.TempDir()
	consensusFile := filepath.Join(dir, "consensus.json")
	pv, err := privval.GenFilePV(consensusFile, filepath.Join(dir, "state.json"), "")
	require.NoError(t, err)
	require.NoError(t, pv.Save())
	seedFile := filepath.Join(dir, "receipt-key")
	require.NoError(t, os.WriteFile(seedFile, []byte(hex.EncodeToString(receiptSeed)), 0o600))
	pins, err := a.SequencerKeeper.ReceiptKeyPins(a.GetCheckCtx())
	require.NoError(t, err)
	key, err := sequencer.LoadReceiptKey(seedFile, consensusFile, pins, 2)
	require.NoError(t, err)
	receipter, err := sequencer.NewReceipter(exec, a, &a.EvmKeeper, key, app.EVMReceiptModuleID)
	require.NoError(t, err)
	source, err := sequencer.NewConsensusSource(a, exec, key)
	require.NoError(t, err)
	a.SetSequencerSource(source)
	at = at.Add(time.Second)
	require.NoError(t, exec.Open(2, at.UnixMilli()))
	auth, err := layerxauth.NewPrecompile(a.GetPrecompileKeepers())
	require.NoError(t, err)
	root := [32]byte{0x61}
	recovery := append(append([]byte{0x71, 3, 0, 3}, actor.Authority.Actor[:]...), root[:]...)
	recovery = binary.BigEndian.AppendUint16(recovery, 2)
	good, err := auth.GetABI().Pack("recordLifecycle", recovery, [32]byte{0x52})
	require.NoError(t, err)
	badPayload := bytes.Clone(recovery)
	clear(badPayload[36:68])
	bad, err := auth.GetABI().Pack("recordLifecycle", badPayload, [32]byte{0x53})
	require.NoError(t, err)
	to := common.HexToAddress(layerxauth.AuthAddress)
	var receipts, raws [][]byte
	var activities [][32]byte
	for index, data := range [][]byte{good, good, bad} {
		target := to
		if index == 0 {
			target = utils.NewSigner().EvmAddress
			data = nil
		}
		signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.DynamicFeeTx{
			ChainID: a.EvmKeeper.ChainID(a.GetCheckCtx()), Nonce: uint64(index), GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(500_000_000_000),
			Gas: 2_000_000, To: &target, Value: new(big.Int), Data: data,
		}), ethtypes.LatestSignerForChainID(a.EvmKeeper.ChainID(a.GetCheckCtx())), sender.EvmPrivateKey)
		require.NoError(t, err)
		raw, err := signed.MarshalBinary()
		require.NoError(t, err)
		txData, err := ethtx.NewTxDataFromTx(signed)
		require.NoError(t, err)
		msg, err := evmtypes.NewMsgEVMTransaction(txData)
		require.NoError(t, err)
		builder := a.GetTxConfig().NewTxBuilder()
		require.NoError(t, builder.SetMsgs(msg))
		builder.SetGasLimit(signed.Gas())
		encoded, err := a.GetTxConfig().TxEncoder()(builder.GetTx())
		require.NoError(t, err)
		accepted, receipt, err := receipter.Accept(encoded)
		require.NoError(t, err)
		decoded, err := codec.DecodeReceipt(receipt)
		require.NoError(t, err)
		if index < 2 {
			require.Zero(t, decoded.ResultCode)
		} else {
			require.Equal(t, int32(1), decoded.ResultCode)
		}
		receipts = append(receipts, receipt)
		raws = append(raws, raw)
		activities = append(activities, accepted.ActivityID)
	}
	snapshot, err := exec.Snapshot()
	require.NoError(t, err)
	identity, err := a.LayerXHost.ReadIdentityState(snapshot, actor.DID)
	require.NoError(t, err)
	require.Equal(t, root, identity.RecoveryRoot)
	require.Equal(t, uint16(2), identity.RecoveryThreshold)
	sealed, err := source.Seal(context.Background(), 2)
	require.NoError(t, err)
	decodedBatch, err := app.DecodeSequenceBatch(sealed.Batch)
	require.NoError(t, err)
	require.NoError(t, seqtypes.VerifyBatchTxs(decodedBatch, activities))
	txs := tmtypes.Txs{tmtypes.Tx(sealed.Batch)}
	for _, tx := range sealed.Txs {
		txs = append(txs, tx)
	}
	require.Len(t, txs, 4)
	produced := source.Sealed()
	encodedResults, err := abci.MarshalTxResults(produced.Results)
	require.NoError(t, err)
	resultsRoot, resultProofs := merkle.ProofsFromByteSlices(encodedResults)
	consensusPublic, err := tmed25519.PublicKeyFromBytes(validator.PubKey().Bytes())
	require.NoError(t, err)
	set := tmtypes.NewValidatorSet([]*tmtypes.Validator{tmtypes.NewValidator(consensusPublic, 100)})
	empty := sha256.Sum256(nil)
	execution := &tmtypes.Header{
		Version: tmversion.Consensus{Block: 11, App: 1}, ChainID: "pax-test", Height: 2, Time: at,
		LastBlockID:    tmtypes.BlockID{Hash: empty[:], PartSetHeader: tmtypes.PartSetHeader{Total: 1, Hash: empty[:]}},
		LastCommitHash: empty[:], DataHash: txs.Hash(), ValidatorsHash: set.Hash(), NextValidatorsHash: set.Hash(),
		ConsensusHash: empty[:], AppHash: a.LastCommitID().Hash, LastResultsHash: empty[:], EvidenceHash: empty[:], ProposerAddress: validator.PubKey().Address(),
	}
	eh, ec := signedHeader(t, execution, validator)
	_, err = a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{Hash: execution.Hash(), Header: execution.ToProto(), Txs: txs.ToSliceOfBytes()})
	require.NoError(t, err)
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	successor := *execution
	successor.Height = 3
	successor.Time = at.Add(time.Second)
	successor.LastBlockID.Hash = execution.Hash()
	successor.AppHash = a.LastCommitID().Hash
	successor.LastResultsHash = resultsRoot
	sh, sc := signedHeader(t, &successor, validator)
	query, err := a.Query(context.Background(), &abci.RequestQuery{Path: "/store/sequencer/key", Data: []byte{3}, Height: 2, Prove: true})
	require.NoError(t, err)
	require.Zero(t, query.Code, query.Log)
	require.NotNil(t, query.ProofOps)
	require.Len(t, query.ProofOps.Ops, 2)
	keyProof := []byte("LXCP\x01")
	for _, value := range [][]byte{sh, sc, nil, []byte("sequencer"), {3}} {
		keyProof = field(keyProof, value)
	}
	keyProof = append(keyProof, 1)
	keyProof = field(keyProof, query.Value)
	keyProof = binary.BigEndian.AppendUint32(keyProof, uint32(len(query.ProofOps.Ops)))
	for _, op := range query.ProofOps.Ops {
		keyProof = field(keyProof, []byte(op.Type))
		keyProof = field(keyProof, op.Key)
		keyProof = field(keyProof, op.Data)
	}
	batchProof := txs.Proof(0).Proof
	cases := make([]any, 0, len(receipts))
	for index, receipt := range receipts {
		decoded, err := codec.DecodeReceipt(receipt)
		require.NoError(t, err)
		require.Equal(t, activities[index], decoded.ActivityID)
		result := produced.Results[index+1]
		cases = append(cases, map[string]any{
			"receipt": map[string]any{"version": 1, "principal": "alice", "evidence_id": "recovery-1", "canonical_receipt": b64(receipt), "header": b64(sh), "commit": b64(sc), "transaction_proof": ""},
			"inclusion": map[string]any{
				"containing_transaction": b64(sealed.Batch), "transaction_proof": b64(proofBytes(&batchProof)),
				"execution_header": b64(eh), "execution_commit": b64(ec), "activity_ids": activities, "receipt_index": index,
				"raw_transaction": b64(raws[index]), "receipt_keys_proof": b64(keyProof),
				"result_code": result.Code, "result_data": b64(result.Data), "gas_wanted": result.GasWanted, "gas_used": result.GasUsed,
				"result_proof": b64(proofBytes(resultProofs[index+1])), "recovery_did": actor.Authority.Actor,
				"recovery_root": root, "recovery_threshold": 2,
			},
		})
	}
	document, err := json.Marshal(map[string]any{"public_key": b64(validator.PubKey().Bytes()), "cases": cases})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(output, document, 0o600))
}
