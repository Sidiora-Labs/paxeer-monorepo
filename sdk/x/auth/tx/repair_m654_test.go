package tx

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/node"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/testdata"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/query"
	txtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
)

func testdataPubKeyM654(t *testing.T) cryptotypes.PubKey {
	t.Helper()
	_, pub, _ := testdata.KeyTestPubAddr()
	return pub
}

func repairM654Config() client.TxConfig {
	registry := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(registry)
	testdata.RegisterInterfaces(registry)
	return NewTxConfig(codec.NewProtoCodec(registry), DefaultSignModes)
}

func repairM654Decode(t *testing.T, cfg client.TxConfig, infos []*txtypes.SignerInfo, sigs [][]byte) authsigning.Tx {
	t.Helper()
	body, err := (&txtypes.TxBody{Memo: "repair m654 wire regression"}).Marshal()
	require.NoError(t, err)
	auth, err := (&txtypes.AuthInfo{SignerInfos: infos, Fee: &txtypes.Fee{GasLimit: 200000}}).Marshal()
	require.NoError(t, err)
	wire, err := (&txtypes.TxRaw{BodyBytes: body, AuthInfoBytes: auth, Signatures: sigs}).Marshal()
	require.NoError(t, err)
	decoded, err := cfg.TxDecoder()(wire)
	require.NoError(t, err)
	wrapped, err := cfg.WrapTxBuilder(decoded)
	require.NoError(t, err)
	return wrapped.GetTx()
}

func repairM654DecodeError(t *testing.T, tx authsigning.Tx) {
	t.Helper()
	require.NotPanics(t, func() {
		sigs, err := tx.GetSignaturesV2()
		require.Error(t, err)
		require.True(t, sdkerrors.ErrTxDecode.Is(err), "expected typed transaction decode error: %v", err)
		require.Nil(t, sigs)
	})
}

func TestRepairM_654_WrapperSignatureCardinality(t *testing.T) {
	cfg := repairM654Config()
	builder := cfg.NewTxBuilder()
	builder.SetMemo("repair m654 wire regression")
	builder.SetGasLimit(200000)
	keys := make([]cryptotypes.PrivKey, 2)
	sigs := make([]signing.SignatureV2, len(keys))
	for i := range keys {
		keys[i], _, _ = testdata.KeyTestPubAddr()
		sigs[i] = signing.SignatureV2{PubKey: keys[i].PubKey(), Sequence: uint64(i + 1), Data: &signing.SingleSignatureData{SignMode: signing.SignMode_SIGN_MODE_DIRECT}}
	}
	require.NoError(t, builder.SetSignatures(sigs...))
	for i := range keys {
		bz, err := cfg.SignModeHandler().GetSignBytes(signing.SignMode_SIGN_MODE_DIRECT, authsigning.SignerData{ChainID: "repair-m654", AccountNumber: uint64(i + 1)}, builder.GetTx())
		require.NoError(t, err)
		sigs[i].Data.(*signing.SingleSignatureData).Signature, err = keys[i].Sign(bz)
		require.NoError(t, err)
	}
	require.NoError(t, builder.SetSignatures(sigs...))
	encoded, err := cfg.TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	var raw txtypes.TxRaw
	require.NoError(t, raw.Unmarshal(encoded))
	var auth txtypes.AuthInfo
	require.NoError(t, auth.Unmarshal(raw.AuthInfoBytes))

	for _, tc := range []struct {
		name  string
		infos []*txtypes.SignerInfo
		sigs  [][]byte
	}{
		{"short", auth.SignerInfos, raw.Signatures[:1]},
		{"missing", auth.SignerInfos, nil},
		{"extra", auth.SignerInfos[:1], raw.Signatures},
		{"orphan", nil, raw.Signatures[:1]},
		{"mixed_unsigned_short", []*txtypes.SignerInfo{{}, auth.SignerInfos[1]}, nil},
		{"signature_without_mode", []*txtypes.SignerInfo{{}}, raw.Signatures[:1]},
	} {
		t.Run(tc.name, func(t *testing.T) { repairM654DecodeError(t, repairM654Decode(t, cfg, tc.infos, tc.sigs)) })
	}
	t.Run("nil_signer_info", func(t *testing.T) {
		malformed := WrapTx(&txtypes.Tx{AuthInfo: &txtypes.AuthInfo{SignerInfos: []*txtypes.SignerInfo{nil}}, Signatures: raw.Signatures[:1]}).GetTx()
		repairM654DecodeError(t, malformed)
	})
	for name, malformed := range map[string]*txtypes.Tx{"nil_tx": nil, "nil_auth": {}} {
		t.Run(name, func(t *testing.T) { repairM654DecodeError(t, WrapTx(malformed).GetTx()) })
	}

	t.Run("signed_roundtrip", func(t *testing.T) {
		decoded := repairM654Decode(t, cfg, auth.SignerInfos, raw.Signatures)
		got, err := decoded.GetSignaturesV2()
		require.NoError(t, err)
		require.Equal(t, sigs, got)
		for i := range keys {
			require.NoError(t, authsigning.VerifySignature(keys[i].PubKey(), authsigning.SignerData{ChainID: "repair-m654", AccountNumber: uint64(i + 1)}, got[i].Data, cfg.SignModeHandler(), decoded))
		}
	})
	for _, tc := range []struct {
		name  string
		infos []*txtypes.SignerInfo
		sigs  [][]byte
	}{
		{"empty_unsigned", nil, nil},
		{"unsigned_simulation", []*txtypes.SignerInfo{{}, {}}, nil},
		{"simulation_slots", []*txtypes.SignerInfo{{}, {}}, [][]byte{nil, nil}},
		{"mode_with_empty_signature", auth.SignerInfos[:1], [][]byte{nil}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repairM654Decode(t, cfg, tc.infos, tc.sigs).GetSignaturesV2()
			require.NoError(t, err)
			require.Len(t, got, len(tc.infos))
			for i := range got {
				if tc.infos[i].ModeInfo == nil {
					require.Nil(t, got[i].Data)
				} else {
					require.Empty(t, got[i].Data.(*signing.SingleSignatureData).Signature)
				}
			}
		})
	}
}

