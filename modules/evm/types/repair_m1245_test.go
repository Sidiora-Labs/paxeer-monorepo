package types_test

import (
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gogo/protobuf/proto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func repairM1245Codec() *codec.ProtoCodec {
	registry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)
	return codec.NewProtoCodec(registry)
}

func TestRepairM_1245_027_EnvelopeTransport(t *testing.T) {
	key, err := crypto.HexToECDSA(strings.Repeat("01", 32))
	require.NoError(t, err)
	chainID := big.NewInt(125)
	to := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	accesses := ethtypes.AccessList{{Address: to, StorageKeys: []common.Hash{common.HexToHash("0x42")}}}
	auth, err := ethtypes.SignSetCode(key, ethtypes.SetCodeAuthorization{ChainID: *uint256.NewInt(125), Address: to, Nonce: 3})
	require.NoError(t, err)
	inners := []ethtypes.TxData{
		&ethtypes.LegacyTx{Nonce: 7, GasPrice: big.NewInt(100), Gas: 60000, To: &to, Value: big.NewInt(9), Data: []byte{1, 2}},
		&ethtypes.AccessListTx{ChainID: chainID, Nonce: 7, GasPrice: big.NewInt(100), Gas: 60000, To: &to, Value: big.NewInt(9), Data: []byte{1, 2}, AccessList: accesses},
		&ethtypes.DynamicFeeTx{ChainID: chainID, Nonce: 7, GasTipCap: big.NewInt(2), GasFeeCap: big.NewInt(100), Gas: 60000, To: &to, Value: big.NewInt(9), Data: []byte{1, 2}, AccessList: accesses},
		&ethtypes.BlobTx{ChainID: uint256.NewInt(125), Nonce: 7, GasTipCap: uint256.NewInt(2), GasFeeCap: uint256.NewInt(100), Gas: 60000, To: to, Value: uint256.NewInt(9), Data: []byte{1, 2}, AccessList: accesses, BlobFeeCap: uint256.NewInt(3), BlobHashes: []common.Hash{{1, 2}}},
		&ethtypes.SetCodeTx{ChainID: uint256.NewInt(125), Nonce: 7, GasTipCap: uint256.NewInt(2), GasFeeCap: uint256.NewInt(100), Gas: 60000, To: to, Value: uint256.NewInt(9), Data: []byte{1, 2}, AccessList: accesses, AuthList: []ethtypes.SetCodeAuthorization{auth}},
	}
	cdc := repairM1245Codec()
	var envelopes []ethtx.TxData
	for _, inner := range inners {
		tx, err := ethtypes.SignTx(ethtypes.NewTx(inner), ethtypes.NewPragueSigner(chainID), key)
		require.NoError(t, err)
		data, err := ethtx.NewTxDataFromTx(tx)
		require.NoError(t, err)
		envelopes = append(envelopes, data)
	}
	associate, err := ethtx.NewAssociateTx(ethtypes.NewTx(inners[0]), "associate")
	require.NoError(t, err)
	envelopes = append(envelopes, associate)
	for _, data := range envelopes {
		t.Run(proto.MessageName(data), func(t *testing.T) {
			require.NoError(t, data.Validate())
			msg, err := types.NewMsgEVMTransaction(data)
			require.NoError(t, err)
			packed, err := types.PackTxData(data)
			require.NoError(t, err)
			require.Equal(t, msg.Data.TypeUrl, packed.TypeUrl)
			require.Equal(t, msg.Data.Value, packed.Value)
			check := func(t *testing.T, received *types.MsgEVMTransaction) {
				got, err := types.UnpackTxData(received.Data)
				require.NoError(t, err)
				require.IsType(t, data, got)
				encoded, err := proto.Marshal(got)
				require.NoError(t, err)
				require.Equal(t, packed.Value, encoded)
				require.NoError(t, received.ValidateBasic())
				transaction, concrete := received.AsTransaction()
				require.IsType(t, data, concrete)
				if ethData, ok := data.(ethtx.EthereumTxData); ok {
					require.Equal(t, ethtypes.NewTx(ethData.AsEthereumData()).Hash(), transaction.Hash())
				} else {
					require.Nil(t, transaction)
					require.True(t, received.IsAssociateTx())
				}
			}
			check(t, msg)
			uncached := &types.MsgEVMTransaction{Data: &codectypes.Any{TypeUrl: packed.TypeUrl, Value: append([]byte(nil), packed.Value...)}}
			require.Nil(t, uncached.Data.GetCachedValue())
			check(t, uncached)
			wire, err := proto.Marshal(msg)
			require.NoError(t, err)
			var fromProto types.MsgEVMTransaction
			require.NoError(t, proto.Unmarshal(wire, &fromProto))
			require.Nil(t, fromProto.Data.GetCachedValue())
			check(t, &fromProto)
			jsonWire, err := cdc.MarshalAsJSON(msg)
			require.NoError(t, err)
			var fromJSON types.MsgEVMTransaction
			require.NoError(t, cdc.UnmarshalAsJSON(jsonWire, &fromJSON))
			check(t, &fromJSON)
			fromJSON.Data = &codectypes.Any{TypeUrl: fromJSON.Data.TypeUrl, Value: fromJSON.Data.Value}
			check(t, &fromJSON)
		})
	}
	// All schemas accept empty protobuf bytes; the URL must still select each one.
	for _, empty := range []ethtx.TxData{&ethtx.LegacyTx{}, &ethtx.AccessListTx{}, &ethtx.DynamicFeeTx{}, &ethtx.BlobTx{}, &ethtx.AssociateTx{}, &ethtx.SetCodeTx{}} {
		packed, err := types.PackTxData(empty)
		require.NoError(t, err)
		got, err := types.UnpackTxData(&codectypes.Any{TypeUrl: packed.TypeUrl})
		require.NoError(t, err)
		require.IsType(t, empty, got)
	}
}

