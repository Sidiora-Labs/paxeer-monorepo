package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/addr"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

const vectorV2Name = "vector-v2-receipted-revert"
const vectorV2CaseID = "V2-layerxledger-revert"
const vectorV2SourcePath = "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
const vectorV2SourceRef = vectorV2SourcePath + "#" + vectorV2CaseID
const vectorV2Code = "LXP_INSUFFICIENT_BALANCE"

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
	return recordV2FromHome(n, version.Commit)
}

func recordV2FromHome(n *node, revision string) (any, error) {
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
	var primaryKeys seqkeeper.ReceiptKeys
	for _, pin := range keys {
		if bytes.Equal(pin.PublicKey, n.receiptPublicKey) {
			primaryKeys = append(primaryKeys, pin)
		}
	}
	if len(primaryKeys) != 1 {
		return nil, errors.New("V2 requires exactly one genesis pin for this home's receipt key")
	}
	kr, err := keyring.New(sdk.KeyringServiceName(), keyring.BackendTest, n.home, strings.NewReader(""))
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
	return recordV2RPC(n, key, keys, vectorV2Provenance{
		Revision: revision, ChainID: genesis.ChainID,
		Generator: "daemon/committed-fixture/vector_v2.go",
		Command:   "committed-fixture record --paxd paxd --out daemon/committed-fixture/vectors --vector " + vectorV2Name,
		Source:    vectorV2SourceRef,
	})
}

type v2Call struct {
	key   *ecdsa.PrivateKey
	to    common.Address
	value *big.Int
	data  []byte
}

func v2LedgerRecipe(chainID *big.Int, payer, payee *ecdsa.PrivateKey, expiresAt uint64) ([]v2Call, []byte, error) {
	ledger := layerxledger.GetABI()
	addressABI, err := abi.JSON(strings.NewReader(`[{"type":"function","name":"bindLayerX","inputs":[{"name":"didPublicKey","type":"bytes32"},{"name":"signature","type":"bytes"}],"outputs":[]}]`))
	if err != nil {
		return nil, nil, err
	}
	keys := []*ecdsa.PrivateKey{payer, payee}
	var public [2][32]byte
	var calls []v2Call
	// Fund the payee's EVM gas without crediting either kernel ledger account.
	calls = append(calls, v2Call{payer, crypto.PubkeyToAddress(payee.PublicKey), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil), nil})
	for i, key := range keys {
		seed := sha256.Sum256(append(key.D.FillBytes(make([]byte, 32)), []byte("V2 ledger DID")...))
		private := ed25519.NewKeyFromSeed(seed[:])
		copy(public[i][:], private.Public().(ed25519.PublicKey))
		message, err := evmtypes.LayerXBindMessage(chainID, crypto.PubkeyToAddress(key.PublicKey), 0)
		if err != nil {
			clear(private)
			return nil, nil, err
		}
		input, err := addressABI.Pack(addr.BindLayerXMethod, public[i], ed25519.Sign(private, message))
		clear(private)
		clear(seed[:])
		if err != nil {
			return nil, nil, err
		}
		calls = append(calls, v2Call{key, common.HexToAddress(addr.AddrAddress), new(big.Int), input})
	}
	actor, err := lxphost.DIDID([]byte(evmtypes.LayerXDid(public[0])))
	if err != nil {
		return nil, nil, err
	}
	salt := sha256.Sum256([]byte("V2 insufficient balance asset"))
	asset := sha256.Sum256(append(append([]byte("LX:ASSET:v1"), actor[:]...), salt[:]...))
	input, err := ledger.Pack("register", asset, salt, "V2", "V2 unfunded ledger asset", uint8(6), new(big.Int).Lsh(big.NewInt(1), 110), uint8(1), []byte{})
	if err != nil {
		return nil, nil, err
	}
	to := common.HexToAddress(layerxledger.LedgerAddress)
	calls = append(calls, v2Call{payer, to, new(big.Int), input})
	for _, key := range keys {
		input, err := ledger.Pack("openAccount", asset)
		if err != nil {
			return nil, nil, err
		}
		calls = append(calls, v2Call{key, to, new(big.Int), input})
	}
	from, err := ledgerkeeper.AccountID(public[0], asset)
	if err != nil {
		return nil, nil, err
	}
	recipient, err := ledgerkeeper.AccountID(public[1], asset)
	if err != nil {
		return nil, nil, err
	}
	reference := crypto.Keccak256Hash([]byte("vector unfunded payment reference"))
	input, err = ledger.Pack("payExact", from, recipient, asset, new(big.Int).Lsh(big.NewInt(1), 100), expiresAt, [32]byte(reference))
	return calls, input, err
}

