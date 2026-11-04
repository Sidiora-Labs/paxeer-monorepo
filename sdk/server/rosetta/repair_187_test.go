package rosetta_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	ros "github.com/coinbase/rosetta-sdk-go/types"
	"github.com/stretchr/testify/require"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	clienttx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/rosetta"
	crgerrs "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/rosetta/lib/errors"
	servertypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/network"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
)

type repair187Network struct {
	network   *network.Network
	rpc       *rpchttp.HTTP
	client    *rosetta.Client
	url       string
	netID     *ros.NetworkIdentifier
	keys      []*secp256k1.PrivKey
	addresses []sdk.AccAddress
}

func repair187Start(t *testing.T) *repair187Network {
	t.Helper()
	cfg := network.DefaultConfig(t)
	cfg.ChainID = "repair-187"
	cfg.TimeoutCommit = 100 * time.Millisecond
	cfg.MinGasPrices = "0uhpx"
	// Leave InitChain to the real consensus node and its generated genesis.
	cfg.AppConstructor = func(network.Validator) servertypes.Application {
		return app.Setup(t, true, false, false)
	}
	h := &repair187Network{}
	var accounts []authtypes.GenesisAccount
	var balances []banktypes.Balance
	for i := 0; i < 2; i++ {
		key := secp256k1.GenPrivKey()
		addr := sdk.AccAddress(key.PubKey().Address())
		h.keys = append(h.keys, key)
		h.addresses = append(h.addresses, addr)
		accounts = append(accounts, authtypes.NewBaseAccount(addr, key.PubKey(), uint64(i), uint64(2+i*5)))
		balances = append(balances, banktypes.Balance{Address: addr.String(), Coins: sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1000000))})
	}
	var authGenesis authtypes.GenesisState
	cfg.Codec.MustUnmarshalJSON(cfg.GenesisState[authtypes.ModuleName], &authGenesis)
	packed, err := authtypes.PackAccounts(accounts)
	require.NoError(t, err)
	authGenesis.Accounts = append(authGenesis.Accounts, packed...)
	cfg.GenesisState[authtypes.ModuleName] = cfg.Codec.MustMarshalJSON(&authGenesis)
	var bankGenesis banktypes.GenesisState
	cfg.Codec.MustUnmarshalJSON(cfg.GenesisState[banktypes.ModuleName], &bankGenesis)
	bankGenesis.Balances = append(bankGenesis.Balances, balances...)
	cfg.GenesisState[banktypes.ModuleName] = cfg.Codec.MustMarshalJSON(&bankGenesis)
	h.network = network.New(t, cfg)
	t.Cleanup(h.network.Cleanup)
	_, err = h.network.WaitForHeightWithTimeout(2, 30*time.Second)
	require.NoError(t, err)
	val := h.network.Validators[0]
	h.rpc, err = rpchttp.New(val.RPCAddress)
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())
	conf := &rosetta.Config{
		Blockchain: "paxeer", Network: cfg.ChainID, Addr: addr,
		TendermintRPC: val.RPCAddress, GRPCEndpoint: val.AppConfig.GRPC.Address,
		Codec: cfg.Codec.(*codec.ProtoCodec), InterfaceRegistry: cfg.InterfaceRegistry,
	}
	h.client, err = rosetta.NewClient(conf)
	require.NoError(t, err)
	require.NoError(t, h.client.Bootstrap())
	server, err := rosetta.ServerFromConfig(conf)
	require.NoError(t, err)
	started := make(chan error, 1)
	go func() { started <- server.Start() }()
	h.url = "http://" + addr
	h.netID = conf.NetworkIdentifier()
	require.Eventually(t, func() bool {
		select {
		case err := <-started:
			t.Fatalf("Rosetta server stopped: %v", err)
		default:
		}
		resp, err := http.Get(h.url + "/network/list")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return true
	}, 5*time.Second, 10*time.Millisecond)
	return h
}

