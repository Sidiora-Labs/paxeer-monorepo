package custodyproof

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	dbm "github.com/tendermint/tm-db"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/merkle"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/config"
	custodytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
)

const (
	Version        = "paxeer-custody-state-v3"
	BankVersion    = "paxeer-bank-state-v1"
	MaxInputBytes  = 128 * 1024 * 1024
	MaxHistory     = 8192
	MaxValidators  = 1000
	MaxRecordBytes = 4096
	MaxProofBytes  = 1024 * 1024
	TrustingPeriod = 24 * time.Hour
	MaxClockDrift  = time.Minute
	// ModuleDomain tags the custody module identity a profile pins where a
	// Solidity vault profile pinned its runtime code hash.
	ModuleDomain = "LX:CUSTODY:MODULE:v1"
)

type Expected struct {
	GenesisSHA256 string `json:"genesis_sha256"`
	CometChainID  string `json:"comet_chain_id"`
	ChainID       uint64 `json:"chain_id"`
	Custody       string `json:"custody"`
	ModuleSHA256  string `json:"module_sha256"`
	AssetID       string `json:"asset_id"`
	Confirmations uint64 `json:"confirmations"`
	DepositID     string `json:"deposit_id,omitempty"`
	Account       string `json:"account,omitempty"`
	Denom         string `json:"denom,omitempty"`
}

type LightBlock struct {
	Commit     coretypes.ResultCommit       `json:"commit"`
	Validators []coretypes.ResultValidators `json:"validators"`
}

type StatePoint struct {
	Height  int64               `json:"height"`
	Asset   abci.ResponseQuery  `json:"asset"`
	Deposit *abci.ResponseQuery `json:"deposit,omitempty"`
}

type BankStatePoint struct {
	Height  int64              `json:"height"`
	Balance abci.ResponseQuery `json:"balance"`
}

type Bundle struct {
	Version         string           `json:"version"`
	Genesis         []byte           `json:"genesis"`
	History         []LightBlock     `json:"history"`
	StateHeight     int64            `json:"state_height"`
	FinalizedHeight int64            `json:"finalized_height"`
	State           []StatePoint     `json:"state"`
	Bank            []BankStatePoint `json:"bank,omitempty"`
}

type Request struct {
	Operation string   `json:"operation,omitempty"`
	Expected  Expected `json:"expected"`
	Bundle    Bundle   `json:"bundle"`
}

type Result struct {
	Version             string `json:"version"`
	StateHeight         int64  `json:"state_height"`
	StateHeaderHash     string `json:"state_header_hash"`
	ApplicationRoot     string `json:"application_root"`
	FinalizedHeight     int64  `json:"finalized_height"`
	FinalizedHeaderHash string `json:"finalized_header_hash"`
	ModuleSHA256        string `json:"module_sha256"`
	ProofSHA256         string `json:"proof_sha256"`
	Denom               string `json:"denom"`
	DepositID           string `json:"deposit_id,omitempty"`
	Depositor           string `json:"depositor,omitempty"`
	Beneficiary         string `json:"beneficiary,omitempty"`
	Amount              string `json:"amount,omitempty"`
	Nonce               uint64 `json:"nonce,omitempty"`
	Account             string `json:"account,omitempty"`
}

func fixedHex(value string, length int) ([]byte, error) {
	if len(value) != length*2+2 || !strings.HasPrefix(value, "0x") {
		return nil, errors.New("hex length or prefix")
	}
	decoded, err := hex.DecodeString(value[2:])
	if err != nil || bytes.Equal(decoded, make([]byte, length)) {
		return nil, errors.New("invalid or zero identity")
	}
	return decoded, nil
}

func hexBytes(value []byte) string { return "0x" + hex.EncodeToString(value) }

