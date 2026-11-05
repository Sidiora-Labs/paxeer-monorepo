package evmrpc_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	seqnode "github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	occutils "github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

var updateSequencerReceiptVectors = flag.Bool("update", false, "rewrite sequencer receipt conformance fixtures")

const (
	seqVecName            = "sequencer-receipt-v1"
	seqVecFixture         = "../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
	seqVecFeeTokenFixture = "../sdk/platform/conformance/fixtures/sequencer-receipt-fee-token-v1.json"
	seqVecFeeTokenCommand = "go test -count=1 ./rpc/ -run '^TestSequencerReceiptFeeTokenVector$' -update"
	seqVecUCPFixture      = "../sdk/platform/conformance/fixtures/ucp-checkout-receipt-v1.json"
	seqVecUCPCommand      = "go test -count=1 ./rpc/ -run '^TestRepairM_466_UCPCheckoutReceiptVector$' -update"
	seqVecUCPDomain       = "LayerX/interop/ucp/idempotency/v1\x00"
	seqVecUCPIdempotency  = "123e4567-e89b-12d3-a456-426614174000"
	seqVecFiatFixture     = "../sdk/platform/conformance/fixtures/fiat-receipts-v1.json"
	seqVecFiatJourneys    = "../services/interop/specs/conformance/fiat/sequenced-journeys.json"
	seqVecFiatSource      = "../daemon/committed-fixture/vectors/vector-v4-committed.json"
	seqVecFiatSourceHash  = "5013cd232ee94ece8d25fca4c802ca60c892f3a5f637a70962d9909dc71c4e7b"
	seqVecFiatCommand     = "go test -count=1 ./rpc/ -run '^TestRepairM_489_FiatJourneysDerived$' -update"
	seqVecGenerator       = "rpc/sequencer_receipt_vectors_test.go"
	seqVecCommand         = "go test -count=1 ./rpc/ -run TestSequencerReceiptVectors -update"
	seqVecChainID         = 125
	seqVecModuleID        = seqnode.KernelReservedModules + 1
	seqVecFirstHeight     = 1000
	seqVecLastHeight      = 1999
	seqVecBaseFee         = 1_000_000_000
	seqVecTipCap          = 1_000_000_000
	seqVecFeeCap          = 3_000_000_000
	seqVecTimeBase        = 1_750_000_000_000
	seqVecBlockMs         = 400
	seqVecInsufficient    = "LXP_INSUFFICIENT_BALANCE"
)

var (
	seqVecSenderSecret = common.FromHex("0x8f2a55949038a9610f50fb23b5883af3b4ecb3c3bb792cbcefbd1542c692be63")
	seqVecReceiptSeed  = bytes.Repeat([]byte{0x5e}, ed25519.SeedSize)
	seqVecKeyID        = [32]byte(bytes.Repeat([]byte{0x6b}, 32))
	seqVecRogueKeyID   = [32]byte(bytes.Repeat([]byte{0x6c}, 32))
	seqVecRecipient    = common.HexToAddress("0x8ba1f109551bd432803012645ac136ddd64dba72")
	seqVecToken        = common.HexToAddress("0x5fbdb2315678afecb367f032d93f642f64180aa3")
	seqVecSpender      = common.HexToAddress("0x70997970c51812dc3a010c7d01b50e0d17dc79c8")
	seqVecBridgeTo     = common.HexToAddress("0x3c44cdddb6a900fa2b585dd299e03d12fa4293bc")
	seqVecLedger       = common.HexToAddress("0x000000000000000000000000000000000000101A")
	seqVecBridge       = common.HexToAddress("0x0000000000000000000000000000000000001016")
	seqVecOnePAX       = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	seqVecStartBalance = new(big.Int).Mul(big.NewInt(100), seqVecOnePAX)

	errSeqVecRung     = errors.New("R1: answer is not receipted or committed with receipt bytes")
	errSeqVecKeyID    = errors.New("R3: keyId is not in the pinned ReceiptKeys")
	errSeqVecHeight   = errors.New("R3: batch_id height is outside the pinned key range")
	errSeqVecActivity = errors.New("R4: activity_id is not keccak256 of the raw transaction")
	errSeqVecSeqHash  = errors.New("R6: seqHash does not chain prevSeqHash, seq and activity_id")
)

type seqVecPin struct {
	KeyID       common.Hash   `json:"keyId"`
	PublicKey   hexutil.Bytes `json:"publicKey"`
	FirstHeight uint64        `json:"firstHeight"`
	LastHeight  uint64        `json:"lastHeight"`
}

type seqVecExpected struct {
	Rung      string `json:"rung,omitempty"`
	RefusedAt string `json:"refusedAt,omitempty"`
	Check     string `json:"check,omitempty"`
	Verifier  string `json:"verifier"`
}

type seqVecCase struct {
	ID             string          `json:"id"`
	Vector         string          `json:"vector"`
	Description    string          `json:"description"`
	RawTransaction hexutil.Bytes   `json:"rawTransaction"`
	ReceiptJSON    json.RawMessage `json:"receiptJson,omitempty"`
	SignedReceipt  hexutil.Bytes   `json:"signedReceipt"`
	ReceiptDigest  common.Hash     `json:"receiptDigest"`
	PrevSeqHash    common.Hash     `json:"prevSeqHash"`
	PinnedKey      seqVecPin       `json:"pinnedKey"`
	LXPCode        string          `json:"lxpCode,omitempty"`
	Expected       seqVecExpected  `json:"expected"`
	PaymentDenom   string          `json:"paymentDenom,omitempty"`
	FeeDenom       string          `json:"feeDenom,omitempty"`
	Asset          *common.Hash    `json:"asset,omitempty"`
}

type seqVecProvenance struct {
	Generator string `json:"generator"`
	Command   string `json:"command"`
	Source    string `json:"source,omitempty"`
	SHA256    string `json:"sha256,omitempty"`
}

type seqVecFile struct {
	Name       string           `json:"name"`
	Provenance seqVecProvenance `json:"provenance"`
	ChainID    uint64           `json:"chainId"`
	ModuleID   uint16           `json:"moduleId"`
	FeeDenom   string           `json:"feeDenom"`
	Cases      []seqVecCase     `json:"cases"`
}

// seqVecSource is the sequencer send record the vectors are served from.
type seqVecSource struct {
	mu    sync.Mutex
	sends map[common.Hash]*evmrpc.SequencerSend
}

func (s *seqVecSource) SequencerSend(txHash common.Hash) (*evmrpc.SequencerSend, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sends[txHash], nil
}

func (s *seqVecSource) put(txHash common.Hash, send *evmrpc.SequencerSend) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sends[txHash] = send
}

// seqVecTx is one EIP-1559 transaction of the vectors and its execution result.
type seqVecTx struct {
	nonce      uint64
	to         common.Address
	value      *big.Int
	data       []byte
	gasLimit   uint64
	gasUsed    uint64
	status     uint32
	returnData []byte
	logs       []*evmtypes.Log
	vmError    string
}

// seqVecSlot is the sequence position and open block of one receipt.
type seqVecSlot struct {
	seq    uint64
	prev   [32]byte
	height int64
	index  int
}

// seqVecSigned is one receipt as the node signer produced it.
type seqVecSigned struct {
	raw      []byte
	activity common.Hash
	unsigned []byte
	signed   []byte
	digest   [32]byte
	seqHash  [32]byte
	send     *evmrpc.SequencerSend
}

// seqVecBuilder signs the vector transactions and their receipts and tracks the sender balance
// and the cumulative gas of each open block along the accepted chain.
type seqVecBuilder struct {
	t            *testing.T
	sender       common.Address
	ethSigner    ethtypes.Signer
	balance      *big.Int
	cumulative   map[int64]uint64
	paymentDenom string
}

func newSeqVecBuilder(t *testing.T) *seqVecBuilder {
	t.Helper()
	key, err := crypto.ToECDSA(seqVecSenderSecret)
	require.NoError(t, err)
	return &seqVecBuilder{
		t:            t,
		sender:       crypto.PubkeyToAddress(key.PublicKey),
		ethSigner:    ethtypes.NewLondonSigner(big.NewInt(seqVecChainID)),
		balance:      new(big.Int).Set(seqVecStartBalance),
		cumulative:   map[int64]uint64{},
		paymentDenom: evmkeeper.BaseDenom,
	}
}

func (b *seqVecBuilder) signTx(tx seqVecTx) *ethtypes.Transaction {
	b.t.Helper()
	key, err := crypto.ToECDSA(seqVecSenderSecret)
	require.NoError(b.t, err)
	to := tx.to
	signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		ChainID:   big.NewInt(seqVecChainID),
		Nonce:     tx.nonce,
		GasTipCap: big.NewInt(seqVecTipCap),
		GasFeeCap: big.NewInt(seqVecFeeCap),
		Gas:       tx.gasLimit,
		To:        &to,
		Value:     tx.value,
		Data:      tx.data,
	}), b.ethSigner, key)
	require.NoError(b.t, err)
	sender, err := ethtypes.Sender(b.ethSigner, signed)
	require.NoError(b.t, err)
	require.Equal(b.t, b.sender, sender)
	return signed
}

// receipt executes nothing: it encodes the given execution result of signedTx with BuildReceipt,
// signs it with key and checks the signed bytes against the unsigned encoding. advance moves the
// sender balance and the block's cumulative gas forward, as an accepted receipt does.
func (b *seqVecBuilder) receipt(key *seqnode.ReceiptKey, keyID [32]byte, signedTx *ethtypes.Transaction, tx seqVecTx, slot seqVecSlot, advance bool) seqVecSigned {
	b.t.Helper()
	raw, err := signedTx.MarshalBinary()
	require.NoError(b.t, err)
	activity := signedTx.Hash()
	require.Equal(b.t, crypto.Keccak256Hash(raw), activity)
	tip, err := signedTx.EffectiveGasTip(big.NewInt(seqVecBaseFee))
	require.NoError(b.t, err)
	price := new(big.Int).Add(tip, big.NewInt(seqVecBaseFee))
	require.True(b.t, price.IsUint64())
	fee := new(big.Int).Mul(new(big.Int).SetUint64(tx.gasUsed), price)
	spent := new(big.Int).Set(fee)
	transferred := new(big.Int)
	if tx.status == uint32(ethtypes.ReceiptStatusSuccessful) {
		transferred.Set(tx.value)
		spent.Add(spent, tx.value)
	}
	fromAfter := new(big.Int).Sub(b.balance, spent)
	cumulative := b.cumulative[slot.height] + tx.gasUsed
	seqHash := seqReceiptNextHash9005(slot.prev, slot.seq, activity)
	in := &seqnode.ReceiptInput{
		Accepted: &seqnode.Accepted{
			Seq:         slot.seq,
			PrevSeqHash: slot.prev,
			SeqHash:     seqHash,
			ActivityID:  activity,
			Height:      slot.height,
			TxIndex:     slot.index,
			TimestampMs: seqVecTimeBase + slot.height*seqVecBlockMs,
		},
		EVM: &evmtypes.Receipt{
			Status:            tx.status,
			GasUsed:           tx.gasUsed,
			CumulativeGasUsed: cumulative,
			EffectiveGasPrice: price.Uint64(),
			To:                tx.to.Hex(),
			Logs:              tx.logs,
			VmError:           tx.vmError,
		},
		Operation:         signedTx.Type(),
		ReturnData:        tx.returnData,
		PaymentDenom:      b.paymentDenom,
		FeeDenom:          evmkeeper.BaseDenom,
		FeeCharged:        fee,
		Amount:            tx.value,
		From:              b.sender,
		FromBalanceBefore: new(big.Int).Set(b.balance),
		FromBalanceAfter:  fromAfter,
		FromSequence:      signedTx.Nonce(),
		To:                tx.to,
		ToBalanceBefore:   new(big.Int),
		ToBalanceAfter:    transferred,
		ModuleID:          seqVecModuleID,
	}
	unsigned, err := seqnode.BuildReceipt(in, keyID)
	require.NoError(b.t, err)
	signed, err := key.SignReceipt(in)
	require.NoError(b.t, err)
	digest := codec.ReceiptDigest(unsigned)
	require.Equal(b.t, unsigned[:len(unsigned)-1], signed[:len(unsigned)-1], "the signed receipt extends the BuildReceipt encoding")
	require.Equal(b.t, []byte{1, 0, 0, 0, ed25519.SignatureSize}, signed[len(unsigned)-1:len(unsigned)+4])
	public := key.PublicKey()
	require.True(b.t, ed25519.Verify(public[:], digest[:], signed[len(unsigned)+4:]))
	require.Len(b.t, signed, len(unsigned)+4+ed25519.SignatureSize)
	if advance {
		b.balance, b.cumulative[slot.height] = fromAfter, cumulative
	}
	logData := make([][]byte, len(tx.logs))
	for i, log := range tx.logs {
		logData[i] = log.Data
	}
	return seqVecSigned{
		raw:      raw,
		activity: activity,
		unsigned: unsigned,
		signed:   signed,
		digest:   digest,
		seqHash:  seqHash,
		send: &evmrpc.SequencerSend{
			Nonce:      signedTx.Nonce(),
			Receipt:    signed,
			ReturnData: tx.returnData,
			LogData:    logData,
			VMError:    tx.vmError,
		},
	}
}

