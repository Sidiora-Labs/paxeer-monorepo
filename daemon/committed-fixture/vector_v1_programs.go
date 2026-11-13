package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/addr"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

//go:embed programs_abi.json
var fixtureProgramsABI embed.FS

var v1ProgramsABI = pcommon.MustGetABI(fixtureProgramsABI, "programs_abi.json")

const v1ProgramsName = "vector-v1-programs-positive"
const v1ProgramsCaseID = "V1-layerxprograms-call"

// Match TestProgramsDeployRegistersProgram's deploy payload ABI 1 and
// LayerX/program-interface/v1 encoding. The EVM deploy call reaches submit.go,
// which builds activity 0x00090001 with codec.StateCommitmentProtocolVersion;
// that constant and LXP_PROTOCOL_VERSION_STATE_COMMITMENT are both 3.
const v1ProgramsABIVersion uint16 = 1

type v1ProgramsVector struct {
	Name           string                         `json:"name"`
	Provenance     vectorV2Provenance             `json:"provenance"`
	ChainID        *hexutil.Big                   `json:"chainId"`
	Program        common.Hash                    `json:"programId"`
	DeploymentRaw  hexutil.Bytes                  `json:"deploymentRawTransaction"`
	Deployment     *evmrpc.SequencerReceiptResult `json:"deploymentReceipt"`
	RawTransaction hexutil.Bytes                  `json:"rawTransaction"`
	Receipt        *evmrpc.SequencerReceiptResult `json:"receiptJson"`
	ReceiptKeys    seqkeeper.ReceiptKeys          `json:"receiptKeys"`
}

// The module has no imports or guest effects. Its actual execution emits the
// terminal ProgramCallOutcome; no event or return value is supplied by a peer.
func v1ProgramsRecipe(principal [32]byte) (common.Hash, []byte, []byte, error) {
	wasm := []byte{0, 97, 115, 109, 1, 0, 0, 0, 1, 12, 2, 96, 2, 127, 127, 1, 127, 96, 1, 127, 1, 127, 3, 3, 2, 0, 1, 5, 3, 1, 0, 1, 7, 41, 3,
		11, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'c', 'a', 'l', 'l', 0, 0, 14, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'r', 'e', 's', 'e', 'r', 'v', 'e', 0, 1,
		6, 'm', 'e', 'm', 'o', 'r', 'y', 2, 0, 10, 11, 2, 4, 0, 65, 0, 11, 4, 0, 65, 0, 11}
	codeHash := sha256.Sum256(wasm)
	program := sha256.Sum256(append([]byte("committed V1 Programs producer"), principal[:]...))
	iface := append([]byte("LayerX/program-interface/v1\x00"), codeHash[:]...)
	iface = append(iface, 0, 1, 0, 1, 0, 11)
	iface = append(iface, []byte("layerx_call")...)
	iface = append(iface, 0, 0, 0, 0, 1, 0x20, 0, 0, 0, 64, 1, 0x20, 0, 0, 0, 64, 0, 0, 0, 0, 0, 0)
	deploy, err := v1ProgramsABI.Pack("deploy", program, v1ProgramsABIVersion, uint8(1), principal, codeHash, iface, wasm)
	if err != nil {
		return common.Hash{}, nil, nil, err
	}
	budget := [7]uint64{1000000, 16777216, 1048576, 1048576, 64, 1048576, 4096}
	call, err := v1ProgramsABI.Pack("call", program, v1ProgramsABIVersion, "layerx_call", []byte{}, []byte{0, 0}, []byte("LayerX/programs/access-declaration/v1\x00\x00"), uint32(64), budget, big.NewInt(100000000))
	return common.Hash(program), deploy, call, err
}

func v1ProgramsSigner(payer *ecdsa.PrivateKey) (*ecdsa.PrivateKey, error) {
	seed := sha256.Sum256(append(payer.D.FillBytes(make([]byte, 32)), []byte("V1 Programs EVM signer")...))
	defer clear(seed[:])
	return crypto.ToECDSA(seed[:])
}

func v1ProgramsIdentity(key *ecdsa.PrivateKey) (ed25519.PrivateKey, [32]byte, error) {
	seed := sha256.Sum256(append(key.D.FillBytes(make([]byte, 32)), []byte("V1 Programs DID")...))
	private := ed25519.NewKeyFromSeed(seed[:])
	clear(seed[:])
	public := [32]byte(private.Public().(ed25519.PublicKey))
	principal, err := lxphost.DIDID([]byte(evmtypes.LayerXDid(public)))
	return private, principal, err
}

