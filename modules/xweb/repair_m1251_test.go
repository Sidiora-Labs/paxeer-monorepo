package xweb_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"maps"
	"math"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	bridgetypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/keeper"
	xwebtestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/testutil"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	xwebprecompile "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/xweb"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

type repairM1251Suite struct {
	t    *testing.T
	a    *app.App
	k    keeper.Keeper
	db   *state.DBImpl
	evm  *vm.EVM
	p    *xwebprecompile.Precompile
	keys []xwebtestutil.Attestor
}

func repairM1251NewSuite(t *testing.T) *repairM1251Suite {
	t.Helper()
	a := app.Setup(t, false, true, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	ctx := a.GetContextForDeliverTx(nil).WithBlockHeight(100).WithBlockTime(time.Unix(1_800_000_000, 0))
	k, ctx := xwebtestutil.NewKeeper(a, ctx)
	keys := xwebtestutil.Attestors(3)
	g := types.DefaultGenesis()
	g.Paused = false
	g.Params.Fee = sdk.NewInt(75)
	g.Params.KernelNetworkId = 7
	g.Params.ProgramFeeDenoms = []string{types.ProgramFeeDenomPAX}
	g.Attestors = xwebtestutil.Set(keys, 2)
	require.NoError(t, g.Validate())
	k.InitGenesis(ctx, *g)
	a.UpgradeKeeper.SetDone(ctx, keeper.ProgramOriginPlan)
	p := xwebprecompile.NewPrecompileWithKeeper(&k, a.BankKeeper, &a.EvmKeeper)
	db := state.NewDBImpl(ctx, &a.EvmKeeper, false)
	block, err := a.EvmKeeper.GetVMBlockContext(ctx, core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	contracts := maps.Clone(a.EvmKeeper.CustomPrecompiles(ctx))
	contracts[common.HexToAddress(xwebprecompile.XWebAddress)] = p
	cfg := evmtypes.DefaultChainConfig().EthereumConfig(a.EvmKeeper.ChainID(ctx))
	return &repairM1251Suite{t: t, a: a, k: k, db: db, p: p, keys: keys,
		evm: vm.NewEVM(*block, db, cfg, vm.Config{}, contracts)}
}

func (s *repairM1251Suite) call(name string, args ...interface{}) ([]interface{}, string) {
	s.t.Helper()
	input, err := s.p.GetABI().Pack(name, args...)
	require.NoError(s.t, err)
	ret, _, err := s.evm.Call(common.Address{0x51}, common.HexToAddress(xwebprecompile.XWebAddress),
		input, 2_000_000, uint256.NewInt(0))
	if err != nil {
		require.ErrorIs(s.t, err, vm.ErrExecutionReverted)
		reason, err := abi.UnpackRevert(ret)
		require.NoError(s.t, err)
		return nil, reason
	}
	out, err := s.p.GetABI().Methods[name].Outputs.Unpack(ret)
	require.NoError(s.t, err)
	return out, ""
}

func (s *repairM1251Suite) fund(account sdk.AccAddress, denom string, amount int64) {
	s.t.Helper()
	coins := sdk.NewCoins(sdk.NewInt64Coin(denom, amount))
	require.NoError(s.t, s.a.BankKeeper.MintCoins(s.db.Ctx(), "evm", coins))
	require.NoError(s.t, s.a.BankKeeper.SendCoinsFromModuleToAccount(s.db.Ctx(), "evm", account, coins))
}

func repairM1251Signed(t *testing.T, observation types.Observation, keys ...xwebtestutil.Attestor) []byte {
	t.Helper()
	digest := types.Digest(types.Attestation{Origin: observation.Origin,
		NetworkID: new(big.Int).SetUint64(uint64(observation.NetworkID)), Requester: observation.ProgramID,
		RequestID: observation.RequestID, Kind: observation.Kind, PayloadHash: observation.PayloadHash,
		ContentDigest: observation.ContentDigest, ResponseHash: types.Keccak(observation.Response), FullLength: observation.FullLength})
	observation.Signatures = xwebtestutil.Sign(digest, keys...)
	raw, err := types.EncodeObservation(observation)
	require.NoError(t, err)
	return raw
}

func TestRepairM_1251_ProgramEnforcement(t *testing.T) {
	s := repairM1251NewSuite(t)
	program, asset := types.Hash32{0x81}, types.Hash32{9}
	principal := sdk.AccAddress(bytes.Repeat([]byte{0x42}, 20))
	denom := types.ProgramFeeDenomPAX
	s.fund(principal, denom, 150)
	payload := []byte("paxeer x network")
	for _, kind := range []uint8{types.KindApi, types.KindStorageReceipt, types.KindFetch, types.KindSearch} {
		id := uint64(kind)
		data := binary.BigEndian.AppendUint64(nil, id)
		data = append(data, kind)
		data = binary.BigEndian.AppendUint32(data, uint32(len(payload)))
		data = append(data, payload...)
		cached, write := s.db.Ctx().CacheContext()
		coins := sdk.NewCoins(sdk.NewInt64Coin(denom, 75))
		require.NoError(t, s.a.BankKeeper.SendCoins(cached, principal, keeper.ProgramFeeAccount(asset), coins))
		call := &keeper.ProgramCall{ProgramID: program, Principal: principal, Sequence: id,
			Legs: []keeper.ProgramLeg{{From: principal, To: keeper.ProgramFeeAccount(asset), Asset: asset,
				Denom: denom, Amount: sdk.NewInt(75)}}}
		err := s.k.RequestFromProgram(cached, call, []byte(keeper.ProgramRequestTopic), data)
		if kind == types.KindApi || kind == types.KindStorageReceipt {
			require.EqualError(t, err, "LXP_NON_CANONICAL")
			_, found, err := s.k.GetProgramPendingRecord(s.db.Ctx(), program, id)
			require.NoError(t, err)
			require.False(t, found)
			continue
		}
		require.NoError(t, err)
		write()
		observation := types.Observation{Origin: types.OriginProgram, NetworkID: 7, ProgramID: program,
			RequestID: id, Kind: kind, PayloadHash: types.Keccak(payload),
			Response: bytes.Repeat([]byte("answer"), 200), FullLength: 2000, ContentDigest: types.Keccak([]byte("canonical answer"))}
		before := s.k.ExportGenesis(s.db.Ctx())
		for name, invalid := range map[string]struct {
			observation types.Observation
			keys        []xwebtestutil.Attestor
			reason      string
		}{
			"threshold": {observation, s.keys[:1], "LXP_ATTESTATION_THRESHOLD"},
			"duplicate": {observation, []xwebtestutil.Attestor{s.keys[0], s.keys[0]}, "LXP_AUTH_DUPLICATE_SIGNER"},
			"order":     {observation, []xwebtestutil.Attestor{s.keys[1], s.keys[0]}, "LXP_UNSORTED_SEQUENCE"},
			"network":   {func() types.Observation { o := observation; o.NetworkID++; return o }(), s.keys[:2], "LXP_WRONG_NETWORK"},
			"payload":   {func() types.Observation { o := observation; o.PayloadHash[0] ^= 1; return o }(), s.keys[:2], "LXP_CONTEXT_MISMATCH"},
		} {
			t.Run(name+"/"+big.NewInt(int64(kind)).String(), func(t *testing.T) {
				_, reason := s.call(xwebprecompile.FulfilProgramMethod, repairM1251Signed(t, invalid.observation, invalid.keys...))
				require.Contains(t, reason, invalid.reason)
				require.Equal(t, before, s.k.ExportGenesis(s.db.Ctx()))
				require.Equal(t, sdk.NewInt(75), s.a.BankKeeper.GetBalance(s.db.Ctx(), keeper.ProgramFeeAccount(asset), denom).Amount)
			})
		}
		raw := repairM1251Signed(t, observation, s.keys[:2]...)
		_, reason := s.call(xwebprecompile.FulfilProgramMethod, raw)
		require.Empty(t, reason)
		out, reason := s.call(xwebprecompile.GetProgramResultMethod, [32]byte(program), id)
		require.Empty(t, reason)
		require.Equal(t, []interface{}{[32]byte(observation.ContentDigest), observation.FullLength,
			uint32(len(observation.Response)), observation.Response}, out)
		record, found, err := s.k.GetProgramPendingRecord(s.db.Ctx(), program, id)
		require.NoError(t, err)
		require.True(t, found)
		require.True(t, record.Fulfilled)
		require.Zero(t, s.k.ProgramPendingCount(s.db.Ctx()))
		require.Equal(t, id, s.k.ProgramCommittedCount(s.db.Ctx()))
		require.Equal(t, sdk.ZeroInt(), s.a.BankKeeper.GetBalance(s.db.Ctx(), keeper.ProgramFeeAccount(asset), denom).Amount)
		for i, share := range []int64{38, 37, 0} {
			require.Equal(t, sdk.NewInt(share*int64(id)), s.a.BankKeeper.GetBalance(s.db.Ctx(), s.keys[i].Payout, denom).Amount)
		}
		_, reason = s.call(xwebprecompile.FulfilProgramMethod, raw)
		require.Contains(t, reason, "LXP_SEQUENCE_REUSED")
	}
	out, reason := s.call(xwebprecompile.GetKernelNetworkIdMethod)
	require.Empty(t, reason)
	require.Equal(t, []interface{}{uint32(7)}, out)
	require.Equal(t, xwebtestutil.Set(s.keys, 2), s.k.GetAttestorSet(s.db.Ctx()))
}

func TestRepairM_1251_StorageReceiptPolicy(t *testing.T) {
	s := repairM1251NewSuite(t)
	requester := common.Address{0x41}
	ctx := s.db.Ctx()
	payer := s.a.EvmKeeper.GetPaxAddressOrDefault(ctx, requester)
	s.fund(payer, sdk.MustGetBaseDenom(), 150)
	digest := sha256.Sum256([]byte("stored object"))
	payload, err := types.EncodeStorageReceiptPayload(types.StorageReceiptPayload{ObjectDigest: digest, BaseURL: "https://storage.example/bucket"})
	require.NoError(t, err)
	require.Equal(t, digest[:], payload[:32])
	require.Equal(t, "https://storage.example/bucket", string(payload[32:]))
	for name, invalid := range map[string][]byte{
		"zero digest":        append(make([]byte, 32), payload[32:]...),
		"short digest":       payload[:31],
		"empty URL":          payload[:32],
		"http":               bytes.Replace(payload, []byte("https:"), []byte("http:"), 1),
		"no host":            append(bytes.Clone(payload[:32]), []byte("https:///bucket")...),
		"userinfo":           append(bytes.Clone(payload[:32]), []byte("https://user@storage.example/bucket")...),
		"query":              append(bytes.Clone(payload), '?'),
		"fragment":           append(bytes.Clone(payload), '#'),
		"trailing slash":     append(bytes.Clone(payload), '/'),
		"invalid UTF8":       append(bytes.Clone(payload), 0xff),
		"URL over 512 bytes": append(bytes.Clone(payload[:32]), []byte("https://storage.example/"+strings.Repeat("x", 513-len("https://storage.example/")))...),
	} {
		t.Run(name, func(t *testing.T) {
			before := s.k.ExportGenesis(ctx)
			id, err := s.k.Request(ctx, requester, types.KindStorageReceipt, invalid, 200_000, sdk.NewInt(75))
			require.ErrorIs(t, err, types.ErrInvalidPayload)
			require.Zero(t, id)
			require.Equal(t, before, s.k.ExportGenesis(ctx))
			require.Equal(t, sdk.NewInt(150), s.a.BankKeeper.GetBalance(ctx, payer, sdk.MustGetBaseDenom()).Amount)
		})
	}
	id, err := s.k.Request(ctx, requester, types.KindStorageReceipt, payload, 200_000, sdk.NewInt(75))
	require.NoError(t, err)
	request, found := s.k.GetRequest(ctx, id)
	require.True(t, found)
	require.Equal(t, types.LevelMajority, request.Level)
	require.Equal(t, types.Keccak(payload), request.PayloadHash)
	response := []byte("stored receipt response")
	content := types.Keccak([]byte("canonical receipt bytes"))
	require.NotEqual(t, types.Hash32(digest), content)
	attested := s.k.Attestation(ctx, request, response, content, uint32(len(response)))
	signatures := xwebtestutil.Sign(types.Digest(attested), s.keys[:2]...)
	for name, changed := range map[string]types.Attestation{
		"object digest as content": func() types.Attestation { a := attested; a.ContentDigest = types.Hash32(digest); return a }(),
		"different payload":        func() types.Attestation { a := attested; a.PayloadHash[0] ^= 1; return a }(),
		"different response":       func() types.Attestation { a := attested; a.ResponseHash[0] ^= 1; return a }(),
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := s.k.Fulfil(ctx, id, response, content, uint32(len(response)), xwebtestutil.Sign(types.Digest(changed), s.keys[:2]...))
			require.ErrorIs(t, err, types.ErrBadSignature)
			stored, _ := s.k.GetRequest(ctx, id)
			require.Equal(t, types.StatusPending, stored.Status)
			_, found := s.k.GetResult(ctx, id)
			require.False(t, found)
		})
	}
	_, _, err = s.k.Fulfil(ctx, id, response, content, uint32(len(response)), signatures[:1])
	require.ErrorIs(t, err, types.ErrBelowThreshold)
	_, result, err := s.k.Fulfil(ctx, id, response, content, uint32(len(response)), signatures)
	require.NoError(t, err)
	stored, found := s.k.GetResult(ctx, id)
	require.True(t, found)
	require.Equal(t, result, stored)
	require.Equal(t, content, stored.ContentDigest)
	require.Equal(t, response, stored.Response)
	require.Equal(t, []types.Address20{s.keys[0].Signer, s.keys[1].Signer}, stored.Signers)
	for i, share := range []int64{38, 37, 0} {
		require.Equal(t, sdk.NewInt(share), s.a.BankKeeper.GetBalance(ctx, s.keys[i].Payout, sdk.MustGetBaseDenom()).Amount)
	}
	_, _, err = s.k.Fulfil(ctx, id, response, content, uint32(len(response)), signatures)
	require.ErrorIs(t, err, types.ErrAlreadyFulfilled)
}

