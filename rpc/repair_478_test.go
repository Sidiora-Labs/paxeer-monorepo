//go:build cgo

package evmrpc_test

import (
	"archive/tar"
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

const repair478KernelAllowance = uint64(10_000_000)
const repair478CPUAllowance = uint64(1_000_000)
const repair478Attempt9 = "8fc7d8b34b2026250836f64aba21ef67bd02f663"
const repair478Base = "71ea2b5cfcf9fa483db0ed65d0630e08bca86d9f"

var repair478Program = [32]byte(bytes.Repeat([]byte{11}, 32))

func repair478Command(t *testing.T, dir string, env []string, name string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s %v: %s", name, args, output)
	return output
}

// All guests use the same compiler, profile, SDK and fixture. Historical
// sources come from the fixed revisions, never from a reconstructed test double.
type repair478Guest struct {
	Revision       string `json:"revision"`
	RawBytes       int    `json:"raw_bytes"`
	OptimizedBytes int    `json:"optimized_bytes"`
	raw, packaged  []byte
	modulePath     string
}

func repair478RawGuest(t *testing.T, revision string) repair478Guest {
	t.Helper()
	root, err := filepath.Abs("..")
	require.NoError(t, err)
	target := filepath.Join(root, "programs", "target", "repair-478-guest")
	sdkPath := filepath.Join(root, "programs", "sdk", "rust")

	dir := t.TempDir()
	if revision != "" {
		data := repair478Command(t, root, nil, "git", "archive", revision, "programs/crates/layerx-programs-ai-market/src")
		reader := tar.NewReader(bytes.NewReader(data))
		for {
			header, err := reader.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			if header.Typeflag != tar.TypeReg {
				continue
			}
			rel, err := filepath.Rel("programs/crates/layerx-programs-ai-market", header.Name)
			require.NoError(t, err)
			destination := filepath.Join(dir, rel)
			require.NoError(t, os.MkdirAll(filepath.Dir(destination), 0755))
			content, err := io.ReadAll(reader)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(destination, content, 0644))
		}
	} else {
		source := filepath.Join(root, "programs/crates/layerx-programs-ai-market/src")
		require.NoError(t, filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(source, path)
			if err != nil {
				return err
			}
			destination := filepath.Join(dir, "src", rel)
			if entry.IsDir() {
				return os.MkdirAll(destination, 0755)
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(destination, content, 0644)
		}))
	}
	manifest := fmt.Sprintf(`[package]
name = "layerx-programs-ai-market"
version = "0.1.0"
edition = "2021"
[workspace]
[dependencies]
layerx-program-sdk = { path = %q }
sha2 = { version = "=0.10.9", default-features = false }
`, sdkPath)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(manifest), 0644))
	buildScript, err := os.ReadFile(filepath.Join(root, "programs/crates/layerx-programs-ai-market/build.rs"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "build.rs"), buildScript, 0644))
	// Resolve the same vendored dependencies as the production workspace.
	// Cargo configuration is discovered from the isolated workspace's cwd.
	config := fmt.Sprintf(`[source.crates-io]
replace-with = "vendored-sources"
[source.vendored-sources]
directory = %q
[net]
offline = true
`, filepath.Join(root, "programs", "vendor"))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".cargo"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".cargo", "config.toml"), []byte(config), 0644))
	recipe := filepath.Join(root, "programs/crates/layerx-programs-ai-market/tests/fixtures/guest-recipe.sh")
	repair478Command(t, root, nil, "bash", recipe, "check")
	// Isolated workspaces need their own lock, then compile with the same
	// locked recipe as the production target and both committed fixtures.
	repair478Command(t, dir, nil, "cargo", "generate-lockfile", "--offline")
	repair478Command(t, dir, []string{
		"CARGO_TARGET_DIR=" + target,
		"PAXAI_CHAIN_DOMAIN=62b6b14734fa769c3e0fbc94de495a3d038be49a2524b1c071d75d0a4eed71f6",
	}, "bash", recipe, "compile", dir)
	modulePath := filepath.Join(target, "wasm32-unknown-unknown/release/layerx_programs_ai_market.wasm")
	raw, err := os.ReadFile(modulePath)
	require.NoError(t, err)
	label := revision
	if label == "" {
		label = "working-tree"
	}
	return repair478Guest{Revision: label, RawBytes: len(raw), raw: raw, modulePath: modulePath}
}

