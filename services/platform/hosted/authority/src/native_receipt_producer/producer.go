//go:build ignore

// This integration producer executes the native host and commits its actual
// module writes through the disk-backed SDK store, then exports real queries.
package main

/*
#cgo CFLAGS: -I${SRCDIR}/../../../../../../include -I${SRCDIR}/../../../../../../build/generated
#include <stdlib.h>
#include <string.h>
#include "layerx/lxp_batch_identity.h"
#include "layerx/lxp_receipt.h"

typedef struct sealed_batch {
    uint8_t memory[4194304];
    lxp_byte_span receipt;
    lxp_byte_span header;
    lxp_byte_span proof;
    uint8_t digest[32];
} sealed_batch;

static int32_t seal_native(const uint8_t *raw, size_t length,
    const uint8_t seed[32], const uint8_t key_id[32], uint64_t height,
    sealed_batch **output)
{
    sealed_batch *batch = calloc(1, sizeof(*batch));
    lxp_receipt *receipt = calloc(1, sizeof(*receipt));
    lxp_arena arena;
    lxp_batch_header header = {0};
    lxp_merkle_proof proof;
    lxp_codec_writer writer;
    int32_t status = batch && receipt ? LXP_OK : LXP_ERR_ARENA_EXHAUSTED;
#define STEP(call) do { if (status == LXP_OK) status = (call); } while (0)
    STEP(lxp_arena_init(&arena, batch->memory, sizeof(batch->memory)));
    STEP(lxp_receipt_decode(raw, length, false, receipt));
    if (status == LXP_OK) {
        if (memcmp(receipt->activity_root, (uint8_t[32]){0}, 32) == 0) {
            STEP(lxp_batch_identity_activity(receipt->previous_state_root,
                receipt->activity_id, receipt->global_sequence, height, receipt->batch_id));
        } else {
            STEP(lxp_batch_identity_committed(receipt->previous_state_root,
                receipt->activity_root, receipt->global_sequence,
                receipt->global_sequence, height, receipt->batch_id));
        }
        memcpy(receipt->context_hash, key_id, 32);
    }
    STEP(lxp_receipt_sign(receipt, seed, &arena));
    STEP(lxp_receipt_encode(receipt, true, &arena, &batch->receipt));
    if (status == LXP_OK) {
        header.protocol_version = receipt->protocol_version;
        header.network_id = 7;
        header.epoch = 1;
        header.batch_number = height;
        header.first_sequence = receipt->global_sequence;
        header.last_sequence = receipt->global_sequence;
        header.timestamp_ms = receipt->timestamp;
        memcpy(header.previous_state_root, receipt->previous_state_root, 32);
        memcpy(header.resulting_state_root, receipt->resulting_state_root, 32);
        memcpy(header.activity_merkle_root, receipt->activity_root, 32);
        memcpy(header.sequencer_id, key_id, 32);
    }
    uint8_t leaf[1][32];
    STEP(lxp_merkle_leaf_hash(batch->receipt.bytes, batch->receipt.length, leaf[0]));
    STEP(lxp_merkle_proof_generate(leaf, 1, 0, &arena, &proof, header.receipt_merkle_root));
    STEP(lxp_codec_writer_init(&writer, &arena, 64));
    STEP(lxp_merkle_proof_encode(&writer, &proof));
    if (status == LXP_OK) batch->proof = (lxp_byte_span){writer.bytes, writer.length};
    STEP(lxp_batch_header_encode(&header, &arena, &batch->header));
    STEP(lxp_batch_header_hash(&header, &arena, batch->digest));
#undef STEP
    free(receipt);
    if (status != LXP_OK) { free(batch); return status; }
    *output = batch;
    return LXP_OK;
}
*/
import "C"

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmed "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	tmjson "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/json"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/version"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	rootmulti "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
)

func checked(err error) {
	if err != nil {
		panic(err)
	}
}
func decoded(text string) []byte { value, err := hex.DecodeString(text); checked(err); return value }
func object(value any) any {
	data, err := tmjson.Marshal(value)
	checked(err)
	var result any
	checked(json.Unmarshal(data, &result))
	return result
}