// loadSeqVecKey loads the fixed receipt seed through the node loader with pin as its only entry.
func loadSeqVecKey(t *testing.T, dir string, pin verify.SequencerAuthorization) *seqnode.ReceiptKey {
	t.Helper()
	keyFile := filepath.Join(dir, "receipt_key")
	consensusFile := filepath.Join(dir, "priv_validator_key.json")
	if _, err := os.Stat(keyFile); errors.Is(err, os.ErrNotExist) {
		require.NoError(t, os.WriteFile(keyFile, []byte(hex.EncodeToString(seqVecReceiptSeed)+"\n"), 0o600))
		pv, err := privval.GenFilePV(consensusFile, filepath.Join(dir, "priv_validator_state.json"), "")
		require.NoError(t, err)
		require.NoError(t, pv.Key.Save())
	}
	key, err := seqnode.LoadReceiptKey(keyFile, consensusFile, []verify.SequencerAuthorization{pin}, int64(pin.FirstBatchNumber))
	require.NoError(t, err)
	return key
}

func seqVecWord(v *big.Int) common.Hash { return common.BigToHash(v) }

func seqVecArgs(t *testing.T, types ...string) abi.Arguments {
	t.Helper()
	args := make(abi.Arguments, len(types))
	for i, name := range types {
		typ, err := abi.NewType(name, "", nil)
		require.NoError(t, err)
		args[i] = abi.Argument{Type: typ}
	}
	return args
}

func seqVecSelector(signature string) []byte { return crypto.Keccak256([]byte(signature))[:4] }

func seqVecLog(address common.Address, data []byte, topics ...common.Hash) *evmtypes.Log {
	hexTopics := make([]string, len(topics))
	for i, topic := range topics {
		hexTopics[i] = topic.Hex()
	}
	return &evmtypes.Log{Address: address.Hex(), Topics: hexTopics, Data: data}
}

// seqVecApprove is an ERC-20 approve of spender with its Approval log.
func seqVecApprove(t *testing.T, owner common.Address, nonce uint64) seqVecTx {
	t.Helper()
	selector := seqVecSelector("approve(address,uint256)")
	require.Equal(t, common.FromHex("0x095ea7b3"), selector)
	allowance := new(big.Int).Mul(big.NewInt(250), seqVecOnePAX)
	args, err := seqVecArgs(t, "address", "uint256").Pack(seqVecSpender, allowance)
	require.NoError(t, err)
	approval := crypto.Keccak256Hash([]byte("Approval(address,address,uint256)"))
	require.Equal(t, common.HexToHash("0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925"), approval)
	return seqVecTx{
		nonce: nonce, to: seqVecToken, value: new(big.Int), data: append(selector, args...),
		gasLimit: 60_000, gasUsed: 46_220, status: uint32(ethtypes.ReceiptStatusSuccessful),
		returnData: seqVecWord(big.NewInt(1)).Bytes(),
		logs: []*evmtypes.Log{seqVecLog(seqVecToken, seqVecWord(allowance).Bytes(),
			approval, common.BytesToHash(owner.Bytes()), common.BytesToHash(seqVecSpender.Bytes()))},
	}
}

type seqVecPayExact struct {
	from, to, asset, reference common.Hash
	amount                     *big.Int
	expiresAt                  uint64
}

// seqVecPayExactCall is the layerxledger payExact calldata of address-table.md a2.
func seqVecPayExactCall(t *testing.T, pay seqVecPayExact) []byte {
	t.Helper()
	selector := seqVecSelector("payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)")
	require.Equal(t, common.FromHex("0x78ea11db"), selector)
	args, err := seqVecArgs(t, "bytes32", "bytes32", "bytes32", "uint128", "uint64", "bytes32").
		Pack([32]byte(pay.from), [32]byte(pay.to), [32]byte(pay.asset), pay.amount, pay.expiresAt, [32]byte(pay.reference))
	require.NoError(t, err)
	return append(selector, args...)
}

func seqVecLedgerPay(t *testing.T) seqVecPayExact {
	t.Helper()
	to, err := codec.DeriveAccountID([]byte("agent:did:layerx:payee:main"))
	require.NoError(t, err)
	return seqVecPayExact{
		from:      crypto.Keccak256Hash([]byte("vector payer account")),
		to:        common.Hash(to),
		asset:     crypto.Keccak256Hash([]byte("vector ledger asset")),
		reference: crypto.Keccak256Hash([]byte("vector payment reference")),
		amount:    big.NewInt(1_000_000),
		expiresAt: seqVecTimeBase + 3_600_000,
	}
}

// seqVecLedgerPayment is a layerxledger payExact with its PaymentExact log.
func seqVecLedgerPayment(t *testing.T, nonce uint64) seqVecTx {
	t.Helper()
	return seqVecLedgerPaymentFor(t, nonce, seqVecLedgerPay(t))
}

func seqVecLedgerPaymentFor(t *testing.T, nonce uint64, pay seqVecPayExact) seqVecTx {
	t.Helper()
	data, err := seqVecArgs(t, "bytes32", "uint128").Pack([32]byte(pay.asset), pay.amount)
	require.NoError(t, err)
	topic := crypto.Keccak256Hash([]byte("PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)"))
	return seqVecTx{
		nonce: nonce, to: seqVecLedger, value: new(big.Int), data: seqVecPayExactCall(t, pay),
		gasLimit: 90_000, gasUsed: 61_340, status: uint32(ethtypes.ReceiptStatusSuccessful),
		logs: []*evmtypes.Log{seqVecLog(seqVecLedger, data, topic, pay.reference, pay.from, pay.to)},
	}
}

// seqVecLedgerRevert is a layerxledger payExact refused with LXP_INSUFFICIENT_BALANCE.
func seqVecLedgerRevert(t *testing.T, nonce uint64) seqVecTx {
	t.Helper()
	pay := seqVecLedgerPay(t)
	pay.amount = new(big.Int).Lsh(big.NewInt(1), 100)
	pay.reference = crypto.Keccak256Hash([]byte("vector unfunded payment reference"))
	return seqVecTx{
		nonce: nonce, to: seqVecLedger, value: new(big.Int), data: seqVecPayExactCall(t, pay),
		gasLimit: 90_000, gasUsed: 33_120, status: uint32(ethtypes.ReceiptStatusFailed),
		returnData: pcommon.LXPRevert(seqVecInsufficient, ""), vmError: "execution reverted",
	}
}

// seqVecBridgeOut is a layerxbridge bridgeOut with its BridgeOut log, from the precompile ABI.
func seqVecBridgeOut(t *testing.T, nonce uint64) seqVecTx {
	t.Helper()
	raw, err := os.ReadFile("../precompiles/layerxbridge/abi.json")
	require.NoError(t, err)
	bridge, err := abi.JSON(bytes.NewReader(raw))
	require.NoError(t, err)
	const remoteChain, bridgeNonce = uint64(1), uint64(7)
	amount := new(big.Int).Mul(big.NewInt(5), seqVecOnePAX)
	data, err := bridge.Pack("bridgeOut", remoteChain, seqVecToken, amount, seqVecBridgeTo)
	require.NoError(t, err)
	returnData, err := bridge.Methods["bridgeOut"].Outputs.Pack(bridgeNonce)
	require.NoError(t, err)
	event := bridge.Events["BridgeOut"]
	require.Equal(t, crypto.Keccak256Hash([]byte("BridgeOut(uint64,address,uint256,address,uint64)")), event.ID)
	logData, err := event.Inputs.NonIndexed().Pack(amount, seqVecBridgeTo)
	require.NoError(t, err)
	return seqVecTx{
		nonce: nonce, to: seqVecBridge, value: new(big.Int), data: data,
		gasLimit: 120_000, gasUsed: 78_512, status: uint32(ethtypes.ReceiptStatusSuccessful),
		returnData: returnData,
		logs: []*evmtypes.Log{seqVecLog(seqVecBridge, logData, event.ID,
			seqVecWord(new(big.Int).SetUint64(remoteChain)), common.BytesToHash(seqVecToken.Bytes()),
			seqVecWord(new(big.Int).SetUint64(bridgeNonce)))},
	}
}