func repair478PackageGuest(t *testing.T, guest repair478Guest) repair478Guest {
	t.Helper()
	root, err := filepath.Abs("..")
	require.NoError(t, err)
	recipe := filepath.Join(root, "programs/crates/layerx-programs-ai-market/tests/fixtures/guest-recipe.sh")
	dir := filepath.Dir(guest.modulePath)
	rawPath := guest.modulePath
	optimizedPath := filepath.Join(dir, "ai_market.packaged.wasm")
	repair478Command(t, dir, nil, "bash", recipe, "package", rawPath, optimizedPath)
	guest.packaged, err = os.ReadFile(optimizedPath)
	require.NoError(t, err)
	require.LessOrEqual(t, len(guest.packaged), 524184)
	guest.OptimizedBytes = len(guest.packaged)
	fmt.Fprintf(os.Stderr, "Repair478 guest revision=%s raw_bytes=%d optimized_bytes=%d\n", guest.Revision, guest.RawBytes, guest.OptimizedBytes)
	return guest
}

func repair478GuestSet(t *testing.T, revisions []string) map[string]repair478Guest {
	t.Helper()
	guests := make(map[string]repair478Guest)
	for _, revision := range revisions {
		guest := repair478PackageGuest(t, repair478RawGuest(t, revision))
		guests[guest.Revision] = guest
	}
	return guests
}

func repair478Guests(t *testing.T) (before, after []byte) {
	t.Helper()
	root, err := filepath.Abs("..")
	require.NoError(t, err)
	mergedBase := string(bytes.TrimSpace(repair478Command(t, root, nil, "git", "merge-base", "HEAD", "origin/main")))
	sizes := repair478GuestSet(t, []string{repair478Base, mergedBase, ""})
	measurement := map[string]map[string]repair478Guest{
		"merged_base": {"historical": sizes[repair478Base], "candidate": sizes[mergedBase]},
		"final_tree":  {"historical": sizes[repair478Base], "candidate": sizes["working-tree"]},
	}
	encoded, err := json.MarshalIndent(measurement, "", "  ")
	require.NoError(t, err)
	evidenceDir := filepath.Join(root, "programs", "target", "repair-478")
	require.NoError(t, os.MkdirAll(evidenceDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(evidenceDir, "guest-sizes.json"), append(encoded, '\n'), 0600))
	return sizes[repair478Base].packaged, sizes["working-tree"].packaged
}

func TestFundConsumptionAttribution(t *testing.T) {
	// Measure each source-changing boundary, including its immediate predecessor.
	revisions := []string{repair478Base, "075a770edb^", "075a770edb", "2d8083af02^", "2d8083af02", "7dc80ac4fc2fdc96f295d738f48f26c355f6a9a0", "101f526f032f95aa585c3458967dae66f1bb053c", ""}
	for _, revision := range revisions {
		label := revision
		if label == "" {
			label = "working-tree"
		}
		t.Run(label, func(t *testing.T) {
			if revision != "" {
				repair478Command(t, "..", nil, "git", "cat-file", "-e", revision+"^{commit}")
				repair478Command(t, "..", nil, "git", "merge-base", "--is-ancestor", revision, "origin/main")
			}
			guest := repair478PackageGuest(t, repair478RawGuest(t, revision))
			dir := t.TempDir()
			receipt := repair478RunObserved(t, guest.packaged, repair478KernelAllowance, false, dir)
			// Native admission enforces the production ceiling before guest execution.
			require.Equal(t, int64(-3), int64(receipt.ResultCode))
			require.Zero(t, receipt.ProgramOutcome.CPUFuel)
			measure := func(t *testing.T, form string, wasm []byte) {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "wasm"), wasm, 0600))
				output := repair478Command(t, "../programs", nil, "cargo", "run", "-q", "-p", "layerx-programs-ai-market", "--example", "repair_478_fixture", "--", "--measure", dir, fmt.Sprint(repair478KernelAllowance))
				output = bytes.TrimSpace(output)
				if last := bytes.LastIndexByte(output, '\n'); last >= 0 {
					output = output[last+1:]
				}
				var measurement struct {
					CPUFuel uint64 `json:"cpu_fuel"`
				}
				require.NoError(t, json.Unmarshal(output, &measurement), "%s", output)
				require.Positive(t, measurement.CPUFuel)
				require.LessOrEqual(t, measurement.CPUFuel, repair478KernelAllowance)
				if revision == "" && form == "packaged" {
					require.LessOrEqual(t, measurement.CPUFuel, uint64(994968))
				}
				fmt.Fprintf(os.Stderr, "Repair478 attribution revision=%s form=%s bytes=%d cpu_fuel=%d native_admission=%d kernel_gas=%d\n", label, form, len(wasm), measurement.CPUFuel, receipt.ResultCode, receipt.KernelGasUsed)
			}
			t.Run("raw", func(t *testing.T) { measure(t, "raw", guest.raw) })
			t.Run("packaged", func(t *testing.T) { measure(t, "packaged", guest.packaged) })
		})
	}
}

