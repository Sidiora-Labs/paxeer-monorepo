package custodyproof

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"testing"
	"time"

	consensuscrypto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func TestRepairM_856_NativeBankRoundtrip(t *testing.T) {
	nativeBankRoundtrip(t)
}

func TestRepairM_856_GenesisNativeBank(t *testing.T) {
	f := nativeBankProducer(t, 1)
	request := nativeTransport(t, f.request)
	request.Bundle.History = append([]LightBlock{f.pin.Block}, request.Bundle.History...)
	request = nativeTransport(t, request)
	result, err := Verify(request, f.now)
	require.NoError(t, err)
	require.Equal(t, "750", result.Amount)
	require.Equal(t, request.Expected.Account, result.Account)
	require.Equal(t, hexBytes(f.blocks[2].Commit.AppHash), result.ApplicationRoot)
	require.Equal(t, hexBytes(f.blocks[3].Commit.Hash()), result.FinalizedHeaderHash)
	_, err = Verify(request, f.blocks[3].Commit.Time.Add(TrustingPeriod))
	require.Error(t, err)
	_, err = Verify(request, f.blocks[3].Commit.Time.Add(-MaxClockDrift))
	require.Error(t, err)
	request.Bundle.Bank[0].Balance.Value[0] ^= 1
	_, err = Verify(request, f.now)
	require.Error(t, err)
}

func TestRepairM_856_MatureHeightAndWorkBound(t *testing.T) {
	f := nativeBankProducer(t, MaxHistory*1000+17)
	db := dbm.NewMemDB()
	v, err := OpenVerifier(db, f.request.Expected, f.pin, f.now)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, v.Close()) })
	result, err := v.Verify(nativeTransport(t, f.request), f.now)
	require.NoError(t, err)
	require.Greater(t, result.StateHeight, int64(MaxHistory))
	require.Equal(t, f.request.Bundle.StateHeight, result.StateHeight)
	before, err := db.Get([]byte(trustVersion))
	require.NoError(t, err)
	require.Error(t, v.VerifyHeaders(make([]LightBlock, MaxHistory+1), f.now))
	require.Error(t, v.VerifyHeaders(nil, f.now))
	after, err := db.Get([]byte(trustVersion))
	require.NoError(t, err)
	require.Equal(t, before, after)
	_, err = v.TrustedHeader(result.FinalizedHeight + 2)
	require.Error(t, err)
	_, err = v.TrustedHeader(-1)
	require.Error(t, err)
}