func seqVecProgramsCall(t *testing.T, b *seqVecBuilder, nonce uint64) seqVecTx {
	t.Helper()
	a := app.Setup(t, false, false, false)
	ctx, _ := a.GetContextForDeliverTx(nil).CacheContext()
	ctx = ctx.WithBlockHeight(seqVecFirstHeight + 2).WithBlockTime(time.UnixMilli(seqVecTimeBase + (seqVecFirstHeight+2)*seqVecBlockMs))
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx)).WithEventManager(sdk.NewEventManager())
	key := a.GetKey(app.LayerXHostStoreKey)
	host, err := lxphost.NewHost(lxphost.Config{StoreKey: key, NetworkID: seqVecChainID, ParameterVersion: 1, TimestampWindowMs: 60000, GasLimit: 1000000})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x41}, ed25519.SeedSize))
	publicKey := [32]byte(private.Public().(ed25519.PublicKey))
	message, err := evmtypes.LayerXBindMessage(a.EvmKeeper.ChainID(ctx), b.sender, 0)
	require.NoError(t, err)
	_, err = a.EvmKeeper.BindLayerX(ctx, b.sender, publicKey, [64]byte(ed25519.Sign(private, message)))
	require.NoError(t, err)
	did := []byte(evmtypes.LayerXDid(publicKey))
	actor, err := lxphost.DIDID(did)
	require.NoError(t, err)
	authorityHash, err := lxphost.AuthorityHash(lxphost.AuthorityKindOwner, [32]byte{}, publicKey)
	require.NoError(t, err)
	authority := lxphost.KernelAuthority{Kind: lxphost.AuthorityKindOwner, AuthorityHash: authorityHash, Bound: true, Actor: actor, Principal: actor, VerifiedKey: publicKey}
	moduleKey := func(key []byte) []byte { return append([]byte{1, 0, 9}, key...) }
	store := ctx.KVStore(key)
	initialFee := binary.BigEndian.AppendUint32([]byte("LXFR1"), 1)
	for i := 0; i < 7; i++ {
		initialFee = binary.BigEndian.AppendUint64(initialFee, 1)
	}
	occupancyAsset := [32]byte{0x91}
	initialFee = append(initialFee, occupancyAsset[:]...)
	for _, value := range []uint64{1000, 8, 1, 8, 1, 100, 1, 0, 1} {
		initialFee = binary.BigEndian.AppendUint64(initialFee, value)
	}
	initialFee = append(initialFee, authority.Actor[:]...)
	initialFee = append(initialFee, make([]byte, 16)...)
	store.Set(moduleKey([]byte("progfee/active/v1")), initialFee)
	store.Set(moduleKey(binary.BigEndian.AppendUint32([]byte("progfee/history/v1/"), 1)), initialFee)
	metering := binary.BigEndian.AppendUint32([]byte("LXMR1"), 1)
	for _, coefficient := range []uint64{1, 1, 1, 1, 1, 8, 8, 64, 8} {
		metering = binary.BigEndian.AppendUint64(metering, coefficient)
	}
	metering = binary.BigEndian.AppendUint64(metering, 1)
	metering = append(metering, 1)
	metering = append(metering, authority.Actor[:]...)
	store.Set(moduleKey([]byte("progmet/active/v1")), metering)
	store.Set(moduleKey(binary.BigEndian.AppendUint32([]byte("progmet/history/v1/"), 1)), metering)

	execute := func(activityType uint32, payload []byte) {
		sequence, err := host.ReadIdentitySequence(ctx, did)
		require.NoError(t, err)
		last, err := host.ReadLastSequence(ctx, did)
		require.NoError(t, err)
		receipt, err := lxphost.ExecuteActivity(lxphost.WithSequence(ctx, last+1), host, authority, activityType, payload, sequence)
		require.NoError(t, err)
		require.Zero(t, receipt.ResultCode)
	}
	salt := bytes.Repeat([]byte{0x55}, 32)
	assetMaterial := append([]byte("LX:ASSET:v1"), authority.Actor[:]...)
	asset := sha256.Sum256(append(assetMaterial, salt...))
	register := append([]byte{0, 1}, asset[:]...)
	register = append(register, salt...)
	register = append(register, 1, 'T', 1, 'T', 6)
	register = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(register, 0), 1000000000)
	execute(0x00010001, append(register, 1, 0))
	execute(0x00010004, append([]byte{0, 1}, asset[:]...))
	_, payer, err := lxphost.AssetAccountID(publicKey, asset)
	require.NoError(t, err)
	mint := append([]byte{0, 1}, asset[:]...)
	mint = append(mint, payer[:]...)
	mint = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(mint, 0), 500000000)
	execute(0x0001000a, mint)

	mainName, mainID, err := lxphost.MainAccountID(publicKey)
	require.NoError(t, err)
	field := func(out, value []byte) []byte {
		return append(binary.BigEndian.AppendUint32(out, uint32(len(value))), value...)
	}
	main := field(nil, mainID[:])
	main = field(main, []byte(mainName))
	main = append(main, 1)
	main = binary.BigEndian.AppendUint64(main, 0)
	main = binary.BigEndian.AppendUint64(main, 100000000)
	mainAsset := [32]byte{0x91}
	main = field(main, mainAsset[:])
	main = append(main, 1)
	main = binary.BigEndian.AppendUint64(main, 0)
	main = binary.BigEndian.AppendUint64(main, 1)
	main = append(main, 0, 0)
	main = field(main, publicKey[:])
	store.Set(append([]byte{6}, mainID[:]...), append(main, 1))
	mainAccount, err := host.ReadAccount(ctx, mainName, mainID)
	require.NoError(t, err)
	require.Equal(t, int64(100000000), mainAccount.Balance.Int64())

	treasuryID, err := codec.DeriveAccountID([]byte("system:fees"))
	require.NoError(t, err)
	treasury := field(nil, treasuryID[:])
	treasury = field(treasury, []byte("system:fees"))
	treasury = append(treasury, 10)
	treasury = append(treasury, make([]byte, 16)...)
	treasury = field(treasury, asset[:])
	treasury = append(treasury, 1)
	treasury = append(treasury, make([]byte, 16)...)
	treasury = append(treasury, 0, 0)
	treasury = field(treasury, make([]byte, 32))
	store.Set(append([]byte{6}, treasuryID[:]...), append(treasury, 0))
	treasuryAccount, err := host.ReadAccount(ctx, "system:fees", treasuryID)
	require.NoError(t, err)
	require.False(t, treasuryAccount.Frozen)
	require.True(t, treasuryAccount.Balance.IsZero())
	require.Equal(t, asset, treasuryAccount.AssetID)

	fee := binary.BigEndian.AppendUint32([]byte("LXFR1"), 1)
	for i := 0; i < 7; i++ {
		fee = binary.BigEndian.AppendUint64(fee, 1)
	}
	fee = append(fee, asset[:]...)
	for _, value := range []uint64{1000, 8, 1, 8, 1, 100, 1, 0, 1} {
		fee = binary.BigEndian.AppendUint64(fee, value)
	}
	fee = append(fee, authority.Actor[:]...)
	fee = append(fee, make([]byte, 16)...)
	store.Set(moduleKey([]byte("progfee/active/v1")), fee)
	store.Set(moduleKey(binary.BigEndian.AppendUint32([]byte("progfee/history/v1/"), 1)), fee)

	wasm := []byte{
		0, 97, 115, 109, 1, 0, 0, 0, 1, 12, 2, 96, 2, 127, 127, 1, 127, 96, 1, 127, 1, 127,
		3, 3, 2, 0, 1, 5, 3, 1, 0, 1, 7, 41, 3,
		11, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'c', 'a', 'l', 'l', 0, 0,
		14, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'r', 'e', 's', 'e', 'r', 'v', 'e', 0, 1,
		6, 'm', 'e', 'm', 'o', 'r', 'y', 2, 0, 10, 11, 2, 4, 0, 65, 0, 11, 4, 0, 65, 0, 11,
	}

	program := sha256.Sum256([]byte("sequencer receipt vector program"))
	codeHash := sha256.Sum256(wasm)
	iface := append([]byte("LayerX/program-interface/v1\x00"), codeHash[:]...)
	iface = append(iface, 0, 1, 0, 1, 0, 11)
	iface = append(iface, "layerx_call"...)
	iface = append(iface, 0, 0, 0, 0, 1, 0x20, 0, 0, 0, 64, 1, 0x20, 0, 0, 0, 64, 0, 0, 0, 0, 0, 0)
	rawABI, err := os.ReadFile("../precompiles/layerxprograms/abi.json")
	require.NoError(t, err)
	programs, err := abi.JSON(bytes.NewReader(rawABI))
	require.NoError(t, err)
	budget := [7]uint64{1000000, 16777216, 1048576, 1048576, 64, 1048576, 4096}
	input, err := programs.Pack("call", program, uint16(1), "layerx_call", []byte{}, []byte{0, 0}, []byte("LayerX/programs/access-declaration/v1\x00\x00"), uint32(64), budget, big.NewInt(100000000))
	require.NoError(t, err)
	tx := seqVecTx{nonce: nonce, to: lp.Address, value: new(big.Int), data: input, gasLimit: 20000000, status: uint32(ethtypes.ReceiptStatusSuccessful)}
	ctx = ctx.WithEVMTxHash(b.signTx(tx).Hash())
	precompile, err := lp.NewPrecompile(host, &a.EvmKeeper, a.EvmKeeper.UpgradeKeeper(), app.MergeUpgrade)
	require.NoError(t, err)
	db := state.NewDBImpl(ctx, &a.EvmKeeper, false)
	t.Cleanup(db.Cleanup)
	block, err := a.EvmKeeper.GetVMBlockContext(ctx, core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	evm := vm.NewEVM(*block, db, evmtypes.DefaultChainConfig().EthereumConfig(a.EvmKeeper.ChainID(ctx)), vm.Config{}, map[common.Address]vm.PrecompiledContract{lp.Address: precompile})
	deploy, err := programs.Pack("deploy", program, uint16(1), uint8(1), authority.Principal, codeHash, iface, wasm)
	require.NoError(t, err)
	output, _, err := evm.Call(b.sender, lp.Address, deploy, 2000000, uint256.NewInt(0))
	require.NoError(t, err, "deploy committed runtime fixture: %x; %v", output, db.GetPrecompileError())
	record, err := host.ReadProgram(db.Ctx(), program)
	require.NoError(t, err)
	require.True(t, record.Active)
	require.Equal(t, codeHash, record.CodeHash)
	before := len(db.GetAllLogs())
	output, remaining, err := evm.Call(b.sender, lp.Address, input, tx.gasLimit, uint256.NewInt(0))
	require.NoError(t, err, "call committed runtime fixture: %x; %v", output, db.GetPrecompileError())
	values, err := programs.Methods["call"].Outputs.Unpack(output)
	require.NoError(t, err)
	require.Equal(t, int32(0), values[0])
	require.Equal(t, uint8(1), values[1])
	seqVecAssertProgramTerminal(t, values[2].([]byte))
	logs := db.GetAllLogs()[before:]
	require.Len(t, logs, 1)
	require.Equal(t, []common.Hash{programs.Events["ProgramCallOutcome"].ID, common.Hash(program)}, logs[0].Topics)
	tx.returnData, tx.gasUsed = output, tx.gasLimit-remaining
	tx.logs = []*evmtypes.Log{seqVecLog(logs[0].Address, logs[0].Data, logs[0].Topics...)}
	return tx
}

func seqVecAssertProgramTerminal(t *testing.T, encoded []byte) {
	t.Helper()
	remaining := encoded
	take := func(length uint32) []byte {
		t.Helper()
		require.LessOrEqual(t, uint64(length), uint64(len(remaining)))
		value := remaining[:length]
		remaining = remaining[length:]
		return value
	}
	sized := func() []byte {
		t.Helper()
		return take(binary.BigEndian.Uint32(take(4)))
	}
	domain := []byte("LXP/programs/terminal-applied-legs/v1\x00")
	require.Equal(t, domain, take(uint32(len(domain))))
	detail, legs := sized(), sized()
	require.Empty(t, legs)
	require.Empty(t, remaining)
	remaining = detail
	occupancy := []byte("LXP/program-execution-with-occupancy/v1\x00")
	require.Equal(t, occupancy, take(uint32(len(occupancy))))
	execution, settlement := sized(), sized()
	require.Empty(t, remaining)
	settlementDomain := []byte("LXP/storage-occupancy-settlement/v3\x00")
	require.True(t, bytes.HasPrefix(settlement, settlementDomain))
	require.Equal(t, uint64(seqVecFirstHeight+2), binary.BigEndian.Uint64(settlement[len(settlementDomain):]))
	remaining = execution
	executionDomain := []byte("LXP/program-execution/v2\x00")
	require.Equal(t, executionDomain, take(uint32(len(executionDomain))))
	require.Equal(t, uint16(1), binary.BigEndian.Uint16(take(2)))
	require.Equal(t, uint16(1), binary.BigEndian.Uint16(take(2)))
	require.Equal(t, uint32(1), binary.BigEndian.Uint32(take(4)))
	require.Equal(t, common.BigToHash(big.NewInt(1)).Bytes()[16:], take(16))
	require.Equal(t, []byte{1, 0, 0, 0, 0}, take(5))
	require.Positive(t, binary.BigEndian.Uint64(take(8)))
	require.Equal(t, uint64(65536), binary.BigEndian.Uint64(take(8)))
	require.Zero(t, binary.BigEndian.Uint64(take(8)))
	require.Zero(t, binary.BigEndian.Uint64(take(8)))
	require.Equal(t, uint32(1), binary.BigEndian.Uint32(take(4)))
	require.Positive(t, new(big.Int).SetBytes(take(16)).Sign())
	require.Empty(t, remaining)
}