func recordV2RPC(n *node, payer *ecdsa.PrivateKey, keys seqkeeper.ReceiptKeys, provenance vectorV2Provenance) (*vectorV2, error) {
	ctx, cancel := context.WithTimeout(n.ctx, 30*time.Second)
	defer cancel()
	var chainID, gasPrice hexutil.Big
	if err := n.client.CallContext(ctx, &chainID, "eth_chainId"); err != nil {
		return nil, err
	}
	if err := n.client.CallContext(ctx, &gasPrice, "eth_gasPrice"); err != nil {
		return nil, err
	}
	payee, err := crypto.GenerateKey()
	if err != nil {
		return nil, err
	}
	defer payee.D.SetInt64(0)
	setup, data, err := v2LedgerRecipe(chainID.ToInt(), payer, payee, uint64(time.Now().Add(time.Hour).UnixMilli()))
	if err != nil {
		return nil, err
	}
	for i, call := range setup {
		_, receipt, err := sendV2Call(ctx, n, chainID.ToInt(), gasPrice.ToInt(), call)
		if err != nil {
			return nil, fmt.Errorf("V2 setup call %d: %w", i, err)
		}
		if err := receipt.Verify(keys); err != nil {
			return nil, fmt.Errorf("V2 setup call %d authorization: %w", i, err)
		}
		if receipt.Status != 1 {
			return nil, fmt.Errorf("V2 setup call %d reverted: %x", i, receipt.ReturnData)
		}
	}
	raw, result, err := sendV2Call(ctx, n, chainID.ToInt(), gasPrice.ToInt(), v2Call{
		payer, common.HexToAddress(layerxledger.LedgerAddress), new(big.Int), data,
	})
	if err != nil {
		return nil, err
	}
	reason, err := abi.UnpackRevert(result.ReturnData)
	if err != nil {
		return nil, fmt.Errorf("decode V2 Error(string): %w", err)
	}
	provenance.FirstHeight, provenance.LastHeight = uint64(result.Height), uint64(result.Height)
	provenance.Source = vectorV2SourceRef
	v := &vectorV2{Name: vectorV2Name, Provenance: provenance, ChainID: &chainID, RawTransaction: raw,
		ReceiptKeys: keys, Receipt: *result, LXPCode: strings.SplitN(reason, ":", 2)[0], RevertReason: reason}
	source, err := canonicalV2(v)
	if err != nil {
		return nil, err
	}
	return vectorV2FromSource(source)
}