func TestRepairM_654_MultisigModeShape(t *testing.T) {
	cfg := repairM654Config()
	one := func() *txtypes.ModeInfo {
		return &txtypes.ModeInfo{Sum: &txtypes.ModeInfo_Single_{Single: &txtypes.ModeInfo_Single{Mode: signing.SignMode_SIGN_MODE_DIRECT}}}
	}
	multi := func(ba *cryptotypes.CompactBitArray, modes ...*txtypes.ModeInfo) *txtypes.ModeInfo {
		return &txtypes.ModeInfo{Sum: &txtypes.ModeInfo_Multi_{Multi: &txtypes.ModeInfo_Multi{Bitarray: ba, ModeInfos: modes}}}
	}
	bits := func(n int) *cryptotypes.CompactBitArray {
		ba := cryptotypes.NewCompactBitArray(n)
		for i := 0; i < n; i++ {
			require.True(t, ba.SetIndex(i, true))
		}
		return ba
	}
	wireSigs := func(sigs ...[]byte) []byte {
		bz, err := (&cryptotypes.MultiSignature{Signatures: sigs}).Marshal()
		require.NoError(t, err)
		return bz
	}
	for _, tc := range []struct {
		name string
		mode *txtypes.ModeInfo
		sig  []byte
	}{
		{"missing_signature", multi(bits(2), one(), one()), wireSigs([]byte("one"))},
		{"extra_signature", multi(bits(1), one()), wireSigs([]byte("one"), []byte("two"))},
		{"missing_modes", multi(bits(1)), wireSigs()},
		{"too_many_modes", multi(bits(1), one(), one()), wireSigs([]byte("one"), []byte("two"))},
		{"empty_mode", &txtypes.ModeInfo{}, nil},
		{"nil_nested_discriminant", multi(bits(1), &txtypes.ModeInfo{}), wireSigs([]byte("one"))},
		{"nil_bitarray", multi(nil, one()), wireSigs([]byte("one"))},
		{"empty_bitarray", multi(&cryptotypes.CompactBitArray{}, one()), wireSigs([]byte("one"))},
		{"missing_bitarray_elems", multi(&cryptotypes.CompactBitArray{ExtraBitsStored: 1}, one()), wireSigs([]byte("one"))},
		{"invalid_extra_bits", multi(&cryptotypes.CompactBitArray{ExtraBitsStored: 9, Elems: []byte{0x80}}, one()), wireSigs([]byte("one"))},
		{"noncanonical_eight_bits", multi(&cryptotypes.CompactBitArray{ExtraBitsStored: 8, Elems: []byte{0x80}}, one()), wireSigs([]byte("one"))},
		{"padding_bits", multi(&cryptotypes.CompactBitArray{ExtraBitsStored: 1, Elems: []byte{0x81}}, one()), wireSigs([]byte("one"))},
		{"invalid_multisig_wire", multi(bits(1), one()), []byte{0xff}},
		{"unknown_multisig_field", multi(bits(1), one()), append(wireSigs([]byte("one")), 0x10, 0x01)},
		{"nested_missing_signature", multi(bits(1), multi(bits(2), one(), one())), wireSigs(wireSigs([]byte("one")))},
		{"unknown_sign_mode", &txtypes.ModeInfo{Sum: &txtypes.ModeInfo_Single_{Single: &txtypes.ModeInfo_Single{Mode: signing.SignMode(12345)}}}, []byte("one")},
		{"unspecified_sign_mode", &txtypes.ModeInfo{Sum: &txtypes.ModeInfo_Single_{Single: &txtypes.ModeInfo_Single{}}}, []byte("one")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decoded := repairM654Decode(t, cfg, []*txtypes.SignerInfo{{ModeInfo: tc.mode}}, [][]byte{tc.sig})
			repairM654DecodeError(t, decoded)
		})
	}
	for name, mode := range map[string]*txtypes.ModeInfo{
		"nil_mode":           nil,
		"nil_single_wrapper": {Sum: (*txtypes.ModeInfo_Single_)(nil)},
		"nil_single":         {Sum: &txtypes.ModeInfo_Single_{}},
		"nil_multi_wrapper":  {Sum: (*txtypes.ModeInfo_Multi_)(nil)},
		"nil_multi":          {Sum: &txtypes.ModeInfo_Multi_{}},
		"nil_nested_mode":    multi(bits(1), nil),
	} {
		t.Run(name, func(t *testing.T) {
			require.NotPanics(t, func() {
				data, err := ModeInfoAndSigToSignatureData(mode, wireSigs([]byte("one")))
				require.Nil(t, data)
				require.Error(t, err)
				require.True(t, sdkerrors.ErrTxDecode.Is(err), "%v", err)
			})
		})
	}
	t.Run("nested_producer_roundtrip", func(t *testing.T) {
		ba := cryptotypes.NewCompactBitArray(9)
		require.True(t, ba.SetIndex(0, true))
		require.True(t, ba.SetIndex(8, true))
		original := &signing.MultiSignatureData{BitArray: ba, Signatures: []signing.SignatureData{
			&signing.SingleSignatureData{SignMode: signing.SignMode_SIGN_MODE_DIRECT, Signature: []byte("first")},
			&signing.MultiSignatureData{BitArray: bits(1), Signatures: []signing.SignatureData{&signing.SingleSignatureData{SignMode: signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON, Signature: []byte("nested")}}},
		}}
		builder := cfg.NewTxBuilder()
		require.NoError(t, builder.SetSignatures(signing.SignatureV2{PubKey: testdataPubKeyM654(t), Data: original, Sequence: 11}))
		encoded, err := cfg.TxEncoder()(builder.GetTx())
		require.NoError(t, err)
		decoded, err := cfg.TxDecoder()(encoded)
		require.NoError(t, err)
		wrapped, err := cfg.WrapTxBuilder(decoded)
		require.NoError(t, err)
		got, err := wrapped.GetTx().GetSignaturesV2()
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, original, got[0].Data)
		require.Equal(t, uint64(11), got[0].Sequence)
	})
	for _, n := range []int{1, 8, 9} {
		ba := cryptotypes.NewCompactBitArray(n)
		mode, sig := SignatureDataToModeInfoAndSig(&signing.MultiSignatureData{BitArray: ba})
		got, err := ModeInfoAndSigToSignatureData(mode, sig)
		require.NoError(t, err)
		require.Equal(t, ba, got.(*signing.MultiSignatureData).BitArray)
		require.Empty(t, got.(*signing.MultiSignatureData).Signatures)
	}
}

