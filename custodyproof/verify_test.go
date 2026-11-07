package custodyproof

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	consensuscrypto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	rootmultiv2 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func realRequest(t *testing.T) *Request {
	t.Helper()
	encoded, err := os.ReadFile("../tests/fixtures/custody/paxeer-state-v3/state-credit.json")
	if err != nil {
		t.Fatal(err)
	}
	request, err := Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func TestRealStateCreditAndClockBounds(t *testing.T) {
	request := realRequest(t)
	last := request.Bundle.History[len(request.Bundle.History)-1].Commit.Time
	result, err := Verify(request, last.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if result.DepositID != request.Expected.DepositID || result.StateHeight != request.Bundle.StateHeight ||
		result.FinalizedHeight != request.Bundle.FinalizedHeight {
		t.Fatal("verified state identity")
	}
	if _, err := Verify(request, last.Add(TrustingPeriod)); err == nil {
		t.Fatal("expired live head accepted")
	}
	if _, err := Verify(request, last.Add(-MaxClockDrift)); err == nil {
		t.Fatal("future header accepted")
	}
}

func TestRealStateCreditRefusesMissingAuthority(t *testing.T) {
	request := realRequest(t)
	now := request.Bundle.History[len(request.Bundle.History)-1].Commit.Time.Add(time.Second)
	request.Bundle.History[0].Validators[0].Validators[0].PubKey = consensuscrypto.PubKey{}
	if _, err := Verify(request, now); err == nil {
		t.Fatal("missing validator key accepted")
	}
	request = realRequest(t)
	request.Bundle.History[0].Commit.Commit = nil
	if _, err := Verify(request, now); err == nil {
		t.Fatal("missing commit accepted")
	}
}

func TestJSONRequestRefusesAmbiguousFields(t *testing.T) {
	for _, encoded := range []string{`{"expected":{},"expected":{},"bundle":{}}`,
		`{"expected":{},"bundle":{},"verified":true}`, `{} {}`} {
		if _, err := Decode([]byte(encoded)); err == nil {
			t.Fatal("ambiguous request accepted")
		}
	}
}

type nativeBankFixture struct {
	request *Request
	pin     PinnedCheckpoint
	blocks  []LightBlock
	points  []BankStatePoint
	now     time.Time
}

func nativeSigner(t *testing.T, name string) *privval.FilePV {
	t.Helper()
	dir := t.TempDir()
	return privval.NewFilePV(ed25519.TestSecretKey([]byte("custodyproof-native-"+name)),
		filepath.Join(dir, "key.json"), filepath.Join(dir, "state.json"))
}

// These are production consensus blocks, commits and FilePV signatures, not a
// replacement verifier or signer. Roots and query responses come from paxd's
// native bank keeper and versioned commitment store below.
func nativeBlock(t *testing.T, signer *privval.FilePV, next *types.ValidatorSet,
	height int64, root []byte, previous *LightBlock, at time.Time) LightBlock {
	t.Helper()
	validators := types.NewValidatorSet([]*types.Validator{types.NewValidator(signer.Key.PubKey, 10)})
	lastCommit := &types.Commit{}
	if previous != nil {
		lastCommit = previous.Commit.Commit
	}
	block := types.MakeBlock(height, nil, lastCommit, nil)
	block.ChainID, block.Time = "hyperpax_125-1", at
	block.AppHash = bytes.Clone(root)
	block.ValidatorsHash, block.NextValidatorsHash = validators.Hash(), next.Hash()
	block.ProposerAddress = signer.Key.Address
	if previous != nil {
		block.LastBlockID = previous.Commit.Commit.BlockID
	}
	parts, err := block.MakePartSet(types.BlockPartSizeBytes)
	require.NoError(t, err)
	blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: parts.Header()}
	votes := types.NewVoteSet(block.ChainID, height, 0, tmproto.PrecommitType, validators)
	commit, err := types.MakeCommit(context.Background(), blockID, height, 0, votes,
		[]types.PrivValidator{signer}, at)
	require.NoError(t, err)
	require.NoError(t, signer.Save())
	return LightBlock{Commit: *coretypes.NewResultCommit(&block.Header, commit, true),
		Validators: []coretypes.ResultValidators{{BlockHeight: height, Count: 1, Total: 1,
			Validators: validators.Validators}}}
}

func nativeBankProducer(t *testing.T, height int64) *nativeBankFixture {
	t.Helper()
	sc := config.DefaultStateCommitConfig()
	sc.MemIAVLConfig.AsyncCommitBuffer = 0
	sc.HistoricalProofRateLimit = 0
	store := rootmultiv2.NewStore(t.TempDir(), sc, config.StateStoreConfig{}, nil)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	bankKey, authKey, paramsKey := sdk.NewKVStoreKey(banktypes.StoreKey),
		sdk.NewKVStoreKey(authtypes.StoreKey), sdk.NewKVStoreKey(paramtypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	for _, key := range []*sdk.KVStoreKey{bankKey, authKey, paramsKey} {
		store.MountStoreWithDB(key, storetypes.StoreTypeIAVL, nil)
	}
	store.MountStoreWithDB(transientKey, storetypes.StoreTypeTransient, nil)
	require.NoError(t, store.LoadLatestVersion())
	require.NoError(t, store.SetInitialVersion(height))
	appDB := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, appDB.Close()) })
	options := viper.New()
	options.Set(baseapp.FlagChainID, "hyperpax_125-1")
	app := baseapp.NewBaseApp("paxd-native-proof", appDB, nil, nil, options)
	app.SetCMS(store)
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	bankParams := paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, banktypes.ModuleName)
	authParams := paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, authtypes.ModuleName)
	accounts := authkeeper.NewAccountKeeper(cdc, authKey, authParams, authtypes.ProtoBaseAccount, map[string][]string{})
	bank := bankkeeper.NewBaseKeeper(cdc, bankKey, accounts, bankParams, map[string]bool{})
	owner, recipient := sdk.AccAddress(bytes.Repeat([]byte{0x17}, 20)), sdk.AccAddress(bytes.Repeat([]byte{0x28}, 20))
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	var roots [][]byte
	var points []BankStatePoint
	for index := 0; index < 4; index++ {
		cache := store.CacheMultiStore()
		ctx := sdk.NewContext(cache, tmproto.Header{Height: height + int64(index), Time: at.Add(time.Duration(index) * time.Second)}, false)
		if index == 0 {
			accounts.SetParams(ctx, authtypes.DefaultParams())
			for _, addr := range []sdk.AccAddress{owner, recipient} {
				accounts.SetAccount(ctx, accounts.NewAccountWithAddress(ctx, addr))
			}
			genesis := banktypes.DefaultGenesisState()
			genesis.Balances = []banktypes.Balance{{Address: owner.String(), Coins: sdk.NewCoins(sdk.NewInt64Coin("upax", 1000))}}
			bank.InitGenesis(ctx, genesis)
		} else {
			require.NoError(t, bank.SendCoins(ctx, owner, recipient, sdk.NewCoins(sdk.NewInt64Coin("upax", 250))))
		}
		balance := bank.GetBalance(ctx, owner, "upax")
		require.Equal(t, int64(1000-index*250), balance.Amount.Int64())
		cache.Write()
		commit := store.Commit(true)
		require.Equal(t, height+int64(index), commit.Version)
		roots = append(roots, bytes.Clone(commit.Hash))
		// BaseApp deliberately refuses proof queries at genesis height one.
		if commit.Version == 1 {
			continue
		}
		response, err := app.Query(context.Background(), &abci.RequestQuery{Path: "/store/bank/key",
			Data: banktypes.CreatePrefixedAccountStoreKey(owner, []byte("upax")), Height: commit.Version, Prove: true})
		require.NoError(t, err)
		require.NotNil(t, response)
		require.Zero(t, response.Code, response.Log)
		require.Equal(t, commit.Version, response.Height)
		require.Len(t, response.ProofOps.Ops, 2)
		encoded, err := cdc.Marshal(&balance)
		require.NoError(t, err)
		require.Equal(t, encoded, response.Value)
		points = append(points, BankStatePoint{Height: commit.Version, Balance: *response})
	}
	signers := []*privval.FilePV{nativeSigner(t, "a"), nativeSigner(t, "b"), nativeSigner(t, "c"), nativeSigner(t, "d")}
	sets := make([]*types.ValidatorSet, len(signers))
	for index, signer := range signers {
		sets[index] = types.NewValidatorSet([]*types.Validator{types.NewValidator(signer.Key.PubKey, 10)})
	}
	genesis := &types.GenesisDoc{GenesisTime: at.Add(-time.Hour), ChainID: "hyperpax_125-1", InitialHeight: 1,
		Validators:      []types.GenesisValidator{{Address: signers[0].Key.Address, PubKey: signers[0].Key.PubKey, Power: 10}},
		ConsensusParams: types.DefaultConsensusParams()}
	encodedGenesis, err := json.Marshal(genesis)
	require.NoError(t, err)
	genesisDigest := sha256.Sum256(encodedGenesis)
	initialRoot := sha256.Sum256(nil)
	var blocks []LightBlock
	// Rotate A -> B -> C, persist C -> D, then consume D after reopening.
	current, next := []int{0, 1, 2, 2, 3}, []int{1, 2, 2, 3, 3}
	for index := 0; index < 5; index++ {
		root := initialRoot[:]
		var previous *LightBlock
		if index > 0 {
			root, previous = roots[index-1], &blocks[index-1]
		}
		blocks = append(blocks, nativeBlock(t, signers[current[index]], sets[next[index]],
			height+int64(index), root, previous, at.Add(time.Duration(index)*time.Second)))
	}
	expected := Expected{GenesisSHA256: hexBytes(genesisDigest[:]), CometChainID: genesis.ChainID,
		ChainID: 125, Confirmations: 2, Account: hexBytes(owner), Denom: "upax"}
	stateOffset := int64(0)
	if height == 1 {
		stateOffset = 1
	}
	return &nativeBankFixture{request: &Request{Operation: "verify-bank", Expected: expected,
		Bundle: Bundle{Version: BankVersion, Genesis: encodedGenesis, History: blocks[1 : 3+stateOffset], StateHeight: height + stateOffset,
			FinalizedHeight: height + stateOffset + 1, Bank: points[:2]}},
		pin:    PinnedCheckpoint{HeaderSHA256: hexBytes(blocks[0].Commit.Hash()), Block: blocks[0]},
		blocks: blocks, points: points, now: at.Add(5 * time.Second)}
}