func (h *repair187Network) post(t *testing.T, path string, request, response interface{}) int {
	t.Helper()
	body, err := json.Marshal(request)
	require.NoError(t, err)
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Post(h.url+path, "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, response), "%s: %s", path, data)
	return resp.StatusCode
}

func (h *repair187Network) account(t *testing.T, index int) *rosetta.SignerData {
	t.Helper()
	options, err := (rosetta.PreprocessOperationsOptionsResponse{ExpectedSigners: []string{h.addresses[index].String()}, GasLimit: 1000000, GasPrice: "10000uhpx"}).ToMetadata()
	require.NoError(t, err)
	meta, err := h.client.ConstructionMetadataFromOptions(context.Background(), options)
	require.NoError(t, err)
	var construction rosetta.ConstructionMetadata
	require.NoError(t, construction.FromMetadata(meta))
	return construction.SignersData[0]
}

func (h *repair187Network) send(t *testing.T, amount int64) *coretypes.ResultTx {
	t.Helper()
	cfg := h.network.Config.TxConfig
	builder := cfg.NewTxBuilder()
	require.NoError(t, builder.SetMsgs(&banktypes.MsgSend{FromAddress: h.addresses[0].String(), ToAddress: h.addresses[1].String(), Amount: sdk.NewCoins(sdk.NewInt64Coin("uhpx", amount))}))
	builder.SetGasLimit(1000000)
	builder.SetFeeAmount(sdk.NewCoins(sdk.NewInt64Coin("uhpx", 10000)))
	account := h.account(t, 0)
	sig, err := clienttx.SignWithPrivKey(signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON, authsigning.SignerData{ChainID: h.netID.Network, AccountNumber: account.AccountNumber, Sequence: account.Sequence}, builder, h.keys[0], cfg, account.Sequence)
	require.NoError(t, err)
	require.NoError(t, builder.SetSignatures(sig))
	raw, err := cfg.TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	broadcast, err := h.rpc.BroadcastTxSync(context.Background(), raw)
	require.NoError(t, err)
	require.Zero(t, broadcast.Code, broadcast.Log)
	var result *coretypes.ResultTx
	require.Eventually(t, func() bool {
		result, err = h.rpc.Tx(context.Background(), broadcast.Hash, false)
		return err == nil && result != nil
	}, 20*time.Second, 50*time.Millisecond)
	return result
}

func (h *repair187Network) block(t *testing.T, height int64) *ros.Block {
	t.Helper()
	var response ros.BlockResponse
	status := h.post(t, "/block", &ros.BlockRequest{NetworkIdentifier: h.netID, BlockIdentifier: &ros.PartialBlockIdentifier{Index: &height}}, &response)
	require.Equal(t, http.StatusOK, status)
	require.NotNil(t, response.Block)
	return response.Block
}

func repair187FindTx(t *testing.T, block *ros.Block, hash []byte) *ros.Transaction {
	t.Helper()
	for _, tx := range block.Transactions {
		if tx.TransactionIdentifier.Hash == fmt.Sprintf("%X", hash) {
			return tx
		}
	}
	t.Fatalf("transaction %X missing from /block", hash)
	return nil
}

func TestRepair_187_ResultsDiscarded(t *testing.T) {
	h := repair187Start(t)
	for _, test := range []struct {
		name   string
		amount int64
		status string
	}{{"success", 17, rosetta.StatusTxSuccess}, {"failed", 2000000, rosetta.StatusTxReverted}} {
		t.Run(test.name, func(t *testing.T) {
			result := h.send(t, test.amount)
			if test.status == rosetta.StatusTxSuccess {
				require.Zero(t, result.TxResult.Code)
			} else {
				require.NotZero(t, result.TxResult.Code)
			}
			block := h.block(t, result.Height)
			tx := repair187FindTx(t, block, result.Hash)
			require.NotEmpty(t, tx.Operations)
			for i, op := range tx.Operations {
				require.EqualValues(t, i, op.OperationIdentifier.Index)
				require.Equal(t, test.status, *op.Status)
			}
			// Compare each observable balance operation with events emitted by the real application.
			cdc, ir := rosetta.MakeCodec()
			converter := rosetta.NewConverter(cdc, ir, h.network.Config.TxConfig)
			expected := converter.ToRosetta().BalanceOps(test.status, result.TxResult.Events)
			require.NotEmpty(t, expected, "the real producer must emit fee/transfer events")
			require.Equal(t, rosetta.AddOperationIndexes(nil, expected), rosetta.AddOperationIndexes(nil, tx.Operations[1:]))
			if test.status == rosetta.StatusTxSuccess {
				var sent, received bool
				for _, op := range expected {
					if op.Account.Address == h.addresses[0].String() && op.Amount.Value == "-17" {
						sent = true
					}
					if op.Account.Address == h.addresses[1].String() && op.Amount.Value == "17" {
						received = true
					}
				}
				require.True(t, sent)
				require.True(t, received)
			}
		})
	}
}

func TestRepair_187_NormalTxLookup(t *testing.T) {
	h := repair187Start(t)
	for _, amount := range []int64{23, 2000000} {
		result := h.send(t, amount)
		block := h.block(t, result.Height)
		expected := repair187FindTx(t, block, result.Hash)
		var response ros.BlockTransactionResponse
		status := h.post(t, "/block/transaction", &ros.BlockTransactionRequest{NetworkIdentifier: h.netID, BlockIdentifier: block.BlockIdentifier, TransactionIdentifier: expected.TransactionIdentifier}, &response)
		require.Equal(t, http.StatusOK, status)
		require.Equal(t, expected, response.Transaction)
	}
	unknown := strings.Repeat("AB", 32)
	_, err := h.client.GetTx(context.Background(), unknown)
	require.ErrorIs(t, err, crgerrs.ErrNotFound)
	var missing ros.Error
	block := h.block(t, 2)
	status := h.post(t, "/block/transaction", &ros.BlockTransactionRequest{NetworkIdentifier: h.netID, BlockIdentifier: block.BlockIdentifier, TransactionIdentifier: &ros.TransactionIdentifier{Hash: unknown}}, &missing)
	require.Equal(t, http.StatusInternalServerError, status)
	require.Equal(t, crgerrs.ToRosetta(crgerrs.ErrNotFound).Code, missing.Code)
	_, err = h.client.GetTx(context.Background(), "bad hash")
	require.ErrorIs(t, err, crgerrs.ErrCodec)
	_, err = h.client.GetTx(context.Background(), "AA")
	require.ErrorIs(t, err, crgerrs.ErrBadArgument)
}

func TestRepair_187_MultisignerKey(t *testing.T) {
	h := repair187Start(t)
	cdc, ir := rosetta.MakeCodec()
	converter := rosetta.NewConverter(cdc, ir, h.network.Config.TxConfig)
	var operations []*ros.Operation
	for i := range h.keys {
		ops, err := converter.ToRosetta().Ops("", &banktypes.MsgSend{FromAddress: h.addresses[i].String(), ToAddress: h.addresses[1-i].String(), Amount: sdk.NewCoins(sdk.NewInt64Coin("uhpx", int64(31+i)))})
		require.NoError(t, err)
		for _, op := range ops {
			op.Status = nil
		}
		operations = append(operations, ops...)
	}
	operations = rosetta.AddOperationIndexes(operations, nil)
	var preprocess ros.ConstructionPreprocessResponse
	require.Equal(t, http.StatusOK, h.post(t, "/construction/preprocess", &ros.ConstructionPreprocessRequest{NetworkIdentifier: h.netID, Operations: operations, Metadata: map[string]interface{}{"gas_limit": "1000000", "gas_price": "10000uhpx", "memo": "two distinct signers"}}, &preprocess))
	require.Len(t, preprocess.RequiredPublicKeys, 2)
	pubkeys := make([]*ros.PublicKey, 2)
	for i, key := range h.keys {
		require.Equal(t, h.addresses[i].String(), preprocess.RequiredPublicKeys[i].Address)
		pubkeys[i] = &ros.PublicKey{Bytes: key.PubKey().Bytes(), CurveType: ros.Secp256k1}
	}
	var metadata ros.ConstructionMetadataResponse
	require.Equal(t, http.StatusOK, h.post(t, "/construction/metadata", &ros.ConstructionMetadataRequest{NetworkIdentifier: h.netID, Options: preprocess.Options, PublicKeys: pubkeys}, &metadata))
	var meta rosetta.ConstructionMetadata
	require.NoError(t, meta.FromMetadata(metadata.Metadata))
	require.Len(t, meta.SignersData, 2)
	require.NotEqual(t, meta.SignersData[0].AccountNumber, meta.SignersData[1].AccountNumber)
	require.EqualValues(t, 2, meta.SignersData[0].Sequence)
	require.EqualValues(t, 7, meta.SignersData[1].Sequence)
	var payloads ros.ConstructionPayloadsResponse
	require.Equal(t, http.StatusOK, h.post(t, "/construction/payloads", &ros.ConstructionPayloadsRequest{NetworkIdentifier: h.netID, Operations: operations, Metadata: metadata.Metadata, PublicKeys: pubkeys}, &payloads))
	require.Len(t, payloads.Payloads, 2)
	unsigned, err := hex.DecodeString(payloads.UnsignedTransaction)
	require.NoError(t, err)
	var envelope rosetta.UnsignedTransaction
	require.NoError(t, json.Unmarshal(unsigned, &envelope))
	decoded, err := h.network.Config.TxConfig.TxDecoder()(envelope.Transaction)
	require.NoError(t, err)
	signatures := make([]*ros.Signature, 2)
	for i, payload := range payloads.Payloads {
		require.Equal(t, h.addresses[i].String(), payload.AccountIdentifier.Address)
		data := authsigning.SignerData{ChainID: meta.ChainID, AccountNumber: meta.SignersData[i].AccountNumber, Sequence: meta.SignersData[i].Sequence}
		canonical, err := h.network.Config.TxConfig.SignModeHandler().GetSignBytes(signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON, data, decoded)
		require.NoError(t, err)
		digest := sha256.Sum256(canonical)
		require.Equal(t, digest[:], payload.Bytes)
		key, _ := btcec.PrivKeyFromBytes(h.keys[i].Key)
		sig := ecdsa.SignCompact(key, payload.Bytes, true)[1:]
		signatures[i] = &ros.Signature{SigningPayload: payload, PublicKey: pubkeys[i], SignatureType: ros.Ecdsa, Bytes: sig}
	}
	var combined ros.ConstructionCombineResponse
	require.Equal(t, http.StatusOK, h.post(t, "/construction/combine", &ros.ConstructionCombineRequest{NetworkIdentifier: h.netID, UnsignedTransaction: payloads.UnsignedTransaction, Signatures: signatures}, &combined))
	var parsed ros.ConstructionParseResponse
	require.Equal(t, http.StatusOK, h.post(t, "/construction/parse", &ros.ConstructionParseRequest{NetworkIdentifier: h.netID, Signed: true, Transaction: combined.SignedTransaction}, &parsed))
	require.Equal(t, preprocess.RequiredPublicKeys, parsed.AccountIdentifierSigners)
	require.Len(t, parsed.Operations, len(operations))
	for i, op := range operations {
		require.Equal(t, op.Type, parsed.Operations[i].Type)
		require.Equal(t, op.Account, parsed.Operations[i].Account)
		require.Equal(t, op.Metadata, parsed.Operations[i].Metadata)
	}
	raw, err := hex.DecodeString(combined.SignedTransaction)
	require.NoError(t, err)
	decoded, err = h.network.Config.TxConfig.TxDecoder()(raw)
	require.NoError(t, err)
	signed := decoded.(authsigning.Tx)
	sigs, err := signed.GetSignaturesV2()
	require.NoError(t, err)
	require.Len(t, sigs, 2)
	for i, sig := range sigs {
		require.Equal(t, h.keys[i].PubKey(), sig.PubKey)
		require.Equal(t, meta.SignersData[i].Sequence, sig.Sequence)
		require.NoError(t, authsigning.VerifySignature(sig.PubKey, authsigning.SignerData{ChainID: meta.ChainID, AccountNumber: meta.SignersData[i].AccountNumber, Sequence: sig.Sequence}, sig.Data, h.network.Config.TxConfig.SignModeHandler(), signed))
	}
	var submitted ros.TransactionIdentifierResponse
	require.Equal(t, http.StatusOK, h.post(t, "/construction/submit", &ros.ConstructionSubmitRequest{NetworkIdentifier: h.netID, SignedTransaction: combined.SignedTransaction}, &submitted))
	require.Equal(t, fmt.Sprintf("%X", tmtypes.Tx(raw).Hash()), submitted.TransactionIdentifier.Hash)
	hash := tmtypes.Tx(raw).Hash()
	var committed *coretypes.ResultTx
	require.Eventually(t, func() bool {
		committed, err = h.rpc.Tx(context.Background(), hash[:], false)
		return err == nil && committed != nil
	}, 20*time.Second, 50*time.Millisecond)
	require.Equal(t, uint32(abci.CodeTypeOK), committed.TxResult.Code, committed.TxResult.Log)
	for i := range h.keys {
		require.Equal(t, meta.SignersData[i].Sequence+1, h.account(t, i).Sequence)
	}
	// Ordered keys remain mandatory after enabling the valid second signer.
	var refused ros.Error
	require.Equal(t, http.StatusInternalServerError, h.post(t, "/construction/payloads", &ros.ConstructionPayloadsRequest{NetworkIdentifier: h.netID, Operations: operations, Metadata: metadata.Metadata, PublicKeys: []*ros.PublicKey{pubkeys[1], pubkeys[0]}}, &refused))
	require.Equal(t, crgerrs.ToRosetta(crgerrs.ErrBadArgument).Code, refused.Code)
}
