package keeper

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func kernelVectorBytes(t *testing.T, source, symbol string) []byte {
	t.Helper()
	pattern := `(?m)^static const uint8_t ` + regexp.QuoteMeta(symbol) +
		`\[\] = \{\s*((?:0x[0-9a-fA-F]{2}U,\s*)*0x[0-9a-fA-F]{2}U\s*)\};`
	arrays := regexp.MustCompile(pattern).FindAllStringSubmatch(source, -1)
	require.Len(t, arrays, 1, "%s must have exactly one canonical byte array", symbol)
	var encoded strings.Builder
	for _, token := range regexp.MustCompile(`0x([0-9a-fA-F]{2})U`).FindAllStringSubmatch(arrays[0][1], -1) {
		encoded.WriteString(token[1])
	}
	decoded, err := hex.DecodeString(encoded.String())
	require.NoError(t, err)
	return decoded
}

type kernelProgramFixture struct {
	suite       *programFulfilSuite
	observation types.Observation
	outsider    []byte
	highS       []byte
	digest      types.Hash32
}

func newKernelProgramFixture(t *testing.T) kernelProgramFixture {
	t.Helper()
	source, err := os.ReadFile("../../../tests/modules/test_web_intake.c")
	require.NoError(t, err)
	vector := func(symbol string, size int) []byte {
		value := kernelVectorBytes(t, string(source), symbol)
		require.Len(t, value, size, symbol)
		return value
	}
	s := newProgramFulfilSuite(t, 3)
	params := s.k.GetParams(s.ctx)
	params.KernelNetworkId = 9
	require.NoError(t, s.k.SetParams(s.ctx, params))
	set := types.AttestorSet{Threshold: 2}
	var signatures [][]byte
	for i := 0; i < 3; i++ {
		set.Attestors = append(set.Attestors, types.Attestor{
			Signer: types.Address20(vector(fmt.Sprintf("web_signer_%d", i), 20)),
			Payout: sdk.AccAddress(bytes.Repeat([]byte{byte(0x10 + i)}, 32)).String(),
		})
		signatures = append(signatures, vector(fmt.Sprintf("web_first_sig_%d", i), types.SignatureLength))
	}
	require.NoError(t, set.Validate())
	s.k.setAttestorSet(s.ctx, set)
	s.set = set
	outsider := types.Address20(vector("web_outsider", 20))
	_, found := set.Find(outsider)
	require.False(t, found)
	member, found := set.Find(set.Attestors[1].Signer)
	require.True(t, found)
	require.Equal(t, set.Attestors[1], member)
	var programID types.Hash32
	for i := range programID {
		programID[i] = byte(0xa0 + i)
	}
	observation := types.Observation{
		Origin: types.OriginProgram, NetworkID: 9, ProgramID: programID,
		RequestID: 0x0102030405060708, Kind: types.KindFetch,
		PayloadHash:   types.Hash32(vector("web_payload_hash", 32)),
		ContentDigest: types.Hash32(vector("web_content_digest", 32)),
		Response:      []byte("Paxeer X Network"), FullLength: uint32(len("Paxeer X Network")),
		Signatures: signatures,
	}
	require.Equal(t, types.Keccak([]byte("https://paxeer.app/")), observation.PayloadHash)
	record := ProgramPendingRecord{
		Kind: types.KindFetch, PayloadHash: observation.PayloadHash,
		Amount: sdk.ZeroInt(), Sequence: 40,
	}
	encoded, err := record.Bytes()
	require.NoError(t, err)
	s.k.setProgramPendingRecord(s.ctx, ProgramPendingKey(programID, observation.RequestID), encoded)
	s.k.setProgramPendingCount(s.ctx, 1)
	require.False(t, s.pending(programID, observation.RequestID).Fulfilled)
	require.Zero(t, s.k.ProgramCommittedCount(s.ctx))
	return kernelProgramFixture{
		suite: s, observation: observation,
		outsider: vector("web_first_sig_outsider", types.SignatureLength),
		highS:    vector("web_first_sig_0_high_s", types.SignatureLength),
		digest:   types.Hash32(vector("web_first_digest", 32)),
	}
}