func seqVecTransfer(nonce uint64, paxUnits int64) seqVecTx {
	return seqVecTx{
		nonce: nonce, to: seqVecRecipient, value: new(big.Int).Mul(big.NewInt(paxUnits), seqVecOnePAX),
		gasLimit: 21_000, gasUsed: 21_000, status: uint32(ethtypes.ReceiptStatusSuccessful),
	}
}

func seqVecInsufficientTransfer(t *testing.T, b *seqVecBuilder) seqVecTx {
	t.Helper()
	tx := seqVecTransfer(6, 101)
	require.Positive(t, b.balance.Sign())
	require.Positive(t, tx.value.Cmp(b.balance))
	a := app.Setup(t, false, false, false)
	ctx, _ := a.GetContextForDeliverTx(nil).CacheContext()
	ctx = ctx.WithBlockHeight(seqVecFirstHeight + 3).WithBlockTime(time.UnixMilli(seqVecTimeBase + (seqVecFirstHeight+3)*seqVecBlockMs))
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	db := state.NewDBImpl(ctx, &a.EvmKeeper, false)
	t.Cleanup(db.Cleanup)
	db.AddBalance(b.sender, uint256.MustFromBig(b.balance), tracing.BalanceChangeUnspecified)
	block, err := a.EvmKeeper.GetVMBlockContext(ctx, core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	evm := vm.NewEVM(*block, db, evmtypes.DefaultChainConfig().EthereumConfig(a.EvmKeeper.ChainID(ctx)), vm.Config{}, nil)
	output, remaining, err := evm.Call(b.sender, tx.to, tx.data, tx.gasLimit, uint256.MustFromBig(tx.value))
	require.ErrorIs(t, err, vm.ErrInsufficientBalance)
	require.Empty(t, output)
	require.Empty(t, db.GetAllLogs())
	require.Equal(t, b.balance, db.GetBalance(b.sender).ToBig())
	require.True(t, db.GetBalance(tx.to).IsZero())
	tx.status = uint32(ethtypes.ReceiptStatusFailed)
	tx.gasUsed, tx.returnData, tx.vmError = tx.gasLimit-remaining, output, err.Error()
	return tx
}

// seqVecServe serves signed through pax_sequencerReceipt and returns the JSON answer.
func seqVecServe(t *testing.T, port int, source *seqVecSource, signed seqVecSigned) json.RawMessage {
	t.Helper()
	source.put(signed.activity, signed.send)
	raw, rpcErr := callSeqReceipt9005(t, port, evmrpc.SequencerReceiptMethod, signed.activity)
	require.Nil(t, rpcErr)
	var result *evmrpc.SequencerReceiptResult
	require.NoError(t, json.Unmarshal(raw, &result))
	require.NotNil(t, result)
	require.Equal(t, evmrpc.RungReceipted, result.Rung)
	require.NoError(t, result.Verify())
	require.Equal(t, signed.signed, []byte(result.Receipt))
	return raw
}

// seqVecEdit re-renders a JSON answer after edit changes it, as a tampering transport would.
func seqVecEdit(t *testing.T, raw json.RawMessage, edit func(*evmrpc.SequencerReceiptResult)) json.RawMessage {
	t.Helper()
	var result *evmrpc.SequencerReceiptResult
	require.NoError(t, json.Unmarshal(raw, &result))
	edit(result)
	out, err := json.Marshal(result)
	require.NoError(t, err)
	return out
}

// seqVecClientCheck runs client checks R1-R6 of clients.md (c) and returns the first check that
// refuses with its error, or "" when the answer holds the receipted rung.
func seqVecClientCheck(raw []byte, answer json.RawMessage, prev common.Hash, pin seqVecPin) (string, error) {
	var result *evmrpc.SequencerReceiptResult
	if err := json.Unmarshal(answer, &result); err != nil {
		return "R1", err
	}
	if result == nil || (result.Rung != evmrpc.RungReceipted && result.Rung != evmrpc.RungCommitted) ||
		result.SequencerReceiptView == nil || len(result.Receipt) == 0 {
		return "R1", errSeqVecRung
	}
	decoded, err := codec.DecodeReceipt(result.Receipt)
	if err != nil {
		return "R2", err
	}
	if common.Hash(decoded.ContextHash) != pin.KeyID {
		return "R3", errSeqVecKeyID
	}
	height := binary.BigEndian.Uint64(decoded.BatchID[24:])
	if !bytes.Equal(decoded.BatchID[:24], make([]byte, 24)) || height < pin.FirstHeight || height > pin.LastHeight {
		return "R3", errSeqVecHeight
	}
	if _, err := verify.ReceiptSignature(result.Receipt, [32]byte(pin.PublicKey)); err != nil {
		return "R2", err
	}
	activity := crypto.Keccak256Hash(raw)
	if common.Hash(decoded.ActivityID) != activity || result.TxHash != activity {
		return "R4", errSeqVecActivity
	}
	if err := result.Verify(); err != nil {
		return "R5", err
	}
	if common.Hash(decoded.PreviousStateRoot) != prev ||
		decoded.ResultingStateRoot != seqReceiptNextHash9005(prev, decoded.GlobalSequence, decoded.ActivityID) {
		return "R6", errSeqVecSeqHash
	}
	return "", nil
}

type seqVecRefusal struct {
	refusedAt string
	err       error
	verifier  string
}

func seqVecCaseOf(id, vector, description string, signed seqVecSigned, raw []byte, answer json.RawMessage, prev [32]byte, pin seqVecPin) seqVecCase {
	return seqVecCase{
		ID:             id,
		Vector:         vector,
		Description:    description,
		RawTransaction: raw,
		ReceiptJSON:    answer,
		SignedReceipt:  signed.signed,
		ReceiptDigest:  signed.digest,
		PrevSeqHash:    prev,
		PinnedKey:      pin,
	}
}

func installSeqVecReceiptKey(t *testing.T, chain *seqReceiptChain9005, pin seqVecPin) {
	t.Helper()
	height := chain.latest + 1
	ctx := chain.writeCtx9005(height)
	existing, err := chain.keeper.ReceiptKeys(ctx)
	require.NoError(t, err)
	keys := make(seqkeeper.ReceiptKeys, 0, len(existing)+1)
	for _, key := range existing {
		keys = append(keys, seqkeeper.ReceiptKeyEntry{
			KeyId: bytes.Clone(key.KeyId[:]), PublicKey: bytes.Clone(key.PublicKey[:]),
			FirstHeight: key.FirstHeight, LastHeight: key.LastHeight,
		})
	}
	keys = append(keys, seqkeeper.ReceiptKeyEntry{
		KeyId: bytes.Clone(pin.KeyID[:]), PublicKey: bytes.Clone(pin.PublicKey),
		FirstHeight: pin.FirstHeight, LastHeight: pin.LastHeight,
	})
	require.NoError(t, chain.keeper.SetReceiptKeys(ctx, keys))
	chain.commit9005(height)
}

func TestSequencerReceiptVectors(t *testing.T) {
	dir := t.TempDir()
	public := [32]byte(ed25519.NewKeyFromSeed(seqVecReceiptSeed).Public().(ed25519.PublicKey))
	pin := seqVecPin{KeyID: seqVecKeyID, PublicKey: public[:], FirstHeight: seqVecFirstHeight, LastHeight: seqVecLastHeight}
	primary := loadSeqVecKey(t, dir, verify.SequencerAuthorization{SequencerID: seqVecKeyID, PublicKey: public, FirstBatchNumber: seqVecFirstHeight, LastBatchNumber: seqVecLastHeight})
	rogue := loadSeqVecKey(t, dir, verify.SequencerAuthorization{SequencerID: seqVecRogueKeyID, PublicKey: public, FirstBatchNumber: seqVecFirstHeight, LastBatchNumber: seqVecLastHeight})
	extended := loadSeqVecKey(t, dir, verify.SequencerAuthorization{SequencerID: seqVecKeyID, PublicKey: public, FirstBatchNumber: seqVecLastHeight + 1, LastBatchNumber: 2 * seqVecLastHeight})

	chain := newSeqReceiptChain9005(t)
	installSeqVecReceiptKey(t, chain, pin)
	source := &seqVecSource{sends: map[common.Hash]*evmrpc.SequencerSend{}}
	port := startSeqReceiptServer9005(t, app.Setup(t, false, false, false), chain.ctxProvider,
		evmrpc.SequencerReceiptOption{Source: source, Keeper: &chain.keeper})

	b := newSeqVecBuilder(t)
	var cases []seqVecCase
	expected := map[string]seqVecRefusal{}
	prev := [32]byte{}
	accept := func(id, vector, description string, tx seqVecTx, height int64, index int) (seqVecSigned, json.RawMessage, [32]byte) {
		signedTx := b.signTx(tx)
		slot := seqVecSlot{seq: uint64(len(cases) + 1), prev: prev, height: height, index: index}
		signed := b.receipt(primary, seqVecKeyID, signedTx, tx, slot, true)
		answer := seqVecServe(t, port, source, signed)
		c := seqVecCaseOf(id, vector, description, signed, signed.raw, answer, prev, pin)
		c.Expected = seqVecExpected{Rung: string(evmrpc.RungReceipted), Verifier: "accepts"}
		cases = append(cases, c)
		before := prev
		prev = signed.seqHash
		return signed, answer, before
	}

	transfer, transferJSON, transferPrev := accept("V1-pax-transfer", "V1", "PAX transfer to an externally owned account", seqVecTransfer(0, 1), seqVecFirstHeight, 1)
	approve, approveJSON, approvePrev := accept("V1-erc20-approve", "V1", "ERC-20 approve with one Approval log", seqVecApprove(t, b.sender, 1), seqVecFirstHeight, 2)
	accept("V1-layerxledger-pay-exact", "V1", "layerxledger payExact at 0x101A with one PaymentExact log", seqVecLedgerPayment(t, 2), seqVecFirstHeight+1, 1)
	accept("V1-layerxbridge-bridge-out", "V1", "layerxbridge bridgeOut at 0x1016 with one BridgeOut log", seqVecBridgeOut(t, 3), seqVecFirstHeight+1, 2)
	accept("V1-layerxprograms-call", "V1", "layerxprograms call at 0x101D with one ProgramCallOutcome log", seqVecProgramsCall(t, b, 4), seqVecFirstHeight+2, 1)
	accept("V2-layerxledger-revert", "V2", "layerxledger payExact reverted with Error(string) LXP_INSUFFICIENT_BALANCE", seqVecLedgerRevert(t, 5), seqVecFirstHeight+2, 2)
	cases[len(cases)-1].LXPCode = seqVecInsufficient

	refuse := func(id, description string, c seqVecCase, refusal seqVecRefusal) {
		c.ID, c.Vector, c.Description = id, "V3", description
		c.Expected = seqVecExpected{RefusedAt: refusal.refusedAt, Check: refusal.err.Error(), Verifier: refusal.verifier}
		expected[id] = refusal
		cases = append(cases, c)
	}
	next := seqVecSlot{seq: uint64(len(cases) + 1), prev: prev, height: seqVecFirstHeight + 3, index: 1}
	nextCase := func(key *seqnode.ReceiptKey, keyID [32]byte, tx seqVecTx, slot seqVecSlot) seqVecCase {
		signedTx := b.signTx(tx)
		signed := b.receipt(key, keyID, signedTx, tx, slot, false)
		return seqVecCaseOf("", "", "", signed, signed.raw, seqVecServe(t, port, source, signed), next.prev, pin)
	}
	refusedCase := func(key *seqnode.ReceiptKey, keyID [32]byte, tx seqVecTx, slot seqVecSlot, reason string) seqVecCase {
		signedTx := b.signTx(tx)
		signed := b.receipt(key, keyID, signedTx, tx, slot, false)
		source.put(signed.activity, signed.send)
		_, rpcErr := callSeqReceipt9005(t, port, evmrpc.SequencerReceiptMethod, signed.activity)
		require.NotNil(t, rpcErr)
		require.Equal(t, -32000, rpcErr.Code)
		require.Contains(t, rpcErr.Message, seqkeeper.ErrReceiptKeyNotPinned.Error())
		require.Contains(t, rpcErr.Message, reason)

		authorized := b.receipt(primary, seqVecKeyID, signedTx, tx, next, false)
		answer := seqVecEdit(t, seqVecServe(t, port, source, authorized), func(r *evmrpc.SequencerReceiptResult) {
			r.KeyID = keyID
			r.Height = hexutil.Uint64(slot.height)
			r.Timestamp = hexutil.Uint64(seqVecTimeBase + slot.height*seqVecBlockMs)
			r.Receipt = signed.signed
		})
		var result evmrpc.SequencerReceiptResult
		require.NoError(t, json.Unmarshal(answer, &result))
		require.NoError(t, result.Verify())
		return seqVecCaseOf("", "", "", signed, signed.raw, answer, next.prev, pin)
	}

	refuse("V3-key-not-pinned", "receipt signed under a keyId outside the pinned ReceiptKeys",
		refusedCase(rogue, seqVecRogueKeyID, seqVecTransfer(6, 2), next, fmt.Sprintf("is not in the list at height %d", next.height)),
		seqVecRefusal{refusedAt: "R3", err: errSeqVecKeyID, verifier: "SequencerSignature"})

	outOfRange := next
	outOfRange.height = seqVecLastHeight + 1
	refuse("V3-height-outside-key-range", "receipt at a height above the pinned key's lastHeight",
		refusedCase(extended, seqVecKeyID, seqVecTransfer(6, 3), outOfRange, fmt.Sprintf("excludes height %d", outOfRange.height)),
		seqVecRefusal{refusedAt: "R3", err: errSeqVecHeight, verifier: "SequencerSignature"})

	sentTx := b.signTx(seqVecTransfer(6, 4))
	sentRaw, err := sentTx.MarshalBinary()
	require.NoError(t, err)
	substituted := nextCase(primary, seqVecKeyID, seqVecTransfer(6, 5), next)
	require.NotEqual(t, crypto.Keccak256(sentRaw), crypto.Keccak256(substituted.RawTransaction))
	substituted.RawTransaction = sentRaw
	refuse("V3-activity-id-not-raw-hash", "receipt whose activity_id is not keccak256 of the raw transaction the client signed",
		substituted, seqVecRefusal{refusedAt: "R4", err: errSeqVecActivity, verifier: "accepts"})

	forked := next
	forked.prev = approvePrev
	refuse("V3-seq-hash-mismatch", "receipt chained from a prevSeqHash other than the client's predecessor",
		nextCase(primary, seqVecKeyID, seqVecTransfer(6, 6), forked),
		seqVecRefusal{refusedAt: "R6", err: errSeqVecSeqHash, verifier: "accepts"})

	tamperedLog := seqVecCaseOf("", "", "", approve, approve.raw, seqVecEdit(t, approveJSON, func(r *evmrpc.SequencerReceiptResult) {
		r.Logs[0].Data = append(bytes.Clone(r.Logs[0].Data[:len(r.Logs[0].Data)-1]), r.Logs[0].Data[len(r.Logs[0].Data)-1]^0x01)
	}), approvePrev, pin)
	refuse("V3-tampered-log", "Approval log data altered in the JSON answer while the signed receipt is intact",
		tamperedLog, seqVecRefusal{refusedAt: "R5", err: evmrpc.ErrSequencerReceiptMismatch, verifier: "accepts"})

	nonCanonical := bytes.Clone(transfer.signed)
	flagAt := len(nonCanonical) - 1 - 4 - ed25519.SignatureSize
	require.Equal(t, byte(1), nonCanonical[flagAt])
	nonCanonical[flagAt] = 2
	mutated := seqVecCaseOf("", "", "", transfer, transfer.raw, seqVecEdit(t, transferJSON, func(r *evmrpc.SequencerReceiptResult) {
		r.Receipt = nonCanonical
	}), transferPrev, pin)
	mutated.SignedReceipt = nonCanonical
	refuse("V3-non-canonical-bytes", "signature presence flag 2, which no canonical receipt encodes",
		mutated, seqVecRefusal{refusedAt: "R2", err: codec.ErrNonCanonical, verifier: "Decode"})

	// Keep the existing recordings and refusal-chain identities intact.
	tx := seqVecInsufficientTransfer(t, b)
	signedTx := b.signTx(tx)
	signed := b.receipt(primary, seqVecKeyID, signedTx, tx, next, false)
	refusal := seqVecCaseOf("V2-pax-transfer-insufficient-funds", "V2", "PAX transfer refused before execution because its value exceeds the funded sender balance", signed, signed.raw, seqVecServe(t, port, source, signed), next.prev, pin)
	refusal.Expected = seqVecExpected{Rung: string(evmrpc.RungReceipted), Verifier: "accepts"}
	cases = append(cases[:6], append([]seqVecCase{refusal}, cases[6:]...)...)

	seqVecAssertCases(t, cases, expected)

	file := seqVecFile{
		Name:       seqVecName,
		Provenance: seqVecProvenance{Generator: seqVecGenerator, Command: seqVecCommand},
		ChainID:    seqVecChainID,
		ModuleID:   seqVecModuleID,
		FeeDenom:   evmkeeper.BaseDenom,
		Cases:      cases,
	}
	seqVecWriteFixture(t, seqVecFixture, seqVecCommand, file)
	t.Run("conformance-copies", TestVectorFilesV1ToV3MatchSequencerReceiptFixture)
}

func seqVecWriteFixture(t *testing.T, path, command string, file seqVecFile) {
	t.Helper()
	rendered, err := json.MarshalIndent(file, "", "  ")
	require.NoError(t, err)
	rendered = append(rendered, '\n')
	if *updateSequencerReceiptVectors {
		require.NoError(t, os.WriteFile(path, rendered, 0o644)) //nolint:gosec // a published conformance fixture
	}
	stored, err := os.ReadFile(path)
	require.NoError(t, err, "run %s to write the fixture", command)
	require.Equal(t, string(stored), string(rendered), "the fixture differs from the node signer output; run %s", command)
}

func seqVecUCPGatewayKey(t *testing.T) common.Hash {
	t.Helper()
	key, err := hex.DecodeString(strings.ReplaceAll(seqVecUCPIdempotency, "-", ""))
	require.NoError(t, err)
	require.Len(t, key, 16)
	return common.Hash(sha256.Sum256(append([]byte(seqVecUCPDomain), key...)))
}

func TestRepairM_466_UCPCheckoutReceiptVector(t *testing.T) {
	public := [32]byte(ed25519.NewKeyFromSeed(seqVecReceiptSeed).Public().(ed25519.PublicKey))
	pin := seqVecPin{KeyID: seqVecKeyID, PublicKey: public[:], FirstHeight: seqVecFirstHeight, LastHeight: seqVecLastHeight}
	key := loadSeqVecKey(t, t.TempDir(), verify.SequencerAuthorization{
		SequencerID: seqVecKeyID, PublicKey: public, FirstBatchNumber: seqVecFirstHeight, LastBatchNumber: seqVecLastHeight,
	})
	chain := newSeqReceiptChain9005(t)
	installSeqVecReceiptKey(t, chain, pin)
	source := &seqVecSource{sends: map[common.Hash]*evmrpc.SequencerSend{}}
	port := startSeqReceiptServer9005(t, app.Setup(t, false, false, false), chain.ctxProvider,
		evmrpc.SequencerReceiptOption{Source: source, Keeper: &chain.keeper})
	b := newSeqVecBuilder(t)
	pay := seqVecLedgerPay(t)
	pay.reference = seqVecUCPGatewayKey(t)
	tx := seqVecLedgerPaymentFor(t, 0, pay)
	signed := b.receipt(key, seqVecKeyID, b.signTx(tx), tx, seqVecSlot{seq: 1, height: seqVecFirstHeight, index: 1}, true)
	answer := seqVecServe(t, port, source, signed)
	c := seqVecCaseOf("V1-ucp-checkout-pay-exact", "V1",
		"layerxledger payExact with the UCP gateway key of idempotency key "+seqVecUCPIdempotency,
		signed, signed.raw, answer, [32]byte{}, pin)
	c.Expected = seqVecExpected{Rung: string(evmrpc.RungReceipted), Verifier: "accepts"}
	seqVecAssertUCPCheckout(t, c)
	file := seqVecFile{
		Name:       "ucp-checkout-receipt-v1",
		Provenance: seqVecProvenance{Generator: seqVecGenerator, Command: seqVecUCPCommand},
		ChainID:    seqVecChainID, ModuleID: seqVecModuleID, FeeDenom: evmkeeper.BaseDenom,
		Cases: []seqVecCase{c},
	}
	seqVecWriteFixture(t, seqVecUCPFixture, seqVecUCPCommand, file)
}

func seqVecReadUCPCheckout(t *testing.T) seqVecCase {
	t.Helper()
	stored, err := os.ReadFile(seqVecUCPFixture)
	require.NoError(t, err)
	var file seqVecFile
	require.NoError(t, json.Unmarshal(stored, &file))
	require.Equal(t, "ucp-checkout-receipt-v1", file.Name)
	require.Equal(t, seqVecProvenance{Generator: seqVecGenerator, Command: seqVecUCPCommand}, file.Provenance)
	require.Equal(t, uint64(seqVecChainID), file.ChainID)
	require.Equal(t, uint16(seqVecModuleID), file.ModuleID)
	require.Equal(t, evmkeeper.BaseDenom, file.FeeDenom)
	require.Len(t, file.Cases, 1)
	return file.Cases[0]
}

func seqVecAssertUCPCheckout(t *testing.T, c seqVecCase) {
	t.Helper()
	require.Equal(t, "V1-ucp-checkout-pay-exact", c.ID)
	require.Equal(t, "V1", c.Vector)
	require.Equal(t, seqVecExpected{Rung: string(evmrpc.RungReceipted), Verifier: "accepts"}, c.Expected)
	public := ed25519.NewKeyFromSeed(seqVecReceiptSeed).Public().(ed25519.PublicKey)
	require.Equal(t, seqVecPin{KeyID: seqVecKeyID, PublicKey: []byte(public), FirstHeight: seqVecFirstHeight, LastHeight: seqVecLastHeight}, c.PinnedKey)
	check, err := seqVecClientCheck(c.RawTransaction, c.ReceiptJSON, c.PrevSeqHash, c.PinnedKey)
	require.NoError(t, err, check)
	require.Empty(t, check)
	verified, err := verify.ReceiptSignature(c.SignedReceipt, [32]byte(public))
	require.NoError(t, err)
	require.Equal(t, c.ReceiptDigest, common.Hash(verified.Digest))
	require.Equal(t, seqVecKeyID, verified.Receipt.ContextHash)
	require.Zero(t, verified.Receipt.ResultCode)
	var result evmrpc.SequencerReceiptResult
	require.NoError(t, json.Unmarshal(c.ReceiptJSON, &result))
	require.Equal(t, c.SignedReceipt, result.Receipt)
	require.Equal(t, uint64(ethtypes.ReceiptStatusSuccessful), uint64(result.Status))
	require.Len(t, result.Logs, 1)
	pay := seqVecLedgerPay(t)
	pay.reference = seqVecUCPGatewayKey(t)
	log := result.Logs[0]
	require.Equal(t, seqVecLedger, log.Address)
	require.Equal(t, []common.Hash{
		crypto.Keccak256Hash([]byte("PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)")),
		pay.reference, pay.from, pay.to,
	}, log.Topics)
	data, err := seqVecArgs(t, "bytes32", "uint128").Pack([32]byte(pay.asset), pay.amount)
	require.NoError(t, err)
	require.Equal(t, data, []byte(log.Data))
	tx := new(ethtypes.Transaction)
	require.NoError(t, tx.UnmarshalBinary(c.RawTransaction))
	require.Equal(t, big.NewInt(seqVecChainID), tx.ChainId())
	require.Equal(t, seqVecLedger, *tx.To())
	require.Equal(t, seqVecPayExactCall(t, pay), tx.Data())
}

func TestRepairM_466_UCPCheckoutGatewayKey(t *testing.T) {
	key := []byte{0x12, 0x3e, 0x45, 0x67, 0xe8, 0x9b, 0x12, 0xd3, 0xa4, 0x56, 0x42, 0x66, 0x14, 0x17, 0x40, 0x00}
	material := append([]byte("LayerX/interop/ucp/idempotency/v1"), 0)
	expected := common.Hash(sha256.Sum256(append(material, key...)))
	require.Equal(t, expected, seqVecUCPGatewayKey(t))
	c := seqVecReadUCPCheckout(t)
	seqVecAssertUCPCheckout(t, c)
	var result evmrpc.SequencerReceiptResult
	require.NoError(t, json.Unmarshal(c.ReceiptJSON, &result))
	require.Equal(t, expected, result.Logs[0].Topics[1])
	require.NotEqual(t, seqVecLedgerPay(t).reference, expected)
}

func TestRepairM_466_UCPCheckoutPinnedReceipt(t *testing.T) {
	seqVecAssertUCPCheckout(t, seqVecReadUCPCheckout(t))
}

type seqVecFiatJourney struct {
	Name              string          `json:"name"`
	Token             string          `json:"token"`
	ProviderFacts     json.RawMessage `json:"provider_facts"`
	ProviderPublicKey hexutil.Bytes   `json:"provider_public_key"`
	ProviderSignature hexutil.Bytes   `json:"provider_signature"`
	Sequencer         struct {
		Fixture        string        `json:"fixture"`
		Case           string        `json:"case"`
		Rung           string        `json:"rung"`
		Transaction    common.Hash   `json:"transaction_hash"`
		Digest         *common.Hash  `json:"receipt_digest"`
		IdempotencyKey common.Hash   `json:"idempotency_key"`
		Reference      common.Hash   `json:"payment_reference"`
		Calldata       hexutil.Bytes `json:"payment_calldata"`
		Counterparty   common.Hash   `json:"counterparty"`
		ExpiresAt      uint64        `json:"expires_at"`
	} `json:"sequencer"`
	Expect struct {
		State       string       `json:"state"`
		Refusal     string       `json:"refusal"`
		Digest      *common.Hash `json:"receipt_digest"`
		Translation string       `json:"translation"`
	} `json:"expect"`
}

func seqVecFiatPayment(t *testing.T, journey seqVecFiatJourney) seqVecPayExact {
	t.Helper()
	var canonical map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(journey.ProviderFacts, &canonical))
	payload, err := json.Marshal(canonical)
	require.NoError(t, err)
	require.Len(t, journey.ProviderPublicKey, ed25519.PublicKeySize)
	require.Len(t, journey.ProviderSignature, ed25519.SignatureSize)
	require.True(t, ed25519.Verify(ed25519.PublicKey(journey.ProviderPublicKey),
		append([]byte("LayerX/fiat/sequenced-provider/v1\x00"), payload...), journey.ProviderSignature))
	var facts struct {
		Provider    string      `json:"provider"`
		Settlement  string      `json:"settlement"`
		Rail        string      `json:"rail"`
		Class       string      `json:"class"`
		Amount      string      `json:"amount"`
		Asset       common.Hash `json:"asset"`
		Destination common.Hash `json:"destination"`
		TokenDigest common.Hash `json:"token_reference_sha256"`
	}
	require.NoError(t, json.Unmarshal(journey.ProviderFacts, &facts))
	require.NotEmpty(t, facts.Provider)
	require.NotEmpty(t, facts.Settlement)
	require.Equal(t, "settled", facts.Class)
	require.Equal(t, common.Hash(sha256.Sum256([]byte(journey.Token))), facts.TokenDigest)
	rail, ok := map[string]byte{"card": 1, "bank": 2, "rtp": 3}[facts.Rail]
	require.True(t, ok, facts.Rail)
	material := append([]byte("LayerX/interop/fiat/idempotency/v1\x00"), []byte(facts.Provider)...)
	material = append(material, 0)
	material = append(material, []byte(facts.Settlement)...)
	material = append(material, rail, 3)
	amount, ok := new(big.Int).SetString(facts.Amount, 10)
	require.True(t, ok)
	require.Positive(t, amount.Sign())
	require.LessOrEqual(t, amount.BitLen(), 128)
	pay := seqVecLedgerPay(t)
	pay.to, pay.asset, pay.amount = facts.Destination, facts.Asset, amount
	pay.reference = sha256.Sum256(material)
	require.Equal(t, pay.from, journey.Sequencer.Counterparty)
	require.Equal(t, pay.expiresAt, journey.Sequencer.ExpiresAt)
	return pay
}