func seal(raw []byte, height uint64, seedByte byte) (string, any) {
	seed := bytes.Repeat([]byte{seedByte}, 32)
	private := ed25519.NewKeyFromSeed(seed)
	public := private.Public().(ed25519.PublicKey)
	keyID := sha256.Sum256(public)
	var batch *C.sealed_batch
	status := C.seal_native((*C.uint8_t)(unsafe.Pointer(&raw[0])), C.size_t(len(raw)),
		(*C.uint8_t)(unsafe.Pointer(&seed[0])), (*C.uint8_t)(unsafe.Pointer(&keyID[0])), C.uint64_t(height), &batch)
	if status != C.LXP_OK {
		panic(fmt.Sprintf("native batch sealing: %d", status))
	}
	defer C.free(unsafe.Pointer(batch))
	encoded := func(span C.lxp_byte_span) string {
		return hex.EncodeToString(C.GoBytes(unsafe.Pointer(span.bytes), C.int(span.length)))
	}
	digest := C.GoBytes(unsafe.Pointer(&batch.digest[0]), 32)
	document := map[string]any{"authority_replica_id": hex.EncodeToString(bytes.Repeat([]byte{9}, 32)),
		"sequencer_public_key": hex.EncodeToString(public), "batch_evidence": map[string]any{
			"header_hex": encoded(batch.header), "header_signature": hex.EncodeToString(ed25519.Sign(private, digest)),
			"receipt_proof_hex": encoded(batch.proof)}}
	return encoded(batch.receipt), document
}