func repair478Fixture(t *testing.T, owner, asset [32]byte, wasm []byte) map[string][]byte {
	t.Helper()
	guest := filepath.Join(t.TempDir(), "ai_market.wasm")
	require.NoError(t, os.WriteFile(guest, wasm, 0600))
	output := repair478Command(t, "../programs", nil, "cargo", "run", "-q", "-p", "layerx-programs-ai-market", "--example", "repair_478_fixture", "--", hex.EncodeToString(owner[:]), hex.EncodeToString(asset[:]), guest)
	output = bytes.TrimSpace(output)
	if last := bytes.LastIndexByte(output, '\n'); last >= 0 {
		output = output[last+1:]
	}
	var encoded map[string]string
	require.NoError(t, json.Unmarshal(output, &encoded), "%s", output)
	decoded := make(map[string][]byte)
	for key, value := range encoded {
		data, err := hex.DecodeString(value)
		require.NoError(t, err)
		decoded[key] = data
	}
	return decoded
}

func repair478Run(t *testing.T, wasm []byte, cpuAllowance uint64, evidence bool) lxphost.Receipt {
	t.Helper()
	return repair478RunObserved(t, wasm, cpuAllowance, evidence, "")
}

func repair478RunObserved(t *testing.T, wasm []byte, cpuAllowance uint64, evidence bool, observationDir string) lxphost.Receipt {
	t.Helper()
	b := newSeqVecBuilder(t)
	a := app.Setup(t, false, false, false)
	ctx, _ := a.GetContextForDeliverTx(nil).CacheContext()
	ctx = ctx.WithBlockHeight(1).WithBlockTime(time.UnixMilli(seqVecTimeBase + seqVecBlockMs))
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx)).WithEventManager(sdk.NewEventManager())
	key := a.GetKey(app.LayerXHostStoreKey)
	host, err := lxphost.NewHost(lxphost.Config{StoreKey: key, NetworkID: seqVecChainID, ParameterVersion: 1, TimestampWindowMs: 60000, GasLimit: repair478KernelAllowance, ProgramsGuestABI: 5})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x41}, ed25519.SeedSize))
	publicKey := [32]byte(private.Public().(ed25519.PublicKey))
	message, err := evmtypes.LayerXBindMessage(a.EvmKeeper.ChainID(ctx), b.sender, 0)
	require.NoError(t, err)
	_, err = a.EvmKeeper.BindLayerX(ctx, b.sender, publicKey, [64]byte(ed25519.Sign(private, message)))
	require.NoError(t, err)
	did := []byte(evmtypes.LayerXDid(publicKey))
	actor, err := lxphost.DIDID(did)
	require.NoError(t, err)
	authorityHash, err := lxphost.AuthorityHash(lxphost.AuthorityKindOwner, [32]byte{}, publicKey)
	require.NoError(t, err)
	authority := lxphost.KernelAuthority{Kind: lxphost.AuthorityKindOwner, AuthorityHash: authorityHash, Bound: true, Actor: actor, Principal: actor, VerifiedKey: publicKey}
	moduleKey := func(key []byte) []byte { return append([]byte{1, 0, 9}, key...) }
	store := ctx.KVStore(key)
	initialFee := binary.BigEndian.AppendUint32([]byte("LXFR1"), 1)
	for i := 0; i < 7; i++ {
		initialFee = binary.BigEndian.AppendUint64(initialFee, 1)
	}
	occupancySalt := bytes.Repeat([]byte{0x56}, 32)
	occupancyMaterial := append([]byte("LX:ASSET:v1"), authority.Actor[:]...)
	occupancyAsset := sha256.Sum256(append(occupancyMaterial, occupancySalt...))
	initialFee = append(initialFee, occupancyAsset[:]...)
	for _, value := range []uint64{1000, 8, 1, 8, 1, 100, 1, 0, 1} {
		initialFee = binary.BigEndian.AppendUint64(initialFee, value)
	}
	initialFee = append(initialFee, authority.Actor[:]...)
	initialFee = append(initialFee, make([]byte, 16)...)
	store.Set(moduleKey([]byte("progfee/active/v1")), initialFee)
	store.Set(moduleKey(binary.BigEndian.AppendUint32([]byte("progfee/history/v1/"), 1)), initialFee)
	metering := binary.BigEndian.AppendUint32([]byte("LXMR1"), 1)
	for _, coefficient := range []uint64{1, 1, 1, 1, 1, 8, 8, 64, 8} {
		metering = binary.BigEndian.AppendUint64(metering, coefficient)
	}
	metering = binary.BigEndian.AppendUint64(metering, 1)
	metering = append(metering, 1)
	metering = append(metering, authority.Actor[:]...)
	store.Set(moduleKey([]byte("progmet/active/v1")), metering)
	store.Set(moduleKey(binary.BigEndian.AppendUint32([]byte("progmet/history/v1/"), 1)), metering)

	mainName, mainID, err := lxphost.MainAccountID(publicKey)
	require.NoError(t, err)
	field := func(out, value []byte) []byte {
		return append(binary.BigEndian.AppendUint32(out, uint32(len(value))), value...)
	}
	main := field(nil, mainID[:])
	main = field(main, []byte(mainName))
	main = append(main, 1)
	main = binary.BigEndian.AppendUint64(main, 0)
	main = binary.BigEndian.AppendUint64(main, 1000000000)
	main = field(main, occupancyAsset[:])
	main = append(main, 1)
	main = binary.BigEndian.AppendUint64(main, 0)
	main = binary.BigEndian.AppendUint64(main, 1)
	main = append(main, 0, 0)
	main = field(main, publicKey[:])
	store.Set(append([]byte{6}, mainID[:]...), append(main, 1))
	mainAccount, err := host.ReadAccount(ctx, mainName, mainID)
	require.NoError(t, err)
	require.Equal(t, int64(1000000000), mainAccount.Balance.Int64())

	execute := func(activityType uint32, payload []byte) lxphost.Receipt {
		sequence, err := host.ReadIdentitySequence(ctx, did)
		require.NoError(t, err)
		last, err := host.ReadLastSequence(ctx, did)
		require.NoError(t, err)
		var feeLimit [16]byte
		binary.BigEndian.PutUint64(feeLimit[8:], 100000000)
		timestamp := uint64(ctx.BlockTime().UnixMilli())
		receipt, err := host.Execute(lxphost.WithSequence(ctx, last+1), lxphost.Activity{
			ProtocolVersion: codec.StateCommitmentProtocolVersion, Type: activityType,
			ActorDID: did, AuthorityKey: publicKey, AccountSequence: sequence,
			NotBefore: timestamp, NotAfter: timestamp,
			IdempotencyKey: sha256.Sum256(binary.BigEndian.AppendUint64(append([]byte{}, payload...), last+1)),
			Payload:        payload, FeeLimit: feeLimit,
		}, authority)
		require.NoError(t, err)
		if receipt.ProgramOutcome != nil {
			t.Logf("program activity=%08x candidate=%t result=%v status=%d kind=%d CPU fuel=%d response=%x", activityType, evidence, receipt.ResultCode, receipt.ProgramOutcome.Status, receipt.ProgramOutcome.Kind, receipt.ProgramOutcome.CPUFuel, receipt.ProgramOutcome.Response)
		}
		require.Zero(t, receipt.ResultCode, "activity=%08x candidate=%t", activityType, evidence)
		return receipt
	}
	occupancyRegistration := append([]byte{0, 1}, occupancyAsset[:]...)
	occupancyRegistration = append(occupancyRegistration, occupancySalt...)
	occupancyRegistration = append(occupancyRegistration, 1, 'O', 1, 'O', 6)
	occupancyRegistration = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(occupancyRegistration, 0), 1000000000)
	execute(0x00010001, append(occupancyRegistration, 1, 0))
	salt := bytes.Repeat([]byte{0x55}, 32)
	assetMaterial := append([]byte("LX:ASSET:v1"), authority.Actor[:]...)
	asset := sha256.Sum256(append(assetMaterial, salt...))
	register := append([]byte{0, 1}, asset[:]...)
	register = append(register, salt...)
	register = append(register, 1, 'T', 1, 'T', 6)
	register = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(register, 0), 1000000000)
	execute(0x00010001, append(register, 1, 0))
	execute(0x00010004, append([]byte{0, 1}, asset[:]...))
	_, payer, err := lxphost.AssetAccountID(publicKey, asset)
	require.NoError(t, err)
	mint := append([]byte{0, 1}, asset[:]...)
	mint = append(mint, payer[:]...)
	mint = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(mint, 0), 500000000)
	execute(0x0001000a, mint)

	treasuryID, err := codec.DeriveAccountID([]byte("system:fees"))
	require.NoError(t, err)
	treasury := field(nil, treasuryID[:])
	treasury = field(treasury, []byte("system:fees"))
	treasury = append(treasury, 10)
	treasury = append(treasury, make([]byte, 16)...)
	treasury = field(treasury, occupancyAsset[:])
	treasury = append(treasury, 1)
	treasury = append(treasury, make([]byte, 16)...)
	treasury = append(treasury, 0, 0)
	treasury = field(treasury, make([]byte, 32))
	store.Set(append([]byte{6}, treasuryID[:]...), append(treasury, 0))
	treasuryAccount, err := host.ReadAccount(ctx, "system:fees", treasuryID)
	require.NoError(t, err)
	require.False(t, treasuryAccount.Frozen)
	require.True(t, treasuryAccount.Balance.IsZero())
	require.Equal(t, occupancyAsset, treasuryAccount.AssetID)

	fee := binary.BigEndian.AppendUint32([]byte("LXFR1"), 1)
	for i := 0; i < 7; i++ {
		fee = binary.BigEndian.AppendUint64(fee, 1)
	}
	fee = append(fee, occupancyAsset[:]...)
	for _, value := range []uint64{1000, 8, 1, 8, 1, 100, 1, 0, 1} {
		fee = binary.BigEndian.AppendUint64(fee, value)
	}
	fee = append(fee, authority.Actor[:]...)
	fee = append(fee, make([]byte, 16)...)
	store.Set(moduleKey([]byte("progfee/active/v1")), fee)
	store.Set(moduleKey(binary.BigEndian.AppendUint32([]byte("progfee/history/v1/"), 1)), fee)

	// The fee record starts at genesis. Finalize each empty preceding block
	// through the kernel rather than synthesizing occupancy or pricing cursors.
	for ctx.BlockHeight() < seqVecFirstHeight+2 {
		last, err := host.ReadLastSequence(ctx, did)
		require.NoError(t, err)
		finalized, err := host.FinalizeProgramsOccupancy(lxphost.WithSequence(ctx, last+1), codec.StateCommitmentProtocolVersion)
		require.NoError(t, err, "finalize empty block %d", ctx.BlockHeight())
		require.NotEmpty(t, finalized)
		ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1)
		ctx = ctx.WithBlockTime(time.UnixMilli(seqVecTimeBase + ctx.BlockHeight()*seqVecBlockMs))
	}
	fixture := repair478Fixture(t, authority.Principal, asset, wasm)
	codeHash := sha256.Sum256(wasm)
	deploy, err := lp.EncodeDeployPayload(repair478Program, 5, 1, authority.Principal, codeHash, nil, wasm)
	require.NoError(t, err)
	execute(0x00090001, deploy)
	// ABI5 registration consumes the profile opt-in established before upgrade.
	store.Set(moduleKey(append([]byte("program-owner\x00"), repair478Program[:]...)), append([]byte{1}, authority.Principal[:]...))
	store.Set(moduleKey(append([]byte("program-account-profile\x00"), repair478Program[:]...)), append([]byte{2}, authority.Principal[:]...))
	registration, err := lp.EncodeAccountPayload(repair478Program, 1, asset, []byte("paxai/rewards/v1"))
	require.NoError(t, err)
	execute(0x00090006, registration)
	budget := [7]uint64{repair478CPUAllowance, 16777216, 1048576, 1048576, 64, 1048576, 4096}
	access := []byte("LayerX/programs/access-declaration/v1\x00\x00")
	baseCaps := []byte{0, 3, 3, 7, 8}
	create, err := lp.EncodeCallPayload(repair478Program, 5, "layerx_call", fixture["create"], baseCaps, access, 16384, budget)
	require.NoError(t, err)
	created := execute(0x00090003, create)
	require.NotNil(t, created.ProgramOutcome)
	require.Equal(t, uint8(1), created.ProgramOutcome.Kind)
	readState := func() []byte {
		head, err := host.ReadProgramStateHead(ctx, repair478Program)
		require.NoError(t, err)
		manifest := store.Get(append([]byte{5, 0, 9}, head[6:]...))
		require.GreaterOrEqual(t, len(manifest), 6)
		cursor := 6
		for i := uint32(0); i < binary.BigEndian.Uint32(manifest[2:6]); i++ {
			require.GreaterOrEqual(t, len(manifest)-cursor, 2)
			n := int(binary.BigEndian.Uint16(manifest[cursor:]))
			cursor += 2
			require.GreaterOrEqual(t, len(manifest)-cursor, n+36)
			if bytes.Equal(manifest[cursor:cursor+n], []byte("paxai/state/v1")) {
				key := manifest[cursor+n : cursor+n+32]
				value := store.Get(append([]byte{5, 0, 9}, key...))
				require.Equal(t, int(binary.BigEndian.Uint32(manifest[cursor+n+32:])), len(value))
				return bytes.Clone(value)
			}
			cursor += n + 36
		}
		t.Fatal("shared state is absent")
		return nil
	}
	require.Equal(t, fixture["current"], readState())
	sequence, err := host.ReadIdentitySequence(ctx, did)
	require.NoError(t, err)
	last, err := host.ReadLastSequence(ctx, did)
	require.NoError(t, err)
	finalized, err := host.FinalizeProgramsOccupancy(lxphost.WithSequence(ctx, last+1), codec.StateCommitmentProtocolVersion)
	require.NoError(t, err, "finalize occupancy for block %d", ctx.BlockHeight())
	require.NotEmpty(t, finalized)
	last, err = host.ReadLastSequence(ctx, did)
	require.NoError(t, err)
	nextIdentity, err := host.ReadIdentitySequence(ctx, did)
	require.NoError(t, err)
	require.Equal(t, sequence, nextIdentity, "block finalization must preserve the actor nonce")
	// Balance sight binds the latest sequence and root, including finalization.
	// Re-register the existing account through the production idempotent path.
	anchor := execute(0x00090006, registration)
	require.Equal(t, fixture["current"], readState())
	require.Greater(t, anchor.GlobalSequence, created.GlobalSequence)
	require.NotEqual(t, anchor.ResultingStateRoot, created.ResultingStateRoot)
	sequence, err = host.ReadIdentitySequence(ctx, did)
	require.NoError(t, err)
	last, err = host.ReadLastSequence(ctx, did)
	require.NoError(t, err)
	require.Equal(t, anchor.GlobalSequence, last)
	decoded, err := codec.DecodeReceipt(anchor.Encoded)
	require.NoError(t, err)
	digest := decoded.Digest()
	rewards := [32]byte(fixture["rewards"])
	// Canonical v1 capability order follows the runtime's authority-key order:
	// event, transfer, sight, shared read and shared write.
	caps := []byte{0, 5, 3, 5}
	caps = append(caps, asset[:]...)
	caps = append(caps, rewards[:]...)
	caps = append(caps, make([]byte, 8)...)
	caps = binary.BigEndian.AppendUint64(caps, 100)
	caps = append(caps, 10)
	caps = append(caps, rewards[:]...)
	caps = append(caps, asset[:]...)
	caps = append(caps, digest[:]...)
	caps = append(caps, 7, 8)
	// Every activity stays within the protocol CPU ceiling.
	budget[0] = cpuAllowance
	fund, err := lp.EncodeCallPayload(repair478Program, 5, "layerx_call", fixture["fund"], caps, access, 16384, budget)
	require.NoError(t, err)
	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1)
	ctx = ctx.WithBlockTime(time.UnixMilli(seqVecTimeBase + ctx.BlockHeight()*seqVecBlockMs))
	timestamp := uint64(ctx.BlockTime().UnixMilli())
	var feeLimit [16]byte
	binary.BigEndian.PutUint64(feeLimit[8:], 100000000)
	activity := lxphost.Activity{
		ProtocolVersion: codec.StateCommitmentProtocolVersion, Type: 0x00090003,
		ActorDID: did, AuthorityKey: publicKey, AccountSequence: sequence,
		NotBefore: timestamp, NotAfter: timestamp, IdempotencyKey: sha256.Sum256(fund),
		Payload: fund, FeeLimit: feeLimit,
		ReceiptAuthorities: []lxphost.ReceiptAuthority{{CanonicalReceipt: anchor.Encoded, BlockHeight: uint64(ctx.BlockHeight() - 1)}},
	}
	// Transport the real kernel inputs and observed balance to the Rust executor.
	dir := "../programs/target/repair-478"
	if observationDir != "" {
		dir = observationDir
	}
	if evidence || observationDir != "" {
		require.NoError(t, os.MkdirAll(dir, 0755))
		balance, err := host.ReadBalance(ctx, rewards, asset)
		require.NoError(t, err)
		sight := append([]byte{}, rewards[:]...)
		sight = append(sight, asset[:]...)
		sight = append(sight, balance.BigInt().FillBytes(make([]byte, 16))...)
		sight = append(sight, digest[:]...)
		sight = append(sight, anchor.ResultingStateRoot[:]...)
		sight = binary.BigEndian.AppendUint64(sight, anchor.GlobalSequence)
		fixture["owner"], fixture["asset"] = authority.Principal[:], asset[:]
		fixture["wasm"], fixture["sight"], fixture["prior-receipt"] = wasm, sight, anchor.Encoded
		fixture["call"] = fund
		for key, value := range fixture {
			require.NoError(t, os.WriteFile(filepath.Join(dir, key), value, 0600))
		}
	}
	receipt, err := host.Execute(lxphost.WithSequence(ctx, last+1), activity, authority)
	require.NoError(t, err)
	require.NotNil(t, receipt.ProgramOutcome)
	t.Logf("Fund kernel gas=%d kernel allowance=%d CPU allowance=%d CPU fuel=%d result=%v status=%d terminal=%d response=%x", receipt.KernelGasUsed, repair478KernelAllowance, cpuAllowance, receipt.ProgramOutcome.CPUFuel, receipt.ResultCode, receipt.ProgramOutcome.Status, receipt.ProgramOutcome.Kind, receipt.ProgramOutcome.Response)
	if evidence {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "observed-terminal"), receipt.ProgramOutcome.Response, 0600))
		revision := bytes.TrimSpace(repair478Command(t, "..", nil, "git", "rev-parse", "HEAD"))
		outcome := fmt.Sprintf("[fund.outcome]\nrevision = %q\nresult_code = %d\ncpu_fuel = %d\nterminal_kind = %d\nkernel_allowance = %d\ncpu_allowance = %d\n", revision, receipt.ResultCode, receipt.ProgramOutcome.CPUFuel, receipt.ProgramOutcome.Kind, repair478KernelAllowance, cpuAllowance)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "fund-outcome.kvx"), []byte(outcome), 0600))
	}
	terminal := filepath.Join(t.TempDir(), "terminal")
	require.NoError(t, os.WriteFile(terminal, receipt.ProgramOutcome.Response, 0600))
	diagnostic := repair478Command(t, "../programs", nil, "cargo", "run", "-q", "-p", "layerx-programs-ai-market", "--example", "repair_478_fixture", "--", "--terminal", fmt.Sprint(receipt.ProgramOutcome.Kind), terminal)
	t.Logf("Fund decoded terminal: %s", diagnostic)
	if evidence {
		probe := repair478Command(t, "../programs", nil, "cargo", "run", "-q", "-p", "layerx-programs-ai-market", "--example", "repair_478_fixture", "--", "--probe", "target/repair-478")
		t.Logf("Fund runtime executor: %s", probe)
	}
	if evidence {
		require.Zero(t, receipt.ResultCode)
		require.Zero(t, receipt.ProgramOutcome.Status)
		require.Equal(t, uint8(1), receipt.ProgramOutcome.Kind)
		require.Positive(t, receipt.KernelGasUsed)
		require.LessOrEqual(t, receipt.KernelGasUsed, repair478KernelAllowance)
		require.Positive(t, receipt.ProgramOutcome.CPUFuel)
		require.LessOrEqual(t, receipt.ProgramOutcome.CPUFuel, budget[0])
		require.Less(t, receipt.ProgramOutcome.CPUFuel, repair478KernelAllowance)
		require.Equal(t, fixture["next"], readState())
		require.Len(t, receipt.Transfers, 1)
		require.Equal(t, asset, receipt.Transfers[0].Asset)
		require.Equal(t, payer, receipt.Transfers[0].From)
		require.Equal(t, rewards, receipt.Transfers[0].To)
		require.Equal(t, int64(100), receipt.Transfers[0].Amount.Int64())
		balance, err := host.ReadBalance(ctx, rewards, asset)
		require.NoError(t, err)
		require.Equal(t, int64(100), balance.Int64())
		t.Logf("Balance sight anchor sequence=%d root=%x", anchor.GlobalSequence, anchor.ResultingStateRoot)
		fixture["terminal"] = receipt.ProgramOutcome.Response
		for key, value := range fixture {
			require.NoError(t, os.WriteFile(filepath.Join(dir, key), value, 0600))
		}
	}
	return receipt
}

