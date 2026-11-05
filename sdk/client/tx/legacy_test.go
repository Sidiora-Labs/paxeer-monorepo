package tx_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	tx2 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/testdata"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	signing2 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/legacy/legacytx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
)

const (
	memo          = "waboom"
	gas           = uint64(10000)
	gasEstimate   = uint64(7500)
	timeoutHeight = uint64(5)
)

var (
	fee            = types.NewCoins(types.NewInt64Coin("bam", 100))
	_, pub1, addr1 = testdata.KeyTestPubAddr()
	_, _, addr2    = testdata.KeyTestPubAddr()
	sig            = signing2.SignatureV2{
		PubKey: pub1,
		Data: &signing2.SingleSignatureData{
			SignMode:  signing2.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
			Signature: []byte("dummy"),
		},
	}
	msg0 = banktypes.NewMsgSend(addr1, addr2, types.NewCoins(types.NewInt64Coin("wack", 1)))
	msg1 = banktypes.NewMsgSend(addr1, addr2, types.NewCoins(types.NewInt64Coin("wack", 2)))
)

func buildTestTx(t *testing.T, builder client.TxBuilder) {
	builder.SetMemo(memo)
	builder.SetGasLimit(gas)
	builder.SetGasEstimate(gasEstimate)
	builder.SetFeeAmount(fee)
	err := builder.SetMsgs(msg0, msg1)
	require.NoError(t, err)
	err = builder.SetSignatures(sig)
	require.NoError(t, err)
	builder.SetTimeoutHeight(timeoutHeight)
}

type TestSuite struct {
	suite.Suite
	encCfg   txTestEncodingConfig
	protoCfg client.TxConfig
	aminoCfg client.TxConfig
}

func (s *TestSuite) SetupSuite() {
	encCfg := makeTxEncodingConfig()
	s.encCfg = encCfg
	s.protoCfg = tx.NewTxConfig(codec.NewProtoCodec(encCfg.InterfaceRegistry), tx.DefaultSignModes)
	s.aminoCfg = legacytx.StdTxConfig{Cdc: encCfg.Amino}
}

func (s *TestSuite) TestCopyTx() {
	// proto -> amino -> proto
	protoBuilder := s.protoCfg.NewTxBuilder()
	buildTestTx(s.T(), protoBuilder)
	aminoBuilder := s.aminoCfg.NewTxBuilder()
	err := tx2.CopyTx(protoBuilder.GetTx(), aminoBuilder, false)
	s.Require().NoError(err)
	protoBuilder2 := s.protoCfg.NewTxBuilder()
	err = tx2.CopyTx(aminoBuilder.GetTx(), protoBuilder2, false)
	s.Require().NoError(err)
	// Check sigs, signers and msgs.
	sigsV2_1, err := protoBuilder.GetTx().GetSignaturesV2()
	s.Require().NoError(err)
	sigsV2_2, err := protoBuilder2.GetTx().GetSignaturesV2()
	s.Require().NoError(err)
	s.Require().Equal(sigsV2_1, sigsV2_2)
	s.Require().Equal(protoBuilder.GetTx().GetSigners(), protoBuilder2.GetTx().GetSigners())
	s.Require().Equal(protoBuilder.GetTx().GetMsgs()[0], protoBuilder2.GetTx().GetMsgs()[0])
	s.Require().Equal(protoBuilder.GetTx().GetMsgs()[1], protoBuilder2.GetTx().GetMsgs()[1])
	s.Require().Equal(gasEstimate, aminoBuilder.GetTx().GetGasEstimate())
	s.Require().Equal(gasEstimate, protoBuilder2.GetTx().GetGasEstimate())

	// amino -> proto -> amino
	aminoBuilder = s.aminoCfg.NewTxBuilder()
	buildTestTx(s.T(), aminoBuilder)
	protoBuilder = s.protoCfg.NewTxBuilder()
	err = tx2.CopyTx(aminoBuilder.GetTx(), protoBuilder, false)
	s.Require().NoError(err)
	aminoBuilder2 := s.aminoCfg.NewTxBuilder()
	err = tx2.CopyTx(protoBuilder.GetTx(), aminoBuilder2, false)
	s.Require().NoError(err)
	// Check sigs, signers, and msgs
	sigsV2_1, err = aminoBuilder.GetTx().GetSignaturesV2()
	s.Require().NoError(err)
	sigsV2_2, err = aminoBuilder2.GetTx().GetSignaturesV2()
	s.Require().NoError(err)
	s.Require().Equal(sigsV2_1, sigsV2_2)
	s.Require().Equal(aminoBuilder.GetTx().GetSigners(), aminoBuilder2.GetTx().GetSigners())
	s.Require().Equal(aminoBuilder.GetTx().GetMsgs()[0], aminoBuilder2.GetTx().GetMsgs()[0])
	s.Require().Equal(aminoBuilder.GetTx().GetMsgs()[1], aminoBuilder2.GetTx().GetMsgs()[1])
	s.Require().Equal(gasEstimate, protoBuilder.GetTx().GetGasEstimate())
	s.Require().Equal(gasEstimate, aminoBuilder2.GetTx().GetGasEstimate())
}

