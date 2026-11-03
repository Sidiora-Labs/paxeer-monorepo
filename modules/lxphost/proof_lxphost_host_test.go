//go:build cgo

// Proves: lxphost.Host.HostExecute and ExecuteActivity -> lxphost.Code, ResultError and Receipt -> layerxexchange.RevertError, common.RevertReason and codec.DecodeReceipt.
// Repairs execute.go:ResultError losing its kernel Code when formatting the
// result; host.go:lxphost_authority_write accepting counts beyond AuthorityScope's
// fixed arrays; and precompiles/layerxexchange/errors.go:RevertError turning a
// fatal kernel failure into an empty revert reason. Every receipt and refusal
// starts at the real host; malformed inputs below are mutations of its output.
package lxphost_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxexchange"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

const proofHostTime int64 = 1700000000000

type proofHostChain struct {
	key *sdk.KVStoreKey
	cms sdk.CommitMultiStore
	cfg lxphost.Config
}

func newProofHostChain(t *testing.T, workers int) *proofHostChain {
	t.Helper()
	db := dbm.NewMemDB()
	key := sdk.NewKVStoreKey("lxphost")
	cms := store.NewCommitMultiStore(db)
	cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	require.NoError(t, cms.LoadLatestVersion())
	return &proofHostChain{key: key, cms: cms, cfg: lxphost.Config{
		Workers: workers, StoreKey: key, NetworkID: 7, ParameterVersion: 1,
		TimestampWindowMs: 60000, GasLimit: 1000000,
	}}
}

func (c *proofHostChain) open(t *testing.T) *lxphost.Host {
	t.Helper()
	host, err := lxphost.NewHost(c.cfg)
	require.NoError(t, err)
	t.Cleanup(host.Close)
	return host
}

func (c *proofHostChain) context(sequence uint64) sdk.Context {
	return lxphost.WithSequence(sdk.NewContext(c.cms, tmproto.Header{
		ChainID: "lxphost", Height: 1, Time: time.UnixMilli(proofHostTime),
	}, false), sequence)
}

func (c *proofHostChain) rows() map[string][]byte {
	rows := map[string][]byte{}
	it := c.context(1).KVStore(c.key).Iterator(nil, nil)
	defer it.Close()
	for ; it.Valid(); it.Next() {
		rows[string(it.Key())] = bytes.Clone(it.Value())
	}
	return rows
}

func proofHostAuthority(t *testing.T, seed byte) (ed25519.PrivateKey, lxphost.KernelAuthority) {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))
	var key [32]byte
	copy(key[:], private.Public().(ed25519.PublicKey))
	id, err := lxphost.DIDID([]byte("did:layerx:" + hex.EncodeToString(key[:])))
	require.NoError(t, err)
	return private, lxphost.KernelAuthority{Bound: true, Actor: id, Principal: id, VerifiedKey: key}
}

func proofHostAsset(authority lxphost.KernelAuthority, salt byte) ([]byte, [32]byte) {
	saltBytes := bytes.Repeat([]byte{salt}, 32)
	preimage := append([]byte("LX:ASSET:v1"), authority.Actor[:]...)
	asset := sha256.Sum256(append(preimage, saltBytes...))
	payload := append([]byte{0, 1}, asset[:]...)
	payload = append(payload, saltBytes...)
	payload = append(payload, 1, 'T', 1, 'T', 6)
	payload = binary.BigEndian.AppendUint64(payload, 0)
	payload = binary.BigEndian.AppendUint64(payload, 1000)
	return append(payload, 1, 0), asset
}

func proofHostActivity(authority lxphost.KernelAuthority, payload []byte) lxphost.Activity {
	return lxphost.Activity{
		ProtocolVersion: codec.StateCommitmentProtocolVersion, Type: 0x00010001,
		ActorDID:     []byte("did:layerx:" + hex.EncodeToString(authority.VerifiedKey[:])),
		AuthorityKey: authority.VerifiedKey, NotBefore: uint64(proofHostTime),
		NotAfter: uint64(proofHostTime), IdempotencyKey: sha256.Sum256(payload), Payload: payload,
	}
}