func validatorSet(pages []coretypes.ResultValidators, height int64) (*types.ValidatorSet, error) {
	if len(pages) == 0 || len(pages) > (MaxValidators+99)/100 {
		return nil, errors.New("validator page bound")
	}
	var validators []*types.Validator
	seen := make(map[string]bool)
	var totalPower int64
	total := pages[0].Total
	if total <= 0 || total > MaxValidators || len(pages) != (total+99)/100 {
		return nil, errors.New("validator total bound")
	}
	for index, page := range pages {
		count := min(100, total-index*100)
		if page.BlockHeight != height || page.Total != total || page.Count != count || len(page.Validators) != count {
			return nil, errors.New("validator page identity")
		}
		for _, validator := range page.Validators {
			if validator == nil || bytes.Equal(validator.PubKey.Bytes(), make([]byte, 32)) || validator.ValidateBasic() != nil ||
				validator.VotingPower <= 0 || validator.VotingPower > types.MaxTotalVotingPower-totalPower ||
				!bytes.Equal(validator.Address, validator.PubKey.Address()) || seen[string(validator.Address)] {
				return nil, errors.New("invalid or duplicate validator")
			}
			totalPower += validator.VotingPower
			seen[string(validator.Address)] = true
			validators = append(validators, validator)
		}
	}
	return types.NewValidatorSet(validators), nil
}

func genesisValidators(genesisBytes []byte, expected Expected, now time.Time) (*types.GenesisDoc, *types.ValidatorSet, error) {
	genesisHash, err := fixedHex(expected.GenesisSHA256, 32)
	if err != nil {
		return nil, nil, err
	}
	if expected.ChainID != 125 || config.GetEVMChainID(expected.CometChainID).Uint64() != expected.ChainID ||
		len(genesisBytes) == 0 || len(genesisBytes) > 32*1024*1024 {
		return nil, nil, errors.New("genesis identity bounds")
	}
	digest := sha256.Sum256(genesisBytes)
	if !bytes.Equal(digest[:], genesisHash) {
		return nil, nil, errors.New("genesis identity")
	}
	loaded, err := types.GenesisDocFromJSON(genesisBytes)
	if err != nil {
		return nil, nil, err
	}
	genesis := *loaded
	if genesis.ChainID != expected.CometChainID || genesis.InitialHeight != 1 || genesis.GenesisTime.IsZero() ||
		genesis.GenesisTime.After(now.Add(MaxClockDrift)) || len(genesis.Validators) == 0 || len(genesis.Validators) > MaxValidators {
		return nil, nil, errors.New("genesis chain or validator identity")
	}
	validators := make([]*types.Validator, 0, len(genesis.Validators))
	for _, validator := range genesis.Validators {
		validators = append(validators, &types.Validator{Address: validator.Address,
			PubKey: validator.PubKey, VotingPower: validator.Power})
	}
	var pages []coretypes.ResultValidators
	for start := 0; start < len(validators); start += 100 {
		end := min(start+100, len(validators))
		pages = append(pages, coretypes.ResultValidators{BlockHeight: 1, Count: end - start,
			Total: len(validators), Validators: validators[start:end]})
	}
	initial, err := validatorSet(pages, 1)
	return &genesis, initial, err
}

func verifyCommit(entry *LightBlock, chainID string, now time.Time) (*types.ValidatorSet, error) {
	header := &entry.Commit.SignedHeader
	if !entry.Commit.CanonicalCommit || header.ValidateBasic(chainID) != nil || len(header.AppHash) != 32 ||
		header.Time.IsZero() || len(header.NextValidatorsHash) != 32 ||
		!header.Time.Before(now.Add(MaxClockDrift)) {
		return nil, errors.New("signed header identity")
	}
	validators, err := validatorSet(entry.Validators, header.Height)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(validators.Hash(), header.ValidatorsHash) {
		return nil, errors.New("header validator binding")
	}
	if err := validators.VerifyCommitLightAllSignatures(chainID, header.Commit.BlockID,
		header.Height, header.Commit); err != nil {
		return nil, fmt.Errorf("header quorum: %w", err)
	}
	return validators, nil
}