func TestRepairM_654_EmptyBlockQuery(t *testing.T) {
	registry := codectypes.NewInterfaceRegistry()
	cfg := NewTxConfig(codec.NewProtoCodec(registry), DefaultSignModes)
	conf, err := tmconfig.ResetTestRoot(t.TempDir(), "repair-m654")
	require.NoError(t, err)
	conf.Mode = tmconfig.ModeSeed
	conf.P2P.PexReactor = true
	genesis, err := tmtypes.GenesisDocFromFile(conf.GenesisFile())
	require.NoError(t, err)
	seed, err := node.New(t.Context(), conf, func() {}, kvstore.NewApplication(), genesis, nil, node.NoOpMetricsProvider(), tmtypes.DefaultConsensusPolicy())
	require.NoError(t, err)
	t.Cleanup(seed.(interface{ OnStop() }).OnStop)
	localClient, err := local.New(seed)
	require.NoError(t, err)
	block := tmtypes.MakeBlock(1, nil, &tmtypes.Commit{}, nil)
	block.Header.ChainID = genesis.ChainID
	block.Header.Time = genesis.GenesisTime
	block.Header.ProposerAddress = genesis.Validators[0].PubKey.Address()
	block.Header.ValidatorsHash = tmtypes.NewValidatorSet([]*tmtypes.Validator{tmtypes.NewValidator(genesis.Validators[0].PubKey, genesis.Validators[0].Power)}).Hash()
	require.NoError(t, block.ValidateBasic(tmtypes.DefaultConsensusPolicy()))
	parts, err := block.MakePartSet(tmtypes.BlockPartSizeBytes)
	require.NoError(t, err)
	blockID := tmtypes.BlockID{Hash: block.Hash(), PartSetHeader: parts.Header()}
	store, ok := seed.RPCEnvironment().BlockStore.(interface {
		SaveBlock(*tmtypes.Block, *tmtypes.PartSet, *tmtypes.Commit)
	})
	require.True(t, ok)
	store.SaveBlock(block, parts, &tmtypes.Commit{Height: 1, BlockID: blockID})
	helper := baseapp.NewQueryServerTestHelper(sdk.NewContext(nil, tmproto.Header{Height: 1}, false), registry)
	RegisterTxService(helper, localClient, cfg, nil, registry)
	queryClient := txtypes.NewServiceClient(helper)
	for _, page := range []*query.PageRequest{nil, {}, {Limit: 1}, {Reverse: true}, {Limit: query.MaxLimit}} {
		response, err := queryClient.GetBlockWithTxs(context.Background(), &txtypes.GetBlockWithTxsRequest{Height: 1, Pagination: page})
		require.NoError(t, err)
		require.NotNil(t, response.BlockId)
		require.Equal(t, blockID.ToProto(), *response.BlockId)
		require.NotNil(t, response.Block)
		require.Equal(t, int64(1), response.Block.Header.Height)
		require.Equal(t, block.Header.ChainID, response.Block.Header.ChainID)
		require.Empty(t, response.Block.Data.Txs)
		require.Empty(t, response.Txs)
		require.NotNil(t, response.Pagination)
		require.Zero(t, response.Pagination.Total)
	}
	for _, offset := range []uint64{1, math.MaxUint64} {
		_, err := queryClient.GetBlockWithTxs(context.Background(), &txtypes.GetBlockWithTxsRequest{Height: 1, Pagination: &query.PageRequest{Offset: offset}})
		require.ErrorContains(t, err, "out of range")
	}
	_, err = queryClient.GetBlockWithTxs(context.Background(), &txtypes.GetBlockWithTxsRequest{Height: 1, Pagination: &query.PageRequest{Limit: query.MaxLimit + 1}})
	require.ErrorContains(t, err, "invalid pagination limit")
}