func proofHostRevert(t *testing.T, source error, expected lxphost.Code, text string) {
	t.Helper()
	require.Error(t, source)
	converted := layerxexchange.RevertError(fmt.Errorf("host execution: %w", source))
	require.EqualError(t, converted, text)
	require.ErrorIs(t, converted, expected)
	var code lxphost.Code
	require.ErrorAs(t, converted, &code)
	require.Equal(t, expected, code)
	encoded := pcommon.RevertReason(converted)
	decoded, err := abi.UnpackRevert(encoded)
	require.NoError(t, err)
	require.Equal(t, text, decoded)
	_, err = abi.UnpackRevert(encoded[:len(encoded)-32])
	require.Error(t, err, "consumer must refuse a truncated producer revert")
}

func proofHostReceipt(t *testing.T, receipt lxphost.Receipt, sequence uint64, asset [32]byte) {
	t.Helper()
	for observation := 0; observation < 2; observation++ {
		decoded, err := codec.DecodeReceipt(bytes.Clone(receipt.Encoded))
		require.NoError(t, err)
		require.Equal(t, int32(0), decoded.ResultCode)
		require.Equal(t, receipt.ResultCode, lxphost.Code(decoded.ResultCode))
		require.Nil(t, layerxexchange.RevertError(lxphost.ResultError(receipt.ResultCode)))
		require.Equal(t, "LXP_OK", receipt.ResultCode.Error())
		require.Equal(t, "LXP_OK", layerxexchange.KernelCodeText(receipt.ResultCode))
		require.Equal(t, sequence, decoded.GlobalSequence)
		require.Equal(t, sequence, receipt.GlobalSequence)
		require.Equal(t, uint64(proofHostTime), decoded.Timestamp)
		require.Equal(t, decoded.Timestamp, receipt.Timestamp)
		require.Equal(t, decoded.PreviousStateRoot, receipt.PreviousStateRoot)
		require.Equal(t, decoded.ResultingStateRoot, receipt.ResultingStateRoot)
		require.NotEqual(t, receipt.PreviousStateRoot, receipt.ResultingStateRoot)
		amount := decoded.FeeCharged.Bytes()
		require.True(t, sdk.NewIntFromBigInt(new(big.Int).SetBytes(amount[:])).Equal(receipt.FeeCharged))
		var events []lxphost.ReceiptEvent
		for _, effect := range decoded.Effects {
			if effect.Kind == 3 {
				events = append(events, lxphost.ReceiptEvent{Code: effect.EventType, Body: effect.Body})
			}
		}
		require.Equal(t, events, receipt.Events)
		require.Contains(t, events, lxphost.ReceiptEvent{Code: 1, Body: asset[:]})
		require.Empty(t, receipt.Transfers)
		require.Nil(t, decoded.ProgramOutcome)
		require.Nil(t, receipt.ProgramOutcome)
	}
	for _, malformed := range [][]byte{
		bytes.Clone(receipt.Encoded[:len(receipt.Encoded)-1]),
		append(bytes.Clone(receipt.Encoded), 0),
	} {
		_, err := codec.DecodeReceipt(malformed)
		require.Error(t, err, "receipt consumer must refuse malformed host output")
	}
}