func (n *node) v1ProgramsGenesisAuthority() ([32]byte, error) {
	_, actor, err := n.v1ProgramsGenesisIdentity()
	return actor, err
}

type v1ProgramsAuthority struct {
	evm *ecdsa.PrivateKey
	did ed25519.PrivateKey
}

func newV1ProgramsAuthority() (*v1ProgramsAuthority, error) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	key, err := crypto.GenerateKey()
	if err != nil {
		clear(private)
		return nil, err
	}
	return &v1ProgramsAuthority{evm: key, did: private}, nil
}

func (n *node) v1ProgramsGenesisIdentity() ([32]byte, [32]byte, error) {
	if n.programsAuthority == nil || len(n.programsAuthority.did) != ed25519.PrivateKeySize {
		return [32]byte{}, [32]byte{}, errors.New("V1 Programs genesis authority is not initialized")
	}
	public := [32]byte(n.programsAuthority.did.Public().(ed25519.PublicKey))
	actor, err := lxphost.DIDID([]byte(evmtypes.LayerXDid(public)))
	return public, actor, err
}

// Prepared calls require a registered fee asset even when the guest imports
// nothing: lxp_kernel_batch_snapshot_create refuses asset_count == 0 at
// src/protocol/lxp_kernel.c:811, before a call replay record can be produced.
// These are genesis balances for native fee accounting, as in seedProgram in
// tests/daemon/recorder; they do not grant the guest any transfer capability.
func v1ProgramsGenesisFunding(public, actor [32]byte) ([32]byte, []kv.Pair, error) {
	salt := bytes.Repeat([]byte{0x55}, 32)
	material := append([]byte("LX:ASSET:v1"), actor[:]...)
	asset := sha256.Sum256(append(material, salt...))
	mainName, mainID, err := lxphost.MainAccountID(public)
	if err != nil {
		return [32]byte{}, nil, err
	}
	payerName, payerID, err := lxphost.AssetAccountID(public, asset)
	if err != nil {
		return [32]byte{}, nil, err
	}
	treasuryID, err := codec.DeriveAccountID([]byte("system:fees"))
	if err != nil {
		return [32]byte{}, nil, err
	}
	amount := func(out []byte, value uint64) []byte {
		return binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(out, 0), value)
	}
	field := func(out, value []byte) []byte {
		return append(binary.BigEndian.AppendUint32(out, uint32(len(value))), value...)
	}
	// lx_asset_record_encode's version-3 native issuance record, with total
	// units equal to the funded fee payer. The main balance uses the separate
	// fixture asset, as seedProgram does: payment-account resolution requires
	// exactly one account of this DID holding the fee asset (lxp_kernel.c:258).
	record := append([]byte{0, 3}, asset[:]...)
	record = append(record, 1, 'T', 6, 1, 0, 0, 0, 1, 'T')
	record = amount(record, 1000000000)
	record = append(record, actor[:]...)
	record = append(record, 1)
	record = amount(record, 500000000)
	record = append(record, salt...)
	rows := []kv.Pair{{Key: append([]byte{1, 0, 1}, append([]byte("asset:"), asset[:]...)...), Value: record}}
	for _, account := range []struct {
		name      string
		id        [32]byte
		kind      byte
		balance   uint64
		authority bool
	}{
		{mainName, mainID, 1, 100000000, true},
		{payerName, payerID, 14, 500000000, true},
		{"system:fees", treasuryID, 10, 0, false},
	} {
		value := field(nil, account.id[:])
		value = field(value, []byte(account.name))
		value = append(value, account.kind)
		value = amount(value, account.balance)
		accountAsset := asset
		if account.id == mainID {
			accountAsset = [32]byte{0x91}
		}
		value = field(value, accountAsset[:])
		value = append(value, 1)
		value = binary.BigEndian.AppendUint64(value, 0)
		value = binary.BigEndian.AppendUint64(value, 0)
		value = append(value, 0, 0)
		if account.authority {
			value = append(field(value, public[:]), 1)
		} else {
			value = append(field(value, make([]byte, 32)), 0)
		}
		rows = append(rows, kv.Pair{Key: append([]byte{6}, account.id[:]...), Value: value})
	}
	return asset, rows, nil
}