func TestRepairM_1251_FrozenVectorsAndDocumentation(t *testing.T) {
	doc, err := os.ReadFile("ATTESTATION.md")
	require.NoError(t, err)
	for _, claim := range []string{"keeper.Keeper.FulfilProgram", xwebprecompile.XWebAddress,
		"getProgramResult(bytes32,uint64)", "governance-managed xweb attestor set", "types.EncodeObservation",
		"| 1 | fetch | majority | majority |", "| 2 | search | majority | majority |",
		"| 3 | api | majority or single, selected by payload | unsupported |",
		"| 4 | storage receipt | majority | unsupported |", "1 to 512 bytes", "SHA-256", "ContentKind::Fetch",
		"text/plain", "188 bytes", "frozen compatibility vectors", "historical origin-1 test chain id"} {
		require.Contains(t, string(doc), claim)
	}
	var file struct {
		Domain  string `json:"domain"`
		Length  int    `json:"preimage_length"`
		Vectors []struct {
			Name         string `json:"name"`
			Origin       uint8  `json:"origin"`
			NetworkID    uint64 `json:"network_id"`
			Requester    string `json:"requester"`
			RequestID    uint64 `json:"request_id"`
			Kind         uint8  `json:"kind"`
			Payload      string `json:"payload_hex"`
			PayloadHash  string `json:"payload_hash"`
			Content      string `json:"content_digest"`
			Response     string `json:"response_hex"`
			ResponseHash string `json:"response_hash"`
			FullLength   uint32 `json:"full_length"`
			Preimage     string `json:"preimage"`
			Digest       string `json:"digest"`
			Signer       string `json:"signer"`
			Signature    string `json:"signature"`
		} `json:"vectors"`
	}
	raw, err := os.ReadFile("types/testdata/preimage-vectors.json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &file))
	require.Equal(t, types.Domain, file.Domain)
	require.Equal(t, 188, file.Length)
	require.Equal(t, 188, types.PreimageLength)
	require.Len(t, file.Vectors, 3)
	for _, v := range file.Vectors {
		t.Run(v.Name, func(t *testing.T) {
			payload, err := hexutil.Decode(v.Payload)
			require.NoError(t, err)
			response, err := hexutil.Decode(v.Response)
			require.NoError(t, err)
			a := types.Attestation{Origin: v.Origin, NetworkID: new(big.Int).SetUint64(v.NetworkID),
				Requester: types.Hash32(common.HexToHash(v.Requester)), RequestID: v.RequestID, Kind: v.Kind,
				PayloadHash: types.Keccak(payload), ContentDigest: types.Hash32(common.HexToHash(v.Content)),
				ResponseHash: types.Keccak(response), FullLength: v.FullLength}
			require.Equal(t, v.PayloadHash, a.PayloadHash.Hex())
			require.Equal(t, v.ResponseHash, a.ResponseHash.Hex())
			require.Equal(t, v.Preimage, hexutil.Encode(types.Preimage(a)))
			digest := types.Digest(a)
			require.Equal(t, v.Digest, digest.Hex())
			signature, err := hexutil.Decode(v.Signature)
			require.NoError(t, err)
			signer, err := bridgetypes.RecoverSigner(bridgetypes.Hash32(digest), signature)
			require.NoError(t, err)
			require.Equal(t, v.Signer, signer.Hex())
			for _, value := range []string{v.Name, v.Digest, v.Signature} {
				require.Contains(t, string(doc), value)
			}
		})
	}
}
