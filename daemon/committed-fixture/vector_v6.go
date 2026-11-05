package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
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
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/addr"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
)

const vectorV6Name = "vector-v6-sponsored-bind"

var v6Token = common.HexToAddress("0x21f7b20a555199fa73A238B1a91FD0f549068fEe")

//go:embed v6_contracts.json
var v6ContractsJSON []byte

type v6Contract struct {
	Source              string          `json:"source"`
	SourceHash          string          `json:"sourceSha256"`
	ABI                 json.RawMessage `json:"abi"`
	CreationCode        hexutil.Bytes   `json:"creationCode"`
	RuntimeCode         hexutil.Bytes   `json:"runtimeCode"`
	ImmutableReferences map[string][]struct {
		Start  int `json:"start"`
		Length int `json:"length"`
	} `json:"immutableReferences"`
}
type v6Contracts struct {
	Compiler string          `json:"compiler"`
	Settings json.RawMessage `json:"settings"`
	Delegate v6Contract      `json:"delegate"`
	Token    v6Contract      `json:"token"`
}

func v6LoadContracts() (*v6Contracts, abi.ABI, abi.ABI, error) {
	var c v6Contracts
	if err := json.Unmarshal(v6ContractsJSON, &c); err != nil {
		return nil, abi.ABI{}, abi.ABI{}, err
	}
	d, err := abi.JSON(bytes.NewReader(c.Delegate.ABI))
	if err != nil {
		return nil, d, abi.ABI{}, err
	}
	t, err := abi.JSON(bytes.NewReader(c.Token.ABI))
	return &c, d, t, err
}

type v6Call struct {
	To    common.Address
	Value *big.Int
	Data  []byte
}
type v6Quote struct {
	Sponsor        common.Address
	Token          common.Address
	MaxTokenAmount *big.Int
	TokenAmount    *big.Int
	Deadline       *big.Int
	QuoteNonce     *big.Int
	GasCost        *big.Int
}
type v6Case struct {
	ID                   string                         `json:"id"`
	RawTransaction       hexutil.Bytes                  `json:"rawTransaction"`
	Receipt              *evmrpc.SequencerReceiptResult `json:"receiptJson"`
	SignedReceipt        hexutil.Bytes                  `json:"signedReceipt"`
	ReceiptDigest        common.Hash                    `json:"receiptDigest"`
	SponsorPaidFee       *hexutil.Big                   `json:"sponsorPaidFee"`
	SponsorBalanceBefore *hexutil.Big                   `json:"sponsorBalanceBefore"`
	SponsorBalanceAfter  *hexutil.Big                   `json:"sponsorBalanceAfter"`
	UserBalanceBefore    *hexutil.Big                   `json:"userBalanceBefore"`
	UserBalanceAfter     *hexutil.Big                   `json:"userBalanceAfter"`
	DelegationDesignator hexutil.Bytes                  `json:"delegationDesignator"`
}
type v6Vector struct {
	Name               string                    `json:"name"`
	Provenance         vectorV2Provenance        `json:"provenance"`
	ChainID            uint64                    `json:"chainId"`
	PinnedKey          seqkeeper.ReceiptKeyEntry `json:"pinnedKey"`
	ReceiptKeys        seqkeeper.ReceiptKeys     `json:"receiptKeys"`
	Sponsor            common.Address            `json:"sponsor"`
	User               common.Address            `json:"user"`
	Delegate           common.Address            `json:"delegate"`
	DelegateCode       hexutil.Bytes             `json:"delegateCode"`
	DelegateDeployment v6Case                    `json:"delegateDeployment"`
	DIDPublicKey       hexutil.Bytes             `json:"didPublicKey"`
	BindCalldata       hexutil.Bytes             `json:"bindCalldata"`
	BoundDID           string                    `json:"boundDid"`
	Positive           v6Case                    `json:"positive"`
	Refusal            v6Case                    `json:"refusal"`
}

func init() { registerVector(vectorV6Name, recordV6) }