func TestRepairM_489_FiatJourneysDerived(t *testing.T) {
	source, err := os.ReadFile(seqVecFiatSource)
	require.NoError(t, err)
	require.Equal(t, seqVecFiatSourceHash, fmt.Sprintf("%x", sha256.Sum256(source)))
	var committed struct {
		Settlement struct {
			RawTransaction hexutil.Bytes   `json:"raw_transaction"`
			Receipt        json.RawMessage `json:"receipt"`
		} `json:"settlement"`
	}
	require.NoError(t, json.Unmarshal(source, &committed))
	var committedReceipt evmrpc.SequencerReceiptResult
	require.NoError(t, json.Unmarshal(committed.Settlement.Receipt, &committedReceipt))
	require.Equal(t, evmrpc.RungCommitted, committedReceipt.Rung)
	require.NoError(t, committedReceipt.Verify())
	committedTx := new(ethtypes.Transaction)
	require.NoError(t, committedTx.UnmarshalBinary(committed.Settlement.RawTransaction))
	require.Equal(t, uint8(ethtypes.DynamicFeeTxType), committedTx.Type())
	require.Equal(t, big.NewInt(seqVecChainID), committedTx.ChainId())
	require.Equal(t, committedTx.Hash(), committedReceipt.TxHash)
	decoded, err := codec.DecodeReceipt(committedReceipt.Receipt)
	require.NoError(t, err)
	require.Equal(t, uint16(2), decoded.ProtocolVersion)

	stored, err := os.ReadFile(seqVecFiatJourneys)
	require.NoError(t, err)
	var journeys []seqVecFiatJourney
	require.NoError(t, json.Unmarshal(stored, &journeys))
	require.Len(t, journeys, 5)
	public := [32]byte(ed25519.NewKeyFromSeed(seqVecReceiptSeed).Public().(ed25519.PublicKey))
	pin := seqVecPin{KeyID: seqVecKeyID, PublicKey: public[:], FirstHeight: seqVecFirstHeight, LastHeight: seqVecLastHeight}
	key := loadSeqVecKey(t, t.TempDir(), verify.SequencerAuthorization{
		SequencerID: seqVecKeyID, PublicKey: public, FirstBatchNumber: seqVecFirstHeight, LastBatchNumber: seqVecLastHeight,
	})
	chain := newSeqReceiptChain9005(t)
	installSeqVecReceiptKey(t, chain, pin)
	sourceRPC := seqnode.NewReceiptSource(len(journeys))
	port := startSeqReceiptServer9005(t, app.Setup(t, false, false, false), chain.ctxProvider,
		evmrpc.SequencerReceiptOption{Source: sourceRPC, Keeper: &chain.keeper})
	b := newSeqVecBuilder(t)
	file := seqVecFile{
		Name: "fiat-receipts-v1", ChainID: seqVecChainID, ModuleID: seqVecModuleID, FeeDenom: evmkeeper.BaseDenom,
		Provenance: seqVecProvenance{Generator: seqVecGenerator, Command: seqVecFiatCommand,
			Source: strings.TrimPrefix(seqVecFiatSource, "../"), SHA256: seqVecFiatSourceHash},
	}
	prev := [32]byte{}
	coverage := map[string]int{}
	seen := map[common.Hash]bool{}
	for i, journey := range journeys {
		t.Run(journey.Name, func(t *testing.T) {
			pay := seqVecFiatPayment(t, journey)
			require.False(t, seen[pay.reference], "provider settlement keys must be distinct")
			seen[pay.reference] = true
			require.Equal(t, pay.reference, journey.Sequencer.IdempotencyKey)
			require.Equal(t, pay.reference, journey.Sequencer.Reference)
			calldata := make([]byte, 196)
			copy(calldata[:4], []byte{0x78, 0xea, 0x11, 0xdb})
			copy(calldata[4:36], pay.from[:])
			copy(calldata[36:68], pay.to[:])
			copy(calldata[68:100], pay.asset[:])
			pay.amount.FillBytes(calldata[116:132])
			binary.BigEndian.PutUint64(calldata[156:164], pay.expiresAt)
			copy(calldata[164:196], pay.reference[:])
			require.Equal(t, calldata, []byte(journey.Sequencer.Calldata))
			require.Equal(t, calldata, seqVecPayExactCall(t, pay))
			tx := seqVecLedgerPaymentFor(t, uint64(i), pay)
			vector := "V1"
			if journey.Expect.Refusal != "" {
				require.Equal(t, "ReceiptMismatch", journey.Expect.Refusal)
				tx = seqVecLedgerRevert(t, uint64(i))
				tx.data = calldata
				vector = "V2"
			}
			signedTx := b.signTx(tx)
			signed := b.receipt(key, seqVecKeyID, signedTx, tx,
				seqVecSlot{seq: uint64(i + 1), prev: prev, height: seqVecFirstHeight + int64(i), index: 1}, true)
			sourceRPC.Record(signed.activity, *signed.send)
			answer, rpcErr := callSeqReceipt9005(t, port, evmrpc.SequencerReceiptMethod, signed.activity)
			require.Nil(t, rpcErr)
			c := seqVecCaseOf(journey.Name, vector, "Fiat adapter payment for "+journey.Name,
				signed, signed.raw, answer, prev, pin)
			c.Expected = seqVecExpected{Rung: "receipted", Verifier: "accepts"}
			if vector == "V2" {
				c.LXPCode = seqVecInsufficient
			}
			file.Cases = append(file.Cases, c)
			check, err := seqVecClientCheck(c.RawTransaction, c.ReceiptJSON, c.PrevSeqHash, c.PinnedKey)
			require.NoError(t, err, check)
			require.Empty(t, check)
			verified, err := verify.ReceiptSignature(c.SignedReceipt, public)
			require.NoError(t, err)
			require.Equal(t, uint16(2), verified.Receipt.ProtocolVersion)
			require.Equal(t, uint8(ethtypes.DynamicFeeTxType), verified.Receipt.Operation)
			require.Equal(t, c.ReceiptDigest, common.Hash(verified.Digest))
			require.Equal(t, [32]byte(signedTx.Hash()), verified.Receipt.ActivityID)
			require.Equal(t, calldata, signedTx.Data())
			require.Equal(t, big.NewInt(seqVecChainID), signedTx.ChainId())
			require.Equal(t, "sdk/platform/conformance/fixtures/fiat-receipts-v1.json", journey.Sequencer.Fixture)
			require.Equal(t, c.ID, journey.Sequencer.Case)
			require.Equal(t, signedTx.Hash(), journey.Sequencer.Transaction)
			var result evmrpc.SequencerReceiptResult
			require.NoError(t, json.Unmarshal(answer, &result))
			if journey.Sequencer.Rung == "submitted" {
				require.Equal(t, "credit-pending", journey.Expect.State)
				require.Equal(t, "pending", journey.Expect.Translation)
				require.Nil(t, journey.Sequencer.Digest)
				require.Nil(t, journey.Expect.Digest)
				coverage["submitted"]++
			} else {
				require.Equal(t, "receipted", journey.Sequencer.Rung)
				require.NotNil(t, journey.Sequencer.Digest)
				require.Equal(t, c.ReceiptDigest, *journey.Sequencer.Digest)
				if vector == "V2" {
					require.EqualValues(t, 0, result.Status)
					require.EqualValues(t, 1, verified.Receipt.ResultCode)
					require.Empty(t, result.Logs)
					require.Equal(t, pcommon.LXPRevert(seqVecInsufficient, ""), []byte(result.ReturnData))
					require.Equal(t, "pending", journey.Expect.Translation)
					coverage["reverted"]++
				} else {
					require.Equal(t, "credited", journey.Expect.State)
					require.Equal(t, "receipt-verified", journey.Expect.Translation)
					require.NotNil(t, journey.Expect.Digest)
					require.Equal(t, c.ReceiptDigest, *journey.Expect.Digest)
					require.EqualValues(t, 1, result.Status)
					require.Zero(t, verified.Receipt.ResultCode)
					require.Len(t, result.Logs, 1)
					require.Equal(t, []common.Hash{crypto.Keccak256Hash([]byte("PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)")),
						pay.reference, pay.from, pay.to}, result.Logs[0].Topics)
					require.Equal(t, seqVecLedger, result.Logs[0].Address)
					data, err := seqVecArgs(t, "bytes32", "uint128").Pack([32]byte(pay.asset), pay.amount)
					require.NoError(t, err)
					require.Equal(t, data, []byte(result.Logs[0].Data))
					coverage["credited"]++
				}
			}
			prev = signed.seqHash
		})
	}
	require.Equal(t, map[string]int{"credited": 3, "submitted": 1, "reverted": 1}, coverage)
	seqVecWriteFixture(t, seqVecFiatFixture, seqVecFiatCommand, file)
}