func main() {
	var input struct {
		Directory string
		Now       int64
		Grant     string
	}
	checked(json.NewDecoder(os.Stdin).Decode(&input))
	cfg := config.DefaultStateCommitConfig()
	cfg.MemIAVLConfig.AsyncCommitBuffer = 0
	key := storetypes.NewKVStoreKey("lxphost")
	open := func() *rootmulti.Store {
		store := rootmulti.NewStore(filepath.Join(input.Directory, "sdk-store"), cfg, config.StateStoreConfig{}, nil)
		store.MountStoreWithDB(key, storetypes.StoreTypeIAVL, nil)
		checked(store.LoadLatestVersion())
		return store
	}
	store := open()
	host, err := lxphost.NewHost(lxphost.Config{StoreKey: key, NetworkID: 7, Epoch: 1, ParameterVersion: 1, TimestampWindowMs: 60000, GasLimit: 1000000})
	checked(err)
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x61}, 32))
	public := private.Public().(ed25519.PublicKey)
	did := []byte("did:layerx:" + hex.EncodeToString(public))
	id, err := lxphost.DIDID(did)
	checked(err)
	var public32 [32]byte
	copy(public32[:], public)
	authority := lxphost.KernelAuthority{Actor: id, Principal: id, VerifiedKey: public32}
	execute := func(ordinal byte, sequence uint64, payload []byte) lxphost.Receipt {
		activity := lxphost.Activity{ProtocolVersion: 3, Type: 0x00070000 + uint32(ordinal), ActorDID: did, AuthorityKey: public32,
			AccountSequence: sequence - 1, NotBefore: uint64(input.Now) - 1000, NotAfter: uint64(input.Now) + 1000,
			IdempotencyKey: sha256.Sum256(payload), Payload: payload}
		hash, err := host.SigningHash(activity)
		checked(err)
		activity.Signature = ed25519.Sign(private, hash[:])
		cache := store.CacheMultiStore()
		ctx := lxphost.WithSequence(sdk.NewContext(cache, tmproto.Header{ChainID: "125", Height: int64(sequence), Time: time.UnixMilli(input.Now)}, false), sequence)
		receipt, err := host.HostExecute(ctx, activity, authority)
		checked(err)
		if receipt.ResultCode != 0 {
			panic(fmt.Sprintf("native execution %d: %v", ordinal, receipt.ResultCode))
		}
		cache.Write()
		return receipt
	}
	register := append(append([]byte{0x71, 1, 0, 2}, id[:]...), public...)
	execute(1, 1, register)
	roots := [][]byte{append([]byte(nil), store.Commit(true).Hash...)}
	next := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x62}, 32)).Public().(ed25519.PublicKey)
	rotation := append(append([]byte{0x71, 2, 0, 4}, id[:]...), next...)
	rotation = binary.BigEndian.AppendUint64(rotation, uint64(input.Now)+600000)
	rotation = binary.BigEndian.AppendUint64(rotation, uint64(input.Now)+1200000)
	rotation = binary.BigEndian.AppendUint64(rotation, 10000)
	rotated := execute(2, 2, rotation)
	roots = append(roots, append([]byte(nil), store.Commit(true).Hash...))
	action := sha256.Sum256([]byte("repair activity action"))
	session, err := authtypes.EncodeSessionPayload(decoded(input.Grant), 10000, action, nil)
	checked(err)
	granted := execute(5, 3, session)
	roots = append(roots, append([]byte(nil), store.Commit(true).Hash...))
	host.Close()
	for store.LastCommitID().Version < 8 {
		roots = append(roots, append([]byte(nil), store.Commit(true).Hash...))
	}
	// Stop and reopen the real store before producing any membership proofs.
	checked(store.Close())
	store = open()
	defer store.Close()
	if store.LastCommitID().Version != 8 {
		panic("SDK store recovery height")
	}
	cache := store.CacheMultiStore()
	iterator := cache.GetKVStore(key).Iterator(nil, nil)
	queries := map[string]any{}
	for ; iterator.Valid(); iterator.Next() {
		raw := append([]byte(nil), iterator.Key()...)
		if len(raw) < 3 || !bytes.Equal(raw[:3], []byte{1, 0, 7}) {
			continue
		}
		response := store.Query(context.Background(), abci.RequestQuery{Path: "/lxphost/key", Data: raw, Height: 8, Prove: true})
		if response.Code != 0 || response.ProofOps == nil || !bytes.Equal(response.Value, iterator.Value()) {
			panic("real SDK query refused")
		}
		queries[hex.EncodeToString(raw)] = map[string]any{"code": response.Code, "height": fmt.Sprint(response.Height), "key": response.Key, "value": response.Value, "proofOps": response.ProofOps}
	}
	checked(iterator.Close())
	validator := tmed.TestSecretKey([]byte("repair committed validator"))
	pv := privval.NewFilePV(validator, filepath.Join(input.Directory, "validator-key"), filepath.Join(input.Directory, "validator-state"))
	pub, err := pv.GetPubKey(context.Background())
	checked(err)
	validators := tmtypes.NewValidatorSet([]*tmtypes.Validator{tmtypes.NewValidator(pub, 10)})
	previous := bytes.Repeat([]byte{1}, 32)
	blocks := map[string]any{}
	var last *tmtypes.Header
	for height := int64(1); height <= 9; height++ {
		apphash := roots[0]
		if height > 1 {
			apphash = roots[height-2]
		}
		header := &tmtypes.Header{Version: version.Consensus{Block: version.BlockProtocol, App: 1}, ChainID: "125", Height: height,
			Time: time.UnixMilli(input.Now), LastBlockID: tmtypes.BlockID{Hash: previous, PartSetHeader: tmtypes.PartSetHeader{Total: 1, Hash: previous}},
			ValidatorsHash: validators.Hash(), NextValidatorsHash: validators.Hash(), AppHash: apphash,
			ProposerAddress: pub.Address(), DataHash: sha256Bytes("transactions"), LastResultsHash: sha256Bytes("receipts")}
		hash := header.Hash()
		blocks[fmt.Sprintf("0x%x", height)] = map[string]any{"number": fmt.Sprintf("0x%x", height), "hash": "0x" + hex.EncodeToString(hash),
			"parentHash": "0x" + hex.EncodeToString(previous), "timestamp": fmt.Sprintf("0x%x", input.Now/1000), "stateRoot": "0x" + hex.EncodeToString(apphash),
			"transactionsRoot": "0x" + hex.EncodeToString(header.DataHash), "receiptsRoot": "0x" + hex.EncodeToString(header.LastResultsHash), "transactions": []any{}}
		previous = hash
		last = header
	}
	blockID := tmtypes.BlockID{Hash: last.Hash(), PartSetHeader: tmtypes.PartSetHeader{Total: 1, Hash: sha256Bytes("parts")}}
	votes := tmtypes.NewVoteSet("125", 9, 0, tmproto.PrecommitType, validators)
	commit, err := tmtypes.MakeCommit(context.Background(), blockID, 9, 0, votes, []tmtypes.PrivValidator{pv}, last.Time)
	checked(err)
	checked(tmtypes.VerifyCommitLight("125", validators, blockID, 9, commit))
	rotationReceipt, rotationEvidence := seal(rotated.Encoded, 2, 0x51)
	grantReceipt, grantEvidence := seal(granted.Encoded, 3, 0x52)
	output, err := os.Create(filepath.Join(input.Directory, "corpus.json"))
	checked(err)
	defer output.Close()
	checked(json.NewEncoder(output).Encode(map[string]any{"did": string(did), "did_id": hex.EncodeToString(id[:]),
		"rotation": rotationReceipt, "grant": grantReceipt, "rotation_evidence": rotationEvidence, "grant_evidence": grantEvidence, "queries": queries, "blocks": blocks,
		"commit": map[string]any{"height": "9", "signedHeader": map[string]any{"header": object(last), "commit": object(commit)}}, "action": hex.EncodeToString(action[:])}))
}
func sha256Bytes(text string) []byte { value := sha256.Sum256([]byte(text)); return value[:] }