func v1ProgramsRevertError(operation string, data []byte) error {
	activity := uint32(0x00090001)
	if operation == "call" {
		activity = 0x00090003
	}
	reason, err := abi.UnpackRevert(data)
	if err != nil {
		return fmt.Errorf("V1 Programs %s reverted (submit protocol %d, activity 0x%08x, ABI %d): cannot decode revert data %x: %w", operation, codec.StateCommitmentProtocolVersion, activity, v1ProgramsABIVersion, data, err)
	}
	return fmt.Errorf("V1 Programs %s reverted (submit protocol %d, activity 0x%08x, ABI %d): %s (revert data %x)", operation, codec.StateCommitmentProtocolVersion, activity, v1ProgramsABIVersion, reason, data)
}

func recordV1ProgramsRPC(n *node, key *ecdsa.PrivateKey, pins seqkeeper.ReceiptKeys, provenance vectorV2Provenance) (*v1ProgramsVector, error) {
	worker, err := v1ProgramsSigner(key)
	if err != nil {
		return nil, err
	}
	defer worker.D.SetInt64(0)
	private, _, err := v1ProgramsIdentity(worker)
	if err != nil {
		return nil, err
	}
	defer clear(private)
	return recordV1ProgramsAuthorityRPC(n, key, worker, private, pins, provenance)
}

func recordV1ProgramsAuthorityRPC(n *node, key, worker *ecdsa.PrivateKey, private ed25519.PrivateKey, pins seqkeeper.ReceiptKeys, provenance vectorV2Provenance) (*v1ProgramsVector, error) {
	if err := pins.Validate(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(n.ctx, 30*time.Second)
	defer cancel()
	var chainID, price hexutil.Big
	if err := n.client.CallContext(ctx, &chainID, "eth_chainId"); err != nil {
		return nil, err
	}
	if err := n.client.CallContext(ctx, &price, "eth_gasPrice"); err != nil {
		return nil, err
	}
	_, funded, err := sendV2Call(ctx, n, chainID.ToInt(), price.ToInt(), v2Call{key, crypto.PubkeyToAddress(worker.PublicKey), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil), nil})
	if err != nil {
		return nil, err
	}
	if err := funded.Verify(pins); err != nil {
		return nil, err
	}
	if funded.Status != 1 {
		return nil, errors.New("V1 Programs signer funding failed")
	}
	key = worker
	public := [32]byte(private.Public().(ed25519.PublicKey))
	message, err := evmtypes.LayerXBindMessage(chainID.ToInt(), crypto.PubkeyToAddress(key.PublicKey), 0)
	if err != nil {
		return nil, err
	}
	addressABI, err := abi.JSON(strings.NewReader(`[{"type":"function","name":"bindLayerX","inputs":[{"name":"didPublicKey","type":"bytes32"},{"name":"signature","type":"bytes"}],"outputs":[]},{"type":"function","name":"getLayerXDid","inputs":[{"name":"account","type":"address"}],"outputs":[{"name":"publicKey","type":"bytes32"},{"name":"did","type":"string"}]}]`))
	if err != nil {
		return nil, err
	}
	binding, err := addressABI.Pack(addr.BindLayerXMethod, public, ed25519.Sign(private, message))
	if err != nil {
		return nil, err
	}
	_, bound, err := sendV2Call(ctx, n, chainID.ToInt(), price.ToInt(), v2Call{key, common.HexToAddress(addr.AddrAddress), new(big.Int), binding})
	if err != nil {
		return nil, err
	}
	if err := bound.Verify(pins); err != nil {
		return nil, err
	}
	if bound.Status != 1 {
		return nil, errors.New("V1 Programs DID binding failed")
	}
	query, err := addressABI.Pack(addr.GetLayerXDidMethod, crypto.PubkeyToAddress(key.PublicKey))
	if err != nil {
		return nil, err
	}
	var output hexutil.Bytes
	if err := v4PendingCall(ctx, n, &output, "eth_call", map[string]any{"to": addr.AddrAddress, "data": hexutil.Bytes(query)}, "pending"); err != nil {
		return nil, err
	}
	resolved, err := addressABI.Unpack(addr.GetLayerXDidMethod, output)
	if err != nil {
		return nil, err
	}
	if resolved[0].([32]byte) != public || resolved[1].(string) != evmtypes.LayerXDid(public) {
		return nil, errors.New("V1 Programs caller binding differs from the deployment authority")
	}
	// Use the bound caller's authority principal, as principalOf in the
	// programs precompile does, for the policy-1 upgrade authority.
	principal, err := lxphost.DIDID([]byte(resolved[1].(string)))
	if err != nil {
		return nil, err
	}
	program, deploy, call, err := v1ProgramsRecipe(principal)
	if err != nil {
		return nil, err
	}
	deploymentRaw, deployment, err := sendV2Call(ctx, n, chainID.ToInt(), price.ToInt(), v2Call{key, lp.Address, new(big.Int), deploy})
	if err != nil {
		return nil, fmt.Errorf("V1 Programs deployment: %w", err)
	}
	if err := deployment.Verify(pins); err != nil {
		return nil, err
	}
	if deployment.Status != 1 {
		return nil, v1ProgramsRevertError("deployment", deployment.ReturnData)
	}
	raw, receipt, err := sendV2Call(ctx, n, chainID.ToInt(), price.ToInt(), v2Call{key, lp.Address, new(big.Int), call})
	if err != nil {
		return nil, err
	}
	if err := receipt.Verify(pins); err != nil {
		return nil, err
	}
	if receipt.Status != 1 {
		return nil, v1ProgramsRevertError("call", receipt.ReturnData)
	}
	provenance.Source = vectorV2SourcePath + "#" + v1ProgramsCaseID
	provenance.FirstHeight, provenance.LastHeight = uint64(deployment.Height), uint64(receipt.Height)
	v := &v1ProgramsVector{v1ProgramsName, provenance, &chainID, program, deploymentRaw, deployment, raw, receipt, pins}
	if err := verifyV1Programs(v); err != nil {
		return nil, err
	}
	return v, nil
}