func TestRepairM_1245_027_RejectUnknownAndMismatchedTypes(t *testing.T) {
	for _, url := range []string{"", "/eth.UnknownTx", "/paxprotocol.paxchain.evm.MsgSend"} {
		packed, err := types.PackTxData(&ethtx.LegacyTx{})
		require.NoError(t, err)
		packed.TypeUrl = url
		_, err = types.UnpackTxData(packed)
		require.Error(t, err)
		_, err = types.UnpackTxData(&codectypes.Any{TypeUrl: url, Value: packed.Value})
		require.Error(t, err)
	}
	packed, err := types.PackTxData(&ethtx.LegacyTx{})
	require.NoError(t, err)
	other, err := types.PackTxData(&ethtx.AssociateTx{})
	require.NoError(t, err)
	packed.TypeUrl = other.TypeUrl
	_, err = types.UnpackTxData(packed)
	require.Error(t, err)
	_, err = types.UnpackTxData(&codectypes.Any{TypeUrl: other.TypeUrl, Value: []byte{0xff}})
	require.Error(t, err)
	_, err = types.UnpackTxData(nil)
	require.Error(t, err)
}

func repairM1245Genesis(t *testing.T) *types.GenesisState {
	t.Helper()
	a := testkeeper.EVMTestApp
	ctx, _ := a.GetContextForDeliverTx(nil).CacheContext()
	address := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	paxAddress := sdk.AccAddress(address.Bytes())
	k := &a.EvmKeeper
	k.SetAddressMapping(ctx, paxAddress, address)
	k.SetCode(ctx, address, []byte{0x60, 0x00})
	k.SetState(ctx, address, common.HexToHash("0x01"), common.HexToHash("0x02"))
	k.SetNonce(ctx, address, 4)
	ctx.KVStore(k.GetStoreKey()).Set(types.AccountFeeDenomKey(address), []byte("uhpx"))
	genesis := evm.ExportGenesis(ctx, k)
	require.NoError(t, genesis.Validate())
	return genesis
}

