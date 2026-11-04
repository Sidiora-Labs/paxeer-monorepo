package oracle

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	upgradekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
)

const submitABIBytes = 4 + 3*32
const submitTransportBytes = 4 + 2 + 32 + 3*8 + 32 + 16 + 137 + 64
const oraclePushActivity uint32 = 0x00060003

type Precompile struct {
	*agentcommon.Precompile
	executor *OraclePrecompile
	upgrade  *upgradekeeper.Keeper
	planName string
}

// Run extracts the signed transport into a call-local executor. The shared
// executor and the EVM context are never mutated, including during nested calls.
func (p *Precompile) Run(evm *vm.EVM, caller, callingContract common.Address, input []byte, value *big.Int, readOnly, delegate bool, hooks *tracing.Hooks) ([]byte, error) {
	if p == nil || p.executor == nil {
		return nil, fmt.Errorf("%w: oracle executor is unavailable", agentcommon.ErrNoContext)
	}
	executor := *p.executor
	if len(input) >= 4 && bytes.Equal(input[:4], p.GetABI().Methods[SubmitPriceMethod].ID) && len(input) > submitABIBytes {
		executor.transport = input[submitABIBytes:]
	}
	call := agentcommon.NewPrecompile(p.GetABI(), &executor, p.Address(), p.GetName(), p.upgrade, p.planName)
	return call.Run(evm, caller, callingContract, input, value, readOnly, delegate, hooks)
}

func oracleDID(key [32]byte) []byte { return []byte("did:layerx:" + hex.EncodeToString(key[:])) }

type Observation struct {
	FeedID                                 [32]byte
	Price                                  *big.Int
	Sequence, ObservedAt, SourceIdentifier uint64
}

// SignObservation produces both signatures required by the native oracle intake.
func SignObservation(host *lxphost.Host, private ed25519.PrivateKey, observation Observation, identitySequence, notBefore, notAfter uint64) (lxphost.Activity, error) {
	if host == nil || len(private) != ed25519.PrivateKeySize || observation.FeedID == ([32]byte{}) || observation.Price == nil || observation.Price.Sign() <= 0 || observation.Price.BitLen() > 128 || observation.Sequence == 0 || notAfter < notBefore {
		return lxphost.Activity{}, fmt.Errorf("%w: invalid oracle observation", agentcommon.ErrABIDecode)
	}
	payload := append([]byte(nil), observation.FeedID[:]...)
	payload = binary.BigEndian.AppendUint64(payload, observation.Sequence)
	var price [16]byte
	observation.Price.FillBytes(price[:])
	payload = append(payload, price[:]...)
	payload = binary.BigEndian.AppendUint64(payload, observation.ObservedAt)
	payload = binary.BigEndian.AppendUint64(payload, observation.SourceIdentifier)
	digest, err := codec.DomainHash(codec.DomainSignaturePreimage, append([]byte("LXP:ORACLE:OBSERVATION:v1"), payload...))
	if err != nil {
		return lxphost.Activity{}, err
	}
	transport := append([]byte{1}, payload...)
	transport = append(transport, ed25519.Sign(private, digest[:])...)
	var key [32]byte
	copy(key[:], private.Public().(ed25519.PublicKey))
	idempotency := binary.BigEndian.AppendUint64(append([]byte(nil), key[:]...), identitySequence)
	idempotency = append(idempotency, transport...)
	a := lxphost.Activity{ProtocolVersion: 3, Type: oraclePushActivity, ActorDID: oracleDID(key), AuthorityKey: key, AccountSequence: identitySequence, NotBefore: notBefore, NotAfter: notAfter, IdempotencyKey: sha256.Sum256(idempotency), Payload: transport}
	digest, err = host.SigningHash(a)
	if err != nil {
		return lxphost.Activity{}, err
	}
	a.Signature = ed25519.Sign(private, digest[:])
	return a, nil
}

// PackSubmitPrice appends the versioned LXO1 transport to the existing ABI call.
// All integers are big endian: protocol(u16), key(32), identity sequence(u64),
// not-before(u64), not-after(u64), idempotency(32), fee limit(16), observation
// transport(137), activity signature(64). The host's SigningHash signs the outer
// activity; the observation contains its own domain-separated signature.
func PackSubmitPrice(feed [32]byte, price, confidence *big.Int, activity lxphost.Activity) ([]byte, error) {
	if activity.Type != oraclePushActivity || len(activity.Payload) != 137 || len(activity.Signature) != 64 || len(activity.ReceiptAuthorities) != 0 || !bytes.Equal(activity.ActorDID, oracleDID(activity.AuthorityKey)) {
		return nil, fmt.Errorf("%w: invalid signed oracle activity", agentcommon.ErrABIDecode)
	}
	a, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return nil, err
	}
	input, err := a.Pack(SubmitPriceMethod, feed, price, confidence)
	if err != nil {
		return nil, err
	}
	input = append(input, 'L', 'X', 'O', 1)
	input = binary.BigEndian.AppendUint16(input, activity.ProtocolVersion)
	input = append(input, activity.AuthorityKey[:]...)
	input = binary.BigEndian.AppendUint64(input, activity.AccountSequence)
	input = binary.BigEndian.AppendUint64(input, activity.NotBefore)
	input = binary.BigEndian.AppendUint64(input, activity.NotAfter)
	input = append(input, activity.IdempotencyKey[:]...)
	input = append(input, activity.FeeLimit[:]...)
	input = append(input, activity.Payload...)
	return append(input, activity.Signature...), nil
}

func decodeSubmitTransport(input []byte) (lxphost.Activity, error) {
	if len(input) != submitTransportBytes || !bytes.Equal(input[:4], []byte{'L', 'X', 'O', 1}) {
		return lxphost.Activity{}, fmt.Errorf("%w: invalid LXO1 signed oracle transport", agentcommon.ErrABIDecode)
	}
	a := lxphost.Activity{Type: oraclePushActivity, ProtocolVersion: binary.BigEndian.Uint16(input[4:6])}
	copy(a.AuthorityKey[:], input[6:38])
	a.ActorDID = oracleDID(a.AuthorityKey)
	a.AccountSequence = binary.BigEndian.Uint64(input[38:46])
	a.NotBefore = binary.BigEndian.Uint64(input[46:54])
	a.NotAfter = binary.BigEndian.Uint64(input[54:62])
	copy(a.IdempotencyKey[:], input[62:94])
	copy(a.FeeLimit[:], input[94:110])
	a.Payload = bytes.Clone(input[110:247])
	a.Signature = bytes.Clone(input[247:311])
	return a, nil
}