func verifyV1Programs(v *v1ProgramsVector) error {
	if v == nil || v.Name != v1ProgramsName || v.ChainID == nil || v.ChainID.ToInt().Sign() <= 0 || v.Deployment == nil || v.Receipt == nil {
		return errors.New("V1 Programs requires deployment and call evidence")
	}
	if revision, err := hex.DecodeString(v.Provenance.Revision); err != nil || len(revision) != 20 {
		return errors.New("V1 Programs provenance must name the full recorder revision")
	}
	if v.Provenance.Generator == "" || v.Provenance.Command == "" || v.Provenance.ChainID == "" || v.Provenance.Source != vectorV2SourcePath+"#"+v1ProgramsCaseID || v.Provenance.FirstHeight == 0 || v.Provenance.FirstHeight > uint64(v.Deployment.Height) || v.Provenance.LastHeight < uint64(v.Receipt.Height) {
		return errors.New("V1 Programs provenance is incomplete")
	}
	if err := v.ReceiptKeys.Validate(); err != nil {
		return err
	}
	var sender common.Address
	var principal [32]byte
	for i, pair := range []struct {
		raw     []byte
		receipt *evmrpc.SequencerReceiptResult
		method  string
	}{{v.DeploymentRaw, v.Deployment, "deploy"}, {v.RawTransaction, v.Receipt, "call"}} {
		if err := pair.receipt.Verify(v.ReceiptKeys); err != nil {
			return err
		}
		var tx ethtypes.Transaction
		if err := tx.UnmarshalBinary(pair.raw); err != nil {
			return err
		}
		from, err := ethtypes.Sender(ethtypes.LatestSignerForChainID(tx.ChainId()), &tx)
		if err != nil {
			return err
		}
		if tx.Type() != ethtypes.DynamicFeeTxType || tx.ChainId().Cmp(v.ChainID.ToInt()) != 0 || tx.To() == nil || *tx.To() != lp.Address || tx.Value().Sign() != 0 || tx.Hash() != pair.receipt.TxHash || tx.Nonce() != uint64(pair.receipt.Nonce) || pair.receipt.From != from || pair.receipt.To != lp.Address || pair.receipt.Status != 1 || pair.receipt.Rung != evmrpc.RungReceipted {
			return fmt.Errorf("V1 Programs %s transaction does not bind its accepted receipt: type=%d chain=%s/%s to=%v/%s from=%s/%s hash=%s/%s nonce=%d/%d status=%d rung=%s", pair.method, tx.Type(), tx.ChainId(), v.ChainID.ToInt(), tx.To(), pair.receipt.To, from, pair.receipt.From, tx.Hash(), pair.receipt.TxHash, tx.Nonce(), uint64(pair.receipt.Nonce), pair.receipt.Status, pair.receipt.Rung)
		}
		if len(tx.Data()) < 4 || !bytes.Equal(tx.Data()[:4], v1ProgramsABI.Methods[pair.method].ID) {
			return errors.New("V1 Programs transaction has another method")
		}
		args, err := v1ProgramsABI.Methods[pair.method].Inputs.Unpack(tx.Data()[4:])
		if err != nil {
			return err
		}
		if common.Hash(args[0].([32]byte)) != v.Program {
			return errors.New("V1 Programs deployment and call target different programs")
		}
		if i == 0 {
			principal = args[3].([32]byte)
			_, expectedDeploy, _, err := v1ProgramsRecipe(principal)
			if err != nil {
				return err
			}
			if !bytes.Equal(tx.Data(), expectedDeploy) {
				return errors.New("V1 Programs deployment differs from the committed module")
			}
		} else {
			_, _, expectedCall, err := v1ProgramsRecipe(principal)
			if err != nil {
				return err
			}
			if !bytes.Equal(tx.Data(), expectedCall) {
				return errors.New("V1 Programs call differs from the committed entrypoint and bounds")
			}
		}
		if i == 0 {
			sender = from
		} else if from != sender {
			return errors.New("V1 Programs caller differs from deployer")
		}
	}
	logs := v.Receipt.Logs
	if len(logs) != 1 || logs[0].Address != lp.Address || uint64(logs[0].LogIndex) != 0 || len(logs[0].Topics) != 2 || logs[0].Topics[0] != v1ProgramsABI.Events["ProgramCallOutcome"].ID || logs[0].Topics[1] != v.Program {
		return errors.New("V1 Programs requires the authenticated ProgramCallOutcome log")
	}
	fields, err := v1ProgramsABI.Events["ProgramCallOutcome"].Inputs.NonIndexed().Unpack(logs[0].Data)
	if err != nil {
		return err
	}
	if fields[0].([32]byte) != principal || fields[8].(int32) != 0 || fields[4].(uint16) != 1 || fields[5].(uint16) != v1ProgramsABIVersion {
		return errors.New("V1 Programs terminal outcome differs from the deployed principal or versions")
	}
	decoded, err := lp.DecodeProgramExecutionResult(v.Receipt.ReturnData, 64, fields[9].([32]byte))
	if err != nil {
		return err
	}
	if decoded.TerminalResult != 0 || decoded.TerminalKind != 1 || len(decoded.GuestResponse) != 0 {
		return errors.New("V1 Programs return data differs from the successful terminal outcome")
	}
	return nil
}