func repairM1297FeeTokenVector(t *testing.T) seqVecFile {
	t.Helper()
	private := &secp256k1.PrivKey{Key: bytes.Clone(seqVecSenderSecret)}
	ethKey, err := crypto.ToECDSA(seqVecSenderSecret)
	require.NoError(t, err)
	sender := occutils.TestAcct{AccountAddress: sdk.AccAddress(private.PubKey().Address()), EvmAddress: crypto.PubkeyToAddress(ethKey.PublicKey), EvmPrivateKey: ethKey}
	s := newPendingSequencer(t, 1, sender)
	// Configure the real working store before reopening the empty speculative
	// block, so the receipter and executor see the same fee policy.
	empty, err := s.exec.Seal(2)
	require.NoError(t, err)
	require.Empty(t, empty.Txs)
	setupStore := s.app.CommitMultiStore().CacheMultiStore()
	setup := sdk.NewContext(setupStore, tmproto.Header{ChainID: pendingChainID, Height: 2}, false)
	params := s.app.EvmKeeper.GetParams(setup)
	params.FeeTokenEnabled = true
	rate := sdk.NewDec(evmtypes.InitialSidioraBaseUnitsPerPax)
	params.AllowedFeeDenoms = []evmtypes.AllowedFeeDenom{{Denom: "usid", Rate: rate, RateUpdateHeight: 1}}
	s.app.EvmKeeper.SetParams(setup, params)
	require.NoError(t, s.app.EvmKeeper.SetAccountFeeDenom(setup, sender.EvmAddress, "usid"))
	funding := sdk.NewCoins(sdk.NewInt64Coin("usid", 1_000_000_000))
	require.NoError(t, s.app.BankKeeper.MintCoins(setup, "mint", funding))
	require.NoError(t, s.app.BankKeeper.SendCoinsFromModuleToAccount(setup, "mint", sender.AccountAddress, funding))

	public := [32]byte(ed25519.NewKeyFromSeed(seqVecReceiptSeed).Public().(ed25519.PublicKey))
	pin := seqVecPin{KeyID: seqVecKeyID, PublicKey: public[:], FirstHeight: 1, LastHeight: 1000}
	key := loadSeqVecKey(t, t.TempDir(), verify.SequencerAuthorization{SequencerID: seqVecKeyID, PublicKey: public, FirstBatchNumber: 1, LastBatchNumber: 1000})
	pins := seqkeeper.ReceiptKeys{{KeyId: seqVecKeyID[:], PublicKey: public[:], FirstHeight: 1, LastHeight: 1000}}
	require.NoError(t, s.app.SequencerKeeper.SetReceiptKeys(setup, pins))
	setupStore.Write()
	require.EqualValues(t, 2, s.app.CommitMultiStore().Commit(true).Version)
	s.committed = 2
	require.True(t, s.app.EvmKeeper.GetFeeTokenEnabled(s.ctxProvider(2)))
	require.Equal(t, "usid", s.app.EvmKeeper.GetAccountFeeDenom(s.ctxProvider(2), sender.EvmAddress))
	require.NoError(t, s.exec.Open(3, seqVecTimeBase+3*seqVecBlockMs))
	producer, err := seqnode.NewReceipter(s.exec, s.app, &s.app.EvmKeeper, key, seqVecModuleID)
	require.NoError(t, err)
	to := seqVecRecipient
	raw, hash := subscribeRawTx(t, sender, 0, &to, new(big.Int).Set(seqVecOnePAX), pendingTransferGas, nil)
	var signedTx ethtypes.Transaction
	require.NoError(t, signedTx.UnmarshalBinary(raw))
	msg := pendingTransfer(t, sender, 0, to, new(big.Int).Set(seqVecOnePAX))
	// The canonical Cosmos transport must carry exactly the signed EVM bytes.
	transported := pendingBlockEthHash(t, s, msg)
	require.Equal(t, hash, transported)
	acc, signed, err := producer.Accept(msg)
	require.NoError(t, err)
	require.NotNil(t, acc)
	execution, err := sendSeqExecutionReceipt(acc)
	require.NoError(t, err)
	require.EqualValues(t, 1, execution.Status, execution.VmError)
	verified, err := verify.ReceiptSignature(signed, public)
	require.NoError(t, err)
	decoded := verified.Receipt
	baseDenom := s.app.EvmKeeper.GetBaseDenom(setup)
	asset := common.Hash(sha256.Sum256([]byte(baseDenom)))
	require.Equal(t, asset, common.Hash(decoded.Asset))
	require.NotEqual(t, sha256.Sum256([]byte("usid")), decoded.Asset)
	require.Equal(t, seqVecModuleID, decoded.ModuleID)
	require.Equal(t, [32]byte(seqVecKeyID), decoded.ContextHash)
	require.Equal(t, [32]byte(hash), decoded.ActivityID)
	require.Equal(t, acc.SeqHash, decoded.ResultingStateRoot)
	require.Equal(t, uint64(1), decoded.GlobalSequence)
	require.Equal(t, int32(0), decoded.ResultCode)
	require.Len(t, decoded.Effects, 1)
	summary := decoded.Effects[0].Body
	require.GreaterOrEqual(t, len(summary), 109)
	require.Equal(t, len("usid"), int(summary[108]))
	require.Equal(t, "usid", string(summary[109:]))
	nativeFee := new(big.Int).Mul(new(big.Int).SetUint64(execution.GasUsed), new(big.Int).SetUint64(execution.EffectiveGasPrice))
	converted, err := evmkeeper.ConvertFeeToDenom(sdk.NewIntFromBigInt(nativeFee), rate, true)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(32697), converted)
	fee := decoded.FeeCharged.Bytes()
	require.Equal(t, converted.BigInt(), new(big.Int).SetBytes(fee[:]))
	after, err := s.exec.Snapshot()
	require.NoError(t, err)
	require.Equal(t, converted, sdk.NewInt(1_000_000_000).Sub(s.app.BankKeeper.GetBalance(after, sender.AccountAddress, "usid").Amount))

	source := seqnode.NewReceiptSource(4)
	source.Record(hash, evmrpc.SequencerSend{Nonce: 0, Receipt: signed, ReturnData: []byte{}, LogData: [][]byte{}})
	port := startSeqReceiptServer9005(t, s.app, s.ctxProvider, evmrpc.SequencerReceiptOption{Source: source, Keeper: &s.app.SequencerKeeper})
	answer, rpcErr := callSeqReceipt9005(t, port, evmrpc.SequencerReceiptMethod, hash)
	require.Nil(t, rpcErr)
	var result *evmrpc.SequencerReceiptResult
	require.NoError(t, json.Unmarshal(answer, &result))
	require.NotNil(t, result)
	require.NoError(t, result.Verify(pins))
	require.Equal(t, signed, []byte(result.Receipt))
	require.Equal(t, baseDenom, result.PaymentDenom)
	require.Equal(t, "usid", result.FeeDenom)
	require.Equal(t, converted.BigInt(), result.FeeAmount.ToInt())
	require.Equal(t, nativeFee, new(big.Int).Mul(new(big.Int).SetUint64(uint64(result.GasUsed)), new(big.Int).SetUint64(uint64(result.EffectiveGasPrice))))
	refusedAt, err := seqVecClientCheck(raw, answer, common.Hash{}, pin)
	require.NoError(t, err, refusedAt)
	c := seqVecCaseOf("V1-fee-token-payment", "V1", "Base payment asset with a converted registered fee-token charge",
		seqVecSigned{signed: signed, digest: verified.Digest}, raw, answer, [32]byte{}, pin)
	c.Expected = seqVecExpected{Rung: "receipted", Verifier: "accepts"}
	c.PaymentDenom, c.FeeDenom, c.Asset = baseDenom, "usid", &asset
	return seqVecFile{Name: "sequencer-receipt-fee-token-v1", Provenance: seqVecProvenance{Generator: seqVecGenerator, Command: seqVecFeeTokenCommand}, ChainID: seqVecChainID, ModuleID: seqVecModuleID, FeeDenom: "usid", Cases: []seqVecCase{c}}
}