func TestRepairM_1245_021_GenesisEntries(t *testing.T) {
	valid := repairM1245Genesis(t)
	cdc := repairM1245Codec()
	basic := evm.AppModuleBasic{}
	wire, err := cdc.MarshalAsJSON(valid)
	require.NoError(t, err)
	require.NoError(t, basic.ValidateGenesis(cdc, nil, wire))
	cases := []struct {
		name   string
		mutate func(*types.GenesisState)
	}{
		{"association nil", func(g *types.GenesisState) { g.AddressAssociations = append(g.AddressAssociations, nil) }},
		{"association pax", func(g *types.GenesisState) { g.AddressAssociations[0].PaxAddress = "pax1invalid" }},
		{"association evm", func(g *types.GenesisState) { g.AddressAssociations[0].EthAddress = "0x12" }},
		{"code nil", func(g *types.GenesisState) { g.Codes = append(g.Codes, nil) }},
		{"code address", func(g *types.GenesisState) { g.Codes[0].Address = strings.Repeat("g", 40) }},
		{"state nil", func(g *types.GenesisState) { g.States = append(g.States, nil) }},
		{"state address", func(g *types.GenesisState) { g.States[0].Address += "00" }},
		{"state short key", func(g *types.GenesisState) { g.States[0].Key = g.States[0].Key[:31] }},
		{"state long key", func(g *types.GenesisState) { g.States[0].Key = append(g.States[0].Key, 0) }},
		{"state short value", func(g *types.GenesisState) { g.States[0].Value = g.States[0].Value[:31] }},
		{"state long value", func(g *types.GenesisState) { g.States[0].Value = append(g.States[0].Value, 0) }},
		{"nonce nil", func(g *types.GenesisState) { g.Nonces = append(g.Nonces, nil) }},
		{"nonce address", func(g *types.GenesisState) { g.Nonces[0].Address = "" }},
		{"serialized nil", func(g *types.GenesisState) { g.Serialized = append(g.Serialized, nil) }},
		{"serialized key", func(g *types.GenesisState) { g.Serialized = append(g.Serialized, &types.Serialized{Value: []byte{1}}) }},
		{"serialized value", func(g *types.GenesisState) { g.Serialized = append(g.Serialized, &types.Serialized{Prefix: []byte{1}}) }},
		{"serialized fee key", func(g *types.GenesisState) {
			g.Serialized = append(g.Serialized, &types.Serialized{Prefix: types.AccountFeeDenomKeyPrefix, Key: []byte{1}, Value: []byte("uhpx")})
		}},
		{"serialized fee denom", func(g *types.GenesisState) {
			g.Serialized = append(g.Serialized, &types.Serialized{Prefix: types.AccountFeeDenomKeyPrefix, Key: make([]byte, 20), Value: []byte("!")})
		}},
		{"serialized binding", func(g *types.GenesisState) {
			g.Serialized = append(g.Serialized, &types.Serialized{Prefix: types.LayerXBindNonceKeyPrefix, Key: make([]byte, 20), Value: []byte{1}})
		}},
		{"invalid parameters", func(g *types.GenesisState) { g.Params.PriorityNormalizer = sdk.NewDec(-1) }},
		{"partial parameters", func(g *types.GenesisState) { g.Params.MaximumFeePerGas = sdk.Dec{} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var invalid types.GenesisState
			require.NoError(t, cdc.UnmarshalAsJSON(wire, &invalid))
			tc.mutate(&invalid)
			require.Error(t, invalid.Validate())
			chunks := make(chan types.GenesisState, 3)
			chunks <- *valid
			chunks <- invalid
			chunks <- *valid
			close(chunks)
			require.Error(t, types.ValidateStream(chunks))
		})
	}
}

func TestRepairM_1245_021_StreamTransport(t *testing.T) {
	valid := repairM1245Genesis(t)
	cdc := repairM1245Codec()
	basic := evm.AppModuleBasic{}
	validWire, err := cdc.MarshalAsJSON(valid)
	require.NoError(t, err)
	run := func(chunks ...json.RawMessage) error {
		ch := make(chan json.RawMessage, len(chunks))
		for _, chunk := range chunks {
			ch <- chunk
		}
		close(ch)
		return basic.ValidateGenesisStream(cdc, nil, ch)
	}
	var dataOnly types.GenesisState
	require.NoError(t, cdc.UnmarshalAsJSON(validWire, &dataOnly))
	dataOnly.Params = types.Params{}
	dataWire, err := cdc.MarshalAsJSON(&dataOnly)
	require.NoError(t, err)
	require.NoError(t, run(validWire, dataWire, validWire))
	require.NoError(t, run(dataWire, validWire))
	require.Error(t, run())
	require.Error(t, run(dataWire))
	for _, mutate := range []func(*types.GenesisState){
		func(g *types.GenesisState) { g.Params.PriorityNormalizer = sdk.NewDec(-1) },
		func(g *types.GenesisState) { g.Codes[0].Address = "0x1" },
		func(g *types.GenesisState) { g.AddressAssociations[0].PaxAddress = "malformed" },
	} {
		var invalid types.GenesisState
		require.NoError(t, cdc.UnmarshalAsJSON(validWire, &invalid))
		mutate(&invalid)
		invalidWire, err := cdc.MarshalAsJSON(&invalid)
		require.NoError(t, err)
		require.Error(t, basic.ValidateGenesis(cdc, nil, invalidWire))
		require.Error(t, run(validWire, invalidWire, validWire))
		require.Error(t, run(invalidWire, validWire))
	}
	dataOnly.Codes[0].Address = "bad"
	dataWire, err = cdc.MarshalAsJSON(&dataOnly)
	require.NoError(t, err)
	require.Error(t, run(validWire, dataWire, validWire))
}

