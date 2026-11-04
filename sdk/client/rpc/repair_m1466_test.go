package rpc_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	tmjson "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/json"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/legacy"
)

// The peer produces real blocks, part sets and Ed25519 precommit signatures,
// served by the production JSON-RPC codec over a loopback HTTP connection.
type repairM1466Peer struct {
	block  *coretypes.ResultBlock
	signed *coretypes.ResultCommit
	trust  rpc.BlockTrust
	mu     sync.Mutex
	calls  []string
}

func repairM1466Fixture(t *testing.T) *repairM1466Peer {
	t.Helper()
	keys := make(map[string]ed25519.SecretKey)
	var validators []*tmtypes.Validator
	for i := 0; i < 3; i++ {
		key := ed25519.TestSecretKey([]byte(fmt.Sprintf("repair-m1466-validator-%d", i)))
		keys[string(key.Public().Address())] = key
		validators = append(validators, tmtypes.NewValidator(key.Public(), 10))
	}
	vals := tmtypes.NewValidatorSet(validators)
	block := tmtypes.MakeBlock(7, []tmtypes.Tx{[]byte("authenticated transaction")}, &tmtypes.Commit{}, nil)
	block.ChainID = "repair-m1466"
	block.Time = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	block.ValidatorsHash = vals.Hash()
	block.NextValidatorsHash = vals.Hash()
	block.ProposerAddress = vals.Proposer.Address
	parts, err := block.MakePartSet(65536)
	require.NoError(t, err)
	id := tmtypes.BlockID{Hash: block.Hash(), PartSetHeader: parts.Header()}
	commit := &tmtypes.Commit{Height: block.Height, BlockID: id}
	for i, val := range vals.Validators {
		vote := &tmtypes.Vote{Type: tmproto.PrecommitType, Height: block.Height,
			BlockID: id, Timestamp: block.Time, ValidatorAddress: val.Address, ValidatorIndex: int32(i)}
		vote.Signature = utils.Some(keys[string(val.Address)].Sign(tmtypes.VoteSignBytes(block.ChainID, vote.ToProto())))
		commit.Signatures = append(commit.Signatures, vote.CommitSig())
	}
	require.NoError(t, vals.VerifyCommitLightAllSignatures(block.ChainID, id, block.Height, commit))
	header := block.Header
	return &repairM1466Peer{
		block:  &coretypes.ResultBlock{BlockID: id, Block: block},
		signed: &coretypes.ResultCommit{SignedHeader: tmtypes.SignedHeader{Header: &header, Commit: commit}, CanonicalCommit: true},
		trust:  rpc.BlockTrust{ChainID: block.ChainID, Height: block.Height, ValidatorSet: vals},
	}
}

func (p *repairM1466Peer) record(method string, req *coretypes.RequestBlockInfo) {
	height := "latest"
	if req.Height != nil {
		height = fmt.Sprint(*req.Height)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, method+":"+height)
}

func (p *repairM1466Peer) serve(t *testing.T) client.Context {
	t.Helper()
	mux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, map[string]*rpcserver.RPCFunc{
		"block": rpcserver.NewRPCFunc(func(ctx context.Context, req *coretypes.RequestBlockInfo) (*coretypes.ResultBlock, error) {
			p.record("block", req)
			return p.block, nil
		}),
		"commit": rpcserver.NewRPCFunc(func(ctx context.Context, req *coretypes.RequestBlockInfo) (*coretypes.ResultCommit, error) {
			p.record("commit", req)
			return p.signed, nil
		}),
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	node, err := rpchttp.NewWithClient(server.URL, server.Client())
	require.NoError(t, err)
	return client.Context{}.WithClient(node).WithChainID(p.trust.ChainID)
}

func repairM1466TrustFile(t *testing.T, trust rpc.BlockTrust) string {
	t.Helper()
	data, err := tmjson.Marshal(trust)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "trusted-validators.json")
	require.NoError(t, os.WriteFile(path, data, 0600))
	return path
}

func repairM1466Command(ctx client.Context, trustPath string, args ...string) (string, error) {
	cmd := rpc.BlockCommand()
	var out, diagnostic bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&diagnostic)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	argv := []string{}
	if trustPath != "" {
		argv = append(argv, "--trusted-validator-set", trustPath)
	}
	argv = append(argv, "--")
	cmd.SetArgs(append(argv, args...))
	err := cmd.ExecuteContext(context.WithValue(context.Background(), client.ClientContextKey, &ctx))
	return out.String(), err
}