func verifyEntry(previous *types.SignedHeader, entry *LightBlock, genesis *types.GenesisDoc,
	initial *types.ValidatorSet, now time.Time) error {
	validators, err := verifyCommit(entry, genesis.ChainID, now)
	if err != nil {
		return err
	}
	header := &entry.Commit.SignedHeader
	if previous == nil {
		if header.Height != 1 || !bytes.Equal(initial.Hash(), validators.Hash()) ||
			!bytes.Equal(initial.Hash(), header.ValidatorsHash) || header.Time.Before(genesis.GenesisTime) {
			return errors.New("initial header genesis binding")
		}
	} else {
		if previous.ValidateBasic(genesis.ChainID) != nil || previous.Height >= int64(^uint64(0)>>1) ||
			header.Height != previous.Height+1 || !header.Time.After(previous.Time) ||
			!header.LastBlockID.Equals(previous.Commit.BlockID) ||
			!bytes.Equal(header.ValidatorsHash, previous.NextValidatorsHash) ||
			!bytes.Equal(header.ValidatorsHash, validators.Hash()) {
			return errors.New("historical header transition")
		}
	}
	return nil
}

func verifyHistory(bundle *Bundle, expected Expected, now time.Time) ([]*types.SignedHeader, error) {
	if (bundle.Version != Version && bundle.Version != BankVersion) || len(bundle.History) == 0 || len(bundle.History) > MaxHistory ||
		bundle.FinalizedHeight < 1 || bundle.FinalizedHeight == int64(^uint64(0)>>1) ||
		int64(len(bundle.History)) != bundle.FinalizedHeight+1 {
		return nil, errors.New("individual history message bound")
	}
	genesis, initial, err := genesisValidators(bundle.Genesis, expected, now)
	if err != nil {
		return nil, err
	}
	headers := make([]*types.SignedHeader, 0, len(bundle.History))
	var previous *types.SignedHeader
	for index := range bundle.History {
		entry := &bundle.History[index]
		if err := verifyEntry(previous, entry, genesis, initial, now); err != nil {
			return nil, err
		}
		previous = &entry.Commit.SignedHeader
		headers = append(headers, previous)
	}
	return headers, nil
}

func membership(response *abci.ResponseQuery, height int64, root, key []byte, maxValue int) error {
	return storeMembership(response, height, root, []byte(custodytypes.StoreKey), key, maxValue)
}

func storeMembership(response *abci.ResponseQuery, height int64, root, store, key []byte, maxValue int) error {
	if response == nil || response.Code != 0 || response.Height != height ||
		!bytes.Equal(response.Key, key) || len(response.Value) == 0 || len(response.Value) > maxValue ||
		response.ProofOps == nil || len(response.ProofOps.Ops) != 2 {
		return errors.New("state query identity or missing proof")
	}
	ops := response.ProofOps.Ops
	if ops[0].Type != storetypes.ProofOpIAVLCommitment || !bytes.Equal(ops[0].Key, key) ||
		ops[1].Type != storetypes.ProofOpSimpleMerkleCommitment || !bytes.Equal(ops[1].Key, store) {
		return errors.New("state proof store or key")
	}
	for _, op := range ops {
		if len(op.Data) == 0 || len(op.Data) > MaxProofBytes {
			return errors.New("state proof size")
		}
	}
	path := merkle.KeyPath{}.AppendKey(store, merkle.KeyEncodingURL).AppendKey(key, merkle.KeyEncodingURL)
	if err := rootmulti.DefaultProofRuntime().VerifyValue(response.ProofOps, root, path.String(), response.Value); err != nil {
		return fmt.Errorf("application-root membership: %w", err)
	}
	return nil
}