func TestRepair478_FundWithinProductionAllowance(t *testing.T) {
	beforeWasm, afterWasm := repair478Guests(t)
	var before, after lxphost.Receipt
	baselineOK := t.Run("before", func(t *testing.T) { before = repair478Run(t, beforeWasm, repair478CPUAllowance, false) })
	candidateOK := t.Run("after", func(t *testing.T) { after = repair478Run(t, afterWasm, repair478CPUAllowance, true) })
	require.True(t, baselineOK, "baseline fixture setup must succeed")
	require.True(t, candidateOK, "candidate Fund must succeed")
	require.Less(t, after.ProgramOutcome.CPUFuel, before.ProgramOutcome.CPUFuel)
	require.Less(t, after.ProgramOutcome.CPUFuel, repair478CPUAllowance)
	require.Less(t, after.ProgramOutcome.CPUFuel, uint64(999968))
	require.LessOrEqual(t, after.ProgramOutcome.CPUFuel, uint64(802503))
	t.Logf("Fund kernel gas before=%d after=%d allowance=%d", before.KernelGasUsed, after.KernelGasUsed, repair478KernelAllowance)
	t.Logf("Fund CPU fuel historical_stop=%d attempt9_stop=999968 production=%d kernel_allowance=%d", before.ProgramOutcome.CPUFuel, after.ProgramOutcome.CPUFuel, repair478KernelAllowance)
	fmt.Fprintf(os.Stderr, "Repair478 Fund CPU fuel historical_stop=%d attempt9_stop=999968 production=%d kernel_allowance=%d\n", before.ProgramOutcome.CPUFuel, after.ProgramOutcome.CPUFuel, repair478KernelAllowance)
	fmt.Fprintf(os.Stderr, "Repair478 Fund kernel gas before=%d after=%d allowance=%d\n", before.KernelGasUsed, after.KernelGasUsed, repair478KernelAllowance)
	revision := bytes.TrimSpace(repair478Command(t, "..", nil, "git", "rev-parse", "HEAD"))
	measurement := fmt.Sprintf("[measurement.22.31.478]\nrevision = %q\nbase_revision = %q\nattempt9_revision = %q\nbefore_cpu_fuel = 999968\nhistorical_stop_cpu_fuel = %d\nafter_cpu_fuel = %d\nkernel_allowance = %d\ncpu_allowance = %d\nresult_code = %d\nbefore_kernel_gas = %d\nafter_kernel_gas = %d\n", revision, repair478Base, repair478Attempt9, before.ProgramOutcome.CPUFuel, after.ProgramOutcome.CPUFuel, repair478KernelAllowance, repair478CPUAllowance, after.ResultCode, before.KernelGasUsed, after.KernelGasUsed)
	require.NoError(t, os.WriteFile("../programs/target/repair-478/measurement.kvx", []byte(measurement), 0600))
}