func TestProofLxphostHostProducerTransportConsumer(t *testing.T) {
	for _, workers := range []int{0, 2} {
		t.Run(fmt.Sprintf("workers_%d", workers), func(t *testing.T) {
			chain := newProofHostChain(t, workers)
			host := chain.open(t)
			private, authority := proofHostAuthority(t, 71)
			payload, asset := proofHostAsset(authority, 1)
			activity := proofHostActivity(authority, payload)
			before := chain.rows()
			_, intruder := proofHostAuthority(t, 72)
			wrongSigner := authority
			wrongSigner.VerifiedKey = intruder.VerifiedKey
			for observation := 0; observation < 2; observation++ {
				receipt, err := host.HostExecute(chain.context(41), activity, wrongSigner)
				require.Equal(t, lxphost.Receipt{}, receipt)
				proofHostRevert(t, err, lxphost.Code(-201), "LXP_BAD_SIGNATURE")
				require.Equal(t, before, chain.rows())
			}
			// Exercise the signed Activity path independently of EVM binding.
			hash, err := host.SigningHash(activity)
			require.NoError(t, err)
			activity.Signature = ed25519.Sign(private, hash[:])
			receipt, err := host.HostExecute(chain.context(41), activity, authority)
			require.NoError(t, err)
			chain.cms.Commit(true)
			host.Close()
			host.Close()
			// Consume the producer output only after stopping and reopening the host.
			reopened := chain.open(t)
			proofHostReceipt(t, receipt, 41, asset)
			for observation := 0; observation < 2; observation++ {
				record, err := reopened.ReadAsset(chain.context(42), asset)
				require.NoError(t, err)
				require.Equal(t, asset, record.AssetID)
				require.Equal(t, authority.Actor, record.IssuerDID)
				require.Equal(t, sdk.NewInt(1000), record.SupplyCap)
			}
			// New producer results must reach the consumers rather than a cached receipt.
			for sequence := uint64(1); sequence <= 2; sequence++ {
				payload, nextAsset := proofHostAsset(authority, byte(sequence+1))
				next, err := lxphost.ExecuteActivity(chain.context(41+sequence), reopened, authority, activity.Type, payload, sequence)
				require.NoError(t, err)
				proofHostReceipt(t, next, 41+sequence, nextAsset)
				require.NotEqual(t, receipt.Encoded, next.Encoded)
				require.Equal(t, receipt.ResultingStateRoot, next.PreviousStateRoot)
				receipt = next
			}
			before = chain.rows()
			payload, _ = proofHostAsset(authority, 4)
			_, err = lxphost.ExecuteActivity(chain.context(44), reopened, authority, activity.Type, payload, 1)
			proofHostRevert(t, err, lxphost.Code(-301), "LXP_SEQUENCE_REUSED")
			// ResultError must retain the same typed error all the way to ABI decoding.
			var code lxphost.Code
			require.ErrorAs(t, err, &code)
			proofHostRevert(t, lxphost.ResultError(code), code, "LXP_SEQUENCE_REUSED")
			require.Equal(t, before, chain.rows())
			_, err = host.HostExecute(chain.context(44), activity, authority)
			proofHostRevert(t, err, lxphost.Code(-3), "LXP_NON_CANONICAL")
			_, err = host.ReadAsset(chain.context(44), asset)
			proofHostRevert(t, err, lxphost.Code(-3), "LXP_NON_CANONICAL")
			require.Equal(t, before, chain.rows())
		})
	}
}

func TestProofLxphostHostAuthorityScopeAndFatalRefusal(t *testing.T) {
	chain := newProofHostChain(t, 1)
	host := chain.open(t)
	_, authority := proofHostAuthority(t, 73)
	payload, _ := proofHostAsset(authority, 5)
	activity := proofHostActivity(authority, payload)
	before := chain.rows()
	for _, scope := range []lxphost.AuthorityScope{
		{SignerCount: 9}, {ApprovalCount: 9}, {SignerCount: 255, ApprovalCount: 255},
		{MaximumTotal: sdk.NewInt(-1)},
	} {
		authority.Scope = &scope
		_, err := host.HostExecute(chain.context(41), activity, authority)
		proofHostRevert(t, err, lxphost.Code(-3), "LXP_NON_CANONICAL")
		require.Equal(t, before, chain.rows())
	}
	// The full eight-entry scope still crosses the same real boundary.
	authority.Scope = &lxphost.AuthorityScope{SignerCount: 8, ApprovalCount: 8}
	receipt, err := host.HostExecute(chain.context(41), activity, authority)
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), receipt.ResultCode)
	// A malformed persisted producer counter is decoded by the host, never
	// converted by the exchange consumer into an empty ordinary revert.
	chain.context(42).KVStore(chain.key).Set([]byte{2}, []byte{1})
	before = chain.rows()
	payload, _ = proofHostAsset(authority, 6)
	for observation := 0; observation < 2; observation++ {
		_, err := lxphost.ExecuteActivity(chain.context(42), host, authority, activity.Type, payload, 1)
		require.Error(t, err)
		var code lxphost.Code
		require.ErrorAs(t, err, &code)
		require.Equal(t, "LXP_FATAL_INVARIANT", code.Error())
		require.Empty(t, layerxexchange.KernelCodeText(code))
		wrapped := fmt.Errorf("host counter: %w", lxphost.ResultError(code))
		require.Same(t, wrapped, layerxexchange.RevertError(wrapped))
		require.True(t, errors.Is(wrapped, code))
		require.Equal(t, before, chain.rows())
	}
}