type V1ProgramsCaseVector = v1ProgramsVector

type v1ProgramsSourceCase struct {
	*V1ProgramsCaseVector
	ID            string        `json:"id"`
	Vector        string        `json:"vector"`
	Description   string        `json:"description"`
	SignedReceipt hexutil.Bytes `json:"signedReceipt"`
	ReceiptDigest common.Hash   `json:"receiptDigest"`
	PrevSeqHash   common.Hash   `json:"prevSeqHash"`
	PinnedKey     v2Pin         `json:"pinnedKey"`
	Expected      struct {
		Rung     string `json:"rung"`
		Verifier string `json:"verifier"`
	} `json:"expected"`
}

type v1ProgramsSource struct {
	Name     string                 `json:"name"`
	ChainID  uint64                 `json:"chainId"`
	ModuleID uint16                 `json:"moduleId"`
	FeeDenom string                 `json:"feeDenom"`
	Cases    []v1ProgramsSourceCase `json:"cases"`
}

func canonicalV1Programs(v *v1ProgramsVector) (*v1ProgramsSource, error) {
	if err := verifyV1Programs(v); err != nil {
		return nil, err
	}
	if !v.ChainID.ToInt().IsUint64() {
		return nil, errors.New("V1 Programs source chain id exceeds uint64")
	}
	verified, err := verify.ReceiptSignature(v.Receipt.Receipt, [32]byte(v.Receipt.SigningKey))
	if err != nil {
		return nil, err
	}
	var pinned v2Pin
	for _, pin := range v.ReceiptKeys {
		if bytes.Equal(pin.KeyId, v.Receipt.KeyID[:]) && bytes.Equal(pin.PublicKey, v.Receipt.SigningKey) {
			pinned = v2Pin{common.BytesToHash(pin.KeyId), bytes.Clone(pin.PublicKey), pin.FirstHeight, pin.LastHeight}
		}
	}
	if len(pinned.PublicKey) != 32 {
		return nil, errors.New("V1 Programs signer has no pin")
	}
	c := v1ProgramsSourceCase{V1ProgramsCaseVector: v, ID: v1ProgramsCaseID, Vector: "V1", Description: "real layerxprograms deployment and accepted call with ProgramCallOutcome", SignedReceipt: bytes.Clone(v.Receipt.Receipt), ReceiptDigest: common.Hash(verified.Digest), PrevSeqHash: common.Hash(verified.Receipt.PreviousStateRoot), PinnedKey: pinned}
	c.Expected.Rung, c.Expected.Verifier = string(v.Receipt.Rung), "accepts"
	return &v1ProgramsSource{"sequencer-receipt-v1", v.ChainID.ToInt().Uint64(), verified.Receipt.ModuleID, v.Receipt.FeeDenom, []v1ProgramsSourceCase{c}}, nil
}