func nativeTransport(t *testing.T, request *Request) *Request {
	t.Helper()
	encoded, err := json.Marshal(request)
	require.NoError(t, err)
	decoded, err := Decode(encoded)
	require.NoError(t, err)
	return decoded
}

func nativeTrustDB(t *testing.T, dir string) dbm.DB {
	t.Helper()
	db, err := dbm.NewDB("trust", dbm.GoLevelDBBackend, dir)
	require.NoError(t, err)
	return db
}

func nativeBankRoundtrip(t *testing.T) {
	t.Helper()
	f := nativeBankProducer(t, MaxHistory+64)
	dir := t.TempDir()
	v, err := OpenVerifier(nativeTrustDB(t, dir), f.request.Expected, f.pin, f.now)
	require.NoError(t, err)
	result, err := v.Verify(nativeTransport(t, f.request), f.now)
	require.NoError(t, err)
	require.Equal(t, "1000", result.Amount)
	require.Equal(t, f.request.Expected.Account, result.Account)
	require.Equal(t, "upax", result.Denom)
	require.Equal(t, BankVersion, result.Version)
	require.Equal(t, f.request.Bundle.StateHeight, result.StateHeight)
	require.Equal(t, f.request.Bundle.FinalizedHeight, result.FinalizedHeight)
	require.Equal(t, hexBytes(f.blocks[1].Commit.AppHash), result.ApplicationRoot)
	require.Equal(t, hexBytes(f.blocks[2].Commit.Hash()), result.FinalizedHeaderHash)
	require.NotEmpty(t, result.ProofSHA256)
	require.Empty(t, result.DepositID)
	require.Empty(t, result.ModuleSHA256)
	require.NoError(t, v.VerifyHeaders(f.blocks[3:4], f.now))
	header, err := v.TrustedHeader(f.blocks[3].Commit.Height)
	require.NoError(t, err)
	require.Equal(t, f.blocks[4].Commit.ValidatorsHash, header.NextValidatorsHash)
	header.NextValidatorsHash[0] ^= 1 // Returned data cannot mutate local trust.
	require.NoError(t, v.Close())
	_, err = v.Verify(f.request, f.now)
	require.Error(t, err)
	v, err = OpenVerifier(nativeTrustDB(t, dir), f.request.Expected, f.pin, f.now)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, v.Close()) })
	header, err = v.TrustedHeader(f.blocks[3].Commit.Height)
	require.NoError(t, err)
	require.Equal(t, f.blocks[4].Commit.ValidatorsHash, header.NextValidatorsHash)
	// Recovery supplies proofs with no historical replay, then uses the persisted
	// validator transition to extend the head with the newly authorized key D.
	request := nativeTransport(t, f.request)
	request.Bundle.History, request.Bundle.Genesis = nil, nil
	recovered, err := v.Verify(request, f.now)
	require.NoError(t, err)
	require.Equal(t, result.Amount, recovered.Amount)
	require.Equal(t, result.ApplicationRoot, recovered.ApplicationRoot)
	request.Bundle.StateHeight += 2
	request.Bundle.FinalizedHeight += 2
	request.Bundle.Bank, request.Bundle.History = f.points[2:4], f.blocks[4:5]
	advanced, err := v.Verify(nativeTransport(t, request), f.now)
	require.NoError(t, err)
	require.Equal(t, "500", advanced.Amount)
	require.Equal(t, hexBytes(f.blocks[4].Commit.Hash()), advanced.FinalizedHeaderHash)
}

func TestNativeBankCommittedStateAndRecovery(t *testing.T) {
	nativeBankRoundtrip(t)
}

func TestNativeBankMalformedProofRefusal(t *testing.T) {
	nativeBankRefusals(t)
}