// ModuleIdentity is the value a custody profile pins for the native module:
// the custody store name and the precompile address under ModuleDomain. A
// native module has no EVM runtime to hash; what a proof authenticates is the
// module's own records under its store in the application root.
func ModuleIdentity() [32]byte {
	address, _ := custodytypes.ParseAddress(custodytypes.CustodyAddress)
	return sha256.Sum256(append(append([]byte(ModuleDomain), custodytypes.StoreKey...), address.Bytes()...))
}

func Verify(request *Request, now time.Time) (*Result, error) {
	if request == nil {
		return nil, errors.New("missing proof request")
	}
	headers, err := verifyHistory(&request.Bundle, request.Expected, now)
	if err != nil {
		return nil, err
	}
	return verifyRecords(request, now, func(height int64) (*types.SignedHeader, error) {
		if height <= 0 || height > int64(len(headers)) {
			return nil, errors.New("unverified state height")
		}
		return headers[height-1], nil
	})
}

func verifyRecords(request *Request, now time.Time, lookup func(int64) (*types.SignedHeader, error)) (*Result, error) {
	if request == nil {
		return nil, errors.New("missing proof request")
	}
	switch request.Bundle.Version {
	case BankVersion:
		if request.Operation != "verify-bank" || len(request.Bundle.State) != 0 || request.Expected.Custody != "" ||
			request.Expected.ModuleSHA256 != "" || request.Expected.AssetID != "" || request.Expected.DepositID != "" {
			return nil, errors.New("native bank proof identity")
		}
		return verifyBankState(request, now, lookup)
	case Version:
		if (request.Operation != "" && request.Operation != "verify") || len(request.Bundle.Bank) != 0 ||
			request.Expected.Account != "" || request.Expected.Denom != "" {
			return nil, errors.New("historical custody proof identity")
		}
		return verifyState(request, now, lookup)
	default:
		return nil, errors.New("unsupported proof version")
	}
}

func committedState(request *Request, now time.Time, lookup func(int64) (*types.SignedHeader, error)) (*Result, error) {
	if request == nil || request.Expected.ChainID != 125 || request.Expected.CometChainID == "" || now.IsZero() {
		return nil, errors.New("paxeer proof identity")
	}
	expected, bundle := request.Expected, &request.Bundle
	if config.GetEVMChainID(expected.CometChainID).Uint64() != expected.ChainID {
		return nil, errors.New("comet to EVM chain identity")
	}
	if bundle.StateHeight < 1 || bundle.FinalizedHeight < bundle.StateHeight ||
		bundle.FinalizedHeight == int64(^uint64(0)>>1) || expected.Confirmations == 0 || expected.Confirmations >= MaxHistory ||
		bundle.FinalizedHeight-bundle.StateHeight >= MaxHistory {
		return nil, errors.New("state finality window")
	}
	confirmations := bundle.FinalizedHeight - bundle.StateHeight + 1
	if confirmations < 1 || uint64(confirmations) < expected.Confirmations {
		return nil, errors.New("state finality window")
	}
	stateHeader, err := lookup(bundle.StateHeight + 1)
	if err != nil {
		return nil, err
	}
	finalHeader, err := lookup(bundle.FinalizedHeight + 1)
	if err != nil {
		return nil, err
	}
	if light.HeaderExpired(finalHeader, TrustingPeriod, now) || !finalHeader.Time.Before(now.Add(MaxClockDrift)) {
		return nil, errors.New("live head freshness")
	}
	return &Result{Version: bundle.Version, StateHeight: bundle.StateHeight,
		StateHeaderHash: hexBytes(stateHeader.Hash()), ApplicationRoot: hexBytes(stateHeader.AppHash),
		FinalizedHeight: bundle.FinalizedHeight, FinalizedHeaderHash: hexBytes(finalHeader.Hash())}, nil
}