func nativeBankRefusals(t *testing.T) {
	t.Helper()
	f := nativeBankProducer(t, MaxHistory+64)
	db := dbm.NewMemDB()
	v, err := OpenVerifier(db, f.request.Expected, f.pin, f.now)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, v.Close()) })
	before, err := db.Get([]byte(trustVersion))
	require.NoError(t, err)
	for _, tc := range []struct {
		name   string
		mutate func(*Request)
	}{
		{"missing_header", func(r *Request) { r.Bundle.History[0].Commit.Header = nil }},
		{"missing_commit", func(r *Request) { r.Bundle.History[0].Commit.Commit = nil }},
		{"bad_header", func(r *Request) { r.Bundle.History[0].Commit.AppHash[0] ^= 1 }},
		{"noncanonical", func(r *Request) { r.Bundle.History[0].Commit.CanonicalCommit = false }},
		{"missing_quorum", func(r *Request) { r.Bundle.History[0].Commit.Commit.Signatures = nil }},
		{"bad_signature", func(r *Request) {
			r.Bundle.History[0].Commit.Commit.Signatures[0].Signature = utils.Some(ed25519.TestSecretKey([]byte("unauthorized")).Sign([]byte("wrong vote")))
		}},
		{"missing_validator_key", func(r *Request) { r.Bundle.History[0].Validators[0].Validators[0].PubKey = consensuscrypto.PubKey{} }},
		{"validator_height", func(r *Request) { r.Bundle.History[0].Validators[0].BlockHeight++ }},
		{"missing_validator_page", func(r *Request) { r.Bundle.History[0].Validators = nil }},
		{"wrong_header_chain", func(r *Request) { r.Bundle.History[0].Commit.ChainID = "atlantic-2" }},
		{"wrong_expected_chain", func(r *Request) { r.Expected.CometChainID = "atlantic-2" }},
		{"wrong_evm_chain", func(r *Request) { r.Expected.ChainID = 1 }},
		{"wrong_genesis_pin", func(r *Request) { r.Expected.GenesisSHA256 = "0x" + string(bytes.Repeat([]byte{'1'}, 64)) }},
		{"wrong_genesis_bytes", func(r *Request) { r.Bundle.Genesis[0] ^= 1 }},
		{"skipped_height", func(r *Request) { r.Bundle.History = r.Bundle.History[1:] }},
		{"reordered_history", func(r *Request) { r.Bundle.History[0], r.Bundle.History[1] = r.Bundle.History[1], r.Bundle.History[0] }},
		{"absent_membership", func(r *Request) { r.Bundle.Bank[0].Balance.ProofOps = nil }},
		{"bad_membership", func(r *Request) { r.Bundle.Bank[0].Balance.ProofOps.Ops[0].Data[0] ^= 1 }},
		{"bad_outer_membership", func(r *Request) { r.Bundle.Bank[0].Balance.ProofOps.Ops[1].Data[0] ^= 1 }},
		{"wrong_store", func(r *Request) { r.Bundle.Bank[0].Balance.ProofOps.Ops[1].Key = []byte("layerxcustody") }},
		{"wrong_key", func(r *Request) { r.Bundle.Bank[0].Balance.Key[1] ^= 1 }},
		{"wrong_proof_key", func(r *Request) { r.Bundle.Bank[0].Balance.ProofOps.Ops[0].Key[0] ^= 1 }},
		{"wrong_proof_type", func(r *Request) { r.Bundle.Bank[0].Balance.ProofOps.Ops[0].Type = "unregistered" }},
		{"wrong_proof_count", func(r *Request) { r.Bundle.Bank[0].Balance.ProofOps.Ops = r.Bundle.Bank[0].Balance.ProofOps.Ops[:1] }},
		{"oversized_membership", func(r *Request) { r.Bundle.Bank[0].Balance.ProofOps.Ops[0].Data = make([]byte, MaxProofBytes+1) }},
		{"wrong_value", func(r *Request) { r.Bundle.Bank[0].Balance.Value[0] ^= 1 }},
		{"empty_value", func(r *Request) { r.Bundle.Bank[0].Balance.Value = nil }},
		{"query_failure", func(r *Request) { r.Bundle.Bank[0].Balance.Code = 1 }},
		{"wrong_state_height", func(r *Request) { r.Bundle.Bank[0].Balance.Height++ }},
		{"wrong_point_height", func(r *Request) { r.Bundle.Bank[0].Height++ }},
		{"uncommitted_root_height", func(r *Request) { r.Bundle.StateHeight--; r.Bundle.Bank[0].Height-- }},
		{"wrong_account", func(r *Request) { r.Expected.Account = "0x" + string(bytes.Repeat([]byte{'2'}, 40)) }},
		{"zero_account", func(r *Request) { r.Expected.Account = "0x" + string(bytes.Repeat([]byte{'0'}, 40)) }},
		{"wrong_denom", func(r *Request) { r.Expected.Denom = "other" }},
		{"invalid_denom", func(r *Request) { r.Expected.Denom = "?" }},
		{"missing_finalized_membership", func(r *Request) { r.Bundle.Bank = r.Bundle.Bank[:1] }},
		{"extra_membership", func(r *Request) { r.Bundle.Bank = append(r.Bundle.Bank, r.Bundle.Bank[0]) }},
		{"insufficient_confirmations", func(r *Request) { r.Expected.Confirmations = 3 }},
		{"zero_confirmations", func(r *Request) { r.Expected.Confirmations = 0 }},
		{"unbounded_finality", func(r *Request) { r.Bundle.FinalizedHeight = r.Bundle.StateHeight + MaxHistory }},
		{"overflow_finality", func(r *Request) { r.Bundle.FinalizedHeight = int64(^uint64(0) >> 1) }},
		{"mixed_custody", func(r *Request) { r.Expected.Custody = "0x0000000000000000000000000000000000001013" }},
		{"wrong_operation", func(r *Request) { r.Operation = "verify" }},
		{"unknown_version", func(r *Request) { r.Bundle.Version = "unknown" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := nativeTransport(t, f.request)
			tc.mutate(request)
			result, err := v.Verify(request, f.now)
			require.Error(t, err)
			require.Nil(t, result)
			after, err := db.Get([]byte(trustVersion))
			require.NoError(t, err)
			require.Equal(t, before, after, "refused proof advanced persistent trust")
			_, err = v.TrustedHeader(f.blocks[1].Commit.Height)
			require.Error(t, err)
		})
	}
	_, err = v.Verify(nil, f.now)
	require.Error(t, err)
	_, err = v.Verify(f.request, time.Time{})
	require.Error(t, err)
	_, err = v.Verify(nativeTransport(t, f.request), f.now)
	require.NoError(t, err)
}

