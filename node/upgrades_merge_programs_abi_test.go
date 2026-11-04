package app

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	programs "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type hostedProgramsABIFixture struct {
	app       *App
	ctx       sdk.Context
	private   ed25519.PrivateKey
	did       []byte
	authority lxphost.KernelAuthority
	program   [32]byte
	proposer  sdk.ConsAddress
}

func (f *hostedProgramsABIFixture) execute(t *testing.T, activityType uint32, payload []byte) lxphost.Receipt {
	t.Helper()
	host := f.app.LayerXHost
	sequence, err := host.ReadIdentitySequence(f.ctx, f.did)
	require.NoError(t, err)
	next, err := lxphost.NextSequence(f.ctx, f.app.GetKey(LayerXHostStoreKey))
	require.NoError(t, err)
	timestamp := uint64(f.ctx.BlockTime().UnixMilli())
	material := binary.BigEndian.AppendUint64(nil, next)
	material = binary.BigEndian.AppendUint32(material, activityType)
	activity := lxphost.Activity{
		ProtocolVersion: codec.StateCommitmentProtocolVersion,
		Type:            activityType, ActorDID: f.did, AuthorityKey: f.authority.VerifiedKey,
		AccountSequence: sequence, NotBefore: timestamp, NotAfter: timestamp,
		IdempotencyKey: sha256.Sum256(material), Payload: payload,
	}
	if activityType == 0x00090003 {
		binary.BigEndian.PutUint64(activity.FeeLimit[8:], 100000000)
	}
	hash, err := host.SigningHash(activity)
	require.NoError(t, err)
	activity.Signature = ed25519.Sign(f.private, hash[:])
	receipt, err := host.Execute(lxphost.WithSequence(f.ctx, next), activity, f.authority)
	require.NoError(t, err)
	return receipt
}

func hostedProgramsABIReceiptVersion(t *testing.T, receipt lxphost.Receipt, version uint32) {
	t.Helper()
	decoded, err := codec.DecodeReceipt(receipt.Encoded)
	require.NoError(t, err)
	require.Equal(t, uint16(9), decoded.ModuleID)
	require.Equal(t, version, decoded.ModuleVersion)
}