// verifyState validates historical custody records. Native bank proofs use the
// same committed-header trust without requiring the retired custody module.
func verifyState(request *Request, now time.Time, lookup func(int64) (*types.SignedHeader, error)) (*Result, error) {
	result, err := committedState(request, now, lookup)
	if err != nil {
		return nil, err
	}
	expected, bundle := request.Expected, &request.Bundle
	if bundle.StateHeight < 2 {
		return nil, errors.New("historical custody state height")
	}
	custody, err := fixedHex(expected.Custody, 20)
	if err != nil {
		return nil, err
	}
	precompile, err := custodytypes.ParseAddress(custodytypes.CustodyAddress)
	if err != nil || !bytes.Equal(custody, precompile.Bytes()) {
		return nil, errors.New("custody precompile address")
	}
	moduleHash, err := fixedHex(expected.ModuleSHA256, 32)
	if err != nil {
		return nil, err
	}
	if identity := ModuleIdentity(); !bytes.Equal(moduleHash, identity[:]) {
		return nil, errors.New("custody module identity")
	}
	assetBytes, err := fixedHex(expected.AssetID, 32)
	if err != nil {
		return nil, err
	}
	var assetID, depositID [32]byte
	copy(assetID[:], assetBytes)
	var depositKey []byte
	if expected.DepositID != "" {
		identifier, err := fixedHex(expected.DepositID, 32)
		if err != nil {
			return nil, err
		}
		copy(depositID[:], identifier)
		depositKey = custodytypes.DepositKey(depositID)
	}
	points := []int64{bundle.StateHeight}
	if bundle.FinalizedHeight != bundle.StateHeight {
		points = append(points, bundle.FinalizedHeight)
	}
	if len(bundle.State) != len(points) {
		return nil, errors.New("state point count")
	}
	result.ModuleSHA256, result.DepositID = hexBytes(moduleHash), expected.DepositID
	var recorded []byte
	for index, height := range points {
		point := &bundle.State[index]
		if point.Height != height {
			return nil, errors.New("state point height")
		}
		header, err := lookup(height + 1)
		if err != nil {
			return nil, err
		}
		root := header.AppHash
		if err := membership(&point.Asset, height, root, custodytypes.AssetKey(assetID), MaxRecordBytes); err != nil {
			return nil, err
		}
		var asset custodytypes.AssetMapping
		if err := asset.Unmarshal(point.Asset.Value); err != nil || asset.Validate() != nil ||
			asset.AssetId != custodytypes.Hash32(assetID) || (result.Denom != "" && result.Denom != asset.Denom) {
			return nil, errors.New("authenticated custody asset")
		}
		result.Denom = asset.Denom
		if depositKey != nil {
			if err := membership(point.Deposit, height, root, depositKey, MaxRecordBytes); err != nil {
				return nil, err
			}
			var deposit custodytypes.Deposit
			if err := deposit.Unmarshal(point.Deposit.Value); err != nil || deposit.Validate() != nil ||
				deposit.DepositId != custodytypes.Hash32(depositID) || deposit.AssetId != asset.AssetId ||
				deposit.Denom != asset.Denom || deposit.Height <= 0 || deposit.Height > bundle.StateHeight ||
				(recorded != nil && !bytes.Equal(recorded, point.Deposit.Value)) {
				return nil, errors.New("authenticated custody deposit")
			}
			depositor, err := custodytypes.ParseAddress(deposit.Depositor)
			if err != nil {
				return nil, err
			}
			beneficiary, err := custodytypes.ParseNonZeroHash32(deposit.Beneficiary)
			if err != nil {
				return nil, err
			}
			amount, err := custodytypes.ParseAmount(deposit.Amount)
			if err != nil {
				return nil, err
			}
			if custodytypes.DepositID(new(big.Int).SetUint64(expected.ChainID), depositor, assetID, beneficiary,
				amount.BigInt(), deposit.Nonce) != depositID {
				return nil, errors.New("deposit identifier preimage")
			}
			recorded = point.Deposit.Value
			result.Depositor, result.Beneficiary = hexBytes(depositor.Bytes()), hexBytes(beneficiary[:])
			result.Amount, result.Nonce = amount.String(), deposit.Nonce
		} else if point.Deposit != nil {
			return nil, errors.New("unexpected deposit proof")
		}
	}
	return proofResult(result, bundle)
}