func TestFulfilProgramMatchesTheKernelRefusalMatrix(t *testing.T) {
	for _, row := range []struct {
		name   string
		code   error
		change func(*kernelProgramFixture, *types.Observation)
		raw    func([]byte) []byte
	}{
		{name: "unknown_request", code: errProgramUnknownField, change: func(_ *kernelProgramFixture, o *types.Observation) { o.RequestID++ }},
		{name: "wrong_network", code: errProgramWrongNetwork, change: func(_ *kernelProgramFixture, o *types.Observation) { o.NetworkID++ }},
		{name: "wrong_kind", code: errProgramContextMismatch, change: func(_ *kernelProgramFixture, o *types.Observation) { o.Kind = types.KindSearch }},
		{name: "wrong_payload_hash", code: errProgramContextMismatch, change: func(_ *kernelProgramFixture, o *types.Observation) { o.PayloadHash[5] ^= 1 }},
		{name: "half_threshold", code: errProgramAttestationThreshold, change: func(f *kernelProgramFixture, _ *types.Observation) {
			set := f.suite.set
			set.Threshold = 1
			f.suite.k.setAttestorSet(f.suite.ctx, set)
		}},
		{name: "one_signature", code: errProgramAttestationThreshold, change: func(_ *kernelProgramFixture, o *types.Observation) { o.Signatures = o.Signatures[:1] }},
		{name: "swapped_signers", code: errProgramUnsortedSequence, change: func(_ *kernelProgramFixture, o *types.Observation) {
			o.Signatures = [][]byte{o.Signatures[1], o.Signatures[0]}
		}},
		{name: "duplicate_signer", code: errProgramDuplicateSigner, change: func(_ *kernelProgramFixture, o *types.Observation) {
			o.Signatures = [][]byte{o.Signatures[0], o.Signatures[0]}
		}},
		{name: "high_s", code: errProgramBadSignature, change: func(f *kernelProgramFixture, o *types.Observation) { o.Signatures = [][]byte{f.highS, o.Signatures[1]} }},
		{name: "v_29", code: errProgramBadSignature, change: func(_ *kernelProgramFixture, o *types.Observation) {
			o.Signatures = o.Signatures[:2]
			o.Signatures[0][64] = 29
		}},
		{name: "v_zero", code: errProgramBadSignature, change: func(_ *kernelProgramFixture, o *types.Observation) {
			o.Signatures = o.Signatures[:2]
			o.Signatures[0][64] = 0
		}},
		{name: "flipped_v", code: errProgramInvalidAttestation, change: func(_ *kernelProgramFixture, o *types.Observation) {
			o.Signatures = o.Signatures[:2]
			o.Signatures[0][64] = 55 - o.Signatures[0][64]
		}},
		{name: "outsider_second", code: errProgramInvalidAttestation, change: func(f *kernelProgramFixture, o *types.Observation) {
			o.Signatures = [][]byte{o.Signatures[0], f.outsider}
		}},
		{name: "outsider_third", code: errProgramInvalidAttestation, change: func(f *kernelProgramFixture, o *types.Observation) { o.Signatures[2] = f.outsider }},
		{name: "changed_content_digest", code: errProgramInvalidAttestation, change: func(_ *kernelProgramFixture, o *types.Observation) { o.ContentDigest[0] ^= 1 }},
		{name: "changed_response", code: errProgramInvalidAttestation, change: func(_ *kernelProgramFixture, o *types.Observation) { o.Response[0] ^= 1 }},
		{name: "changed_full_length", code: errProgramInvalidAttestation, change: func(_ *kernelProgramFixture, o *types.Observation) { o.FullLength++ }},
		{name: "trailing_byte", code: errProgramNonCanonical, raw: func(raw []byte) []byte { return append(raw, 0) }},
		{name: "nonzero_padding", code: errProgramNonCanonical, raw: func(raw []byte) []byte { raw[3] = 1; return raw }},
		{name: "evm_origin", code: errProgramNonCanonical, raw: func(raw []byte) []byte { raw[0] = types.OriginEVM; return raw }},
		{name: "response_over_bound", code: errProgramNonCanonical, raw: func(raw []byte) []byte {
			binary.BigEndian.PutUint32(raw[138:142], types.MaxResponseBytes+1)
			binary.BigEndian.PutUint32(raw[142:146], types.MaxResponseBytes+1)
			return raw
		}},
		{name: "full_length_below_response", code: errProgramNonCanonical, raw: func(raw []byte) []byte {
			binary.BigEndian.PutUint32(raw[138:142], uint32(len("Paxeer X Network")-1))
			return raw
		}},
	} {
		t.Run(row.name, func(t *testing.T) {
			fixture := newKernelProgramFixture(t)
			observation := fixture.observation
			if row.change != nil {
				row.change(&fixture, &observation)
			}
			raw, err := types.EncodeObservation(observation)
			require.NoError(t, err)
			if row.raw != nil {
				raw = row.raw(raw)
			}
			s := fixture.suite
			before := s.state()
			err = s.k.FulfilProgram(s.ctx, raw)
			require.Equal(t, before, s.state())
			require.Equal(t, row.code.Error(), programFulfilCode(err))
			require.Equal(t, uint64(1), s.k.ProgramPendingCount(s.ctx))
			require.Zero(t, s.k.ProgramCommittedCount(s.ctx))
			require.False(t, s.pending(fixture.observation.ProgramID, fixture.observation.RequestID).Fulfilled)
		})
	}
}

func TestFulfilProgramAcceptsTheKernelFirstObservation(t *testing.T) {
	fixture := newKernelProgramFixture(t)
	s, observation := fixture.suite, fixture.observation
	require.Equal(t, fixture.digest, programSignatureDigest(observation))
	signers, err := verifyProgramSignatures(s.k.GetAttestorSet(s.ctx), observation)
	require.NoError(t, err)
	require.Equal(t, []types.Address20{s.set.Attestors[0].Signer, s.set.Attestors[1].Signer, s.set.Attestors[2].Signer}, signers)
	require.Nil(t, s.answerPart(observation.ProgramID, observation.RequestID, 0))
	raw, err := types.EncodeObservation(observation)
	require.NoError(t, err)
	require.NoError(t, s.k.FulfilProgram(s.ctx, raw))
	require.Zero(t, s.k.ProgramPendingCount(s.ctx))
	require.Equal(t, uint64(1), s.k.ProgramCommittedCount(s.ctx))
	require.True(t, s.pending(observation.ProgramID, observation.RequestID).Fulfilled)
	header := s.answerPart(observation.ProgramID, observation.RequestID, 0)
	require.Len(t, header, ProgramAnswerHeaderBytes)
	require.Equal(t, observation.ContentDigest[:], header[:32])
	require.Equal(t, observation.FullLength, binary.BigEndian.Uint32(header[32:36]))
	require.Equal(t, uint32(len("Paxeer X Network")), binary.BigEndian.Uint32(header[36:40]))
	require.Equal(t, []byte("Paxeer X Network"), s.answerPart(observation.ProgramID, observation.RequestID, 1))
	require.Nil(t, s.answerPart(observation.ProgramID, observation.RequestID, 2))
}