func TestSequencerReceiptFeeTokenVector(t *testing.T) {
	file := repairM1297FeeTokenVector(t)
	seqVecWriteFixture(t, seqVecFeeTokenFixture, seqVecFeeTokenCommand, file)
}

func repairM1297FeeTokenVectorProof(t *testing.T) {
	file := repairM1297FeeTokenVector(t)
	require.Len(t, file.Cases, 1)
	// Retain the producer-generated replacement privately; publishing the SDK
	// fixture is the out-of-scope wiring request recorded in the proof index.
	rendered, err := json.MarshalIndent(file, "", "  ")
	require.NoError(t, err)
	path := "/tmp/repair-m1297-fee-token-vector.json"
	require.NoError(t, os.WriteFile(path, append(rendered, '\n'), 0o600))
	t.Logf("producer fixture: %s", path)
}

func TestVectorFilesV1ToV3MatchSequencerReceiptFixture(t *testing.T) {
	for name, digest := range map[string]string{
		"vector-v1-receipted-positive.json": "d74c1e130b5f9e3687655a4f8a1c048e0ee77a479931a87d790510f4419a22e2",
		"vector-v2-receipted-revert.json":   "64fa4664780380bd6da0e36cd2444fa41e751e9fb0919514a4cb2a6f73feea0a",
		"vector-v3-receipt-refusals.json":   "480ed0e242f16cd5064ba8a7a613ef24e96e9119d9327da3ffd67563ca12df5e",
	} {
		recorded, err := os.ReadFile(filepath.Join("../daemon/committed-fixture/vectors", name))
		require.NoError(t, err)
		require.Equal(t, digest, fmt.Sprintf("%x", sha256.Sum256(recorded)), "receipted vectors must retain their original recording")
	}
	var source struct {
		Cases []json.RawMessage `json:"cases"`
	}
	stored, err := os.ReadFile(seqVecFixture)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(stored, &source))
	require.Len(t, source.Cases, 13)
	for _, group := range []struct {
		name, vector string
		count        int
	}{
		{"vector-v1-receipted-positive", "V1", 5},
		{"vector-v2-receipted-revert", "V2", 2},
		{"vector-v3-receipt-refusals", "V3", 6},
	} {
		t.Run(group.vector, func(t *testing.T) {
			var expected []json.RawMessage
			for _, raw := range source.Cases {
				var c seqVecCase
				require.NoError(t, json.Unmarshal(raw, &c))
				if c.Vector == group.vector {
					expected = append(expected, raw)
				}
			}
			var copy struct {
				Name  string            `json:"name"`
				Cases []json.RawMessage `json:"cases"`
			}
			require.Len(t, expected, group.count)
			paths := []string{filepath.Join(filepath.Dir(seqVecFixture), group.name+".json")}
			if group.vector == "V1" {
				paths = append(paths, filepath.Join("../daemon/committed-fixture/vectors", group.name+".json"))
			}
			for _, path := range paths {
				stored, err := os.ReadFile(path)
				require.NoError(t, err)
				if *updateSequencerReceiptVectors {
					var document map[string]json.RawMessage
					require.NoError(t, json.Unmarshal(stored, &document))
					document["cases"], err = json.Marshal(expected)
					require.NoError(t, err)
					stored, err = json.MarshalIndent(document, "", "  ")
					require.NoError(t, err)
					stored = append(stored, '\n')
					require.NoError(t, os.WriteFile(path, stored, 0o644))
				}
				require.NoError(t, json.Unmarshal(stored, &copy))
				require.Equal(t, group.name, copy.Name)
				require.Len(t, copy.Cases, group.count)
				require.Equal(t, expected, copy.Cases, "%s cases must be byte-identical to the source", path)
			}
		})
	}
}