func verifyBankState(request *Request, now time.Time, lookup func(int64) (*types.SignedHeader, error)) (*Result, error) {
	result, err := committedState(request, now, lookup)
	if err != nil {
		return nil, err
	}
	expected, bundle := request.Expected, &request.Bundle
	account, err := fixedHex(expected.Account, 20)
	if err != nil {
		return nil, err
	}
	if err := sdk.ValidateDenom(expected.Denom); err != nil {
		return nil, err
	}
	points := []int64{bundle.StateHeight}
	if bundle.FinalizedHeight != bundle.StateHeight {
		points = append(points, bundle.FinalizedHeight)
	}
	if len(bundle.Bank) != len(points) {
		return nil, errors.New("bank state point count")
	}
	key := banktypes.CreatePrefixedAccountStoreKey(account, []byte(expected.Denom))
	for index, height := range points {
		point := &bundle.Bank[index]
		if point.Height != height {
			return nil, errors.New("bank state point height")
		}
		header, err := lookup(height + 1)
		if err != nil {
			return nil, err
		}
		if err := storeMembership(&point.Balance, height, header.AppHash, []byte(banktypes.StoreKey), key, MaxRecordBytes); err != nil {
			return nil, err
		}
		var balance sdk.Coin
		if err := balance.Unmarshal(point.Balance.Value); err != nil || balance.Amount.IsNil() ||
			balance.Validate() != nil || !balance.Amount.IsPositive() || balance.Denom != expected.Denom {
			return nil, errors.New("authenticated bank balance")
		}
		if index == 0 {
			result.Amount = balance.Amount.String()
		}
	}
	result.Account, result.Denom = hexBytes(account), expected.Denom
	return proofResult(result, bundle)
}

func proofResult(result *Result, bundle *Bundle) (*Result, error) {
	canonical, err := json.Marshal(bundle)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(canonical)
	result.ProofSHA256 = hexBytes(digest[:])
	return result, nil
}

// PinnedCheckpoint is operator-provisioned trust, never part of an untrusted
// Request. HeaderSHA256 pins the Comet header hash, including its validator set
// and the next validator set authorized to extend it.
type PinnedCheckpoint struct {
	HeaderSHA256 string     `json:"header_sha256"`
	Block        LightBlock `json:"block"`
}

type trustNetwork struct {
	GenesisSHA256 string `json:"genesis_sha256"`
	CometChainID  string `json:"comet_chain_id"`
	ChainID       uint64 `json:"chain_id"`
}

func networkOf(expected Expected) trustNetwork {
	return trustNetwork{expected.GenesisSHA256, expected.CometChainID, expected.ChainID}
}

type trustState struct {
	Version   string       `json:"version"`
	Network   trustNetwork `json:"network"`
	PinHash   string       `json:"pin_hash"`
	PinHeight int64        `json:"pin_height"`
	History   []LightBlock `json:"history"`
}

const trustVersion = "paxeer-committed-trust-v1"

// Verifier owns a dedicated local DB and must be closed before reopening it.
// The DB is trusted local state; remote proof inputs cannot replace its pin or
// rewind its verified head. Each update is one synchronous, atomic DB write.
type Verifier struct {
	mu     sync.Mutex
	db     dbm.DB
	state  trustState
	closed bool
}

