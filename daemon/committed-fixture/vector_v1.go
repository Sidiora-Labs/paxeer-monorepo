package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/params"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/viper"
)

const v1Name = "vector-v1-receipted-positive"
const v1Creation = "0x6001600052601160206000a16002600052602260206000a160006000f3"

type v1ReceiptKey struct {
	KeyID       common.Hash   `json:"keyId"`
	PublicKey   hexutil.Bytes `json:"publicKey"`
	FirstHeight uint64        `json:"firstHeight"`
	LastHeight  uint64        `json:"lastHeight"`
}

type v1Provenance struct {
	PaxdRevision    string `json:"paxdRevision"`
	ChainID         string `json:"chainId"`
	EVMChainID      string `json:"evmChainId"`
	FirstHeight     uint64 `json:"firstHeight"`
	LastHeight      uint64 `json:"lastHeight"`
	RecorderCommand string `json:"recorderCommand"`
}

type v1Vector struct {
	Name           string                         `json:"name"`
	Provenance     v1Provenance                   `json:"provenance"`
	RawTransaction hexutil.Bytes                  `json:"rawTransaction"`
	SignedReceipt  hexutil.Bytes                  `json:"signedReceipt"`
	ReceiptJSON    *evmrpc.SequencerReceiptResult `json:"receiptJson"`
	ReceiptKeys    []v1ReceiptKey                 `json:"receiptKeys"`
}

func init() {
	registerVector(v1Name, recordV1)
}

func (n *node) prime() error {
	params.SetAddressPrefixes()
	cfg := tmconfig.DefaultConfig().SetRoot(n.home)
	if _, err := toml.DecodeFile(filepath.Join(n.home, "config", "config.toml"), cfg); err != nil {
		return err
	}
	opts := viper.New()
	opts.SetConfigFile(filepath.Join(n.home, "config", "app.toml"))
	if err := opts.ReadInConfig(); err != nil {
		return err
	}
	opts.Set("home", n.home)
	opts.Set("chain-id", fixtureChainID)
	a := app.New(nil, nil, true, map[int64]bool{}, n.home, 0, true, cfg,
		app.MakeEncodingConfig(), wasm.EnableAllProposals, opts, app.EmptyWasmOpts, app.EmptyAppOptions,
		baseapp.SetMinGasPrices(opts.GetString("minimum-gas-prices")))
	defer a.Close()
	seqCfg, err := sequencer.ReadConfig(opts)
	if err != nil {
		return err
	}
	primary, err := sequencer.NewNode(seqCfg, a)
	if err != nil {
		return err
	}
	defer primary.Stop()
	if err := primary.Start(n.ctx); err != nil {
		return err
	}
	kr, err := keyring.New(sdk.KeyringServiceName(), keyring.BackendTest, n.home, nil)
	if err != nil {
		return err
	}
	secret, err := keyring.NewUnsafe(kr).UnsafeExportPrivKeyHex("fixture")
	if err != nil {
		return err
	}
	key, err := crypto.HexToECDSA(strings.TrimSpace(secret))
	if err != nil {
		return err
	}
	defer key.D.SetInt64(0)
	ctx, err := primary.Executor.Snapshot()
	if err != nil {
		return err
	}
	from := crypto.PubkeyToAddress(key.PublicKey)
	baseFee, err := primary.FeeFloor.BaseFeePerGas()
	if err != nil {
		return err
	}
	price := baseFee.Ceil().TruncateInt().BigInt()
	tx, err := ethtypes.SignNewTx(key, ethtypes.LatestSignerForChainID(a.EvmKeeper.ChainID(ctx)), &ethtypes.LegacyTx{
		Nonce: a.EvmKeeper.GetNonce(ctx, from), GasPrice: price, Gas: 21000, To: &from, Value: new(big.Int),
	})
	if err != nil {
		return err
	}
	data, err := ethtx.NewTxDataFromTx(tx)
	if err != nil {
		return err
	}
	msg, err := types.NewMsgEVMTransaction(data)
	if err != nil {
		return err
	}
	config := a.GetTxConfig()
	builder := config.NewTxBuilder()
	if err := builder.SetMsgs(msg); err != nil {
		return err
	}
	builder.SetGasLimit(tx.Gas())
	encoded, err := config.TxEncoder()(builder.GetTx())
	if err != nil {
		return err
	}
	if _, err := primary.Accepter.Accept(encoded); err != nil {
		return fmt.Errorf("prime the first sequenced block with a self transfer: %w", err)
	}
	n.gasPrice = new(big.Int).Set(price)
	return nil
}