// configureV6Genesis installs the real benchmark ERC20 implementation at the
// protocol's token address on this disposable chain. Mint and repayment execute
// that implementation; neither token results nor account delegation are seeded.
func (n *node) configureV6Genesis() error {
	c, _, _, err := v6LoadContracts()
	if err != nil {
		return err
	}
	path := filepath.Join(n.home, "config", "genesis.json")
	var doc map[string]json.RawMessage
	if err := readJSON(path, &doc); err != nil {
		return err
	}
	var state map[string]json.RawMessage
	if err := json.Unmarshal(doc["app_state"], &state); err != nil {
		return err
	}
	var evm map[string]json.RawMessage
	if err := json.Unmarshal(state["evm"], &evm); err != nil {
		return err
	}
	var codes []*evmtypes.Code
	if raw, exists := evm["codes"]; exists {
		if err := json.Unmarshal(raw, &codes); err != nil {
			return err
		}
	}
	for _, code := range codes {
		if common.HexToAddress(code.Address) == v6Token {
			return errors.New("V6 token address already has genesis code")
		}
	}
	codes = append(codes, &evmtypes.Code{Address: v6Token.Hex(), Code: c.Token.RuntimeCode})
	evm["codes"], err = json.Marshal(codes)
	if err != nil {
		return err
	}
	state["evm"], err = json.Marshal(evm)
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
	return os.WriteFile(path, append(data, '\n'), 0600)
}

func recordV6(n *node) (any, error) {
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
	sponsor, err := crypto.HexToECDSA(strings.TrimSpace(secret))
	if err != nil {
		return nil, err
	}
	defer sponsor.D.SetInt64(0)
	return recordV6RPC(n, sponsor, genesis.AppState.Sequencer.ReceiptKeys, vectorV2Provenance{
		Revision: "0a76cd72f6c808eb67379b35bfa45c17d608d093", ChainID: genesis.ChainID,
		Generator: "daemon/committed-fixture/vector_v6.go",
		Command:   "committed-fixture record --paxd /tmp/r509-paxd --out daemon/committed-fixture/vectors --vector " + vectorV6Name,
	}, true)
}