func TestRepairM_856_BadHeadersMembershipAndChain(t *testing.T) {
	nativeBankRefusals(t)
}

func TestRepairM_856_CheckpointRefusals(t *testing.T) {
	f := nativeBankProducer(t, MaxHistory+64)
	db := dbm.NewMemDB()
	v, err := OpenVerifier(db, f.request.Expected, f.pin, f.now)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, v.Close()) })
	before, err := db.Get([]byte(trustVersion))
	require.NoError(t, err)
	wrongPin := f.pin
	digest := sha256.Sum256([]byte("wrong checkpoint"))
	wrongPin.HeaderSHA256 = hexBytes(digest[:])
	_, err = OpenVerifier(db, f.request.Expected, wrongPin, f.now)
	require.Error(t, err)
	_, err = OpenVerifier(nil, f.request.Expected, f.pin, f.now)
	require.Error(t, err)
	wrongNetwork := f.request.Expected
	wrongNetwork.GenesisSHA256 = hexBytes(digest[:])
	_, err = OpenVerifier(db, wrongNetwork, f.pin, f.now)
	require.Error(t, err)
	_, err = OpenVerifier(db, f.request.Expected, f.pin, f.pin.Block.Commit.Time.Add(TrustingPeriod))
	require.Error(t, err)
	_, err = OpenVerifier(db, f.request.Expected, f.pin, f.pin.Block.Commit.Time.Add(-MaxClockDrift))
	require.Error(t, err)
	next := types.NewValidatorSet([]*types.Validator{types.NewValidator(nativeSigner(t, "b").Key.PubKey, 10)})
	root := bytes.Clone(f.pin.Block.Commit.AppHash)
	root[0] ^= 1
	fork := nativeBlock(t, nativeSigner(t, "a"), next, f.pin.Block.Commit.Height, root, nil, f.pin.Block.Commit.Time)
	_, err = OpenVerifier(db, f.request.Expected, PinnedCheckpoint{HeaderSHA256: hexBytes(fork.Commit.Hash()), Block: fork}, f.now)
	require.Error(t, err, "valid but conflicting pinned checkpoint replaced persisted trust")
	unauthorized := nativeSigner(t, "d")
	unauthorizedSet := types.NewValidatorSet([]*types.Validator{types.NewValidator(unauthorized.Key.PubKey, 10)})
	badTransition := nativeBlock(t, unauthorized, unauthorizedSet, f.pin.Block.Commit.Height+1,
		f.blocks[1].Commit.AppHash, &f.pin.Block, f.blocks[1].Commit.Time)
	require.Error(t, v.VerifyHeaders([]LightBlock{badTransition}, f.now))
	wrongParent := nativeBlock(t, nativeSigner(t, "b"), next, f.pin.Block.Commit.Height+1,
		f.blocks[1].Commit.AppHash, &fork, f.blocks[1].Commit.Time)
	require.Error(t, v.VerifyHeaders([]LightBlock{wrongParent}, f.now))
	backwardsTime := nativeBlock(t, nativeSigner(t, "b"), next, f.pin.Block.Commit.Height+1,
		f.blocks[1].Commit.AppHash, &f.pin.Block, f.pin.Block.Commit.Time)
	require.Error(t, v.VerifyHeaders([]LightBlock{backwardsTime}, f.now))
	after, err := db.Get([]byte(trustVersion))
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.NoError(t, v.VerifyHeaders(f.blocks[1:2], f.now))
	c := types.NewValidatorSet([]*types.Validator{types.NewValidator(nativeSigner(t, "c").Key.PubKey, 10)})
	fork = nativeBlock(t, nativeSigner(t, "b"), c, f.blocks[1].Commit.Height, root, &f.pin.Block, f.blocks[1].Commit.Time)
	require.Error(t, v.VerifyHeaders([]LightBlock{fork}, f.now), "signed conflicting overlap accepted")
	_, err = v.TrustedHeader(f.blocks[1].Commit.Height)
	require.NoError(t, err)
	_, err = v.TrustedHeader(f.blocks[2].Commit.Height)
	require.Error(t, err)
}