func recordV1(n *node) (any, error) {
	ctx, cancel := context.WithTimeout(n.ctx, 45*time.Second)
	defer cancel()
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
	pins := genesis.AppState.Sequencer.ReceiptKeys
	if err := pins.Validate(); err != nil {
		return nil, err
	}
	var primaryPins seqkeeper.ReceiptKeys
	for _, pin := range pins {
		if bytes.Equal(pin.PublicKey, n.receiptPublicKey) {
			primaryPins = append(primaryPins, pin)
		}
	}
	if len(primaryPins) != 1 {
		return nil, errors.New("V1 genesis must pin the node's generated receipt key")
	}
	versionBytes, err := n.command("version", "--long", "--output", "json")
	if err != nil {
		return nil, err
	}
	var version struct {
		Commit string `json:"commit"`
	}
	if err := json.Unmarshal(versionBytes, &version); err != nil {
		return nil, err
	}
	if revision, err := hex.DecodeString(version.Commit); err != nil || len(revision) != 20 {
		return nil, errors.New("V1 recording requires paxd built with its full Git revision")
	}
	kr, err := keyring.New(sdk.KeyringServiceName(), keyring.BackendTest, n.home, nil)
	if err != nil {
		return nil, err
	}
	secret, err := keyring.NewUnsafe(kr).UnsafeExportPrivKeyHex("fixture")
	if err != nil {
		return nil, err
	}
	key, err := crypto.HexToECDSA(strings.TrimSpace(secret))
	if err != nil {
		return nil, errors.New("decode temporary fixture signing key")
	}
	defer key.D.SetInt64(0)
	from := crypto.PubkeyToAddress(key.PublicKey)
	if n.gasPrice == nil || n.gasPrice.Sign() <= 0 {
		return nil, errors.New("V1 requires the primed open block's positive gas price")
	}
	var chainID hexutil.Big
	if err := n.client.CallContext(ctx, &chainID, "eth_chainId"); err != nil {
		return nil, err
	}
	var nonce hexutil.Uint64
	if err := n.client.CallContext(ctx, &nonce, "eth_getTransactionCount", from, "pending"); err != nil {
		return nil, err
	}
	tx, err := ethtypes.SignNewTx(key, ethtypes.LatestSignerForChainID(chainID.ToInt()), &ethtypes.LegacyTx{
		Nonce: uint64(nonce), GasPrice: new(big.Int).Set(n.gasPrice), Gas: 200000,
		Value: new(big.Int), Data: common.FromHex(v1Creation),
	})
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
		return nil, errors.New("eth_sendRawTransaction returned a different transaction hash")
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var result *evmrpc.SequencerReceiptResult
		if err := n.client.CallContext(ctx, &result, "pax_sequencerReceipt", hash); err != nil {
			return nil, err
		}
		if result != nil && result.Rung != evmrpc.RungSubmitted {
			if result.Rung != evmrpc.RungReceipted {
				return nil, fmt.Errorf("V1 requires the receipted rung, got %s", result.Rung)
			}
			if err := result.Verify(pins); err != nil {
				return nil, err
			}
			created := crypto.CreateAddress(from, tx.Nonce())
			if result.TxHash != hash || uint64(result.Nonce) != tx.Nonce() || result.Status != 1 ||
				result.From != from || result.To != created || result.ContractAddress != created || len(result.Logs) != 2 {
				return nil, errors.New("V1 creation did not succeed with two logs bound to the transaction")
			}
			for i, log := range result.Logs {
				var data [32]byte
				data[31] = byte(i + 1)
				var topic common.Hash
				topic[31] = byte(0x11 * (i + 1))
				if log.Address != created || uint64(log.LogIndex) != uint64(i) || len(log.Topics) != 1 ||
					log.Topics[0] != topic || !bytes.Equal(log.Data, data[:]) {
					return nil, fmt.Errorf("V1 constructor log %d differs from its expected address, ordinal, topic or data", i)
				}
			}
			pin := primaryPins[0]
			if sha256.Sum256(pin.PublicKey) != [32]byte(pin.KeyId) {
				return nil, errors.New("V1 genesis key id differs from the generated key digest")
			}
			return &v1Vector{
				Name: v1Name,
				Provenance: v1Provenance{
					PaxdRevision: version.Commit, ChainID: genesis.ChainID, EVMChainID: chainID.String(),
					FirstHeight: uint64(result.Height), LastHeight: uint64(result.Height),
					RecorderCommand: "committed-fixture record --paxd " + n.paxd + " --out daemon/committed-fixture/vectors --vector " + v1Name,
				},
				RawTransaction: raw, SignedReceipt: bytes.Clone(result.Receipt), ReceiptJSON: result,
				ReceiptKeys: []v1ReceiptKey{{KeyID: common.BytesToHash(pin.KeyId), PublicKey: bytes.Clone(pin.PublicKey), FirstHeight: pin.FirstHeight, LastHeight: pin.LastHeight}},
			}, nil
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("wait for V1 receipted rung: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}