func v1ProgramsFromSource(source *v1ProgramsSource) (*v1ProgramsVector, error) {
	if source == nil || source.Name != "sequencer-receipt-v1" || len(source.Cases) != 1 {
		return nil, errors.New("expected the canonical V1 Programs group")
	}
	c := source.Cases[0]
	if c.ID != v1ProgramsCaseID || c.Vector != "V1" || c.V1ProgramsCaseVector == nil {
		return nil, errors.New("V1 Programs canonical case missing")
	}
	v := c.V1ProgramsCaseVector
	if err := verifyV1Programs(v); err != nil {
		return nil, err
	}
	expected, err := canonicalV1Programs(v)
	if err != nil {
		return nil, err
	}
	a, err := json.Marshal(source)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(expected)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(a, b) {
		return nil, errors.New("V1 Programs source differs from authenticated evidence")
	}
	return v, nil
}

func writeV1ProgramsCopies(out string, v *v1ProgramsVector) error {
	if err := verifyV1Programs(v); err != nil {
		return err
	}
	// A copy is decoded from the canonical bytes before it is written.
	source, err := canonicalV1Programs(v)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(source, "", "  ")
	if err != nil {
		return err
	}
	var transported v1ProgramsSource
	if err := json.Unmarshal(encoded, &transported); err != nil {
		return err
	}
	if len(transported.Cases) != 1 {
		return errors.New("V1 Programs canonical case missing")
	}
	copy, err := v1ProgramsFromSource(&transported)
	if err != nil {
		return err
	}
	if err := verifyV1Programs(copy); err != nil {
		return err
	}
	if copy.Receipt.TxHash != v.Receipt.TxHash || copy.Deployment.TxHash != v.Deployment.TxHash || !bytes.Equal(copy.RawTransaction, v.RawTransaction) || !bytes.Equal(copy.DeploymentRaw, v.DeploymentRaw) {
		return errors.New("V1 Programs canonical copy changed transaction identity")
	}
	conformance := filepath.Join(out, "conformance")
	if err := os.MkdirAll(conformance, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(conformance, "sequencer-receipt-v1-programs.json"), append(encoded, '\n'), 0644); err != nil {
		return err
	}
	data, err := json.MarshalIndent(copy, "", "  ")
	if err != nil {
		return err
	}
	for _, path := range []string{filepath.Join(out, v1ProgramsName+".json"), filepath.Join(conformance, v1ProgramsName+".json")} {
		if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
			return err
		}
	}
	return nil
}

func recordV1Programs(n *node) (any, error) {
	version, err := n.command("version", "--long", "--output", "json")
	if err != nil {
		return nil, err
	}
	var build struct {
		Commit string `json:"commit"`
	}
	if err := json.Unmarshal(version, &build); err != nil {
		return nil, err
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
	kr, err := keyring.New(sdk.KeyringServiceName(), keyring.BackendTest, n.home, nil)
	if err != nil {
		return nil, err
	}
	secret, err := keyring.NewUnsafe(kr).UnsafeExportPrivKeyHex("fixture")
	if err != nil {
		return nil, err
	}
	key, err := crypto.HexToECDSA(secret)
	if err != nil {
		return nil, err
	}
	defer key.D.SetInt64(0)
	if n.programsAuthority == nil {
		return nil, errors.New("V1 Programs genesis authority is not initialized")
	}
	defer n.programsAuthority.evm.D.SetInt64(0)
	defer clear(n.programsAuthority.did)
	return recordV1ProgramsAuthorityRPC(n, key, n.programsAuthority.evm, n.programsAuthority.did, genesis.AppState.Sequencer.ReceiptKeys, vectorV2Provenance{Revision: build.Commit, ChainID: genesis.ChainID, Generator: "daemon/committed-fixture/vector_v1.go", Command: "committed-fixture record --vector " + v1ProgramsName})
}