func TestRepairM_856_ExpiredTrustCannotRefresh(t *testing.T) {
	f := nativeBankProducer(t, MaxHistory+64)
	for _, now := range []time.Time{f.pin.Block.Commit.Time.Add(TrustingPeriod), f.pin.Block.Commit.Time.Add(-MaxClockDrift)} {
		db := dbm.NewMemDB()
		_, err := OpenVerifier(db, f.request.Expected, f.pin, now)
		require.Error(t, err)
		stored, err := db.Get([]byte(trustVersion))
		require.NoError(t, err)
		require.Empty(t, stored)
		require.NoError(t, db.Close())
	}
	v, err := OpenVerifier(dbm.NewMemDB(), f.request.Expected, f.pin, f.now)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, v.Close()) })
	now := f.pin.Block.Commit.Time.Add(TrustingPeriod)
	signer := nativeSigner(t, "b")
	next := types.NewValidatorSet([]*types.Validator{types.NewValidator(signer.Key.PubKey, 10)})
	newHead := nativeBlock(t, signer, next, f.pin.Block.Commit.Height+1, f.blocks[1].Commit.AppHash,
		&f.pin.Block, now.Add(-time.Second))
	require.Error(t, v.VerifyHeaders([]LightBlock{newHead}, now))
	_, err = v.TrustedHeader(newHead.Commit.Height)
	require.Error(t, err)
}

func TestRepairM_856_RecoveryAfterOriginalPinExpires(t *testing.T) {
	f := nativeBankProducer(t, MaxHistory+64)
	dir := t.TempDir()
	v, err := OpenVerifier(nativeTrustDB(t, dir), f.request.Expected, f.pin, f.now)
	require.NoError(t, err)
	b, c := nativeSigner(t, "b"), nativeSigner(t, "c")
	cSet := types.NewValidatorSet([]*types.Validator{types.NewValidator(c.Key.PubKey, 10)})
	first := nativeBlock(t, b, cSet, f.pin.Block.Commit.Height+1, f.blocks[1].Commit.AppHash,
		&f.pin.Block, f.pin.Block.Commit.Time.Add(TrustingPeriod/2))
	require.NoError(t, v.VerifyHeaders([]LightBlock{first}, first.Commit.Time.Add(time.Second)))
	second := nativeBlock(t, c, cSet, first.Commit.Height+1, f.blocks[2].Commit.AppHash,
		&first, first.Commit.Time.Add(TrustingPeriod/2))
	now := second.Commit.Time.Add(time.Second)
	require.NoError(t, v.VerifyHeaders([]LightBlock{second}, now))
	require.NoError(t, v.Close())
	v, err = OpenVerifier(nativeTrustDB(t, dir), f.request.Expected, f.pin, now)
	require.NoError(t, err, "persisted live head was lost when the original pin expired")
	t.Cleanup(func() { require.NoError(t, v.Close()) })
	header, err := v.TrustedHeader(second.Commit.Height)
	require.NoError(t, err)
	require.Equal(t, second.Commit.Hash(), header.Hash())
	request := nativeTransport(t, f.request)
	request.Bundle.History = nil
	result, err := v.Verify(request, now)
	require.NoError(t, err)
	require.Equal(t, "1000", result.Amount)
	_, err = v.Verify(request, second.Commit.Time.Add(TrustingPeriod))
	require.Error(t, err)
}