// seqVecAssertCases checks every V1 and V2 case through R1-R6 and its decoded fields, and every V3
// case refusing at exactly its named check.
func seqVecAssertCases(t *testing.T, cases []seqVecCase, refusals map[string]seqVecRefusal) {
	t.Helper()
	require.Len(t, cases, 13)
	counts := map[string]int{}
	seen := map[string]bool{}
	for _, c := range cases {
		require.False(t, seen[c.ID], "duplicate vector case %s", c.ID)
		seen[c.ID] = true
		counts[c.Vector]++
	}
	require.Equal(t, map[string]int{"V1": 5, "V2": 2, "V3": 6}, counts)
	require.True(t, seen["V1-layerxprograms-call"])
	require.True(t, seen["V2-pax-transfer-insufficient-funds"])
	for _, c := range cases {
		check, err := seqVecClientCheck(c.RawTransaction, c.ReceiptJSON, c.PrevSeqHash, c.PinnedKey)
		if c.Vector == "V3" {
			refusal, ok := refusals[c.ID]
			require.True(t, ok, c.ID)
			require.Equal(t, refusal.refusedAt, check, "%s refuses at %s: %v", c.ID, refusal.refusedAt, err)
			require.ErrorIs(t, err, refusal.err, c.ID)
			continue
		}
		require.Equal(t, "", check, "%s: %v", c.ID, err)
		require.NoError(t, err, c.ID)
		seqVecAssertFields(t, c)
	}
}

func seqVecAssertFields(t *testing.T, c seqVecCase) {
	t.Helper()
	var result *evmrpc.SequencerReceiptResult
	require.NoError(t, json.Unmarshal(c.ReceiptJSON, &result))
	verified, err := verify.ReceiptSignature(c.SignedReceipt, [32]byte(c.PinnedKey.PublicKey))
	require.NoError(t, err, c.ID)
	require.Equal(t, c.ReceiptDigest, common.Hash(verified.Digest), c.ID)
	decoded := verified.Receipt
	require.Equal(t, []byte(c.SignedReceipt), []byte(result.Receipt), c.ID)
	require.Equal(t, uint16(2), decoded.ProtocolVersion)
	require.Equal(t, seqVecModuleID, decoded.ModuleID)
	require.Equal(t, uint32(seqnode.ReceiptLayoutVersion), decoded.ModuleVersion)
	require.Equal(t, c.PinnedKey.KeyID, common.Hash(decoded.ContextHash))
	require.Equal(t, uint64(result.Height), binary.BigEndian.Uint64(decoded.BatchID[24:]))
	require.Equal(t, uint8(ethtypes.DynamicFeeTxType), decoded.Operation)
	require.Equal(t, sha256.Sum256([]byte(evmkeeper.BaseDenom)), decoded.Asset)
	require.Equal(t, evmkeeper.BaseDenom, result.FeeDenom)
	require.Equal(t, c.PinnedKey.KeyID, result.KeyID)
	require.Equal(t, sha256.Sum256(result.ReturnData), decoded.ActivityRoot)
	require.Equal(t, uint64(2_000_000_000), uint64(result.EffectiveGasPrice))
	fee := new(big.Int).Mul(new(big.Int).SetUint64(uint64(result.GasUsed)), big.NewInt(2_000_000_000))
	require.Equal(t, 0, fee.Cmp(result.FeeAmount.ToInt()), c.ID)
	tx := new(ethtypes.Transaction)
	require.NoError(t, tx.UnmarshalBinary(c.RawTransaction))
	require.Equal(t, big.NewInt(seqVecChainID), tx.ChainId())
	require.Equal(t, tx.Nonce(), uint64(result.Nonce))
	require.Equal(t, *tx.To(), result.To)
	require.Equal(t, uint64(seqVecTimeBase)+uint64(result.Height)*seqVecBlockMs, uint64(result.Timestamp))
	switch c.Vector {
	case "V1":
		require.Equal(t, uint64(1), uint64(result.Status), c.ID)
		require.Equal(t, int32(0), decoded.ResultCode)
		require.Empty(t, result.VMError)
		require.Empty(t, c.LXPCode)
		topic, logged := map[string]string{
			"V1-erc20-approve":           "Approval(address,address,uint256)",
			"V1-layerxledger-pay-exact":  "PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)",
			"V1-layerxbridge-bridge-out": "BridgeOut(uint64,address,uint256,address,uint64)",
			"V1-layerxprograms-call":     "ProgramCallOutcome(bytes32,bytes32,bytes32,uint64,uint8,uint16,uint16,uint32,uint32,int32,bytes32,bytes32,bytes32,bytes32)",
		}[c.ID]
		if !logged {
			require.Equal(t, "V1-pax-transfer", c.ID)
			require.Empty(t, result.Logs)
			require.Equal(t, 0, tx.Value().Cmp(seqVecOnePAX))
			break
		}
		require.Len(t, result.Logs, 1, c.ID)
		require.Equal(t, crypto.Keccak256Hash([]byte(topic)), result.Logs[0].Topics[0], c.ID)
		require.Equal(t, *tx.To(), result.Logs[0].Address, c.ID)
		if c.ID == "V1-layerxprograms-call" {
			rawABI, err := os.ReadFile("../precompiles/layerxprograms/abi.json")
			require.NoError(t, err)
			programs, err := abi.JSON(bytes.NewReader(rawABI))
			require.NoError(t, err)
			method := programs.Methods["call"]
			require.Equal(t, method.ID, tx.Data()[:4])
			inputs, err := method.Inputs.Unpack(tx.Data()[4:])
			require.NoError(t, err)
			require.Equal(t, sha256.Sum256([]byte("sequencer receipt vector program")), inputs[0])
			require.Equal(t, uint16(1), inputs[1])
			require.Equal(t, "layerx_call", inputs[2])
			log := result.Logs[0]
			require.Equal(t, lp.Address, log.Address)
			require.Equal(t, []common.Hash{programs.Events["ProgramCallOutcome"].ID, common.Hash(inputs[0].([32]byte))}, log.Topics)
			outcome, err := programs.Events["ProgramCallOutcome"].Inputs.NonIndexed().Unpack(log.Data)
			require.NoError(t, err)
			require.NotEqual(t, [32]byte{}, outcome[0])
			require.NotEqual(t, [32]byte{}, outcome[1])
			require.Equal(t, uint64(0), outcome[2])
			require.Equal(t, uint8(0), outcome[3])
			require.Equal(t, inputs[1], outcome[5])
			require.Equal(t, uint32(1), outcome[6])
			require.Equal(t, uint32(1), outcome[7])
			require.Equal(t, int32(0), outcome[8])
			output, err := method.Outputs.Unpack(result.ReturnData)
			require.NoError(t, err)
			require.Equal(t, outcome[8], output[0])
			require.Equal(t, uint8(1), output[1])
			seqVecAssertProgramTerminal(t, output[2].([]byte))
		}
		if c.ID == "V1-layerxledger-pay-exact" {
			require.Len(t, tx.Data(), 4+6*32, c.ID)
			require.Len(t, result.Logs[0].Topics, 4, c.ID)
			for index, word := range []int{5, 0, 1} {
				require.Equal(t, common.BytesToHash(tx.Data()[4+word*32:4+(word+1)*32]), result.Logs[0].Topics[index+1], c.ID)
			}
			require.Equal(t, common.HexToHash("0x112b929c0d6cb01c57eaad307e6d0a094c6c6e58f2e0ce3c62856cf815c0fba4"), result.Logs[0].Topics[3], c.ID)
		}
	case "V2":
		require.Equal(t, uint64(0), uint64(result.Status), c.ID)
		require.Equal(t, int32(1), decoded.ResultCode)
		if c.ID == "V2-pax-transfer-insufficient-funds" {
			require.Equal(t, vm.ErrInsufficientBalance.Error(), result.VMError)
			require.Empty(t, result.ReturnData)
			require.Empty(t, result.Logs)
			require.Empty(t, c.LXPCode)
			require.Empty(t, tx.Data())
			require.Equal(t, seqVecRecipient, *tx.To())
			require.Positive(t, tx.Value().Cmp(seqVecStartBalance))
			require.Zero(t, uint64(result.GasUsed))
			require.Len(t, decoded.Effects, 1)
			return
		}
		require.Equal(t, "V2-layerxledger-revert", c.ID)
		require.Equal(t, "execution reverted", result.VMError)
		require.Empty(t, result.Logs)
		reason, err := abi.UnpackRevert(result.ReturnData)
		require.NoError(t, err)
		code, message, found := strings.Cut(reason, ": ")
		require.True(t, found, reason)
		require.Equal(t, c.LXPCode, code)
		require.Equal(t, "insufficient balance", message)
		require.Equal(t, pcommon.LXPRevert(code, ""), []byte(result.ReturnData), "the revert maps through the error code registry")
	default:
		require.Fail(t, fmt.Sprintf("unexpected vector %s", c.Vector))
	}
}

func init() { evmrpc.RepairM1297Proofs["fee-token-vector-reversed"] = repairM1297FeeTokenVectorProof }