func (s *TestSuite) TestCopyTxProtobufFeeFields() {
	source := s.protoCfg.NewTxBuilder()
	buildTestTx(s.T(), source)
	source.SetFeeGranter(addr2)
	target := s.protoCfg.NewTxBuilder()
	s.Require().NoError(tx2.CopyTx(source.GetTx(), target, false))
	s.Require().Equal(addr2, target.GetTx().FeeGranter())
	s.Require().Equal(gasEstimate, target.GetTx().GetGasEstimate())
	s.Require().Equal(fee, target.GetTx().GetFee())
	s.Require().Equal(gas, target.GetTx().GetGas())
}

func (s *TestSuite) TestCopyTxUnsupportedFeeGranter() {
	source := s.protoCfg.NewTxBuilder()
	buildTestTx(s.T(), source)
	source.SetFeeGranter(addr2)
	for _, ignoreSignatureError := range []bool{false, true} {
		target := s.aminoCfg.NewTxBuilder()
		s.Require().EqualError(tx2.CopyTx(source.GetTx(), target, ignoreSignatureError), "cannot copy fee granter to legacy StdTx")
	}
	_, err := tx2.ConvertTxToStdTx(s.encCfg.Amino, source.GetTx())
	s.Require().EqualError(err, "cannot copy fee granter to legacy StdTx")
}

func (s *TestSuite) TestConvertTxToStdTx() {
	// proto tx
	protoBuilder := s.protoCfg.NewTxBuilder()
	buildTestTx(s.T(), protoBuilder)
	stdTx, err := tx2.ConvertTxToStdTx(s.encCfg.Amino, protoBuilder.GetTx())
	s.Require().NoError(err)
	s.Require().Equal(memo, stdTx.Memo)
	s.Require().Equal(gas, stdTx.Fee.Gas)
	s.Require().Equal(gasEstimate, stdTx.GetGasEstimate())
	s.Require().Equal(fee, stdTx.Fee.Amount)
	s.Require().Equal(msg0, stdTx.Msgs[0])
	s.Require().Equal(msg1, stdTx.Msgs[1])
	s.Require().Equal(timeoutHeight, stdTx.TimeoutHeight)
	s.Require().Equal(sig.PubKey, stdTx.Signatures[0].PubKey)
	s.Require().Equal(sig.Data.(*signing2.SingleSignatureData).Signature, stdTx.Signatures[0].Signature)

	// SIGN_MODE_DIRECT should fall back to an unsigned tx
	err = protoBuilder.SetSignatures(signing2.SignatureV2{
		PubKey: pub1,
		Data: &signing2.SingleSignatureData{
			SignMode:  signing2.SignMode_SIGN_MODE_DIRECT,
			Signature: []byte("dummy"),
		},
	})
	s.Require().NoError(err)
	stdTx, err = tx2.ConvertTxToStdTx(s.encCfg.Amino, protoBuilder.GetTx())
	s.Require().NoError(err)
	s.Require().Equal(memo, stdTx.Memo)
	s.Require().Equal(gas, stdTx.Fee.Gas)
	s.Require().Equal(gasEstimate, stdTx.GetGasEstimate())
	s.Require().Equal(fee, stdTx.Fee.Amount)
	s.Require().Equal(msg0, stdTx.Msgs[0])
	s.Require().Equal(msg1, stdTx.Msgs[1])
	s.Require().Equal(timeoutHeight, stdTx.TimeoutHeight)
	s.Require().Empty(stdTx.Signatures)

	// std tx
	aminoBuilder := s.aminoCfg.NewTxBuilder()
	buildTestTx(s.T(), aminoBuilder)
	stdTx = aminoBuilder.GetTx().(legacytx.StdTx)
	stdTx2, err := tx2.ConvertTxToStdTx(s.encCfg.Amino, stdTx)
	s.Require().NoError(err)
	s.Require().Equal(stdTx, stdTx2)
}

func TestTestSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}