func TestRepairM_856_CorruptTrustRefusal(t *testing.T) {
	f := nativeBankProducer(t, MaxHistory+64)
	for _, encoded := range [][]byte{{}, []byte("{malformed")} {
		db := dbm.NewMemDB()
		require.NoError(t, db.SetSync([]byte(trustVersion), encoded))
		_, err := OpenVerifier(db, f.request.Expected, f.pin, f.now)
		require.Error(t, err, "corrupt trust was silently reinitialized")
		after, err := db.Get([]byte(trustVersion))
		require.NoError(t, err)
		require.Equal(t, encoded, after)
		require.NoError(t, db.Close())
	}
	for _, tc := range []struct {
		name    string
		corrupt func(*trustState)
	}{
		{"version", func(s *trustState) { s.Version = "unknown" }},
		{"network", func(s *trustState) { s.Network.ChainID = 1 }},
		{"pin", func(s *trustState) { s.PinHash = "0x" + string(bytes.Repeat([]byte{'1'}, 64)) }},
		{"missing_history", func(s *trustState) { s.History = nil }},
		{"missing_header", func(s *trustState) { s.History[0].Commit.Header = nil }},
		{"bad_validators", func(s *trustState) { s.History[1].Validators[0].Validators[0].VotingPower++ }},
		{"bad_transition", func(s *trustState) { s.History[1], s.History[2] = s.History[2], s.History[1] }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			v, err := OpenVerifier(nativeTrustDB(t, dir), f.request.Expected, f.pin, f.now)
			require.NoError(t, err)
			_, err = v.Verify(nativeTransport(t, f.request), f.now)
			require.NoError(t, err)
			require.NoError(t, v.Close())
			db := nativeTrustDB(t, dir)
			defer func() { require.NoError(t, db.Close()) }()
			encoded, err := db.Get([]byte(trustVersion))
			require.NoError(t, err)
			var stored trustState
			require.NoError(t, json.Unmarshal(encoded, &stored))
			tc.corrupt(&stored)
			encoded, err = json.Marshal(stored)
			require.NoError(t, err)
			require.NoError(t, db.SetSync([]byte(trustVersion), encoded))
			_, err = OpenVerifier(db, f.request.Expected, f.pin, f.now)
			require.Error(t, err, "corrupt trust was accepted or silently reset")
			after, err := db.Get([]byte(trustVersion))
			require.NoError(t, err)
			require.Equal(t, encoded, after)
		})
	}
}

func TestRepairM_856_HistoricalCustodyBoundary(t *testing.T) {
	request := realRequest(t)
	now := request.Bundle.History[len(request.Bundle.History)-1].Commit.Time.Add(time.Second)
	original, err := Verify(request, now)
	require.NoError(t, err)
	dir := t.TempDir()
	pin := PinnedCheckpoint{Block: request.Bundle.History[0], HeaderSHA256: hexBytes(request.Bundle.History[0].Commit.Hash())}
	v, err := OpenVerifier(nativeTrustDB(t, dir), request.Expected, pin, now)
	require.NoError(t, err)
	result, err := v.Verify(nativeTransport(t, request), now)
	require.NoError(t, err)
	require.Equal(t, original, result)
	require.NoError(t, v.Close())
	v, err = OpenVerifier(nativeTrustDB(t, dir), request.Expected, pin, now)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, v.Close()) })
	request.Bundle.History, request.Bundle.Genesis = nil, nil
	result, err = v.Verify(nativeTransport(t, request), now)
	require.NoError(t, err)
	require.Equal(t, original.DepositID, result.DepositID)
	require.Equal(t, original.Amount, result.Amount)
	require.Equal(t, original.ApplicationRoot, result.ApplicationRoot)
	request.Expected.Account = "0x" + string(bytes.Repeat([]byte{'1'}, 40))
	_, err = v.Verify(request, now)
	require.Error(t, err, "bank identity was admitted as historical custody")
}