func newHostedProgramsABIFixture(t *testing.T) *hostedProgramsABIFixture {
	t.Helper()
	valPub := secp256k1.GenPrivKey().PubKey()
	w := NewTestWrapper(t, time.UnixMilli(1700000000000).UTC(), valPub, true)
	f := &hostedProgramsABIFixture{app: w.App, ctx: w.Ctx.WithBlockHeight(mergeHeight - 1), program: [32]byte{0x76}, proposer: sdk.ConsAddress(valPub.Address())}
	t.Cleanup(func() { require.NoError(t, f.app.Close()) })
	f.private = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x75}, ed25519.SeedSize))
	copy(f.authority.VerifiedKey[:], f.private.Public().(ed25519.PublicKey))
	f.did = []byte("did:layerx:" + hex.EncodeToString(f.authority.VerifiedKey[:]))
	actor, err := lxphost.DIDID(f.did)
	require.NoError(t, err)
	f.authority.Actor, f.authority.Principal = actor, actor
	f.authority.AuthorityHash = sha256.Sum256(f.authority.VerifiedKey[:])
	store := f.ctx.KVStore(f.app.GetKey(LayerXHostStoreKey))
	moduleKey := func(key []byte) []byte { return append([]byte{1, 0, 9}, key...) }
	metering := binary.BigEndian.AppendUint32([]byte("LXMR1"), 1)
	for _, coefficient := range []uint64{1, 1, 1, 1, 1, 8, 8, 64, 8} {
		metering = binary.BigEndian.AppendUint64(metering, coefficient)
	}
	metering = binary.BigEndian.AppendUint64(metering, 1)
	metering = append(metering, 1)
	metering = append(metering, actor[:]...)
	store.Set(moduleKey([]byte("progmet/active/v1")), metering)
	store.Set(moduleKey(binary.BigEndian.AppendUint32([]byte("progmet/history/v1/"), 1)), metering)
	fee := binary.BigEndian.AppendUint32([]byte("LXFR1"), 1)
	for i := 0; i < 7; i++ {
		fee = binary.BigEndian.AppendUint64(fee, 1)
	}
	fee = append(fee, bytes.Repeat([]byte{0x91}, 32)...)
	for _, value := range []uint64{1000, 8, 1, 8, 1, 100, 1, 0, 1} {
		fee = binary.BigEndian.AppendUint64(fee, value)
	}
	fee = append(fee, actor[:]...)
	fee = append(fee, make([]byte, 16)...)
	store.Set(moduleKey([]byte("progfee/active/v1")), fee)
	store.Set(moduleKey(binary.BigEndian.AppendUint32([]byte("progfee/history/v1/"), 1)), fee)

	register := append([]byte{0x71, 1, 0, 2}, actor[:]...)
	register = append(register, f.authority.VerifiedKey[:]...)
	require.Zero(t, f.execute(t, 0x00070001, register).ResultCode)
	salt := bytes.Repeat([]byte{0x55}, 32)
	asset := sha256.Sum256(append(append([]byte("LX:ASSET:v1"), actor[:]...), salt...))
	registerAsset := append([]byte{0, 1}, asset[:]...)
	registerAsset = append(registerAsset, salt...)
	registerAsset = append(registerAsset, 1, 'T', 1, 'T', 6)
	registerAsset = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(registerAsset, 0), 1000000000)
	require.Zero(t, f.execute(t, 0x00010001, append(registerAsset, 1, 0)).ResultCode)
	require.Zero(t, f.execute(t, 0x00010004, append([]byte{0, 1}, asset[:]...)).ResultCode)
	_, payer, err := lxphost.AssetAccountID(f.authority.VerifiedKey, asset)
	require.NoError(t, err)
	mint := append(append([]byte{0, 1}, asset[:]...), payer[:]...)
	mint = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(mint, 0), 500000000)
	require.Zero(t, f.execute(t, 0x0001000a, mint).ResultCode)
	copy(fee[65:97], asset[:])
	store.Set(moduleKey([]byte("progfee/active/v1")), fee)
	store.Set(moduleKey(binary.BigEndian.AppendUint32([]byte("progfee/history/v1/"), 1)), fee)
	treasury, err := codec.DeriveAccountID([]byte("system:fees"))
	require.NoError(t, err)
	field := func(out, value []byte) []byte {
		return append(binary.BigEndian.AppendUint32(out, uint32(len(value))), value...)
	}
	mainName, mainID, err := lxphost.MainAccountID(f.authority.VerifiedKey)
	require.NoError(t, err)
	main := field(nil, mainID[:])
	main = field(main, []byte(mainName))
	main = append(main, 1)
	main = binary.BigEndian.AppendUint64(main, 0)
	main = binary.BigEndian.AppendUint64(main, 100000000)
	mainAsset := [32]byte{0x91}
	main = field(main, mainAsset[:])
	main = append(main, 1)
	main = binary.BigEndian.AppendUint64(main, 0)
	main = binary.BigEndian.AppendUint64(main, 1)
	main = append(main, 0, 0)
	main = field(main, f.authority.VerifiedKey[:])
	store.Set(append([]byte{6}, mainID[:]...), append(main, 1))
	account := field(nil, treasury[:])
	account = field(account, []byte("system:fees"))
	account = append(account, 10)
	account = append(account, make([]byte, 16)...)
	account = field(account, asset[:])
	account = append(account, 1)
	account = append(account, make([]byte, 16)...)
	account = append(account, 0, 0)
	account = field(account, make([]byte, 32))
	account = append(account, 0)
	store.Set(append([]byte{6}, treasury[:]...), account)

	section := func(id byte, body []byte) []byte { return append([]byte{id, byte(len(body))}, body...) }
	label := func(s string) []byte { return append([]byte{byte(len(s))}, s...) }
	guest := []byte{0, 97, 115, 109, 1, 0, 0, 0}
	guest = append(guest, section(1, []byte{2, 0x60, 1, 0x7f, 1, 0x7f, 0x60, 2, 0x7f, 0x7f, 1, 0x7f})...)
	guest = append(guest, section(3, []byte{2, 0, 1})...)
	guest = append(guest, section(5, []byte{1, 1, 1, 1})...)
	exports := append(append([]byte{3}, label("layerx_reserve")...), 0, 0)
	exports = append(append(exports, label("layerx_call")...), 0, 1)
	exports = append(append(exports, label("memory")...), 2, 0)
	guest = append(guest, section(7, exports)...)
	guest = append(guest, section(10, []byte{2, 4, 0, 0x41, 0, 0x0b, 4, 0, 0x41, 0, 0x0b})...)
	hash := sha256.Sum256(guest)
	deploy, err := programs.EncodeDeployPayload(f.program, 2, 1, actor, hash, nil, guest)
	require.NoError(t, err)
	deployed := f.execute(t, 0x00090001, deploy)
	require.Zero(t, deployed.ResultCode)
	hostedProgramsABIReceiptVersion(t, deployed, 4)
	upgrade, err := programs.EncodeUpgradePayload(f.program, 5, 0, hash, hash, nil, nil, guest)
	require.NoError(t, err)
	upgraded := f.execute(t, 0x00090002, upgrade)
	require.Zero(t, upgraded.ResultCode)
	hostedProgramsABIReceiptVersion(t, upgraded, 4)
	return f
}