func TestRepairM_1466_HeightConventions(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		request string
		invalid bool
	}{
		{name: "negative", args: []string{"-1"}, invalid: true},
		{name: "zero", args: []string{"0"}, request: "block:latest"},
		{name: "omitted", request: "block:latest"},
		{name: "historical", args: []string{"7"}, request: "block:7"},
		{name: "overflow", args: []string{"9223372036854775808"}, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer := repairM1466Fixture(t)
			ctx := peer.serve(t)
			output, err := repairM1466Command(ctx, repairM1466TrustFile(t, peer.trust), tc.args...)
			peer.mu.Lock()
			calls := append([]string(nil), peer.calls...)
			peer.mu.Unlock()
			if tc.invalid {
				require.Error(t, err)
				if tc.name == "negative" {
					require.Contains(t, err.Error(), "negative")
				}
				require.Empty(t, output)
				require.Empty(t, calls, "invalid heights must not call RPC")
				return
			}
			require.NoError(t, err)
			require.Equal(t, []string{tc.request, "commit:7"}, calls)
			var result coretypes.ResultBlock
			require.NoError(t, legacy.Cdc.UnmarshalAsJSON([]byte(output), &result))
			require.Equal(t, int64(7), result.Block.Height)
			require.Equal(t, peer.block.BlockID, result.BlockID)
			require.Equal(t, peer.block.Block.Txs, result.Block.Txs)
		})
	}
}

func TestRepairM_1466_RejectUnauthenticatedBlocks(t *testing.T) {
	for _, tc := range []struct {
		name    string
		alter   func(*repairM1466Peer)
		message string
	}{
		{"altered_header", func(p *repairM1466Peer) { p.block.Block.AppHash = []byte("altered") }, "does not match"},
		{"altered_signed_header", func(p *repairM1466Peer) { p.signed.Header.AppHash = []byte("altered") }, "invalid signed header"},
		{"missing_commit", func(p *repairM1466Peer) { p.signed.Commit = nil }, "missing commit"},
		{"missing_signed_header", func(p *repairM1466Peer) { p.signed.Header = nil }, "missing header"},
		{"missing_block", func(p *repairM1466Peer) { p.block.Block = nil }, "missing block"},
		{"wrong_height", func(p *repairM1466Peer) { p.block.Block.Height++ }, "height"},
		{"wrong_commit_height", func(p *repairM1466Peer) { p.signed.Commit.Height++ }, "height mismatch"},
		{"wrong_chain", func(p *repairM1466Peer) { p.signed.Header.ChainID = "another-chain" }, "another chain"},
		{"altered_transactions", func(p *repairM1466Peer) { p.block.Block.Txs[0] = []byte("forged transaction") }, "DataHash"},
		{"wrong_block_id", func(p *repairM1466Peer) { p.block.BlockID.PartSetHeader.Total++ }, "does not match"},
		{"invalid_signature", func(p *repairM1466Peer) {
			p.signed.Commit.Signatures[0].Signature = utils.Some(ed25519.TestSecretKey([]byte("attacker")).Sign([]byte("invalid")))
		}, "verify block commit"},
		{"insufficient_quorum", func(p *repairM1466Peer) { p.signed.Commit.Signatures[0] = tmtypes.NewCommitSigAbsent() }, "verify block commit"},
		{"self_consistent_forgery", func(p *repairM1466Peer) {
			p.block.Block.AppHash = []byte("forged state")
			p.block.BlockID.Hash = p.block.Block.Header.Hash()
			header := p.block.Block.Header
			p.signed.Header = &header
			p.signed.Commit.BlockID = p.block.BlockID
		}, "verify block commit"},
		{"untrusted_validators", func(p *repairM1466Peer) {
			p.trust.ValidatorSet = tmtypes.NewValidatorSet([]*tmtypes.Validator{tmtypes.NewValidator(ed25519.TestSecretKey([]byte("untrusted")).Public(), 30)})
		}, "trusted validator set"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer := repairM1466Fixture(t)
			tc.alter(peer)
			output, err := repairM1466Command(peer.serve(t), repairM1466TrustFile(t, peer.trust), "7")
			require.ErrorContains(t, err, tc.message)
			require.Empty(t, output, "verification failure must not publish block data")
		})
	}
}

func TestRepairM_1466_RequireIndependentTrust(t *testing.T) {
	for _, name := range []string{"missing_flag", "missing_file", "malformed_file", "missing_validator_set", "missing_chain", "wrong_client_chain", "wrong_trusted_height", "zero_trusted_height"} {
		t.Run(name, func(t *testing.T) {
			peer := repairM1466Fixture(t)
			ctx := peer.serve(t)
			path := ""
			switch name {
			case "missing_file":
				path = filepath.Join(t.TempDir(), "absent.json")
			case "malformed_file":
				path = filepath.Join(t.TempDir(), "bad.json")
				require.NoError(t, os.WriteFile(path, []byte("{"), 0600))
			case "missing_validator_set":
				peer.trust.ValidatorSet = nil
			case "missing_chain":
				peer.trust.ChainID = ""
			case "wrong_client_chain":
				ctx = ctx.WithChainID("wrong-chain")
			case "wrong_trusted_height":
				peer.trust.Height++
			case "zero_trusted_height":
				peer.trust.Height = 0
			}
			if name != "missing_flag" && path == "" {
				path = repairM1466TrustFile(t, peer.trust)
			}
			output, err := repairM1466Command(ctx, path, "7")
			require.Error(t, err)
			require.Empty(t, output)
			peer.mu.Lock()
			defer peer.mu.Unlock()
			require.Empty(t, peer.calls, "invalid or absent trust must fail before querying the node")
		})
	}
}