func OpenVerifier(db dbm.DB, expected Expected, checkpoint PinnedCheckpoint, now time.Time) (*Verifier, error) {
	if db == nil || now.IsZero() || expected.ChainID != 125 ||
		config.GetEVMChainID(expected.CometChainID).Uint64() != expected.ChainID {
		return nil, errors.New("checkpoint network identity")
	}
	if _, err := fixedHex(expected.GenesisSHA256, 32); err != nil {
		return nil, err
	}
	pin, err := fixedHex(checkpoint.HeaderSHA256, 32)
	if err != nil {
		return nil, err
	}
	if _, err := verifyCommit(&checkpoint.Block, expected.CometChainID, now); err != nil {
		return nil, err
	}
	if !bytes.Equal(pin, checkpoint.Block.Commit.Hash()) {
		return nil, errors.New("checkpoint header pin")
	}
	v := &Verifier{db: db, state: trustState{Version: trustVersion, Network: networkOf(expected),
		PinHash: checkpoint.HeaderSHA256, PinHeight: checkpoint.Block.Commit.Height}}
	encoded, err := db.Get([]byte(trustVersion))
	if err != nil {
		return nil, err
	}
	if encoded == nil {
		if err := liveHead(&checkpoint.Block.Commit.SignedHeader, now); err != nil {
			return nil, err
		}
		if err := v.persist([]LightBlock{checkpoint.Block}); err != nil {
			return nil, err
		}
		return v, nil
	}
	if len(encoded) > MaxInputBytes {
		return nil, errors.New("stored trust size")
	}
	var stored trustState
	if err := json.Unmarshal(encoded, &stored); err != nil {
		return nil, err
	}
	if stored.Version != trustVersion || stored.Network != v.state.Network || stored.PinHash != v.state.PinHash ||
		stored.PinHeight != v.state.PinHeight || len(stored.History) == 0 || len(stored.History) > MaxHistory {
		return nil, errors.New("stored checkpoint conflict")
	}
	for index := range stored.History {
		entry := &stored.History[index]
		if index == 0 {
			if _, err := verifyCommit(entry, stored.Network.CometChainID, now); err != nil {
				return nil, err
			}
			if entry.Commit.Height < stored.PinHeight ||
				(entry.Commit.Height == stored.PinHeight && !bytes.Equal(entry.Commit.Hash(), pin)) {
				return nil, errors.New("stored checkpoint rollback or conflict")
			}
		} else if err := verifyEntry(&stored.History[index-1].Commit.SignedHeader, entry,
			&types.GenesisDoc{ChainID: stored.Network.CometChainID}, nil, now); err != nil {
			return nil, err
		}
	}
	if err := liveHead(&stored.History[len(stored.History)-1].Commit.SignedHeader, now); err != nil {
		return nil, err
	}
	v.state = stored
	return v, nil
}

func liveHead(header *types.SignedHeader, now time.Time) error {
	if now.IsZero() || light.HeaderExpired(header, TrustingPeriod, now) || !header.Time.Before(now.Add(MaxClockDrift)) {
		return errors.New("trusted checkpoint expired or in future")
	}
	return nil
}

func (v *Verifier) extend(history []LightBlock, now time.Time) ([]LightBlock, error) {
	if v.closed {
		return nil, errors.New("verifier closed")
	}
	if len(history) > MaxHistory {
		return nil, errors.New("checkpoint extension bound")
	}
	if err := liveHead(&v.state.History[len(v.state.History)-1].Commit.SignedHeader, now); err != nil {
		return nil, err
	}
	// Detach incoming data from the caller before validating or retaining it.
	encoded, err := json.Marshal(history)
	if err != nil {
		return nil, err
	}
	if len(encoded) > MaxInputBytes {
		return nil, errors.New("checkpoint extension size")
	}
	var incoming []LightBlock
	if err := json.Unmarshal(encoded, &incoming); err != nil {
		return nil, err
	}
	candidate := append([]LightBlock(nil), v.state.History...)
	first := candidate[0].Commit.Height
	head := candidate[len(candidate)-1].Commit.Height
	var prior int64
	for index := range incoming {
		entry := &incoming[index]
		if _, err := verifyCommit(entry, v.state.Network.CometChainID, now); err != nil {
			return nil, err
		}
		height := entry.Commit.Height
		if index > 0 && (prior == int64(^uint64(0)>>1) || height != prior+1) {
			return nil, errors.New("noncontiguous checkpoint extension")
		}
		prior = height
		if height < first {
			return nil, errors.New("checkpoint extension precedes retained trust")
		}
		last := &candidate[len(candidate)-1].Commit.SignedHeader
		if height <= last.Height {
			if !bytes.Equal(entry.Commit.Hash(), candidate[height-first].Commit.Hash()) {
				return nil, errors.New("conflicting committed header")
			}
			continue
		}
		if err := verifyEntry(last, entry, &types.GenesisDoc{ChainID: v.state.Network.CometChainID}, nil, now); err != nil {
			return nil, err
		}
		candidate = append(candidate, *entry)
	}
	if len(incoming) > 0 && prior < head {
		return nil, errors.New("checkpoint extension rewinds trusted head")
	}
	return candidate, nil
}