func TestRepairM_1245_023_MessageServiceRecipient(t *testing.T) {
	a := testkeeper.EVMTestApp
	ctx, _ := a.GetContextForDeliverTx(nil).CacheContext()
	fromEVM := common.HexToAddress("0x1111111111111111111111111111111111111111")
	to := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	from := sdk.AccAddress(fromEVM.Bytes())
	recipient := sdk.AccAddress(common.HexToAddress("0x2222222222222222222222222222222222222222").Bytes())
	k := &a.EvmKeeper
	k.SetAddressMapping(ctx, from, fromEVM)
	k.SetAddressMapping(ctx, recipient, to)
	bank := k.BankKeeper()
	require.NoError(t, bank.AddCoins(ctx, from, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 100)), true))
	cdc := repairM1245Codec()
	route := a.MsgServiceRouter().Handler(&types.MsgSend{})
	require.NotNil(t, route)
	for _, destination := range []string{"", "not-hex", "0x", "0x1", "0x" + strings.Repeat("a", 39), "0x" + strings.Repeat("a", 41), "0x" + strings.Repeat("a", 42), strings.Repeat("a", 64), "0x" + strings.Repeat("g", 40), " " + to.Hex(), to.Hex() + " "} {
		t.Run(destination, func(t *testing.T) {
			msg := types.NewMsgSend(from, to, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 5)))
			msg.ToAddress = destination
			wire, err := cdc.MarshalAsJSON(msg)
			require.NoError(t, err)
			var received types.MsgSend
			require.NoError(t, cdc.UnmarshalAsJSON(wire, &received))
			require.ErrorIs(t, received.ValidateBasic(), sdkerrors.ErrInvalidAddress)
			beforeFrom := bank.GetBalance(ctx, from, "uhpx")
			beforeRecipient := bank.GetBalance(ctx, recipient, "uhpx")
			normalized := k.GetPaxAddressOrDefault(ctx, common.HexToAddress(destination))
			beforeNormalized := bank.GetBalance(ctx, normalized, "uhpx")
			result, err := route(ctx, &received)
			require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
			require.Nil(t, result)
			require.Equal(t, beforeFrom, bank.GetBalance(ctx, from, "uhpx"))
			require.Equal(t, beforeRecipient, bank.GetBalance(ctx, recipient, "uhpx"))
			require.Equal(t, beforeNormalized, bank.GetBalance(ctx, normalized, "uhpx"))
		})
	}
	for _, destination := range []string{to.Hex(), strings.ToLower(to.Hex()), strings.TrimPrefix(to.Hex(), "0x"), "0X" + strings.TrimPrefix(to.Hex(), "0x")} {
		msg := types.NewMsgSend(from, to, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 5)))
		msg.ToAddress = destination
		wire, err := proto.Marshal(msg)
		require.NoError(t, err)
		var received types.MsgSend
		require.NoError(t, proto.Unmarshal(wire, &received))
		require.NoError(t, received.ValidateBasic())
		beforeFrom := bank.GetBalance(ctx, from, "uhpx").Amount
		beforeRecipient := bank.GetBalance(ctx, recipient, "uhpx").Amount
		_, err = route(ctx, &received)
		require.NoError(t, err)
		require.Equal(t, beforeFrom.Sub(sdk.NewInt(5)), bank.GetBalance(ctx, from, "uhpx").Amount)
		require.Equal(t, beforeRecipient.Add(sdk.NewInt(5)), bank.GetBalance(ctx, recipient, "uhpx").Amount)
	}
}