func recordV6RPC(n *node, sponsor *ecdsa.PrivateKey, pins seqkeeper.ReceiptKeys, provenance vectorV2Provenance, committed bool) (*v6Vector, error) {
	ctx, cancel := context.WithTimeout(n.ctx, 90*time.Second)
	defer cancel()
	if err := pins.Validate(); err != nil {
		return nil, err
	}
	c, delegateABI, tokenABI, err := v6LoadContracts()
	if err != nil {
		return nil, err
	}
	user, err := crypto.GenerateKey()
	if err != nil {
		return nil, err
	}
	defer user.D.SetInt64(0)
	v := &v6Vector{Name: vectorV6Name, Provenance: provenance, ChainID: 125, ReceiptKeys: pins,
		Sponsor: crypto.PubkeyToAddress(sponsor.PublicKey), User: crypto.PubkeyToAddress(user.PublicKey)}
	var id hexutil.Big
	if err := n.client.CallContext(ctx, &id, "eth_chainId"); err != nil {
		return nil, err
	}
	if id.ToInt().Cmp(big.NewInt(125)) != 0 {
		return nil, errors.New("V6 requires chain 125")
	}
	found := false
	for _, pin := range pins {
		if bytes.Equal(pin.PublicKey, n.receiptPublicKey) {
			v.PinnedKey = pin
			found = true
		}
	}
	if !found {
		return nil, errors.New("V6 primary key is absent from genesis pins")
	}
	balance := func(account common.Address) (*hexutil.Big, error) {
		var b hexutil.Big
		err := v4PendingCall(ctx, n, &b, "eth_getBalance", account, "pending")
		return &b, err
	}
	send := func(to *common.Address, value *big.Int, data []byte, auth []ethtypes.SetCodeAuthorization, name string) (v6Case, error) {
		entry := v6Case{ID: name}
		var nonce hexutil.Uint64
		var price hexutil.Big
		if err := v4PendingCall(ctx, n, &nonce, "eth_getTransactionCount", v.Sponsor, "pending"); err != nil {
			return entry, err
		}
		if err := v4PendingCall(ctx, n, &price, "eth_gasPrice"); err != nil {
			return entry, err
		}
		var payload ethtypes.TxData = &ethtypes.DynamicFeeTx{ChainID: big.NewInt(125), Nonce: uint64(nonce), GasTipCap: new(big.Int), GasFeeCap: price.ToInt(), Gas: 3_000_000, To: to, Value: value, Data: data}
		if auth != nil {
			if to == nil {
				return entry, errors.New("set-code requires a recipient")
			}
			payload = &ethtypes.SetCodeTx{ChainID: uint256.NewInt(125), Nonce: uint64(nonce), GasTipCap: new(uint256.Int), GasFeeCap: uint256.MustFromBig(price.ToInt()), Gas: 3_000_000, To: *to, Value: uint256.MustFromBig(value), Data: data, AuthList: auth}
		}
		tx, err := ethtypes.SignNewTx(sponsor, ethtypes.LatestSignerForChainID(big.NewInt(125)), payload)
		if err != nil {
			return entry, err
		}
		entry.RawTransaction, err = tx.MarshalBinary()
		if err != nil {
			return entry, err
		}
		entry.SponsorBalanceBefore, err = balance(v.Sponsor)
		if err != nil {
			return entry, err
		}
		entry.UserBalanceBefore, err = balance(v.User)
		if err != nil {
			return entry, err
		}
		var hash common.Hash
		if err := n.client.CallContext(ctx, &hash, "eth_sendRawTransaction", entry.RawTransaction); err != nil {
			return entry, err
		}
		if hash != tx.Hash() {
			return entry, errors.New("V6 RPC changed transaction hash")
		}
		if committed {
			entry.Receipt, err = v4WaitCommitted(ctx, n, hash)
		} else {
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			for {
				if err = n.client.CallContext(ctx, &entry.Receipt, evmrpc.SequencerReceiptMethod, hash); err != nil {
					break
				}
				if entry.Receipt != nil && entry.Receipt.Rung != evmrpc.RungSubmitted {
					break
				}
				select {
				case <-ctx.Done():
					return entry, ctx.Err()
				case <-ticker.C:
				}
			}
		}
		if err != nil {
			return entry, err
		}
		if err := entry.Receipt.Verify(pins); err != nil {
			return entry, err
		}
		if entry.Receipt.Status != 1 {
			return entry, fmt.Errorf("V6 %s reverted: %s %x", name, entry.Receipt.VMError, entry.Receipt.ReturnData)
		}
		entry.SignedReceipt = bytes.Clone(entry.Receipt.Receipt)
		verified, err := verify.ReceiptSignature(entry.SignedReceipt, [32]byte(entry.Receipt.SigningKey))
		if err != nil {
			return entry, err
		}
		entry.ReceiptDigest = common.Hash(verified.Digest)
		entry.SponsorPaidFee = (*hexutil.Big)(new(big.Int).Mul(new(big.Int).SetUint64(uint64(entry.Receipt.GasUsed)), new(big.Int).SetUint64(uint64(entry.Receipt.EffectiveGasPrice))))
		entry.SponsorBalanceAfter, err = balance(v.Sponsor)
		if err != nil {
			return entry, err
		}
		entry.UserBalanceAfter, err = balance(v.User)
		if err != nil {
			return entry, err
		}
		err = v4PendingCall(ctx, n, &entry.DelegationDesignator, "eth_getCode", v.User, "pending")
		return entry, err
	}
	ctor, err := delegateABI.Pack("", v.Sponsor, big.NewInt(3_114_000), big.NewInt(3600))
	if err != nil {
		return nil, err
	}
	v.DelegateDeployment, err = send(nil, new(big.Int), append(bytes.Clone(c.Delegate.CreationCode), ctor...), nil, "V6-delegate-deployment")
	if err != nil {
		return nil, err
	}
	v.Delegate = v.DelegateDeployment.Receipt.ContractAddress
	if err := v4PendingCall(ctx, n, &v.DelegateCode, "eth_getCode", v.Delegate, "pending"); err != nil {
		return nil, err
	}
	mint, err := tokenABI.Pack("mint", v.User, big.NewInt(100_000_000))
	if err != nil {
		return nil, err
	}
	if _, err := send(&v6Token, new(big.Int), mint, nil, "V6-token-funding"); err != nil {
		return nil, err
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	defer clear(private)
	v.DIDPublicKey = hexutil.Bytes(public)
	message, err := evmtypes.LayerXBindMessage(big.NewInt(125), v.User, 0)
	if err != nil {
		return nil, err
	}
	addressABI, err := abi.JSON(strings.NewReader(`[{"type":"function","name":"bindLayerX","inputs":[{"name":"didPublicKey","type":"bytes32"},{"name":"signature","type":"bytes"}],"outputs":[]},{"type":"function","name":"getLayerXDid","inputs":[{"name":"account","type":"address"}],"outputs":[{"name":"publicKey","type":"bytes32"},{"name":"did","type":"string"}]}]`))
	if err != nil {
		return nil, err
	}
	v.BindCalldata, err = addressABI.Pack("bindLayerX", [32]byte(public), ed25519.Sign(private, message))
	if err != nil {
		return nil, err
	}
	calls := []v6Call{{To: common.HexToAddress(addr.AddrAddress), Value: new(big.Int), Data: v.BindCalldata}}
	q := v6Quote{Sponsor: v.Sponsor, Token: v6Token, MaxTokenAmount: big.NewInt(3_114_000), TokenAmount: big.NewInt(3_114_000), Deadline: big.NewInt(time.Now().Add(10 * time.Minute).Unix()), QuoteNonce: big.NewInt(7), GasCost: big.NewInt(1_000_000_000_000_000_000)}
	quoteDigest := v6QuoteDigest(v.User, q)
	encoded, err := abi.Arguments{{Type: delegateABI.Methods["executeSponsored"].Inputs[0].Type}}.Pack(calls)
	if err != nil {
		return nil, err
	}
	batchDigest := accounts.TextHash(crypto.Keccak256(crypto.Keccak256([]byte("SponsoredBatch(uint256 nonce,bytes32 callsHash,bytes32 quoteDigest)")), make([]byte, 32), crypto.Keccak256(encoded), quoteDigest))
	accountSig, err := v6Sign(user, batchDigest)
	if err != nil {
		return nil, err
	}
	sponsorSig, err := v6Sign(sponsor, quoteDigest)
	if err != nil {
		return nil, err
	}
	input, err := delegateABI.Pack("executeSponsored", calls, q, accountSig, sponsorSig)
	if err != nil {
		return nil, err
	}
	var userNonce hexutil.Uint64
	if err := v4PendingCall(ctx, n, &userNonce, "eth_getTransactionCount", v.User, "pending"); err != nil {
		return nil, err
	}
	authorization, err := ethtypes.SignSetCode(user, ethtypes.SetCodeAuthorization{ChainID: *uint256.NewInt(125), Address: v.Delegate, Nonce: uint64(userNonce)})
	if err != nil {
		return nil, err
	}
	v.Positive, err = send(&v.User, new(big.Int), input, []ethtypes.SetCodeAuthorization{authorization}, "V6-sponsored-bind")
	if err != nil {
		return nil, err
	}
	query, err := addressABI.Pack("getLayerXDid", v.User)
	if err != nil {
		return nil, err
	}
	var output hexutil.Bytes
	if err := v4PendingCall(ctx, n, &output, "eth_call", map[string]any{"to": addr.AddrAddress, "data": hexutil.Bytes(query)}, "pending"); err != nil {
		return nil, err
	}
	decoded, err := addressABI.Unpack("getLayerXDid", output)
	if err != nil {
		return nil, err
	}
	if decoded[0].([32]byte) != [32]byte(public) {
		return nil, errors.New("read-back DID public key differs")
	}
	v.BoundDID = decoded[1].(string)
	v.Refusal, err = send(&v.User, big.NewInt(1), nil, nil, "V6-plain-transfer")
	if err != nil {
		return nil, err
	}
	v.Provenance.FirstHeight = uint64(v.DelegateDeployment.Receipt.Height)
	v.Provenance.LastHeight = uint64(v.Refusal.Receipt.Height)
	return v, verifyVectorV6(v)
}

func v6QuoteDigest(account common.Address, q v6Quote) []byte {
	word := func(v *big.Int) []byte { return common.BigToHash(v).Bytes() }
	addressWord := func(a common.Address) []byte { return common.BytesToHash(a.Bytes()).Bytes() }
	return accounts.TextHash(crypto.Keccak256(crypto.Keccak256([]byte("Quote(uint256 chainId,address account,address sponsor,address token,uint256 maxTokenAmount,uint256 tokenAmount,uint256 deadline,uint256 quoteNonce,uint256 gasCost)")), word(big.NewInt(125)), addressWord(account), addressWord(q.Sponsor), addressWord(q.Token), word(q.MaxTokenAmount), word(q.TokenAmount), word(q.Deadline), word(q.QuoteNonce), word(q.GasCost)))
}
func v6Sign(key *ecdsa.PrivateKey, digest []byte) ([]byte, error) {
	sig, err := crypto.Sign(digest, key)
	if err == nil {
		sig[64] += 27
	}
	return sig, err
}

func verifyVectorV6(v *v6Vector) error {
	if v == nil || v.Name != vectorV6Name || v.ChainID != 125 || v.Sponsor == v.User || v.Delegate == (common.Address{}) || v.Provenance.Revision == "" || v.Provenance.Generator == "" || v.Provenance.Command == "" || v.Provenance.ChainID == "" {
		return errors.New("invalid V6 identity or provenance")
	}
	if err := v.ReceiptKeys.Validate(); err != nil {
		return err
	}
	if err := (seqkeeper.ReceiptKeys{v.PinnedKey}).Validate(); err != nil {
		return err
	}
	if sha256.Sum256(v.PinnedKey.PublicKey) != [32]byte(v.PinnedKey.KeyId) {
		return errors.New("V6 genesis pin id differs from its public key")
	}
	pinned := false
	for _, pin := range v.ReceiptKeys {
		if bytes.Equal(pin.KeyId, v.PinnedKey.KeyId) && bytes.Equal(pin.PublicKey, v.PinnedKey.PublicKey) && pin.FirstHeight == v.PinnedKey.FirstHeight && pin.LastHeight == v.PinnedKey.LastHeight {
			pinned = true
		}
	}
	if !pinned || len(v.DelegateCode) == 0 || len(v.DIDPublicKey) != 32 || v.BoundDID != evmtypes.LayerXDid([32]byte(v.DIDPublicKey)) {
		return errors.New("V6 lost genesis pin, delegate code or executed DID binding")
	}
	c, d, _, err := v6LoadContracts()
	if err != nil {
		return err
	}
	expectedCode := bytes.Clone(c.Delegate.RuntimeCode)
	for name, spans := range c.Delegate.ImmutableReferences {
		var word []byte
		switch name {
		case "rateSource":
			word = common.BytesToHash(v.Delegate.Bytes()).Bytes()
		case "maxRateAge":
			word = common.BigToHash(big.NewInt(3600)).Bytes()
		default:
			return fmt.Errorf("unknown delegate immutable %s", name)
		}
		for _, span := range spans {
			if span.Length != 32 || span.Start < 0 || span.Start+32 > len(expectedCode) {
				return errors.New("invalid delegate immutable offset")
			}
			copy(expectedCode[span.Start:span.Start+32], word)
		}
	}
	if !bytes.Equal(expectedCode, v.DelegateCode) {
		return errors.New("read-back delegate is not the production wallet contract")
	}
	ctor, err := d.Pack("", v.Sponsor, big.NewInt(3_114_000), big.NewInt(3600))
	if err != nil {
		return err
	}
	var deployment ethtypes.Transaction
	if err := deployment.UnmarshalBinary(v.DelegateDeployment.RawTransaction); err != nil {
		return err
	}
	if !bytes.Equal(deployment.Data(), append(bytes.Clone(c.Delegate.CreationCode), ctor...)) {
		return errors.New("delegate creation differs from production constructor")
	}
	for _, entry := range []*v6Case{&v.DelegateDeployment, &v.Positive, &v.Refusal} {
		r := entry.Receipt
		if r == nil || r.SequencerReceiptView == nil || (r.Rung != evmrpc.RungReceipted && r.Rung != evmrpc.RungCommitted) {
			return errors.New("V6 requires an executed receipt")
		}
		if err := r.Verify(v.ReceiptKeys); err != nil {
			return err
		}
		if err := r.Verify(seqkeeper.ReceiptKeys{v.PinnedKey}); err != nil {
			return err
		}
		if !bytes.Equal(entry.SignedReceipt, r.Receipt) {
			return errors.New("V6 signed receipt differs from RPC bytes")
		}
		verified, err := verify.ReceiptSignature(entry.SignedReceipt, [32]byte(v.PinnedKey.PublicKey))
		if err != nil {
			return err
		}
		if common.Hash(verified.Digest) != entry.ReceiptDigest || r.Status != 1 || verified.Receipt.ResultCode != 0 {
			return errors.New("V6 digest or successful execution differs")
		}
		var tx ethtypes.Transaction
		if err := tx.UnmarshalBinary(entry.RawTransaction); err != nil {
			return err
		}
		sender, err := ethtypes.Sender(ethtypes.LatestSignerForChainID(big.NewInt(125)), &tx)
		if err != nil {
			return err
		}
		if sender != v.Sponsor || r.From != sender || tx.Hash() != r.TxHash || tx.Hash() != common.Hash(verified.Receipt.ActivityID) || tx.ChainId().Cmp(big.NewInt(125)) != 0 || uint64(r.Nonce) != tx.Nonce() || verified.Receipt.Operation != tx.Type() {
			return errors.New("V6 transaction differs from authenticated execution")
		}
		if uint64(r.Height) < v.Provenance.FirstHeight || uint64(r.Height) > v.Provenance.LastHeight {
			return errors.New("V6 receipt outside provenance heights")
		}
		if entry.SponsorPaidFee == nil || entry.SponsorBalanceBefore == nil || entry.SponsorBalanceAfter == nil || entry.UserBalanceBefore == nil || entry.UserBalanceAfter == nil {
			return errors.New("V6 lacks measured balances or fee")
		}
		fee := new(big.Int).Mul(new(big.Int).SetUint64(uint64(r.GasUsed)), new(big.Int).SetUint64(uint64(r.EffectiveGasPrice)))
		delta := new(big.Int).Sub(entry.SponsorBalanceBefore.ToInt(), entry.SponsorBalanceAfter.ToInt())
		if fee.Sign() <= 0 || fee.Cmp(entry.SponsorPaidFee.ToInt()) != 0 || r.FeeAmount == nil || fee.Cmp(r.FeeAmount.ToInt()) != 0 || delta.Cmp(new(big.Int).Add(fee, tx.Value())) != 0 {
			return errors.New("V6 sponsor did not pay authenticated fee and value")
		}
		if entry == &v.DelegateDeployment {
			if tx.To() != nil || tx.Type() != ethtypes.DynamicFeeTxType || r.ContractAddress != v.Delegate || crypto.CreateAddress(sender, tx.Nonce()) != v.Delegate {
				return errors.New("V6 delegate was not deployed by the sponsor")
			}
			continue
		}
		if tx.To() == nil || *tx.To() != v.User || r.To != v.User {
			return errors.New("V6 receipt recipient differs from user")
		}
		designator := ethtypes.AddressToDelegation(v.Delegate)
		if !bytes.Equal(entry.DelegationDesignator, designator) {
			return errors.New("V6 read-back code is not the authorized delegation designator")
		}
		if entry == &v.Positive {
			if err := v6SponsoredExecution(v, &tx); err != nil {
				return err
			}
			if entry.UserBalanceBefore.ToInt().Sign() != 0 || entry.UserBalanceAfter.ToInt().Sign() != 0 {
				return errors.New("V6 user paid native currency during sponsored binding")
			}
		} else {
			if tx.Type() != ethtypes.DynamicFeeTxType || len(tx.Data()) != 0 || len(tx.SetCodeAuthorizations()) != 0 || tx.Value().Cmp(big.NewInt(1)) != 0 || r.Sponsor != (common.Address{}) || len(r.Logs) != 0 {
				return errors.New("V6 refusal is not a plain successful transfer")
			}
			if new(big.Int).Sub(entry.UserBalanceAfter.ToInt(), entry.UserBalanceBefore.ToInt()).Cmp(tx.Value()) != 0 {
				return errors.New("V6 transfer did not credit the user")
			}
			if v6SponsoredExecution(v, &tx) == nil {
				return errors.New("V6 accepted a plain transfer as sponsorship evidence")
			}
		}
	}
	return nil
}

func v6SponsoredExecution(v *v6Vector, tx *ethtypes.Transaction) error {
	if tx.Type() != ethtypes.SetCodeTxType || tx.To() == nil || *tx.To() != v.User || tx.Value().Sign() != 0 || len(tx.SetCodeAuthorizations()) != 1 {
		return errors.New("not a sponsored set-code execution")
	}
	auth := tx.SetCodeAuthorizations()[0]
	authority, err := auth.Authority()
	if err != nil {
		return err
	}
	if authority != v.User || auth.Address != v.Delegate || auth.ChainID.CmpUint64(125) != 0 || auth.Nonce != 0 {
		return errors.New("invalid user delegation authorization")
	}
	_, d, _, err := v6LoadContracts()
	if err != nil {
		return err
	}
	method := d.Methods["executeSponsored"]
	if len(tx.Data()) < 4 || !bytes.Equal(tx.Data()[:4], method.ID) {
		return errors.New("missing executeSponsored call")
	}
	args, err := method.Inputs.Unpack(tx.Data()[4:])
	if err != nil {
		return err
	}
	calls := *abi.ConvertType(args[0], new([]v6Call)).(*[]v6Call)
	q := *abi.ConvertType(args[1], new(v6Quote)).(*v6Quote)
	if len(calls) != 1 || calls[0].To != common.HexToAddress(addr.AddrAddress) || calls[0].Value.Sign() != 0 || !bytes.Equal(calls[0].Data, v.BindCalldata) || q.Sponsor != v.Sponsor || q.Token != v6Token {
		return errors.New("sponsored construction differs from gateway bind")
	}
	bindABI, err := abi.JSON(strings.NewReader(`[{"type":"function","name":"bindLayerX","inputs":[{"name":"didPublicKey","type":"bytes32"},{"name":"signature","type":"bytes"}],"outputs":[]}]`))
	if err != nil {
		return err
	}
	bind := bindABI.Methods["bindLayerX"]
	if len(v.BindCalldata) < 4 || !bytes.Equal(v.BindCalldata[:4], bind.ID) {
		return errors.New("missing bindLayerX call")
	}
	binding, err := bind.Inputs.Unpack(v.BindCalldata[4:])
	if err != nil {
		return err
	}
	message, err := evmtypes.LayerXBindMessage(big.NewInt(125), v.User, 0)
	if err != nil {
		return err
	}
	if binding[0].([32]byte) != [32]byte(v.DIDPublicKey) || !ed25519.Verify(ed25519.PublicKey(v.DIDPublicKey), message, binding[1].([]byte)) {
		return errors.New("invalid DID binding signature")
	}
	r := v.Positive.Receipt
	if r.Sponsor != v.Sponsor {
		return errors.New("signed receipt lacks executed sponsor")
	}
	event := d.Events["Sponsored"]
	found := false
	bound := false
	boundID := crypto.Keccak256Hash([]byte("LayerXBound(address,bytes32,uint64)"))
	for _, log := range r.Logs {
		if log.Address == v.User && len(log.Topics) == 3 && log.Topics[0] == event.ID {
			if found || log.Topics[1] != common.BytesToHash(v.Sponsor.Bytes()) || log.Topics[2] != common.BytesToHash(v6Token.Bytes()) {
				return errors.New("wrong Sponsored log")
			}
			values, err := event.Inputs.NonIndexed().Unpack(log.Data)
			if err != nil {
				return err
			}
			if values[0].(*big.Int).Cmp(q.TokenAmount) != 0 || values[1].(*big.Int).Cmp(q.QuoteNonce) != 0 {
				return errors.New("wrong Sponsored repayment")
			}
			found = true
		}
		if log.Address == common.HexToAddress(addr.AddrAddress) && len(log.Topics) > 0 && log.Topics[0] == boundID {
			if bound || len(log.Topics) != 3 || log.Topics[1] != common.BytesToHash(v.User.Bytes()) || log.Topics[2] != common.Hash([32]byte(v.DIDPublicKey)) || !bytes.Equal(log.Data, make([]byte, 32)) {
				return errors.New("wrong LayerXBound log")
			}
			bound = true
		}
	}
	if !found || !bound {
		return errors.New("missing executed sponsorship or DID binding log")
	}
	encoded, err := rlp.EncodeToBytes(tx.SetCodeAuthorizations())
	if err != nil {
		return err
	}
	signed, err := verify.ReceiptSignature(r.Receipt, [32]byte(v.PinnedKey.PublicKey))
	if err != nil {
		return err
	}
	if signed.Receipt.AuthorizationHash != crypto.Keccak256Hash(encoded) {
		return errors.New("signed receipt does not bind authorization list")
	}
	return nil
}