func (f *hostedProgramsABIFixture) call(t *testing.T) lxphost.Receipt {
	t.Helper()
	budget := [7]uint64{1000000, 16777216, 1048576, 1048576, 64, 1048576, 4096}
	payload, err := programs.EncodeCallPayload(f.program, 5, "layerx_call", nil, []byte{0, 0},
		[]byte("LayerX/programs/access-declaration/v1\x00\x00"), 16, budget)
	require.NoError(t, err)
	return f.execute(t, 0x00090003, payload)
}

func (f *hostedProgramsABIFixture) merge(t *testing.T) {
	t.Helper()
	f.ctx = f.ctx.WithBlockHeight(mergeHeight)
	rewindBeforeMerge(t, f.app, f.ctx)
	f.app.UpgradeKeeper.ApplyUpgrade(f.ctx, upgradetypes.Plan{Name: MergeUpgrade, Height: mergeHeight, Info: activationInfo(t)})
	require.Equal(t, []byte{0, 5}, f.ctx.KVStore(f.app.GetKey(LayerXHostStoreKey)).Get([]byte(hostedProgramsGuestABIKey)))
	require.Equal(t, mergeHeight, f.app.UpgradeKeeper.GetDoneHeight(f.ctx, MergeUpgrade))
	require.Equal(t, uint16(5), f.app.hostedProgramsGuestABI)
	require.Same(t, f.app.LayerXHost, f.app.LayerXPayKeeper.Host())
	require.Same(t, f.app.LayerXHost, f.app.LayerXAgentKeeper.Host())
}