func (v *Verifier) persist(history []LightBlock) error {
	if len(history) > MaxHistory {
		history = history[len(history)-MaxHistory:]
	}
	state := v.state
	state.History = history
	encoded, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if len(encoded) > MaxInputBytes {
		return errors.New("stored trust size")
	}
	// The initial checkpoint may still be owned by the caller.
	var detached trustState
	if err := json.Unmarshal(encoded, &detached); err != nil {
		return err
	}
	if err := v.db.SetSync([]byte(trustVersion), encoded); err != nil {
		return err
	}
	v.state = detached
	return nil
}

// VerifyHeaders verifies a bounded adjacent extension and durably records its
// validator transitions independently of any bank or historical custody record.
func (v *Verifier) VerifyHeaders(history []LightBlock, now time.Time) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if len(history) == 0 {
		return errors.New("missing checkpoint extension")
	}
	candidate, err := v.extend(history, now)
	if err != nil {
		return err
	}
	return v.persist(candidate)
}

// Verify consumes the same decoded request as the stateless genesis verifier,
// using the local checkpoint instead of requiring replay from height one.
func (v *Verifier) Verify(request *Request, now time.Time) (*Result, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if request == nil || networkOf(request.Expected) != v.state.Network {
		return nil, errors.New("proof checkpoint network conflict")
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	request, err = Decode(encoded)
	if err != nil {
		return nil, err
	}
	if len(request.Bundle.Genesis) > 0 {
		digest := sha256.Sum256(request.Bundle.Genesis)
		pin, err := fixedHex(v.state.Network.GenesisSHA256, 32)
		if err != nil || !bytes.Equal(digest[:], pin) {
			return nil, errors.New("proof genesis checkpoint conflict")
		}
	}
	candidate, err := v.extend(request.Bundle.History, now)
	if err != nil {
		return nil, err
	}
	first := candidate[0].Commit.Height
	result, err := verifyRecords(request, now, func(height int64) (*types.SignedHeader, error) {
		if height < first || height-first >= int64(len(candidate)) {
			return nil, errors.New("unverified checkpoint state height")
		}
		return &candidate[height-first].Commit.SignedHeader, nil
	})
	if err != nil {
		return nil, err
	}
	if err := v.persist(candidate); err != nil {
		return nil, err
	}
	return result, nil
}

func (v *Verifier) TrustedHeader(height int64) (*types.SignedHeader, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.closed {
		return nil, errors.New("verifier closed")
	}
	first := v.state.History[0].Commit.Height
	if height < first || height-first >= int64(len(v.state.History)) {
		return nil, errors.New("unverified checkpoint header")
	}
	encoded, err := json.Marshal(v.state.History[height-first].Commit.SignedHeader)
	if err != nil {
		return nil, err
	}
	var header types.SignedHeader
	if err := json.Unmarshal(encoded, &header); err != nil {
		return nil, err
	}
	return &header, nil
}

func (v *Verifier) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.closed {
		return errors.New("verifier already closed")
	}
	v.closed = true
	return v.db.Close()
}