func sendV2Call(ctx context.Context, n *node, chainID, gasPrice *big.Int, call v2Call) ([]byte, *evmrpc.SequencerReceiptResult, error) {
	sender := crypto.PubkeyToAddress(call.key.PublicKey)
	var nonce hexutil.Uint64
	if err := n.client.CallContext(ctx, &nonce, "eth_getTransactionCount", sender, "pending"); err != nil {
		return nil, nil, err
	}
	tx, err := ethtypes.SignNewTx(call.key, ethtypes.LatestSignerForChainID(chainID), &ethtypes.DynamicFeeTx{
		ChainID: chainID, Nonce: uint64(nonce), GasTipCap: new(big.Int), GasFeeCap: gasPrice,
		Gas: 2000000, To: &call.to, Value: call.value, Data: call.data,
	})
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
		return nil, nil, errors.New("eth_sendRawTransaction returned another transaction hash")
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var result *evmrpc.SequencerReceiptResult
		if err := n.client.CallContext(ctx, &result, "pax_sequencerReceipt", hash); err != nil {
			return nil, nil, err
		}
		if result != nil && result.Rung != evmrpc.RungSubmitted {
			return raw, result, nil
		}
		select {
		case <-ctx.Done():
			return nil, nil, fmt.Errorf("wait for V2 signed receipt: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func verifyVectorV2(v *vectorV2) error {
	if v.Name != vectorV2Name || v.ChainID == nil || (*big.Int)(v.ChainID).Sign() <= 0 {
		return errors.New("invalid V2 name or chain id")
	}
	if v.Provenance.Revision == "" || v.Provenance.ChainID == "" || v.Provenance.Generator == "" || v.Provenance.Command == "" || v.Provenance.Source != vectorV2SourceRef {
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
	if sender != r.From || tx.To() == nil || *tx.To() != common.HexToAddress(layerxledger.LedgerAddress) || *tx.To() != r.To || len(tx.Data()) != 4+6*32 || tx.Type() != ethtypes.DynamicFeeTxType || tx.Value().Sign() != 0 || !bytes.Equal(tx.Data()[:4], layerxledger.GetABI().Methods["payExact"].ID) {
		return errors.New("V2 transaction is not the recorded ledger precompile call")
	}
	reason, err := abi.UnpackRevert(r.ReturnData)
	if err != nil {
		return fmt.Errorf("V2 return data is not Error(string): %w", err)
	}
	if v.LXPCode != vectorV2Code || strings.SplitN(reason, ":", 2)[0] != v.LXPCode || reason != v.RevertReason {
		return errors.New("V2 revert data does not preserve the recorded LXP code and reason")
	}
	return nil
}

type v2Pin struct {
	KeyID       common.Hash   `json:"keyId"`
	PublicKey   hexutil.Bytes `json:"publicKey"`
	FirstHeight uint64        `json:"firstHeight"`
	LastHeight  uint64        `json:"lastHeight"`
}

type v2SourceCase struct {
	ID             string                        `json:"id"`
	Vector         string                        `json:"vector"`
	Description    string                        `json:"description"`
	RawTransaction hexutil.Bytes                 `json:"rawTransaction"`
	Receipt        evmrpc.SequencerReceiptResult `json:"receiptJson"`
	SignedReceipt  hexutil.Bytes                 `json:"signedReceipt"`
	ReceiptDigest  common.Hash                   `json:"receiptDigest"`
	PrevSeqHash    common.Hash                   `json:"prevSeqHash"`
	PinnedKey      v2Pin                         `json:"pinnedKey"`
	LXPCode        string                        `json:"lxpCode"`
	Expected       struct {
		Rung     string `json:"rung"`
		Verifier string `json:"verifier"`
	} `json:"expected"`
}

type v2Source struct {
	ReceiptKeys seqkeeper.ReceiptKeys `json:"receiptKeys,omitempty"`
	Name        string                `json:"name"`
	Provenance  vectorV2Provenance    `json:"provenance"`
	ChainID     uint64                `json:"chainId"`
	ModuleID    uint16                `json:"moduleId"`
	FeeDenom    string                `json:"feeDenom"`
	Cases       []v2SourceCase        `json:"cases"`
}

func canonicalV2(v *vectorV2) (*v2Source, error) {
	if err := verifyVectorV2(v); err != nil {
		return nil, err
	}
	if !v.ChainID.ToInt().IsUint64() {
		return nil, errors.New("V2 source requires a uint64 chain ID")
	}
	verified, err := verify.ReceiptSignature(v.Receipt.Receipt, [32]byte(v.Receipt.SigningKey))
	if err != nil {
		return nil, err
	}
	var pin seqkeeper.ReceiptKeyEntry
	for _, candidate := range v.ReceiptKeys {
		if bytes.Equal(candidate.KeyId, v.Receipt.KeyID[:]) && bytes.Equal(candidate.PublicKey, v.Receipt.SigningKey) {
			pin = candidate
		}
	}
	if len(pin.PublicKey) != 32 {
		return nil, errors.New("V2 signing key is absent from the validated authorization set")
	}
	c := v2SourceCase{ID: vectorV2CaseID, Vector: "V2", Description: "layerxledger payExact reverted with Error(string) LXP_INSUFFICIENT_BALANCE",
		RawTransaction: bytes.Clone(v.RawTransaction), Receipt: v.Receipt, SignedReceipt: bytes.Clone(v.Receipt.Receipt),
		ReceiptDigest: common.Hash(verified.Digest), PrevSeqHash: common.Hash(verified.Receipt.PreviousStateRoot),
		PinnedKey: v2Pin{common.BytesToHash(pin.KeyId), bytes.Clone(pin.PublicKey), pin.FirstHeight, pin.LastHeight}, LXPCode: v.LXPCode}
	c.Expected.Rung, c.Expected.Verifier = string(v.Receipt.Rung), "accepts"
	var allPins seqkeeper.ReceiptKeys
	if len(v.ReceiptKeys) > 1 {
		allPins = v.ReceiptKeys
	}
	return &v2Source{ReceiptKeys: allPins, Name: "sequencer-receipt-v1", Provenance: v.Provenance, ChainID: v.ChainID.ToInt().Uint64(),
		ModuleID: verified.Receipt.ModuleID, FeeDenom: v.Receipt.FeeDenom, Cases: []v2SourceCase{c}}, nil
}

func vectorV2FromSource(source *v2Source) (*vectorV2, error) {
	if source.Name != "sequencer-receipt-v1" || len(source.Cases) != 1 {
		return nil, errors.New("expected the canonical V2 source group")
	}
	c := source.Cases[0]
	if c.ID != vectorV2CaseID || c.Vector != "V2" || c.Expected.Verifier != "accepts" || c.Expected.Rung != string(c.Receipt.Rung) ||
		!bytes.Equal(c.SignedReceipt, c.Receipt.Receipt) || len(c.PinnedKey.PublicKey) != 32 {
		return nil, errors.New("invalid V2 canonical case")
	}
	reason, err := abi.UnpackRevert(c.Receipt.ReturnData)
	if err != nil {
		return nil, err
	}
	chainID := (*hexutil.Big)(new(big.Int).SetUint64(source.ChainID))
	v := &vectorV2{Name: vectorV2Name, Provenance: source.Provenance, ChainID: chainID, RawTransaction: bytes.Clone(c.RawTransaction),
		ReceiptKeys: seqkeeper.ReceiptKeys{{KeyId: bytes.Clone(c.PinnedKey.KeyID[:]), PublicKey: bytes.Clone(c.PinnedKey.PublicKey), FirstHeight: c.PinnedKey.FirstHeight, LastHeight: c.PinnedKey.LastHeight}},
		Receipt:     c.Receipt, LXPCode: c.LXPCode, RevertReason: reason}
	if len(source.ReceiptKeys) > 0 {
		v.ReceiptKeys = source.ReceiptKeys
		if err := v.ReceiptKeys.Validate(); err != nil {
			return nil, err
		}
		var matches int
		for _, pin := range v.ReceiptKeys {
			if bytes.Equal(pin.KeyId, c.PinnedKey.KeyID[:]) && bytes.Equal(pin.PublicKey, c.PinnedKey.PublicKey) && pin.FirstHeight == c.PinnedKey.FirstHeight && pin.LastHeight == c.PinnedKey.LastHeight {
				matches++
			}
		}
		if matches != 1 {
			return nil, errors.New("V2 canonical signing pin differs from its authorization set")
		}
	}
	if err := verifyVectorV2(v); err != nil {
		return nil, err
	}
	verified, err := verify.ReceiptSignature(c.SignedReceipt, [32]byte(c.Receipt.SigningKey))
	if err != nil {
		return nil, err
	}
	if c.ReceiptDigest != common.Hash(verified.Digest) || c.PrevSeqHash != common.Hash(verified.Receipt.PreviousStateRoot) ||
		source.ModuleID != verified.Receipt.ModuleID || source.FeeDenom != c.Receipt.FeeDenom {
		return nil, errors.New("V2 source differs from authenticated receipt")
	}
	return v, nil
}

func writeVectorV2Copies(out string, v *vectorV2) error {
	source, err := canonicalV2(v)
	if err != nil {
		return err
	}
	// Serialize the canonical source first; every copy consumes those exact bytes.
	encoded, err := json.MarshalIndent(source, "", "  ")
	if err != nil {
		return err
	}
	var transported v2Source
	if err := json.Unmarshal(encoded, &transported); err != nil {
		return err
	}
	copy, err := vectorV2FromSource(&transported)
	if err != nil {
		return err
	}
	group := struct {
		ReceiptKeys seqkeeper.ReceiptKeys `json:"receiptKeys,omitempty"`
		Name        string                `json:"name"`
		Provenance  vectorV2Provenance    `json:"provenance"`
		ChainID     uint64                `json:"chainId"`
		Cases       []v2SourceCase        `json:"cases"`
	}{transported.ReceiptKeys, vectorV2Name, transported.Provenance, transported.ChainID, transported.Cases}
	conformance := filepath.Join(out, "conformance")
	if err := os.MkdirAll(conformance, 0755); err != nil {
		return err
	}
	artifacts := []struct {
		path  string
		value any
	}{
		{filepath.Join(conformance, "sequencer-receipt-v1.json"), &transported},
		{filepath.Join(conformance, vectorV2Name+".json"), group},
		{filepath.Join(out, vectorV2Name+".json"), copy},
	}
	for _, artifact := range artifacts {
		data, err := json.MarshalIndent(artifact.value, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(artifact.path, append(data, '\n'), 0644); err != nil {
			return err
		}
	}
	return nil
}

func (n *node) configureV2Genesis() error {
	if n.bootstrapped || !n.standbyPrepared || n.standby == nil {
		return errors.New("V2 merge genesis requires the prepared primary and standby before bootstrap")
	}
	var doc map[string]json.RawMessage
	if err := readJSON(filepath.Join(n.home, "config", "genesis.json"), &doc); err != nil {
		return err
	}
	var state map[string]json.RawMessage
	if err := json.Unmarshal(doc["app_state"], &state); err != nil {
		return err
	}
	var sequencer map[string]json.RawMessage
	if err := json.Unmarshal(state["sequencer"], &sequencer); err != nil {
		return err
	}
	sequencer["activation_height"] = json.RawMessage("3")
	info, err := json.Marshal(sequencer)
	if err != nil {
		return err
	}
	state["sequencer"] = info
	encoding := app.MakeEncodingConfig()
	var gov govtypes.GenesisState
	if err := encoding.Marshaler.UnmarshalAsJSON(state[govtypes.ModuleName], &gov); err != nil {
		return err
	}
	if len(gov.Proposals) != 0 || len(gov.Votes) != 0 || len(gov.Deposits) != 0 {
		return errors.New("V2 requires a fresh private governance genesis")
	}
	content := upgradetypes.NewSoftwareUpgradeProposal("Activate the private V2 fixture merge", "Run the real merge handler before recording ledger receipts", upgradetypes.Plan{Name: app.MergeUpgrade, Height: 2, Info: string(info)})
	if err := content.ValidateBasic(); err != nil {
		return err
	}
	// The genesis vote is tallied at block 1 under the normal quorum rules;
	// the upgrade handler then runs at 2 and sequenced execution begins at 3.
	at := time.Unix(1, 0).UTC()
	proposal, err := govtypes.NewProposal(content, gov.StartingProposalId, at, at, false)
	if err != nil {
		return err
	}
	proposal.Status = govtypes.StatusVotingPeriod
	proposal.VotingStartTime, proposal.VotingEndTime = at, at
	kr, err := keyring.New(sdk.KeyringServiceName(), keyring.BackendTest, n.home, nil)
	if err != nil {
		return err
	}
	voter, err := kr.Key("fixture")
	if err != nil {
		return err
	}
	gov.Proposals = append(gov.Proposals, proposal)
	gov.Votes = append(gov.Votes, govtypes.NewVote(proposal.ProposalId, voter.GetAddress(), govtypes.NewNonSplitVoteOption(govtypes.OptionYes)))
	gov.StartingProposalId++
	state[govtypes.ModuleName], err = encoding.Marshaler.MarshalAsJSON(&gov)
	if err != nil {
		return err
	}
	doc["app_state"], err = json.Marshal(state)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	for _, peer := range []*node{n, n.standby} {
		if err := os.WriteFile(filepath.Join(peer.home, "config", "genesis.json"), append(data, '\n'), 0600); err != nil {
			return err
		}
	}
	n.bootstrapHeight = 2
	return nil
}