func TestHostedKernelProgramsABIFollowsTheMergeUpgrade(t *testing.T) {
	f := newHostedProgramsABIFixture(t)
	cached, _ := f.ctx.WithBlockHeight(mergeHeight).CacheContext()
	_, err := f.app.mergeUpgradePlan(cached,
		upgradetypes.Plan{Name: MergeUpgrade, Height: mergeHeight, Info: "invalid"},
		f.app.UpgradeKeeper.GetModuleVersionMap(cached))
	require.Error(t, err)
	require.Nil(t, cached.KVStore(f.app.GetKey(LayerXHostStoreKey)).Get([]byte(hostedProgramsGuestABIKey)))
	require.Equal(t, uint16(4), f.app.hostedProgramsGuestABI)
	before := f.call(t)
	require.Equal(t, "LXP_ERR_VERSION_UNSUPPORTED", before.ResultCode.Error())
	hostedProgramsABIReceiptVersion(t, before, 4)
	oldHost := f.app.LayerXHost
	address := common.HexToAddress("0x101D")
	oldPrecompile := f.app.customPrecompiles[address]
	f.merge(t)
	require.NotSame(t, oldHost, f.app.LayerXHost)
	require.NotEqual(t, oldPrecompile, f.app.customPrecompiles[address])
	f.ctx = f.ctx.WithBlockHeight(mergeHeight + 1)
	f.app.DistrKeeper.SetPreviousProposerConsAddr(f.ctx, f.proposer)
	f.app.BeginBlock(f.ctx, f.ctx.BlockHeight(), nil, nil, false)
	after := f.call(t)
	require.Zero(t, after.ResultCode, after.ResultCode.Error())
	require.NotNil(t, after.ProgramOutcome)
	require.Zero(t, after.ProgramOutcome.Status)
	hostedProgramsABIReceiptVersion(t, after, 5)
}

func TestHostedKernelProgramsABIDefaultsToFourWithoutTheUpgrade(t *testing.T) {
	f := newHostedProgramsABIFixture(t)
	f.app.DistrKeeper.SetPreviousProposerConsAddr(f.ctx, f.proposer)
	f.app.BeginBlock(f.ctx, f.ctx.BlockHeight(), nil, nil, false)
	require.Nil(t, f.ctx.KVStore(f.app.GetKey(LayerXHostStoreKey)).Get([]byte(hostedProgramsGuestABIKey)))
	require.Equal(t, uint16(4), f.app.hostedProgramsGuestABI)
	receipt := f.call(t)
	require.Equal(t, "LXP_ERR_VERSION_UNSUPPORTED", receipt.ResultCode.Error())
	hostedProgramsABIReceiptVersion(t, receipt, 4)
}

func TestHostedKernelProgramsABISurvivesRestart(t *testing.T) {
	f := newHostedProgramsABIFixture(t)
	f.merge(t)
	info, err := f.app.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	home := filepath.Dir(filepath.Dir(info))
	f.app.SetDeliverStateToCommit()
	f.app.WriteState().Commit(true)
	require.NoError(t, f.app.Close())

	f.app = New(dbm.NewMemDB(), nil, true, map[int64]bool{}, home, 1, true, config.TestConfig(),
		MakeEncodingConfig(), wasm.EnableAllProposals, TestAppOpts{}, EmptyWasmOpts, nil)
	f.ctx = f.app.NewUncachedContext(false, tmproto.Header{
		Height: mergeHeight + 1, Time: f.ctx.BlockTime(), ChainID: f.ctx.ChainID(),
	})
	require.Equal(t, []byte{0, 5}, f.ctx.KVStore(f.app.GetKey(LayerXHostStoreKey)).Get([]byte(hostedProgramsGuestABIKey)))
	require.Equal(t, uint16(5), f.app.hostedProgramsGuestABI)
	t.Log("Host construction precedes store loading; the restarted app selects ABI 5 after LoadLatestVersion and before its first block")
	require.Equal(t, mergeHeight, f.app.UpgradeKeeper.GetDoneHeight(f.ctx, MergeUpgrade))
	f.app.DistrKeeper.SetPreviousProposerConsAddr(f.ctx, f.proposer)
	f.app.BeginBlock(f.ctx, f.ctx.BlockHeight(), nil, nil, false)
	receipt := f.call(t)
	require.Zero(t, receipt.ResultCode, receipt.ResultCode.Error())
	hostedProgramsABIReceiptVersion(t, receipt, 5)
	require.Equal(t, mergeHeight, f.app.UpgradeKeeper.GetDoneHeight(f.ctx, MergeUpgrade))
}
